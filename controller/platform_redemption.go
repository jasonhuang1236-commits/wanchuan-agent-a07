package controller

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// Platform-code administration keeps its existing bound transaction proof,
// without re-exposing the retired agent procurement/approval workflows.
func PreparePlatformRedemption(c *gin.Context) {
	identity, ok := agentPlatformIdentity(c)
	if !ok {
		writeAgentSimpleError(c, model.ErrAgentSimplePermission)
		return
	}
	var request struct {
		Action    string          `json:"action"`
		RequestID string          `json:"request_id"`
		Payload   json.RawMessage `json:"payload"`
	}
	if common.DecodeJsonStrict(c.Request.Body, &request) != nil || !platformRedemptionAction(request.Action) {
		writeAgentSimpleError(c, model.ErrAgentSimpleInvalid)
		return
	}
	prepared, err := model.PrepareAgentLegacyRedemptionOperation(identity, request.Action, request.RequestID, request.Payload)
	if err != nil {
		writeAgentPurchaseError(c, err)
		return
	}
	prepared.StatusURL = "/api/redemption/operations/" + request.RequestID
	common.ApiSuccess(c, prepared)
}

func GetPlatformRedemptionOperation(c *gin.Context) {
	identity, ok := agentPlatformIdentity(c)
	if !ok {
		writeAgentSimpleError(c, model.ErrAgentSimplePermission)
		return
	}
	requestID := c.Param("request_id")
	if !isCanonicalAgentPlatformRequestID(requestID) {
		writeAgentSimpleError(c, model.ErrAgentSimpleInvalid)
		return
	}
	result, err := model.FindAgentPurchaseOperation(identity, requestID)
	if err != nil {
		writeAgentPurchaseError(c, err)
		return
	}
	if result == nil {
		common.ApiSuccess(c, model.AgentOperationStatus{RequestID: requestID, Status: "not_observed"})
		return
	}
	if !platformRedemptionAction(result.Action) {
		writeAgentSimpleError(c, model.ErrAgentSimplePermission)
		return
	}
	common.ApiSuccess(c, result)
}

func platformRedemptionAction(action string) bool {
	return strings.HasPrefix(action, "redemption.admin.") && (action == "redemption.admin.create" || action == "redemption.admin.update" || action == "redemption.admin.delete")
}
