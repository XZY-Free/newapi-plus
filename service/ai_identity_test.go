package service

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAIGovernanceService 迁移治理表并清空，保证每个测试独立。
func setupAIGovernanceService(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(model.AIGovernanceModels()...))
	tables := []string{
		"ai_business_domains", "ai_owner_teams", "ai_usage_teams", "ai_principals",
		"ai_credential_purposes", "ai_applications", "ai_identity_profiles",
		"ai_identity_app_bindings", "ai_identity_signing_keys", "ai_identity_audit_events",
	}
	for _, tbl := range tables {
		model.DB.Exec("DELETE FROM " + tbl)
	}
	model.DB.Exec("DELETE FROM tokens")
	invalidateIdentitySnapshotCache()
}

func createAIToken(t *testing.T) int {
	t.Helper()
	tk := model.Token{
		Key:            "sk-test-" + common.GetRandomString(24),
		UserId:         1,
		Status:         1,
		Name:           "t",
		CreatedTime:    common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}
	require.NoError(t, model.DB.Create(&tk).Error)
	return tk.Id
}

type aiMasterFixture struct {
	DomainID    int
	UsageTeamID int
	OwnerTeamID int
	PrincipalID int
	PurposeID   int
	AppID       int
	App2ID      int
}

func setupAIMasterData(t *testing.T) aiMasterFixture {
	t.Helper()
	f := aiMasterFixture{}
	d, err := CreateBusinessDomain("finance", "财务")
	require.NoError(t, err)
	f.DomainID = d.Id

	ut, err := CreateUsageTeam("finance_digital", "财务数字化组")
	require.NoError(t, err)
	f.UsageTeamID = ut.Id

	ot, err := CreateOwnerTeam("ai_application", "AI应用组")
	require.NoError(t, err)
	f.OwnerTeamID = ot.Id

	p, err := CreatePrincipal("zhangsan", "张三", f.DomainID, f.UsageTeamID)
	require.NoError(t, err)
	f.PrincipalID = p.Id

	purpose, err := CreateCredentialPurpose("workbuddy", "WorkBuddy", constant.PurposeTypeDesktopClient)
	require.NoError(t, err)
	f.PurposeID = purpose.Id

	app, err := CreateApplication("hr_assistant", "人力助手", f.DomainID, f.OwnerTeamID)
	require.NoError(t, err)
	f.AppID = app.Id

	app2, err := CreateApplication("finance_assistant", "财务助手", f.DomainID, f.OwnerTeamID)
	require.NoError(t, err)
	f.App2ID = app2.Id
	return f
}

// 门禁 4：code 创建后不可修改（更新接口不接受 code，code 保持不变）。
func TestIdentityCodeImmutable(t *testing.T) {
	setupAIGovernanceService(t)
	d, err := CreateBusinessDomain("finance", "财务")
	require.NoError(t, err)
	enabled := false
	updated, err := UpdateBusinessDomain(d.Id, "财务部", &enabled)
	require.NoError(t, err)
	assert.Equal(t, "finance", updated.DomainCode, "code 不可修改")
}

// 门禁 5/6/7：disabled Domain/OwnerTeam/UsageTeam 不能分配给新 App/Principal。
func TestIdentityDisabledReferentialGate(t *testing.T) {
	setupAIGovernanceService(t)
	d, _ := CreateBusinessDomain("finance", "财务")
	ut, _ := CreateUsageTeam("finance_digital", "财务数字化组")
	ot, _ := CreateOwnerTeam("ai_application", "AI应用组")

	disable := false
	UpdateBusinessDomain(d.Id, "", &disable)
	UpdateUsageTeam(ut.Id, "", &disable)
	UpdateOwnerTeam(ot.Id, "", &disable)

	// 已停用 Domain 不能新建 Principal。
	_, err := CreatePrincipal("zhangsan", "张三", d.Id, ut.Id)
	require.Error(t, err, "已停用 Domain 不能分配给新 Principal")

	// 已停用 Domain 不能新建 App。
	_, err = CreateApplication("hr_assistant", "人力助手", d.Id, ot.Id)
	require.Error(t, err, "已停用 Domain 不能分配给新 App")

	// 恢复后可行。
	enable := true
	UpdateBusinessDomain(d.Id, "", &enable)
	UpdateUsageTeam(ut.Id, "", &enable)
	UpdateOwnerTeam(ot.Id, "", &enable)

	_, err = CreatePrincipal("zhangsan", "张三", d.Id, ut.Id)
	require.NoError(t, err)
	// 已停用 UsageTeam 不能新建 Principal。
	UpdateUsageTeam(ut.Id, "", &disable)
	_, err = CreatePrincipal("lisi", "李四", d.Id, ut.Id)
	require.Error(t, err)
	// 已停用 OwnerTeam 不能新建 App。
	UpdateUsageTeam(ut.Id, "", &enable)
	UpdateOwnerTeam(ot.Id, "", &disable)
	_, err = CreateApplication("hr2", "人力助手2", d.Id, ot.Id)
	require.Error(t, err)
}

