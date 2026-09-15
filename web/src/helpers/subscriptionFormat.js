/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

export function formatSubscriptionDuration(plan, t) {
  const unit = plan?.duration_unit || 'month';
  const value = plan?.duration_value || 1;
  const unitLabels = {
    year: t('年'),
    month: t('个月'),
    day: t('天'),
    hour: t('小时'),
    custom: t('自定义'),
  };
  if (unit === 'custom') {
    const seconds = plan?.custom_seconds || 0;
    if (seconds >= 86400) return `${Math.floor(seconds / 86400)} ${t('天')}`;
    if (seconds >= 3600) return `${Math.floor(seconds / 3600)} ${t('小时')}`;
    return `${seconds} ${t('秒')}`;
  }
  return `${value} ${unitLabels[unit] || unit}`;
}

export function formatSubscriptionResetPeriod(plan, t) {
  const period = plan?.quota_reset_period || 'never';
  if (period === 'never') return t('不重置');
  if (period === 'daily') return t('每天');
  if (period === 'weekly') return t('每周');
  if (period === 'monthly') return t('每月');
  if (period === 'yearly' || period === 'annual') return t('每年');
  if (period === 'custom') {
    const seconds = Number(plan?.quota_reset_custom_seconds || 0);
    if (seconds === 365 * 86400) return t('每年');
    if (seconds >= 86400) return `${Math.floor(seconds / 86400)} ${t('天')}`;
    if (seconds >= 3600) return `${Math.floor(seconds / 3600)} ${t('小时')}`;
    if (seconds >= 60) return `${Math.floor(seconds / 60)} ${t('分钟')}`;
    return `${seconds} ${t('秒')}`;
  }
  return t('不重置');
}

// Subscription quota fields are returned by different API generations. Keep
// all fallback logic in one place so cards and admin tables render old plans
// without losing the new rolling/periodic windows.  The API currently emits
// `five_hour` and `weekly` windows, but consumers should not assume those are
// the only values: newer servers may return `daily`, `monthly`, `yearly`, or
// an arbitrary `custom` window in `quota_windows`.
function getSubscriptionSource(value) {
  if (value?.subscription || value?.plan) {
    // A summary carries policy fields on the subscription snapshot while
    // usage/extension fields may still live on the plan. Merge both and let
    // the snapshot win when the same field is present.
    return { ...(value.plan || {}), ...(value.subscription || {}) };
  }
  return value || {};
}

// Prefer a positive value when a migrated response contains both a new scalar
// field (often `0`) and its legacy alias.  A literal zero is still preserved
// when every alias is zero, which means "unlimited" remains distinguishable.
function firstPositiveNumber(...values) {
  let fallback = 0;
  for (const value of values) {
    const numeric = Number(value);
    if (!Number.isFinite(numeric)) continue;
    if (numeric > 0) return numeric;
    if (numeric === 0) fallback = 0;
  }
  return fallback;
}

const WINDOW_TYPE_LABELS = {
  five_hour: '5小时',
  hour: '每小时',
  hourly: '每小时',
  day: '每天',
  daily: '每天',
  week: '每周',
  weekly: '每周',
  month: '每月',
  monthly: '每月',
  year: '每年',
  yearly: '每年',
  annual: '每年',
  custom: '自定义周期',
};

export function formatSubscriptionWindowType(type, t) {
  const normalized = String(type || '').toLowerCase();
  return t(WINDOW_TYPE_LABELS[normalized] || type || '额度窗口');
}

/**
 * Return normalized quota windows from either a summary or a plan object.
 * `quota_windows` is deliberately treated as an extension point.  For old
 * API responses we synthesize windows from the canonical legacy fields so
 * callers can render one consistent shape.
 */
