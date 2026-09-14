package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Midjourney struct {
	Id          int    `json:"id"`
	Code        int    `json:"code"`
	UserId      int    `json:"user_id" gorm:"index"`
	Action      string `json:"action" gorm:"type:varchar(40);index"`
	MjId        string `json:"mj_id" gorm:"index"`
	Prompt      string `json:"prompt"`
	PromptEn    string `json:"prompt_en"`
	Description string `json:"description"`
	State       string `json:"state"`
	SubmitTime  int64  `json:"submit_time" gorm:"index"`
	StartTime   int64  `json:"start_time" gorm:"index"`
	FinishTime  int64  `json:"finish_time" gorm:"index"`
	ImageUrl    string `json:"image_url"`
	VideoUrl    string `json:"video_url"`
	VideoUrls   string `json:"video_urls"`
	Status      string `json:"status" gorm:"type:varchar(20);index"`
	Progress    string `json:"progress" gorm:"type:varchar(30);index"`
	FailReason  string `json:"fail_reason"`
	ChannelId   int    `json:"channel_id"`
	Quota       int    `json:"quota"`
	// Billing fields are internal snapshots used by the unified billing
	// session.  They are deliberately hidden from the public Midjourney API
	// response, but persisted so an asynchronous task can be settled or
	// refunded after the submit request has returned.
	BillingSource           string `json:"-" gorm:"column:billing_source;type:varchar(32);not null;default:''"`
	BillingPreConsumed      int64  `json:"-" gorm:"column:billing_pre_consumed;type:bigint;not null;default:0"`
	SubscriptionId          int    `json:"-" gorm:"column:subscription_id;type:int;not null;default:0;index"`
	SubscriptionRequestId   string `json:"-" gorm:"column:subscription_request_id;type:varchar(128);not null;default:'';index"`
	SubscriptionPreConsumed int64  `json:"-" gorm:"column:subscription_pre_consumed;type:bigint;not null;default:0"`
	TokenId                 int    `json:"-" gorm:"column:token_id;type:int;not null;default:0;index"`
	BillingRefunded         int    `json:"-" gorm:"column:billing_refunded;type:int;not null;default:0"`
	// Refund progress is persisted independently for the funding and token
	// sides.  A Midjourney task is settled asynchronously and can be observed
	// by more than one polling worker; keeping these checkpoints on the task
	// prevents a retry (or a concurrent worker) from crediting either side
	// twice.  Values are 0=pending, 1=completed, 2=in-flight claim.
	BillingRefundFundingDone int `json:"-" gorm:"column:billing_refund_funding_done;type:int;not null;default:0"`
	BillingRefundTokenDone   int `json:"-" gorm:"column:billing_refund_token_done;type:int;not null;default:0"`
	// Async wallet funding snapshot. Integer flags keep the schema portable
	// across SQLite, MySQL and PostgreSQL.
	WalletRewardUsed             int    `json:"-" gorm:"column:wallet_reward_used;not null;default:0"`
	WalletPaidUsed               int    `json:"-" gorm:"column:wallet_paid_used;not null;default:0"`
	WalletQuotaBreakdownRecorded int    `json:"-" gorm:"column:wallet_quota_breakdown_recorded;not null;default:0"`
	ConsumeRebateSettled         int    `json:"-" gorm:"column:consume_rebate_settled;not null;default:0"`
	Buttons                      string `json:"buttons"`
	Properties                   string `json:"properties"`
}

