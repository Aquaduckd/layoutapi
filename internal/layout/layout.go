package layout

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	minNameLen = 3
	DefaultTag = "cmini"
)

var allowedBoards = map[string]bool{
	"ortho":   true,
	"angle":   true,
	"stagger": true,
	"mini":    true,
}

var allowedFingers = map[string]bool{
	"LP": true, "LR": true, "LM": true, "LI": true,
	"RI": true, "RM": true, "RR": true, "RP": true,
	"LT": true, "RT": true, "TB": true,
}

// Doc is the cmini layout JSON shape. Optional "free" is preserved on encode.
// Tag is the catalog that may write this layout. Blame is the app that last wrote it.
// Source is accepted only when reading old files.
type Doc struct {
	Name   string              `json:"name"`
	User   int64               `json:"user"`
	Board  string              `json:"board"`
	Tag    string              `json:"tag,omitempty"`
	Blame  string              `json:"blame,omitempty"`
	Keys   map[string]Position `json:"keys"`
	Free   []any               `json:"free,omitempty"`
	Source string              `json:"source,omitempty"`
}

type Position struct {
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Finger string `json:"finger"`
}

type Summary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	User     int64  `json:"user"`
	Board    string `json:"board"`
	Tag      string `json:"tag"`
	Blame    string `json:"blame,omitempty"`
	KeyCount int    `json:"key_count"`
}

func IDFromName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func Parse(raw []byte) (Doc, error) {
	var doc Doc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Doc{}, fmt.Errorf("invalid layout json: %w", err)
	}
	return doc, nil
}

func NormalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return DefaultTag
	}
	return tag
}

func (d Doc) CatalogTag() string {
	if tag := strings.TrimSpace(d.Tag); tag != "" {
		return tag
	}
	if tag := strings.TrimSpace(d.Source); tag != "" {
		return tag
	}
	return DefaultTag
}

func (d Doc) Summary(id string) Summary {
	return Summary{
		ID:       id,
		Name:     d.Name,
		User:     d.User,
		Board:    d.Board,
		Tag:      d.CatalogTag(),
		Blame:    strings.TrimSpace(d.Blame),
		KeyCount: len(d.Keys),
	}
}

func (d Doc) Validate(id string, strictName bool) error {
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strictName {
		if err := ValidateName(d.Name); err != nil {
			return err
		}
		if id != "" && IDFromName(d.Name) != id {
			return fmt.Errorf("layout name %q does not match id %q", d.Name, id)
		}
	}
	if d.User < 0 {
		return fmt.Errorf("user must be a Discord id")
	}
	if !allowedBoards[d.Board] {
		return fmt.Errorf("unknown board %q", d.Board)
	}
	if len(d.Keys) == 0 {
		return fmt.Errorf("layout has no keys")
	}
	for ch, pos := range d.Keys {
		if utf8.RuneCountInString(ch) != 1 {
			return fmt.Errorf("key %q must be a single character", ch)
		}
		if !allowedFingers[pos.Finger] {
			return fmt.Errorf("unknown finger %q on key %q", pos.Finger, ch)
		}
		if pos.Row < 0 || pos.Col < 0 {
			return fmt.Errorf("negative position on key %q", ch)
		}
	}
	return nil
}

func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if strings.HasPrefix(name, "_") {
		return fmt.Errorf("names cannot start with an underscore")
	}
	if utf8.RuneCountInString(name) < minNameLen {
		return fmt.Errorf("names must be at least %d characters long", minNameLen)
	}
	for _, r := range name {
		if !nameRuneOK(r) {
			return fmt.Errorf("names cannot contain %q", string(r))
		}
	}
	return nil
}

func nameRuneOK(r rune) bool {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}
	switch r {
	case ' ', '_', '-', '\'', '(', ')', ':', '~':
		return true
	}
	return false
}

func Encode(doc Doc) ([]byte, error) {
	doc.Tag = doc.CatalogTag()
	doc.Source = ""
	out, err := json.MarshalIndent(doc, "", "    ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
