package model

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setupAIGovernance 迁移第一批治理表并清空，供每个测试独立使用。
func setupAIGovernance(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(AIGovernanceModels()...))
	tables := []string{
		"ai_business_domains", "ai_owner_teams", "ai_usage_teams", "ai_principals",
		"ai_credential_purposes", "ai_applications", "ai_identity_profiles",
		"ai_identity_app_bindings", "ai_identity_signing_keys", "ai_identity_audit_events",
	}
	for _, tbl := range tables {
		DB.Exec("DELETE FROM " + tbl)
	}
}

// 门禁 1：SQLite 全新库可迁移全部 10 张治理/审计表，且 TableName 正确。
func TestAIGovernanceTablesMigrateAndTableNames(t *testing.T) {
	setupAIGovernance(t)
	expect := map[string]string{
		"ai_business_domains":      AIBusinessDomain{}.TableName(),
		"ai_owner_teams":           AIOwnerTeam{}.TableName(),
		"ai_usage_teams":           AIUsageTeam{}.TableName(),
		"ai_principals":            AIPrincipal{}.TableName(),
		"ai_credential_purposes":   AICredentialPurpose{}.TableName(),
		"ai_applications":          AIApplication{}.TableName(),
		"ai_identity_profiles":     AIIdentityProfile{}.TableName(),
		"ai_identity_app_bindings": AIIdentityAppBinding{}.TableName(),
		"ai_identity_signing_keys": AIIdentitySigningKey{}.TableName(),
		"ai_identity_audit_events": AIIdentityAuditEvent{}.TableName(),
	}
	for table, name := range expect {
		require.Equal(t, table, name, "TableName mismatch")
		require.True(t, DB.Migrator().HasTable(table), "table %s not migrated", table)
	}
}

// 门禁 3：Domain/Owner Team/Usage Team/Principal/Purpose/App code 唯一约束。
func TestAIGovernanceCodeUniqueConstraints(t *testing.T) {
	setupAIGovernance(t)
	now := common.GetTimestamp()
	cases := []struct {
		name  string
		first func() error
		dup   func() error
	}{
		{"domain_code", func() error {
			return DB.Create(&AIBusinessDomain{DomainCode: "finance", DomainName: "财务", Enabled: true, CreatedAt: now}).Error
		}, func() error {
			return DB.Create(&AIBusinessDomain{DomainCode: "finance", DomainName: "财务2", Enabled: true, CreatedAt: now}).Error
		}},
		{"owner_team_code", func() error {
			return DB.Create(&AIOwnerTeam{TeamCode: "ai_application", TeamName: "AI应用组", Enabled: true, CreatedAt: now}).Error
		}, func() error {
			return DB.Create(&AIOwnerTeam{TeamCode: "ai_application", TeamName: "重复", Enabled: true, CreatedAt: now}).Error
		}},
		{"usage_team_code", func() error {
			return DB.Create(&AIUsageTeam{TeamCode: "finance_digital", TeamName: "财务数字化组", Enabled: true, CreatedAt: now}).Error
		}, func() error {
			return DB.Create(&AIUsageTeam{TeamCode: "finance_digital", TeamName: "重复", Enabled: true, CreatedAt: now}).Error
		}},
		{"principal_code", func() error {
			return DB.Create(&AIPrincipal{PrincipalCode: "zhangsan", PrincipalName: "张三", PrincipalType: "PERSON", Enabled: true, CreatedAt: now}).Error
		}, func() error {
			return DB.Create(&AIPrincipal{PrincipalCode: "zhangsan", PrincipalName: "重复", PrincipalType: "PERSON", Enabled: true, CreatedAt: now}).Error
		}},
		{"purpose_code", func() error {
			return DB.Create(&AICredentialPurpose{PurposeCode: "workbuddy", PurposeName: "WorkBuddy", PurposeType: "DESKTOP_CLIENT", Enabled: true, CreatedAt: now}).Error
		}, func() error {
			return DB.Create(&AICredentialPurpose{PurposeCode: "workbuddy", PurposeName: "重复", PurposeType: "DESKTOP_CLIENT", Enabled: true, CreatedAt: now}).Error
		}},
		{"app_code", func() error {
			return DB.Create(&AIApplication{AppCode: "hr_assistant", AppName: "人力助手", Enabled: true, CreatedAt: now}).Error
		}, func() error {
			return DB.Create(&AIApplication{AppCode: "hr_assistant", AppName: "重复", Enabled: true, CreatedAt: now}).Error
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupAIGovernance(t)
			require.NoError(t, tc.first())
			err := tc.dup()
			require.Error(t, err, "duplicate %s should violate unique constraint", tc.name)
			require.True(t, isUniqueViolation(err), "expected unique violation, got: %v", err)
		})
	}
}

