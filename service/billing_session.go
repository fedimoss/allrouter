package service

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// BillingSession — 统一计费会话
// ---------------------------------------------------------------------------

// BillingSession 封装单次请求的预扣费/结算/退款生命周期。
// 实现 relaycommon.BillingSettler 接口。
type BillingSession struct {
	relayInfo *relaycommon.RelayInfo
	funding   FundingSource
	// tokenQuotaAdjuster is normally nil and token changes are persisted via
	// model.DecreaseTokenQuota/IncreaseTokenQuota.  Keeping the operation
	// behind a small hook also lets tests exercise the two-phase settlement
	// state machine without requiring a live token row/database.
	tokenQuotaAdjuster func(*relaycommon.RelayInfo, int) error
	preConsumedQuota   int  // 实际预扣额度（信任用户可能为 0）
	tokenConsumed      int  // 令牌额度当前实际扣减量
	fundingSettled     bool // funding.Settle 至少成功调整过一次（兼容诊断字段）

	// fundingQuota/tokenQuota are the independently committed amounts on the
	// two sides of the reservation.  They can temporarily differ when funding
	// adjustment succeeds but token adjustment fails; retaining both values
	// makes a retry or refund correct instead of applying the funding delta a
	// second time.
	fundingQuota          int
	tokenQuota            int
	quotaStateInitialized bool
	settled               bool // Settle 全部完成（资金 + 令牌）
	refunded              bool // Refund 已调用
	// refundStarted is a terminal lifecycle fence set before the asynchronous
	// refund releases the mutex.  Without it, Settle/SettleProgress could race
	// the refund goroutine and charge a request after its error path had already
	// started returning the reservation.
	refundStarted     bool
	refundInFlight    bool
	refundFundingDone bool
	refundTokenDone   bool
	// rollback... tracks compensation after a successful settlement.  A
	// normal Refund intentionally skips settled sessions; these markers let a
	// caller undo a charge when a subsequent local persistence step fails,
	// while ensuring retries never repeat a side that already completed.
	settlementRolledBack bool
	rollbackInFlight     bool
	rollbackFundingDone  bool
	rollbackTokenDone    bool
	rebateApplied        bool
	// SettleProgress receives an incremental delta.  If funding commits but
	// token persistence fails, remember the logical operation so retrying the
	// same delta targets the same absolute amount instead of adding it twice.
	progressPending       bool
	pendingProgressDelta  int
	pendingProgressTarget int
	mu                    sync.Mutex
}

// tokenQuotaRequired reports whether this billing source should reserve and
// settle the per-API-token numeric quota.  Subscription quota is an
// independent allowance; an enabled token with a zero balance must therefore
// not be rejected (or driven negative) while a subscription is active.  The
// token is still authenticated normally by middleware, including status,
// expiry, provider, and model-limit checks.
func (s *BillingSession) tokenQuotaRequired() bool {
	return s != nil && s.relayInfo != nil && !s.relayInfo.IsPlayground &&
		(s.funding == nil || s.funding.Source() != BillingSourceSubscription)
}

// initializeQuotaStateLocked initializes the independently committed sides of
// a reservation.  It must be called with s.mu held.
func (s *BillingSession) initializeQuotaStateLocked() {
	if s.quotaStateInitialized {
		return
	}
	s.fundingQuota = s.preConsumedQuota
	s.tokenQuota = s.tokenConsumed
	s.quotaStateInitialized = true
}

