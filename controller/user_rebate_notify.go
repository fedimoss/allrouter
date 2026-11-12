package controller

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// notifyInviteRebateChange 消费分佣资格开关变化后的邮件通知：
// 先到 users 表查询该用户的邮箱，有值则渲染模板并通过 mailProviderId 对应的
// 邮件配置发送到该邮箱；无值或发送失败时返回给前端的提示信息（空串表示已发送，
// 无需提示）。邮件结果不影响开关本身的保存。
func notifyInviteRebateChange(c *gin.Context, userId int, enabled bool, mailProviderId int) string {
	var user model.User
	if err := model.DB.Omit("password").Where("id = ?", userId).First(&user).Error; err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("failed to load user %d for rebate notify: %s", userId, err.Error()))
		return "消费分佣资格已更新，但查询用户邮箱失败，未发送通知邮件"
	}
	email := strings.TrimSpace(user.Email)
	if email == "" {
		return "该用户未配置邮箱，未发送消费分佣资格变更通知"
	}
	// users 表的邮箱来自各类写入路径，未强制校验格式；发送前必须校验，
	// 防止 "a@x.com;b@y.com" 之类的串值被 SMTP 按分号拆分后转发给任意收件人
	if err := common.Validate.Var(email, "email"); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("skip sending rebate email to invalid address of user %d", userId))
		return "该用户邮箱格式无效，未发送消费分佣资格变更通知"
	}
	// 站点名优先取发件服务商自己配置的 SiteName（主站管理员编辑分站用户时，
	// 请求上下文没有分站租户信息，直接取请求上下文会错用主站名），
	// 未配置时再回退到请求上下文（分站域名访问）→ 主站全局名
	systemName := ""
	if mailProviderId > 0 {
		if cfg, err := model.GetProviderPublicConfigCached(mailProviderId); err == nil && cfg != nil {
			systemName = strings.TrimSpace(cfg.SiteName)
		}
	}
	if systemName == "" {
		systemName = getRequestSystemName(c)
	}
	templateName := "user_rebate_disabled.html"
	subject := fmt.Sprintf("%s消费分佣资格已关闭 / Consumption Rebate Qualification Disabled", systemName)
	if enabled {
		templateName = "user_rebate_enabled.html"
		subject = fmt.Sprintf("%s消费分佣资格已开通 / Consumption Rebate Qualification Enabled", systemName)
	}
	content, err := common.RenderEmailTemplate(templateName, map[string]any{
		"SystemName":  systemName,
		"DisplayName": user.DisplayName,
	})
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("failed to render rebate email for user %d: %s", userId, err.Error()))
		return "消费分佣资格已更新，但通知邮件生成失败，未发送"
	}
	if err := service.SendProviderMail(mailProviderId, subject, email, content); err != nil {
		// SMTP 错误可能包含主机、端口等配置信息，仅记录在服务端日志
		logger.LogError(c.Request.Context(), fmt.Sprintf("failed to send rebate email to %s: %s", email, err.Error()))
		return "消费分佣资格已更新，但通知邮件发送失败，请检查邮件服务配置"
	}
	return ""
}
