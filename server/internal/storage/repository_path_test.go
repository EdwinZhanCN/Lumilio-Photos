package storage

import (
	"errors"
	"testing"

	"server/internal/storage/layout"
)

func TestParseUserMediaPath(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"Trips/2026/photo.jpg",
		"inbox/2026/08/upload.heic",
		".hidden/photo.jpg",
		"photo.jpg",
	} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			got, err := layout.ParseMedia(value)
			if err != nil {
				t.Fatalf("layout.ParseMedia(%q): %v", value, err)
			}
			if got.String() != value {
				t.Fatalf("path = %q, want %q", got.String(), value)
			}
		})
	}
}

func TestParseUserMediaPathRejectsNonCanonicalAndReservedPaths(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"",
		".",
		"../photo.jpg",
		"Trips/../photo.jpg",
		"/photo.jpg",
		`C:\\photo.jpg`,
		`Trips\\photo.jpg`,
		"Trips//photo.jpg",
		"Trips/photo.jpg/",
		"Trips/CON.jpg",
		"Trips/photo.jpg ",
		"Trips/photo?.jpg",
		"Trips/\x00photo.jpg",
		".lumilio/assets/photo.jpg",
		".lumiliorepo",
		".lumilioroot",
	} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			if _, err := layout.ParseMedia(value); err == nil {
				t.Fatalf("layout.ParseMedia(%q) unexpectedly succeeded", value)
			}
		})
	}
}

func TestParsePrivateRepositoryPath(t *testing.T) {
	t.Parallel()

	privatePath, err := layout.Parse(".lumilio/staging/incoming/upload.tmp")
	if err != nil {
		t.Fatal(err)
	}
	if !privatePath.IsPrivate() {
		t.Fatal("private path did not retain its namespace")
	}
	if _, err := layout.Parse("inbox/upload.jpg"); !errors.Is(err, layout.ErrPathNamespace) {
		t.Fatalf("error = %v, want namespace error", err)
	}
}
