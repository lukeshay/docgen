package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/adrg/frontmatter"
	"github.com/go-playground/validator/v10"
	cli "github.com/urfave/cli/v2"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"

	"github.com/briandowns/spinner"
	"github.com/lukeshay/docgen/pkg/assets"
	cp "github.com/otiai10/copy"
)

var version = "dev"

type Social struct {
	Twitter   string `json:"twitter"`
	Facebook  string `json:"facebook"`
	Instagram string `json:"instagram"`
	LinkedIn  string `json:"linkedin"`
	GitHub    string `json:"github"`
	GitLab    string `json:"gitlab"`
	Bitbucket string `json:"bitbucket"`
}

type Config struct {
	Name        string  `json:"name" validate:"required"`
	Description string  `json:"description"`
	Url         string  `json:"url"`
	Social      *Social `json:"social"`
	Source      string  `json:"src" validate:"required"`
	Output      string  `json:"out" validate:"required"`
	Assets      string  `json:"assets"`
}

type Frontmatter struct {
	Title       string `yaml:"title" validate:"required"`
	Description string `yaml:"description" `
	Path        string `yaml:"path"`
	Section     string `yaml:"section"`
}

type NavPage struct {
	Title string
	Href  string
}

type NavSection struct {
	Title string
	Pages []NavPage
}

type PageTemplateData struct {
	Markdown    template.HTML
	Name        string
	Title       string
	Description string
	Url         string
	Twitter     string
	Path        string
	NavSections []*NavSection
}

const CONFIG_PATH = "docgen.json"

var validate *validator.Validate

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
)

func getConfig(c *cli.Context) Config {
	return c.Context.Value(CONFIG_PATH).(Config)
}

var logFile = (func() *os.File {
	logFile, err := os.CreateTemp("", "docgen-*.log")
	if err != nil {
		panic(err)
	}
	return logFile
})()

func configureLog(cli *cli.Context) {
	writers := []io.Writer{logFile}

	if cli.Bool("verbose") {
		writers = append(writers, os.Stderr)
	}
	writer := io.MultiWriter(writers...)
	slog.SetDefault(
		slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	)
}

func validateAndLog(message string, val interface{}) error {
	if err := validate.Struct(val); err != nil {
		fmt.Println(message)

		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)

			return err
		}

		for _, err := range err.(validator.ValidationErrors) {
			fmt.Println(err.Namespace())
			fmt.Println(err.Field())
			fmt.Println(err.StructNamespace())
			fmt.Println(err.StructField())
			fmt.Println(err.Tag())
			fmt.Println(err.ActualTag())
			fmt.Println(err.Kind())
			fmt.Println(err.Type())
			fmt.Println(err.Value())
			fmt.Println(err.Param())
			fmt.Println()
		}

		return err
	}

	return nil
}

type File struct {
	Path     string
	OutPath  string
	InPath   string
	Matter   Frontmatter
	File     *os.File
	Contents string
}

