package build

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	cp "github.com/otiai10/copy"

	"github.com/adrg/frontmatter"
	"github.com/briandowns/spinner"
	"github.com/lukeshay/gocden/pkg/assets"
	"github.com/lukeshay/gocden/pkg/cmds"
	"github.com/lukeshay/gocden/pkg/config"
	"github.com/lukeshay/gocden/pkg/markdown"
	"github.com/lukeshay/gocden/pkg/validation"
	cli "github.com/urfave/cli/v2"
	"github.com/yuin/goldmark/parser"
)

type DocMatter struct {
	Title       string `yaml:"title" validate:"required"`
	Description string `yaml:"description" `
	Path        string `yaml:"path"`
	Section     string `yaml:"section"`
}

type DocFile struct {
	Path     string
	OutPath  string
	InPath   string
	Matter   DocMatter
	File     *os.File
	Contents string
	ModTime  time.Time
}

var (
	md                  = markdown.Create()
	mdRegExp            = regexp.MustCompile("^.+\\.(md)$")
	fileNameOrderRegExp = regexp.MustCompile("/(\\d+)-")
)

const sitemapUrl = `
  <url>
    <loc>%s%s</loc>
    <lastmod>%s</lastmod>
  </url>`

const sitemapStart = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`

const sitemapEnd = `
</urlset>`

func Build(c *cli.Context) error {
	conf := cmds.GetConfigFromCliContext(c)

	fmt.Printf("Build docs in %s to %s\n", conf.Build.Source, conf.Build.Output)

	spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	spin.Suffix = " Building docs..."
	spin.Start()

	_, navSections, err := BuildAllFiles(c)

	sitemap := sitemapStart

	for _, section := range *navSections {
		for _, page := range section.Pages {
			sitemap += fmt.Sprintf(sitemapUrl, conf.Url, page.Href, time.Now().Format("2006-01-02"))
		}
	}

	sitemap += sitemapEnd

	sitemapPath := filepath.Join(conf.Build.Output, "sitemap.xml")

	if err := os.WriteFile(sitemapPath, []byte(sitemap), 0755); err != nil {
		fmt.Printf("Error writing sitemap: %s\n", err.Error())

		return err
	}

	spin.Stop()

	if err != nil {
		fmt.Printf("Error building docs: %s\n", err.Error())
		return err
	}

	fmt.Println("Docs built successfully")

	return nil
}

func BuildAllFiles(c *cli.Context) (*[]DocFile, *[]*assets.NavSection, error) {
	cwd := cmds.GetCwdFlag(c)
	conf := cmds.GetConfigFromCliContext(c)

	srcDir := filepath.Join(cwd, conf.Build.Source)
	outDir := filepath.Join(cwd, conf.Build.Output)

	os.RemoveAll(outDir)
	os.MkdirAll(outDir, os.ModePerm)

	if err := assets.CopyTo("assets", outDir); err != nil {
		return nil, nil, multierror.Prefix(err, "Could not copy gocden assets")
	}

	files := []DocFile{}
	navSections := []*assets.NavSection{
		{
			Title: "",
			Pages: []assets.NavPage{},
		},
	}

	if err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		slog.Info("Processing file in src directory", "src", srcDir, "path", path)

		if err != nil || info.IsDir() {
			return nil
		}

		if !mdRegExp.MatchString(info.Name()) {
			if err := cp.Copy(path, strings.Replace(path, srcDir, outDir, 1)); err != nil {
				return fmt.Errorf("Error copying file %s: %v", info.Name(), err)
			}
		}

		file, err := CreateDocFile(conf, srcDir, outDir, path, info)
		if err != nil {
			return err
		} else if file == nil {
			return nil
		}

		found := false

		for _, section := range navSections {
			if section.Title == file.Matter.Section {
				slog.Info("Adding page to section", "section", section.Title, "page", file.Matter.Title, "href", file.Path)

				section.Pages = append(section.Pages, assets.NavPage{
					Title: file.Matter.Title,
					Href:  file.Path,
				})

				found = true
				break
			}
		}

		if !found {
			slog.Info("Creating new section for page", "section", file.Matter.Section, "page", file.Matter.Title, "href", file.Path)

			navSections = append(navSections, &assets.NavSection{
				Title: file.Matter.Section,
				Pages: []assets.NavPage{
					{
						Title: file.Matter.Title,
						Href:  file.Path,
					},
				},
			})
		}

		files = append(files, *file)

		return nil
	}); err != nil {
		return nil, nil, multierror.Prefix(err, "Could not walk src directory")
	}

	slog.Info("Writing html files", "sections", navSections)

	var wg sync.WaitGroup
	var result *multierror.Error

	wg.Add(len(files))

	for idx, file := range files {
		go func(idx int, file DocFile) {
			defer wg.Done()
			defer file.File.Close()

			if err := BuildFile(files, conf, navSections, idx, file); err != nil {
				result = multierror.Append(result, err)
			}
		}(idx, file)
	}

	wg.Wait()

	return &files, &navSections, result.ErrorOrNil()
}

func BuildFile(files []DocFile, conf *config.Config, navSections []*assets.NavSection, idx int, file DocFile) error {
	if info, err := os.Stat(filepath.Dir(file.OutPath)); err != nil || !info.IsDir() {
		_ = os.MkdirAll(filepath.Dir(file.OutPath), os.ModePerm)
	}

	dstHtmlFile, err := os.Create(file.OutPath)
	if err != nil {
		return fmt.Errorf("Could not create file %s: %v", file.OutPath, err.Error())
	}
	defer dstHtmlFile.Close()

	var prev DocFile
	var next DocFile

	if idx == 0 {
		prev = file

		if len(files) > 1 {
			next = files[idx+1]
		} else {
			next = file
		}
	} else if idx == len(files)-1 {
		prev = files[idx-1]
		next = file
	} else {
		prev = files[idx-1]
		next = files[idx+1]
	}

	basePath := ""

	url, err := url.Parse(conf.Url)
	if err == nil {
		basePath = url.Path
	}

	slog.Info("Writing HTML file", "source", file.InPath, "destinition", file.OutPath)

	page := &assets.PageTemplateData{
		Markdown:    template.HTML(file.Contents),
		Name:        conf.Name,
		Title:       file.Matter.Title,
		Description: conf.Description,
		Url:         conf.Url,
		Path:        file.Path,
		Twitter:     conf.Social.Twitter,
		NavSections: navSections,
		UpdatedAt:   file.ModTime,
		Prev: assets.NavPage{
			Title: prev.Matter.Title,
			Href:  prev.Path,
		},
		Next: assets.NavPage{
			Title: next.Matter.Title,
			Href:  next.Path,
		},
		BasePath: basePath,
	}

	if err := page.Execute(dstHtmlFile); err != nil {
		return fmt.Errorf("Could not execute template: %v", err.Error())
	}

	return nil
}

func CreateDocFile(conf *config.Config, srcDir string, outDir string, path string, info os.FileInfo) (*DocFile, error) {
	slog.Info("Processing file in src directory", "src", srcDir, "path", path)

	if info.IsDir() || !mdRegExp.MatchString(info.Name()) {
		return nil, nil
	}

	slog.Info("Adding file to list", "path", path)

	markdownFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Could not open file: %v", err)
	}

	var matter DocMatter

	pageMarkdown, err := frontmatter.MustParse(markdownFile, &matter)
	if err != nil {
		return nil, fmt.Errorf("Could not parse frontmatter: %v", err)
	}

	if err := validation.ValidateAndPrint(fmt.Sprintf("The frontmatter is invalid in %s", info.Name()), &matter); err != nil {
		return nil, err
	}

	var markdownHtmlBuf bytes.Buffer

	if err := md.Convert(pageMarkdown, &markdownHtmlBuf, parser.WithContext(parser.NewContext())); err != nil {
		return nil, fmt.Errorf("Could not convert markdown to html: %v", err)
	}

	markdownHtml := markdownHtmlBuf.String()

	var dstPath string

	if matter.Path != "" {
		dstPath = filepath.Join(outDir, matter.Path)
	} else {
		dstPath = strings.Replace(strings.Replace(path, srcDir, outDir, 1), ".md", ".html", 1)

		if conf.Options.Ordering {
			dstPath = fileNameOrderRegExp.ReplaceAllString(dstPath, "/")
		}
	}

	finalPath := strings.Replace(dstPath, outDir, "", 1)

	file := &DocFile{
		Path: finalPath, OutPath: dstPath,
		InPath:   path,
		Matter:   matter,
		File:     markdownFile,
		Contents: markdownHtml,
		ModTime:  info.ModTime(),
	}

	return file, nil
}
