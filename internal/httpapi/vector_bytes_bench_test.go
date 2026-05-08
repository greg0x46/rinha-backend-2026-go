package httpapi

import (
	"encoding/json"
	"testing"

	"github.com/greg/rinha-be-2026/internal/fastjson"
)

var benchBody = []byte(`{"id":"tx-1329056812","transaction":{"amount":2508.13,"installments":7,"requested_at":"2026-03-11T03:45:53Z"},"customer":{"avg_amount":209.74,"tx_count_24h":13,"known_merchants":["MERC-003","MERC-016","MERC-001","MERC-007"]},"merchant":{"id":"MERC-089","mcc":"7801","avg_amount":25.15},"terminal":{"is_online":false,"card_present":true,"km_from_home":667.7296579973},"last_transaction":{"timestamp":"2026-03-11T01:30:00Z","km_from_current":42.5}}`)

// BenchmarkDecodeAndVectorizeFastJSON measures the fastjson hot path:
// fastjson.Parse + VectorizeFromPayload. Sets the bar that the handler
// fast path is actually faster than encoding/json + Vectorize.
func BenchmarkDecodeAndVectorizeFastJSON(b *testing.B) {
	v, err := NewVectorizer()
	if err != nil {
		b.Fatal(err)
	}
	p := &fastjson.Payload{KnownMerchants: make([][]byte, 0, 16)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Reset()
		if err := fastjson.Parse(benchBody, p); err != nil {
			b.Fatalf("parse: %v", err)
		}
		_ = v.VectorizeFromPayload(p)
	}
}

// BenchmarkDecodeAndVectorizeEncodingJSON is the reference path used
// when fastjson rejects the body.
func BenchmarkDecodeAndVectorizeEncodingJSON(b *testing.B) {
	v, err := NewVectorizer()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var req FraudScoreRequest
		if err := json.Unmarshal(benchBody, &req); err != nil {
			b.Fatalf("unmarshal: %v", err)
		}
		_ = v.Vectorize(req)
	}
}
