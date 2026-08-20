package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"layoutapi/internal/store"
)

type WriteKey struct {
	Name   string   `json:"name"`
	Secret string   `json:"secret"`
	Tag    string   `json:"tag,omitempty"`
	Write  []string `json:"write,omitempty"`
}

type AppsFile struct {
	Apps []WriteKey `json:"apps"`
}

type hashedKey struct {
	name  string
	tag   string
	hash  [32]byte
	write map[string]bool
}

func hashKeys(keys []WriteKey) ([]hashedKey, error) {
	out := make([]hashedKey, 0, len(keys))
	seenName := map[string]bool{}
	seenHash := map[[32]byte]string{}
	for i, key := range keys {
		name := strings.TrimSpace(key.Name)
		secret := strings.TrimSpace(key.Secret)
		if secret == "" {
			return nil, fmt.Errorf("write token %q is empty", name)
		}
		if name == "" {
			name = fmt.Sprintf("token-%d", i+1)
		}
		if seenName[name] {
			return nil, fmt.Errorf("duplicate app name %q", name)
		}
		tag := strings.TrimSpace(key.Tag)
		if tag == "" {
			tag = name
		}
		write := map[string]bool{tag: true}
		for _, item := range key.Write {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			write[item] = true
		}
		sum := sha256.Sum256([]byte(secret))
		if other, ok := seenHash[sum]; ok {
			return nil, fmt.Errorf("apps %q and %q share the same secret", other, name)
		}
		seenName[name] = true
		seenHash[sum] = name
		out = append(out, hashedKey{name: name, tag: tag, hash: sum, write: write})
	}
	return out, nil
}

func LoadAppsFile(path string) ([]WriteKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file AppsFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(file.Apps) == 0 {
		return nil, fmt.Errorf("%s: no apps defined", path)
	}
	return file.Apps, nil
}

func (s *Server) authorizeWrite(header string) (store.Actor, error) {
	if len(s.keys) == 0 {
		return store.Actor{}, errWritesDisabled
	}
	got := bearerToken(header)
	if got == "" {
		return store.Actor{}, errUnauthorized
	}
	sum := sha256.Sum256([]byte(got))
	var actor store.Actor
	ok := 0
	for _, key := range s.keys {
		match := subtle.ConstantTimeCompare(sum[:], key.hash[:])
		ok |= match
		if match == 1 {
			actor = store.Actor{Name: key.name, Tag: key.tag, Write: key.write}
		}
	}
	if ok != 1 {
		return store.Actor{}, errUnauthorized
	}
	return actor, nil
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	typ, rest, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(typ, "Bearer") {
		return ""
	}
	return strings.TrimSpace(rest)
}
