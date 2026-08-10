package storage

import (
	"errors"
	"testing"
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
			got, err := ParseUserMediaPath(value)
			if err != nil {
				t.Fatalf("ParseUserMediaPath(%q): %v", value, err)
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
			if _, err := ParseUserMediaPath(value); err == nil {
				t.Fatalf("ParseUserMediaPath(%q) unexpectedly succeeded", value)
			}
		})
	}
}

func TestParsePrivateRepositoryPath(t *testing.T) {
	t.Parallel()

	privatePath, err := ParsePrivateRepositoryPath(".lumilio/staging/incoming/upload.tmp")
	if err != nil {
		t.Fatal(err)
	}
	if !privatePath.isPrivate() {
		t.Fatal("private path did not retain its namespace")
	}
	if _, err := ParsePrivateRepositoryPath("inbox/upload.jpg"); !errors.Is(err, ErrRepositoryPathNamespace) {
		t.Fatalf("error = %v, want namespace error", err)
	}
}
