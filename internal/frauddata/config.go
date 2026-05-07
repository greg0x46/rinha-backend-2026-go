package frauddata

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed normalization.json
var normalizationJSON []byte

//go:embed mcc_risk.json
var mccRiskJSON []byte

type Normalization struct {
	MaxAmount            float64 `json:"max_amount"`
	MaxInstallments      float64 `json:"max_installments"`
	AmountVsAvgRatio     float64 `json:"amount_vs_avg_ratio"`
	MaxMinutes           float64 `json:"max_minutes"`
	MaxKm                float64 `json:"max_km"`
	MaxTxCount24h        float64 `json:"max_tx_count_24h"`
	MaxMerchantAvgAmount float64 `json:"max_merchant_avg_amount"`
}

const mccTableSize = 10000
const mccDefaultRisk float32 = 0.5

type MCCRisk struct {
	table [mccTableSize]float32
}

func LoadNormalization() (Normalization, error) {
	var normalization Normalization
	if err := json.Unmarshal(normalizationJSON, &normalization); err != nil {
		return Normalization{}, fmt.Errorf("decode embedded normalization.json: %w", err)
	}
	return normalization, nil
}

func LoadMCCRisk() (*MCCRisk, error) {
	var raw map[string]float64
	if err := json.Unmarshal(mccRiskJSON, &raw); err != nil {
		return nil, fmt.Errorf("decode embedded mcc_risk.json: %w", err)
	}
	risk := &MCCRisk{}
	for i := range risk.table {
		risk.table[i] = mccDefaultRisk
	}
	for k, v := range raw {
		idx, ok := parseMCC(k)
		if !ok {
			continue
		}
		risk.table[idx] = float32(v)
	}
	return risk, nil
}

func (r *MCCRisk) For(mcc string) float32 {
	idx, ok := parseMCC(mcc)
	if !ok {
		return mccDefaultRisk
	}
	return r.table[idx]
}

func parseMCC(s string) (int, bool) {
	if len(s) == 0 || len(s) > 4 {
		return 0, false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}
