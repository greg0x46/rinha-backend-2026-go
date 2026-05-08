package httpapi

import (
	"bytes"

	"github.com/greg/rinha-be-2026/internal/fastjson"
)

// VectorizeFromPayload mirrors Vectorize but consumes the typed Payload
// produced by the fastjson parser, avoiding an intermediate
// FraudScoreRequest. The output must match Vectorize bit-for-bit so the
// scorer's decisions are identical regardless of decode path.
func (v Vectorizer) VectorizeFromPayload(p *fastjson.Payload) Vector {
	vector := Vector{
		float32(Clamp(p.Amount*v.invMaxAmount, 0, 1)),
		float32(Clamp(float64(p.Installments)*v.invMaxInstallments, 0, 1)),
		float32(amountVsAvg(p.Amount, p.AvgAmount, v.invAmountVsAvgRatio)),
		float32(float64(p.RequestedAt.Hour) / 23),
		float32(float64(p.RequestedAt.WeekdayMon0) / 6),
		-1,
		-1,
		float32(Clamp(p.KmFromHome*v.invMaxKm, 0, 1)),
		float32(Clamp(float64(p.TxCount24h)*v.invMaxTxCount24h, 0, 1)),
		boolFloat(p.IsOnline),
		boolFloat(p.CardPresent),
		boolFloat(!knownMerchantBytes(p.MerchantID, p.KnownMerchants)),
		v.mccRisk.ForBytes(p.MCC),
		float32(Clamp(p.MerchantAvgAmount*v.invMaxMerchantAvgAmount, 0, 1)),
	}

	if p.HasLastTransaction {
		diffSeconds := p.RequestedAt.UnixSeconds - p.LastTimestamp.UnixSeconds
		minutes := float64(diffSeconds) / 60
		vector[5] = float32(Clamp(minutes*v.invMaxMinutes, 0, 1))
		vector[6] = float32(Clamp(p.LastKmFromCurrent*v.invMaxKm, 0, 1))
	}

	return vector
}

func knownMerchantBytes(id []byte, knownMerchants [][]byte) bool {
	for _, known := range knownMerchants {
		if bytes.Equal(known, id) {
			return true
		}
	}
	return false
}
