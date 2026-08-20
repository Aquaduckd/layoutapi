package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"layoutapi/internal/layout"
)

var (
	ErrNotFound   = errors.New("layout not found")
	ErrExists     = errors.New("layout already exists")
	ErrForbidden  = errors.New("not the layout owner")
	ErrValidation = errors.New("invalid layout")
)

type Actor struct {
	Name  string
	Tag   string
	Write map[string]bool
}

type Filter struct {
	Query  string
	Board  string
	User   *int64
	Limit  int
	Offset int
}

type record struct {
	summary layout.Summary
	body    []byte
	user    int64
	tag     string
}

type Store struct {
	dir  string
	mu   sync.RWMutex
	byID map[string]record
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, byID: map[string]record{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(ent.Name(), ".json")
		raw, err := os.ReadFile(filepath.Join(dir, ent.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", ent.Name(), err)
		}
		doc, err := layout.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", ent.Name(), err)
		}
		if strings.TrimSpace(doc.Name) == "" {
			doc.Name = id
		}
		s.byID[id] = record{
			summary: doc.Summary(id),
			body:    raw,
			user:    doc.User,
			tag:     doc.CatalogTag(),
		}
	}
	return s, nil
}

func (s *Store) Dir() string { return s.dir }

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byID)
}

func (s *Store) Get(id string) ([]byte, error) {
	id = layout.IDFromName(id)
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	out := make([]byte, len(rec.body))
	copy(out, rec.body)
	return out, nil
}

func (s *Store) List(f Filter) ([]layout.Summary, int) {
	q := strings.ToLower(strings.TrimSpace(f.Query))
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.byID))
	for id, rec := range s.byID {
		if f.Board != "" && rec.summary.Board != f.Board {
			continue
		}
		if f.User != nil && rec.user != *f.User {
			continue
		}
		if q != "" && !strings.Contains(id, q) && !strings.Contains(strings.ToLower(rec.summary.Name), q) {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	total := len(ids)
	if f.Offset < 0 {
		f.Offset = 0
	}
	if f.Offset > total {
		f.Offset = total
	}
	ids = ids[f.Offset:]
	if f.Limit > 0 && f.Limit < len(ids) {
		ids = ids[:f.Limit]
	}
	out := make([]layout.Summary, 0, len(ids))
	for _, id := range ids {
		out = append(out, s.byID[id].summary)
	}
	return out, total
}

func (s *Store) Create(actor Actor, raw []byte) (string, []byte, error) {
	doc, err := parseAndValidate(raw, "", true)
	if err != nil {
		return "", nil, err
	}
	if err := requireApp(actor); err != nil {
		return "", nil, err
	}
	id := layout.IDFromName(doc.Name)
	if err := checkWrite(actor, actor.Tag); err != nil {
		return "", nil, err
	}
	doc.Tag = actor.Tag
	doc.Blame = actor.Name
	doc.Source = ""
	body, err := encodeDoc(doc)
	if err != nil {
		return "", nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byID[id]; exists {
		return "", nil, ErrExists
	}
	if err := s.writeLocked(id, doc, body); err != nil {
		return "", nil, err
	}
	return id, body, nil
}

func (s *Store) Replace(id string, actor Actor, raw []byte) ([]byte, error) {
	id = layout.IDFromName(id)
	doc, err := parseAndValidate(raw, id, true)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	if err := checkWrite(actor, rec.tag); err != nil {
		return nil, err
	}
	doc.Tag = rec.tag
	doc.Blame = actor.Name
	doc.Source = ""
	body, err := encodeDoc(doc)
	if err != nil {
		return nil, err
	}
	if err := s.writeLocked(id, doc, body); err != nil {
		return nil, err
	}
	return body, nil
}

func (s *Store) Delete(id string, actor Actor) error {
	id = layout.IDFromName(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.byID[id]
	if !ok {
		return ErrNotFound
	}
	if err := checkWrite(actor, rec.tag); err != nil {
		return err
	}
	if err := os.Remove(s.path(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	delete(s.byID, id)
	return nil
}

func (s *Store) Rename(id, newName string, actor Actor) (string, []byte, error) {
	id = layout.IDFromName(id)
	if err := layout.ValidateName(newName); err != nil {
		return "", nil, fmt.Errorf("%w: %s", ErrValidation, err)
	}
	newID := layout.IDFromName(newName)
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.byID[id]
	if !ok {
		return "", nil, ErrNotFound
	}
	if err := checkWrite(actor, rec.tag); err != nil {
		return "", nil, err
	}
	if newID != id {
		if _, exists := s.byID[newID]; exists {
			return "", nil, ErrExists
		}
	}
	doc, err := layout.Parse(rec.body)
	if err != nil {
		return "", nil, err
	}
	doc.Name = newName
	doc.Tag = rec.tag
	doc.Blame = actor.Name
	doc.Source = ""
	body, err := encodeDoc(doc)
	if err != nil {
		return "", nil, err
	}
	if err := s.writeLocked(newID, doc, body); err != nil {
		return "", nil, err
	}
	if newID != id {
		if err := os.Remove(s.path(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", nil, err
		}
		delete(s.byID, id)
	}
	return newID, body, nil
}

func parseAndValidate(raw []byte, id string, strictName bool) (layout.Doc, error) {
	doc, err := layout.Parse(raw)
	if err != nil {
		return layout.Doc{}, fmt.Errorf("%w: %s", ErrValidation, err)
	}
	if id == "" {
		id = layout.IDFromName(doc.Name)
	}
	if err := doc.Validate(id, strictName); err != nil {
		return layout.Doc{}, fmt.Errorf("%w: %s", ErrValidation, err)
	}
	return doc, nil
}

func encodeDoc(doc layout.Doc) ([]byte, error) {
	body, err := layout.Encode(doc)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (s *Store) writeLocked(id string, doc layout.Doc, body []byte) error {
	path := s.path(id)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	s.byID[id] = record{
		summary: doc.Summary(id),
		body:    body,
		user:    doc.User,
		tag:     doc.CatalogTag(),
	}
	return nil
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func requireApp(actor Actor) error {
	if strings.TrimSpace(actor.Name) == "" || strings.TrimSpace(actor.Tag) == "" {
		return fmt.Errorf("%w: missing app", ErrForbidden)
	}
	return nil
}

func checkWrite(actor Actor, tag string) error {
	if err := requireApp(actor); err != nil {
		return err
	}
	tag = layout.NormalizeTag(tag)
	if actor.Write[tag] {
		return nil
	}
	return fmt.Errorf("%w: layout belongs to %s", ErrForbidden, tag)
}
