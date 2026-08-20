package layout

import "testing"

func TestNormalizeSource(t *testing.T) {
	if got := NormalizeSource(""); got != DefaultSource {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeSource("web"); got != "web" {
		t.Fatalf("got %q", got)
	}
}

func TestIDFromName(t *testing.T) {
	if got := IDFromName("QWERTY"); got != "qwerty" {
		t.Fatalf("got %q", got)
	}
}

func TestValidateName(t *testing.T) {
	if err := ValidateName("ab"); err == nil {
		t.Fatal("expected short name to fail")
	}
	if err := ValidateName("_nope"); err == nil {
		t.Fatal("expected leading underscore to fail")
	}
	if err := ValidateName("ok-name"); err != nil {
		t.Fatal(err)
	}
}

func TestParseAndValidate(t *testing.T) {
	raw := []byte(`{
		"name": "QWERTY",
		"user": 1,
		"board": "stagger",
		"keys": {
			"q": {"row": 0, "col": 0, "finger": "LP"}
		}
	}`)
	doc, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate("qwerty", true); err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate("colemak", true); err == nil {
		t.Fatal("expected id mismatch")
	}
}