// 门禁 9：STATIC/PRINCIPAL 缺 Principal 或 Purpose 拒绝。
func TestIdentityStaticPrincipalMissingRefsRejected(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	// 缺 Principal。
	_, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		CredentialPurposeId:   f.PurposeID, Enabled: true,
	}, nil)
	require.Error(t, err)
	// 缺 Purpose。
	_, err = CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		PrincipalId:           f.PrincipalID, Enabled: true,
	}, nil)
	require.Error(t, err)
}

// 门禁 10：STATIC/PRINCIPAL 存在 App Binding 拒绝。
func TestIdentityStaticPrincipalWithBindingRejected(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)
	_, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		PrincipalId:           f.PrincipalID, CredentialPurposeId: f.PurposeID, Enabled: true,
	}, []int{f.AppID})
	require.Error(t, err, "STATIC/PRINCIPAL 不允许绑定应用")
}

// 门禁 11：STATIC/APPLICATION 0/2 个 Binding 拒绝。
func TestIdentityStaticApplicationBindingCount(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)

	// 0 个绑定。
	t0 := createAIToken(t)
	_, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: t0, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetApplication,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
	}, nil)
	require.Error(t, err)

	// 2 个绑定。
	t1 := createAIToken(t)
	_, err = CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: t1, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetApplication,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
	}, []int{f.AppID, f.App2ID})
	require.Error(t, err)
}

// 门禁 12：DYNAMIC/PLATFORM 0 个 Binding 拒绝。
func TestIdentityDynamicPlatformNoBindingRejected(t *testing.T) {
	setupAIGovernanceService(t)
	setupAIMasterData(t)
	tokenID := createAIToken(t)
	_, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeDynamic,
		AttributionTargetType: constant.AttributionTargetPlatform,
		IdentityAssurance:     constant.IdentityAssuranceSignedContext,
		CallerId:              "workflow-platform-prod",
	}, nil)
	require.Error(t, err, "DYNAMIC/PLATFORM 必须至少绑定 1 个应用")
}

// 门禁 13：DYNAMIC/PLATFORM 缺 Caller 或 Signing Key 拒绝。
func TestIdentityDynamicPlatformMissingCallerOrKey(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	// 缺 Caller。
	_, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeDynamic,
		AttributionTargetType: constant.AttributionTargetPlatform,
		IdentityAssurance:     constant.IdentityAssuranceSignedContext,
	}, []int{f.AppID})
	require.Error(t, err)

	// 有 Caller、有绑定，但无 ACTIVE 签名密钥 → 不能启用。
	t2 := createAIToken(t)
	p, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: t2, IdentityMode: constant.IdentityModeDynamic,
		AttributionTargetType: constant.AttributionTargetPlatform,
		IdentityAssurance:     constant.IdentityAssuranceSignedContext,
		CallerId:              "workflow-platform-prod",
	}, []int{f.AppID})
	require.NoError(t, err)
	// 尝试启用。
	enabled := true
	_, err = UpdateIdentityProfile(&IdentityProfilePatch{Id: p.Id, Enabled: &enabled})
	require.Error(t, err, "无 ACTIVE 签名密钥不能启用 DYNAMIC/PLATFORM")
}

// 门禁 14：HYBRID/APPLICATION 0/2 个 Binding 拒绝。
func TestIdentityHybridApplicationBindingCount(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)

	t0 := createAIToken(t)
	_, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: t0, IdentityMode: constant.IdentityModeHybrid,
		AttributionTargetType: constant.AttributionTargetApplication,
		IdentityAssurance:     constant.IdentityAssuranceHybridVerified,
		CallerId:              "caller-x",
	}, nil)
	require.Error(t, err)

	t1 := createAIToken(t)
	_, err = CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: t1, IdentityMode: constant.IdentityModeHybrid,
		AttributionTargetType: constant.AttributionTargetApplication,
		IdentityAssurance:     constant.IdentityAssuranceHybridVerified,
		CallerId:              "caller-x",
	}, []int{f.AppID, f.App2ID})
	require.Error(t, err)
}

