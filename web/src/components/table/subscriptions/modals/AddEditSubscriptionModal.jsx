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

import React, { useEffect, useState, useRef } from 'react';
import {
  Avatar,
  Button,
  Card,
  Col,
  Form,
  Input,
  InputNumber,
  Row,
  Select,
  SideSheet,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconCalendarClock,
  IconClose,
  IconCreditCard,
  IconDelete,
  IconPlus,
  IconSave,
} from '@douyinfe/semi-icons';
import { Clock, RefreshCw } from 'lucide-react';
import {
  API,
  getModelCategories,
  selectFilter,
  showError,
  showSuccess,
} from '../../../../helpers';
import {
  quotaToDisplayAmount,
  displayAmountToQuota,
} from '../../../../helpers/quota';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const durationUnitOptions = [
  { value: 'year', label: '年' },
  { value: 'month', label: '月' },
  { value: 'day', label: '日' },
  { value: 'hour', label: '小时' },
  { value: 'custom', label: '自定义(秒)' },
];

const resetPeriodOptions = [
  { value: 'never', label: '不重置' },
  { value: 'daily', label: '每天' },
  { value: 'weekly', label: '每周' },
  { value: 'monthly', label: '每月' },
  { value: 'yearly', label: '每年' },
  { value: 'custom', label: '自定义(秒)' },
];

const quotaWindowModeOptions = [
  { value: 'legacy', label: '传统总额度' },
  { value: 'five_hour', label: '滚动窗口' },
  { value: 'weekly', label: '周期额度' },
  { value: 'dual', label: '滚动窗口 + 周期额度' },
  { value: 'generic', label: '通用多窗口' },
];

const quotaWindowTypeOptions = [
  { value: 'hour', label: '小时' },
  { value: 'day', label: '日' },
  { value: 'week', label: '周' },
  { value: 'month', label: '月' },
  { value: 'year', label: '年' },
  { value: 'custom', label: '自定义(秒)' },
];

const quotaWindowResetModeOptions = [
  { value: 'rolling', label: '滚动周期' },
  { value: 'calendar', label: '自然周期' },
];

const quotaWindowTypes = new Set(
  quotaWindowTypeOptions.map((option) => option.value),
);

// Keep the client-side window arithmetic in sync with
// model.MaxSubscriptionQuotaWindowSeconds.  The API rejects a fixed-unit
// window when an explicitly supplied `window_seconds` does not equal
// `duration * unit_seconds`; normalising that value here prevents a plan from
// appearing to save successfully only to be rejected by the API afterwards.
const MAX_QUOTA_WINDOW_SECONDS = 100 * 366 * 24 * 60 * 60;
const QUOTA_WINDOW_UNIT_SECONDS = Object.freeze({
  hour: 60 * 60,
  day: 24 * 60 * 60,
  week: 7 * 24 * 60 * 60,
  month: 30 * 24 * 60 * 60,
  year: 365 * 24 * 60 * 60,
});

const isPositiveInteger = (value) =>
  Number.isFinite(value) && Number.isSafeInteger(value) && value > 0;

const isNonNegativeInteger = (value) =>
  Number.isFinite(value) && Number.isSafeInteger(value) && value >= 0;

const genericQuotaWindowModes = new Set([
  'generic',
  '5h',
  '5hour',
  '5_hours',
  'five-hours',
  'hour',
  'hourly',
  'hours',
  'day',
  'daily',
  'days',
  'week',
  'month',
  'monthly',
  'months',
  'year',
  'yearly',
  'years',
  'annual',
  'annually',
  'custom',
  'second',
  'seconds',
  'duration',
]);

let quotaWindowKeySequence = 0;

const getQuotaWindowKey = () => {
  quotaWindowKeySequence += 1;
  return `quota-window-${quotaWindowKeySequence}`;
};

const normalizeQuotaWindowType = (value) => {
  const type = String(value || '').toLowerCase();
  const aliases = {
    '5h': 'hour',
    '5hour': 'hour',
    five_hour: 'hour',
    hourly: 'hour',
    hours: 'hour',
    daily: 'day',
    days: 'day',
    weekly: 'week',
    weeks: 'week',
    monthly: 'month',
    months: 'month',
    yearly: 'year',
    years: 'year',
    annual: 'year',
    annually: 'year',
    second: 'custom',
    seconds: 'custom',
    duration: 'custom',
  };
  const normalized = aliases[type] || type;
  return quotaWindowTypes.has(normalized) ? normalized : 'week';
};

const defaultQuotaWindowResetMode = (type) =>
  ['day', 'week', 'month', 'year'].includes(type) ? 'calendar' : 'rolling';

const createQuotaWindow = () => ({
  _key: getQuotaWindowKey(),
  name: '',
  type: 'week',
  reset_mode: 'calendar',
  duration: 1,
  window_seconds: 0,
  amount: 0,
});

const getRawQuotaWindows = (plan) => {
  let raw = plan?.quota_windows ?? plan?.quota_window_configs ?? [];
  if (typeof raw === 'string') {
    try {
      raw = JSON.parse(raw);
    } catch {
      raw = [];
    }
  }
  return Array.isArray(raw) ? raw : [];
};