// applyTargetLocked moves both funding and token reservations to target.  It
// must be called with s.mu held.  Funding is adjusted first; if token
// adjustment then fails, the two counters remain independently recorded so a
// retry adjusts only the side that is still behind.
func (s *BillingSession) applyTargetLocked(target int) error {
	if target < 0 {
		return fmt.Errorf("actual quota must be >= 0")
	}
	if s.funding == nil || s.relayInfo == nil {
		return fmt.Errorf("billing session is not initialized")
	}
	s.initializeQuotaStateLocked()

	fundingDelta := target - s.fundingQuota
	if fundingDelta != 0 {
		if err := s.funding.Settle(fundingDelta); err != nil {
			return err
		}
		s.fundingQuota = target
		s.fundingSettled = true
	}

	tokenDelta := target - s.tokenQuota
	if tokenDelta != 0 && s.tokenQuotaRequired() {
		err := s.adjustTokenQuotaLocked(tokenDelta)
		if err != nil {
			common.SysLog(fmt.Sprintf("error adjusting token quota (userId=%d, tokenId=%d, delta=%d): %s",
				s.relayInfo.UserId, s.relayInfo.TokenId, tokenDelta, err.Error()))
			return err
		}
	}
	if s.tokenQuotaRequired() {
		s.tokenQuota = target
		s.tokenConsumed = target
	} else {
		// Subscription-funded requests do not reserve the token's numeric
		// allowance.  Keep these bookkeeping fields at zero so a later Refund
		// cannot accidentally credit a token that was never debited.
		s.tokenQuota = 0
		s.tokenConsumed = 0
	}

	// Only expose a subscription delta after both sides have committed.  This
	// keeps log metadata consistent when a token update has to be retried.
	if s.funding.Source() == BillingSourceSubscription {
		committedDelta := int64(target - s.preConsumedQuota)
		previousDelta := s.relayInfo.SubscriptionPostDelta
		if previousDelta != committedDelta {
			s.relayInfo.SubscriptionPostDelta = committedDelta
		}
	}
	return nil
}

// adjustTokenQuotaLocked persists a relative token adjustment.  The hook is
// intentionally optional; production sessions use the model functions while
// tests can inject deterministic failures between funding and token commits.
// The caller must hold s.mu.
func (s *BillingSession) adjustTokenQuotaLocked(delta int) error {
	if delta == 0 {
		return nil
	}
	if s.tokenQuotaAdjuster != nil {
		return s.tokenQuotaAdjuster(s.relayInfo, delta)
	}
	if delta > 0 {
		return model.DecreaseTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, delta)
	}
	return model.IncreaseTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, -delta)
}

// rollbackPreConsumedTokenQuota compensates the token reservation made before
// the funding source was pre-consumed.  Funding and token stores are separate
// persistence systems, so this compensation is deliberately retried a few
// times on transient database/Redis failures.  The caller must only clear its
// token reservation state after this function succeeds; otherwise a later
// retry would have no way to know that token quota is still outstanding.
func (s *BillingSession) rollbackPreConsumedTokenQuota(amount int) error {
	if amount <= 0 || s.relayInfo == nil || s.relayInfo.IsPlayground {
		return nil
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		var err error
		if s.tokenQuotaAdjuster != nil {
			err = s.tokenQuotaAdjuster(s.relayInfo, -amount)
		} else {
			err = model.IncreaseTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, amount)
		}
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
		}
	}
	return lastErr
}

// Settle 根据实际消耗额度进行结算。
// 资金来源和令牌额度分两步提交：若令牌调整失败，会保留未结算状态，
// 使调用方可以重试或退款。
func (s *BillingSession) Settle(actualQuota int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refundStarted {
		return fmt.Errorf("billing session refund already started")
	}
	if s.settled || s.settlementRolledBack || s.refunded {
		return nil
	}
	// A final absolute settlement supersedes any failed progress operation.
	// The independently tracked funding/token targets still make a retry of
	// this final call idempotent, so stale progress metadata must not affect a
	// subsequent SettleProgress invocation.
	s.progressPending = false
	if err := s.applyTargetLocked(actualQuota); err != nil {
		return err
	}
	s.settled = true
	return nil
}