// 门禁 15：非法 identity_mode/target/assurance 组合拒绝。
func TestIdentityIllegalCombinationRejected(t *testing.T) {
	setupAIGovernanceService(t)
	tokenID := createAIToken(t)
	// STATIC/PLATFORM 不存在。
	_, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPlatform,
		IdentityAssurance:     constant.IdentityAssuranceSignedContext,
		CallerId:              "c",
	}, nil)
	require.Error(t, err)
	// DYNAMIC/PRINCIPAL 不存在。
	t2 := createAIToken(t)
	_, err = CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: t2, IdentityMode: constant.IdentityModeDynamic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceSignedContext,
		CallerId:              "c",
	}, nil)
	require.Error(t, err)
}

// 门禁 8：同一 token_id 第二个 Profile 拒绝。
func TestIdentitySecondProfileForTokenRejected(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)
	_, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		PrincipalId:           f.PrincipalID, CredentialPurposeId: f.PurposeID,
		Enabled: false,
	}, nil)
	require.NoError(t, err)
	_, err = CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		PrincipalId:           f.PrincipalID, CredentialPurposeId: f.PurposeID,
		Enabled: false,
	}, nil)
	require.Error(t, err)
}

// 门禁 17：同 (principal,purpose,environment) 第二个 enabled STATIC/PRINCIPAL Profile 拒绝。
func TestIdentityDuplicateEnabledCredentialRejected(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)

	tokenA := createAIToken(t)
	_, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenA, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		PrincipalId:           f.PrincipalID, CredentialPurposeId: f.PurposeID, Enabled: true,
	}, nil)
	require.NoError(t, err)

	// 同一 (principal,purpose,environment) 第二个 enabled Profile → 拒绝。
	tokenB := createAIToken(t)
	_, err = CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenB, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		PrincipalId:           f.PrincipalID, CredentialPurposeId: f.PurposeID, Enabled: true,
	}, nil)
	require.Error(t, err, "同 principal+purpose+environment 不允许第二个 enabled Profile")

	// 不同 environment 允许并存。
	tokenC := createAIToken(t)
	_, err = CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenC, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		PrincipalId:           f.PrincipalID, CredentialPurposeId: f.PurposeID, Environment: "test", Enabled: true,
	}, nil)
	require.NoError(t, err)

	// 停用 A（prod）后，prod 环境的同名 Profile 才能启用。
	tokenAProfile, err := GetIdentityProfileByTokenIDForTest(tokenA)
	require.NoError(t, err)
	require.NoError(t, DisableIdentityProfile(tokenAProfile.Id))

	tokenD := createAIToken(t)
	_, err = CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenD, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		PrincipalId:           f.PrincipalID, CredentialPurposeId: f.PurposeID, Environment: "prod", Enabled: true,
	}, nil)
	require.NoError(t, err)

	// 停用后旧 Profile 仍可再次启用（用于回滚）。
	require.NoError(t, DisableIdentityProfile(tokenAProfile.Id))
	_, err = CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenA, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		PrincipalId:           f.PrincipalID, CredentialPurposeId: f.PurposeID, Environment: "prod", Enabled: true,
	}, nil)
	require.Error(t, err, "tokenA 已存在 Profile，不能重复创建")
}

// 门禁 25：rate_limit_enabled=true 但 window/max 非法时 Profile 不能启用。
func TestIdentityInvalidRateLimitRejectsEnable(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)
	_, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		PrincipalId:           f.PrincipalID, CredentialPurposeId: f.PurposeID,
		RateLimitEnabled: true, RateLimitWindowSeconds: 5, RateLimitMaxRequests: 0,
		Enabled: true,
	}, nil)
	require.Error(t, err, "window/max 非法不能启用")
}