func main() {
	interruptChannel := make(chan os.Signal, 1)
	signal.Notify(interruptChannel, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-interruptChannel
		cancel()
	}()

	validate = validator.New(validator.WithRequiredStructEnabled())

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Could not get current directory")

		os.Exit(1)
	}

	app := &cli.App{
		Name:        "docgen",
		Description: "generate simple documentation",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name: "verbose",
			},
		},
		Before: func(c *cli.Context) error {
			configureLog(c)

			file, err := os.ReadFile(CONFIG_PATH)

			var config Config

			if err != nil {
				config = Config{
					Name:        filepath.Base(cwd),
					Description: "A new docgen site",
					Url:         "",
					Social: &Social{
						GitHub:    "",
						Twitter:   "",
						Facebook:  "",
						Instagram: "",
						LinkedIn:  "",
						GitLab:    "",
						Bitbucket: "",
					},
				}

				jsonConfig, err := json.MarshalIndent(config, "", "    ")
				if err != nil {
					return err
				}

				if err = os.WriteFile(CONFIG_PATH, jsonConfig, 0644); err != nil {
					return err
				}
			} else {
				if err := json.Unmarshal(file, &config); err != nil {
					fmt.Println("Your config is invalid")
					return err
				}
			}

			if err := validateAndLog("Your config is invalid", &config); err != nil {
				return err
			}

			c.Context = context.WithValue(c.Context, CONFIG_PATH, config)

			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "version",
				Flags: []cli.Flag{},
				Action: func(cli *cli.Context) error {
					fmt.Printf("docgen.%s\n", version)
					return nil
				},
			},
			{
				Name:  "build",
				Flags: []cli.Flag{},
				Action: func(c *cli.Context) error {
					config := getConfig(c)

					mdRegexp, err := regexp.Compile("^.+\\.(md)$")
					if err != nil {
						return err
					}

					srcDir := filepath.Join(cwd, config.Source)
					outDir := filepath.Join(cwd, config.Output)

					pageTemplateContent, err := assets.ReadTemplate("page.html")
					if err != nil {
						fmt.Println("Could not read template", err.Error())
						return err
					}

					pageTmpl, err := template.New("page").Parse(string(pageTemplateContent))
					if err != nil {
						fmt.Println("Could not parse template", err.Error())
						return err
					}

					fmt.Println("Build docs in ", config.Source, " to ", config.Output)

					spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
					spin.Suffix = " Building docs..."
					spin.Start()

					os.RemoveAll(outDir)
					os.MkdirAll(outDir, os.ModePerm)

					if err := cp.Copy(filepath.Join(srcDir, "assets"), filepath.Join(outDir, "assets")); err != nil {
						fmt.Println("Could not copy assets", err.Error())

						return err
					}

					if err := assets.CopyTo("assets", outDir); err != nil {
						fmt.Println("Could not copy assets")
						return err
					}

					files := []File{}
					navSections := []*NavSection{
						{
							Title: "",
							Pages: []NavPage{},
						},
					}

					if err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
						slog.Info("Processing file in src directory", "src", srcDir, "path", path)

						if err != nil || info.IsDir() || !mdRegexp.MatchString(info.Name()) {
							return err
						}

						slog.Info("Adding file to list", "path", path)

						markdownFile, err := os.Open(path)
						if err != nil {
							fmt.Println("Could not open file", err.Error())
							return err
						}

						var matter Frontmatter

						pageMarkdown, err := frontmatter.MustParse(markdownFile, &matter)
						if err != nil {
							fmt.Println("Could not parse frontmatter")
							return err
						}

						if err := validateAndLog(fmt.Sprintf("The frontmatter is invalid in %s", info.Name()), &matter); err != nil {
							return err
						}

						var markdownHtmlBuf bytes.Buffer

						if err := md.Convert(pageMarkdown, &markdownHtmlBuf, parser.WithContext(parser.NewContext())); err != nil {
							fmt.Println("Could not convert markdown to html")
							return err
						}

						markdownHtml := markdownHtmlBuf.String()

						var dstPath string

						if matter.Path != "" {
							dstPath = filepath.Join(outDir, matter.Path)
						} else {
							dstPath = strings.Replace(strings.Replace(path, srcDir, outDir, 1), ".md", ".html", 1)
						}

						finalPath := strings.Replace(dstPath, outDir, "", 1)

						file := File{
							Path: finalPath, OutPath: dstPath,
							InPath:   path,
							Matter:   matter,
							File:     markdownFile,
							Contents: markdownHtml,
						}

						found := false

						for _, section := range navSections {
							if section.Title == matter.Section {
								slog.Info("Adding page to section", "section", section.Title, "page", matter.Title, "href", finalPath)

								section.Pages = append(section.Pages, NavPage{
									Title: matter.Title,
									Href:  finalPath,
								})

								found = true
								break
							}
						}

						if !found {
							slog.Info("Creating new section for page", "section", matter.Section, "page", matter.Title, "href", finalPath)

							navSections = append(navSections, &NavSection{
								Title: matter.Section,
								Pages: []NavPage{
									{
										Title: matter.Title,
										Href:  finalPath,
									},
								},
							})
						}

						files = append(files, file)

						return nil
					}); err != nil {
						fmt.Println("Could not walk src directory", err.Error())
					}

					slog.Info("Writing html files", "sections", navSections)

					for _, file := range files {
						defer file.File.Close()

						_ = os.MkdirAll(filepath.Dir(file.OutPath), os.ModePerm)

						dstHtmlFile, err := os.Create(file.OutPath)
						if err != nil {
							fmt.Println("Could not create file", file.OutPath, err.Error())

							return err
						}
						defer dstHtmlFile.Close()

						slog.Info("Writing HTML file", "source", file.InPath, "destinition", file.OutPath)

						if err := pageTmpl.Execute(dstHtmlFile, PageTemplateData{
							Markdown:    template.HTML(file.Contents),
							Name:        config.Name,
							Title:       file.Matter.Title,
							Description: config.Description,
							Url:         config.Url,
							Path:        file.Path,
							Twitter:     config.Social.Twitter,
							NavSections: navSections,
						}); err != nil {
							fmt.Println("Could not execute template", err.Error())
							return err
						}
					}

					spin.Stop()

					fmt.Println("Docs built successfully")

					return nil
				},
			},
		},
	}

	if err := app.RunContext(ctx, os.Args); err != nil {
		panic(err)
	}
}
