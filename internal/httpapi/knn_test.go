package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/greg/rinha-be-2026/internal/fraudindex"
)

func TestLoadReferences(t *testing.T) {
	path := writeTestBinaryReferences(t, []Reference{
		{Vector: Vector{0: 0.01}, Label: LabelLegit},
		{Vector: Vector{0: 0.02}, Label: LabelFraud},
	})

	references, err := LoadReferences(path)
	if err != nil {
		t.Fatalf("LoadReferences failed: %v", err)
	}

	if len(references) != 2 {
		t.Fatalf("len(references) = %d, want 2", len(references))
	}
	if references[0].Vector[0] != 0.01 {
		t.Fatalf("references[0].Vector[0] = %v, want 0.01", references[0].Vector[0])
	}
	if references[0].Label != LabelLegit {
		t.Fatalf("references[0].Label = %v, want LabelLegit", references[0].Label)
	}
}

func TestLoadScorerUsesQuantizedBinary(t *testing.T) {
	path := writeTestQuantizedBinaryReferences(t, []Reference{
		{Vector: Vector{0: 0.00}, Label: LabelFraud},
		{Vector: Vector{0: 0.01}, Label: LabelFraud},
		{Vector: Vector{0: 0.02}, Label: LabelFraud},
		{Vector: Vector{0: 0.03}, Label: LabelLegit},
		{Vector: Vector{0: 0.04}, Label: LabelLegit},
	})

	scorer, err := LoadScorer(path)
	if err != nil {
		t.Fatalf("LoadScorer failed: %v", err)
	}
	if !scorer.quantized {
		t.Fatal("LoadScorer loaded float32 scorer, want quantized")
	}

	response := scorer.Score(Vector{})
	if response.FraudScore != 0.6 {
		t.Fatalf("FraudScore = %v, want 0.6", response.FraudScore)
	}
	if response.Approved {
		t.Fatal("Approved = true, want false")
	}
}

func TestLoadScorerUsesIVFBinary(t *testing.T) {
	path := writeTestIVFBinaryReferences(t, []Reference{
		{Vector: Vector{0: -0.02}, Label: LabelFraud},
		{Vector: Vector{0: -0.01}, Label: LabelFraud},
		{Vector: Vector{0: 0.00}, Label: LabelFraud},
		{Vector: Vector{0: 0.01}, Label: LabelLegit},
		{Vector: Vector{0: 0.02}, Label: LabelLegit},
		{Vector: Vector{0: 0.90}, Label: LabelLegit},
	})

	scorer, err := LoadScorer(path)
	if err != nil {
		t.Fatalf("LoadScorer failed: %v", err)
	}
	if !scorer.ivf {
		t.Fatal("LoadScorer did not load IVF scorer")
	}

	response := scorer.Score(Vector{})
	if response.FraudScore != 0.6 {
		t.Fatalf("FraudScore = %v, want 0.6", response.FraudScore)
	}
	if response.Approved {
		t.Fatal("Approved = true, want false")
	}
}

func TestIVFScorerCanRetryBoundaryWithMoreLists(t *testing.T) {
	centroids := make([]fraudindex.QuantizedVector, 16)
	for i := range centroids {
		centroids[i] = fraudindex.QuantizeVector(Vector{0: float32(i+1) / 10})
	}
	index := fraudindex.IVFIndex{
		Centroids: centroids,
		Offsets:   []uint64{0, 5, 5, 5, 5, 5, 5, 5, 5, 6, 6, 6, 6, 6, 6, 6, 6},
		Vectors: []fraudindex.QuantizedVector{
			fraudindex.QuantizeVector(Vector{0: 0.01}),
			fraudindex.QuantizeVector(Vector{0: 0.02}),
			fraudindex.QuantizeVector(Vector{0: 0.03}),
			fraudindex.QuantizeVector(Vector{0: 0.04}),
			fraudindex.QuantizeVector(Vector{0: 0.05}),
			fraudindex.QuantizeVector(Vector{0: 0.00}),
		},
		Labels: []fraudindex.Label{
			fraudindex.LabelFraud,
			fraudindex.LabelFraud,
			fraudindex.LabelLegit,
			fraudindex.LabelLegit,
			fraudindex.LabelLegit,
			fraudindex.LabelFraud,
		},
	}
	response := NewIVFScorer(index).Score(Vector{})

	if response.FraudScore != 0.6 {
		t.Fatalf("FraudScore = %v, want 0.6", response.FraudScore)
	}
	if response.Approved {
		t.Fatal("Approved = true, want false")
	}
}

func TestSquaredDistance(t *testing.T) {
	a := Vector{0: 1, 1: 2, 2: -1}
	b := Vector{0: 4, 1: 6, 2: -1}

	if got := squaredDistance(a, b); got != 25 {
		t.Fatalf("squaredDistance = %v, want 25", got)
	}
}