const getQuotaWindowCandidates = (plan) => {
  const existing = getRawQuotaWindows(plan);
  if (existing.length > 0) return existing;

  const rawMode = String(plan?.quota_window_mode || '').toLowerCase();
  if (!genericQuotaWindowModes.has(rawMode)) return [];
  const isFiveHourAlias = ['5h', '5hour', '5_hours', 'five-hours'].includes(
    rawMode,
  );
  let type = normalizeQuotaWindowType(rawMode);
  if (rawMode === 'generic') {
    const resetPeriod = String(plan?.quota_reset_period || '').toLowerCase();
    const resetTypes = {
      daily: 'day',
      weekly: 'week',
      monthly: 'month',
      yearly: 'year',
      annual: 'year',
      custom: 'custom',
    };
    type = resetTypes[resetPeriod] || 'week';
  }
  // JSON responses from migrated plans commonly include the new scalar field
  // with a zero value while retaining the legacy total_amount.  Nullish
  // coalescing would stop at that zero and render an apparently empty window;
  // choose the first positive value instead.
  const firstPositive = (...values) => {
    for (const value of values) {
      const numeric = Number(value);
      if (Number.isFinite(numeric) && numeric > 0) return numeric;
    }
    return 0;
  };
  const amount =
    type === 'hour'
      ? firstPositive(
          plan?.five_hour_amount,
          plan?.five_hour_quota,
          plan?.five_hour_limit,
          plan?.total_amount,
        )
      : firstPositive(
          plan?.weekly_amount,
          plan?.weekly_quota,
          plan?.weekly_limit,
          plan?.total_amount,
        );
  const windowSeconds =
    type === 'custom'
      ? Number(plan?.quota_reset_custom_seconds ?? plan?.custom_seconds ?? 0)
      : isFiveHourAlias
        ? 18000
        : 0;
  return [
    {
      type,
      amount,
      duration: isFiveHourAlias ? 5 : 1,
      window_seconds: windowSeconds,
      reset_mode: defaultQuotaWindowResetMode(type),
    },
  ];
};

const parseQuotaWindows = (plan) =>
  getQuotaWindowCandidates(plan)
    .filter((window) => window && typeof window === 'object')
    .map((window) => {
      const type = normalizeQuotaWindowType(
        window.type || window.period || window.unit,
      );
      const rawAmount = Number(
        window.amount ?? window.limit ?? window.quota ?? 0,
      );
      const displayAmount = quotaToDisplayAmount(
        Number.isFinite(rawAmount) ? rawAmount : 0,
      );
      const duration = Number(window.duration ?? window.duration_value ?? 1);
      const windowSeconds = Number(
        window.window_seconds ??
          window.duration_seconds ??
          window.custom_seconds ??
          0,
      );
      let resetMode = String(window.reset_mode || '').toLowerCase();
      if (window.calendar === true) resetMode = 'calendar';
      if (window.rolling === true) resetMode = 'rolling';
      if (!['rolling', 'calendar'].includes(resetMode)) {
        resetMode = defaultQuotaWindowResetMode(type);
      }
      return {
        _key: getQuotaWindowKey(),
        name: String(window.name || ''),
        type,
        reset_mode: resetMode,
        duration: Number.isFinite(duration) && duration > 0 ? duration : 1,
        window_seconds:
          Number.isFinite(windowSeconds) && windowSeconds > 0
            ? windowSeconds
            : 0,
        amount: Number.isFinite(displayAmount) ? displayAmount : 0,
      };
    });

const getQuotaWindowModeForForm = (plan) => {
  const rawMode = String(plan?.quota_window_mode || '').toLowerCase();
  if (
    getRawQuotaWindows(plan).length > 0 ||
    genericQuotaWindowModes.has(rawMode)
  ) {
    return 'generic';
  }
  if (['legacy', 'five_hour', 'weekly', 'dual'].includes(rawMode)) {
    return rawMode;
  }
  if (
    Number(plan?.five_hour_amount || 0) > 0 &&
    Number(plan?.weekly_amount || 0) > 0
  ) {
    return 'dual';
  }
  if (Number(plan?.five_hour_amount || 0) > 0) return 'five_hour';
  if (Number(plan?.weekly_amount || 0) > 0) return 'weekly';
  return 'legacy';
};

