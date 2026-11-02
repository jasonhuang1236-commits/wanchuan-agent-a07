package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentSimpleRechargePersistsOneIntentAndCreditsExactQuote(t *testing.T) {
	_, agent := simpleAccountingFixture(t)
	request := AgentSimpleRechargeRequest{RequestID: "a07-recharge-intent", Amount: "100.01", Kind: "recharge", PaymentMethod: "alipay"}
	quote, err := QuoteAgentSimpleRecharge(request)
	require.NoError(t, err)
	assert.EqualValues(t, 50_005_000, quote.Quota)
	assert.EqualValues(t, 9001, quote.PayFen)
	first, err := CreateAgentSimpleRecharge(agent, request, "a07-recharge-first", "https://pay.example.test", map[string]string{"trade_no": "a07-recharge-first"})
	require.NoError(t, err)
	replay, err := CreateAgentSimpleRecharge(agent, request, "a07-recharge-second", "https://other.example.test", map[string]string{"trade_no": "a07-recharge-second"})
	require.NoError(t, err)
	assert.Equal(t, first, replay)
	changed := request
	changed.Amount = "100.02"
	_, err = CreateAgentSimpleRecharge(agent, changed, "a07-recharge-changed", "https://pay.example.test", map[string]string{})
	require.Error(t, err, "the same payment intent cannot silently change its amount")
	for range 2 {
		_, err = RechargeEpay(first.TradeNo, "alipay", "90.01", "", simpleVerifiedEpayFacts(t, first.TradeNo, "90.01"))
		require.NoError(t, err)
	}
	var account AgentSimpleAccount
	require.NoError(t, DB.First(&account, agent.UserID).Error)
	assert.EqualValues(t, quote.Quota, account.CashQuota, "payment rounding must not change the promised face value")
	assert.Zero(t, getUserBalances(t, agent.UserID).QuotaPaid)
	var count int64
	require.NoError(t, DB.Model(&TopUp{}).Where("agent_simple_top_up = ?", true).Count(&count).Error)
	assert.EqualValues(t, 1, count)
	require.NoError(t, DB.Model(&ReferralCommission{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestAgentSimpleRepaymentAcceptsExactFenWhilePaused(t *testing.T) {
	_, agent := simpleAccountingFixture(t)
	require.NoError(t, DB.Model(&AgentSimpleAccount{}).Where("agent_user_id = ?", agent.UserID).Updates(map[string]any{"status": AgentSimpleStatusPaused, "credit_limit_quota": 10_000_000, "credit_outstanding_quota": 4_525_000}).Error)
	require.NoError(t, DB.Create(&AgentSimpleCreditDebt{AgentUserID: agent.UserID, CodeID: 87001, Quota: 4_525_000}).Error)
	request := AgentSimpleRechargeRequest{RequestID: "a07-repayment-exact", Amount: "9.05", Kind: "repayment", PaymentMethod: "alipay"}
	order, err := CreateAgentSimpleRecharge(agent, request, "a07-repayment-exact", "https://pay.example.test", map[string]string{})
	require.NoError(t, err)
	_, err = RechargeEpay(order.TradeNo, "alipay", "9.05", "", simpleVerifiedEpayFacts(t, order.TradeNo, "9.05"))
	require.NoError(t, err)
	var account AgentSimpleAccount
	require.NoError(t, DB.First(&account, agent.UserID).Error)
	assert.Zero(t, account.CreditOutstandingQuota)
	assert.Zero(t, account.CashQuota)
	assert.Equal(t, AgentSimpleStatusPaused, account.Status)
	request.RequestID = "a07-repay-no-debt"
	_, err = CreateAgentSimpleRecharge(agent, request, "a07-repay-no-debt", "https://pay.example.test", map[string]string{})
	require.ErrorIs(t, err, ErrAgentSimpleInvalid)
}

func TestAgentSimpleRepaymentPreservesOverpaymentAfterAnotherRepayment(t *testing.T) {
	root, agent := simpleAccountingFixture(t)
	require.NoError(t, DB.Model(&AgentSimpleAccount{}).Where("agent_user_id = ?", agent.UserID).Updates(map[string]any{"credit_limit_quota": 10_000_000, "credit_outstanding_quota": 4_500_000}).Error)
	require.NoError(t, DB.Create(&AgentSimpleCreditDebt{AgentUserID: agent.UserID, CodeID: 87002, Quota: 4_500_000}).Error)
	request := AgentSimpleRechargeRequest{RequestID: "a07-overpayment-order", Amount: "9.00", Kind: "repayment", PaymentMethod: "alipay"}
	order, err := CreateAgentSimpleRecharge(agent, request, "a07-overpayment-order", "https://pay.example.test", map[string]string{})
	require.NoError(t, err)
	_, err = RepayAgentSimpleCredit(root, AgentSimpleRepaymentRequest{RequestID: "a07-overpayment-other", AgentUserID: agent.UserID, Quota: 2_250_000, Reason: "Synthetic separate payment"})
	require.NoError(t, err)
	_, err = RechargeEpay(order.TradeNo, "alipay", "9.00", "", simpleVerifiedEpayFacts(t, order.TradeNo, "9.00"))
	require.NoError(t, err)
	var account AgentSimpleAccount
	require.NoError(t, DB.First(&account, agent.UserID).Error)
	assert.Zero(t, account.CreditOutstandingQuota)
	assert.EqualValues(t, 2_500_000, account.CashQuota)
}

func TestAgentSimpleConcurrentRechargeIntentCreatesOneOrder(t *testing.T) {
	_, agent := simpleAccountingFixture(t)
	request := AgentSimpleRechargeRequest{RequestID: "a07-concurrent-recharge", Amount: "100.00", Kind: "recharge", PaymentMethod: "alipay"}
	type result struct {
		view *AgentSimpleRechargeView
		err  error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for _, trade := range []string{"a07-concurrent-first", "a07-concurrent-second"} {
		go func() {
			<-start
			view, err := CreateAgentSimpleRecharge(agent, request, trade, "https://pay.example.test", map[string]string{"trade_no": trade})
			results <- result{view, err}
		}()
	}
	close(start)
	first, second := <-results, <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	assert.Equal(t, first.view, second.view)
	var count int64
	require.NoError(t, DB.Model(&TopUp{}).Where("user_id = ?", agent.UserID).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestAgentSimplePendingRechargeSurvivesCancellationWithoutRestoringAgentStatus(t *testing.T) {
	root, agent := simpleAccountingFixture(t)
	request := AgentSimpleRechargeRequest{RequestID: "a07-cancel-pending", Amount: "100", Kind: "recharge", PaymentMethod: "alipay"}
	order, err := CreateAgentSimpleRecharge(agent, request, "a07-cancel-pending", "https://pay.example.test", map[string]string{})
	require.NoError(t, err)
	_, err = SetAgentSimpleStatus(root, AgentSimpleStatusRequest{RequestID: "a07-cancel-pause", AgentUserID: agent.UserID, Status: AgentSimpleStatusPaused, Reason: "Synthetic pause"})
	require.NoError(t, err)
	_, err = SetAgentSimpleStatus(root, AgentSimpleStatusRequest{RequestID: "a07-cancel-final", AgentUserID: agent.UserID, Status: AgentSimpleStatusCancelled, Reason: "Synthetic cancellation"})
	require.NoError(t, err)
	_, err = RechargeEpay(order.TradeNo, "alipay", "90.00", "", simpleVerifiedEpayFacts(t, order.TradeNo, "90.00"))
	require.NoError(t, err, "a verified payment must not disappear after agent cancellation")
	var account AgentSimpleAccount
	require.NoError(t, DB.First(&account, agent.UserID).Error)
	assert.Equal(t, AgentSimpleStatusCancelled, account.Status)
	assert.EqualValues(t, 50_000_000, account.CashQuota)
	request.RequestID = "a07-cancel-new-pay"
	_, err = CreateAgentSimpleRecharge(agent, request, "a07-cancel-new-pay", "https://pay.example.test", map[string]string{})
	require.ErrorIs(t, err, ErrAgentSimpleUnavailable)
	_, err = SettleAgentSimpleExit(root, AgentSimpleExitRequest{RequestID: "a07-late-pay-exit", AgentUserID: agent.UserID, Quota: 50_000_000, Source: "cash", Target: "refund", Reason: "Return late payment", Evidence: "Synthetic refund evidence"})
	require.NoError(t, err)
}

func simpleVerifiedEpayFacts(t *testing.T, reference, amount string) AgentVerifiedPaymentFacts {
	t.Helper()
	facts, ok := NewAgentVerifiedPaymentFacts(PaymentProviderEpay, "provider-"+reference, "synthetic-merchant", "live", "CNY", amount, false)
	require.True(t, ok)
	return facts
}

func TestAgentSimpleRechargeRejectsInvalidPrecisionAndUnavailableKinds(t *testing.T) {
	for _, amount := range []string{"-1", "0", "0.001", "1e3", "9007199254740992", "NaN"} {
		_, err := QuoteAgentSimpleRecharge(AgentSimpleRechargeRequest{RequestID: "a07-invalid-amount", Amount: amount, Kind: "recharge", PaymentMethod: "alipay"})
		require.ErrorIs(t, err, ErrAgentSimpleInvalid, amount)
	}
	_, err := QuoteAgentSimpleRecharge(AgentSimpleRechargeRequest{RequestID: "a07-invalid-kind", Amount: "10", Kind: "withdrawal", PaymentMethod: "alipay"})
	require.ErrorIs(t, err, ErrAgentSimpleInvalid)
	assert.EqualValues(t, 500_000, common.QuotaPerUnit)
}

func TestAgentSimplePaymentNeedsExplicitFeeButRechargeCanUseConfiguredMethod(t *testing.T) {
	root, agent := simpleAccountingFixture(t)
	_, err := QuoteAgentSimplePayment("wxpay", 500_000, 100)
	require.ErrorIs(t, err, ErrAgentSimpleUnavailable, "unknown channel costs must not silently become zero")
	recharge, err := GetAgentSimpleAvailablePaymentMethods(agent, "recharge")
	require.NoError(t, err)
	assert.Contains(t, recharge, AgentSimplePaymentFee{PaymentMethod: "wxpay", Enabled: true})
	_, err = UpdateAgentSimplePaymentFee(root, AgentSimplePaymentFeeRequest{RequestID: "a07-explicit-channel-fee", PaymentMethod: "wxpay", FeeBPS: 100, Enabled: true, Reason: "Synthetic confirmed fee"})
	require.NoError(t, err)
	_, err = QuoteAgentSimplePayment("wxpay", 500_000, 90)
	require.ErrorIs(t, err, ErrAgentSimpleInvalid, "the minimum price includes the channel fee")
	quote, err := QuoteAgentSimplePayment("wxpay", 500_000, 91)
	require.NoError(t, err)
	assert.Zero(t, quote.ProfitQuota)
}