// RollbackSettlement compensates a settlement that has already committed.
//
// BillingSettler.Refund deliberately ignores fully settled sessions because a
// successful request must not be refunded by a late error path.  There is one
// important exception: asynchronous submission settles the billing session
// before inserting its local task row.  If that insert fails, no durable task
// checkpoint exists from which a worker can recover the charge.  This explicit
// operation is used only by that path (and equivalent persistence failures),
// and refunds the independently committed funding/token amounts synchronously.
// Each side is checkpointed in memory so a retry after a transient failure
// cannot refund the other side twice.  Subscription refunds are request-ID
// idempotent at the model layer; wallet/token operations retain the same
// process-local side guard used by Refund.
func (s *BillingSession) RollbackSettlement(c *gin.Context) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.settlementRolledBack || s.refunded {
		s.mu.Unlock()
		return nil
	}
	if !s.settled {
		// A partially failed settlement is still handled by the ordinary Refund
		// state machine.  Do not run a second synchronous operation here.
		s.mu.Unlock()
		if s.NeedsRefund() {
			s.Refund(c)
		}
		return nil
	}
	if s.rollbackInFlight {
		s.mu.Unlock()
		return fmt.Errorf("billing settlement rollback already in progress")
	}
	s.initializeQuotaStateLocked()
	s.rollbackInFlight = true
	funding := s.funding
	fundingDone := s.rollbackFundingDone
	tokenDone := s.rollbackTokenDone
	tokenAmount := s.tokenQuota
	tokenAdjuster := s.tokenQuotaAdjuster
	relayInfo := s.relayInfo
	// Capture immutable token fields before releasing the lock.
	tokenID, tokenKey, playground, userID := 0, "", false, 0
	if relayInfo != nil {
		tokenID, tokenKey, playground, userID = relayInfo.TokenId, relayInfo.TokenKey, relayInfo.IsPlayground, relayInfo.UserId
	}
	s.mu.Unlock()

	var firstErr error
	if !fundingDone && funding != nil {
		if err := funding.Refund(); err != nil {
			firstErr = fmt.Errorf("rollback billing funding: %w", err)
		} else {
			s.mu.Lock()
			s.rollbackFundingDone = true
			s.mu.Unlock()
		}
	} else if funding == nil {
		s.mu.Lock()
		s.rollbackFundingDone = true
		s.mu.Unlock()
	}

	if !tokenDone && tokenAmount > 0 && !playground {
		var err error
		if tokenAdjuster != nil {
			err = tokenAdjuster(relayInfo, -tokenAmount)
		} else {
			err = model.IncreaseTokenQuota(tokenID, tokenKey, tokenAmount)
		}
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("rollback token quota (userId=%d, tokenId=%d): %w", userID, tokenID, err)
			}
		} else {
			s.mu.Lock()
			s.rollbackTokenDone = true
			s.mu.Unlock()
		}
	} else {
		s.mu.Lock()
		s.rollbackTokenDone = true
		s.mu.Unlock()
	}

	s.mu.Lock()
	s.rollbackInFlight = false
	complete := s.rollbackFundingDone && s.rollbackTokenDone
	if complete {
		// Keep settled=true as a terminal guard against an accidental second
		// Settle call; settlementRolledBack distinguishes this state from a
		// successfully charged request for diagnostics and retries.
		s.settlementRolledBack = true
		s.refunded = true
	}
	s.mu.Unlock()
	return firstErr
}

// SettleProgress applies an incremental amount while a long-lived realtime
// request is still running.  The final Settle call can safely be made with
// the cumulative actual amount; it will only adjust the remaining difference.
// This method is intentionally outside BillingSettler so ordinary request
// paths retain their one-shot lifecycle.
func (s *BillingSession) SettleProgress(delta int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refundStarted {
		return fmt.Errorf("billing session refund already started")
	}
	if s.settled {
		return nil
	}
	s.initializeQuotaStateLocked()
	var target int
	if s.progressPending {
		if delta == s.pendingProgressDelta {
			// Retry of the exact operation that partially committed.  Reuse its
			// absolute target so the already-advanced funding side is not charged
			// a second time.
			target = s.pendingProgressTarget
		} else {
			// A caller may continue streaming after a failed chunk rather than
			// retrying it explicitly.  Preserve the failed target and apply the
			// new delta on top; applyTargetLocked will only adjust each side's
			// remaining difference.
			target = s.pendingProgressTarget + delta
		}
	} else {
		target = s.fundingQuota + delta
	}
	if target < 0 {
		target = 0
	}
	// Record the logical operation before attempting either side.  If the
	// token side fails after funding succeeds, a retry can recover using the
	// same target.  Clear it only after both sides commit successfully.
	s.progressPending = true
	s.pendingProgressDelta = delta
	s.pendingProgressTarget = target
	if err := s.applyTargetLocked(target); err != nil {
		return err
	}
	s.progressPending = false
	return nil
}

