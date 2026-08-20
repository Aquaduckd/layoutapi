package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"layoutapi/internal/layout"
	"layoutapi/internal/store"
)

func testServer(t *testing.T, keys ...WriteKey) *httptest.Server {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h, err := New(st, keys)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func sampleBody() []byte {
	raw, _ := layout.Encode(layout.Doc{
		Name:  "http-demo",
		User:  11,
		Board: "ortho",
		Keys:  map[string]layout.Position{"x": {Row: 0, Col: 0, Finger: "LP"}},
	})
	return raw
}

func TestHTTPFlow(t *testing.T) {
	srv := testServer(t, WriteKey{Name: "dmini", Secret: "secret-dmini"})

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/layouts", bytes.NewReader(sampleBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret-dmini")
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

	req, _ = http.NewRequest(http.MethodDelete, srv.URL+"/v1/layouts/http-demo", nil)
	req.Header.Set("Authorization", "Bearer secret-dmini")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete %d", res.StatusCode)
	}
}

func TestReadsArePublicWritesNeedToken(t *testing.T) {
	srv := testServer(t, WriteKey{Name: "dmini", Secret: "secret-dmini"}, WriteKey{Name: "web", Secret: "secret-web"})

	res, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("health %d", res.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/layouts", bytes.NewReader(sampleBody()))
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated write %d", res.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/v1/layouts", bytes.NewReader(sampleBody()))
	req.Header.Set("Authorization", "Bearer secret-web")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("web token write %d %s", res.StatusCode, b)
	}
	res.Body.Close()
}

func TestRevokedTokenRejected(t *testing.T) {
	live := testServer(t, WriteKey{Name: "dmini", Secret: "keep"}, WriteKey{Name: "oldapp", Secret: "revoke-me"})
	revoked := testServer(t, WriteKey{Name: "dmini", Secret: "keep"})

	create := func(base, secret string) int {
		req, _ := http.NewRequest(http.MethodPost, base+"/v1/layouts", bytes.NewReader(sampleBody()))
		req.Header.Set("Authorization", "Bearer "+secret)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}

	if got := create(live.URL, "revoke-me"); got != http.StatusCreated {
		t.Fatalf("live oldapp %d", got)
	}
	if got := create(revoked.URL, "revoke-me"); got != http.StatusUnauthorized {
		t.Fatalf("revoked oldapp %d", got)
	}
}

func TestWritesDisabledWithoutTokens(t *testing.T) {
	srv := testServer(t)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/layouts", bytes.NewReader(sampleBody()))
	req.Header.Set("Authorization", "Bearer anything")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestLoadAppsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apps.json")
	raw := []byte(`{
		"apps": [
			{"name": "dmini", "secret": "secret-dmini", "tag": "cmini"},
			{"name": "cmini", "secret": "secret-cmini", "tag": "cmini", "write": ["cmini"]},
			{"name": "web", "secret": "secret-web"}
		]
	}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	keys, err := LoadAppsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 || keys[0].Tag != "cmini" || keys[2].Name != "web" {
		t.Fatalf("%+v", keys)
	}
	hashed, err := hashKeys(keys)
	if err != nil {
		t.Fatal(err)
	}
	if !hashed[0].write["cmini"] || hashed[2].tag != "web" || !hashed[2].write["web"] {
		t.Fatalf("%+v", hashed)
	}
}

func TestWriteDoesNotNeedDiscordUserHeader(t *testing.T) {
	srv := testServer(t, WriteKey{Name: "dmini", Secret: "secret"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/layouts", bytes.NewReader(sampleBody()))
	req.Header.Set("Authorization", "Bearer secret")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d %s", res.StatusCode, b)
	}
}

func TestFullList(t *testing.T) {
	srv := testServer(t, WriteKey{Name: "dmini", Secret: "secret"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/layouts", bytes.NewReader(sampleBody()))
	req.Header.Set("Authorization", "Bearer secret")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	res, err = http.Get(srv.URL + "/v1/layouts?full=1")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var full struct {
		Layouts []map[string]any `json:"layouts"`
	}
	if err := json.NewDecoder(res.Body).Decode(&full); err != nil {
		t.Fatal(err)
	}
	if len(full.Layouts) != 1 || full.Layouts[0]["keys"] == nil {
		t.Fatalf("%+v", full)
	}
}

func TestSharedTagAndIsolation(t *testing.T) {
	srv := testServer(t,
		WriteKey{Name: "dmini", Secret: "secret-dmini", Tag: "cmini"},
		WriteKey{Name: "cmini", Secret: "secret-cmini", Tag: "cmini"},
		WriteKey{Name: "web", Secret: "secret-web"},
	)

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/layouts", bytes.NewReader(sampleBody()))
	req.Header.Set("Authorization", "Bearer secret-dmini")
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
	var doc map[string]any
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if doc["tag"] != "cmini" || doc["blame"] != "dmini" {
		t.Fatalf("tag/blame %v %v", doc["tag"], doc["blame"])
	}

	req, _ = http.NewRequest(http.MethodDelete, srv.URL+"/v1/layouts/http-demo", nil)
	req.Header.Set("Authorization", "Bearer secret-web")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("web delete %d", res.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodDelete, srv.URL+"/v1/layouts/http-demo", nil)
	req.Header.Set("Authorization", "Bearer secret-cmini")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("cmini delete %d", res.StatusCode)
	}
}
