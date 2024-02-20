package assets

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"time"
)

//go:embed templates/* assets/*
var Assets embed.FS

var pageTmpl *template.Template

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
	UpdatedAt   time.Time
	Prev        NavPage
	Next        NavPage
}

func (page *PageTemplateData) FormattedUpdatedAt() string {
	return page.UpdatedAt.Format("Tuesday, 2 January 2006")
}

func (page *PageTemplateData) Execute(file *os.File) error {
	pageTemplateContent, err := ReadTemplate("page.html")
	if err != nil {
		return fmt.Errorf("Could not read template: %s", err.Error())
	}

	if pageTmpl == nil {
		pageTmpl, err = template.New("page").Parse(string(pageTemplateContent))
		if err != nil {
			return fmt.Errorf("Could not parse template: %s", err.Error())
		}
	}

	return pageTmpl.Execute(file, page)
}

func CopyTo(srcDir, destDir string) error {
	// Create the destination directory if it doesn't exist
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// List all files and directories in the embedded FS
	entries, err := Assets.ReadDir(srcDir)
	if err != nil {
		return err
	}

	// Loop through each entry (file or directory)
	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		destPath := filepath.Join(destDir, entry.Name())

		if entry.IsDir() {
			// If it's a directory, recursively copy its contents
			if err := CopyTo(srcPath, destPath); err != nil {
				return err
			}
		} else {
			// If it's a file, copy it to the destination directory
			srcFile, err := Assets.Open(srcPath)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			destFile, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer destFile.Close()

			if _, err := io.Copy(destFile, srcFile); err != nil {
				return err
			}
		}
	}

	return nil
}

func ReadTemplate(name string) ([]byte, error) {
	return Assets.ReadFile(filepath.Join("templates", name))
}
