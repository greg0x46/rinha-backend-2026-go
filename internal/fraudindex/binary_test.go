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

func TestQuantizedBinaryRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "references.bin")
	want := []Reference{
		{Vector: Vector{0: 0.1, 1: -1, 13: 0.9}, Label: LabelLegit},
		{Vector: Vector{0: 0.2, 1: 1, 13: 0.8}, Label: LabelFraud},
	}

	writer, err := CreateQuantizedBinary(path)
	if err != nil {
		t.Fatalf("CreateQuantizedBinary failed: %v", err)
	}
	for _, reference := range want {
		if err := writer.Write(reference); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	got, manifest, err := LoadQuantizedBinary(path)
	if err != nil {
		t.Fatalf("LoadQuantizedBinary failed: %v", err)
	}

	if manifest.Version != QuantizedBinaryVersion {
		t.Fatalf("manifest.Version = %d, want %d", manifest.Version, QuantizedBinaryVersion)
	}
	if manifest.Dimension != Dimensions {
		t.Fatalf("manifest.Dimension = %d, want %d", manifest.Dimension, Dimensions)
	}
	if manifest.Scale != QuantizationScale {
		t.Fatalf("manifest.Scale = %d, want %d", manifest.Scale, QuantizationScale)
	}
	if manifest.References != uint64(len(want)) {
		t.Fatalf("manifest.References = %d, want %d", manifest.References, len(want))
	}
	if len(got.Vectors) != len(want) {
		t.Fatalf("len(got.Vectors) = %d, want %d", len(got.Vectors), len(want))
	}
	for i := range want {
		if got.Vectors[i] != QuantizeVector(want[i].Vector) {
			t.Fatalf("vector %d = %#v, want %#v", i, got.Vectors[i], QuantizeVector(want[i].Vector))
		}
		if got.Labels[i] != want[i].Label {
			t.Fatalf("label %d = %v, want %v", i, got.Labels[i], want[i].Label)
		}
	}
}

func TestIVFBinaryRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "references.bin")
	want := []Reference{
		{Vector: Vector{0: -0.9}, Label: LabelLegit},
		{Vector: Vector{0: -0.8}, Label: LabelFraud},
		{Vector: Vector{0: 0.8}, Label: LabelFraud},
		{Vector: Vector{0: 0.9}, Label: LabelLegit},
	}

	manifest, err := WriteIVFBinary(path, want, 2)
	if err != nil {
		t.Fatalf("WriteIVFBinary failed: %v", err)
	}
	if manifest.Version != IVFBinaryVersion {
		t.Fatalf("manifest.Version = %d, want %d", manifest.Version, IVFBinaryVersion)
	}
	if manifest.NList != 2 {
		t.Fatalf("manifest.NList = %d, want 2", manifest.NList)
	}

	got, loadedManifest, err := LoadIVFBinary(path)
	if err != nil {
		t.Fatalf("LoadIVFBinary failed: %v", err)
	}
	if loadedManifest.Version != IVFBinaryVersion {
		t.Fatalf("loadedManifest.Version = %d, want %d", loadedManifest.Version, IVFBinaryVersion)
	}
	if loadedManifest.References != uint64(len(want)) {
		t.Fatalf("loadedManifest.References = %d, want %d", loadedManifest.References, len(want))
	}
	if len(got.Centroids) != 2 {
		t.Fatalf("len(got.Centroids) = %d, want 2", len(got.Centroids))
	}
	if len(got.Offsets) != 3 {
		t.Fatalf("len(got.Offsets) = %d, want 3", len(got.Offsets))
	}
	if got.Offsets[0] != 0 || got.Offsets[len(got.Offsets)-1] != uint64(len(want)) {
		t.Fatalf("got.Offsets = %#v, want first 0 and last %d", got.Offsets, len(want))
	}
	if len(got.Vectors) != len(want) {
		t.Fatalf("len(got.Vectors) = %d, want %d", len(got.Vectors), len(want))
	}
	if len(got.Labels) != len(want) {
		t.Fatalf("len(got.Labels) = %d, want %d", len(got.Labels), len(want))
	}
}

func TestQuantizeVectorClampsToInt16Scale(t *testing.T) {
	got := QuantizeVector(Vector{0: -2, 1: -1, 2: 0.5, 3: 1, 4: 2})

	if got[0] != -int16(QuantizationScale) {
		t.Fatalf("got[0] = %d, want %d", got[0], -int16(QuantizationScale))
	}
	if got[1] != -int16(QuantizationScale) {
		t.Fatalf("got[1] = %d, want %d", got[1], -int16(QuantizationScale))
	}
	if got[2] != 16384 {
		t.Fatalf("got[2] = %d, want 16384", got[2])
	}
	if got[3] != int16(QuantizationScale) {
		t.Fatalf("got[3] = %d, want %d", got[3], int16(QuantizationScale))
	}
	if got[4] != int16(QuantizationScale) {
		t.Fatalf("got[4] = %d, want %d", got[4], int16(QuantizationScale))
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
			b.Skipf("data/references.bin is not float32: %v", err)
		}
		if manifest.References != 3_000_000 {
			b.Fatalf("manifest.References = %d, want 3000000", manifest.References)
		}
		if len(references) != 3_000_000 {
			b.Fatalf("len(references) = %d, want 3000000", len(references))
		}
	}
}

func BenchmarkLoadQuantizedBinaryFull(b *testing.B) {
	path := "../../data/references.bin"
	if _, err := os.Stat(path); err != nil {
		b.Skip("data/references.bin not generated")
	}

	for b.Loop() {
		index, manifest, err := LoadQuantizedBinary(path)
		if err != nil {
			b.Fatalf("LoadQuantizedBinary failed: %v", err)
		}
		if manifest.References != 3_000_000 {
			b.Fatalf("manifest.References = %d, want 3000000", manifest.References)
		}
		if len(index.Vectors) != 3_000_000 {
			b.Fatalf("len(index.Vectors) = %d, want 3000000", len(index.Vectors))
		}
		if len(index.Labels) != 3_000_000 {
			b.Fatalf("len(index.Labels) = %d, want 3000000", len(index.Labels))
		}
	}
}
