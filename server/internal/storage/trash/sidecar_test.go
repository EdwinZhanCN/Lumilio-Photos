package trash

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func validSidecar() Sidecar {
	return Sidecar{
		TrashID: uuid.NewString(), RepositoryID: uuid.NewString(), OriginalPath: "trips/2026/a.jpg",
		AssetID: uuid.NewString(), HashAlgorithm: "blake3-v1", ContentHash: strings.Repeat("a", 64),
		Size: 12, MtimeNs: 1, DeletedAt: time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC), Actor: "web:user:1",
	}
}

func TestSidecarFormatOneRoundTrips(t *testing.T) {
	want := validSidecar()
	encoded, err := want.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"format": 1`) {
		t.Fatalf("sidecar does not declare format 1: %s", encoded)
	}
	got, err := ParseSidecar(encoded)
	if err != nil {
		t.Fatal(err)
	}
	want.Format = SidecarFormat
	if got != want || got.Name() != "a.jpg" {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

func TestSidecarReaderAcceptsAddedFieldsInFormatOne(t *testing.T) {
	encoded, err := validSidecar().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	extended := strings.Replace(string(encoded), `"format": 1,`, `"format": 1, "added_later": true,`, 1)
	if _, err := ParseSidecar([]byte(extended)); err != nil {
		t.Fatalf("format 1 with an added field: %v", err)
	}
}

func TestSidecarReaderRejectsOtherFormatsWithoutGuessing(t *testing.T) {
	encoded, err := validSidecar().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"newer":       strings.Replace(string(encoded), `"format": 1`, `"format": 2`, 1),
		"unversioned": strings.Replace(string(encoded), `"format": 1,`, ``, 1),
		"not json":    "trash",
		"escaping":    strings.Replace(string(encoded), `trips/2026/a.jpg`, `../outside.jpg`, 1),
	} {
		if _, err := ParseSidecar([]byte(data)); !errors.Is(err, ErrSidecarFormat) {
			t.Errorf("%s sidecar: error = %v, want ErrSidecarFormat", name, err)
		}
	}
	_, err = ParseSidecar([]byte(strings.Replace(string(encoded), `"format": 1`, `"format": 2`, 1)))
	if err == nil || !strings.Contains(err.Error(), "format 2") {
		t.Fatalf("newer format error %q does not name the format", err)
	}
}
