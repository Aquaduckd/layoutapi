package api

import (
	"bufio"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"os"
	"strings"
)

type WriteKey struct {
	Name   string
	Secret string
}

type hashedKey struct {
	name string
	hash [32]byte
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
			return nil, fmt.Errorf("duplicate write token name %q", name)
		}
		sum := sha256.Sum256([]byte(secret))
		if other, ok := seenHash[sum]; ok {
			return nil, fmt.Errorf("write tokens %q and %q share the same secret", other, name)
		}
		seenName[name] = true
		seenHash[sum] = name
		out = append(out, hashedKey{name: name, hash: sum})
	}
	return out, nil
}

func ParseTokenList(raw string) []WriteKey {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var keys []WriteKey
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, secret, ok := strings.Cut(part, ":")
		if ok && strings.TrimSpace(name) != "" && strings.TrimSpace(secret) != "" {
			keys = append(keys, WriteKey{Name: strings.TrimSpace(name), Secret: strings.TrimSpace(secret)})
			continue
		}
		keys = append(keys, WriteKey{Secret: part})
	}
	return keys
}

func LoadTokenFile(path string) ([]WriteKey, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var keys []WriteKey
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 1 {
			keys = append(keys, WriteKey{Secret: fields[0]})
			continue
		}
		keys = append(keys, WriteKey{Name: fields[0], Secret: strings.Join(fields[1:], " ")})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
	}
	return keys, nil
}

func (s *Server) authorizeWrite(header string) (string, error) {
	if len(s.keys) == 0 {
		return "", errWritesDisabled
	}
	got := bearerToken(header)
	if got == "" {
		return "", errUnauthorized
	}
	sum := sha256.Sum256([]byte(got))
	matched := ""
	ok := 0
	for _, key := range s.keys {
		match := subtle.ConstantTimeCompare(sum[:], key.hash[:])
		ok |= match
		if match == 1 {
			matched = key.name
		}
	}
	if ok != 1 {
		return "", errUnauthorized
	}
	return matched, nil
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