export function getSubscriptionQuotaWindows(value) {
  const source = getSubscriptionSource(value);
  // Some older API/database combinations expose JSON TEXT columns as a
  // string, and an issued subscription may only retain the policy in
  // `quota_windows_snapshot` after its catalog plan is removed. Prefer the
  // derived usage array when it is populated, then fall back to either policy
  // representation. This keeps rendering independent of the storage driver.
  // An issued subscription is immutable.  When the response includes a
  // versioned policy snapshot, never let a later-edited catalog plan's
  // `quota_windows` leak into the subscriber card.  An empty snapshot is
  // meaningful too: it represents a legacy/scalar policy and must suppress
  // the current plan's generic windows.  Unversioned historical responses may
  // still use the catalog as a fallback.
  const snapshot = value?.subscription;
  const hasVersionedSnapshot =
    Number(snapshot?.plan_policy_snapshot_version || 0) > 0;
  const parseWindowArray = (candidate) => {
    if (Array.isArray(candidate)) return candidate;
    if (typeof candidate !== 'string' || candidate.trim() === '') return null;
    try {
      const parsed = JSON.parse(candidate);
      return Array.isArray(parsed) ? parsed : null;
    } catch {
      return null;
    }
  };

  // `subscription.quota_windows` is the derived usage list, whereas
  // `quota_windows_snapshot` is the immutable policy captured at issuance.
  // An empty snapshot is significant: it means the issued plan used scalar
  // (legacy) limits and must not fall through to a newer generic policy on the
  // live catalog plan.  The previous non-empty-only scan accidentally leaked
  // those edited catalog windows into existing subscriptions.
  let raw = null;
  if (hasVersionedSnapshot) {
    // Prefer the usage-populated list when it contains entries so cards show
    // current used/remaining/reset values rather than a policy-only snapshot.
    const usage = parseWindowArray(snapshot?.quota_windows);
    if (usage && usage.length > 0) {
      raw = usage;
    } else if (
      Object.prototype.hasOwnProperty.call(
        snapshot || {},
        'quota_windows_snapshot',
      )
    ) {
      // Preserve an explicitly empty (or malformed) snapshot as authoritative.
      // Scalar fallback below still renders legacy amount/reset fields, but the
      // live plan's generic window list is never consulted.
      raw = parseWindowArray(snapshot?.quota_windows_snapshot) || [];
    } else {
      // Compatibility responses may expose the policy under one of the older
      // names; only non-empty arrays are useful in this branch.
      // Do not inspect `source.quota_windows` here.  `source` merges the live
      // catalog plan and, for a versioned subscription whose snapshot list is
      // omitted by an older API, that would let a later catalog edit change the
      // policy shown for an already-issued entitlement.
      for (const candidate of [snapshot?.quota_window_configs]) {
        const parsed = parseWindowArray(candidate);
        if (parsed && parsed.length > 0) {
          raw = parsed;
          break;
        }
      }
    }
  } else {
    for (const candidate of [
      source.quota_windows,
      source.quota_windows_snapshot,
      source.quota_window_configs,
    ]) {
      const parsed = parseWindowArray(candidate);
      if (parsed && parsed.length > 0) {
        raw = parsed;
        break;
      }
    }
  }
  if (Array.isArray(raw) && raw.length > 0) {
    return raw
      .map((window) => {
        if (!window || typeof window !== 'object') return null;
        const type = String(
          window.type || window.period || window.unit || 'custom',
        ).toLowerCase();
        const limit = Number(
          window.amount ?? window.limit ?? window.quota ?? 0,
        );
        const used = Number(window.used ?? window.amount_used ?? 0);
        const remainingValue = window.remaining ?? window.amount_remaining;
        const remaining =
          remainingValue == null
            ? limit > 0
              ? Math.max(0, limit - used)
              : 0
            : Math.max(0, Number(remainingValue) || 0);
        const resetAt =
          Number(window.reset_at ?? window.next_reset_time ?? 0) || 0;
        const windowSeconds = Number(
          window.window_seconds ??
            window.duration_seconds ??
            window.custom_seconds ??
            0,
        );
        return {
          ...window,
          type,
          limit: Number.isFinite(limit) ? limit : 0,
          used: Number.isFinite(used) ? used : 0,
          remaining,
          reset_at: resetAt,
          window_seconds:
            Number.isFinite(windowSeconds) && windowSeconds > 0
              ? windowSeconds
              : 0,
        };
      })
      .filter(Boolean);
  }

  // When a versioned subscription has no generic policy list, reconstruct the
  // legacy/scalar representation from the issuance snapshot only.  Do not use
  // merged `source` here: it contains the current catalog plan and could import
  // limits that were added or changed after this subscription was issued.
  const scalarSource = hasVersionedSnapshot
    ? {
        ...(snapshot || {}),
        total_amount: snapshot?.amount_total,
        quota_reset_period: snapshot?.quota_reset_period_snapshot,
        quota_reset_custom_seconds:
          snapshot?.quota_reset_custom_seconds_snapshot,
      }
    : source;
  const mode = String(
    scalarSource.quota_window_mode || (hasVersionedSnapshot ? 'legacy' : ''),
  ).toLowerCase();
  const windows = [];
  const fiveHourAmount = firstPositiveNumber(
    scalarSource.five_hour_amount,
    scalarSource.five_hour_quota,
    scalarSource.five_hour_limit,
  );
  const fiveHourWindowSeconds = Number(
    scalarSource.five_hour_window_seconds ?? 18000,
  );
  const weeklyAmount = firstPositiveNumber(
    scalarSource.weekly_amount,
    scalarSource.weekly_quota,
    scalarSource.weekly_limit,
    scalarSource.total_amount,
    scalarSource.amount_total,
  );
  const weeklyUsed = Number(
    scalarSource.weekly_used ?? scalarSource.amount_used ?? 0,
  );
  const weeklyRemaining =
    scalarSource.weekly_remaining == null
      ? weeklyAmount > 0
        ? Math.max(0, weeklyAmount - weeklyUsed)
        : 0
      : Math.max(0, Number(scalarSource.weekly_remaining) || 0);

  if (
    (mode === 'five_hour' || mode === 'dual') &&
    Number.isFinite(fiveHourAmount) &&
    fiveHourAmount > 0
  ) {
    const used = Number(scalarSource.five_hour_used ?? 0);
    windows.push({
      type: 'five_hour',
      limit: fiveHourAmount,
      used: Number.isFinite(used) ? used : 0,
      remaining:
        scalarSource.five_hour_remaining == null
          ? Math.max(0, fiveHourAmount - (Number.isFinite(used) ? used : 0))
          : Math.max(0, Number(scalarSource.five_hour_remaining) || 0),
      reset_at: Number(scalarSource.five_hour_reset_at ?? 0) || 0,
      window_seconds:
        Number.isFinite(fiveHourWindowSeconds) && fiveHourWindowSeconds > 0
          ? fiveHourWindowSeconds
          : 18000,
    });
  }

  const hasPeriodicWindow =
    mode === 'weekly' ||
    mode === 'dual' ||
    mode === 'day' ||
    mode === 'daily' ||
    mode === 'week' ||
    mode === 'month' ||
    mode === 'monthly' ||
    mode === 'year' ||
    mode === 'yearly' ||
    mode === 'annual' ||
    (mode === '' &&
      (scalarSource.weekly_amount != null ||
        scalarSource.amount_total != null ||
        scalarSource.total_amount != null));
  if (hasPeriodicWindow) {
    const period =
      mode === 'day' ||
      mode === 'daily' ||
      mode === 'week' ||
      mode === 'month' ||
      mode === 'monthly' ||
      mode === 'year' ||
      mode === 'yearly' ||
      mode === 'annual'
        ? mode === 'annual'
          ? 'year'
          : mode === 'daily'
            ? 'day'
            : mode === 'monthly'
              ? 'month'
              : mode === 'yearly'
                ? 'year'
                : mode
        : scalarSource.quota_reset_period || 'weekly';
    windows.push({
      type: period,
      limit: Number.isFinite(weeklyAmount) ? weeklyAmount : 0,
      used: Number.isFinite(weeklyUsed) ? weeklyUsed : 0,
      remaining: weeklyRemaining,
      reset_at:
        Number(
          scalarSource.weekly_reset_at ?? scalarSource.next_reset_time ?? 0,
        ) || 0,
      window_seconds:
        period === 'day' || period === 'daily'
          ? 86400
          : period === 'week' || period === 'weekly'
            ? 604800
            : period === 'month' || period === 'monthly'
              ? 2592000
              : period === 'year' || period === 'yearly'
                ? 31536000
                : Number(scalarSource.quota_reset_custom_seconds || 0),
    });
  }
  return windows;
}

