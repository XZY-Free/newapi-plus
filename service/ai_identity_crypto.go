package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

// Signing Secret 加密规范（文档 6.13）：
//   - 主密钥 AI_ATTRIBUTION_MASTER_KEY：Standard Base64，解码后恰好 32 字节，AES-256-GCM；
//   - Signing Secret 每个 Key 随机 32 原始字节；
//   - 数据库只存带版本前缀密文；
//   - AES-GCM Nonce 12 字节随机值；
//   - AAD 绑定 profile_id 与 key_id；
//   - 不使用 NewAPI CRYPTO_SECRET、Session Secret、API Key 替代。

// getSigningMasterKey 读取并校验主密钥。主密钥错误或缺失必须返回错误，禁止回退。
func getSigningMasterKey() ([]byte, error) {
	encoded := os.Getenv(constant.AIAttributionMasterKeyEnv)
	if encoded == "" {
		return nil, errors.New("未配置 AI_ATTRIBUTION_MASTER_KEY，无法执行 Signing Secret 加解密")
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("AI_ATTRIBUTION_MASTER_KEY 不是合法的 Standard Base64")
	}
	if len(key) != constant.SigningSecretMasterKeyLen {
		return nil, fmt.Errorf("AI_ATTRIBUTION_MASTER_KEY 解码后必须为 %d 字节，实际 %d 字节", constant.SigningSecretMasterKeyLen, len(key))
	}
	return key, nil
}

// GenerateSigningSecret 生成 32 随机原始字节，并返回其 Base64URL no padding 展示形式。
func GenerateSigningSecret() (raw []byte, display string, err error) {
	raw = make([]byte, constant.SigningSecretRawLen)
	if _, err = rand.Read(raw); err != nil {
		return nil, "", fmt.Errorf("生成 Signing Secret 失败: %w", err)
	}
	display = base64.RawURLEncoding.EncodeToString(raw)
	return raw, display, nil
}

// GenerateKeyID 生成一个随机的签名密钥 key_id（varchar(64)）。
func GenerateKeyID() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("生成 key_id 失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// encryptSigningSecret 用 AES-256-GCM 加密 32 原始字节。
// 返回带版本前缀的密文：v1:<base64(nonce||ciphertext)>。
func encryptSigningSecret(profileID int, keyID string, plaintext []byte) (string, error) {
	if len(plaintext) != constant.SigningSecretRawLen {
		return "", fmt.Errorf("Signing Secret 明文必须为 %d 字节", constant.SigningSecretRawLen)
	}
	key, err := getSigningMasterKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("初始化 AES 失败: %w", err)
	}
	nonce := make([]byte, constant.SigningSecretNonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %w", err)
	}
	aad := signingSecretAAD(profileID, keyID)
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("初始化 GCM 失败: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, []byte(aad))
	blob := make([]byte, 0, len(nonce)+len(ciphertext))
	blob = append(blob, nonce...)
	blob = append(blob, ciphertext...)
	return constant.SigningSecretVersionPrefix + base64.StdEncoding.EncodeToString(blob), nil
}

// decryptSigningSecret 解密带版本前缀的密文。主密钥错误时必须解密失败。
func decryptSigningSecret(profileID int, keyID string, stored string) ([]byte, error) {
	if !strings.HasPrefix(stored, constant.SigningSecretVersionPrefix) {
		return nil, errors.New("Signing Secret 密文缺少版本前缀")
	}
	payload := strings.TrimPrefix(stored, constant.SigningSecretVersionPrefix)
	blob, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, errors.New("Signing Secret 密文 Base64 解码失败")
	}
	if len(blob) < constant.SigningSecretNonceLen {
		return nil, errors.New("Signing Secret 密文长度非法")
	}
	nonce := blob[:constant.SigningSecretNonceLen]
	ciphertext := blob[constant.SigningSecretNonceLen:]

	key, err := getSigningMasterKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("初始化 AES 失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("初始化 GCM 失败: %w", err)
	}
	aad := signingSecretAAD(profileID, keyID)
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(aad))
	if err != nil {
		return nil, errors.New("Signing Secret 解密失败")
	}
	return plaintext, nil
}

// signingSecretAAD 构造 AES-GCM 的附加认证数据：profile_id + key_id。
func signingSecretAAD(profileID int, keyID string) string {
	return fmt.Sprintf("%d:%s", profileID, keyID)
}
