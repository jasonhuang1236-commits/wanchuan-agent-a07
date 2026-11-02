package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type AgentSimpleRechargeRequest struct {
	RequestID     string `json:"request_id"`
	Amount        string `json:"amount"`
	Kind          string `json:"kind"`
	PaymentMethod string `json:"payment_method"`
}

type AgentSimpleRechargeQuote struct {
	Quota  int64 `json:"quota"`
	PayFen int64 `json:"pay_fen"`
}

type AgentSimpleRechargeView struct {
	TopUpID int               `json:"topup_id"`
	TradeNo string            `json:"trade_no"`
	URL     string            `json:"url"`
	Data    map[string]string `json:"data"`
	Quota   int64             `json:"quota"`
	PayYuan float64           `json:"pay_yuan"`
}

// The gateway accepts whole fen; preserve sub-fen change rather than strand debt.
func validateAgentSimpleRepayment(outstanding, paidQuota int64) error {
	if validateAgentSimpleAmount(outstanding) != nil || validateAgentSimpleAmount(paidQuota) != nil {
		return ErrAgentSimpleInvalid
	}
	maximum := decimal.NewFromInt(outstanding).Mul(decimal.NewFromInt(100)).Div(decimal.NewFromFloat(common.QuotaPerUnit)).Ceil().Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Div(decimal.NewFromInt(100))
	if decimal.NewFromInt(paidQuota).GreaterThan(maximum) {
		return ErrAgentSimpleInvalid
	}
	return nil
}

func QuoteAgentSimpleRecharge(request AgentSimpleRechargeRequest) (*AgentSimpleRechargeQuote, error) {
	if validateAgentSimpleRequestID(request.RequestID) != nil || (request.Kind != "recharge" && request.Kind != "repayment") || request.PaymentMethod == "" || len(request.PaymentMethod) > 50 || !isSimpleEpayMethod(request.PaymentMethod) || len(request.Amount) > 24 || request.Amount == "" {
		return nil, ErrAgentSimpleInvalid
	}
	parts := strings.Split(request.Amount, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && (parts[1] == "" || len(parts[1]) > 2)) {
		return nil, ErrAgentSimpleInvalid
	}
	for _, part := range parts {
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return nil, ErrAgentSimpleInvalid
			}
		}
	}
	amount, err := decimal.NewFromString(request.Amount)
	if err != nil || !amount.IsPositive() {
		return nil, ErrAgentSimpleInvalid
	}
	face, paid := amount, amount
	if request.Kind == "recharge" {
		paid = amount.Mul(decimal.NewFromInt(9)).Div(decimal.NewFromInt(10)).Round(2)
	} else {
		face = amount.Mul(decimal.NewFromInt(10)).Div(decimal.NewFromInt(9))
	}
	quota, err := common.WalletQuotaFromDecimalStrict(face.Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Truncate(0))
	if err != nil || quota <= 0 || !paid.IsPositive() || paid.Shift(2).GreaterThan(decimal.NewFromInt(common.MaxWalletQuota)) {
		return nil, ErrAgentSimpleInvalid
	}
	return &AgentSimpleRechargeQuote{Quota: int64(quota), PayFen: paid.Shift(2).IntPart()}, nil
}

// CreateAgentSimpleRecharge stores the order and its signed payment form together.
// The operation result is the durable recovery response for the same request ID.
func CreateAgentSimpleRecharge(identity AuthSessionIdentity, request AgentSimpleRechargeRequest, tradeNo, paymentURL string, paymentParams map[string]string) (*AgentSimpleRechargeView, error) {
	quote, err := QuoteAgentSimpleRecharge(request)
	if err != nil || strings.TrimSpace(tradeNo) == "" || strings.TrimSpace(paymentURL) == "" || paymentParams == nil {
		return nil, ErrAgentSimpleInvalid
	}
	var result AgentSimpleRechargeView
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := requireAgentSimpleHistoryTx(tx, identity, identity.UserID); err != nil {
			return err
		}
		actor, err := agentSimpleActorTx(tx, identity)
		if err != nil || actor.Role >= common.RoleAdminUser {
			return ErrAgentSimplePermission
		}
		// Canonical economic quantities make 100 and 100.00 the same intent.
		signature := struct {
			Kind, Method string
			Quota, Fen   int64
		}{request.Kind, request.PaymentMethod, quote.Quota, quote.PayFen}
		operation, replay, err := claimAgentRedemptionOperation(tx, "simple_recharge", request.RequestID, RedemptionScope{UserID: identity.UserID, Role: actor.Role}, identity.UserID, signature)
		if err != nil {
			return err
		}
		if replay {
			return common.UnmarshalJsonStr(operation.Result, &result)
		}
		account, err := lockAgentSimpleAccountTx(tx, identity.UserID, true)
		if err != nil {
			return err
		}
		if account.Status == AgentSimpleStatusCancelled {
			return ErrAgentSimpleUnavailable
		}
		if request.Kind == "recharge" {
			if err := requireAgentSimpleSalesTx(tx, identity, identity.UserID); err != nil {
				return err
			}
		} else {
			paidQuota, err := common.WalletQuotaFromDecimalStrict(decimal.NewFromInt(quote.PayFen).Div(decimal.NewFromInt(100)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
			if err != nil || validateAgentSimpleRepayment(account.CreditOutstandingQuota, int64(paidQuota)) != nil {
				return ErrAgentSimpleInvalid
			}
		}
		order := TopUp{AgentSimpleTopUp: true, AgentSimpleKind: request.Kind, AgentSimpleQuota: quote.Quota, UserId: actor.Id, Amount: decimal.NewFromInt(quote.Quota).Div(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart(), Money: decimal.NewFromInt(quote.PayFen).Div(decimal.NewFromInt(100)).InexactFloat64(), TradeNo: tradeNo, PaymentMethod: request.PaymentMethod, PaymentProvider: PaymentProviderEpay, CreateTime: common.GetTimestamp(), Status: common.TopUpStatusPending}
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		result = AgentSimpleRechargeView{TopUpID: order.Id, TradeNo: tradeNo, URL: paymentURL, Data: paymentParams, Quota: quote.Quota, PayYuan: order.Money}
		encoded, err := common.Marshal(result)
		if err != nil {
			return err
		}
		return tx.Model(operation).Update("result", string(encoded)).Error
	})
	return &result, err
}
