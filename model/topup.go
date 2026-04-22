package model

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// TopUp 充值记录数据模型
// 记录用户所有的充值行为，包括在线支付、兑换码、订阅等
type TopUp struct {
	Id            int     `json:"id"`                                                     // 充值记录ID（主键）
	UserId        int     `json:"user_id" gorm:"index"`                                   // 用户ID（索引）
	Amount        int64   `json:"amount"`                                                 // 充值额度（基础额度，未计算分组倍率）
	Money         float64 `json:"money"`                                                  // 支付金额（美元）
	TradeNo       string  `json:"trade_no" gorm:"unique;type:varchar(255);index"`         // 交易号（唯一索引）
	PaymentMethod string  `json:"payment_method" gorm:"type:varchar(50)"`                 // 支付方式（stripe/creem/waffo/epay等）
	BizType       string  `json:"biz_type" gorm:"type:varchar(32);default:payment;index"` // 业务类型（payment/subscription/redemption）
	SourceID      int     `json:"source_id" gorm:"default:0;index"`                       // 关联源ID（订阅ID/兑换码ID等）
	CreateTime    int64   `json:"create_time"`                                            // 创建时间（Unix时间戳）
	CompleteTime  int64   `json:"complete_time"`                                          // 完成时间（Unix时间戳）
	Status        string  `json:"status"`                                                 // 状态（pending/success/failed等）
	DisplayName   string  `json:"display_name" gorm:"->;-:migration;column:display_name"` // 用户昵称（从users表关联）
}
type TopUpDetails struct {
	*TopUp     `json:"topup"`
	Level1Rate *TopUpRebate `json:"level1_rate"`
}

// 业务类型常量定义
const (
	TopUpBizTypePayment      = "payment"      // 在线支付充值（主要类型，支持返利）
	TopUpBizTypeSubscription = "subscription" // 订阅账单（不支持返利）
	TopUpBizTypeRedemption   = "redemption"   // 兑换码充值（不支持返利）
)

// normalizeTopUpBizType 规范化业务类型
// 如果未指定业务类型，默认返回 payment 类型
func normalizeTopUpBizType(bizType string) string {
	if bizType == "" {
		return TopUpBizTypePayment
	}
	return bizType
}

// normalizeTopUps 规范化充值记录列表
// 遍历列表，为每个充值记录设置正确的业务类型
func normalizeTopUps(topups []*TopUp) {
	for _, topUp := range topups {
		if topUp == nil {
			continue
		}
		bizType := topUp.GetBizType()
		if bizType == "invite_rebate" {
			topUp.PaymentMethod = "invite_rebate"
			money := decimal.NewFromBigInt(big.NewInt(topUp.Amount), 0).Div(decimal.NewFromFloat(common.QuotaPerUnit)).InexactFloat64()
			topUp.Money = money

			//获取充值基本单位
			pr, err := strconv.ParseFloat(common.OptionMap["Price"], 64)
			if err != nil {
				logger.LogError(context.Background(), "获取充值基本单位失败")
				topUp.Amount = -1

			} else {
				//换算成数量

				topUp.Amount = int64(math.Round(money / pr))
			}
			topUp.Status = common.TopUpStatusSuccess

		}
		topUp.BizType = bizType
	}
}

const topUpRecordAlias = "topup_records"

func withAllTopUpRecords(tx *gorm.DB) *gorm.DB {
	return tx.Table("(?) AS "+topUpRecordAlias, tx.Raw(`
		SELECT
			t.id,
			t.user_id,
			t.amount,
			t.money,
			t.trade_no,
			t.payment_method,
			t.create_time,
			t.complete_time,
			t.status,
			t.biz_type,
			t.source_id,
			COALESCE(users.display_name, '') AS display_name
		FROM top_ups AS t
		LEFT JOIN users ON users.id = t.user_id

		UNION ALL

		SELECT
			tr.id,
			tr.inviter_id AS user_id,
			tr.rebate_quota AS amount,
			0 AS money,
			tr.trade_no AS trade_no,
			tr.payment_method AS payment_method,
			tr.created_at AS create_time,
			0 AS complete_time,
			'' AS status,
			'invite_rebate' AS biz_type,
			0 AS source_id,
			COALESCE(users.display_name, '') AS display_name
		FROM topup_rebates AS tr
		LEFT JOIN users ON users.id = tr.inviter_id
	`))
}

