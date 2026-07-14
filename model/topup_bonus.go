package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/shopspring/decimal"
	"gorm.io/gorm/clause"
)

// TopUpBonusGrant 记录用户已享受过的"充值赠送"档位，用于"每用户每档仅一次"幂等。
// (user_id, rule_id) 唯一：同一用户对同一条规则（档位）只赠送一次。
type TopUpBonusGrant struct {
	Id          int     `json:"id" gorm:"primaryKey"`
	UserId      int     `json:"user_id" gorm:"uniqueIndex:ux_topup_bonus_user_rule"`
	RuleId      string  `json:"rule_id" gorm:"type:varchar(64);uniqueIndex:ux_topup_bonus_user_rule"`
	TradeNo     string  `json:"trade_no" gorm:"type:varchar(64)"` // 触发本次赠送的充值订单号，便于追溯
	Quota       int     `json:"quota"`                            // 实际赠送的 quota（内部整数额度）
	Amount      float64 `json:"amount"`                           // 赠送金额（用户币种原值，如 2 或 2.5，直观可读）
	Currency    string  `json:"currency" gorm:"type:varchar(16)"` // 赠送币种 USD/CNY（跟随用户充值币种）
	CreatedTime int64   `json:"created_time" gorm:"bigint"`
}

func (TopUpBonusGrant) TableName() string {
	return "topup_bonus_grants"
}

// 服务商充值赠送沿用 provider_options，以独立键保存规则与总开关。
const (
	ProviderTopUpGiftRulesOptionKey   = "topup_gift.rules"
	ProviderTopUpGiftEnabledOptionKey = "topup_gift.enabled"
)

// TopUpGiftRule 对应 option TopUpGiftRules 中的一条规则。
type TopUpGiftRule struct {
	Id        string  `json:"id"`        // 规则稳定标识，作为幂等键
	Threshold float64 `json:"threshold"` // 充值门槛（用户币种数值，如 10 表示 $10 或 ¥10）
	Bonus     float64 `json:"bonus"`     // 赠送金额（用户币种数值）
}

// TopUpGiftConfig 汇总总开关与规则，供公开接口和实际赠送流程共同使用。
type TopUpGiftConfig struct {
	Enabled bool            `json:"enabled"`
	Rules   []TopUpGiftRule `json:"rules"`
}

// parseTopUpGiftRulesFrom 将持久化 JSON 解析为稳定数组，空配置也返回 [] 而非 null。
func parseTopUpGiftRulesFrom(str string) ([]TopUpGiftRule, error) {
	if strings.TrimSpace(str) == "" {
		return []TopUpGiftRule{}, nil
	}
	var rules []TopUpGiftRule
	if err := common.Unmarshal([]byte(str), &rules); err != nil {
		return nil, fmt.Errorf("解析充值赠送规则失败: %w", err)
	}
	if rules == nil {
		rules = []TopUpGiftRule{}
	}
	return rules, nil
}

// LoadTopUpGiftConfig 按 provider 维度加载充值赠送配置（规则 + 启用开关）。
// providerId == 0 读主站全局 option（common.TopUpGiftRules / TopUpGiftEnabled）；
// providerId > 0 读服务商 provider_options 的 topup_gift.rules / topup_gift.enabled。
// 服务商未配置 enabled 时返回 false（需显式启用才生效）。
func LoadTopUpGiftConfig(providerId int) (TopUpGiftConfig, error) {
	var rulesStr string
	enabled := false
	if providerId == 0 {
		common.OptionMapRWMutex.RLock()
		rulesStr = common.TopUpGiftRules
		enabled = common.TopUpGiftEnabled
		common.OptionMapRWMutex.RUnlock()
	} else {
		// 单次查询同时读取服务商开关和规则，避免两个接口形成不同的数据来源。
		options, err := GetProviderOptions(providerId)
		if err != nil {
			return TopUpGiftConfig{}, err
		}
		for _, option := range options {
			switch option.Key {
			case ProviderTopUpGiftRulesOptionKey:
				rulesStr = option.Value
			case ProviderTopUpGiftEnabledOptionKey:
				enabled = option.Value == "true"
			}
		}
	}

	rules, err := parseTopUpGiftRulesFrom(rulesStr)
	if err != nil {
		return TopUpGiftConfig{}, err
	}
	return TopUpGiftConfig{Enabled: enabled, Rules: rules}, nil
}

// claimTopUpBonusGrant 原子占用名额（OnConflict DoNothing）。
// 返回 true 表示占用成功（该用户该档此前未享受），false 表示已享受或写入失败。
func claimTopUpBonusGrant(userId int, ruleId, tradeNo string, quota int, amount float64, currency string) bool {
	grant := &TopUpBonusGrant{
		UserId:      userId,
		RuleId:      ruleId,
		TradeNo:     tradeNo,
		Quota:       quota,
		Amount:      amount,
		Currency:    currency,
		CreatedTime: common.GetTimestamp(),
	}
	result := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "rule_id"}},
		DoNothing: true,
	}).Create(grant)
	if result.Error != nil {
		common.SysError(fmt.Sprintf("topup bonus claim grant failed for user %d rule %s: %s", userId, ruleId, result.Error.Error()))
		return false
	}
	return result.RowsAffected > 0
}

