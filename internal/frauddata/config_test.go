package frauddata

import "testing"

func TestLoadNormalization(t *testing.T) {
	normalization, err := LoadNormalization()
	if err != nil {
		t.Fatalf("LoadNormalization failed: %v", err)
	}

	if normalization.MaxAmount != 10000 {
		t.Fatalf("MaxAmount = %v, want 10000", normalization.MaxAmount)
	}
	if normalization.MaxInstallments != 12 {
		t.Fatalf("MaxInstallments = %v, want 12", normalization.MaxInstallments)
	}
	if normalization.AmountVsAvgRatio != 10 {
		t.Fatalf("AmountVsAvgRatio = %v, want 10", normalization.AmountVsAvgRatio)
	}
	if normalization.MaxMinutes != 1440 {
		t.Fatalf("MaxMinutes = %v, want 1440", normalization.MaxMinutes)
	}
	if normalization.MaxKm != 1000 {
		t.Fatalf("MaxKm = %v, want 1000", normalization.MaxKm)
	}
	if normalization.MaxTxCount24h != 20 {
		t.Fatalf("MaxTxCount24h = %v, want 20", normalization.MaxTxCount24h)
	}
	if normalization.MaxMerchantAvgAmount != 10000 {
		t.Fatalf("MaxMerchantAvgAmount = %v, want 10000", normalization.MaxMerchantAvgAmount)
	}
}

func TestLoadMCCRisk(t *testing.T) {
	risk, err := LoadMCCRisk()
	if err != nil {
		t.Fatalf("LoadMCCRisk failed: %v", err)
	}

	if got := risk.For("5411"); got != float32(0.15) {
		t.Fatalf("risk.For(5411) = %v, want 0.15", got)
	}
	if got := risk.For("7995"); got != float32(0.85) {
		t.Fatalf("risk.For(7995) = %v, want 0.85", got)
	}
	if got := risk.For("0000"); got != float32(0.5) {
		t.Fatalf("risk.For(0000) = %v, want 0.5", got)
	}
	if got := risk.For("abcd"); got != float32(0.5) {
		t.Fatalf("risk.For(abcd) = %v, want 0.5 (parse fail fallback)", got)
	}
	if got := risk.For("12345"); got != float32(0.5) {
		t.Fatalf("risk.For(12345) = %v, want 0.5 (out of range fallback)", got)
	}
}
