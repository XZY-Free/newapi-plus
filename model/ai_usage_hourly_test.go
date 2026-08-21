package model

import "testing"

// §12.2：稳定 dimension_hash——相同维度恒同，任一变体即不同。
func TestUsageDimensionHashDeterministic(t *testing.T) {
	mk := func(modelName string) UsageProjectionDim {
		return UsageProjectionDim{
			BucketTime: 1750000000, ProfileID: 9, PrincipalID: 1, CredentialPurposeID: 10,
			UsageBusinessDomainID: 3, UsageTeamID: 55, CallerKey: "", RootAppCode: "app-wb",
			AppID: 7, AppBusinessDomainID: 2, OwnerTeamID: 4, IdentityAssurance: "CREDENTIAL_ONLY",
			ClientVerified: false, ModelName: modelName,
		}
	}
	h1 := HashDimension(mk("gpt-4o").CanonicalDimension())
	h2 := HashDimension(mk("gpt-4o").CanonicalDimension())
	if h1 != h2 {
		t.Fatalf("相同维度应得到相同 hash: %s vs %s", h1, h2)
	}
	if h1 == HashDimension(mk("claude-3").CanonicalDimension()) {
		t.Fatal("不同 model 应得到不同 hash")
	}
}

// §12.2：dimension_hash 稳定长度为 64（SHA-256 hex）。
func TestUsageDimensionHashLen(t *testing.T) {
	d := UsageProjectionDim{BucketTime: 1, ModelName: "m", IdentityAssurance: "UNVERIFIED"}
	if got := HashDimension(d.CanonicalDimension()); len(got) != 64 {
		t.Fatalf("dimension_hash 应为 64 位 hex, got %d", len(got))
	}
}

// §12.2：空 assurance 规范化为 UNVERIFIED。
func TestUsageCanonicalAssurance(t *testing.T) {
	d := UsageProjectionDim{IdentityAssurance: ""}
	if got := d.CanonicalAssurance(); got != "UNVERIFIED" {
		t.Fatalf("空 assurance 应规范化为 UNVERIFIED, got %s", got)
	}
}

// §12.2：ParseUsageAttribution 从 ai_attribution 快照解析维度。
func TestParseUsageAttribution(t *testing.T) {
	a := map[string]interface{}{
		"profile_id":               9,
		"principal_id":             1,
		"credential_purpose_id":    10,
		"usage_business_domain_id": 3,
		"usage_team_id":            55,
		"caller_id":                "caller-9",
		"root_app_id":              "app-wb",
		"application_business_domain_id": 2,
		"owner_team_id":                   4,
		"identity_assurance":              "SIGNED_CONTEXT",
		"client_verified":                 true,
	}
	d, ok := ParseUsageAttribution(a)
	if !ok {
		t.Fatal("应识别出可信归因")
	}
	if d.ProfileID != 9 || d.PrincipalID != 1 || d.CredentialPurposeID != 10 ||
		d.UsageBusinessDomainID != 3 || d.UsageTeamID != 55 || d.CallerKey != "caller-9" ||
		d.RootAppCode != "app-wb" || d.OwnerTeamID != 4 || !d.ClientVerified {
		t.Fatalf("维度解析错误: %+v", d)
	}
}

// §12.2：无归因快照 → ok=false（走未归因桶逻辑）。
func TestParseUsageAttributionNone(t *testing.T) {
	if _, ok := ParseUsageAttribution(nil); ok {
		t.Fatal("nil 不应被识别为可信归因")
	}
	if _, ok := ParseUsageAttribution(map[string]interface{}{}); ok {
		t.Fatal("空 map 不应被识别为可信归因")
	}
}