export function getSubscriptionQuotaWindowMode(value) {
  const source = getSubscriptionSource(value);
  const explicit = String(source.quota_window_mode || '').toLowerCase();
  if (
    [
      'legacy',
      'five_hour',
      'weekly',
      'dual',
      'generic',
      'hour',
      'day',
      'week',
      'month',
      'year',
      'daily',
      'monthly',
      'yearly',
      'annual',
      'custom',
    ].includes(explicit)
  ) {
    return explicit === 'annual' ? 'yearly' : explicit;
  }
  const windows = getSubscriptionQuotaWindows(source);
  if (windows.length > 1) return 'dual';
  if (windows.length === 1) return windows[0].type || 'custom';
  return 'legacy';
}

export function getSubscriptionWeeklyAmount(value) {
  const source = getSubscriptionSource(value);
  return firstPositiveNumber(
    source.weekly_amount,
    source.weekly_quota,
    source.weekly_limit,
    source.total_amount,
    source.amount_total,
  );
}

export function getSubscriptionFiveHourAmount(value) {
  const source = getSubscriptionSource(value);
  return firstPositiveNumber(
    source.five_hour_amount,
    source.five_hour_quota,
    source.five_hour_limit,
  );
}

export function getSubscriptionFiveHourWindowSeconds(value) {
  const source = getSubscriptionSource(value);
  const seconds = Number(source.five_hour_window_seconds ?? 18000);
  return Number.isFinite(seconds) && seconds > 0 ? seconds : 18000;
}

