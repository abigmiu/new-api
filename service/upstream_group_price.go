package service

import (
	"errors"
	"strings"

	"github.com/shopspring/decimal"
)

var upstreamGroupMarkup = decimal.RequireFromString("1.18")

func CalculateUpstreamGroupSaleRatio(source string) (string, error) {
	ratio, err := decimal.NewFromString(strings.TrimSpace(source))
	if err != nil || ratio.LessThanOrEqual(decimal.Zero) {
		return "", errors.New("source ratio must be a positive decimal")
	}
	return ratio.Mul(upstreamGroupMarkup).Mul(decimal.NewFromInt(1000)).Ceil().Div(decimal.NewFromInt(1000)).StringFixed(3), nil
}
