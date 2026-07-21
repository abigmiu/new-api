package service

import (
	"encoding/base64"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitChannelAliasReadsEnvironmentKey(t *testing.T) {
	original := channelAliasKey
	channelAliasKey = nil
	t.Cleanup(func() { channelAliasKey = original })

	t.Setenv(channelAliasKeyEnv, "")
	assert.Error(t, InitChannelAlias())
	t.Setenv(channelAliasKeyEnv, "invalid")
	assert.Error(t, InitChannelAlias())
	t.Setenv(channelAliasKeyEnv, base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")))
	require.NoError(t, InitChannelAlias())
	assert.Equal(t, []byte("0123456789abcdef"), channelAliasKey)
}

func setChannelAliasKeyForTest(t *testing.T) {
	t.Helper()
	original := channelAliasKey
	channelAliasKey = []byte("0123456789abcdef")
	t.Cleanup(func() { channelAliasKey = original })
}

func TestChannelAliasRoundTripAndGroupIsolation(t *testing.T) {
	setChannelAliasKeyForTest(t)

	defaultAlias, err := EncryptChannelAlias("default", 12)
	require.NoError(t, err)
	secondDefaultAlias, err := EncryptChannelAlias("default", 12)
	require.NoError(t, err)
	vipAlias, err := EncryptChannelAlias("vip", 12)
	require.NoError(t, err)

	assert.Equal(t, defaultAlias, secondDefaultAlias)
	assert.NotEqual(t, defaultAlias, vipAlias)
	assert.Regexp(t, regexp.MustCompile(`^[0-9A-Z]{6}$`), defaultAlias)

	channelID, err := DecryptChannelAlias("default", defaultAlias)
	require.NoError(t, err)
	assert.Equal(t, 12, channelID)
}

func TestChannelAliasRejectsInvalidInputs(t *testing.T) {
	setChannelAliasKeyForTest(t)

	_, err := EncryptChannelAlias("", 1)
	assert.Error(t, err)
	_, err = EncryptChannelAlias("default", 0)
	assert.Error(t, err)
	_, err = EncryptChannelAlias("default", int(channelAliasCapacity))
	assert.Error(t, err)
	_, err = DecryptChannelAlias("default", "ABC")
	assert.Error(t, err)
	_, err = DecryptChannelAlias("default", "ABC-12")
	assert.Error(t, err)

	channelAliasKey = nil
	_, err = EncryptChannelAlias("default", 1)
	assert.Error(t, err)
}
