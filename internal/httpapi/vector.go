package httpapi

import (
	"github.com/greg/rinha-be-2026/internal/frauddata"
	"github.com/greg/rinha-be-2026/internal/fraudindex"
)

type Vector = fraudindex.Vector

type Vectorizer struct {
	mccRisk frauddata.MCCRisk

	invMaxAmount            float64
	invMaxInstallments      float64
	invAmountVsAvgRatio     float64
	invMaxMinutes           float64
	invMaxKm                float64
	invMaxTxCount24h        float64
	invMaxMerchantAvgAmount float64
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
		mccRisk:                 mccRisk,
		invMaxAmount:            inv(normalization.MaxAmount),
		invMaxInstallments:      inv(normalization.MaxInstallments),
		invAmountVsAvgRatio:     inv(normalization.AmountVsAvgRatio),
		invMaxMinutes:           inv(normalization.MaxMinutes),
		invMaxKm:                inv(normalization.MaxKm),
		invMaxTxCount24h:        inv(normalization.MaxTxCount24h),
		invMaxMerchantAvgAmount: inv(normalization.MaxMerchantAvgAmount),
	}, nil
}

func inv(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return 1 / value
}

func (v Vectorizer) Vectorize(request FraudScoreRequest) Vector {
	requestedAt := request.Transaction.RequestedAt.UTC()
	vector := Vector{
		float32(Clamp(request.Transaction.Amount*v.invMaxAmount, 0, 1)),
		float32(Clamp(float64(request.Transaction.Installments)*v.invMaxInstallments, 0, 1)),
		float32(amountVsAvg(request.Transaction.Amount, request.Customer.AvgAmount, v.invAmountVsAvgRatio)),
		float32(float64(requestedAt.Hour()) / 23),
		float32(float64(dayOfWeekMondayZero(int(requestedAt.Weekday()))) / 6),
		-1,
		-1,
		float32(Clamp(request.Terminal.KmFromHome*v.invMaxKm, 0, 1)),
		float32(Clamp(float64(request.Customer.TxCount24h)*v.invMaxTxCount24h, 0, 1)),
		boolFloat(request.Terminal.IsOnline),
		boolFloat(request.Terminal.CardPresent),
		boolFloat(!knownMerchant(request.Merchant.ID, request.Customer.KnownMerchants)),
		float32(v.mccRisk.For(request.Merchant.MCC)),
		float32(Clamp(request.Merchant.AvgAmount*v.invMaxMerchantAvgAmount, 0, 1)),
	}

	if request.LastTransaction != nil {
		lastAt := request.LastTransaction.Timestamp.UTC()
		minutes := requestedAt.Sub(lastAt).Minutes()
		vector[5] = float32(Clamp(minutes*v.invMaxMinutes, 0, 1))
		vector[6] = float32(Clamp(request.LastTransaction.KmFromCurrent*v.invMaxKm, 0, 1))
	}

	return vector
}

func Clamp(value, minValue, maxValue float64) float64 {
	return min(max(value, minValue), maxValue)
}

func amountVsAvg(amount, avgAmount, invRatio float64) float64 {
	if avgAmount <= 0 || invRatio <= 0 {
		if amount > 0 {
			return 1
		}
		return 0
	}
	return Clamp((amount/avgAmount)*invRatio, 0, 1)
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