// 门禁 20/22：generate 只本次返回 Secret；rotate 后旧 Key 进入 RETIRING。
func TestIdentitySigningKeyGenerateAndRotate(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	p, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeDynamic,
		AttributionTargetType: constant.AttributionTargetPlatform,
		IdentityAssurance:     constant.IdentityAssuranceSignedContext,
		CallerId:              "workflow-platform-prod",
	}, []int{f.AppID})
	require.NoError(t, err)

	key1, secret1, err := GenerateSigningKey(p.Id)
	require.NoError(t, err)
	require.NotEmpty(t, secret1, "generate 必须返回明文一次")

	// 密钥持久化的是密文，不是明文。
	require.True(t, strings.HasPrefix(key1.SecretCiphertext, constant.SigningSecretVersionPrefix))
	require.NotContains(t, key1.SecretCiphertext, secret1)

	// 生成的 secret 是 Base64URL no padding 明文（GenerateSigningSecret 已覆盖）。
	// rotate → 旧 Key RETIRING，新 Key ACTIVE，明文只本次返回。
	key2, secret2, err := RotateSigningKey(p.Id)
	require.NoError(t, err)
	require.NotEmpty(t, secret2)
	require.NotEqual(t, secret1, secret2, "轮换必须生成新明文")

	keys, err := ListSigningKeysMeta(p.Id)
	require.NoError(t, err)
	statusByID := map[string]string{}
	for _, k := range keys {
		statusByID[k.KeyId] = k.Status
	}
	require.Equal(t, constant.SigningKeyStatusRetiring, statusByID[key1.KeyId], "旧 Key 必须进入 RETIRING")
	require.Equal(t, constant.SigningKeyStatusActive, statusByID[key2.KeyId])

	// 序列化边界：密钥模型 JSON 化后绝不含密文/明文（json:"-" 守卫）。
	rawKey, err := GetSigningKey(p.Id, key1.KeyId)
	require.NoError(t, err)
	jsonData, err := common.Marshal(rawKey)
	require.NoError(t, err)
	jsonStr := string(jsonData)
	require.NotContains(t, jsonStr, key1.SecretCiphertext, "JSON 不得包含密文")
	require.NotContains(t, jsonStr, secret1, "JSON 不得包含明文")
	require.NotContains(t, jsonStr, "secret_ciphertext", "JSON 不得暴露密文字段名")
}

// 门禁 21：Signing Keys GET 视图不含明文/密文（通过 signingKeyMetaView 间接验证 controller 使用）。
func TestIdentitySigningKeyGeneratePersistsOnlyCiphertext(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)
	p, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeDynamic,
		AttributionTargetType: constant.AttributionTargetPlatform,
		IdentityAssurance:     constant.IdentityAssuranceSignedContext,
		CallerId:              "caller",
	}, []int{f.AppID})
	require.NoError(t, err)
	key, secret, err := GenerateSigningKey(p.Id)
	require.NoError(t, err)

	var stored model.AIIdentitySigningKey
	require.NoError(t, model.DB.First(&stored, key.Id).Error)
	require.NotEqual(t, secret, stored.SecretCiphertext, "数据库不得存明文")
	require.True(t, strings.HasPrefix(stored.SecretCiphertext, constant.SigningSecretVersionPrefix))
}

// 快照：按 token_id 统一读取 + 节点内缓存失效。
func TestIdentitySnapshotAndCacheInvalidation(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	snap, err := GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	require.Nil(t, snap, "未登记 Profile 返回 nil")

	_, err = CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		PrincipalId:           f.PrincipalID, CredentialPurposeId: f.PurposeID, Enabled: true,
	}, nil)
	require.NoError(t, err)

	snap, err = GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Equal(t, f.PrincipalID, snap.PrincipalID)
	require.Equal(t, "zhangsan", snap.PrincipalCode)
	require.Equal(t, "workbuddy", snap.CredentialPurposeCode)
	require.Equal(t, "财务", snap.UsageDomainName)
	require.Equal(t, "财务数字化组", snap.UsageTeamName)

	// 缓存命中的情况下，停用后再次读取应看到 enabled=false（缓存已失效）。
	require.True(t, snap.Enabled)
	require.NoError(t, DisableIdentityProfile(snap.ProfileID))
	snap2, err := GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	require.False(t, snap2.Enabled, "缓存必须在停用后失效")
}

// 门禁 13 辅助：按 token_id 查找 Profile（测试用）。
func GetIdentityProfileByTokenIDForTest(tokenID int) (*model.AIIdentityProfile, error) {
	var p model.AIIdentityProfile
	if err := model.DB.Where("token_id = ?", tokenID).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}
