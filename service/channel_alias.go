package service

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/capitalone/fpe/ff1"
)

const (
	channelAliasKeyEnv   = "CHANNEL_ALIAS_KEY"
	channelAliasLength   = 6
	channelAliasCapacity = uint64(36 * 36 * 36 * 36 * 36 * 36)
)

var channelAliasKey []byte

func InitChannelAlias() error {
	encoded := strings.TrimSpace(os.Getenv(channelAliasKeyEnv))
	if encoded == "" {
		return fmt.Errorf("%s is required", channelAliasKeyEnv)
	}
	key, err := decodeChannelAliasKey(encoded)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", channelAliasKeyEnv, err)
	}
	channelAliasKey = key
	return nil
}

func EncryptChannelAlias(group string, channelID int) (string, error) {
	key, tweak, err := channelAliasInputs(group)
	if err != nil {
		return "", err
	}
	if channelID <= 0 || uint64(channelID) >= channelAliasCapacity {
		return "", errors.New("channel id is outside alias range")
	}
	plain := strings.ToLower(fmt.Sprintf("%06s", strconv.FormatInt(int64(channelID), 36)))
	cipher, err := ff1.NewCipher(36, 16, key, tweak)
	if err != nil {
		return "", err
	}
	alias, err := cipher.Encrypt(plain)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(alias), nil
}

func DecryptChannelAlias(group string, alias string) (int, error) {
	key, tweak, err := channelAliasInputs(group)
	if err != nil {
		return 0, err
	}
	alias = strings.TrimSpace(alias)
	if len(alias) != channelAliasLength {
		return 0, errors.New("invalid channel alias length")
	}
	for _, char := range strings.ToUpper(alias) {
		if !strings.ContainsRune("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ", char) {
			return 0, errors.New("invalid channel alias characters")
		}
	}
	cipher, err := ff1.NewCipher(36, 16, key, tweak)
	if err != nil {
		return 0, err
	}
	plain, err := cipher.Decrypt(strings.ToLower(alias))
	if err != nil {
		return 0, err
	}
	channelID, err := strconv.ParseUint(plain, 36, 64)
	if err != nil || channelID == 0 || channelID >= channelAliasCapacity {
		return 0, errors.New("decrypted channel id is outside alias range")
	}
	return int(channelID), nil
}

func channelAliasInputs(group string) ([]byte, []byte, error) {
	group = strings.TrimSpace(group)
	if group == "" {
		return nil, nil, errors.New("group is required")
	}
	if len(channelAliasKey) != 16 {
		return nil, nil, errors.New("channel alias key is not initialized")
	}
	hash := sha256.Sum256([]byte(group))
	return channelAliasKey, hash[:16], nil
}

func decodeChannelAliasKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 16 {
		return nil, errors.New("channel alias key must be base64 encoded AES-128")
	}
	return key, nil
}
