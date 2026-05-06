package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
)

const maxRequestBodyBytes = 16 << 10
const defaultReferencesPath = "/app/data/references.bin"

type Handler struct {
	vectorizer Vectorizer
	scorer     Scorer
	isReady    bool
}

func NewHandler() http.Handler {
	vectorizer, err := NewVectorizer()
	if err != nil {
		panic(err)
	}
	references, err := LoadReferences(referencesPath())

	return newHandler(vectorizer, NewScorer(references), err == nil && len(references) > 0)
}

func NewHandlerWithDependencies(vectorizer Vectorizer, scorer Scorer) http.Handler {
	return newHandler(vectorizer, scorer, true)
}

func newHandler(vectorizer Vectorizer, scorer Scorer, ready bool) http.Handler {
	mux := http.NewServeMux()
	h := Handler{
		vectorizer: vectorizer,
		scorer:     scorer,
		isReady:    ready,
	}
	mux.HandleFunc("GET /ready", h.ready)
	mux.HandleFunc("POST /fraud-score", h.fraudScore)
	return mux
}

func (h Handler) ready(w http.ResponseWriter, r *http.Request) {
	if !h.isReady {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
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
