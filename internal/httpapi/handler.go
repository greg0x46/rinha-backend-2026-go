package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
)

const maxRequestBodyBytes = 16 << 10
const defaultReferencesPath = "/app/data/references.bin"

var fraudScoreResponses = [nearestNeighbors + 1][]byte{
	mustEncode(FraudScoreResponse{Approved: true, FraudScore: 0.0}),
	mustEncode(FraudScoreResponse{Approved: true, FraudScore: 0.2}),
	mustEncode(FraudScoreResponse{Approved: true, FraudScore: 0.4}),
	mustEncode(FraudScoreResponse{Approved: false, FraudScore: 0.6}),
	mustEncode(FraudScoreResponse{Approved: false, FraudScore: 0.8}),
	mustEncode(FraudScoreResponse{Approved: false, FraudScore: 1.0}),
}

var fallbackResponse = fraudScoreResponses[0]

func mustEncode(response FraudScoreResponse) []byte {
	encoded, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}
	return append(encoded, '\n')
}

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
	scorer, err := LoadScorer(referencesPath())

	return newHandler(vectorizer, scorer, err == nil && scorer.HasReferences())
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

func (s Scorer) HasReferences() bool {
	if s.ivf {
		return len(s.ivfIndex.Vectors) > 0
	}
	if s.quantized {
		return len(s.quantizedIndex.Vectors) > 0
	}
	return len(s.references) > 0
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
		writeFraudScore(w, fallbackResponse)
		return
	}

	vector := h.vectorizer.Vectorize(payload)
	writeFraudScore(w, fraudScoreResponses[h.scorer.Frauds(vector)])
}

func writeFraudScore(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func referencesPath() string {
	if path := os.Getenv("REFERENCES_PATH"); path != "" {
		return path
	}
	return defaultReferencesPath
}