export function getSubscriptionWeeklyUsed(value) {
  const source = getSubscriptionSource(value);
  const used = Number(source.weekly_used ?? source.amount_used ?? 0);
  return Number.isFinite(used) ? used : 0;
}

export function getSubscriptionWeeklyRemaining(value) {
  const source = getSubscriptionSource(value);
  if (source.weekly_remaining != null) {
    const remaining = Number(source.weekly_remaining);
    return Number.isFinite(remaining) ? Math.max(0, remaining) : 0;
  }
  const amount = getSubscriptionWeeklyAmount(source);
  return amount > 0
    ? Math.max(0, amount - getSubscriptionWeeklyUsed(source))
    : 0;
}

export function getSubscriptionWeeklyResetAt(value) {
  const source = getSubscriptionSource(value);
  return Number(source.weekly_reset_at ?? source.next_reset_time ?? 0) || 0;
}

export function getSubscriptionFiveHourUsed(value) {
  const source = getSubscriptionSource(value);
  const used = Number(source.five_hour_used ?? 0);
  return Number.isFinite(used) ? used : 0;
}

export function getSubscriptionFiveHourRemaining(value) {
  const source = getSubscriptionSource(value);
  if (source.five_hour_remaining != null) {
    const remaining = Number(source.five_hour_remaining);
    return Number.isFinite(remaining) ? Math.max(0, remaining) : 0;
  }
  const amount = getSubscriptionFiveHourAmount(source);
  return amount > 0
    ? Math.max(0, amount - getSubscriptionFiveHourUsed(source))
    : 0;
}

export function getSubscriptionFiveHourResetAt(value) {
  const source = getSubscriptionSource(value);
  return Number(source.five_hour_reset_at ?? 0) || 0;
}

export function formatSubscriptionWindowSeconds(seconds, t) {
  const value = Number(seconds || 0);
  if (!Number.isFinite(value) || value <= 0) return t('余额永久有效');
  if (value % 86400 === 0) return `${value / 86400} ${t('天')}`;
  if (value % 3600 === 0) return `${value / 3600} ${t('小时')}`;
  if (value % 60 === 0) return `${value / 60} ${t('分钟')}`;
  return `${value} ${t('秒')}`;
}
