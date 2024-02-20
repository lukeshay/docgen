package dev

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/lukeshay/gocden/pkg/cmds"
	"github.com/lukeshay/gocden/pkg/cmds/build"
	"github.com/lukeshay/gocden/pkg/cmds/serve"
	"github.com/urfave/cli/v2"
)

var mu = sync.Mutex{}

func Dev(c *cli.Context) error {
	cwd := cmds.GetCwdFlag(c)
	config := cmds.GetConfigFromCliContext(c)
	files, navSections, err := build.BuildAllFiles(c)
	if err != nil {
		return err
	}

	go func() {
		if err := serve.RunServer(c); err != nil {
			os.Exit(1)
		}
	}()

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("Error creating watcher: %v\n", err)
		return err
	}
	defer watcher.Close()

	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig

		fmt.Println("Shutting down server...")

		os.Exit(1)
	}()

	// Start listening for events.
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				fmt.Printf("event: %v\n", event)

				mu.Lock()
				if event.Has(fsnotify.Create) {
					if newFiles, newNavSections, err := build.BuildAllFiles(c); err != nil {
						fmt.Printf("Error building: %v\n", err)
					} else {
						files = newFiles
						navSections = newNavSections
					}
				} else if event.Has(fsnotify.Write) {
					idx := -1

					for i, file := range *files {
						if file.InPath == event.Name {
							idx = i
							break
						}
					}
					if idx != -1 {
						if err := build.BuildFile(idx, *files, config, *navSections); err != nil {
							fmt.Printf("Error building: %v\n", err)
						}
					}

				}
				mu.Unlock()
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Printf("error: %v\n", err)
			}
		}
	}()

	// Add a path.
	err = watcher.Add(filepath.Join(cwd, config.Build.Source))
	if err != nil {
		fmt.Printf("Error watching source directory: %v\n", err)
		return err
	}

	// Block main goroutine forever.
	<-make(chan struct{})

	return nil
}
