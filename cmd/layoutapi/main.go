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
	token := fs.String("token", os.Getenv("LAYOUTAPI_TOKEN"), "single write token")
	tokens := fs.String("tokens", os.Getenv("LAYOUTAPI_TOKENS"), "comma-separated write tokens, as name:secret or secret")
	tokenFile := fs.String("token-file", envOr("LAYOUTAPI_TOKEN_FILE", "tokens.txt"), "file of write tokens, one `name secret` per line")
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := store.Open(*dir)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	log.Printf("loaded %d layouts from %s", st.Count(), st.Dir())

	keys, err := collectWriteKeys(*token, *tokens, *tokenFile)
	if err != nil {
		return err
	}
	srv, err := api.New(st, keys)
	if err != nil {
		return err
	}
	if srv.WriteKeyCount() == 0 {
		log.Printf("no write tokens configured; GET is public, writes return 503")
	} else {
		log.Printf("write tokens loaded: %d", srv.WriteKeyCount())
	}

	log.Printf("listening on %s", *addr)
	return http.ListenAndServe(*addr, srv.Handler())
}

func collectWriteKeys(token, tokens, tokenFile string) ([]api.WriteKey, error) {
	var keys []api.WriteKey
	if strings.TrimSpace(token) != "" {
		keys = append(keys, api.WriteKey{Name: "default", Secret: strings.TrimSpace(token)})
	}
	keys = append(keys, api.ParseTokenList(tokens)...)
	if strings.TrimSpace(tokenFile) == "" {
		return keys, nil
	}
	fileKeys, err := api.LoadTokenFile(tokenFile)
	if err != nil {
		if os.IsNotExist(err) && tokenFile == "tokens.txt" && os.Getenv("LAYOUTAPI_TOKEN_FILE") == "" {
			return keys, nil
		}
		return nil, fmt.Errorf("token file: %w", err)
	}
	return append(keys, fileKeys...), nil
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
