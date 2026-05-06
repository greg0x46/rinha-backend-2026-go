package httpapi

import (
	"encoding/json"
	"math"
	"testing"
)

func TestClamp(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  float64
	}{
		{name: "below min", value: -0.2, want: 0},
		{name: "inside range", value: 0.7, want: 0.7},
		{name: "above max", value: 1.4, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Clamp(tt.value, 0, 1); got != tt.want {
				t.Fatalf("Clamp(%v, 0, 1) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestVectorizeOfficialLegitExample(t *testing.T) {
	vectorizer := newTestVectorizer(t)
	request := decodeFraudScoreRequest(t, `{
		"id": "tx-1329056812",
		"transaction": { "amount": 41.12, "installments": 2, "requested_at": "2026-03-11T18:45:53Z" },
		"customer": { "avg_amount": 82.24, "tx_count_24h": 3, "known_merchants": ["MERC-003", "MERC-016"] },
		"merchant": { "id": "MERC-016", "mcc": "5411", "avg_amount": 60.25 },
		"terminal": { "is_online": false, "card_present": true, "km_from_home": 29.23 },
		"last_transaction": null
	}`)

	want := Vector{
		0.004112,
		0.1666666667,
		0.05,
		0.7826086957,
		0.3333333333,
		-1,
		-1,
		0.02923,
		0.15,
		0,
		1,
		0,
		0.15,
		0.006025,
	}
	assertVectorApprox(t, vectorizer.Vectorize(request), want)
}

func TestVectorizeOfficialFraudExample(t *testing.T) {
	vectorizer := newTestVectorizer(t)
	request := decodeFraudScoreRequest(t, `{
		"id": "tx-3330991687",
		"transaction": { "amount": 9505.97, "installments": 10, "requested_at": "2026-03-14T05:15:12Z" },
		"customer": { "avg_amount": 81.28, "tx_count_24h": 20, "known_merchants": ["MERC-008", "MERC-007", "MERC-005"] },
		"merchant": { "id": "MERC-068", "mcc": "7802", "avg_amount": 54.86 },
		"terminal": { "is_online": false, "card_present": true, "km_from_home": 952.27 },
		"last_transaction": null
	}`)

	want := Vector{
		0.950597,
		0.8333333333,
		1,
		0.2173913043,
		0.8333333333,
		-1,
		-1,
		0.95227,
		1,
		0,
		1,
		1,
		0.75,
		0.005486,
	}
	assertVectorApprox(t, vectorizer.Vectorize(request), want)
}

func TestVectorizeLastTransactionAndFallbacks(t *testing.T) {
	vectorizer := newTestVectorizer(t)
	request := decodeFraudScoreRequest(t, `{
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
			"mcc": "0000",
			"avg_amount": 298.95
		},
		"terminal": {
			"is_online": true,
			"card_present": false,
			"km_from_home": 13.7090520965
		},
		"last_transaction": {
			"timestamp": "2026-03-11T14:58:35Z",
			"km_from_current": 18.8626479774
		}
	}`)

	vector := vectorizer.Vectorize(request)
	assertApprox(t, float64(vector[5]), 325.0/1440.0)
	assertApprox(t, float64(vector[6]), 0.0188626479774)
	assertApprox(t, float64(vector[9]), 1)
	assertApprox(t, float64(vector[10]), 0)
	assertApprox(t, float64(vector[11]), 0)
	assertApprox(t, float64(vector[12]), 0.5)
}

func newTestVectorizer(t *testing.T) Vectorizer {
	t.Helper()
	vectorizer, err := NewVectorizer()
	if err != nil {
		t.Fatalf("NewVectorizer failed: %v", err)
	}
	return vectorizer
}

func decodeFraudScoreRequest(t *testing.T, input string) FraudScoreRequest {
	t.Helper()
	var request FraudScoreRequest
	if err := json.Unmarshal([]byte(input), &request); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	return request
}

func assertVectorApprox(t *testing.T, got, want Vector) {
	t.Helper()
	for i := range got {
		assertApprox(t, float64(got[i]), float64(want[i]))
	}
}

func assertApprox(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0000001 {
		t.Fatalf("value = %.10f, want %.10f", got, want)
	}
}
