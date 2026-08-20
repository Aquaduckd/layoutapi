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
	actor := Actor{User: 42}
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
	if err := st.Delete(id, Actor{User: 99}); !errors.Is(err, ErrForbidden) {
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
	actor := Actor{User: 7}
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
	if _, err := st.Get("hours"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Get("qwerty"); err != nil {
		t.Fatal(err)
	}
}
