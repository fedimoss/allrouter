package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type TopUp struct {
	Id            int     `json:"id"`
	UserId        int     `json:"user_id" gorm:"index"`
	Amount        int64   `json:"amount"`
	Money         float64 `json:"money"`
	TradeNo       string  `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	PaymentMethod string  `json:"payment_method" gorm:"type:varchar(50)"`
	BizType       string  `json:"biz_type" gorm:"type:varchar(32);not null;default:payment;index"`
	SourceID      int     `json:"source_id" gorm:"default:0;index"`
	CreateTime    int64   `json:"create_time"`
	CompleteTime  int64   `json:"complete_time"`
	Status        string  `json:"status"`
}

const (
	TopUpBizTypePayment      = "payment"
	TopUpBizTypeSubscription = "subscription"
	TopUpBizTypeRedemption   = "redemption"
)

func normalizeTopUpBizType(bizType string) string {
	if bizType == "" {
		return TopUpBizTypePayment
	}
	return bizType
}

func normalizeTopUps(topups []*TopUp) {
	for _, topUp := range topups {
		if topUp == nil {
			continue
		}
		topUp.BizType = topUp.GetBizType()
	}
}

func (topUp *TopUp) applyDefaults() {
	if topUp == nil {
		return
	}
	topUp.BizType = normalizeTopUpBizType(topUp.BizType)
}

func (topUp *TopUp) GetBizType() string {
	if topUp == nil {
		return TopUpBizTypePayment
	}
	if topUp.BizType == "" {
		// Legacy subscription mirror rows were stored with amount=0 and a sub_* trade number.
		if topUp.Amount == 0 && strings.HasPrefix(strings.ToLower(topUp.TradeNo), "sub") {
			return TopUpBizTypeSubscription
		}
		return TopUpBizTypePayment
	}
	return normalizeTopUpBizType(topUp.BizType)
}

func (topUp *TopUp) CanManualComplete() bool {
	return topUp.GetBizType() == TopUpBizTypePayment
}

func (topUp *TopUp) GetQuotaToAdd() (int, error) {
	if topUp == nil {
		return 0, errors.New("充值记录不存在")
	}

	switch topUp.GetBizType() {
	case TopUpBizTypeRedemption:
		if topUp.Amount <= 0 {
			return 0, errors.New("无效的兑换额度")
		}
		return int(topUp.Amount), nil
	case TopUpBizTypeSubscription:
		return 0, errors.New("订阅账单不支持直接补单")
	case TopUpBizTypePayment:
		switch topUp.PaymentMethod {
		case "stripe":
			return int(decimal.NewFromFloat(topUp.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()), nil
		case "creem":
			if topUp.Amount <= 0 {
				return 0, errors.New("无效的充值额度")
			}
			return int(topUp.Amount), nil
		case "":
			return 0, errors.New("订单支付方式缺失，无法安全计算充值额度")
		default:
			return int(decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()), nil
		}
	default:
		return 0, fmt.Errorf("不支持的账单类型: %s", topUp.BizType)
	}
}

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

func Recharge(referenceId string, customerId string) (err error) {
	if referenceId == "" {
		return errors.New("未提供支付单号")
	}

	var quotaToAdd int
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

		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		if topUp.GetBizType() != TopUpBizTypePayment {
			return errors.New("账单类型错误")
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		err = tx.Save(topUp).Error
		if err != nil {
			return err
		}

		quotaToAdd, err = topUp.GetQuotaToAdd()
		if err != nil {
			return err
		}

		err = tx.Model(&User{}).Where("id = ?", topUp.UserId).Updates(map[string]interface{}{"stripe_customer": customerId, "quota": gorm.Expr("quota + ?", quotaToAdd)}).Error
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

	asyncIncrUserQuotaCache(topUp.UserId, quotaToAdd)
	RecordLog(topUp.UserId, LogTypeTopup, fmt.Sprintf("使用在线充值成功，充值金额: %v，支付金额：%.2f", logger.FormatQuota(quotaToAdd), topUp.Money))

	return nil
}

func RechargeEpay(referenceId string) (err error) {
	if referenceId == "" {
		return errors.New("未提供支付单号")
	}

	var quotaToAdd int
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
	RecordLog(topUp.UserId, LogTypeTopup, fmt.Sprintf("使用在线充值成功，充值金额: %v，支付金额：%.2f", logger.FormatQuota(quotaToAdd), topUp.Money))
	return nil
}

func GetUserTopUps(userId int, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	// Start transaction
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get total count within transaction
	err = tx.Model(&TopUp{}).Where("user_id = ?", userId).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated topups within same transaction
	err = tx.Where("user_id = ?", userId).Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error
	if err != nil {
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

	if err = tx.Model(&TopUp{}).Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
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

	query := tx.Model(&TopUp{}).Where("user_id = ?", userId)
	if keyword != "" {
		like := "%%" + keyword + "%%"
		query = query.Where("trade_no LIKE ?", like)
	}

	if err = query.Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	normalizeTopUps(topups)

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return topups, total, nil
}

// SearchAllTopUps 按订单号搜索全平台充值记录（管理员使用）
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

	query := tx.Model(&TopUp{})
	if keyword != "" {
		like := "%%" + keyword + "%%"
		query = query.Where("trade_no LIKE ?", like)
	}

	if err = query.Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
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
	// 事务外记录日志，避免阻塞
	RecordLog(userId, LogTypeTopup, fmt.Sprintf("管理员补单成功，充值金额: %v，支付金额：%f", logger.FormatQuota(quotaToAdd), payMoney))
	return nil
}
func RechargeCreem(referenceId string, customerEmail string, customerName string) (err error) {
	if referenceId == "" {
		return errors.New("未提供支付单号")
	}

	var quotaToAdd int
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

		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		if topUp.GetBizType() != TopUpBizTypePayment {
			return errors.New("账单类型错误")
		}
		if topUp.PaymentMethod == "" {
			topUp.PaymentMethod = "creem"
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		err = tx.Save(topUp).Error
		if err != nil {
			return err
		}

		quotaToAdd, err = topUp.GetQuotaToAdd()
		if err != nil {
			return err
		}

		// 构建更新字段，优先使用邮箱，如果邮箱为空则使用用户名
		updateFields := map[string]interface{}{
			"quota": gorm.Expr("quota + ?", quotaToAdd),
		}

		// 如果有客户邮箱，尝试更新用户邮箱（仅当用户邮箱为空时）
		if customerEmail != "" {
			// 先检查用户当前邮箱是否为空
			var user User
			err = tx.Where("id = ?", topUp.UserId).First(&user).Error
			if err != nil {
				return err
			}

			// 如果用户邮箱为空，则更新为支付时使用的邮箱
			if user.Email == "" {
				updateFields["email"] = customerEmail
			}
		}

		err = tx.Model(&User{}).Where("id = ?", topUp.UserId).Updates(updateFields).Error
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

	asyncIncrUserQuotaCache(topUp.UserId, quotaToAdd)
	RecordLog(topUp.UserId, LogTypeTopup, fmt.Sprintf("使用Creem充值成功，充值额度: %v，支付金额：%.2f", quotaToAdd, topUp.Money))

	return nil
}
