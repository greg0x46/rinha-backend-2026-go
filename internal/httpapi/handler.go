package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
)

const maxRequestBodyBytes = 16 << 10
const defaultReferencesPath = "../resources/example-references.json"

type Handler struct {
	vectorizer Vectorizer
	scorer     Scorer
}

func NewHandler() http.Handler {
	vectorizer, err := NewVectorizer()
	if err != nil {
		panic(err)
	}
	references, _ := LoadReferences(referencesPath())

	return NewHandlerWithDependencies(vectorizer, NewScorer(references))
}

func NewHandlerWithDependencies(vectorizer Vectorizer, scorer Scorer) http.Handler {
	mux := http.NewServeMux()
	h := Handler{
		vectorizer: vectorizer,
		scorer:     scorer,
	}
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

	vector := h.vectorizer.Vectorize(payload)

	writeJSON(w, h.scorer.Score(vector))
}

func writeJSON(w http.ResponseWriter, response FraudScoreResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

func referencesPath() string {
	if path := os.Getenv("REFERENCES_PATH"); path != "" {
		return path
	}
	return defaultReferencesPath
}
