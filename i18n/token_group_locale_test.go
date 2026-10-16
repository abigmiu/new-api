package i18n

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenGroupMessagesCannotLeakChannelIdentity protects the hard requirement
// that an error returned to a managed key's owner never names the upstream
// channel serving it. The guarantee comes from these templates carrying no
// placeholders at all: whatever a call site passes as template data, the
// rendered message stays the same, so a channel id, name, or URL cannot reach
// the response body even by accident.
func TestTokenGroupMessagesCannotLeakChannelIdentity(t *testing.T) {
	require.NoError(t, Init())

	hostile := map[string]any{
		"channel":      "uo-12-ab",
		"channel_name": "渠道12-gpt-4o",
		"id":           12,
		"max":          12,
		"url":          "http://upstream.example.com",
	}
	digit := regexp.MustCompile(`\d`)

	for _, lang := range []string{LangEn, LangZhCN, LangZhTW} {
		for _, key := range []string{
			MsgTokenGroupPriceChanged,
			MsgTokenGroupChannelRemoved,
			MsgTokenGroupUnavailable,
		} {
			message := Translate(lang, key, hostile)
			require.NotEqual(t, key, message, "%s is missing from the %s bundle", key, lang)
			require.NotEmpty(t, message)

			assert.Equal(t, Translate(lang, key), message,
				"%s must ignore template data in %s", key, lang)
			assert.NotContains(t, message, "http", "%s leaks a URL in %s", key, lang)
			assert.NotContains(t, message, "uo-", "%s leaks a local group name in %s", key, lang)
			assert.False(t, digit.MatchString(message),
				"%s leaks a numeric channel identifier in %s: %q", key, lang, message)
		}
	}
}
