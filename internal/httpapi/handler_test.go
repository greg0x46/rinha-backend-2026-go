package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReady(t *testing.T) {
	handler := newTestHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestFraudScoreReturnsValidFallback(t *testing.T) {
	handler := newTestHandler(t)
	request := httptest.NewRequest(
		http.MethodPost,
		"/fraud-score",
		strings.NewReader(`{"id":"tx-1"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
}

func TestReadyReturnsUnavailableWithoutReferences(t *testing.T) {
	vectorizer := newTestVectorizer(t)
	handler := newHandler(vectorizer, NewScorer(nil), false)
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestNewHandlerReadyAfterLoadingBinaryReferences(t *testing.T) {
	path := writeTestBinaryReferences(t, []Reference{
		{Vector: Vector{}, Label: LabelLegit},
		{Vector: Vector{0: 0.01}, Label: LabelLegit},
		{Vector: Vector{0: 0.02}, Label: LabelLegit},
		{Vector: Vector{0: 0.03}, Label: LabelLegit},
		{Vector: Vector{0: 0.04}, Label: LabelLegit},
	})
	t.Setenv("REFERENCES_PATH", path)

	handler := NewHandler()
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	vectorizer := newTestVectorizer(t)
	return NewHandlerWithDependencies(vectorizer, NewScorer([]Reference{
		{Vector: Vector{}, Label: LabelLegit},
		{Vector: Vector{0: 0.01}, Label: LabelLegit},
		{Vector: Vector{0: 0.02}, Label: LabelLegit},
		{Vector: Vector{0: 0.03}, Label: LabelLegit},
		{Vector: Vector{0: 0.04}, Label: LabelLegit},
	}))
}

func TestFraudScoreRequestDecodesOfficialPayload(t *testing.T) {
	t.Run("with last transaction", func(t *testing.T) {
		var request FraudScoreRequest
		err := json.Unmarshal([]byte(`{
			"id": "tx-3576980410",
			"transaction": {
				"amount": 384.88,
				"installments": 3,
				"requested_at": "2026-03-11T20:23:35Z"
			},
			"customer": {
				"avg_amount": 769.76,
				"tx_count_24h": 3,
				"known_merchants": ["MERC-009", "MERC-001", "MERC-001"]
			},
			"merchant": {
				"id": "MERC-001",
				"mcc": "5912",
				"avg_amount": 298.95
			},
			"terminal": {
				"is_online": false,
				"card_present": true,
				"km_from_home": 13.7090520965
			},
			"last_transaction": {
				"timestamp": "2026-03-11T14:58:35Z",
				"km_from_current": 18.8626479774
			}
		}`), &request)
		if err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}

		if request.ID != "tx-3576980410" {
			t.Fatalf("id = %q, want tx-3576980410", request.ID)
		}
		if request.Transaction.RequestedAt.IsZero() {
			t.Fatal("requested_at was not decoded")
		}
		if request.LastTransaction == nil {
			t.Fatal("last_transaction = nil, want value")
		}
		if request.LastTransaction.Timestamp.IsZero() {
			t.Fatal("last_transaction.timestamp was not decoded")
		}
	})

	t.Run("with null last transaction", func(t *testing.T) {
		var request FraudScoreRequest
		err := json.Unmarshal([]byte(`{
			"id": "tx-1329056812",
			"transaction": { "amount": 41.12, "installments": 2, "requested_at": "2026-03-11T18:45:53Z" },
			"customer": { "avg_amount": 82.24, "tx_count_24h": 3, "known_merchants": ["MERC-003", "MERC-016"] },
			"merchant": { "id": "MERC-016", "mcc": "5411", "avg_amount": 60.25 },
			"terminal": { "is_online": false, "card_present": true, "km_from_home": 29.23 },
			"last_transaction": null
		}`), &request)
		if err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}

		if request.LastTransaction != nil {
			t.Fatalf("last_transaction = %#v, want nil", request.LastTransaction)
		}
	})
}