// RefundMidjourneyBilling atomically refunds a persisted Midjourney task.
//
// Midjourney jobs are asynchronous: the submitter can return before the
// upstream reports failure, and more than one polling worker may observe the
// same terminal transition.  User/token balances and the task's refunded
// marker therefore have to be changed in one database transaction.  The
// previous implementation performed relative balance updates first and saved
// the marker afterwards, which could double-credit a wallet (or token) after
// a crash between those operations.  This function is deliberately in the
// model package so subscription usage, wallet balances, token quota, and the
// Midjourney row all share the same transaction and row locks.
//
// It returns nil when the task was already refunded, making retries and
// concurrent pollers idempotent.  Unsaved tasks (Id <= 0) are rejected so
// callers can use their legacy/in-memory fallback when no durable row exists.
func RefundMidjourneyBilling(task *Midjourney) error {
	if task == nil {
		return nil
	}
	if task.Id <= 0 {
		return errors.New("midjourney task is not persisted")
	}
	if DB == nil {
		return errors.New("database is not initialized")
	}

	// Keep the values needed for cache repair outside the transaction.  Cache
	// writes are best-effort and must not run before the SQL commit.
	var walletUserID, tokenID int
	var tokenKey string
	var cacheDelta int64
	var cacheTokenDelta int64

	err := DB.Transaction(func(tx *gorm.DB) error {
		var current Midjourney
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", task.Id).First(&current).Error; err != nil {
			return err
		}
		if current.BillingRefunded == 1 {
			*task = current
			return nil
		}

		refundAmount := current.BillingPreConsumed
		if refundAmount <= 0 {
			refundAmount = current.SubscriptionPreConsumed
		}
		if refundAmount <= 0 && current.Quota > 0 {
			refundAmount = int64(current.Quota)
		}
		if refundAmount < 0 {
			return errors.New("midjourney refund amount must be >= 0")
		}
		maxInt := int64(^uint(0) >> 1)
		if refundAmount > maxInt {
			return fmt.Errorf("midjourney refund amount overflows int: %d", refundAmount)
		}

		// Keep the model package independent from service (which imports model).
		// The persisted billing source uses the stable wire values "wallet" and
		// "subscription".  Empty means a legacy wallet-funded task; reject any
		// other value rather than accidentally crediting an unknown source.
		source := strings.TrimSpace(current.BillingSource)
		if source != "" && source != "wallet" && source != "subscription" {
			return fmt.Errorf("invalid Midjourney billing source: %s", source)
		}
		walletSource := source != "subscription"
		if walletSource && refundAmount > 0 {
			reward := 0
			total := int(refundAmount)
			if current.WalletQuotaBreakdownRecorded == 1 {
				consumed := current.WalletRewardUsed + current.WalletPaidUsed
				if consumed != total {
					return fmt.Errorf("Midjourney wallet snapshot mismatch: quota=%d consumed=%d", total, consumed)
				}
				reward = current.WalletRewardUsed
				if reward < 0 || reward > total {
					return fmt.Errorf("invalid Midjourney wallet reward snapshot: reward=%d total=%d", reward, total)
				}
			}
			result := tx.Model(&User{}).Where("id = ?", current.UserId).Updates(map[string]interface{}{
				"quota":        gorm.Expr("quota + ?", total),
				"reward_quota": gorm.Expr("reward_quota + ?", reward),
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
			walletUserID = current.UserId
			cacheDelta = refundAmount
		}

		if strings.TrimSpace(current.BillingSource) == "subscription" {
			requestID := strings.TrimSpace(current.SubscriptionRequestId)
			if requestID != "" {
				if err := refundSubscriptionPreConsumeTx(tx, requestID, getDBTimestampTx(tx)); err != nil {
					return err
				}
			} else if current.SubscriptionId > 0 && current.SubscriptionPreConsumed > 0 {
				return errors.New("Midjourney subscription refund request id is missing")
			}
		}

		// Only refund token quota that was explicitly reserved.  Subscription
		// funded requests deliberately bypass the token's numeric allowance
		// (BillingPreConsumed/SubscriptionPreConsumed are subscription units),
		// so they must never credit a token on failure.  Older wallet rows with
		// an empty source use Quota as their historical fallback.
		var tokenRefundAmount int64
		if source != "subscription" {
			tokenRefundAmount = current.BillingPreConsumed
			if tokenRefundAmount <= 0 && source == "" {
				tokenRefundAmount = int64(current.Quota)
			}
		}
		if tokenRefundAmount > 0 && current.TokenId > 0 {
			if tokenRefundAmount > maxInt {
				return fmt.Errorf("midjourney token refund amount overflows int: %d", tokenRefundAmount)
			}
			result := tx.Model(&Token{}).Where("id = ?", current.TokenId).Updates(map[string]interface{}{
				"remain_quota":  gorm.Expr("remain_quota + ?", int(tokenRefundAmount)),
				"used_quota":    gorm.Expr("used_quota - ?", int(tokenRefundAmount)),
				"accessed_time": common.GetTimestamp(),
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
			tokenID = current.TokenId
			var token Token
			keyColumn := commonKeyCol
			if keyColumn == "" {
				keyColumn = "`key`"
				if common.UsingPostgreSQL {
					keyColumn = `"key"`
				}
			}
			if err := tx.Select(keyColumn).Where("id = ?", current.TokenId).First(&token).Error; err != nil {
				return err
			}
			tokenKey = token.Key
			cacheTokenDelta = tokenRefundAmount
		}

		updates := map[string]interface{}{
			"billing_refunded":                1,
			"billing_refund_funding_done":     1,
			"billing_refund_token_done":       1,
			"wallet_reward_used":              0,
			"wallet_paid_used":                0,
			"wallet_quota_breakdown_recorded": 0,
			"consume_rebate_settled":          1,
		}
		result := tx.Model(&Midjourney{}).Where("id = ? AND billing_refunded = 0", current.Id).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			// A concurrent transaction can only win after our row lock is
			// released; treat a zero-row update as an idempotent completion.
			var check Midjourney
			if err := tx.Select("billing_refunded").Where("id = ?", current.Id).First(&check).Error; err != nil {
				return err
			}
			if check.BillingRefunded != 1 {
				return errors.New("midjourney refund marker was not persisted")
			}
		}
		current.BillingRefunded = 1
		current.BillingRefundFundingDone = 1
		current.BillingRefundTokenDone = 1
		current.WalletRewardUsed = 0
		current.WalletPaidUsed = 0
		current.WalletQuotaBreakdownRecorded = 0
		current.ConsumeRebateSettled = 1
		*task = current
		return nil
	})
	if err != nil {
		return err
	}

	// Keep Redis in step with the committed SQL balances.  These updates are
	// intentionally asynchronous, matching IncreaseUserQuota and
	// IncreaseTokenQuota; failure only causes a cache refresh on the next read.
	if common.RedisEnabled && walletUserID > 0 && cacheDelta > 0 {
		userID, delta := walletUserID, cacheDelta
		gopool.Go(func() {
			if err := cacheIncrUserQuota(userID, delta); err != nil {
				common.SysLog("failed to update user quota cache after Midjourney refund: " + err.Error())
			}
		})
	}
	if common.RedisEnabled && tokenID > 0 && tokenKey != "" && cacheTokenDelta > 0 {
		key, delta := tokenKey, cacheTokenDelta
		gopool.Go(func() {
			if err := cacheIncrTokenQuota(key, delta); err != nil {
				common.SysLog("failed to update token quota cache after Midjourney refund: " + err.Error())
			}
		})
	}
	return nil
}

// TaskQueryParams 用于包含所有搜索条件的结构体，可以根据需求添加更多字段
type TaskQueryParams struct {
	ChannelID      string
	MjID           string
	StartTimestamp string
	EndTimestamp   string
}

func GetAllUserTask(userId int, startIdx int, num int, queryParams TaskQueryParams) []*Midjourney {
	var tasks []*Midjourney
	var err error

	// 初始化查询构建器
	query := DB.Where("user_id = ?", userId)

	if queryParams.MjID != "" {
		query = query.Where("mj_id = ?", queryParams.MjID)
	}
	if queryParams.StartTimestamp != "" {
		// 假设您已将前端传来的时间戳转换为数据库所需的时间格式，并处理了时间戳的验证和解析
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != "" {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}

	// 获取数据
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&tasks).Error
	if err != nil {
		return nil
	}

	return tasks
}

func GetAllTasks(startIdx int, num int, queryParams TaskQueryParams) []*Midjourney {
	var tasks []*Midjourney
	var err error

	// 初始化查询构建器
	query := DB

	// 添加过滤条件
	if queryParams.ChannelID != "" {
		query = query.Where("channel_id = ?", queryParams.ChannelID)
	}
	if queryParams.MjID != "" {
		query = query.Where("mj_id = ?", queryParams.MjID)
	}
	if queryParams.StartTimestamp != "" {
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != "" {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}

	// 获取数据
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&tasks).Error
	if err != nil {
		return nil
	}

	return tasks
}

func GetAllUnFinishTasks() []*Midjourney {
	var tasks []*Midjourney
	var err error
	// get all tasks progress is not 100%
	err = DB.Where("progress != ?", "100%").Find(&tasks).Error
	if err != nil {
		return nil
	}
	return tasks
}

func GetByOnlyMJId(mjId string) *Midjourney {
	var mj *Midjourney
	var err error
	err = DB.Where("mj_id = ?", mjId).First(&mj).Error
	if err != nil {
		return nil
	}
	return mj
}

func GetByMJId(userId int, mjId string) *Midjourney {
	var mj *Midjourney
	var err error
	err = DB.Where("user_id = ? and mj_id = ?", userId, mjId).First(&mj).Error
	if err != nil {
		return nil
	}
	return mj
}

func GetByMJIds(userId int, mjIds []string) []*Midjourney {
	var mj []*Midjourney
	var err error
	err = DB.Where("user_id = ? and mj_id in (?)", userId, mjIds).Find(&mj).Error
	if err != nil {
		return nil
	}
	return mj
}

func GetMjByuId(id int) *Midjourney {
	var mj *Midjourney
	var err error
	err = DB.Where("id = ?", id).First(&mj).Error
	if err != nil {
		return nil
	}
	return mj
}

func UpdateProgress(id int, progress string) error {
	return DB.Model(&Midjourney{}).Where("id = ?", id).Update("progress", progress).Error
}

func (midjourney *Midjourney) Insert() error {
	var err error
	err = DB.Create(midjourney).Error
	return err
}

func (midjourney *Midjourney) Update() error {
	var err error
	err = DB.Save(midjourney).Error
	return err
}

// UpdateWithStatus performs a conditional UPDATE guarded by fromStatus (CAS).
// Returns (true, nil) if this caller won the update, (false, nil) if
// another process already moved the task out of fromStatus.
// UpdateWithStatus performs a conditional UPDATE guarded by fromStatus (CAS).
// Uses Model().Select("*").Updates() to avoid GORM Save()'s INSERT fallback.
func (midjourney *Midjourney) UpdateWithStatus(fromStatus string) (bool, error) {
	result := DB.Model(midjourney).Where("status = ?", fromStatus).Select("*").Updates(midjourney)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func MjBulkUpdate(mjIds []string, params map[string]any) error {
	return DB.Model(&Midjourney{}).
		Where("mj_id in (?)", mjIds).
		Updates(params).Error
}

func MjBulkUpdateByTaskIds(taskIDs []int, params map[string]any) error {
	return DB.Model(&Midjourney{}).
		Where("id in (?)", taskIDs).
		Updates(params).Error
}

// CountAllTasks returns total midjourney tasks for admin query
func CountAllTasks(queryParams TaskQueryParams) int64 {
	var total int64
	query := DB.Model(&Midjourney{})
	if queryParams.ChannelID != "" {
		query = query.Where("channel_id = ?", queryParams.ChannelID)
	}
	if queryParams.MjID != "" {
		query = query.Where("mj_id = ?", queryParams.MjID)
	}
	if queryParams.StartTimestamp != "" {
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != "" {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}
	_ = query.Count(&total).Error
	return total
}

// CountAllUserTask returns total midjourney tasks for user
func CountAllUserTask(userId int, queryParams TaskQueryParams) int64 {
	var total int64
	query := DB.Model(&Midjourney{}).Where("user_id = ?", userId)
	if queryParams.MjID != "" {
		query = query.Where("mj_id = ?", queryParams.MjID)
	}
	if queryParams.StartTimestamp != "" {
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != "" {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}
	_ = query.Count(&total).Error
	return total
}
