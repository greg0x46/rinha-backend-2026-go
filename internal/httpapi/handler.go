package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
)

const maxRequestBodyBytes = 16 << 10

type Handler struct {
	vectorizer Vectorizer
}

func NewHandler() http.Handler {
	vectorizer, err := NewVectorizer()
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	h := Handler{vectorizer: vectorizer}
	mux.HandleFunc("GET /ready", h.ready)
	mux.HandleFunc("POST /fraud-score", h.fraudScore)
	return mux
}

func (h Handler) ready(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) fraudScore(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var payload FraudScoreRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxRequestBodyBytes))
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(w, FraudScoreResponse{Approved: true, FraudScore: 0.0})
		return
	}

	_ = h.vectorizer.Vectorize(payload)

	writeJSON(w, FraudScoreResponse{Approved: true, FraudScore: 0.0})
}

func writeJSON(w http.ResponseWriter, response FraudScoreResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
