package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentSimpleMigrationTransfersOldReferralOnceAndPreservesNewRewards(t *testing.T) {
	setupLedgerTestState(t)
	seedAffQuotaUser(t, 9801, 500_000, 700_000)
	seedAffQuotaUser(t, 9802, 300_000, 400_000)
	for range 2 {
		require.NoError(t, MigrateAgentSimpleA07WithDB(DB))
	}
	var report AgentSimpleMigration
	require.NoError(t, DB.First(&report, 1).Error)
	assert.EqualValues(t, 800_000, report.Transferred)
	assert.Equal(t, report.Transferred, report.Credited)
	assert.Equal(t, 2, report.UserCount)
	assert.Equal(t, 500_000, getUserBalances(t, 9801).QuotaPaid)
	assert.Zero(t, getUserBalances(t, 9801).AffQuota)
	assert.Equal(t, 700_000, getUserBalances(t, 9801).AffHistoryQuota)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 9801).Update("aff_quota", 123).Error)
	require.NoError(t, MigrateAgentSimpleA07WithDB(DB))
	assert.Equal(t, 123, getUserBalances(t, 9801).AffQuota)
	assert.Equal(t, 500_000, getUserBalances(t, 9801).QuotaPaid)
}

func TestAgentSimpleMigrationRejectsExistingAgentFundsWithoutPartialTransfers(t *testing.T) {
	setupLedgerTestState(t)
	seedAffQuotaUser(t, 9811, 500_000, 500_000)
	require.NoError(t, DB.Create(&AgentRedemptionAccount{AgentUserID: 9811, AllocatedQuota: 100, AvailableQuota: 100}).Error)
	require.Error(t, MigrateAgentSimpleA07WithDB(DB))
	assert.Equal(t, 500_000, getUserBalances(t, 9811).AffQuota)
	assert.Zero(t, getUserBalances(t, 9811).QuotaPaid)
	var count int64
	require.NoError(t, DB.Model(&AgentSimpleMigration{}).Count(&count).Error)
	assert.Zero(t, count)
	require.NoError(t, DB.Model(&AgentRedemptionAccount{}).Where("agent_user_id = ?", 9811).Updates(map[string]any{"available_quota": 0, "allocated_quota": 0}).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 9811).Updates(map[string]any{"quota": common.MaxWalletQuota, "quota_paid": common.MaxWalletQuota}).Error)
	require.Error(t, MigrateAgentSimpleA07WithDB(DB), "wallet ceiling failure must roll back the migration")
	assert.Equal(t, 500_000, getUserBalances(t, 9811).AffQuota)
}

func TestAgentSimpleMigrationRequiresReviewOfPendingRetiredEntitlements(t *testing.T) {
	for _, route := range []string{"legacy", "cash"} {
		t.Run(route, func(t *testing.T) {
			setupLedgerTestState(t)
			seedAffQuotaUser(t, 9821, 500_000, 500_000)
			event := AgentCommissionOutbox{EventKey: "migration-pending-" + route, Kind: "payment", ReferenceID: "migration-pending-" + route, SourceUserID: 9821, Route: route, Payload: "{}", Status: "pending"}
			require.NoError(t, DB.Create(&event).Error)
			require.Error(t, MigrateAgentSimpleA07WithDB(DB))
			assert.EqualValues(t, 500_000, getUserBalances(t, 9821).AffQuota)
			assert.Zero(t, getUserBalances(t, 9821).QuotaPaid)
			var count int64
			require.NoError(t, DB.Model(&AgentSimpleMigration{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}
