package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReady(t *testing.T) {
	server := httptest.NewServer(NewHandler())
	defer server.Close()

	response, err := http.Get(server.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
}

func TestFraudScoreReturnsValidFallback(t *testing.T) {
	server := httptest.NewServer(NewHandler())
	defer server.Close()

	response, err := http.Post(
		server.URL+"/fraud-score",
		"application/json",
		strings.NewReader(`{"id":"tx-1"}`),
	)
	if err != nil {
		t.Fatalf("POST /fraud-score failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
}
