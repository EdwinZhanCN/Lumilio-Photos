package locales_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

func catalogKeys(t *testing.T, file string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	messages := map[string]string{}
	for key, value := range raw {
		if strings.HasPrefix(key, "@") {
			continue
		}
		var message string
		if err := json.Unmarshal(value, &message); err != nil {
			t.Fatalf("%s: message %q is not a string", file, key)
		}
		messages[key] = message
	}
	return messages
}

// A message nothing says is copy a translator still has to keep in step. The
// catalogs hold exactly what the Settings window and the tray say.
func TestEveryMessageIsSaid(t *testing.T) {
	var source strings.Builder
	for _, dir := range []string{"../panel", "../host"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			source.Write(data)
		}
	}
	said := map[string]bool{}
	for _, match := range regexp.MustCompile(`locales\.([A-Z]\w*)\(`).FindAllStringSubmatch(source.String(), -1) {
		said[match[1]] = true
	}
	if len(said) < 100 {
		t.Fatalf("found only %d messages said; the sources moved", len(said))
	}
	for key := range catalogKeys(t, "en.json") {
		runes := []rune(key)
		runes[0] = unicode.ToUpper(runes[0])
		if !said[string(runes)] {
			t.Errorf("message %q is in the catalogs but nothing says it; remove it from every catalog and regenerate", key)
		}
	}
}

// Every language says every message: a missing one would fall back to
// English in the middle of a translated window.
func TestEveryLanguageHasEveryMessage(t *testing.T) {
	source := catalogKeys(t, "en.json")
	files, err := filepath.Glob("*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 2 {
		t.Fatalf("found %d catalogs, want the source and at least one translation", len(files))
	}
	for _, file := range files {
		if file == "en.json" {
			continue
		}
		translated := catalogKeys(t, file)
		for key := range source {
			if strings.TrimSpace(translated[key]) == "" {
				t.Errorf("%s has no translation of %q", file, key)
			}
		}
		for key := range translated {
			if _, ok := source[key]; !ok {
				t.Errorf("%s translates %q, which the source catalog does not have", file, key)
			}
		}
	}
}
