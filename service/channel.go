package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

const UpstreamServiceUnavailableMessage = "Service temporarily unavailable. Please contact the administrator."

var upstreamAccountBalanceErrorMarkers = []string{
	"insufficient balance",
	"balance is insufficient",
	"not enough balance",
	"balance is too low",
	"credit balance is too low",
	"insufficient credit",
	"insufficient funds",
	"no credits left",
	"out of credits",
	"payment required",
	"insufficient_quota",
	"billing_hard_limit_reached",
	"you exceeded your current quota",
	"insufficient account balance",
	"balance not enough",
	"account has insufficient balance",
	"quota exhausted",
	"credits exhausted",
	"credit exhausted",
	"余额不足",
	"余额已用完",
	"账户欠费",
	"账号欠费",
	"请充值",
	"需要充值",
}

// IsUpstreamAccountBalanceError identifies explicit upstream billing failures.
// It must only be called for errors returned while relaying to an upstream channel.
func IsUpstreamAccountBalanceError(err *types.NewAPIError) bool {
	if err == nil || err.StatusCode < http.StatusBadRequest || err.StatusCode >= http.StatusInternalServerError {
		return false
	}

	message := strings.ToLower(err.Error())
	for _, marker := range upstreamAccountBalanceErrorMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// NormalizeUpstreamAccountBalanceError hides channel account details from clients
// and turns the failure into a retryable upstream service error.
func NormalizeUpstreamAccountBalanceError(err *types.NewAPIError) *types.NewAPIError {
	if !IsUpstreamAccountBalanceError(err) {
		return err
	}
	return types.NewErrorWithStatusCode(
		errors.New(UpstreamServiceUnavailableMessage),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
	)
}

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

// disable & notify
func DisableChannel(channelError types.ChannelError, reason string) {
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）发生错误，准备禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被禁用 / Channel Disabled", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
		notify := dto.NewNotify(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content, nil)
		notify.TemplateName = "channel_disabled.html"
		notify.TemplateData = map[string]any{
			"ChannelName": channelError.ChannelName,
			"ChannelId":   channelError.ChannelId,
			"Reason":      reason,
		}
		NotifyRootUserWithNotify(notify)
	}
}

func EnableChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被启用 / Channel Enabled", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		notify := dto.NewNotify(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content, nil)
		notify.TemplateName = "channel_enabled.html"
		notify.TemplateData = map[string]any{
			"ChannelName": channelName,
			"ChannelId":   channelId,
		}
		NotifyRootUserWithNotify(notify)
	}
}

func ShouldDisableChannel(err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}
	if types.IsChannelError(err) {
		return true
	}
	if IsUpstreamAccountBalanceError(err) {
		return true
	}
	if types.IsSkipRetryError(err) {
		return false
	}
	if operation_setting.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}

	lowerMessage := strings.ToLower(err.Error())
	search, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
	return search
}

func ShouldEnableChannel(newAPIError *types.NewAPIError, status int) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled {
		return false
	}
	return true
}