func withUserTopUpRecords(tx *gorm.DB, userId int) *gorm.DB {
	return withAllTopUpRecords(tx).
		Where(topUpRecordAlias+".user_id = ?", userId)
}

func withTopUpRecordKeyword(query *gorm.DB, keyword string) *gorm.DB {
	if keyword == "" {
		return query
	}
	like := "%" + keyword + "%"
	return query.Where(
		fmt.Sprintf("%s.trade_no LIKE ? OR COALESCE(%s.display_name, '') LIKE ?", topUpRecordAlias, topUpRecordAlias),
		like,
		like,
	)
}

func withTopUpRecordOrder(query *gorm.DB) *gorm.DB {
	return query.
		Order(topUpRecordAlias + ".create_time desc").
		Order(topUpRecordAlias + ".id desc")
}

// applyDefaults 应用默认值
// 对充值记录进行默认值设置，确保 BizType 有默认值
func (topUp *TopUp) applyDefaults() {
	if topUp == nil {
		return
	}
	topUp.BizType = normalizeTopUpBizType(topUp.BizType)
}

// GetBizType 获取充值记录的业务类型
// 自动判断并返回正确的业务类型：
// 1. 如果 BizType 已设置，直接返回
// 2. 如果未设置，检查是否为历史遗留的订阅记录（amount=0且tradeNo以sub开头）
// 3. 否则默认为 payment 类型
func (topUp *TopUp) GetBizType() string {
	if topUp == nil {
		return TopUpBizTypePayment
	}
	if topUp.BizType == "" {
		// Legacy subscription mirror rows were stored with amount=0 and a sub_* trade number.
		// 处理历史遗留数据：订阅记录通常额度为0且订单号以 sub 开头
		if topUp.Amount == 0 && strings.HasPrefix(strings.ToLower(topUp.TradeNo), "sub") {
			return TopUpBizTypeSubscription
		}
		return TopUpBizTypePayment
	}
	return normalizeTopUpBizType(topUp.BizType)
}

// CanManualComplete 检查是否可以手动补单
// 只有 payment 类型的充值订单可以手动补单
func (topUp *TopUp) CanManualComplete() bool {
	return topUp.GetBizType() == TopUpBizTypePayment
}

