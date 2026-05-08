package httpapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/greg/rinha-be-2026/internal/fastjson"
)

// TestVectorizeFromPayloadMatchesVectorize walks the official test dataset
// and checks that the fastjson decode + VectorizeFromPayload path produces
// the exact same vector as encoding/json + Vectorize for every entry.
// Bit-for-bit equality is required so the scorer's decisions are
// independent of the decode path.
func TestVectorizeFromPayloadMatchesVectorize(t *testing.T) {
	path := findTestData(t)
	if path == "" {
		t.Skip("test-data.json not available")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read test-data: %v", err)
	}
	var doc struct {
		Entries []struct {
			Request json.RawMessage `json:"request"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decode test-data: %v", err)
	}

	v, err := NewVectorizer()
	if err != nil {
		t.Fatalf("NewVectorizer: %v", err)
	}

	step := len(doc.Entries) / 2000
	if step < 1 {
		step = 1
	}
	checked := 0
	for i := 0; i < len(doc.Entries); i += step {
		body := []byte(doc.Entries[i].Request)

		var req FraudScoreRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("entry %d: encoding/json: %v", i, err)
		}
		expected := v.Vectorize(req)

		var p fastjson.Payload
		if err := fastjson.Parse(body, &p); err != nil {
			t.Fatalf("entry %d: fastjson Parse: %v", i, err)
		}
		got := v.VectorizeFromPayload(&p)

		if got != expected {
			t.Fatalf("entry %d: vector mismatch\n got=%v\nwant=%v\nbody=%s", i, got, expected, body)
		}
		checked++
	}
	if checked < 50 {
		t.Fatalf("expected at least 50 entries checked, got %d", checked)
	}
	t.Logf("verified %d entries", checked)
}

func findTestData(t *testing.T) string {
	t.Helper()
	for _, c := range []string{
		"../../../test/test-data.json",
		"../../test/test-data.json",
	} {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}
