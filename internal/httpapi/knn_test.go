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
