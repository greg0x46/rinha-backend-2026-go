package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
)

const maxRequestBodyBytes = 16 << 10

type Handler struct{}

type FraudScoreResponse struct {
	Approved   bool    `json:"approved"`
	FraudScore float64 `json:"fraud_score"`
}

func NewHandler() http.Handler {
	mux := http.NewServeMux()
	h := Handler{}
	mux.HandleFunc("GET /ready", h.ready)
	mux.HandleFunc("POST /fraud-score", h.fraudScore)
	return mux
}

func (h Handler) ready(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) fraudScore(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var payload map[string]any
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxRequestBodyBytes))
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(w, FraudScoreResponse{Approved: true, FraudScore: 0.0})
		return
	}

	writeJSON(w, FraudScoreResponse{Approved: true, FraudScore: 0.0})
}

func writeJSON(w http.ResponseWriter, response FraudScoreResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