// 门禁 8：同一 token_id 第二个 Profile 唯一约束拒绝。
func TestAIGovernanceTokenIDUniqueConstraint(t *testing.T) {
	setupAIGovernance(t)
	now := common.GetTimestamp()
	require.NoError(t, DB.Create(&AIIdentityProfile{TokenId: 101, IdentityMode: "STATIC", Enabled: false, CreatedAt: now}).Error)
	err := DB.Create(&AIIdentityProfile{TokenId: 101, IdentityMode: "STATIC", Enabled: false, CreatedAt: now}).Error
	require.Error(t, err)
	require.True(t, isUniqueViolation(err))
}

// 门禁 18（模型层）：签名密钥表只有密文字段，无明文 secret 字段。
func TestAIGovernanceSigningKeyHasNoPlaintextColumn(t *testing.T) {
	setupAIGovernance(t)
	cols, err := DB.Migrator().ColumnTypes(&AIIdentitySigningKey{})
	require.NoError(t, err)
	colNames := make([]string, 0, len(cols))
	for _, c := range cols {
		colNames = append(colNames, c.Name())
	}
	require.Contains(t, colNames, "secret_ciphertext", "必须存在密文字段 secret_ciphertext")
	for _, forbidden := range []string{"secret", "secret_plaintext", "plaintext", "secret_value"} {
		require.NotContains(t, colNames, forbidden, "不得存在明文 secret 字段: %s", forbidden)
	}
}

// 绑定唯一约束：profile_id + app_id。
func TestAIGovernanceAppBindingUniqueConstraint(t *testing.T) {
	setupAIGovernance(t)
	now := common.GetTimestamp()
	require.NoError(t, DB.Create(&AIIdentityAppBinding{ProfileId: 1, AppId: 10, Enabled: true, CreatedAt: now}).Error)
	err := DB.Create(&AIIdentityAppBinding{ProfileId: 1, AppId: 10, Enabled: true, CreatedAt: now}).Error
	require.Error(t, err)
	require.True(t, isUniqueViolation(err))
}

// 签名密钥唯一约束：profile_id + key_id。
func TestAIGovernanceSigningKeyUniqueConstraint(t *testing.T) {
	setupAIGovernance(t)
	now := common.GetTimestamp()
	require.NoError(t, DB.Create(&AIIdentitySigningKey{ProfileId: 1, KeyId: "k1", SecretCiphertext: "v1:abc", Status: "ACTIVE", CreatedAt: now}).Error)
	err := DB.Create(&AIIdentitySigningKey{ProfileId: 1, KeyId: "k1", SecretCiphertext: "v1:def", Status: "ACTIVE", CreatedAt: now}).Error
	require.Error(t, err)
	require.True(t, isUniqueViolation(err))
}

// 迁移注册：AIGovernanceModels 恰好返回 11 张表
// （第一批 10 张 + §12 企业用量整点投影 AIUsageHourly）。
func TestAIGovernanceModelsCount(t *testing.T) {
	require.Len(t, AIGovernanceModels(), 11)
}

// isUniqueViolation 判定 GORM 唯一约束冲突。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unique") || strings.Contains(s, "duplicate entry") ||
		strings.Contains(s, "duplicate key") || strings.Contains(s, "constraint failed")
}

