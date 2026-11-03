package model

import (
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AgentSimpleMigration struct {
	ID          int   `json:"id" gorm:"primaryKey;autoIncrement:false"`
	Transferred int64 `json:"transferred_quota" gorm:"type:bigint;not null"`
	Credited    int64 `json:"credited_quota" gorm:"type:bigint;not null"`
	UserCount   int   `json:"user_count" gorm:"not null"`
	CompletedAt int64 `json:"completed_at" gorm:"not null"`
}

// MigrateAgentSimpleA07WithDB runs only against the caller's startup database.
// A single transaction preserves both sides of every invitation transfer and
// records totals. Existing agent financial history requires a separate plan.
func MigrateAgentSimpleA07WithDB(db *gorm.DB) error {
	if db == nil {
		return ErrAgentSimpleInvalid
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&AgentSimpleMigration{ID: 1}).Error; err != nil {
			return err
		}
		var migration AgentSimpleMigration
		if err := lockForUpdate(tx).First(&migration, 1).Error; err != nil {
			return err
		}
		if migration.CompletedAt > 0 {
			return nil
		}
		for _, check := range []struct {
			model any
			where string
		}{
			{&AgentRedemptionAccount{}, "allocated_quota <> 0 OR available_quota <> 0 OR reserved_quota <> 0 OR spent_quota <> 0"},
			{&AgentFundBatch{}, "original_quota <> 0"},
			{&AgentCommissionAccount{}, "earned_minor <> 0 OR debt_minor <> 0 OR held_minor <> 0"},
			{&AgentCommissionOutbox{}, "kind = 'payment' AND route IN ('legacy', 'cash') AND status <> 'done'"},
			{&AgentSimpleLedger{}, "id > 0"},
		} {
			var count int64
			if err := tx.Model(check.model).Where(check.where).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return errors.New("A07 migration requires review of existing agent financial history")
			}
		}
		var users []User
		if err := lockForUpdate(tx).Where("aff_quota <> 0").Order("id").Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			if user.AffQuota < 0 || migration.Transferred > math.MaxInt64-int64(user.AffQuota) {
				return ErrAgentSimpleInvariant
			}
			if _, err := CreditQuotaTx(tx, LedgerEntry{UserId: user.Id, Account: AccountPaid, Delta: user.AffQuota, Reason: "a07_referral_migration", ReferenceType: "a07_migration", ReferenceId: fmt.Sprint(user.Id), IdempotencyKey: fmt.Sprintf("a07_referral_migration:%d", user.Id)}); err != nil {
				return err
			}
			result := tx.Model(&User{}).Where("id = ? AND aff_quota = ?", user.Id, user.AffQuota).UpdateColumn("aff_quota", 0)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrAgentSimpleConflict
			}
			migration.Transferred += int64(user.AffQuota)
			migration.Credited += int64(user.AffQuota)
			migration.UserCount++
		}
		if migration.Transferred != migration.Credited {
			return ErrAgentSimpleInvariant
		}
		migration.CompletedAt = common.GetTimestamp()
		return tx.Save(&migration).Error
	})
}
