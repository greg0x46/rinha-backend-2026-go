package fraudindex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadJSONReferences(t *testing.T) {
	references, err := LoadJSONReferences("../../../resources/example-references.json")
	if err != nil {
		t.Fatalf("LoadJSONReferences failed: %v", err)
	}

	if len(references) == 0 {
		t.Fatal("len(references) = 0, want > 0")
	}
	if references[0].Vector[0] != 0.01 {
		t.Fatalf("references[0].Vector[0] = %v, want 0.01", references[0].Vector[0])
	}
	if references[0].Label != LabelLegit {
		t.Fatalf("references[0].Label = %v, want LabelLegit", references[0].Label)
	}
}

func TestBinaryRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "references.bin")
	want := []Reference{
		{Vector: Vector{0: 0.1, 13: 0.9}, Label: LabelLegit},
		{Vector: Vector{0: 0.2, 13: 0.8}, Label: LabelFraud},
	}

	writer, err := CreateBinary(path)
	if err != nil {
		t.Fatalf("CreateBinary failed: %v", err)
	}
	for _, reference := range want {
		if err := writer.Write(reference); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	got, manifest, err := LoadBinary(path)
	if err != nil {
		t.Fatalf("LoadBinary failed: %v", err)
	}

	if manifest.Version != BinaryVersion {
		t.Fatalf("manifest.Version = %d, want %d", manifest.Version, BinaryVersion)
	}
	if manifest.Dimension != Dimensions {
		t.Fatalf("manifest.Dimension = %d, want %d", manifest.Dimension, Dimensions)
	}
	if manifest.References != uint64(len(want)) {
		t.Fatalf("manifest.References = %d, want %d", manifest.References, len(want))
	}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("reference %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestLoadBinaryRejectsInvalidManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "references.bin")
	if err := os.WriteFile(path, []byte("invalid"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if _, _, err := LoadBinary(path); err == nil {
		t.Fatal("LoadBinary succeeded, want error")
	}
}

func BenchmarkLoadBinaryFull(b *testing.B) {
	path := "../../data/references.bin"
	if _, err := os.Stat(path); err != nil {
		b.Skip("data/references.bin not generated")
	}

	for b.Loop() {
		references, manifest, err := LoadBinary(path)
		if err != nil {
			b.Fatalf("LoadBinary failed: %v", err)
		}
		if manifest.References != 3_000_000 {
			b.Fatalf("manifest.References = %d, want 3000000", manifest.References)
		}
		if len(references) != 3_000_000 {
			b.Fatalf("len(references) = %d, want 3000000", len(references))
		}
	}
}