// Refund 退还所有预扣费，幂等安全，异步执行。
func (s *BillingSession) Refund(c *gin.Context) {
	s.mu.Lock()
	if s.settled || s.refunded || s.refundInFlight || !s.needsRefundLocked() {
		s.mu.Unlock()
		return
	}
	s.initializeQuotaStateLocked()
	s.refundStarted = true
	s.refundInFlight = true
	if s.funding == nil {
		s.refundFundingDone = true
	}
	if s.relayInfo == nil || s.relayInfo.IsPlayground || s.tokenConsumed <= 0 {
		s.refundTokenDone = true
	}
	// 复制需要的值到闭包中，同时避免异步退款期间读取可变字段。
	relayInfo := s.relayInfo
	funding := s.funding
	tokenConsumed := s.tokenConsumed
	fundingDone := s.refundFundingDone
	tokenDone := s.refundTokenDone
	tokenId, tokenKey, isPlayground, userId := 0, "", false, 0
	if relayInfo != nil {
		tokenId, tokenKey, isPlayground, userId = relayInfo.TokenId, relayInfo.TokenKey, relayInfo.IsPlayground, relayInfo.UserId
	}
	source := ""
	if funding != nil {
		source = funding.Source()
	}
	s.mu.Unlock()

	logger.LogInfo(c, fmt.Sprintf("用户 %d 请求失败, 返还预扣费（token_quota=%s, funding=%s）",
		userId,
		logger.FormatQuota(tokenConsumed),
		source,
	))

	gopool.Go(func() {
		// 1) 退还资金来源
		if !fundingDone && funding != nil {
			if err := funding.Refund(); err != nil {
				common.SysLog("error refunding billing source: " + err.Error())
			} else {
				s.mu.Lock()
				s.refundFundingDone = true
				s.mu.Unlock()
			}
		}
		// 2) 退还令牌额度
		if !tokenDone && tokenConsumed > 0 && !isPlayground {
			if err := model.IncreaseTokenQuota(tokenId, tokenKey, tokenConsumed); err != nil {
				common.SysLog("error refunding token quota: " + err.Error())
			} else {
				s.mu.Lock()
				s.refundTokenDone = true
				s.tokenConsumed = 0
				s.mu.Unlock()
			}
		}
		s.mu.Lock()
		s.refundInFlight = false
		if s.refundFundingDone && s.refundTokenDone {
			s.refunded = true
		}
		s.mu.Unlock()
	})
}

// NeedsRefund 返回是否存在需要退还的预扣状态。
func (s *BillingSession) NeedsRefund() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.needsRefundLocked()
}

func (s *BillingSession) needsRefundLocked() bool {
	if s.settled || s.refunded {
		// fundingSettled 但令牌调整失败时仍需允许回滚；只有完整结算或已退款
		// 的会话才应在这里直接返回。
		return false
	}
	if !s.refundTokenDone && s.tokenConsumed > 0 {
		return true
	}
	if !s.refundFundingDone {
		if wallet, ok := s.funding.(*WalletFunding); ok && wallet.consumed > 0 {
			// Playground requests do not reserve token quota, but they can still
			// reserve wallet quota and therefore must remain refundable.
			return true
		}
		// 订阅可能在 tokenConsumed=0 时仍预扣了额度。
		if sub, ok := s.funding.(*SubscriptionFunding); ok && sub.preConsumed > 0 {
			return true
		}
	}
	return false
}

// GetPreConsumedQuota 返回实际预扣的额度。
func (s *BillingSession) GetPreConsumedQuota() int {
	return s.preConsumedQuota
}

// ClaimPaidConsumedForRebate 确保一次请求结算后只会取一次”充值额度消费部分”，避免重复返利。
// 实际委托给 ClaimWalletConsumedForRebate，仅保留 paid 部分兼容旧调用方。
func (s *BillingSession) ClaimPaidConsumedForRebate() int {
	_, paid := s.ClaimWalletConsumedForRebate()
	return paid
}

// ClaimWalletConsumedForRebate 返回结算后钱包消费的奖励/充值完整拆分，且仅允许调用一次。
// 异步任务将此完整拆分持久化到 task.PrivateData 中，
// 后续退款和差额调整即可按原资金来源比例退回。
func (s *BillingSession) ClaimWalletConsumedForRebate() (reward int, paid int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rebateApplied || !s.settled {
		return 0, 0
	}
	wallet, ok := s.funding.(*WalletFunding)
	if !ok {
		return 0, 0
	}
	s.rebateApplied = true
	return wallet.RewardConsumed(), wallet.PaidConsumed()
}

// ---------------------------------------------------------------------------
// PreConsume — 统一预扣费入口（含信任额度旁路）
// ---------------------------------------------------------------------------