const QuotaWindowsEditor = ({ windows, onChange, t }) => {
  const updateWindow = (index, patch) => {
    onChange(
      windows.map((window, currentIndex) =>
        currentIndex === index ? { ...window, ...patch } : window,
      ),
    );
  };

  const removeWindow = (index) => {
    onChange(windows.filter((_, currentIndex) => currentIndex !== index));
  };

  return (
    <Card className='!rounded-2xl shadow-sm border-0 mb-4'>
      <div className='flex items-start justify-between gap-3 mb-3'>
        <div>
          <Text className='text-lg font-medium'>{t('独立额度窗口')}</Text>
          <div className='text-xs text-gray-600 mt-1'>
            {t(
              '可组合多个滚动或自然周期额度；所有窗口同时生效，任一额度用尽即回落钱包。',
            )}
          </div>
          <div className='text-xs text-gray-500 mt-1'>
            {t('模型限制由“适用模型”统一应用到所有额度窗口。')}
          </div>
        </div>
        <Button
          type='primary'
          theme='light'
          icon={<IconPlus />}
          disabled={windows.length >= 16}
          onClick={() => onChange([...windows, createQuotaWindow()])}
        >
          {t('添加额度窗口')}
        </Button>
      </div>

      {windows.length === 0 ? (
        <div className='rounded-xl border border-dashed border-gray-300 p-5 text-center'>
          <Text type='tertiary'>{t('暂无额度窗口，请至少添加一个')}</Text>
        </div>
      ) : (
        <div className='space-y-3'>
          {windows.map((window, index) => (
            <div
              key={window._key}
              className='rounded-xl border border-gray-200 p-3'
            >
              <div className='flex items-center justify-between mb-2'>
                <Space>
                  <Tag color='blue' shape='circle'>
                    {t('额度窗口')} {index + 1}
                  </Tag>
                  {window.name ? <Text>{window.name}</Text> : null}
                </Space>
                <Button
                  type='danger'
                  theme='borderless'
                  size='small'
                  icon={<IconDelete />}
                  aria-label={t('删除额度窗口')}
                  onClick={() => removeWindow(index)}
                />
              </div>

              <Row gutter={12}>
                <Col span={12}>
                  <div className='mb-3'>
                    <Text strong>{t('窗口名称')}</Text>
                    <Input
                      className='mt-1'
                      value={window.name}
                      placeholder={t('可选，例如：5 小时')}
                      showClear
                      onChange={(name) => updateWindow(index, { name })}
                    />
                  </div>
                </Col>
                <Col span={12}>
                  <div className='mb-3'>
                    <Text strong>{t('窗口类型')}</Text>
                    <Select
                      className='mt-1'
                      value={window.type}
                      style={{ width: '100%' }}
                      onChange={(type) =>
                        updateWindow(index, {
                          type,
                          reset_mode: defaultQuotaWindowResetMode(type),
                          duration: type === 'custom' ? 1 : window.duration,
                          // Fixed-unit windows are derived from their type and
                          // duration. Clear a previously derived value when
                          // changing units so it cannot become inconsistent.
                          window_seconds:
                            type === 'custom' ? window.window_seconds : 0,
                        })
                      }
                    >
                      {quotaWindowTypeOptions.map((option) => (
                        <Select.Option key={option.value} value={option.value}>
                          {t(option.label)}
                        </Select.Option>
                      ))}
                    </Select>
                  </div>
                </Col>
                <Col span={12}>
                  <div className='mb-3'>
                    <Text strong>{t('重置方式')}</Text>
                    <Select
                      className='mt-1'
                      value={window.reset_mode}
                      style={{ width: '100%' }}
                      onChange={(resetMode) =>
                        updateWindow(index, { reset_mode: resetMode })
                      }
                    >
                      {quotaWindowResetModeOptions.map((option) => (
                        <Select.Option key={option.value} value={option.value}>
                          {t(option.label)}
                        </Select.Option>
                      ))}
                    </Select>
                  </div>
                </Col>
                <Col span={12}>
                  <div className='mb-3'>
                    <Text strong>{t('周期数')}</Text>
                    <InputNumber
                      className='mt-1'
                      value={window.duration}
                      min={1}
                      max={
                        QUOTA_WINDOW_UNIT_SECONDS[window.type]
                          ? Math.floor(
                              MAX_QUOTA_WINDOW_SECONDS /
                                QUOTA_WINDOW_UNIT_SECONDS[window.type],
                            )
                          : 1
                      }
                      precision={0}
                      disabled={window.type === 'custom'}
                      style={{ width: '100%' }}
                      onChange={(duration) =>
                        updateWindow(index, {
                          duration,
                          window_seconds: 0,
                        })
                      }
                    />
                  </div>
                </Col>
                <Col span={12}>
                  <div className='mb-1'>
                    <Text strong>{t('精确窗口秒数')}</Text>
                    <InputNumber
                      className='mt-1'
                      value={window.window_seconds}
                      min={window.type === 'custom' ? 1 : 0}
                      max={MAX_QUOTA_WINDOW_SECONDS}
                      precision={0}
                      style={{ width: '100%' }}
                      onChange={(windowSeconds) =>
                        updateWindow(index, {
                          window_seconds: windowSeconds,
                        })
                      }
                    />
                    <div className='text-xs text-gray-500 mt-1'>
                      {window.type === 'custom'
                        ? t('自定义窗口必须填写精确窗口秒数')
                        : t('0 表示按窗口类型和周期数自动计算')}
                    </div>
                  </div>
                </Col>
                <Col span={12}>
                  <div className='mb-1'>
                    <Text strong>{t('窗口额度')}</Text>
                    <InputNumber
                      className='mt-1'
                      value={window.amount}
                      min={0}
                      precision={2}
                      style={{ width: '100%' }}
                      onChange={(amount) => updateWindow(index, { amount })}
                    />
                    <div className='text-xs text-gray-500 mt-1'>
                      {t('原生额度')}：{displayAmountToQuota(window.amount)}
                    </div>
                  </div>
                </Col>
              </Row>
            </div>
          ))}
        </div>
      )}

      <div className='text-xs text-gray-500 mt-3'>
        {t('额度窗口最多支持 16 个')}
      </div>
    </Card>
  );
};