// 门禁 24（真实数据库迁移门禁）：按约定连接 TEST_MYSQL_DSN / TEST_POSTGRES_DSN
// 配置的数据库，对 AIGovernanceModels() 全部 10 张表做真实迁移与幂等校验，并验证
// 第 1 批 schema 合约。未配置对应环境变量时子测试跳过（本地普通验证即该路径）。
//
// 关键约束：绝不迁移/删除固定生产表名。每轮测试生成不可预测的唯一前缀测试表，
// 只删除本测试精确生成并记录的这 10 张表，反向删除，且逐表校验目标名带本次前缀，
// 不使用任何通配符、不触碰其他任何表。测试数据仅 RFC/示例值，不含任何凭证体。
func TestAIGovernanceModelsMigrateConfiguredDatabases(t *testing.T) {
	tests := []struct {
		name      string
		env       string
		dialector func(string) gorm.Dialector
	}{
		{name: "mysql", env: "TEST_MYSQL_DSN", dialector: func(dsn string) gorm.Dialector { return mysql.Open(dsn) }},
		{name: "postgres", env: "TEST_POSTGRES_DSN", dialector: func(dsn string) gorm.Dialector {
			return postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(test.env))
			if dsn == "" {
				t.Skip(test.env + " is not configured")
			}
			db, err := gorm.Open(test.dialector(dsn), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			testAIGovernanceModelsMigrate(t, db)
		})
	}
}

// aiGovernanceMigratableModel 将第 1 批 10 张模型表与其唯一测试表后缀配对，
// 顺序与 AIGovernanceModels() 保持一致（businessdomains…auditevents）。
type aiGovernanceMigratableModel struct {
	tag   string
	model interface{}
}

func aiGovernanceMigratableModels() []aiGovernanceMigratableModel {
	return []aiGovernanceMigratableModel{
		{"businessdomains", &AIBusinessDomain{}},
		{"ownerteams", &AIOwnerTeam{}},
		{"usageteams", &AIUsageTeam{}},
		{"principals", &AIPrincipal{}},
		{"purposes", &AICredentialPurpose{}},
		{"apps", &AIApplication{}},
		{"profiles", &AIIdentityProfile{}},
		{"bindings", &AIIdentityAppBinding{}},
		{"signkeys", &AIIdentitySigningKey{}},
		{"auditevents", &AIIdentityAuditEvent{}},
	}
}

func testAIGovernanceModelsMigrate(t *testing.T, db *gorm.DB) {
	t.Helper()
	// 不可预测的唯一前缀（纳秒时间戳 + 随机串），保证并发/多轮测试互不冲突，
	// 且绝不会与固定生产表名冲突。
	nonce := fmt.Sprintf("%d%s", time.Now().UnixNano(), common.GetRandomString(8))
	prefix := "aigov_" + nonce + "_"
	models := aiGovernanceMigratableModels()

	tableNames := make([]string, 0, len(models))
	// 在创建任何表之前注册清理，即使中途迁移或断言失败也不留残表。
	t.Cleanup(func() {
		for i := len(tableNames) - 1; i >= 0; i-- {
			name := tableNames[i]
			if !strings.HasPrefix(name, prefix) {
				t.Errorf("cleanup table must carry generated prefix: %s", name)
				continue
			}
			if !db.Migrator().HasTable(name) {
				continue
			}
			if err := db.Migrator().DropTable(name); err != nil {
				t.Errorf("drop %s: %v", name, err)
			}
		}
	})
	for _, m := range models {
		name := prefix + m.tag
		tableNames = append(tableNames, name)

		// 迁移到唯一测试表，并验证表确实存在。
		require.NoError(t, db.Table(name).AutoMigrate(m.model), "migrate %s", name)
		require.True(t, db.Migrator().HasTable(name), "table %s should exist after migrate", name)
		// 重复 AutoMigrate 必须幂等、无错误。
		require.NoError(t, db.Table(name).AutoMigrate(m.model), "re-migrate %s must be idempotent", name)
	}

	// Schema 合约：Profile 的 principal_id/credential_purpose_id/caller_id 可空；
	// Audit id 为 BIGINT 类；token_id、绑定(profile_id,app_id)、签名密钥(profile_id,key_id)
	// 唯一约束均通过真实重复插入验证。
	profilesTable, bindingsTable, signkeysTable, auditTable := tableNames[6], tableNames[7], tableNames[8], tableNames[9]
	verifyAIGovernanceProfileNullable(t, db, profilesTable)
	verifyAIGovernanceAuditIDBigInt(t, db, auditTable)
	verifyAIGovernanceTokenIDUnique(t, db, profilesTable)
	verifyAIGovernanceAppBindingUnique(t, db, bindingsTable)
	verifyAIGovernanceSigningKeyUnique(t, db, signkeysTable)
}

