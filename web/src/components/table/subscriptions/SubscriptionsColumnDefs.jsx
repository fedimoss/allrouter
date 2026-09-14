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

import React from 'react';
import {
  Button,
  Modal,
  Space,
  Tag,
  Typography,
  Popover,
  Divider,
  Badge,
  Tooltip,
} from '@douyinfe/semi-ui';
import { renderQuota } from '../../../helpers';
import { convertUSDToCurrency } from '../../../helpers/render';
import {
  formatSubscriptionWindowSeconds,
  formatSubscriptionWindowType,
  getSubscriptionQuotaWindows,
  getSubscriptionQuotaWindowMode,
  getSubscriptionWeeklyAmount,
} from '../../../helpers/subscriptionFormat';

const { Text } = Typography;

function formatDuration(plan, t) {
  if (!plan) return '';
  const u = plan.duration_unit || 'month';
  if (u === 'custom') {
    return `${t('自定义')} ${plan.custom_seconds || 0}s`;
  }
  const unitMap = {
    year: t('年'),
    month: t('月'),
    day: t('日'),
    hour: t('小时'),
  };
  return `${plan.duration_value || 0}${unitMap[u] || u}`;
}

function formatResetPeriod(plan, t) {
  const period = plan?.quota_reset_period || 'never';
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

// The backend enforces purchase limits across every plan that shares a
// provider-scoped purchase_limit_group. Build the same effective scope here
// so the admin table does not show per-plan counters that disagree with the
// actual admission check.
function getPurchaseScopeKey(plan) {
  const group = String(plan?.purchase_limit_group || '')
    .trim()
    .toLowerCase();
  if (group) {
    return `group:${Number(plan?.provider_id || 0)}:${group}`;
  }
  return plan?.id ? `plan:${plan.id}` : '';
}

function minPositive(current, candidate) {
  const value = Number(candidate || 0);
  if (!Number.isFinite(value) || value <= 0) return current;
  return current <= 0 ? value : Math.min(current, value);
}

function addNonNegative(current, candidate) {
  const value = Number(candidate || 0);
  if (!Number.isFinite(value) || value <= 0) return current;
  return current + value;
}

function buildPurchaseScopeStats(plans = []) {
  const stats = new Map();
  (plans || []).forEach((item) => {
    const plan = item?.plan || item;
    const key = getPurchaseScopeKey(plan);
    if (!key) return;
    const current = stats.get(key) || {
      maxPurchasePerUser: 0,
      totalPurchaseLimit: 0,
      issuedCount: 0,
      reservedCount: 0,
    };
    current.maxPurchasePerUser = minPositive(
      current.maxPurchasePerUser,
      plan?.max_purchase_per_user,
    );
    current.totalPurchaseLimit = minPositive(
      current.totalPurchaseLimit,
      plan?.total_purchase_limit,
    );
    current.issuedCount = addNonNegative(current.issuedCount, plan?.issued_count);
    current.reservedCount = addNonNegative(
      current.reservedCount,
      plan?.reserved_count,
    );
    stats.set(key, current);
  });
  return stats;
}

function getPurchaseScopeStats(plan, stats) {
  return (
    stats?.get(getPurchaseScopeKey(plan)) || {
      maxPurchasePerUser: Number(plan?.max_purchase_per_user || 0),
      totalPurchaseLimit: Number(plan?.total_purchase_limit || 0),
      issuedCount: Number(plan?.issued_count || 0),
      reservedCount: Number(plan?.reserved_count || 0),
    }
  );
}

const renderPlanTitle = (text, record, t, purchaseScopeStats) => {
  const subtitle = record?.plan?.subtitle;
  const plan = record?.plan;
  const quotaWindowMode = getSubscriptionQuotaWindowMode(plan);
  const quotaWindows = getSubscriptionQuotaWindows(plan);
  const weeklyAmount = getSubscriptionWeeklyAmount(plan);
  const displayAmount =
    quotaWindowMode === 'dual' || quotaWindowMode === 'weekly'
      ? weeklyAmount
      : Number(plan?.total_amount || 0);
  const popoverContent = (
    <div style={{ width: 260 }}>
      <Text strong>{text}</Text>
      {subtitle && (
        <Text type='tertiary' style={{ display: 'block', marginTop: 4 }}>
          {subtitle}
        </Text>
      )}
      <Divider margin={12} />
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
        <Text type='tertiary'>{t('价格')}</Text>
        <Text strong style={{ color: 'var(--semi-color-success)' }}>
          {convertUSDToCurrency(Number(plan?.price_amount || 0), 2)}
        </Text>
        {quotaWindows.length > 0 ? (
          quotaWindows.map((window, index) => (
            <React.Fragment key={`${window.type}-${index}`}>
              <Text type='tertiary'>
                {formatSubscriptionWindowType(window.type, t)}
              </Text>
              <Text>
                {window.limit > 0 ? renderQuota(window.limit) : t('不限')}
                {window.window_seconds > 0 &&
                  ` · ${formatSubscriptionWindowSeconds(window.window_seconds, t)}`}
              </Text>
            </React.Fragment>
          ))
        ) : (
          <>
            <Text type='tertiary'>
              {quotaWindowMode === 'dual' ? t('每周额度') : t('总额度')}
            </Text>
            {displayAmount > 0 ? (
              <Tooltip content={`${t('原生额度')}：${displayAmount}`}>
                <Text>{renderQuota(displayAmount)}</Text>
              </Tooltip>
            ) : (
              <Text>{t('不限')}</Text>
            )}
          </>
        )}
        <Text type='tertiary'>{t('升级分组')}</Text>
        <Text>{plan?.upgrade_group ? plan.upgrade_group : t('不升级')}</Text>
        <Text type='tertiary'>{t('每用户购买上限')}</Text>
        <Text>
          {getPurchaseScopeStats(plan, purchaseScopeStats).maxPurchasePerUser > 0
            ? getPurchaseScopeStats(plan, purchaseScopeStats)
                .maxPurchasePerUser
            : t('不限')}
        </Text>
        <Text type='tertiary'>{t('全局发放上限')}</Text>
        <Text>
          {getPurchaseScopeStats(plan, purchaseScopeStats).totalPurchaseLimit > 0
            ? `${
                getPurchaseScopeStats(plan, purchaseScopeStats).issuedCount +
                getPurchaseScopeStats(plan, purchaseScopeStats).reservedCount
              }/${getPurchaseScopeStats(plan, purchaseScopeStats).totalPurchaseLimit}`
            : t('不限')}
        </Text>
        <Text type='tertiary'>{t('购买限制组')}</Text>
        <Text>
          {plan?.purchase_limit_group ? plan.purchase_limit_group : t('无')}
        </Text>
        <Text type='tertiary'>{t('有效期')}</Text>
        <Text>{formatDuration(plan, t)}</Text>
        <Text type='tertiary'>{t('重置')}</Text>
        <Text>{formatResetPeriod(plan, t)}</Text>
      </div>
    </div>
  );

  return (
    <Popover content={popoverContent} position='rightTop' showArrow>
      <div style={{ cursor: 'pointer', maxWidth: 180 }}>
        <Text strong ellipsis={{ showTooltip: false }}>
          {text}
        </Text>
        {subtitle && (
          <Text
            type='tertiary'
            ellipsis={{ showTooltip: false }}
            style={{ display: 'block' }}
          >
            {subtitle}
          </Text>
        )}
      </div>
    </Popover>
  );
};

const renderPrice = (text) => {
  return (
    <Text strong style={{ color: 'var(--semi-color-success)' }}>
      {convertUSDToCurrency(Number(text || 0), 2)}
    </Text>
  );
};

const renderPurchaseLimit = (text, record, t, purchaseScopeStats) => {
  const limit = getPurchaseScopeStats(record?.plan, purchaseScopeStats)
    .maxPurchasePerUser;
  return (
    <Text type={limit > 0 ? 'secondary' : 'tertiary'}>
      {limit > 0 ? limit : t('不限')}
    </Text>
  );
};

const renderGlobalLimit = (text, record, t, purchaseScopeStats) => {
  const plan = record?.plan || {};
  const scope = getPurchaseScopeStats(plan, purchaseScopeStats);
  const limit = scope.totalPurchaseLimit;
  const issued = scope.issuedCount;
  const reserved = scope.reservedCount;
  const content = (
    <Text type={limit > 0 ? 'secondary' : 'tertiary'}>
      {limit > 0 ? `${issued + reserved}/${limit}` : t('不限')}
    </Text>
  );
  return (
    <Tooltip content={`${t('已发放')}: ${issued}，${t('预占')}: ${reserved}`}>
      {content}
    </Tooltip>
  );
};

const renderDuration = (text, record, t) => {
  return <Text type='secondary'>{formatDuration(record?.plan, t)}</Text>;
};

const renderEnabled = (text, record, t) => {
  return text ? (
    <Tag
      color='white'
      shape='circle'
      type='light'
      prefixIcon={<Badge dot type='success' />}
    >
      {t('启用')}
    </Tag>
  ) : (
    <Tag
      color='white'
      shape='circle'
      type='light'
      prefixIcon={<Badge dot type='danger' />}
    >
      {t('禁用')}
    </Tag>
  );
};

const renderAllowPurchase = (text, record, t) => {
  const allowed = Number(record?.plan?.allow_purchase ?? 1) === 1;
  return allowed ? (
    <Tag
      color='white'
      shape='circle'
      type='light'
      prefixIcon={<Badge dot type='success' />}
    >
      {t('允许订阅')}
    </Tag>
  ) : (
    <Tag
      color='white'
      shape='circle'
      type='light'
      prefixIcon={<Badge dot type='warning' />}
    >
      {t('暂停订阅')}
    </Tag>
  );
};

const renderModelLimits = (text, record, t) => {
  const models = String(record?.plan?.model_limits || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
  if (models.length === 0) {
    return <Text type='tertiary'>{t('全部模型')}</Text>;
  }
  const label =
    models.length <= 2
      ? models.join(', ')
      : `${models.slice(0, 2).join(', ')} +${models.length - 2}`;
  return (
    <Tooltip content={models.join(', ')}>
      <Text type='secondary'>{label}</Text>
    </Tooltip>
  );
};

const renderTotalAmount = (text, record, t) => {
  const plan = record?.plan || {};
  const windows = getSubscriptionQuotaWindows(plan);
  const weekly = getSubscriptionWeeklyAmount(plan);
  if (windows.length > 0) {
    return (
      <div className='text-xs leading-5'>
        {windows.map((window, index) => (
          <div key={`${window.type}-${index}`}>
            {formatSubscriptionWindowType(window.type, t)}:{' '}
            {window.limit > 0 ? renderQuota(window.limit) : t('不限')}
          </div>
        ))}
      </div>
    );
  }
  return (
    <Text type={weekly > 0 ? 'secondary' : 'tertiary'}>
      {weekly > 0 ? (
        <Tooltip content={`${t('原生额度')}：${weekly}`}>
          <span>{renderQuota(weekly)}</span>
        </Tooltip>
      ) : (
        t('不限')
      )}
    </Text>
  );
};

const renderUpgradeGroup = (text, record, t) => {
  const group = record?.plan?.upgrade_group || '';
  return (
    <Text type={group ? 'secondary' : 'tertiary'}>
      {group ? group : t('不升级')}
    </Text>
  );
};

const renderResetPeriod = (text, record, t) => {
  const plan = record?.plan || {};
  const windows = getSubscriptionQuotaWindows(plan);
  if (windows.length > 0) {
    return (
      <Text type='secondary'>
        {windows
          .map((window) => formatSubscriptionWindowType(window.type, t))
          .join(' + ')}
      </Text>
    );
  }
  const period = plan?.quota_reset_period || 'never';
  const isNever = period === 'never';
  return (
    <Text type={isNever ? 'tertiary' : 'secondary'}>
      {formatResetPeriod(plan, t)}
    </Text>
  );
};

const renderPaymentConfig = (text, record, t, enableEpay) => {
  // 判断是否配置了 Stripe 支付：USD Price ID 或 CNY Price ID 任一存在即可
  const hasStripe =
    !!record?.plan?.stripe_price_id || !!record?.plan?.stripe_price_cny_id;
  const hasCreem = !!record?.plan?.creem_product_id;
  const hasEpay = !!enableEpay;

  return (
    <Space spacing={4}>
      {hasStripe && (
        <Tag color='violet' shape='circle'>
          Stripe
        </Tag>
      )}
      {hasCreem && (
        <Tag color='cyan' shape='circle'>
          Creem
        </Tag>
      )}
      {hasEpay && (
        <Tag color='light-green' shape='circle'>
          {t('易支付')}
        </Tag>
      )}
    </Space>
  );
};

const renderOperations = (text, record, { openEdit, setPlanEnabled, t }) => {
  const isEnabled = record?.plan?.enabled;

  const handleToggle = () => {
    if (isEnabled) {
      Modal.confirm({
        title: t('确认禁用'),
        content: t('禁用后用户端不再展示，但历史订单不受影响。是否继续？'),
        centered: true,
        onOk: () => setPlanEnabled(record, false),
        okText: t('确定'),
        cancelText: t('取消'),
      });
    } else {
      Modal.confirm({
        title: t('确认启用'),
        content: t('启用后套餐将在用户端展示。是否继续？'),
        centered: true,
        onOk: () => setPlanEnabled(record, true),
        okText: t('确定'),
        cancelText: t('取消'),
      });
    }
  };

  return (
    <Space spacing={8}>
      <Button
        theme='light'
        type='tertiary'
        size='small'
        onClick={() => openEdit(record)}
      >
        {t('编辑')}
      </Button>
      {isEnabled ? (
        <Button theme='light' type='danger' size='small' onClick={handleToggle}>
          {t('禁用')}
        </Button>
      ) : (
        <Button
          theme='light'
          type='primary'
          size='small'
          onClick={handleToggle}
        >
          {t('启用')}
        </Button>
      )}
    </Space>
  );
};

export const getSubscriptionsColumns = ({
  t,
  openEdit,
  setPlanEnabled,
  enableEpay,
  allPlans = [],
}) => {
  const purchaseScopeStats = buildPurchaseScopeStats(allPlans);
  return [
    {
      title: 'ID',
      dataIndex: ['plan', 'id'],
      width: 60,
      render: (text) => <Text type='tertiary'>#{text}</Text>,
    },
    {
      title: t('套餐'),
      dataIndex: ['plan', 'title'],
      width: 200,
      render: (text, record) =>
        renderPlanTitle(text, record, t, purchaseScopeStats),
    },
    {
      title: t('价格'),
      dataIndex: ['plan', 'price_amount'],
      width: 100,
      render: (text) => renderPrice(text),
    },
    {
      title: t('每用户购买上限'),
      width: 90,
      render: (text, record) =>
        renderPurchaseLimit(text, record, t, purchaseScopeStats),
    },
    {
      title: t('全局发放上限'),
      width: 120,
      render: (text, record) =>
        renderGlobalLimit(text, record, t, purchaseScopeStats),
    },
    {
      title: t('优先级'),
      dataIndex: ['plan', 'sort_order'],
      width: 80,
      render: (text) => <Text type='tertiary'>{Number(text || 0)}</Text>,
    },
    {
      title: t('有效期'),
      width: 100,
      render: (text, record) => renderDuration(text, record, t),
    },
    {
      title: t('重置'),
      width: 80,
      render: (text, record) => renderResetPeriod(text, record, t),
    },
    {
      title: t('状态'),
      dataIndex: ['plan', 'enabled'],
      width: 80,
      render: (text, record) => renderEnabled(text, record, t),
    },
    {
      title: t('订阅开关'),
      dataIndex: ['plan', 'allow_purchase'],
      width: 100,
      render: (text, record) => renderAllowPurchase(text, record, t),
    },
    {
      title: t('适用模型'),
      dataIndex: ['plan', 'model_limits'],
      width: 160,
      render: (text, record) => renderModelLimits(text, record, t),
    },
    {
      title: t('支付渠道'),
      width: 180,
      render: (text, record) =>
        renderPaymentConfig(text, record, t, enableEpay),
    },
    {
      title: t('总额度'),
      width: 100,
      render: (text, record) => renderTotalAmount(text, record, t),
    },
    {
      title: t('升级分组'),
      width: 100,
      render: (text, record) => renderUpgradeGroup(text, record, t),
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      fixed: 'right',
      width: 160,
      render: (text, record) =>
        renderOperations(text, record, { openEdit, setPlanEnabled, t }),
    },
  ];
};
