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

type MCCRisk map[string]float64

func LoadNormalization() (Normalization, error) {
	var normalization Normalization
	if err := json.Unmarshal(normalizationJSON, &normalization); err != nil {
		return Normalization{}, fmt.Errorf("decode embedded normalization.json: %w", err)
	}
	return normalization, nil
}

func LoadMCCRisk() (MCCRisk, error) {
	var risk MCCRisk
	if err := json.Unmarshal(mccRiskJSON, &risk); err != nil {
		return nil, fmt.Errorf("decode embedded mcc_risk.json: %w", err)
	}
	return risk, nil
}

func (r MCCRisk) For(mcc string) float64 {
	if value, ok := r[mcc]; ok {
		return value
	}
	return 0.5
}