func verifyAIGovernanceProfileNullable(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	colTypes, err := db.Table(table).Migrator().ColumnTypes(&AIIdentityProfile{})
	require.NoError(t, err)
	nullable := map[string]bool{}
	for _, ct := range colTypes {
		if ct.Name() == "principal_id" || ct.Name() == "credential_purpose_id" || ct.Name() == "caller_id" {
			n, ok := ct.Nullable()
			require.True(t, ok, "Nullable() unsupported for %s", ct.Name())
			nullable[ct.Name()] = n
		}
	}
	for _, col := range []string{"principal_id", "credential_purpose_id", "caller_id"} {
		require.Truef(t, nullable[col], "Profile column %s must be nullable", col)
	}
}

func verifyAIGovernanceAuditIDBigInt(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	colTypes, err := db.Table(table).Migrator().ColumnTypes(&AIIdentityAuditEvent{})
	require.NoError(t, err)
	found := false
	for _, ct := range colTypes {
		if ct.Name() != "id" {
			continue
		}
		found = true
		// MySQL 为 BIGINT AUTO_INCREMENT，PostgreSQL 为 bigserial，均属 BIGINT 类。
		require.Contains(t, strings.ToUpper(ct.DatabaseTypeName()), "BIG",
			"Audit id must be BIGINT-class, got %s", ct.DatabaseTypeName())
	}
	require.True(t, found, "Audit id column not found")
}

func verifyAIGovernanceTokenIDUnique(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	now := common.GetTimestamp()
	insert := func(tk int) error {
		return db.Table(table).Create(&AIIdentityProfile{TokenId: tk, IdentityMode: "STATIC", Enabled: false, CreatedAt: now, UpdatedAt: now}).Error
	}
	require.NoError(t, insert(1001))
	err := insert(1001)
	require.Error(t, err, "duplicate token_id must fail")
	require.True(t, isUniqueViolation(err))
}

func verifyAIGovernanceAppBindingUnique(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	now := common.GetTimestamp()
	insert := func(pid, aid int) error {
		return db.Table(table).Create(&AIIdentityAppBinding{ProfileId: pid, AppId: aid, Enabled: true, CreatedAt: now, UpdatedAt: now}).Error
	}
	require.NoError(t, insert(2001, 2002))
	err := insert(2001, 2002)
	require.Error(t, err, "duplicate (profile_id, app_id) must fail")
	require.True(t, isUniqueViolation(err))
	// 复合唯一：不同 profile、相同 app 应可插入。
	require.NoError(t, insert(2003, 2002), "(different profile, same app) should be allowed")
}

func verifyAIGovernanceSigningKeyUnique(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	now := common.GetTimestamp()
	insert := func(pid int, keyID string) error {
		return db.Table(table).Create(&AIIdentitySigningKey{ProfileId: pid, KeyId: keyID, SecretCiphertext: "v1:rfc-fake", Status: "ACTIVE", CreatedAt: now, UpdatedAt: now}).Error
	}
	require.NoError(t, insert(3001, "k3001"))
	err := insert(3001, "k3001")
	require.Error(t, err, "duplicate (profile_id, key_id) must fail")
	require.True(t, isUniqueViolation(err))
	// 复合唯一：不同 profile、相同 key_id 应可插入。
	require.NoError(t, insert(3002, "k3001"), "(different profile, same key_id) should be allowed")
}
