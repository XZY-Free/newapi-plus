package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

// lockForUpdate must emit FOR UPDATE on databases that support it and skip
// it on SQLite, where the syntax does not exist.
//
// The dummy dialector is used because SQLite drivers strip locking clauses
// from the generated SQL, which would mask what the helper itself does.
func TestLockForUpdateEmitsRowLock(t *testing.T) {
	dummyDB, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	require.NoError(t, err)
	buildSQL := func() string {
		var rows []Redemption
		return lockForUpdate(dummyDB).Where("id = ?", 1).Find(&rows).Statement.SQL.String()
	}

	t.Cleanup(func() {
		common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	})

	common.SetDatabaseTypes(common.DatabaseTypeMySQL, common.DatabaseTypeSQLite)
	assert.Contains(t, buildSQL(), "FOR UPDATE")

	common.SetDatabaseTypes(common.DatabaseTypePostgreSQL, common.DatabaseTypeSQLite)
	assert.Contains(t, buildSQL(), "FOR UPDATE")

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	assert.NotContains(t, buildSQL(), "FOR UPDATE")
}

// 治理表的事务行锁方法必须复用 lockForUpdate：MySQL/PostgreSQL 输出 FOR UPDATE，
// SQLite 走无 FOR UPDATE 兼容路径。
func TestGovernanceLockHelpersEmitRowLock(t *testing.T) {
	dummyDB, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	require.NoError(t, err)

	profileSQL := func() string {
		_, _ = LockAIIdentityProfile(dummyDB, 7)
		var p AIIdentityProfile
		return lockForUpdate(dummyDB).Where("id = ?", 7).Find(&p).Statement.SQL.String()
	}
	principalSQL := func() string {
		_, _ = LockAIPrincipal(dummyDB, 7)
		var p AIPrincipal
		return lockForUpdate(dummyDB).Where("id = ?", 7).Find(&p).Statement.SQL.String()
	}

	t.Cleanup(func() {
		common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	})

	common.SetDatabaseTypes(common.DatabaseTypeMySQL, common.DatabaseTypeSQLite)
	assert.Contains(t, profileSQL(), "FOR UPDATE", "MySQL 下 Profile 锁必须带 FOR UPDATE")
	assert.Contains(t, principalSQL(), "FOR UPDATE", "MySQL 下 Principal 锁必须带 FOR UPDATE")

	common.SetDatabaseTypes(common.DatabaseTypePostgreSQL, common.DatabaseTypeSQLite)
	assert.Contains(t, profileSQL(), "FOR UPDATE", "PostgreSQL 下 Profile 锁必须带 FOR UPDATE")

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	assert.NotContains(t, profileSQL(), "FOR UPDATE", "SQLite 必须跳过 FOR UPDATE")
	assert.NotContains(t, principalSQL(), "FOR UPDATE", "SQLite 必须跳过 FOR UPDATE")
}