const AddEditSubscriptionModal = ({
  visible,
  handleClose,
  editingPlan,
  placement = 'left',
  refresh,
  t,
  // plansApi/modelsApi 由父组件注入：主站用 /api/subscription/admin/* 与 /api/user/models，
  // 服务商用 /api/provider/subscription/*，使同一弹窗在两套管理页复用。
  plansApi = '/api/subscription/admin/plans',
  modelsApi = '/api/user/models',
}) => {
  const [loading, setLoading] = useState(false);
  const [groupOptions, setGroupOptions] = useState([]);
  const [groupLoading, setGroupLoading] = useState(false);
  const [modelOptions, setModelOptions] = useState([]);
  const [modelLoading, setModelLoading] = useState(false);
  const [quotaWindows, setQuotaWindows] = useState([]);
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);
  const isEdit = editingPlan?.plan?.id !== undefined;
  const formKey = isEdit ? `edit-${editingPlan?.plan?.id}` : 'create';

  const getInitValues = () => ({
    title: '',
    subtitle: '',
    price_amount: 0,
    currency: 'USD',
    duration_unit: 'month',
    duration_value: 1,
    custom_seconds: 0,
    quota_reset_period: 'never',
    quota_reset_custom_seconds: 0,
    enabled: true,
    allow_purchase: true,
    model_limits: [],
    sort_order: 0,
    max_purchase_per_user: 0,
    total_purchase_limit: 0,
    purchase_limit_group: '',
    quota_window_mode: 'legacy',
    five_hour_amount: 0,
    five_hour_window_seconds: 18000,
    weekly_amount: 0,
    total_amount: 0,
    upgrade_group: '',
    stripe_price_id: '',
    stripe_price_cny_id: '',
    creem_product_id: '',
    waffo_pancake_product_id: '',
  });

  const buildFormValues = () => {
    const base = getInitValues();
    if (editingPlan?.plan?.id === undefined) return base;
    const p = editingPlan.plan || {};
    return {
      ...base,
      title: p.title || '',
      subtitle: p.subtitle || '',
      price_amount: Number(p.price_amount || 0),
      currency: 'USD',
      duration_unit: p.duration_unit || 'month',
      duration_value: Number(p.duration_value || 1),
      custom_seconds: Number(p.custom_seconds || 0),
      quota_reset_period:
        p.quota_reset_period === 'custom' &&
        Number(p.quota_reset_custom_seconds || 0) === 365 * 86400
          ? 'yearly'
          : p.quota_reset_period || 'never',
      quota_reset_custom_seconds: Number(p.quota_reset_custom_seconds || 0),
      enabled: p.enabled !== false,
      allow_purchase: Number(p.allow_purchase ?? 1) === 1,
      model_limits:
        Array.isArray(p.model_limits) && p.model_limits.length > 0
          ? p.model_limits.filter(Boolean)
          : typeof p.model_limits === 'string' && p.model_limits !== ''
            ? p.model_limits
                .split(',')
                .map((model) => model.trim())
                .filter(Boolean)
            : [],
      sort_order: Number(p.sort_order || 0),
      max_purchase_per_user: Number(p.max_purchase_per_user || 0),
      total_purchase_limit: Number(p.total_purchase_limit || 0),
      purchase_limit_group: p.purchase_limit_group || '',
      // Newer API responses include weekly_amount=0 on legacy plans. Infer
      // the mode from positive limits instead of property presence.
      quota_window_mode: getQuotaWindowModeForForm(p),
      five_hour_amount: Number(
        quotaToDisplayAmount(p.five_hour_amount || 0).toFixed(2),
      ),
      five_hour_window_seconds: Number(p.five_hour_window_seconds || 18000),
      weekly_amount: Number(
        quotaToDisplayAmount(p.weekly_amount ?? p.total_amount ?? 0).toFixed(2),
      ),
      total_amount: Number(
        quotaToDisplayAmount(p.total_amount ?? p.weekly_amount ?? 0).toFixed(2),
      ),
      upgrade_group: p.upgrade_group || '',
      stripe_price_id: p.stripe_price_id || '',
      stripe_price_cny_id: p.stripe_price_cny_id || '',
      creem_product_id: p.creem_product_id || '',
      waffo_pancake_product_id: p.waffo_pancake_product_id || '',
    };
  };

  useEffect(() => {
    if (!visible) return;
    setQuotaWindows(parseQuotaWindows(editingPlan?.plan));
  }, [visible, editingPlan?.plan?.id]);

  useEffect(() => {
    if (!visible) return;
    setGroupLoading(true);
    API.get('/api/group')
      .then((res) => {
        if (res.data?.success) {
          setGroupOptions(res.data?.data || []);
        } else {
          setGroupOptions([]);
        }
      })
      .catch(() => setGroupOptions([]))
      .finally(() => setGroupLoading(false));
  }, [visible]);

  useEffect(() => {
    if (!visible) return;
    setModelLoading(true);
    // 拉取模型候选：主站取全量用户模型，服务商取其模型广场上架模型；
    // 后端 ProviderListSubscriptionPlanModels 已做去重排序，前端直接渲染为多选项。
    API.get(modelsApi)
      .then((res) => {
        if (!res.data?.success) {
          setModelOptions([]);
          return;
        }
        const categories = getModelCategories(t);
        const options = (res.data?.data || []).map((model) => {
          let icon = null;
          for (const [key, category] of Object.entries(categories)) {
            if (key !== 'all' && category.filter({ model_name: model })) {
              icon = category.icon;
              break;
            }
          }
          return {
            label: (
              <span className='flex items-center gap-1'>
                {icon}
                {model}
              </span>
            ),
            value: model,
          };
        });
        setModelOptions(options);
      })
      .catch(() => setModelOptions([]))
      .finally(() => setModelLoading(false));
  }, [visible, t]);

  const submit = async (values) => {
    if (!values.title || values.title.trim() === '') {
      showError(t('套餐标题不能为空'));
      return;
    }

    let normalizedQuotaWindows = [];
    if (values.quota_window_mode === 'generic') {
      if (quotaWindows.length === 0) {
        showError(t('通用额度模式至少需要一个窗口'));
        return;
      }
      if (quotaWindows.length > 16) {
        showError(t('额度窗口最多支持 16 个'));
        return;
      }
      for (const window of quotaWindows) {
        const type = normalizeQuotaWindowType(window.type);
        const displayAmount = Number(window.amount);
        const amount = displayAmountToQuota(displayAmount);
        const duration = Number(window.duration);
        const windowSeconds = Number(window.window_seconds);
        if (
          !Number.isFinite(displayAmount) ||
          displayAmount <= 0 ||
          !Number.isFinite(amount) ||
          !Number.isSafeInteger(amount) ||
          amount <= 0
        ) {
          showError(t('额度窗口的额度必须大于 0'));
          return;
        }

        // InputNumber normally emits integers because the editor uses
        // precision={0}, but values can still arrive as strings, null, or
        // Infinity from imported/legacy plans.  Validate the canonical
        // representation before sending it to the API.
        if (!isNonNegativeInteger(windowSeconds)) {
          showError(t('精确窗口秒数必须是非负整数'));
          return;
        }

        let normalizedDuration = 1;
        let normalizedWindowSeconds = windowSeconds;
        if (type === 'custom') {
          if (!isPositiveInteger(windowSeconds)) {
            showError(t('自定义窗口必须填写精确窗口秒数'));
            return;
          }
          if (windowSeconds > MAX_QUOTA_WINDOW_SECONDS) {
            showError(t('额度窗口秒数超过最大值'));
            return;
          }
        } else {
          if (!isPositiveInteger(duration)) {
            showError(t('额度窗口的周期必须是正整数'));
            return;
          }
          const unitSeconds = QUOTA_WINDOW_UNIT_SECONDS[type];
          if (!unitSeconds) {
            showError(t('额度窗口类型无效'));
            return;
          }
          if (duration > Math.floor(MAX_QUOTA_WINDOW_SECONDS / unitSeconds)) {
            showError(t('额度窗口秒数超过最大值'));
            return;
          }
          const expectedSeconds = duration * unitSeconds;
          // A zero value means "derive from type and duration" in the UI.
          // Once derived, always send the exact value so API validation and
          // subsequent edits see a single canonical representation.
          if (windowSeconds > 0 && windowSeconds !== expectedSeconds) {
            showError(t('固定周期的精确窗口秒数必须与周期数匹配'));
            return;
          }
          normalizedDuration = duration;
          normalizedWindowSeconds = expectedSeconds;
        }

        normalizedQuotaWindows.push({
          name: String(window.name || '').trim(),
          type,
          amount,
          duration: normalizedDuration,
          window_seconds: normalizedWindowSeconds,
          reset_mode: ['rolling', 'calendar'].includes(window.reset_mode)
            ? window.reset_mode
            : defaultQuotaWindowResetMode(type),
        });
      }
    }

    setLoading(true);
    try {
      const payload = {
        plan: {
          ...values,
          price_amount: Number(values.price_amount || 0),
          currency: 'USD',
          // Keep the duration value canonical even for custom plans.  The
          // backend validates explicit zero values instead of silently
          // defaulting them, while custom duration is carried by
          // custom_seconds.
          duration_value:
            values.duration_unit === 'custom'
              ? 1
              : Number(values.duration_value || 0),
          custom_seconds: Number(values.custom_seconds || 0),
          quota_reset_period:
            values.quota_reset_period === 'never' &&
            ['weekly', 'dual'].includes(values.quota_window_mode)
              ? 'weekly'
              : values.quota_reset_period || 'never',
          quota_reset_custom_seconds:
            values.quota_reset_period === 'custom'
              ? Number(values.quota_reset_custom_seconds || 0)
              : 0,
          sort_order: Number(values.sort_order || 0),
          max_purchase_per_user: Number(values.max_purchase_per_user || 0),
          total_purchase_limit: Number(values.total_purchase_limit || 0),
          purchase_limit_group: String(values.purchase_limit_group || '')
            .trim()
            .toLowerCase(),
          quota_window_mode: quotaWindowModeOptions.some(
            (option) => option.value === values.quota_window_mode,
          )
            ? values.quota_window_mode
            : 'legacy',
          five_hour_amount: displayAmountToQuota(
            ['five_hour', 'dual'].includes(values.quota_window_mode)
              ? Number(values.five_hour_amount || 0)
              : 0,
          ),
          five_hour_window_seconds: Number(
            values.five_hour_window_seconds || 18000,
          ),
          weekly_amount: displayAmountToQuota(
            Number(
              ['weekly', 'dual'].includes(values.quota_window_mode)
                ? values.weekly_amount || 0
                : 0,
            ),
          ),
          quota_windows:
            values.quota_window_mode === 'generic'
              ? normalizedQuotaWindows
              : [],
          // Keep the legacy field in sync for older API/database versions.
          total_amount: displayAmountToQuota(
            Number(
              ['weekly', 'dual'].includes(values.quota_window_mode)
                ? values.weekly_amount || 0
                : values.total_amount || 0,
            ),
          ),
          upgrade_group: values.upgrade_group || '',
          allow_purchase: values.allow_purchase === false ? 0 : 1,
          model_limits: Array.isArray(values.model_limits)
            ? values.model_limits.join(',')
            : '',
        },
      };
      if (editingPlan?.plan?.id) {
        // 编辑：PUT {plansApi}/{id}，后端按当前用户身份校验套餐归属(服务商只能改自己的)。
        const res = await API.put(
          `${plansApi}/${editingPlan.plan.id}`,
          payload,
        );
        if (res.data?.success) {
          showSuccess(t('更新成功'));
          handleClose();
          refresh?.();
        } else {
          showError(res.data?.message || t('更新失败'));
        }
      } else {
        // 新建：POST {plansApi}，后端会把 provider_id 绑定为当前服务商(服务商接口)或保留请求体(主站接口)。
        const res = await API.post(plansApi, payload);
        if (res.data?.success) {
          showSuccess(t('创建成功'));
          handleClose();
          refresh?.();
        } else {
          showError(res.data?.message || t('创建失败'));
        }
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <SideSheet
        placement={placement}
        title={
          <Space>
            {isEdit ? (
              <Tag color='blue' shape='circle'>
                {t('更新')}
              </Tag>
            ) : (
              <Tag color='green' shape='circle'>
                {t('新建')}
              </Tag>
            )}
            <Title heading={4} className='m-0'>
              {isEdit ? t('更新套餐信息') : t('创建新的订阅套餐')}
            </Title>
          </Space>
        }
        bodyStyle={{ padding: '0' }}
        visible={visible}
        width={isMobile ? '100%' : 600}
        footer={
          <div className='flex justify-end bg-white'>
            <Space>
              <Button
                theme='solid'
                onClick={() => formApiRef.current?.submitForm()}
                icon={<IconSave />}
                loading={loading}
              >
                {t('提交')}
              </Button>
              <Button
                theme='light'
                type='primary'
                onClick={handleClose}
                icon={<IconClose />}
              >
                {t('取消')}
              </Button>
            </Space>
          </div>
        }
        closeIcon={null}
        onCancel={handleClose}
      >
        <Spin spinning={loading}>
          <Form
            key={formKey}
            initValues={buildFormValues()}
            getFormApi={(api) => (formApiRef.current = api)}
            onSubmit={submit}
          >
            {({ values }) => (
              <div className='p-2'>
                {/* 基本信息 */}
                <Card className='!rounded-2xl shadow-sm border-0 mb-4'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='blue'
                      className='mr-2 shadow-md'
                    >
                      <IconCalendarClock size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('基本信息')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('套餐的基本信息和定价')}
                      </div>
                    </div>
                  </div>

                  <Row gutter={12}>
                    <Col span={24}>
                      <Form.Input
                        field='title'
                        label={t('套餐标题')}
                        placeholder={t('例如：基础套餐')}
                        required
                        rules={[
                          { required: true, message: t('请输入套餐标题') },
                        ]}
                        showClear
                      />
                    </Col>

                    <Col span={24}>
                      <Form.Input
                        field='subtitle'
                        label={t('套餐副标题')}
                        placeholder={t('例如：适合轻度使用')}
                        showClear
                      />
                    </Col>

                    <Col span={12}>
                      <Form.InputNumber
                        field='price_amount'
                        label={t('实付金额')}
                        required
                        min={0}
                        precision={2}
                        rules={[{ required: true, message: t('请输入金额') }]}
                        style={{ width: '100%' }}
                      />
                    </Col>

                    <Col span={12}>
                      <Form.Select
                        field='quota_window_mode'
                        label={t('额度模式')}
                        onChange={(mode) => {
                          if (mode === 'generic' && quotaWindows.length === 0) {
                            setQuotaWindows([createQuotaWindow()]);
                          }
                        }}
                      >
                        {quotaWindowModeOptions.map((option) => (
                          <Select.Option
                            key={option.value}
                            value={option.value}
                          >
                            {t(option.label)}
                          </Select.Option>
                        ))}
                      </Form.Select>
                    </Col>

                    {values.quota_window_mode !== 'generic' &&
                      (['five_hour', 'dual'].includes(
                        values.quota_window_mode,
                      ) ? (
                        <>
                          <Col span={12}>
                            <Form.InputNumber
                              field='five_hour_amount'
                              label={t('滚动窗口额度')}
                              min={0}
                              precision={2}
                              extraText={`${t('0 表示关闭')} · ${t('原生额度')}：${displayAmountToQuota(
                                values.five_hour_amount,
                              )}`}
                              style={{ width: '100%' }}
                            />
                          </Col>
                          <Col span={12}>
                            <Form.InputNumber
                              field='five_hour_window_seconds'
                              label={t('滚动窗口秒数')}
                              min={60}
                              precision={0}
                              extraText={t(
                                '可配置任意窗口时长，例如 5 小时为 18000 秒',
                              )}
                              style={{ width: '100%' }}
                            />
                          </Col>
                          {values.quota_window_mode === 'dual' && (
                            <Col span={12}>
                              <Form.InputNumber
                                field='weekly_amount'
                                label={t('周期额度')}
                                min={0}
                                precision={2}
                                extraText={`${t('0 表示不限')} · ${t('原生额度')}：${displayAmountToQuota(
                                  values.weekly_amount,
                                )}`}
                                style={{ width: '100%' }}
                              />
                            </Col>
                          )}
                        </>
                      ) : values.quota_window_mode === 'weekly' ? (
                        <Col span={12}>
                          <Form.InputNumber
                            field='weekly_amount'
                            label={t('周期额度')}
                            min={0}
                            precision={2}
                            extraText={`${t('0 表示不限')} · ${t('原生额度')}：${displayAmountToQuota(
                              values.weekly_amount,
                            )}`}
                            style={{ width: '100%' }}
                          />
                        </Col>
                      ) : (
                        <Col span={12}>
                          <Form.InputNumber
                            field='total_amount'
                            label={t('总额度')}
                            required
                            min={0}
                            precision={2}
                            rules={[
                              { required: true, message: t('请输入总额度') },
                            ]}
                            extraText={`${t('0 表示不限')} · ${t('原生额度')}：${displayAmountToQuota(
                              values.total_amount,
                            )}`}
                            style={{ width: '100%' }}
                          />
                        </Col>
                      ))}

                    <Col span={12}>
                      <Form.Select
                        field='upgrade_group'
                        label={t('升级分组')}
                        showClear
                        loading={groupLoading}
                        placeholder={t('不升级')}
                        extraText={t(
                          '购买或手动新增订阅会升级到该分组；当套餐失效/过期或手动作废/删除后，将回退到升级前分组。回退不会立即生效，通常会有几分钟延迟。',
                        )}
                      >
                        <Select.Option value=''>{t('不升级')}</Select.Option>
                        {(groupOptions || []).map((g) => (
                          <Select.Option key={g} value={g}>
                            {g}
                          </Select.Option>
                        ))}
                      </Form.Select>
                    </Col>

                    <Col span={12}>
                      <Form.Input
                        field='currency'
                        label={t('币种')}
                        disabled
                        extraText={t('由全站货币展示设置统一控制')}
                      />
                    </Col>

                    <Col span={12}>
                      <Form.InputNumber
                        field='sort_order'
                        label={t('排序')}
                        precision={0}
                        style={{ width: '100%' }}
                      />
                    </Col>

                    <Col span={12}>
                      <Form.InputNumber
                        field='max_purchase_per_user'
                        label={t('每用户购买上限')}
                        min={0}
                        precision={0}
                        extraText={t('0 表示不限')}
                        style={{ width: '100%' }}
                      />
                    </Col>

                    <Col span={12}>
                      <Form.InputNumber
                        field='total_purchase_limit'
                        label={t('全局发放上限')}
                        min={0}
                        precision={0}
                        extraText={t(
                          '0 表示不限；购买、赠送、空投和注册赠送都会占用',
                        )}
                        style={{ width: '100%' }}
                      />
                    </Col>

                    <Col span={24}>
                      <Form.Input
                        field='purchase_limit_group'
                        label={t('购买限制组')}
                        placeholder={t('例如：k3-month-card')}
                        maxLength={64}
                        showClear
                        extraText={t(
                          '相同限制组的套餐共享每用户购买上限和全局发放上限；留空则仅限制当前套餐。',
                        )}
                      />
                    </Col>

                    <Col span={12}>
                      <Form.Switch
                        field='enabled'
                        label={t('启用状态')}
                        size='large'
                      />
                    </Col>

                    <Col span={12}>
                      <Form.Switch
                        field='allow_purchase'
                        label={t('允许订阅')}
                        size='large'
                      />
                    </Col>

                    <Col span={24}>
                      <Form.Select
                        field='model_limits'
                        label={t('适用模型')}
                        placeholder={t('不选择则支持所有模型')}
                        multiple
                        optionList={modelOptions}
                        loading={modelLoading}
                        filter={selectFilter}
                        autoClearSearchValue={false}
                        searchPosition='dropdown'
                        showClear
                        extraText={t('仅这些模型会优先使用该订阅额度')}
                        style={{ width: '100%' }}
                      />
                    </Col>
                  </Row>
                </Card>

                {values.quota_window_mode === 'generic' ? (
                  <QuotaWindowsEditor
                    windows={quotaWindows}
                    onChange={setQuotaWindows}
                    t={t}
                  />
                ) : null}

                {/* 有效期设置 */}
                <Card className='!rounded-2xl shadow-sm border-0 mb-4'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='green'
                      className='mr-2 shadow-md'
                    >
                      <Clock size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('有效期设置')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('配置套餐的有效时长')}
                      </div>
                    </div>
                  </div>

                  <Row gutter={12}>
                    <Col span={12}>
                      <Form.Select
                        field='duration_unit'
                        label={t('有效期单位')}
                        required
                        rules={[{ required: true }]}
                      >
                        {durationUnitOptions.map((o) => (
                          <Select.Option key={o.value} value={o.value}>
                            {t(o.label)}
                          </Select.Option>
                        ))}
                      </Form.Select>
                    </Col>

                    <Col span={12}>
                      {values.duration_unit === 'custom' ? (
                        <Form.InputNumber
                          field='custom_seconds'
                          label={t('自定义秒数')}
                          required
                          min={1}
                          precision={0}
                          rules={[{ required: true, message: t('请输入秒数') }]}
                          style={{ width: '100%' }}
                        />
                      ) : (
                        <Form.InputNumber
                          field='duration_value'
                          label={t('有效期数值')}
                          required
                          min={1}
                          precision={0}
                          rules={[{ required: true, message: t('请输入数值') }]}
                          style={{ width: '100%' }}
                        />
                      )}
                    </Col>
                  </Row>
                </Card>

                {/* 额度重置 */}
                <Card className='!rounded-2xl shadow-sm border-0 mb-4'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='orange'
                      className='mr-2 shadow-md'
                    >
                      <RefreshCw size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('额度重置')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('支持周期性重置套餐权益额度')}
                      </div>
                    </div>
                  </div>

                  {values.quota_window_mode === 'generic' ? (
                    <Text type='tertiary'>
                      {t('通用多窗口模式的重置规则在各额度窗口中独立配置')}
                    </Text>
                  ) : ['legacy', 'weekly', 'dual'].includes(
                      values.quota_window_mode,
                    ) ? (
                    <Row gutter={12}>
                      <Col span={12}>
                        <Form.Select
                          field='quota_reset_period'
                          label={t('重置周期')}
                        >
                          {resetPeriodOptions.map((o) => (
                            <Select.Option key={o.value} value={o.value}>
                              {t(o.label)}
                            </Select.Option>
                          ))}
                        </Form.Select>
                      </Col>
                      <Col span={12}>
                        {['custom'].includes(values.quota_reset_period) ? (
                          <Form.InputNumber
                            field='quota_reset_custom_seconds'
                            label={t('自定义秒数')}
                            required
                            min={60}
                            precision={0}
                            rules={[
                              { required: true, message: t('请输入秒数') },
                            ]}
                            style={{ width: '100%' }}
                          />
                        ) : (
                          <Form.InputNumber
                            field='quota_reset_custom_seconds'
                            label={t('自定义秒数')}
                            min={0}
                            precision={0}
                            style={{ width: '100%' }}
                            disabled
                          />
                        )}
                      </Col>
                    </Row>
                  ) : (
                    <Text type='tertiary'>
                      {t('仅滚动窗口额度，不使用日/周/月周期重置')}
                    </Text>
                  )}
                </Card>

                {/* 第三方支付配置 */}
                <Card className='!rounded-2xl shadow-sm border-0 mb-4'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='purple'
                      className='mr-2 shadow-md'
                    >
                      <IconCreditCard size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('第三方支付配置')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('Stripe/Creem 商品ID（可选）')}
                      </div>
                    </div>
                  </div>

                  <Row gutter={12}>
                    <Col span={12}>
                      <Form.Input
                        field='stripe_price_id'
                        label='Stripe PriceId ($)'
                        placeholder='price_...'
                        showClear
                      />
                    </Col>

                    <Col span={12}>
                      <Form.Input
                        field='stripe_price_cny_id'
                        label={'Stripe PriceId (\uFFE5)'}
                        placeholder='price_...'
                        showClear
                      />
                    </Col>

                    <Col span={24}>
                      <Form.Input
                        field='creem_product_id'
                        label='Creem ProductId'
                        placeholder='prod_...'
                        showClear
                      />
                    </Col>

                    <Col span={24}>
                      <Form.Input
                        field='waffo_pancake_product_id'
                        label='Waffo Pancake ProductId'
                        placeholder='prod_...'
                        showClear
                      />
                    </Col>
                  </Row>
                </Card>
              </div>
            )}
          </Form>
        </Spin>
      </SideSheet>
    </>
  );
};

export default AddEditSubscriptionModal;
