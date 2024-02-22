package serve

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/lukeshay/gocden/pkg/cmds"
	"github.com/urfave/cli/v2"
)

type route struct {
	pattern *regexp.Regexp
	handler http.Handler
}

type RegexpHandler struct {
	routes []*route
}

func (h *RegexpHandler) Handler(pattern *regexp.Regexp, handler http.Handler) {
	h.routes = append(h.routes, &route{pattern, handler})
}

func (h *RegexpHandler) HandleFunc(pattern *regexp.Regexp, handler func(http.ResponseWriter, *http.Request)) {
	h.routes = append(h.routes, &route{pattern, http.HandlerFunc(handler)})
}

func (h *RegexpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range h.routes {
		if route.pattern.MatchString(r.URL.Path) {
			route.handler.ServeHTTP(w, r)
			return
		}
	}

	http.NotFound(w, r)
}

func RunServer(c *cli.Context) error {
	cwd := cmds.GetCwdFlag(c)
	config := cmds.GetConfigFromCliContext(c)

	basePath := ""
	if uri, err := url.Parse(config.Url); err == nil {
		if strings.HasSuffix(uri.Path, "/") {
			basePath = uri.Path[:len(uri.Path)-1]
		} else {
			basePath = uri.Path
		}
	}

	handler := &RegexpHandler{}

	pathRegExp := regexp.MustCompile(fmt.Sprintf("%s.*", basePath))

	handler.HandleFunc(pathRegExp, func(w http.ResponseWriter, r *http.Request) {
		filePath := strings.Replace(r.URL.Path, basePath, "", 1)

		slog.Info("Serving request", "basePath", basePath, "path", r.URL.Path, "withoutBasePath", filePath)

		if strings.HasSuffix(filePath, "/") || filePath == "" {
			filePath += "index.html"
		} else if !strings.Contains(filePath, ".") {
			filePath += ".html"
		}

		slog.Info("Serving file", "filePath", filePath, "file", filepath.Join(cwd, config.Build.Output, filePath))

		http.ServeFile(w, r, filepath.Join(cwd, config.Build.Output, filePath))
	})

	fmt.Printf("Listening on http://localhost:%d%s ...\n", config.Serve.Port, basePath)

	err := http.ListenAndServe(fmt.Sprintf(":%d", config.Serve.Port), handler)
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