// releaseTopUpBonusGrant 发放失败时释放名额，允许用户下次再享受该档。
func releaseTopUpBonusGrant(userId int, ruleId string) {
	DB.Where("user_id = ? AND rule_id = ?", userId, ruleId).Delete(&TopUpBonusGrant{})
}

// GrantTopUpBonus 在用户单次充值成功后调用。
// 按"最高命中档、每用户每档仅一次"创建一张兑换码并自动兑换给用户。
// 币种跟随用户充值币种：USD 用户按美元折算赠送，CNY 用户按人民币折算赠送。
// 任何错误仅记日志，绝不影响已成功的充值主流程。
//
// 参数：
//   - userId: 充值用户
//   - providerId: 订单维度的服务商 ID（topUp.ProviderId），0=主站。用于分流读主站 options 或服务商 provider_options
//   - moneyUSD: 本次充值的美元归一化金额（topUp.Money）
//   - tradeNo: 本次充值订单号（用于追溯）
func GrantTopUpBonus(userId int, providerId int, moneyUSD float64, tradeNo string) {
	if userId <= 0 || moneyUSD <= 0 {
		return
	}
	// 按 provider 维度加载配置：providerId == 0 读主站 common.TopUpGiftRules；
	// providerId > 0 读 provider_options 的 topup_gift.rules / topup_gift.enabled。
	// 注意：这里必须用订单维度的 providerId（topUp.ProviderId），不能用 users 表的 user.ProviderId，
	// 因为用户可能在主站注册但在服务商域名下充值，二者不一致会导致分流错误。
	config, err := LoadTopUpGiftConfig(providerId)
	if err != nil {
		common.SysError("topup bonus load config failed: " + err.Error())
		return
	}
	// 总开关：未启用则完全不处理（即使配置了规则也不生效）
	if !config.Enabled {
		return
	}
	rules := config.Rules
	if len(rules) == 0 {
		return
	}
	user, err := GetUserById(userId, false)
	if err != nil || user == nil {
		return
	}
	info := GetDisplayCurrencyInfoByTimezone(user.Timezone)

	// 用户币种下的充值数值：USD 直接用 moneyUSD；CNY 按汇率换算
	userValue := moneyUSD
	if info.Currency == "CNY" {
		if info.Rate <= 0 {
			common.SysError(fmt.Sprintf("topup bonus skip: invalid cny rate for user %d", userId))
			return
		}
		userValue = moneyUSD * info.Rate
	}

	// 筛选命中（threshold <= userValue）的规则，取 threshold 最大的（最高命中档）
	// 含小容差 0.001：避免浮点还原误差让"正好等于门槛"的充值（如 CNY ¥10 还原成 9.9999）漏判
	matched := -1
	for i, r := range rules {
		if r.Id == "" || r.Threshold <= 0 || r.Bonus <= 0 {
			continue
		}
		if r.Threshold <= userValue+0.001 {
			if matched < 0 || r.Threshold > rules[matched].Threshold {
				matched = i
			}
		}
	}
	if matched < 0 {
		return
	}
	rule := rules[matched]

	// 金额按 6 位小数计算；USD 直接换算，CNY 先按发放时汇率归一化为 USD。
	bonusAmount := decimal.NewFromFloat(rule.Bonus).Round(redemptionAmountScale)
	bonusUSD := bonusAmount
	if info.Currency == "CNY" {
		bonusUSD = bonusAmount.
			Div(decimal.NewFromFloat(info.Rate)).
			Round(redemptionAmountScale)
	}
	bonusQuota := common.QuotaFromDecimal(
		bonusUSD.Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
	)
	if bonusQuota <= 0 {
		return
	}

	// 幂等占用：每用户每档一次。占用失败=已享受 → 本次不送，且不降级送低档。
	if !claimTopUpBonusGrant(userId, rule.Id, tradeNo, bonusQuota, bonusAmount.InexactFloat64(), info.Currency) {
		return
	}

	// 创建兑换码（系统赠送，UserId=0）
	redemption := &Redemption{
		ProviderId:  user.ProviderId,
		UserId:      0,
		Key:         common.GetUUID(),
		Status:      common.RedemptionCodeStatusEnabled,
		Name:        "充值赠送",
		Quota:       bonusQuota,
		CreatedTime: common.GetTimestamp(),
	}
	if err := redemption.Insert(); err != nil {
		common.SysError(fmt.Sprintf("topup bonus create redemption failed for user %d: %s", userId, err.Error()))
		releaseTopUpBonusGrant(userId, rule.Id)
		return
	}
	// 自动兑换给用户（内部完成加 quota+reward_quota、写 TopUp 流水、标记码已用、日志）
	// 将发放时的原始面值传入兑换流程，避免查看记录时按最新汇率重新计算。
	if _, err := redeemWithOriginalValue(redemption.Key, userId, redemptionOriginalValue{
		Amount:   bonusAmount.InexactFloat64(),
		Currency: info.Currency,
	}); err != nil {
		common.SysError(fmt.Sprintf("topup bonus redeem failed for user %d: %s", userId, err.Error()))
		releaseTopUpBonusGrant(userId, rule.Id)
		return
	}

	RecordLog(userId, LogTypeSystem, fmt.Sprintf("充值赠送命中档位 %g 送 %g（%s），已自动兑换 %s（订单 %s）",
		rule.Threshold, rule.Bonus, info.Currency, logger.LogQuota(bonusQuota), tradeNo))
}
