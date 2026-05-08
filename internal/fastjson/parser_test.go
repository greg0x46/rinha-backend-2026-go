package fastjson

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type referenceRequest struct {
	ID          string `json:"id"`
	Transaction struct {
		Amount       float64   `json:"amount"`
		Installments int       `json:"installments"`
		RequestedAt  time.Time `json:"requested_at"`
	} `json:"transaction"`
	Customer struct {
		AvgAmount      float64  `json:"avg_amount"`
		TxCount24h     int      `json:"tx_count_24h"`
		KnownMerchants []string `json:"known_merchants"`
	} `json:"customer"`
	Merchant struct {
		ID        string  `json:"id"`
		MCC       string  `json:"mcc"`
		AvgAmount float64 `json:"avg_amount"`
	} `json:"merchant"`
	Terminal struct {
		IsOnline    bool    `json:"is_online"`
		CardPresent bool    `json:"card_present"`
		KmFromHome  float64 `json:"km_from_home"`
	} `json:"terminal"`
	LastTransaction *struct {
		Timestamp     time.Time `json:"timestamp"`
		KmFromCurrent float64   `json:"km_from_current"`
	} `json:"last_transaction"`
}

func TestParseBasicLegit(t *testing.T) {
	body := []byte(`{"id":"tx-1","transaction":{"amount":2508.13,"installments":7,"requested_at":"2026-03-11T03:45:53Z"},"customer":{"avg_amount":209.74,"tx_count_24h":13,"known_merchants":["MERC-003","MERC-016"]},"merchant":{"id":"MERC-089","mcc":"7801","avg_amount":25.15},"terminal":{"is_online":false,"card_present":true,"km_from_home":667.7296579973},"last_transaction":null}`)
	var p Payload
	if err := Parse(body, &p); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.Amount != 2508.13 {
		t.Fatalf("amount mismatch: %v", p.Amount)
	}
	if p.Installments != 7 {
		t.Fatalf("installments mismatch: %v", p.Installments)
	}
	if !p.HasRequestedAt {
		t.Fatalf("expected requested_at parsed")
	}
	expectedUnix := time.Date(2026, 3, 11, 3, 45, 53, 0, time.UTC).Unix()
	if p.RequestedAt.UnixSeconds != expectedUnix {
		t.Fatalf("unix mismatch: got %d want %d", p.RequestedAt.UnixSeconds, expectedUnix)
	}
	if p.RequestedAt.Hour != 3 {
		t.Fatalf("hour mismatch: %d", p.RequestedAt.Hour)
	}
	// 2026-03-11 was a Wednesday → Monday-zero=2.
	if p.RequestedAt.WeekdayMon0 != 2 {
		t.Fatalf("weekday mismatch: %d", p.RequestedAt.WeekdayMon0)
	}
	if p.AvgAmount != 209.74 || p.TxCount24h != 13 {
		t.Fatalf("customer mismatch: %v %v", p.AvgAmount, p.TxCount24h)
	}
	if len(p.KnownMerchants) != 2 ||
		!bytes.Equal(p.KnownMerchants[0], []byte("MERC-003")) ||
		!bytes.Equal(p.KnownMerchants[1], []byte("MERC-016")) {
		t.Fatalf("known_merchants mismatch: %q", p.KnownMerchants)
	}
	if !bytes.Equal(p.MerchantID, []byte("MERC-089")) {
		t.Fatalf("merchant id mismatch: %q", p.MerchantID)
	}
	if !bytes.Equal(p.MCC, []byte("7801")) {
		t.Fatalf("mcc mismatch: %q", p.MCC)
	}
	if p.MerchantAvgAmount != 25.15 {
		t.Fatalf("merchant avg_amount mismatch: %v", p.MerchantAvgAmount)
	}
	if p.IsOnline || !p.CardPresent {
		t.Fatalf("terminal flags mismatch")
	}
	if p.KmFromHome != 667.7296579973 {
		t.Fatalf("km mismatch: %v", p.KmFromHome)
	}
	if p.HasLastTransaction {
		t.Fatalf("expected no last_transaction")
	}
}

func TestParseLastTransactionPresent(t *testing.T) {
	body := []byte(`{"transaction":{"amount":1,"installments":1,"requested_at":"2026-03-11T03:45:53Z"},"customer":{"avg_amount":0,"tx_count_24h":0,"known_merchants":[]},"merchant":{"id":"M","mcc":"5411","avg_amount":1},"terminal":{"is_online":true,"card_present":false,"km_from_home":0},"last_transaction":{"timestamp":"2026-03-11T02:30:00Z","km_from_current":42.5}}`)
	var p Payload
	if err := Parse(body, &p); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !p.HasLastTransaction {
		t.Fatalf("expected last_transaction present")
	}
	if p.LastKmFromCurrent != 42.5 {
		t.Fatalf("last km mismatch: %v", p.LastKmFromCurrent)
	}
	expected := time.Date(2026, 3, 11, 2, 30, 0, 0, time.UTC).Unix()
	if p.LastTimestamp.UnixSeconds != expected {
		t.Fatalf("last unix mismatch: %d vs %d", p.LastTimestamp.UnixSeconds, expected)
	}
	if len(p.KnownMerchants) != 0 {
		t.Fatalf("expected empty known_merchants, got %v", p.KnownMerchants)
	}
}

