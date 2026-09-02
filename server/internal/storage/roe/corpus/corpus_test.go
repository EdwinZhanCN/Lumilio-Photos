package corpus

import (
	"io"
	"testing"
)

func TestGeneratedShapesAreDeterministicAndParentFirst(t *testing.T) {
	t.Parallel()
	for _, layout := range []Layout{Wide, Deep} {
		layout := layout
		t.Run(string(layout), func(t *testing.T) {
			t.Parallel()
			shape := Shape{Entries: 100_000, Directories: 1_000, Layout: layout, DuplicateEvery: 5, PayloadBytes: 1 << 20}
			first, err := New(shape)
			if err != nil {
				t.Fatal(err)
			}
			second, err := New(shape)
			if err != nil {
				t.Fatal(err)
			}
			seen := map[string]struct{}{"root": {}}
			count := 0
			for {
				left, leftOK := first.Next()
				right, rightOK := second.Next()
				if leftOK != rightOK || left != right {
					t.Fatalf("generator diverged at %d: left=%+v right=%+v", count, left, right)
				}
				if !leftOK {
					break
				}
				if _, ok := seen[left.ParentID]; !ok {
					t.Fatalf("entry %d references unseen parent %q", count, left.ParentID)
				}
				seen[left.NodeID] = struct{}{}
				count++
			}
			if count != shape.Entries {
				t.Fatalf("entry count = %d, want %d", count, shape.Entries)
			}
		})
	}
}

func TestByteReaderProducesFixedPayloadWithoutShortRead(t *testing.T) {
	t.Parallel()
	left, err := io.ReadAll(ByteReader(65_537, 0x2a))
	if err != nil {
		t.Fatal(err)
	}
	right, err := io.ReadAll(ByteReader(65_537, 0x2a))
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 65_537 || string(left) != string(right) {
		t.Fatal("fixed byte corpus was not deterministic")
	}
}

func BenchmarkMetadataTrees(b *testing.B) {
	for _, entries := range []int{100_000, 500_000} {
		for _, layout := range []Layout{Wide, Deep} {
			b.Run(string(layout)+"/entries-"+benchmarkCount(entries), func(b *testing.B) {
				b.ReportAllocs()
				for range b.N {
					generator, err := New(Shape{
						Entries: entries, Directories: 2_048, Layout: layout,
						DuplicateEvery: 10, PayloadBytes: 1 << 20,
					})
					if err != nil {
						b.Fatal(err)
					}
					count := 0
					for _, ok := generator.Next(); ok; _, ok = generator.Next() {
						count++
					}
					if count != entries {
						b.Fatalf("generated %d entries, want %d", count, entries)
					}
				}
				b.ReportMetric(float64(entries*b.N)/b.Elapsed().Seconds(), "entries/s")
			})
		}
	}
}

func benchmarkCount(value int) string {
	if value == 500_000 {
		return "500k"
	}
	return "100k"
}
