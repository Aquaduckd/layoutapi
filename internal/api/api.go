package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"layoutapi/internal/store"
)

var (
	errUnauthorized   = errors.New("missing or invalid bearer token")
	errWritesDisabled = errors.New("writes are disabled: no write tokens are configured")
)

type Server struct {
	store *store.Store
	keys  []hashedKey
}

func New(st *store.Store, keys []WriteKey) (*Server, error) {
	hashed, err := hashKeys(keys)
	if err != nil {
		return nil, err
	}
	return &Server{store: st, keys: hashed}, nil
}

func (s *Server) WriteKeyCount() int { return len(s.keys) }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /v1/layouts", s.list)
	mux.HandleFunc("POST /v1/layouts", s.create)
	mux.HandleFunc("GET /v1/layouts/{name}", s.get)
	mux.HandleFunc("PUT /v1/layouts/{name}", s.replace)
	mux.HandleFunc("DELETE /v1/layouts/{name}", s.delete)
	mux.HandleFunc("POST /v1/layouts/{name}/rename", s.rename)
	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"count": s.store.Count(),
	})
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.Filter{
		Query: q.Get("q"),
		Board: q.Get("board"),
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		f.Limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
		f.Offset = n
	}
	if v := q.Get("user"); v != "" {
		u, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid user")
			return
		}
		f.User = &u
	}
	items, total := s.store.List(f)
	full := q.Get("full") == "1" || strings.EqualFold(q.Get("full"), "true")
	if !full {
		writeJSON(w, http.StatusOK, map[string]any{
			"total":   total,
			"layouts": items,
		})
		return
	}
	docs := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		raw, err := s.store.Get(item.ID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		docs = append(docs, json.RawMessage(raw))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total":   total,
		"layouts": docs,
	})
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	raw, err := s.store.Get(r.PathValue("name"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.writeActor(w, r)
	if !ok {
		return
	}
	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id, stored, err := s.store.Create(actor, body)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	w.Header().Set("Location", "/v1/layouts/"+id)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(stored)
}

func (s *Server) replace(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.writeActor(w, r)
	if !ok {
		return
	}
	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	stored, err := s.store.Replace(r.PathValue("name"), actor, body)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(stored)
}

func (s *Server) delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.writeActor(w, r)
	if !ok {
		return
	}
	if err := s.store.Delete(r.PathValue("name"), actor); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) rename(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.writeActor(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	newID, stored, err := s.store.Rename(r.PathValue("name"), req.Name, actor)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	w.Header().Set("Location", "/v1/layouts/"+newID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(stored)
}

func (s *Server) writeActor(w http.ResponseWriter, r *http.Request) (store.Actor, bool) {
	app, err := s.authorizeWrite(r.Header.Get("Authorization"))
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, errWritesDisabled) {
			status = http.StatusServiceUnavailable
		}
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, status, err.Error())
		return store.Actor{}, false
	}
	return store.Actor{App: app}, true
}

func readBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, errors.New("empty body")
	}
	return body, nil
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "layout not found")
	case errors.Is(err, store.ErrExists):
		writeError(w, http.StatusConflict, "layout already exists")
	case errors.Is(err, store.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, store.ErrValidation):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	if status == http.StatusOK || status == http.StatusCreated {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
