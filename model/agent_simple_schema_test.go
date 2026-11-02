package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These release-era definitions preserve the existing storage and unique
// indexes while omitting A07 columns. They exercise an actual schema upgrade.
type a07LegacyReferral struct {
	Id            int     `gorm:"primaryKey;autoIncrement"`
	UserId        int     `gorm:"not null;index:idx_referral_commission_user,priority:1"`
	SourceUserId  int     `gorm:"not null;index"`
	ReferenceType string  `gorm:"type:varchar(32);not null;uniqueIndex:idx_referral_commission_ref,priority:1"`
	ReferenceId   string  `gorm:"type:varchar(128);not null;uniqueIndex:idx_referral_commission_ref,priority:2"`
	PaidQuota     int     `gorm:"type:bigint;not null"`
	RewardQuota   int     `gorm:"type:bigint;not null"`
	Rate          float64 `gorm:"type:decimal(10,6);not null"`
	CreatedAt     int64   `gorm:"bigint;autoCreateTime;index:idx_referral_commission_user,priority:2"`
}

func (a07LegacyReferral) TableName() string { return "referral_commissions" }

func TestAgentSimpleSchemaFreshUpgradeAndRepeatedMigration(t *testing.T) {
	if os.Getenv("TEST_A07_SCHEMA_COMPAT") != "1" {
		t.Skip("set TEST_A07_SCHEMA_COMPAT=1 with a disposable database")
	}
	var version string
	switch common.MainDatabaseType() {
	case common.DatabaseTypePostgreSQL:
		require.NoError(t, DB.Raw("SHOW server_version").Scan(&version).Error)
	case common.DatabaseTypeMySQL:
		require.NoError(t, DB.Raw("SELECT VERSION()").Scan(&version).Error)
	default:
		require.NoError(t, DB.Raw("SELECT sqlite_version()").Scan(&version).Error)
	}
	t.Logf("A07 real database: dialect=%s version=%s", common.MainDatabaseType(), version)
	for _, mode := range []string{"fresh", "upgrade"} {
		t.Run(mode, func(t *testing.T) {
			require.NoError(t, DB.Migrator().DropTable(&ReferralCommission{}))
			if mode == "upgrade" {
				require.NoError(t, DB.AutoMigrate(&a07LegacyReferral{}))
				require.NoError(t, DB.Create(&a07LegacyReferral{Id: 9701, UserId: 9702, SourceUserId: 9703, ReferenceType: "topup", ReferenceId: "a07-prior-release", PaidQuota: 3_000_000_000, RewardQuota: 150_000_000, Rate: .05}).Error)
			}
			for range 2 {
				require.NoError(t, DB.AutoMigrate(&ReferralCommission{}))
				require.NoError(t, DB.AutoMigrate(AgentPlatformModels()...))
				require.NoError(t, DB.AutoMigrate(&TopUp{}, &Redemption{}))
			}
			for _, schema := range []any{&AgentSimpleAccount{}, &AgentSimpleLedger{}, &AgentSimplePaymentOrder{}, &AgentSimpleCreditDebt{}, &AgentSimpleBill{}, &AgentSimpleReminder{}, &AgentSimpleExitRecord{}} {
				assert.True(t, DB.Migrator().HasTable(schema))
			}
			if mode == "upgrade" {
				var row ReferralCommission
				require.NoError(t, DB.First(&row, 9701).Error)
				assert.EqualValues(t, 3_000_000_000, row.PaidQuota)
				assert.Equal(t, ReferralCommissionStatusReleased, row.Status)
				assert.Zero(t, row.ReversedQuota)
				assert.Zero(t, row.MaturesAt)
				row.Id = 0
				require.Error(t, DB.Create(&row).Error, "upgrade preserves reference uniqueness")
			}
		})
	}
}
