package app

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// blackboxJPEGWithMetadata is blackboxJPEG with a real EXIF APP1 segment
// (TIFF IFD0: ImageDescription 0x010E and Rating 0x4746) and an XMP APP1
// packet (xmp:Rating, dc:description, dc:subject), the way an editor or a
// tagging tool writes a photo back. Pixels follow shade, so two calls with
// different shades are a pixel edit and two with the same shade a
// metadata-only write.
func blackboxJPEGWithMetadata(tb testing.TB, shade uint8, marker string, rating int, description, keyword string) []byte {
	tb.Helper()
	source := blackboxJPEG(tb, shade, marker)
	exif := exifAPP1(description, rating)
	xmp := xmpAPP1(rating, description, keyword)
	out := make([]byte, 0, len(source)+len(exif)+len(xmp))
	out = append(out, source[:2]...)
	out = append(out, exif...)
	out = append(out, xmp...)
	return append(out, source[2:]...)
}

func app1Segment(payload []byte) []byte {
	length := len(payload) + 2
	return append([]byte{0xff, 0xe1, byte(length >> 8), byte(length)}, payload...)
}

func exifAPP1(description string, rating int) []byte {
	text := append([]byte(description), 0)
	var tiff bytes.Buffer
	tiff.WriteString("II")
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(42))
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(8))
	// IFD0: two entries, then the next-IFD pointer, then the string value.
	const entries = 2
	valueOffset := uint32(8 + 2 + entries*12 + 4)
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(entries))
	// 0x010E ImageDescription, ASCII, count = len+1, offset to the string.
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(0x010E))
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(2))
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(len(text)))
	_ = binary.Write(&tiff, binary.LittleEndian, valueOffset)
	// 0x4746 Rating, SHORT, count 1, value inline.
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(0x4746))
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(3))
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(1))
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(rating))
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(0))
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(0))
	tiff.Write(text)
	return app1Segment(append([]byte("Exif\x00\x00"), tiff.Bytes()...))
}

func xmpAPP1(rating int, description, keyword string) []byte {
	packet := fmt.Sprintf(`<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
<rdf:Description rdf:about="" xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmp:Rating="%d">
<dc:description><rdf:Alt><rdf:li xml:lang="x-default">%s</rdf:li></rdf:Alt></dc:description>
<dc:subject><rdf:Bag><rdf:li>%s</rdf:li></rdf:Bag></dc:subject>
</rdf:Description></rdf:RDF></x:xmpmeta>
<?xpacket end="w"?>`, rating, description, keyword)
	return app1Segment(append([]byte("http://ns.adobe.com/xap/1.0/\x00"), packet...))
}

func (s *blackboxServer) description(assetID string) string {
	s.t.Helper()
	detail := s.asset(assetID)
	var specific struct {
		Description string `json:"description"`
	}
	if len(detail.Metadata) > 0 {
		if err := json.Unmarshal(detail.Metadata, &specific); err != nil {
			s.t.Fatalf("specific metadata %s: %v", detail.Metadata, err)
		}
	}
	return specific.Description
}

// #223 in-place carry-over: a tool rewrites the file in place, with a real
// EXIF and XMP write that carries its own rating, caption, and keyword. The
// Asset keeps its ID, the user's rating and like, user tag, and edited
// description; a pixel edit regenerates the thumbnail. The file's own
// metadata never overrides what the user set.
func TestLifecycleInPlaceEditKeepsUserMetadataAndRegeneratesArtifacts(t *testing.T) {
	server := startBlackboxServer(t)
	cases := []struct {
		name             string
		afterShade       uint8
		wantNewThumbnail bool
	}{
		{"pixel edit with new EXIF and XMP", 220, true},
		{"metadata-only EXIF and XMP write", 40, false},
	}
	for index, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			filename := fmt.Sprintf("in-place-edit-%d.jpg", index)
			marker := fmt.Sprintf("in-place-edit-%d", index)
			server.upload(server.primary, filename, blackboxJPEG(t, 40, marker))
			var listed []blackboxAsset
			if !waitFor(time.Minute, func() bool { listed = server.listByFilename(filename); return len(listed) == 1 }) {
				t.Fatalf("uploaded file never appeared in browse: %+v", listed)
			}
			original := listed[0]
			server.waitProcessingSettled()
			server.setRatingAndLike(original.AssetID, 5, true)
			server.addTag(original.AssetID, "keep-me")
			server.setDescription(original.AssetID, "written by the user")
			var before []byte
			if !waitFor(time.Minute, func() bool { before = server.thumbnail(original.AssetID, "small"); return before != nil }) {
				t.Fatal("no small thumbnail before the edit")
			}
			path := originalPath(t, server.primary, filename)
			if path == "" {
				t.Fatalf("%s is not in the repository", filename)
			}

			rewritten := blackboxJPEGWithMetadata(t, testCase.afterShade, marker, 1, "written by the tool", "from-the-file")
			if err := os.WriteFile(path, rewritten, 0o644); err != nil {
				t.Fatal(err)
			}
			settled := time.Now().Add(-time.Hour)
			if err := os.Chtimes(path, settled, settled); err != nil {
				t.Fatal(err)
			}
			server.scan(server.primary, 2*time.Minute)
			if !waitFor(time.Minute, func() bool {
				listed = server.listByFilename(filename)
				return len(listed) == 1 && listed[0].ContentID != original.ContentID
			}) {
				t.Fatalf("rewritten content never reached browse: %+v", listed)
			}
			if listed[0].AssetID != original.AssetID {
				t.Fatalf("in-place edit produced Asset %s, want the same %s", listed[0].AssetID, original.AssetID)
			}
			server.waitProcessingSettled()

			detail := server.asset(original.AssetID)
			if detail.Rating == nil || *detail.Rating != 5 || detail.Liked == nil || !*detail.Liked {
				t.Fatalf("rating/like after the edit = %v/%v, want the user's 5 and true (the file says 1)", detail.Rating, detail.Liked)
			}
			if tags := server.tags(original.AssetID); tags["keep-me"] != "user" {
				t.Fatalf("tags after the edit = %v, want the user tag keep-me", tags)
			}
			if got := server.description(original.AssetID); got != "written by the user" {
				t.Fatalf("description after the edit = %q, want the user's (the file says %q)", got, "written by the tool")
			}
			if testCase.wantNewThumbnail {
				var after []byte
				if !waitFor(time.Minute, func() bool {
					after = server.thumbnail(original.AssetID, "small")
					return after != nil && !bytes.Equal(after, before)
				}) {
					t.Fatal("small thumbnail was not regenerated after the pixel edit")
				}
			} else if after := server.thumbnail(original.AssetID, "small"); after == nil {
				t.Fatal("small thumbnail is gone after the metadata-only write")
			}
		})
	}
}
