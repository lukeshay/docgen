package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"

	cli "github.com/urfave/cli/v2"

	"github.com/lukeshay/gocden/pkg/cmds/build"
	"github.com/lukeshay/gocden/pkg/cmds/dev"
	"github.com/lukeshay/gocden/pkg/cmds/serve"
	"github.com/lukeshay/gocden/pkg/config"
)

var (
	version = "dev"
	logFile = (func() *os.File {
		logFile, err := os.CreateTemp("", "gocden-*.log")
		if err != nil {
			panic(err)
		}
		return logFile
	})()
)

func getConfigFromCliContext(c *cli.Context) config.Config {
	return c.Context.Value(config.ConfigPath).(config.Config)
}

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

func main() {
	interruptChannel := make(chan os.Signal, 1)
	signal.Notify(interruptChannel, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-interruptChannel
		cancel()
	}()

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Could not get current directory")

		os.Exit(1)
	}

	app := &cli.App{
		Name:        "gocden",
		Description: "generate simple documentation",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name: "verbose",
			},
			&cli.StringFlag{
				Name:  "cwd",
				Value: cwd,
			},
		},
		Before: func(c *cli.Context) error {
			configureLog(c)

			conf, err := config.ReadAndValidateOrCreate(cwd)
			if err != nil {
				fmt.Println("Could not read or validate config", err.Error())
				return err
			}

			c.Context = context.WithValue(c.Context, config.ConfigPath, conf)

			return nil
		},
		Commands: []*cli.Command{
			{
				Name:        "version",
				Description: "Prints the version",
				Action: func(cli *cli.Context) error {
					fmt.Printf("gocden.%s\n", version)
					return nil
				},
			},
			{
				Name:        "init",
				Description: "Creates a configuration file in the current directory if one does not exist",
				Action: func(c *cli.Context) error {
					fmt.Printf("A configuration has been generated.\n\nYou are now ready to build your documentation! Get started by creating a markdown file in `./docs/`.\n")
					return nil
				},
			},
			{
				Name:        "build",
				Description: "Builds the documentation using the configuration",
				Action:      build.Build,
			},
			{
				Name:        "serve",
				Description: "Serves the built documentation",
				Action:      serve.Serve,
			},
			{
				Name:        "dev",
				Description: "Starts a development server and watches for changes",
				Action:      dev.Dev,
			},
		},
	}

	if err := app.RunContext(ctx, os.Args); err != nil {
		panic(err)
	}
}
