package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestShouldStopAfterSelectedChannelRetries(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*gin.Context)
		expected bool
	}{
		{
			name:     "randomly selected channel may switch",
			expected: false,
		},
		{
			name: "token specific channel stops",
			setup: func(c *gin.Context) {
				c.Set(string(constant.ContextKeyTokenSpecificChannelId), "1")
			},
			expected: true,
		},
		{
			name: "user preferred channel stops",
			setup: func(c *gin.Context) {
				common.SetContextKey(c, constant.ContextKeyUserPreferredChannel, true)
			},
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(nil)
			if test.setup != nil {
				test.setup(c)
			}
			assert.Equal(t, test.expected, shouldStopAfterSelectedChannelRetries(c))
		})
	}
}