// GetQuotaToAdd 计算应该给用户增加的额度
// 根据不同的业务类型和支付方式计算实际到账额度
// 返回：实际到账额度、错误信息
func (topUp *TopUp) GetQuotaToAdd() (int, error) {
	if topUp == nil {
		return 0, errors.New("充值记录不存在")
	}

	switch topUp.GetBizType() {
	case TopUpBizTypeRedemption:
		// 兑换码充值：直接使用 Amount 字段的值
		if topUp.Amount <= 0 {
			return 0, errors.New("无效的兑换额度")
		}
		return int(topUp.Amount), nil
	case TopUpBizTypeSubscription:
		// 订阅账单：不支持直接补单，返回错误
		return 0, errors.New("订阅账单不支持直接补单")
	case TopUpBizTypePayment:
		// 在线支付充值：根据不同支付方式计算额度
		switch topUp.PaymentMethod {
		case "stripe":
			// Stripe 订单特殊处理：
			// Stripe 的 Money 字段存储的是"应发放的充值额度（基础额度 × 分组倍率）"
			// 所以这里直接按 Money 换算，不需要再乘以 QuotaPerUnit
			return int(decimal.NewFromFloat(topUp.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()), nil
		case "creem":
			// Creem 支付：直接使用 Amount 字段的值（已经是最终额度）
			if topUp.Amount <= 0 {
				return 0, errors.New("无效的充值额度")
			}
			return int(topUp.Amount), nil
		case "":
			// 支付方式缺失：无法安全计算，返回错误
			return 0, errors.New("订单支付方式缺失，无法安全计算充值额度")
		default:
			// 其他支付方式：按正常公式计算（Amount × QuotaPerUnit）
			return int(decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()), nil
		}
	default:
		// 不支持的业务类型
		return 0, fmt.Errorf("不支持的账单类型: %s", topUp.BizType)
	}
}

// asyncIncrUserQuotaCache 异步增加用户额度缓存
// 使用 goroutine 在后台异步更新 Redis 缓存中的用户额度
// 参数：用户ID、增加的额度
// 注意：只有额度大于0且Redis启用时才会执行
func asyncIncrUserQuotaCache(userId int, quota int) {
	if quota <= 0 || !common.RedisEnabled {
		return
	}
	gopool.Go(func() {
		if err := cacheIncrUserQuota(userId, int64(quota)); err != nil {
			common.SysLog("failed to increase user quota cache: " + err.Error())
		}
	})
}

// applyInviteTopupRebateTx 在事务中处理邀请充值返利
// 这是邀请返利功能的核心函数，在充值成功后自动给邀请人返利
//
// 返利逻辑：
// 1. 检查是否符合返利条件（payment类型、有额度、返利比例>0）
// 2. 查询用户的邀请人信息
// 3. 计算返利额度（充值额度 × 返利比例 ÷ 100）
// 4. 创建返利记录
// 5. 给邀请人增加相应额度
//
// 参数：
// - tx: 数据库事务
// - topUp: 充值记录
// - quotaToAdd: 实际到账额度
//
// 返回：
// - inviterId: 邀请人ID
// - rebateQuota: 返利额度
// - error: 错误信息
func applyInviteTopupRebateTx(tx *gorm.DB, topUp *TopUp, quotaToAdd int) (int, int, error) {
	if tx == nil || topUp == nil {
		return 0, 0, nil
	}

	// 检查返利条件：
	// - 只处理 payment 类型的充值
	// - 确保有实际到账额度
	// - 返利比例必须大于0
	if topUp.GetBizType() != TopUpBizTypePayment || quotaToAdd <= 0 || common.InviteTopupRebateRatio <= 0 {
		return 0, 0, nil
	}

	// 查询用户的邀请人信息
	var invitee struct {
		InviterId int `gorm:"column:inviter_id"`
	}
	if err := tx.Model(&User{}).Select("inviter_id").Where("id = ?", topUp.UserId).Take(&invitee).Error; err != nil {
		return 0, 0, err
	}

	// 如果用户没有邀请人或邀请人ID无效，不执行返利
	if invitee.InviterId <= 0 {
		return 0, 0, nil
	}

	// 计算返利额度：
	// 公式：返利额度 = 充值额度 × 返利比例 ÷ 100
	rebateQuota := int(decimal.NewFromInt(int64(quotaToAdd)).
		Mul(decimal.NewFromFloat(common.InviteTopupRebateRatio)).
		Div(decimal.NewFromInt(100)).
		IntPart())

	// 如果返利额度小于等于0，不执行返利
	if rebateQuota <= 0 {
		return 0, 0, nil
	}

	// 创建返利记录
	rebate := &TopUpRebate{
		InviterId:     invitee.InviterId,             // 邀请人ID
		InviteeId:     topUp.UserId,                  // 被邀请人（充值人）ID
		TopUpId:       topUp.Id,                      // 充值记录ID
		TradeNo:       topUp.TradeNo,                 // 充值订单号
		PaymentMethod: topUp.PaymentMethod,           // 支付方式
		SourceMoney:   topUp.Money,                   // 原始支付金额
		SourceQuota:   quotaToAdd,                    // 原始充值额度
		RebateRatio:   common.InviteTopupRebateRatio, // 返利比例
		RebateQuota:   rebateQuota,                   // 返利额度
		CreatedAt:     common.GetTimestamp(),         // 创建时间
	}

	// 在事务中创建返利记录
	if err := tx.Create(rebate).Error; err != nil {
		return 0, 0, err
	}

	// 给邀请人增加返利额度
	if err := tx.Model(&User{}).Where("id = ?", invitee.InviterId).Update("quota", gorm.Expr("quota + ?", rebateQuota)).Error; err != nil {
		return 0, 0, err
	}

	return invitee.InviterId, rebateQuota, nil
}

var ErrPaymentMethodMismatch = errors.New("payment method mismatch")

func (topUp *TopUp) Insert() error {
	topUp.applyDefaults()
	return DB.Create(topUp).Error
}

func (topUp *TopUp) Update() error {
	topUp.applyDefaults()
	return DB.Save(topUp).Error
}

func (topUp *TopUp) InsertTx(tx *gorm.DB) error {
	if tx == nil {
		return errors.New("tx is nil")
	}
	topUp.applyDefaults()
	return tx.Create(topUp).Error
}

func GetTopUpById(id int) *TopUp {
	var topUp *TopUp
	var err error
	err = DB.Where("id = ?", id).First(&topUp).Error
	if err != nil {
		return nil
	}
	topUp.BizType = topUp.GetBizType()
	return topUp
}

func GetTopUpByTradeNo(tradeNo string) *TopUp {
	var topUp *TopUp
	var err error
	err = DB.Where("trade_no = ?", tradeNo).First(&topUp).Error
	if err != nil {
		return nil
	}
	topUp.BizType = topUp.GetBizType()
	return topUp
}

// Recharge 处理 Stripe 支付回调的充值成功逻辑
// 这是 Stripe 支付成功后的主要处理函数，完成以下步骤：
// 1. 验证支付单号
// 2. 使用行级锁锁定订单，避免重复处理
// 3. 更新订单状态为成功
// 4. 计算并增加用户额度
// 5. 处理邀请返利（如果适用）
// 6. 异步更新缓存并记录日志
//
// 参数：
// - referenceId: Stripe 支付单号
// - customerId: Stripe 客户ID（用于更新用户记录）
//
// 返回：
// - error: 处理过程中的错误
func Recharge(referenceId string, customerId string) (err error) {
	if referenceId == "" {
		return errors.New("未提供支付单号")
	}

	var quotaToAdd int    // 实际到账额度
	var inviterId int     // 邀请人ID
	var rebateQuota int   // 返利额度
	var completedNow bool // 标记是否完成处理
	topUp := &TopUp{}     // 充值记录指针

	// 根据数据库类型设置正确的列引用语法
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	// 使用事务处理，确保数据一致性
	err = DB.Transaction(func(tx *gorm.DB) error {
		// 使用 FOR UPDATE 锁定订单，防止并发重复处理
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", referenceId).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}
		topUp.BizType = topUp.GetBizType()

		// 幂等处理：如果订单已经是成功状态，直接返回
		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}

		// 安全检查：确保是 Stripe 支付方式
		if topUp.PaymentMethod != "stripe" {
			return ErrPaymentMethodMismatch
		}

		// 状态检查：必须是待支付状态才能处理
		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		// 业务类型检查：只处理 payment 类型的充值
		if topUp.GetBizType() != TopUpBizTypePayment {
			return errors.New("账单类型错误")
		}

		// 更新订单状态为完成
		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		err = tx.Save(topUp).Error
		if err != nil {
			return err
		}

		// 计算实际到账额度
		quotaToAdd, err = topUp.GetQuotaToAdd()
		if err != nil {
			return err
		}

		// 更新用户额度，同时保存 Stripe 客户ID
		err = tx.Model(&User{}).Where("id = ?", topUp.UserId).Updates(map[string]interface{}{"stripe_customer": customerId, "quota": gorm.Expr("quota + ?", quotaToAdd)}).Error
		if err != nil {
			return err
		}

		// 处理邀请返利
		inviterId, rebateQuota, err = applyInviteTopupRebateTx(tx, topUp, quotaToAdd)
		if err != nil {
			return err
		}

		completedNow = true
		return nil
	})

	if err != nil {
		common.SysError("topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}
	if !completedNow {
		return nil
	}

	// 事务外处理：
	// 1. 异步更新用户额度缓存
	// 2. 记录充值和返利日志

	// 更新用户额度缓存
	asyncIncrUserQuotaCache(topUp.UserId, quotaToAdd)
	asyncIncrUserQuotaCache(inviterId, rebateQuota)

	// 记录邀请返利日志
	if inviterId > 0 && rebateQuota > 0 {
		RecordLog(inviterId, LogTypeTopup, fmt.Sprintf("invite rebate credited %s, source user ID %d, trade no %s", logger.LogQuota(rebateQuota), topUp.UserId, topUp.TradeNo))
	}

	// 记录用户充值成功日志
	RecordLog(topUp.UserId, LogTypeTopup, fmt.Sprintf("使用在线充值成功，充值金额: %v，支付金额：%.2f", logger.FormatQuota(quotaToAdd), topUp.Money))

	return nil
}

func RechargeEpay(referenceId string, expectedPaymentMethod string) (err error) {
	if referenceId == "" {
		return errors.New("未提供支付单号")
	}

	var quotaToAdd int
	var inviterId int
	var rebateQuota int
	var completedNow bool
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", referenceId).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}

		topUp.BizType = topUp.GetBizType()
		// 关键防线：易支付回调必须与本地订单记录的支付方式完全一致，
		// 否则一律拒绝，避免其他网关的成功回调串用到该订单。
		if expectedPaymentMethod == "" || topUp.PaymentMethod != expectedPaymentMethod {
			return ErrPaymentMethodMismatch
		}
		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}
		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}
		if topUp.GetBizType() != TopUpBizTypePayment {
			return errors.New("账单类型错误")
		}

		quotaToAdd, err = topUp.GetQuotaToAdd()
		if err != nil {
			return err
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}

		if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).Update("quota", gorm.Expr("quota + ?", quotaToAdd)).Error; err != nil {
			return err
		}
		inviterId, rebateQuota, err = applyInviteTopupRebateTx(tx, topUp, quotaToAdd)
		if err != nil {
			return err
		}
		completedNow = true
		return nil
	})

	if err != nil {
		common.SysError("epay topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}
	if !completedNow {
		return nil
	}

	asyncIncrUserQuotaCache(topUp.UserId, quotaToAdd)
	asyncIncrUserQuotaCache(inviterId, rebateQuota)
	if inviterId > 0 && rebateQuota > 0 {
		RecordLog(inviterId, LogTypeTopup, fmt.Sprintf("invite rebate credited %s, source user ID %d, trade no %s", logger.LogQuota(rebateQuota), topUp.UserId, topUp.TradeNo))
	}
	RecordLog(topUp.UserId, LogTypeTopup, fmt.Sprintf("使用在线充值成功，充值金额: %v，支付金额：%.2f", logger.FormatQuota(quotaToAdd), topUp.Money))
	return nil
}

func GetUserTopUps(userId int, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	countQuery := withUserTopUpRecords(tx, userId)
	if err = countQuery.Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	dataQuery := withUserTopUpRecords(tx, userId)
	if err = withTopUpRecordOrder(dataQuery).
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&topups).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	normalizeTopUps(topups)

	// Commit transaction
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return topups, total, nil
}