func TestParseFractionalTimestamp(t *testing.T) {
	body := []byte(`{"transaction":{"amount":1.5,"installments":1,"requested_at":"2026-03-11T03:45:53.250Z"},"customer":{"avg_amount":0,"tx_count_24h":0,"known_merchants":[]},"merchant":{"id":"M","mcc":"5411","avg_amount":1},"terminal":{"is_online":true,"card_present":false,"km_from_home":0},"last_transaction":null}`)
	var p Payload
	if err := Parse(body, &p); err != nil {
		t.Fatalf("parse: %v", err)
	}
	expected := time.Date(2026, 3, 11, 3, 45, 53, 0, time.UTC).Unix()
	if p.RequestedAt.UnixSeconds != expected {
		t.Fatalf("unix mismatch: %d", p.RequestedAt.UnixSeconds)
	}
}

func TestParseRejectsScientific(t *testing.T) {
	body := []byte(`{"transaction":{"amount":1e2,"installments":1,"requested_at":"2026-03-11T03:45:53Z"},"customer":{"avg_amount":0,"tx_count_24h":0,"known_merchants":[]},"merchant":{"id":"M","mcc":"5411","avg_amount":1},"terminal":{"is_online":true,"card_present":false,"km_from_home":0},"last_transaction":null}`)
	var p Payload
	if err := Parse(body, &p); err == nil {
		t.Fatalf("expected error for scientific notation")
	}
}

func TestParseRejectsEscapedString(t *testing.T) {
	body := []byte("{\"transaction\":{\"amount\":1,\"installments\":1,\"requested_at\":\"2026-03-11T03:45:53Z\"},\"customer\":{\"avg_amount\":0,\"tx_count_24h\":0,\"known_merchants\":[]},\"merchant\":{\"id\":\"M\\u00e9\",\"mcc\":\"5411\",\"avg_amount\":1},\"terminal\":{\"is_online\":true,\"card_present\":false,\"km_from_home\":0},\"last_transaction\":null}")
	var p Payload
	if err := Parse(body, &p); err == nil {
		t.Fatalf("expected error on escaped merchant id")
	}
}

func TestParseIgnoresUnknownTopLevelKeys(t *testing.T) {
	body := []byte(`{"id":"tx-1","transaction":{"amount":1.5,"installments":1,"requested_at":"2026-03-11T03:45:53Z"},"extra":{"any":[1,2,3],"more":{"nested":true}},"customer":{"avg_amount":0,"tx_count_24h":0,"known_merchants":[]},"merchant":{"id":"M","mcc":"5411","avg_amount":1},"terminal":{"is_online":true,"card_present":false,"km_from_home":0},"last_transaction":null}`)
	var p Payload
	if err := Parse(body, &p); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.Amount != 1.5 {
		t.Fatalf("amount mismatch: %v", p.Amount)
	}
}

func TestParsePayloadResetReusesSlice(t *testing.T) {
	var p Payload
	body := []byte(`{"transaction":{"amount":1,"installments":1,"requested_at":"2026-03-11T03:45:53Z"},"customer":{"avg_amount":0,"tx_count_24h":0,"known_merchants":["A","B","C","D","E","F"]},"merchant":{"id":"M","mcc":"5411","avg_amount":1},"terminal":{"is_online":true,"card_present":false,"km_from_home":0},"last_transaction":null}`)
	if err := Parse(body, &p); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(p.KnownMerchants) != 6 {
		t.Fatalf("expected 6 known_merchants, got %d", len(p.KnownMerchants))
	}
	cap1 := cap(p.KnownMerchants)
	p.Reset()
	if len(p.KnownMerchants) != 0 {
		t.Fatalf("Reset should keep len 0, got %d", len(p.KnownMerchants))
	}
	if cap(p.KnownMerchants) != cap1 {
		t.Fatalf("Reset dropped capacity (%d vs %d)", cap(p.KnownMerchants), cap1)
	}
	body2 := []byte(`{"transaction":{"amount":2,"installments":2,"requested_at":"2026-03-11T03:45:53Z"},"customer":{"avg_amount":0,"tx_count_24h":0,"known_merchants":["X","Y"]},"merchant":{"id":"M","mcc":"5411","avg_amount":1},"terminal":{"is_online":true,"card_present":false,"km_from_home":0},"last_transaction":null}`)
	if err := Parse(body2, &p); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(p.KnownMerchants) != 2 || cap(p.KnownMerchants) < cap1 {
		t.Fatalf("Reset didn't reuse capacity")
	}
}

