// Package corpus generates deterministic repository metadata and byte streams
// for ROE scale profiles without depending on checked-in media fixtures.
package corpus

import (
	"errors"
	"fmt"
	"io"
)

type Layout string

const (
	Wide Layout = "wide"
	Deep Layout = "deep"
)

type Shape struct {
	Entries        int
	Directories    int
	Layout         Layout
	DuplicateEvery int
	PayloadBytes   int64
}

type Entry struct {
	Ordinal     int
	NodeID      string
	ParentID    string
	Name        string
	Directory   bool
	Size        int64
	ContentHash string
}

type Generator struct {
	shape Shape
	next  int
}

func New(shape Shape) (*Generator, error) {
	if shape.Entries <= 0 {
		return nil, errors.New("corpus entry count must be positive")
	}
	if shape.Directories <= 0 || shape.Directories >= shape.Entries {
		return nil, errors.New("corpus directory count must be between zero and entry count")
	}
	if shape.Layout != Wide && shape.Layout != Deep {
		return nil, fmt.Errorf("unsupported corpus layout %q", shape.Layout)
	}
	if shape.DuplicateEvery <= 0 {
		shape.DuplicateEvery = 1
	}
	if shape.PayloadBytes < 0 {
		return nil, errors.New("corpus payload size cannot be negative")
	}
	return &Generator{shape: shape}, nil
}

func (g *Generator) Next() (Entry, bool) {
	if g == nil || g.next >= g.shape.Entries {
		return Entry{}, false
	}
	ordinal := g.next
	g.next++
	if ordinal < g.shape.Directories {
		parentID := "root"
		if g.shape.Layout == Deep && ordinal > 0 {
			parentID = nodeID(ordinal - 1)
		}
		return Entry{
			Ordinal: ordinal, NodeID: nodeID(ordinal), ParentID: parentID,
			Name: fmt.Sprintf("directory-%06d", ordinal), Directory: true,
		}, true
	}
	fileOrdinal := ordinal - g.shape.Directories
	parentOrdinal := fileOrdinal % g.shape.Directories
	if g.shape.Layout == Deep {
		parentOrdinal = g.shape.Directories - 1 - (fileOrdinal % g.shape.Directories)
	}
	contentGroup := fileOrdinal / g.shape.DuplicateEvery
	return Entry{
		Ordinal:     ordinal,
		NodeID:      nodeID(ordinal),
		ParentID:    nodeID(parentOrdinal),
		Name:        fmt.Sprintf("media-%09d.bin", fileOrdinal),
		Size:        g.shape.PayloadBytes,
		ContentHash: fmt.Sprintf("corpus-blake3-%016x", contentGroup),
	}, true
}

func nodeID(ordinal int) string {
	return fmt.Sprintf("node-%09d", ordinal)
}

// ByteReader emits a repeatable non-zero byte pattern without allocating the
// represented payload. It is safe to construct independently for exact-copy
// hash profiles.
func ByteReader(size int64, seed byte) io.Reader {
	return &patternReader{remaining: size, value: seed}
}

type patternReader struct {
	remaining int64
	value     byte
}

func (r *patternReader) Read(buffer []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	count := int64(len(buffer))
	if count > r.remaining {
		count = r.remaining
	}
	for index := int64(0); index < count; index++ {
		buffer[index] = r.value + byte(index%251)
	}
	r.remaining -= count
	return int(count), nil
}