// GetAllTopUps 获取全平台的充值记录（管理员使用）
func GetAllTopUps(pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	countQuery := withAllTopUpRecords(tx)
	if err = countQuery.Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	dataQuery := withAllTopUpRecords(tx)
	if err = withTopUpRecordOrder(dataQuery).
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&topups).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	normalizeTopUps(topups)

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return topups, total, nil
}

// SearchUserTopUps 按订单号搜索某用户的充值记录
func SearchUserTopUps(userId int, keyword string, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	countQuery := withTopUpRecordKeyword(withUserTopUpRecords(tx, userId), keyword)

	if err = countQuery.Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	dataQuery := withTopUpRecordKeyword(withUserTopUpRecords(tx, userId), keyword)
	if err = withTopUpRecordOrder(dataQuery).
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&topups).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	normalizeTopUps(topups)

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return topups, total, nil
}

// SearchAllTopUps 按订单号或用户昵称搜索全平台充值记录（管理员使用）
func SearchAllTopUps(keyword string, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	countQuery := withTopUpRecordKeyword(withAllTopUpRecords(tx), keyword)

	if err = countQuery.Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	dataQuery := withTopUpRecordKeyword(withAllTopUpRecords(tx), keyword)
	if err = withTopUpRecordOrder(dataQuery).
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&topups).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	normalizeTopUps(topups)

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return topups, total, nil
}