// preConsume 执行预扣费：信任检查 -> 令牌预扣 -> 资金来源预扣。
// 任一步骤失败时原子回滚已完成的步骤。
func (s *BillingSession) preConsume(c *gin.Context, quota int) *types.NewAPIError {
	effectiveQuota := quota

	// ---- 信任额度旁路 ----
	if s.shouldTrust(c) {
		effectiveQuota = 0
		logger.LogInfo(c, fmt.Sprintf("用户 %d 额度充足, 信任且不需要预扣费 (funding=%s)", s.relayInfo.UserId, s.funding.Source()))
	} else if effectiveQuota > 0 {
		logger.LogInfo(c, fmt.Sprintf("用户 %d 需要预扣费 %s (funding=%s)", s.relayInfo.UserId, logger.FormatQuota(effectiveQuota), s.funding.Source()))
	}

	// ---- 1) 预扣令牌额度 ----
	if effectiveQuota > 0 && s.tokenQuotaRequired() {
		if err := PreConsumeTokenQuota(s.relayInfo, effectiveQuota); err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodePreConsumeTokenQuotaFailed, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}
		s.tokenConsumed = effectiveQuota
	}

	// ---- 2) 预扣资金来源 ----
	if err := s.funding.PreConsume(effectiveQuota); err != nil {
		// 预扣费失败，回滚令牌额度
		if s.tokenConsumed > 0 && !s.relayInfo.IsPlayground {
			reserved := s.tokenConsumed
			if rollbackErr := s.rollbackPreConsumedTokenQuota(reserved); rollbackErr != nil {
				// Keep the reservation in memory until compensation succeeds.  The
				// session is normally discarded after this error, so also schedule a
				// bounded background retry to avoid silently burning token quota when
				// the failure was transient.
				common.SysLog(fmt.Sprintf("error rolling back token quota (userId=%d, tokenId=%d, amount=%d, fundingErr=%s): %s",
					s.relayInfo.UserId, s.relayInfo.TokenId, reserved, err.Error(), rollbackErr.Error()))
				relayInfo := s.relayInfo
				adjuster := s.tokenQuotaAdjuster
				gopool.Go(func() {
					for attempt := 0; attempt < 5; attempt++ {
						var retryErr error
						if adjuster != nil {
							retryErr = adjuster(relayInfo, -reserved)
						} else {
							retryErr = model.IncreaseTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, reserved)
						}
						if retryErr == nil {
							return
						}
						time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
					}
					common.SysLog(fmt.Sprintf("token quota rollback retries exhausted (userId=%d, tokenId=%d, amount=%d)", relayInfo.UserId, relayInfo.TokenId, reserved))
				})
				return types.NewErrorWithStatusCode(fmt.Errorf("资金预扣失败且令牌额度回退失败: %w", rollbackErr), types.ErrorCodeUpdateDataError, http.StatusInternalServerError, types.ErrOptionWithSkipRetry())
			}
			// Compensation committed; clear the in-memory reservation.
			s.tokenConsumed = 0
			s.tokenQuota = 0
		}
		// All subscription-window exhaustion errors must be classified as an
		// unavailable subscription so subscription_first/wallet_first can fall
		// back to the wallet.  Keep this tolerant of the legacy English error
		// strings emitted by model while covering the fixed five-hour and
		// aggregate-period variants as well.
		errMsg := err.Error()
		if isSubscriptionQuotaUnavailable(errMsg) {
			return types.NewErrorWithStatusCode(fmt.Errorf("%s", subscriptionQuotaUnavailableMessage(errMsg)), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}
		return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
	}

	s.preConsumedQuota = effectiveQuota
	s.fundingQuota = effectiveQuota
	s.tokenQuota = s.tokenConsumed
	s.quotaStateInitialized = true

	// ---- 同步 RelayInfo 兼容字段 ----
	s.syncRelayInfo()

	return nil
}

func isSubscriptionQuotaUnavailable(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return false
	}
	if strings.Contains(message, "no active subscription") {
		return true
	}
	// Window-specific errors are intentionally matched by their stable
	// "subscription ... quota insufficient" shape rather than by one exact
	// phrase, so future window types retain the same fallback behavior.
	return strings.Contains(message, "subscription") &&
		strings.Contains(message, "quota insufficient")
}

