package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"layoutapi/internal/layout"
	"layoutapi/internal/store"
)

func TestHTTPFlow(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(st).Handler())
	t.Cleanup(srv.Close)

	body, _ := layout.Encode(layout.Doc{
		Name:  "http-demo",
		User:  11,
		Board: "ortho",
		Keys:  map[string]layout.Position{"x": {Row: 0, Col: 0, Finger: "LP"}},
	})

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/layouts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", "11")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("create %d %s", res.StatusCode, b)
	}
	res.Body.Close()

	res, err = http.Get(srv.URL + "/v1/layouts/http-demo")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("get %d", res.StatusCode)
	}
	got, _ := io.ReadAll(res.Body)
	doc, err := layout.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Name != "http-demo" {
		t.Fatalf("name %q", doc.Name)
	}

	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/v1/layouts?q=http", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var list struct {
		Total   int              `json:"total"`
		Layouts []layout.Summary `json:"layouts"`
	}
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 {
		t.Fatalf("total %d", list.Total)
	}

	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/v1/layouts?q=http&full=1", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var full struct {
		Total   int              `json:"total"`
		Layouts []map[string]any `json:"layouts"`
	}
	if err := json.NewDecoder(res.Body).Decode(&full); err != nil {
		t.Fatal(err)
	}
	if full.Total != 1 || full.Layouts[0]["name"] != "http-demo" {
		t.Fatalf("full list: %+v", full)
	}
	if _, ok := full.Layouts[0]["keys"]; !ok {
		t.Fatal("full list should include keys")
	}

	req, _ = http.NewRequest(http.MethodDelete, srv.URL+"/v1/layouts/http-demo", nil)
	req.Header.Set("X-User-Id", "11")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete %d", res.StatusCode)
	}
}

func TestMissingUserHeader(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(st).Handler())
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/layouts", bytes.NewReader([]byte(`{}`)))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", res.StatusCode)
	}
}
