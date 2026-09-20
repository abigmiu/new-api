package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateUpstreamGroupSaleRatioRoundsUpToThousandth(t *testing.T) {
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
			got, err := CalculateUpstreamGroupSaleRatio(test.source)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestCalculateUpstreamGroupSaleRatioRejectsInvalidValue(t *testing.T) {
	for _, source := range []string{"", "0", "-1", "invalid"} {
		_, err := CalculateUpstreamGroupSaleRatio(source)
		assert.Error(t, err)
	}
}
