package service

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func masterKey32(seed byte) string {
	b := make([]byte, 32)
	for i := range b {
		b[i] = seed
	}
	return base64.StdEncoding.EncodeToString(b)
}

// 门禁 18/19/20：加解密 round-trip、错主密钥失败、明文只展示一次（Base64URL no padding）。
func TestSigningSecretEncryptDecryptRoundTrip(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, masterKey32(0x1))

	raw, display, err := GenerateSigningSecret()
	require.NoError(t, err)
	require.Len(t, raw, constant.SigningSecretRawLen)
	// 明文为 32 随机字节的 Base64URL no padding 展示。
	require.Equal(t, base64.RawURLEncoding.EncodeToString(raw), display)

	ct, err := encryptSigningSecret(1, "k1", raw)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(ct, constant.SigningSecretVersionPrefix), "密文必须带版本前缀")
	require.NotEqual(t, raw, ct, "数据库不得存明文")
	require.NotContains(t, ct, display, "密文不得包含明文展示")

	dec, err := decryptSigningSecret(1, "k1", ct)
	require.NoError(t, err)
	require.Equal(t, raw, dec, "解密必须还原原始字节")
}

// 门禁 19：错误主密钥（长度错误）解密必须失败。
func TestSigningSecretWrongMasterKeyLengthFails(t *testing.T) {
	// 16 字节 → 非法（必须是 32 字节）。
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 16)))
	_, err := encryptSigningSecret(1, "k1", make([]byte, 32))
	require.Error(t, err)
}

// 门禁 19：更换主密钥后解密旧密文必须失败（避免错误密钥“通过”）。
func TestSigningSecretWrongMasterKeyDecryptFails(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, masterKey32(0x1))
	raw := make([]byte, 32)
	ct, err := encryptSigningSecret(1, "k1", raw)
	require.NoError(t, err)

	t.Setenv(constant.AIAttributionMasterKeyEnv, masterKey32(0x2))
	_, err = decryptSigningSecret(1, "k1", ct)
	require.Error(t, err, "错误主密钥必须解密失败")
}

// 门禁 19：AAD 绑定 profile_id + key_id，任一不同即解密失败。
func TestSigningSecretAADBinding(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, masterKey32(0x3))
	raw := make([]byte, 32)
	ct, err := encryptSigningSecret(7, "key-a", raw)
	require.NoError(t, err)

	// 不同 profile_id。
	_, err = decryptSigningSecret(8, "key-a", ct)
	require.Error(t, err, "不同 profile_id 必须解密失败")
	// 不同 key_id。
	_, err = decryptSigningSecret(7, "key-b", ct)
	require.Error(t, err, "不同 key_id 必须解密失败")
}

// 门禁 18：非 32 字节明文不得加密。
func TestSigningSecretRejectsNonRawLenPlaintext(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, masterKey32(0x4))
	_, err := encryptSigningSecret(1, "k1", make([]byte, 16))
	require.Error(t, err)
}
