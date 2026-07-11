package migrations

import (
	"strings"
	"testing"
)

func TestMLBaselineSeedsUtilityClassifiers(t *testing.T) {
	sql, err := FS.ReadFile("000005_ml_analysis_results.up.sql")
	if err != nil {
		t.Fatalf("read ML baseline migration: %v", err)
	}

	migration := string(sql)
	if !strings.Contains(migration, "INSERT INTO public.classifier_definitions") {
		t.Fatal("ML baseline migration does not seed classifier definitions")
	}

	for slug, classifier := range map[string]struct {
		displayName string
		tagName     string
	}{
		"documents":    {displayName: "Documents", tagName: "document"},
		"receipts":     {displayName: "Receipts", tagName: "receipt"},
		"illustration": {displayName: "Illustration", tagName: "illustration"},
	} {
		t.Run(slug, func(t *testing.T) {
			rowPrefix := "(\n    '" + slug + "',\n    '" + classifier.displayName + "',\n    '" + classifier.tagName + "',"
			if !strings.Contains(migration, rowPrefix) {
				t.Fatalf(
					"ML baseline migration does not seed classifier %q with tag %q",
					slug,
					classifier.tagName,
				)
			}
		})
	}
}
