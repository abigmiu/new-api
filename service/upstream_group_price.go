package service

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"strings"

	"github.com/shopspring/decimal"
)

// The upstream group sale price adds a surcharge on top of the source ratio.
// The markup is drawn from [1.150, 1.200] in per-mille steps, derived from the
// group identity rather than sampled at random: the sync runs repeatedly, and a
// fresh draw each round would silently move the user's price even while the
// upstream price is unchanged.
const (
	upstreamGroupMarkupMinPermille = 1150
	upstreamGroupMarkupMaxPermille = 1200
)

func upstreamGroupMarkupForIdentity(identity string) decimal.Decimal {
	sum := sha256.Sum256([]byte(identity))
	span := uint64(upstreamGroupMarkupMaxPermille - upstreamGroupMarkupMinPermille + 1)
	permille := uint64(upstreamGroupMarkupMinPermille) + binary.BigEndian.Uint64(sum[:8])%span
	return decimal.NewFromInt(int64(permille)).Div(decimal.NewFromInt(1000))
}

func CalculateUpstreamGroupSaleRatio(source string, identity string) (string, error) {
	ratio, err := decimal.NewFromString(strings.TrimSpace(source))
	if err != nil || ratio.LessThanOrEqual(decimal.Zero) {
		return "", errors.New("source ratio must be a positive decimal")
	}
	return applyUpstreamGroupMarkup(ratio, upstreamGroupMarkupForIdentity(identity)), nil
}

func applyUpstreamGroupMarkup(ratio, markup decimal.Decimal) string {
	return ratio.Mul(markup).Mul(decimal.NewFromInt(1000)).Ceil().Div(decimal.NewFromInt(1000)).StringFixed(3)
}
