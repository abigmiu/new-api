package service

import (
	"fmt"
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
		identity := fmt.Sprintf("newapi:1:%d", i)
		got, err := CalculateUpstreamGroupSaleRatio("1", identity)
		require.NoError(t, err)

		value, err := decimal.NewFromString(got)
		require.NoError(t, err)
		assert.True(t, value.GreaterThanOrEqual(min), "markup below 15%%: %s", got)
		assert.True(t, value.LessThanOrEqual(max), "markup above 20%%: %s", got)

		seen[got] = struct{}{}
	}

	assert.Greater(t, len(seen), 1, "markup should vary between identities")
}

// The sync re-derives the sale ratio on every run, so the markup must be a pure
// function of the group identity: otherwise the user's price drifts while the
// upstream price stands still.
func TestCalculateUpstreamGroupSaleRatioIsStablePerIdentity(t *testing.T) {
	first, err := CalculateUpstreamGroupSaleRatio("0.075", "newapi:1:pro")
	require.NoError(t, err)
	for i := 0; i < 20; i++ {
		again, err := CalculateUpstreamGroupSaleRatio("0.075", "newapi:1:pro")
		require.NoError(t, err)
		assert.Equal(t, first, again)
	}

	// A different source ratio for the same group changes the price, but the
	// markup applied is the group's own, so the sale ratio stays proportional.
	raised, err := CalculateUpstreamGroupSaleRatio("0.150", "newapi:1:pro")
	require.NoError(t, err)
	assert.NotEqual(t, first, raised)
}

func TestCalculateUpstreamGroupSaleRatioRejectsInvalidValue(t *testing.T) {
	for _, source := range []string{"", "0", "-1", "invalid"} {
		_, err := CalculateUpstreamGroupSaleRatio(source, "newapi:1:pro")
		assert.Error(t, err)
	}
}
