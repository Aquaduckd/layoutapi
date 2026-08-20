package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

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
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := store.Open(*dir)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	log.Printf("loaded %d layouts from %s", st.Count(), st.Dir())

	srv := api.New(st)
	log.Printf("listening on %s", *addr)
	return http.ListenAndServe(*addr, srv.Handler())
}

func defaultDir() string {
	if env := os.Getenv("LAYOUTAPI_DIR"); env != "" {
		return env
	}
	return filepath.Join("layouts")
}
