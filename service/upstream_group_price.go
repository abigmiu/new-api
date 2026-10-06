package service

import (
	"errors"
	"math/rand"
	"strings"

	"github.com/shopspring/decimal"
)

// The upstream group sale price adds a random surcharge on top of the source
// ratio. The markup is drawn uniformly from [1.150, 1.200] in per-mille steps.
const (
	upstreamGroupMarkupMinPermille = 1150
	upstreamGroupMarkupMaxPermille = 1200
)

func randomUpstreamGroupMarkup() decimal.Decimal {
	permille := upstreamGroupMarkupMinPermille +
		rand.Intn(upstreamGroupMarkupMaxPermille-upstreamGroupMarkupMinPermille+1)
	return decimal.NewFromInt(int64(permille)).Div(decimal.NewFromInt(1000))
}

func CalculateUpstreamGroupSaleRatio(source string) (string, error) {
	ratio, err := decimal.NewFromString(strings.TrimSpace(source))
	if err != nil || ratio.LessThanOrEqual(decimal.Zero) {
		return "", errors.New("source ratio must be a positive decimal")
	}
	return applyUpstreamGroupMarkup(ratio, randomUpstreamGroupMarkup()), nil
}

func applyUpstreamGroupMarkup(ratio, markup decimal.Decimal) string {
	return ratio.Mul(markup).Mul(decimal.NewFromInt(1000)).Ceil().Div(decimal.NewFromInt(1000)).StringFixed(3)
}