// subscriptionQuotaUnavailableMessage keeps the subscription fallback error
// code stable while making the user-visible reason actionable.  A missing
// subscription and an exhausted quota window require different remediation,
// so they must not be reported as one ambiguous condition.
func subscriptionQuotaUnavailableMessage(message string) string {
	normalized := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(normalized, "no active subscription"):
		return fmt.Sprintf("未配置有效订阅: %s", message)
	case strings.Contains(normalized, "subscription") && strings.Contains(normalized, "quota insufficient"):
		return fmt.Sprintf("订阅窗口额度不足: %s", message)
	default:
		// Keep this helper safe if a future subscription-unavailable reason is
		// added without extending the classifier above.
		return fmt.Sprintf("订阅不可用: %s", message)
	}
}

// shouldTrust 统一信任额度检查，适用于钱包和订阅。
func (s *BillingSession) shouldTrust(c *gin.Context) bool {
	// 异步任务（ForcePreConsume=true）必须预扣全额，不允许信任旁路
	if s.relayInfo.ForcePreConsume {
		return false
	}

	trustQuota := common.GetTrustQuota()
	if trustQuota <= 0 {
		return false
	}

	// 检查令牌是否充足
	tokenTrusted := s.relayInfo.TokenUnlimited
	if !tokenTrusted {
		tokenQuota := c.GetInt("token_quota")
		tokenTrusted = tokenQuota > trustQuota
	}
	if !tokenTrusted {
		return false
	}

	switch s.funding.Source() {
	case BillingSourceWallet:
		return s.relayInfo.UserQuota > trustQuota
	case BillingSourceSubscription:
		// 订阅不能启用信任旁路。原因：
		// 1. PreConsumeUserSubscription 要求 amount>0 来创建预扣记录并锁定订阅
		// 2. SubscriptionFunding.PreConsume 忽略参数，始终用 s.amount 预扣
		// 3. 若信任旁路将 effectiveQuota 设为 0，会导致 preConsumedQuota 与实际订阅预扣不一致
		return false
	default:
		return false
	}
}

// syncRelayInfo 将 BillingSession 的状态同步到 RelayInfo 的兼容字段上。
func (s *BillingSession) syncRelayInfo() {
	info := s.relayInfo
	info.FinalPreConsumedQuota = s.preConsumedQuota
	info.BillingSource = s.funding.Source()

	if sub, ok := s.funding.(*SubscriptionFunding); ok {
		info.SubscriptionId = sub.subscriptionId
		info.SubscriptionPreConsumed = sub.preConsumed
		info.SubscriptionPostDelta = 0
		info.SubscriptionAmountTotal = sub.AmountTotal
		info.SubscriptionAmountUsedAfterPreConsume = sub.AmountUsedAfter
		info.SubscriptionPlanId = sub.PlanId
		info.SubscriptionPlanTitle = sub.PlanTitle
	} else {
		info.SubscriptionId = 0
		info.SubscriptionPreConsumed = 0
	}
}

// ---------------------------------------------------------------------------
// NewBillingSession 工厂 — 根据计费偏好创建会话并处理回退
// ---------------------------------------------------------------------------

func billingUnavailableReason(source string, err *types.NewAPIError) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%s不可用：%s", source, err.Error())
}

func newBillingUnavailableError(reasons ...string) *types.NewAPIError {
	parts := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		reason = strings.TrimSpace(reason)
		if reason != "" {
			parts = append(parts, reason)
		}
	}
	if len(parts) == 0 {
		parts = append(parts, "订阅或钱包额度不足")
	}
	return types.NewErrorWithStatusCode(
		fmt.Errorf("可用额度不足：%s", strings.Join(parts, "；")),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog(),
	)
}