// TestParseEquivalenceAgainstEncodingJSON walks a sample of the official
// test dataset and checks fastjson against encoding/json for every field
// the vectorizer reads.
func TestParseEquivalenceAgainstEncodingJSON(t *testing.T) {
	path := findTestDataJSON(t)
	if path == "" {
		t.Skip("test-data.json not available")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read test-data: %v", err)
	}
	var doc struct {
		Entries []struct {
			Request json.RawMessage `json:"request"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decode test-data: %v", err)
	}
	if len(doc.Entries) == 0 {
		t.Fatalf("test-data has no entries")
	}

	step := len(doc.Entries) / 1000
	if step < 1 {
		step = 1
	}
	checked := 0
	for i := 0; i < len(doc.Entries); i += step {
		body := []byte(doc.Entries[i].Request)
		var got Payload
		if err := Parse(body, &got); err != nil {
			t.Fatalf("entry %d: fastjson Parse: %v\nbody=%s", i, err, body)
		}
		var ref referenceRequest
		if err := json.Unmarshal(body, &ref); err != nil {
			t.Fatalf("entry %d: encoding/json: %v", i, err)
		}
		if got.Amount != ref.Transaction.Amount {
			t.Fatalf("entry %d: amount %v vs %v", i, got.Amount, ref.Transaction.Amount)
		}
		if got.Installments != ref.Transaction.Installments {
			t.Fatalf("entry %d: installments %d vs %d", i, got.Installments, ref.Transaction.Installments)
		}
		expectedUnix := ref.Transaction.RequestedAt.UTC().Unix()
		if got.RequestedAt.UnixSeconds != expectedUnix {
			t.Fatalf("entry %d: requested_at unix %d vs %d", i, got.RequestedAt.UnixSeconds, expectedUnix)
		}
		expectedHour := ref.Transaction.RequestedAt.UTC().Hour()
		if got.RequestedAt.Hour != expectedHour {
			t.Fatalf("entry %d: hour %d vs %d", i, got.RequestedAt.Hour, expectedHour)
		}
		expectedDow := (int(ref.Transaction.RequestedAt.UTC().Weekday()) + 6) % 7
		if got.RequestedAt.WeekdayMon0 != expectedDow {
			t.Fatalf("entry %d: weekday %d vs %d", i, got.RequestedAt.WeekdayMon0, expectedDow)
		}
		if got.AvgAmount != ref.Customer.AvgAmount || got.TxCount24h != ref.Customer.TxCount24h {
			t.Fatalf("entry %d: customer mismatch", i)
		}
		if len(got.KnownMerchants) != len(ref.Customer.KnownMerchants) {
			t.Fatalf("entry %d: known_merchants len %d vs %d", i, len(got.KnownMerchants), len(ref.Customer.KnownMerchants))
		}
		for j, m := range ref.Customer.KnownMerchants {
			if string(got.KnownMerchants[j]) != m {
				t.Fatalf("entry %d: known_merchants[%d] %q vs %q", i, j, got.KnownMerchants[j], m)
			}
		}
		if string(got.MerchantID) != ref.Merchant.ID {
			t.Fatalf("entry %d: merchant id %q vs %q", i, got.MerchantID, ref.Merchant.ID)
		}
		if string(got.MCC) != ref.Merchant.MCC {
			t.Fatalf("entry %d: mcc %q vs %q", i, got.MCC, ref.Merchant.MCC)
		}
		if got.MerchantAvgAmount != ref.Merchant.AvgAmount {
			t.Fatalf("entry %d: merchant avg %v vs %v", i, got.MerchantAvgAmount, ref.Merchant.AvgAmount)
		}
		if got.IsOnline != ref.Terminal.IsOnline ||
			got.CardPresent != ref.Terminal.CardPresent ||
			got.KmFromHome != ref.Terminal.KmFromHome {
			t.Fatalf("entry %d: terminal mismatch", i)
		}
		if (ref.LastTransaction == nil) == got.HasLastTransaction {
			t.Fatalf("entry %d: last_transaction presence mismatch", i)
		}
		if ref.LastTransaction != nil {
			if got.LastTimestamp.UnixSeconds != ref.LastTransaction.Timestamp.UTC().Unix() {
				t.Fatalf("entry %d: last_transaction timestamp mismatch", i)
			}
			if got.LastKmFromCurrent != ref.LastTransaction.KmFromCurrent {
				t.Fatalf("entry %d: last_transaction km mismatch", i)
			}
		}
		checked++
	}
	if checked < 50 {
		t.Fatalf("expected to check at least 50 entries, got %d", checked)
	}
	t.Logf("verified %d entries", checked)
}

func findTestDataJSON(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"../../../test/test-data.json",
		"../../test/test-data.json",
		"../../../../test/test-data.json",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(`{"transaction":{"amount":1.5,"installments":1,"requested_at":"2026-03-11T03:45:53Z"},"customer":{"avg_amount":0,"tx_count_24h":0,"known_merchants":[]},"merchant":{"id":"M","mcc":"5411","avg_amount":1},"terminal":{"is_online":true,"card_present":false,"km_from_home":0},"last_transaction":null}`))
	f.Fuzz(func(t *testing.T, body []byte) {
		var p Payload
		_ = Parse(body, &p)
	})
}
