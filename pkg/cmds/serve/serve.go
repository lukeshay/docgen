package serve

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/lukeshay/gocden/pkg/cmds"
	"github.com/urfave/cli/v2"
)

func RunServer(c *cli.Context) error {
	cwd := cmds.GetCwdFlag(c)
	config := cmds.GetConfigFromCliContext(c)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasSuffix(p, "/") {
			p += "index.html"
		} else if !strings.Contains(p, ".") {
			p += ".html"
		}

		http.ServeFile(w, r, filepath.Join(cwd, config.Build.Output, p))
	})

	fmt.Printf("Listening on :%d...\n", config.Serve.Port)

	err := http.ListenAndServe(fmt.Sprintf(":%d", config.Serve.Port), nil)
	if err != nil {
		fmt.Println("Error starting server: %v", err)
		return fmt.Errorf("Error starting server: %v", err)
	}

	return nil
}

func Serve(c *cli.Context) error {
	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig

		fmt.Println("Shutting down server...")

		os.Exit(1)
	}()

	return RunServer(c)
}