// NewBillingSession 根据用户计费偏好创建 BillingSession，处理 subscription_first / wallet_first 的回退。
func NewBillingSession(c *gin.Context, relayInfo *relaycommon.RelayInfo, preConsumedQuota int) (*BillingSession, *types.NewAPIError) {
	if relayInfo == nil {
		return nil, types.NewError(fmt.Errorf("relayInfo is nil"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	pref := common.NormalizeBillingPreference(relayInfo.UserSetting.BillingPreference)

	// 钱包路径需要先检查用户额度
	tryWallet := func() (*BillingSession, *types.NewAPIError) {
		userQuota, err := model.GetUserQuota(relayInfo.UserId, false)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		}
		if userQuota <= 0 {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("用户额度不足, 剩余额度: %s", logger.FormatQuota(userQuota)),
				types.ErrorCodeInsufficientUserQuota, http.StatusForbidden,
				types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}
		if userQuota-preConsumedQuota < 0 {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("预扣费额度失败, 用户剩余额度: %s, 需要预扣费额度: %s", logger.FormatQuota(userQuota), logger.FormatQuota(preConsumedQuota)),
				types.ErrorCodeInsufficientUserQuota, http.StatusForbidden,
				types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}
		relayInfo.UserQuota = userQuota

		session := &BillingSession{
			relayInfo: relayInfo,
			funding:   &WalletFunding{userId: relayInfo.UserId},
		}
		if apiErr := session.preConsume(c, preConsumedQuota); apiErr != nil {
			return nil, apiErr
		}
		return session, nil
	}

	trySubscription := func() (*BillingSession, *types.NewAPIError) {
		subConsume := int64(preConsumedQuota)
		if subConsume <= 0 {
			subConsume = 1
		}
		// 订阅扣费时使用的模型名：优先取服务商映射后的"对外公开模型名"(public_model_name)，
		// 没有映射时回退到 relay 原始模型名(OriginModelName)。
		// 这样服务商私有订阅按其模型广场上架的公开名扣费，与套餐 model_limits 白名单匹配一致，
		// 避免因上游真实模型名与白名单不一致而无法从订阅扣费。
		subscriptionModelName := relayInfo.OriginModelName
		if publicModelName := common.GetContextKeyString(c, constant.ContextKeyProviderPublicModel); publicModelName != "" {
			subscriptionModelName = publicModelName
		}
		session := &BillingSession{
			relayInfo: relayInfo,
			funding: &SubscriptionFunding{
				requestId: relayInfo.RequestId,
				userId:    relayInfo.UserId,
				modelName: subscriptionModelName,
				amount:    subConsume,
			},
		}
		// 必须传 subConsume 而非 preConsumedQuota，保证 SubscriptionFunding.amount、
		// preConsume 参数和 FinalPreConsumedQuota 三者一致，避免订阅多扣费。
		if apiErr := session.preConsume(c, int(subConsume)); apiErr != nil {
			return nil, apiErr
		}
		return session, nil
	}

	switch pref {
	case "subscription_only":
		return trySubscription()
	case "wallet_only":
		return tryWallet()
	case "wallet_first":
		session, walletErr := tryWallet()
		if walletErr != nil {
			if walletErr.GetErrorCode() == types.ErrorCodeInsufficientUserQuota {
				subSession, subErr := trySubscription()
				if subErr != nil && subErr.GetErrorCode() == types.ErrorCodeInsufficientUserQuota {
					return nil, newBillingUnavailableError(
						billingUnavailableReason("钱包", walletErr),
						billingUnavailableReason("订阅", subErr),
					)
				}
				return subSession, subErr
			}
			return nil, walletErr
		}
		return session, nil
	case "subscription_first":
		fallthrough
	default:
		hasSub, subCheckErr := model.HasActiveUserSubscription(relayInfo.UserId, relayInfo.ProviderId)
		if subCheckErr != nil {
			return nil, types.NewError(subCheckErr, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		}
		if !hasSub {
			walletSession, walletErr := tryWallet()
			if walletErr != nil && walletErr.GetErrorCode() == types.ErrorCodeInsufficientUserQuota {
				return nil, newBillingUnavailableError(
					"订阅不可用：no active subscription",
					billingUnavailableReason("钱包", walletErr),
				)
			}
			return walletSession, walletErr
		}
		session, apiErr := trySubscription()
		if apiErr != nil {
			if apiErr.GetErrorCode() == types.ErrorCodeInsufficientUserQuota {
				walletSession, walletErr := tryWallet()
				if walletErr != nil && walletErr.GetErrorCode() == types.ErrorCodeInsufficientUserQuota {
					return nil, newBillingUnavailableError(
						billingUnavailableReason("订阅", apiErr),
						billingUnavailableReason("钱包", walletErr),
					)
				}
				return walletSession, walletErr
			}
			return nil, apiErr
		}
		return session, nil
	}
}
