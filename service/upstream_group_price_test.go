package service

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyUpstreamGroupMarkupRoundsUpToThousandth(t *testing.T) {
	markup := decimal.RequireFromString("1.18")
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "fractional thousandth", source: "0.075", want: "0.089"},
		{name: "exact thousandth", source: "1", want: "1.180"},
		{name: "equivalent decimal", source: "0.10", want: "0.118"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ratio, err := decimal.NewFromString(test.source)
			require.NoError(t, err)
			assert.Equal(t, test.want, applyUpstreamGroupMarkup(ratio, markup))
		})
	}
}

func TestCalculateUpstreamGroupSaleRatioMarkupWithinRange(t *testing.T) {
	min := decimal.RequireFromString("1.150")
	max := decimal.RequireFromString("1.200")

	seen := make(map[string]struct{})
	for i := 0; i < 200; i++ {
		got, err := CalculateUpstreamGroupSaleRatio("1")
		require.NoError(t, err)

		value, err := decimal.NewFromString(got)
		require.NoError(t, err)
		assert.True(t, value.GreaterThanOrEqual(min), "markup below 15%%: %s", got)
		assert.True(t, value.LessThanOrEqual(max), "markup above 20%%: %s", got)

		seen[got] = struct{}{}
	}

	assert.Greater(t, len(seen), 1, "markup should vary between calls")
}

func TestCalculateUpstreamGroupSaleRatioRejectsInvalidValue(t *testing.T) {
	for _, source := range []string{"", "0", "-1", "invalid"} {
		_, err := CalculateUpstreamGroupSaleRatio(source)
		assert.Error(t, err)
	}
}
