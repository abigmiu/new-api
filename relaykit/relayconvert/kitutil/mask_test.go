package kitutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskSensitiveInfoMasksURLsAndDomains(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "url keeps scheme and tld only",
			input: "upstream failed: https://api.example.com/v1/chat?key=secret",
			want:  "upstream failed: https://***.com/***/***?key=***",
		},
		{
			name:  "bare domain is masked",
			input: "connect to openai.com failed",
			want:  "connect to ***.com failed",
		},
		{
			name:  "ip is masked",
			input: "dial 192.168.1.1 refused",
			want:  "dial ***.***.***.*** refused",
		},
		{
			name:  "api key value is masked",
			input: `api_key:sk-live-abcdef123456`,
			want:  `api_key:***`,
		},
		{
			name:  "managed group key is masked",
			input: "no channel available for group uo-12-ab12cd",
			want:  "no channel available for group uo-***",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, MaskSensitiveInfo(test.input))
		})
	}
}
