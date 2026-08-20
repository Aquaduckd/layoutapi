package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"layoutapi/internal/api"
	"layoutapi/internal/store"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("layoutapi", flag.ContinueOnError)
	addr := fs.String("addr", ":8080", "listen address")
	dir := fs.String("dir", defaultDir(), "directory of layout JSON files")
	apps := fs.String("apps", envOr("LAYOUTAPI_APPS", "apps.json"), "JSON file of apps, secrets, tags, and write access")
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := store.Open(*dir)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	log.Printf("loaded %d layouts from %s", st.Count(), st.Dir())

	keys, err := loadApps(*apps)
	if err != nil {
		return err
	}
	srv, err := api.New(st, keys)
	if err != nil {
		return err
	}
	if srv.WriteKeyCount() == 0 {
		log.Printf("no apps configured; GET is public, writes return 503")
	} else {
		log.Printf("apps loaded: %d", srv.WriteKeyCount())
	}

	log.Printf("listening on %s", *addr)
	return http.ListenAndServe(*addr, srv.Handler())
}

func loadApps(path string) ([]api.WriteKey, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	keys, err := api.LoadAppsFile(path)
	if err != nil {
		if os.IsNotExist(err) && path == "apps.json" && os.Getenv("LAYOUTAPI_APPS") == "" {
			return nil, nil
		}
		return nil, fmt.Errorf("apps file: %w", err)
	}
	return keys, nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func defaultDir() string {
	if env := os.Getenv("LAYOUTAPI_DIR"); env != "" {
		return env
	}
	return filepath.Join("layouts")
}