// ManualCompleteTopUp 管理员手动完成订单并给用户充值
func ManualCompleteTopUp(tradeNo string) error {
	if tradeNo == "" {
		return errors.New("未提供订单号")
	}

	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	var userId int
	var quotaToAdd int
	var inviterId int
	var rebateQuota int
	var payMoney float64
	var completedNow bool

	var err error
	err = DB.Transaction(func(tx *gorm.DB) error {
		topUp := &TopUp{}
		// 行级锁，避免并发补单
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(topUp).Error; err != nil {
			return errors.New("充值订单不存在")
		}
		topUp.BizType = topUp.GetBizType()

		// 幂等处理：已成功直接返回
		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}

		if !topUp.CanManualComplete() {
			return errors.New("当前账单类型不支持补单")
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("订单状态不是待支付，无法补单")
		}

		quotaToAdd, err = topUp.GetQuotaToAdd()
		if err != nil {
			return err
		}
		if quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}

		// 标记完成
		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}

		// 增加用户额度（立即写库，保持一致性）
		if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).Update("quota", gorm.Expr("quota + ?", quotaToAdd)).Error; err != nil {
			return err
		}
		inviterId, rebateQuota, err = applyInviteTopupRebateTx(tx, topUp, quotaToAdd)
		if err != nil {
			return err
		}

		userId = topUp.UserId
		payMoney = topUp.Money
		completedNow = true
		return nil
	})

	if err != nil {
		return err
	}
	if !completedNow {
		return nil
	}

	asyncIncrUserQuotaCache(userId, quotaToAdd)
	asyncIncrUserQuotaCache(inviterId, rebateQuota)
	if inviterId > 0 && rebateQuota > 0 {
		RecordLog(inviterId, LogTypeTopup, fmt.Sprintf("invite rebate credited %s, source user ID %d, trade no %s", logger.LogQuota(rebateQuota), userId, tradeNo))
	}
	// 事务外记录日志，避免阻塞
	RecordLog(userId, LogTypeTopup, fmt.Sprintf("管理员补单成功，充值金额: %v，支付金额：%f", logger.FormatQuota(quotaToAdd), payMoney))
	return nil
}