func TestScorerReturnsFiveNearestNeighbors(t *testing.T) {
	references := []Reference{
		{Vector: Vector{0: 10}, Label: LabelFraud},
		{Vector: Vector{0: 0.50}, Label: LabelLegit},
		{Vector: Vector{0: 0.10}, Label: LabelFraud},
		{Vector: Vector{0: 0.20}, Label: LabelFraud},
		{Vector: Vector{0: 0.30}, Label: LabelLegit},
		{Vector: Vector{0: 0.40}, Label: LabelFraud},
		{Vector: Vector{0: 9}, Label: LabelLegit},
	}

	response := NewScorer(references).Score(Vector{})

	if response.FraudScore != 0.6 {
		t.Fatalf("FraudScore = %v, want 0.6", response.FraudScore)
	}
	if response.Approved {
		t.Fatal("Approved = true, want false")
	}
}

func TestQuantizedScorerMatchesFloatDecision(t *testing.T) {
	references := []Reference{
		{Vector: Vector{0: 0.00, 1: -1}, Label: LabelFraud},
		{Vector: Vector{0: 0.01, 1: -1}, Label: LabelFraud},
		{Vector: Vector{0: 0.02, 1: -1}, Label: LabelLegit},
		{Vector: Vector{0: 0.03, 1: -1}, Label: LabelLegit},
		{Vector: Vector{0: 0.04, 1: -1}, Label: LabelFraud},
		{Vector: Vector{0: 0.90, 1: 1}, Label: LabelFraud},
	}
	index := fraudindex.QuantizedIndex{
		Vectors: make([]fraudindex.QuantizedVector, len(references)),
		Labels:  make([]fraudindex.Label, len(references)),
	}
	for i, reference := range references {
		index.Vectors[i] = fraudindex.QuantizeVector(reference.Vector)
		index.Labels[i] = reference.Label
	}

	query := Vector{0: 0.015, 1: -1}
	floatResponse := NewScorer(references).Score(query)
	quantizedResponse := NewQuantizedScorer(index).Score(query)

	if quantizedResponse != floatResponse {
		t.Fatalf("quantized response = %#v, want %#v", quantizedResponse, floatResponse)
	}
}

func TestSquaredQuantizedDistance(t *testing.T) {
	a := fraudindex.QuantizedVector{0: 32767, 1: -32767}
	b := fraudindex.QuantizedVector{0: -32767, 1: 32767}

	if got := squaredQuantizedDistance(a, b); got != 8_589_410_312 {
		t.Fatalf("squaredQuantizedDistance = %v, want 8589410312", got)
	}
}

func TestFraudScoreUsesScorer(t *testing.T) {
	vectorizer := newTestVectorizer(t)
	references := []Reference{
		{Vector: Vector{0: 0.00}, Label: LabelFraud},
		{Vector: Vector{0: 0.01}, Label: LabelFraud},
		{Vector: Vector{0: 0.02}, Label: LabelFraud},
		{Vector: Vector{0: 0.03}, Label: LabelLegit},
		{Vector: Vector{0: 0.04}, Label: LabelLegit},
	}
	handler := NewHandlerWithDependencies(vectorizer, NewScorer(references))
	request := httptest.NewRequest(
		http.MethodPost,
		"/fraud-score",
		strings.NewReader(`{
			"id": "tx-1",
			"transaction": { "amount": 0, "installments": 0, "requested_at": "2026-03-09T00:00:00Z" },
			"customer": { "avg_amount": 0, "tx_count_24h": 0, "known_merchants": ["MERC-001"] },
			"merchant": { "id": "MERC-001", "mcc": "0000", "avg_amount": 0 },
			"terminal": { "is_online": false, "card_present": false, "km_from_home": 0 },
			"last_transaction": {
				"timestamp": "2026-03-09T00:00:00Z",
				"km_from_current": 0
			}
		}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	var body FraudScoreResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.FraudScore != 0.6 {
		t.Fatalf("FraudScore = %v, want 0.6", body.FraudScore)
	}
	if body.Approved {
		t.Fatal("Approved = true, want false")
	}
}

func writeTestBinaryReferences(t *testing.T, references []Reference) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "references.bin")
	writer, err := fraudindex.CreateBinary(path)
	if err != nil {
		t.Fatalf("CreateBinary failed: %v", err)
	}
	for _, reference := range references {
		if err := writer.Write(reference); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	return path
}

func writeTestQuantizedBinaryReferences(t *testing.T, references []Reference) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "references.bin")
	writer, err := fraudindex.CreateQuantizedBinary(path)
	if err != nil {
		t.Fatalf("CreateQuantizedBinary failed: %v", err)
	}
	for _, reference := range references {
		if err := writer.Write(reference); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	return path
}

func writeTestIVFBinaryReferences(t *testing.T, references []Reference) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "references.bin")
	if _, err := fraudindex.WriteIVFBinary(path, references, 2); err != nil {
		t.Fatalf("WriteIVFBinary failed: %v", err)
	}
	return path
}
