package httpapi

import (
	"github.com/greg/rinha-be-2026/internal/frauddata"
	"github.com/greg/rinha-be-2026/internal/fraudindex"
)

type Vector = fraudindex.Vector

type Vectorizer struct {
	normalization frauddata.Normalization
	mccRisk       frauddata.MCCRisk
}

func NewVectorizer() (Vectorizer, error) {
	normalization, err := frauddata.LoadNormalization()
	if err != nil {
		return Vectorizer{}, err
	}
	mccRisk, err := frauddata.LoadMCCRisk()
	if err != nil {
		return Vectorizer{}, err
	}
	return Vectorizer{
		normalization: normalization,
		mccRisk:       mccRisk,
	}, nil
}

func (v Vectorizer) Vectorize(request FraudScoreRequest) Vector {
	n := v.normalization

	requestedAt := request.Transaction.RequestedAt.UTC()
	vector := Vector{
		float32(Clamp(request.Transaction.Amount/n.MaxAmount, 0, 1)),
		float32(Clamp(float64(request.Transaction.Installments)/n.MaxInstallments, 0, 1)),
		float32(amountVsAvg(request.Transaction.Amount, request.Customer.AvgAmount, n.AmountVsAvgRatio)),
		float32(float64(requestedAt.Hour()) / 23),
		float32(float64(dayOfWeekMondayZero(int(requestedAt.Weekday()))) / 6),
		-1,
		-1,
		float32(Clamp(request.Terminal.KmFromHome/n.MaxKm, 0, 1)),
		float32(Clamp(float64(request.Customer.TxCount24h)/n.MaxTxCount24h, 0, 1)),
		boolFloat(request.Terminal.IsOnline),
		boolFloat(request.Terminal.CardPresent),
		boolFloat(!knownMerchant(request.Merchant.ID, request.Customer.KnownMerchants)),
		float32(v.mccRisk.For(request.Merchant.MCC)),
		float32(Clamp(request.Merchant.AvgAmount/n.MaxMerchantAvgAmount, 0, 1)),
	}

	if request.LastTransaction != nil {
		lastAt := request.LastTransaction.Timestamp.UTC()
		minutes := requestedAt.Sub(lastAt).Minutes()
		vector[5] = float32(Clamp(minutes/n.MaxMinutes, 0, 1))
		vector[6] = float32(Clamp(request.LastTransaction.KmFromCurrent/n.MaxKm, 0, 1))
	}

	return vector
}

func Clamp(value, minValue, maxValue float64) float64 {
	return min(max(value, minValue), maxValue)
}

func amountVsAvg(amount, avgAmount, ratio float64) float64 {
	if avgAmount <= 0 || ratio <= 0 {
		if amount > 0 {
			return 1
		}
		return 0
	}
	return Clamp((amount/avgAmount)/ratio, 0, 1)
}

func dayOfWeekMondayZero(day int) int {
	return (day + 6) % 7
}

func boolFloat(value bool) float32 {
	if value {
		return 1
	}
	return 0
}

func knownMerchant(id string, knownMerchants []string) bool {
	for _, known := range knownMerchants {
		if known == id {
			return true
		}
	}
	return false
}