// RechargeCreem 处理 Creem 支付回调的充值成功逻辑
// Creem 是一个特殊的支付方式，除了基本的充值功能外，还支持：
// 1. 自动同步用户邮箱（如果用户邮箱为空）
// 2. 额度直接充值（不需要通过 QuotaPerUnit 换算）
//
// 处理流程：
// 1. 验证支付单号
// 2. 使用行级锁锁定订单
// 3. 更新订单状态
// 4. 增加用户额度
// 5. 如果提供邮箱且用户邮箱为空，则更新用户邮箱
// 6. 处理邀请返利
// 7. 异步更新缓存并记录日志
//
// 参数：
// - referenceId: Creem 支付单号
// - customerEmail: 客户邮箱（可选，用于更新用户邮箱）
// - customerName: 客户名称（预留，当前未使用）
//
// 返回：
// - error: 处理过程中的错误
func RechargeCreem(referenceId string, customerEmail string, customerName string) (err error) {
	if referenceId == "" {
		return errors.New("未提供支付单号")
	}

	var quotaToAdd int    // 实际到账额度
	var inviterId int     // 邀请人ID
	var rebateQuota int   // 返利额度
	var completedNow bool // 标记是否完成处理
	topUp := &TopUp{}     // 充值记录指针

	// 根据数据库类型设置正确的列引用语法
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	// 使用事务处理，确保数据一致性
	err = DB.Transaction(func(tx *gorm.DB) error {
		// 使用 FOR UPDATE 锁定订单，防止并发重复处理
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", referenceId).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}
		topUp.BizType = topUp.GetBizType()

		// 幂等处理：如果订单已经是成功状态，直接返回
		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}

		// 安全检查：确保是 Creem 支付方式
		if topUp.PaymentMethod != "creem" {
			return ErrPaymentMethodMismatch
		}

		// 状态检查：必须是待支付状态才能处理
		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		// 业务类型检查：只处理 payment 类型的充值
		if topUp.GetBizType() != TopUpBizTypePayment {
			return errors.New("账单类型错误")
		}

		// 如果支付方式为空（兼容性处理），设置为 creem
		if topUp.PaymentMethod == "" {
			topUp.PaymentMethod = "creem"
		}

		// 更新订单状态为完成
		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		err = tx.Save(topUp).Error
		if err != nil {
			return err
		}

		// 计算实际到账额度
		quotaToAdd, err = topUp.GetQuotaToAdd()
		if err != nil {
			return err
		}

		// 构建更新字段，优先处理额度更新
		updateFields := map[string]interface{}{
			"quota": gorm.Expr("quota + ?", quotaToAdd),
		}

		// Creem 特殊功能：自动同步用户邮箱
		// 如果提供了客户邮箱，且当前用户邮箱为空，则更新用户邮箱
		if customerEmail != "" {
			// 先查询用户当前邮箱
			var user User
			err = tx.Where("id = ?", topUp.UserId).First(&user).Error
			if err != nil {
				return err
			}

			// 只有当用户邮箱为空时才更新，避免覆盖已有邮箱
			if user.Email == "" {
				updateFields["email"] = customerEmail
			}
		}

		// 更新用户信息（额度 + 可选的邮箱）
		err = tx.Model(&User{}).Where("id = ?", topUp.UserId).Updates(updateFields).Error
		if err != nil {
			return err
		}

		// 处理邀请返利
		inviterId, rebateQuota, err = applyInviteTopupRebateTx(tx, topUp, quotaToAdd)
		if err != nil {
			return err
		}

		completedNow = true
		return nil
	})

	if err != nil {
		common.SysError("creem topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}
	if !completedNow {
		return nil
	}

	// 事务外处理：
	// 1. 异步更新用户额度缓存
	// 2. 记录充值和返利日志

	// 更新用户额度缓存
	asyncIncrUserQuotaCache(topUp.UserId, quotaToAdd)
	asyncIncrUserQuotaCache(inviterId, rebateQuota)

	// 记录邀请返利日志
	if inviterId > 0 && rebateQuota > 0 {
		RecordLog(inviterId, LogTypeTopup, fmt.Sprintf("invite rebate credited %s, source user ID %d, trade no %s", logger.LogQuota(rebateQuota), topUp.UserId, topUp.TradeNo))
	}

	// 记录 Creem 充值成功日志（注意：Creem 直接显示额度，不需要换算）
	RecordLog(topUp.UserId, LogTypeTopup, fmt.Sprintf("使用Creem充值成功，充值额度: %v，支付金额：%.2f", quotaToAdd, topUp.Money))

	return nil
}

// SumAllTopUp 查询所有用户在时间范围内、指定业务类型且已完成的充值/获赠额度总和
func SumAllTopUp(startTimestamp, endTimestamp int64, bizType string) (int64, error) {
	var total int64
	tx := DB.Model(&TopUp{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("status = ? AND biz_type = ?", common.TopUpStatusSuccess, bizType)
	if startTimestamp != 0 {
		tx = tx.Where("create_time >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("create_time < ?", endTimestamp)
	}
	err := tx.Scan(&total).Error
	return total, err
}

// SumTopUpByUserId 查询指定用户在时间范围内、指定业务类型且已完成的充值/获赠额度总和
func SumTopUpByUserId(userId int, startTimestamp, endTimestamp int64, bizType string) (int64, error) {
	var total int64
	tx := DB.Model(&TopUp{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("user_id = ? AND status = ? AND biz_type = ?", userId, common.TopUpStatusSuccess, bizType)
	if startTimestamp != 0 {
		tx = tx.Where("create_time >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("create_time < ?", endTimestamp)
	}
	err := tx.Scan(&total).Error
	return total, err
}

func RechargeWaffo(tradeNo string) (err error) {
	if tradeNo == "" {
		return errors.New("未提供支付单号")
	}

	var quotaToAdd int
	var inviterId int
	var rebateQuota int
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}

		if topUp.PaymentMethod != "waffo" {
			return ErrPaymentMethodMismatch
		}

		if topUp.Status == common.TopUpStatusSuccess {
			return nil // 幂等：已成功直接返回
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		dAmount := decimal.NewFromInt(topUp.Amount)
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		quotaToAdd = int(dAmount.Mul(dQuotaPerUnit).IntPart())
		if quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}

		if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).Update("quota", gorm.Expr("quota + ?", quotaToAdd)).Error; err != nil {
			return err
		}
		inviterId, rebateQuota, err = applyInviteTopupRebateTx(tx, topUp, quotaToAdd)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		common.SysError("waffo topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}

	if quotaToAdd > 0 {
		asyncIncrUserQuotaCache(inviterId, rebateQuota)
		if inviterId > 0 && rebateQuota > 0 {
			RecordLog(inviterId, LogTypeTopup, fmt.Sprintf("invite rebate credited %s, source user ID %d, trade no %s", logger.LogQuota(rebateQuota), topUp.UserId, topUp.TradeNo))
		}
		RecordLog(topUp.UserId, LogTypeTopup, fmt.Sprintf("Waffo充值成功，充值额度: %v，支付金额: %.2f", logger.FormatQuota(quotaToAdd), topUp.Money))
	}

	return nil
}

// 管理员获取订单详情byid
func GetTopUpDetailsById(id int) (*TopUpDetails, error) {
	if id <= 0 {
		return nil, errors.New("invalid topup id")
	}

	topUp := &TopUp{}
	if err := DB.Model(&TopUp{}).
		Select("top_ups.*, COALESCE(users.display_name, '') AS display_name").
		Joins("LEFT JOIN users ON users.id = top_ups.user_id").
		Where("top_ups.id = ?", id).
		First(topUp).Error; err != nil {
		return nil, err
	}
	topUp.BizType = topUp.GetBizType()

	var rebate *TopUpRebate
	rebateRecord := &TopUpRebate{}
	if err := DB.Where("topup_id = ?", id).First(rebateRecord).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	} else {
		//计算返佣金额
		rebateRecord.Money = decimal.NewFromInt(int64(rebateRecord.RebateQuota)).Div(decimal.NewFromFloat(common.QuotaPerUnit)).InexactFloat64()
		rebateRecord.Status = common.TopUpStatusSuccess
		rebate = rebateRecord
	}

	return &TopUpDetails{
		TopUp:      topUp,
		Level1Rate: rebate,
	}, nil
}
