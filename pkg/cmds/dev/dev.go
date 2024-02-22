package dev

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/lukeshay/gocden/pkg/cmds"
	"github.com/lukeshay/gocden/pkg/cmds/build"
	"github.com/lukeshay/gocden/pkg/cmds/serve"
	"github.com/lukeshay/gocden/pkg/util"
	"github.com/urfave/cli/v2"
)

func Dev(c *cli.Context) error {
	cwd := cmds.GetCwdFlag(c)
	config := cmds.GetConfigFromCliContext(c)

	if _, _, err := build.BuildAllFiles(c); err != nil {
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

	debounce := util.NewDebouncer(250 * time.Millisecond)

	// Start listening for events.
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				slog.Info("File system even detected", "even", event)

				if event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Write) {
					debounce(func() {
						if _, _, err := build.BuildAllFiles(c); err != nil {
							fmt.Printf("Error building: %v\n", err)
						}
					})
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}

				fmt.Printf("Error while watching file system: %v\n", err)
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
