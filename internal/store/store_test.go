package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"layoutapi/internal/layout"
)

func sample(name string, user int64) []byte {
	doc := layout.Doc{
		Name:  name,
		User:  user,
		Board: "ortho",
		Keys: map[string]layout.Position{
			"a": {Row: 0, Col: 0, Finger: "LP"},
			"b": {Row: 0, Col: 1, Finger: "LR"},
		},
	}
	raw, err := layout.Encode(doc)
	if err != nil {
		panic(err)
	}
	return raw
}

func TestCreateGetDelete(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{User: 42, App: "dmini"}
	id, _, err := st.Create(actor, sample("demo-layout", 42))
	if err != nil {
		t.Fatal(err)
	}
	if id != "demo-layout" {
		t.Fatalf("id %q", id)
	}
	raw, err := st.Get("Demo-Layout")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := layout.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Name != "demo-layout" {
		t.Fatalf("name %q", doc.Name)
	}
	if doc.Source != "dmini" {
		t.Fatalf("source %q", doc.Source)
	}
	if err := st.Delete(id, Actor{User: 99, App: "dmini"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if err := st.Delete(id, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Get(id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestRenameAndList(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{User: 7, App: "dmini"}
	if _, _, err := st.Create(actor, sample("alpha-one", 7)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.Create(actor, sample("beta-two", 7)); err != nil {
		t.Fatal(err)
	}
	newID, _, err := st.Rename("alpha-one", "gamma-three", actor)
	if err != nil {
		t.Fatal(err)
	}
	if newID != "gamma-three" {
		t.Fatalf("new id %q", newID)
	}
	if _, err := os.Stat(filepath.Join(dir, "alpha-one.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("old file should be gone")
	}
	items, total := st.List(Filter{Query: "gamma"})
	if total != 1 || len(items) != 1 || items[0].ID != "gamma-three" {
		t.Fatalf("list: total=%d items=%v", total, items)
	}
}

func TestOpenExistingFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hours.json"), sample("hours", 1), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Count() != 1 {
		t.Fatalf("count %d", st.Count())
	}
	raw, err := st.Get("hours")
	if err != nil {
		t.Fatal(err)
	}
	var probe map[string]any
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if probe["name"] != "hours" {
		t.Fatalf("got %v", probe["name"])
	}
}

func TestAppIsolation(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	dmini := Actor{User: 42, App: "dmini"}
	web := Actor{User: 42, App: "web"}
	webAdmin := Actor{User: 1, App: "web", Admin: true}

	id, raw, err := st.Create(dmini, sample("app-demo", 42))
	if err != nil {
		t.Fatal(err)
	}
	spoof := layout.Doc{
		Name:   "spoof-src",
		User:   42,
		Board:  "ortho",
		Source: "web",
		Keys:   map[string]layout.Position{"a": {Row: 0, Col: 0, Finger: "LP"}, "b": {Row: 0, Col: 1, Finger: "LR"}},
	}
	spoofRaw, err := layout.Encode(spoof)
	if err != nil {
		t.Fatal(err)
	}
	_, stamped, err := st.Create(dmini, spoofRaw)
	if err != nil {
		t.Fatal(err)
	}
	spoofDoc, err := layout.Parse(stamped)
	if err != nil {
		t.Fatal(err)
	}
	if spoofDoc.Source != "dmini" {
		t.Fatalf("client source ignored, got %q", spoofDoc.Source)
	}
	_ = st.Delete("spoof-src", dmini)
	doc, err := layout.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Source != "dmini" {
		t.Fatalf("source %q", doc.Source)
	}

	if _, err := st.Replace(id, web, sample("app-demo", 42)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("web replace: %v", err)
	}
	if err := st.Delete(id, webAdmin); !errors.Is(err, ErrForbidden) {
		t.Fatalf("web admin delete: %v", err)
	}
	if _, _, err := st.Rename(id, "app-renamed", web); !errors.Is(err, ErrForbidden) {
		t.Fatalf("web rename: %v", err)
	}

	if _, err := st.Replace(id, dmini, sample("app-demo", 42)); err != nil {
		t.Fatal(err)
	}
	if err := st.Delete(id, Actor{User: 99, App: "dmini", Admin: true}); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyLayoutsBelongToDmini(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "legacy.json"), sample("legacy", 5), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Delete("legacy", Actor{User: 5, App: "web"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("web delete legacy: %v", err)
	}
	if err := st.Delete("legacy", Actor{User: 5, App: "dmini"}); err != nil {
		t.Fatal(err)
	}
}

func TestOpenCopiedCatalog(t *testing.T) {
	dir := filepath.Join("..", "..", "layouts")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("layouts snapshot not present")
	}
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Count() < 4000 {
		t.Fatalf("expected copied cmini catalog, got %d layouts", st.Count())
	}
	raw, err := st.Get("hours")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := layout.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Source != "dmini" {
		t.Fatalf("hours source %q", doc.Source)
	}
	if _, err := st.Get("qwerty"); err != nil {
		t.Fatal(err)
	}
}
