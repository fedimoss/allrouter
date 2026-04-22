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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Badge,
  Button,
  Card,
  Empty,
  Input,
  Modal,
  Pagination,
  Select,
  Table,
  Tag,
  Toast,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { useTranslation } from 'react-i18next';
import {
  BadgeDollarSign,
  CalendarCheck2,
  CheckCircle,
  Coins,
  Gift,
  Wallet,
  BarChart3
} from 'lucide-react';
import { IconSearch, IconCopy, IconEyeOpened } from '@douyinfe/semi-icons';
import { API, timestamp2string, formatDisplayMoney } from '../../helpers';
import {
  getTopupBizTypeConfig,
  isInviteRebateTopup,
  isSubscriptionTopup,
} from '../../helpers/topup';
import { isAdmin } from '../../helpers/utils';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Text } = Typography;

const BILL_PERIOD_OPTIONS = [
  { value: 'day', label: '今天' },
  { value: 'week', label: '本周' },
  { value: 'month', label: '本月' },
  { value: 'year', label: '本年' },
];

const BILLING_SUMMARY_DEFAULTS = {
  expense: 0,
  bonus: '',
  net_change: 0,
  expense_trend: 0,
  topup: 0,
};

const STATUS_CONFIG = {
  success: { type: 'success', key: '成功' },
  pending: { type: 'warning', key: '待支付' },
  failed: { type: 'danger', key: '失败' },
  expired: { type: 'danger', key: '已过期' },
};

const PAYMENT_METHOD_MAP = {
  stripe: 'Stripe',
  creem: 'Creem',
  waffo: 'Waffo',
  alipay: '支付宝',
  wxpay: '微信',
  redemptionCode: '兑换码',
  redemption_code: '兑换码',
};

const HISTORY_PAGE_SIZE_OPTIONS = [10, 20, 50, 100];

const amountFormatter = new Intl.NumberFormat('en-US', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 6,
});

const toNumber = (value) => {
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : 0;
};

const formatCurrency = (value, { signed = false } = {}) => {
  const numeric = toNumber(value);
  const formattedValue = amountFormatter.format(Math.abs(numeric));
  if (numeric < 0) {
    return `-$${formattedValue}`;
  }
  if (signed && numeric > 0) {
    return `+$${formattedValue}`;
  }
  return `$${formattedValue}`;
};

const formatPercent = (value) => {
  if (value === null || value === undefined || value === '') {
    return '--';
  }
  const text = String(value).trim();
  if (!text) {
    return '--';
  }
  return text.includes('%') ? text : `${text}%`;
};

const getPercentToneClassName = (value) => {
  const numeric = parseFloat(String(value || '').replace('%', ''));
  if (Number.isNaN(numeric)) {
    return 'text-cyan-500';
  }
  if (numeric > 0) {
    return 'text-cyan-500';
  }
  if (numeric < 0) {
    return 'text-rose-500';
  }
  return 'text-slate-400';
};

const DETAIL_MODAL_WIDTH = 600;
const DETAIL_MODAL_CONTENT_HEIGHT = 640;

const DETAIL_TYPE_CONFIG = {
  payment: { color: 'cyan', label: '在线充值' },
  subscription: { color: 'violet', label: '订阅套餐' },
  redemption: { color: 'green', label: '兑换码' },
};

const getDetailTypeConfig = (record, t) => {
  const config = DETAIL_TYPE_CONFIG[record?.biz_type] || {
    color: 'grey',
    label: record?.biz_type || '订单详情',
  };
  return {
    color: config.color,
    label: t(config.label),
  };
};

const getDetailStatusTag = (status, t) => {
  const statusMap = {
    success: {
      color: '#22C55E',
      background: '#EEFDF3',
      borderColor: '#B7E6C1',
      label: t('已结算'),
    },
    pending: {
      color: '#F59E0B',
      background: '#FFF7E8',
      borderColor: '#F7D59F',
      label: t('待支付'),
    },
    failed: {
      color: '#EF4444',
      background: '#FFF1F2',
      borderColor: '#F8C6CC',
      label: t('失败'),
    },
    expired: {
      color: '#EF4444',
      background: '#FFF1F2',
      borderColor: '#F8C6CC',
      label: t('已过期'),
    },
  };
  return (
    statusMap[status] || {
      color: '#64748B',
      background: '#F8FAFC',
      borderColor: '#E2E8F0',
      label: status || '--',
    }
  );
};

const isSubscriptionDetailRecord = (record) => {
  if (record?.biz_type) {
    return record.biz_type === 'subscription';
  }
  const tradeNo = (record?.trade_no || '').toLowerCase();
  return Number(record?.amount || 0) === 0 && tradeNo.startsWith('sub');
};

const buildDetailSections = (record, t) => {
  if (!record) {
    return {
      mainInfo: [],
      commissionGroups: [],
    };
  }

  const typeConfig = getDetailTypeConfig(record, t);
  const statusConfig = getDetailStatusTag(record.status, t);
  const displayName = record.display_name || '--';
  const tradeNo = record.trade_no || '--';
  const paymentMethod = PAYMENT_METHOD_MAP[record.payment_method]
    ? t(PAYMENT_METHOD_MAP[record.payment_method])
    : record.payment_method || '--';
  const createTime = timestamp2string(record.complete_time || record.create_time);
  const payAmount = formatCurrency(record.money, { signed: Number(record.money) > 0 });
  const quotaAmount = isSubscriptionDetailRecord(record)
    ? t('订阅套餐')
    : `${record.amount ?? '--'}`;

  return {
    mainInfo: [
      { label: t('关联主单号'), value: tradeNo, copyValue: tradeNo },
      {
        label: t('子结算单号'),
        value: `${tradeNo}${record.id ? `-${record.id}` : ''}`,
        copyValue: `${tradeNo}${record.id ? `-${record.id}` : ''}`,
      },
      { label: t('交易时间'), value: createTime },
      {
        label: t('交易类型'),
        value: (
          <Tag
            size='small'
            color={typeConfig.color}
            shape='circle'
            style={{
              minWidth: 72,
              justifyContent: 'center',
              borderRadius: 6,
            }}
          >
            {typeConfig.label}
          </Tag>
        ),
      },
      { label: t('金额变动'), value: payAmount },
    ],
    commissionGroups: [
      {
        title: t('一级分佣（供应商）'),
        items: [
          { label: t('接收账户'), value: paymentMethod, copyValue: paymentMethod },
          { label: t('分佣比例'), value: isSubscriptionDetailRecord(record) ? '--' : '100%' },
          { label: t('结算金额'), value: payAmount },
          {
            label: t('状态'),
            value: (
              <span
                className='inline-flex items-center gap-2 rounded-md border px-3 py-1 text-sm'
                style={{
                  color: statusConfig.color,
                  background: statusConfig.background,
                  borderColor: statusConfig.borderColor,
                }}
              >
                <span
                  className='h-2 w-2 rounded-full'
                  style={{ backgroundColor: statusConfig.color }}
                />
                {statusConfig.label}
              </span>
            ),
          },
        ],
      },
      {
        title: t('二级分佣（代理商）'),
        items: [
          { label: t('接收账户'), value: displayName, copyValue: displayName },
          { label: t('分佣比例'), value: record.display_name ? '0%' : '--' },
          { label: t('结算金额'), value: quotaAmount },
          {
            label: t('状态'),
            value: (
              <span
                className='inline-flex items-center gap-2 rounded-md border px-3 py-1 text-sm'
                style={{
                  color: statusConfig.color,
                  background: statusConfig.background,
                  borderColor: statusConfig.borderColor,
                }}
              >
                <span
                  className='h-2 w-2 rounded-full'
                  style={{ backgroundColor: statusConfig.color }}
                />
                {statusConfig.label}
              </span>
            ),
          },
        ],
      },
    ],
  };
};

const DetailField = ({ label, value, copyValue, onCopy }) => (
  <div className='grid grid-cols-[96px,1fr] items-center gap-x-2 gap-y-1 py-2'>
    <span className='text-sm font-medium text-[#94A3B8]'>{label}</span>
    <div className='flex min-w-0 items-center gap-2'>
      <span className='min-w-0 break-all text-[15px] font-semibold text-[#475569]'>
        {value}
      </span>
      {copyValue ? (
        <Button
          theme='borderless'
          type='tertiary'
          size='small'
          icon={<IconCopy />}
          onClick={() => onCopy(copyValue)}
          style={{
            color: '#475569',
            padding: 0,
            minWidth: 'auto',
            height: 20,
          }}
        />
      ) : null}
    </div>
  </div>
);

const Billing = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [billingPeriod, setBillingPeriod] = useState('day');
  const [billingSummary, setBillingSummary] = useState(BILLING_SUMMARY_DEFAULTS);
  const [billingSummaryLoading, setBillingSummaryLoading] = useState(false);

  const [activePage, setActivePage] = useState(1);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyRows, setHistoryRows] = useState([]);
  const [historyTotal, setHistoryTotal] = useState(0);
  const [historyPageSize, setHistoryPageSize] = useState(10);
  const [historyKeyword, setHistoryKeyword] = useState('');
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedRecord, setSelectedRecord] = useState(null);

  const userIsAdmin = useMemo(() => isAdmin(), []);
  const billingPageTitle = userIsAdmin ? t('账单管理') : t('账单中心');
  const billingPageDescription = userIsAdmin
    ? t('查看全平台充值、邀请返佣与用户账单状态。')
    : t('查看您的充值、邀请返佣与消费明细。');

  const loadTopups = async (page, pageSize, keyword) => {
    setHistoryLoading(true);
    try {
      const base = userIsAdmin ? '/api/user/topup' : '/api/user/topup/self';
      const qs =
        `p=${page}&page_size=${pageSize}` +
        (keyword ? `&keyword=${encodeURIComponent(keyword)}` : '');
      const res = await API.get(`${base}?${qs}`);
      const { success, message, data } = res.data;
      if (success) {
        setHistoryRows(data.items || []);
        setHistoryTotal(data.total || 0);
      } else {
        Toast.error({ content: message || t('加载失败') });
      }
    } catch (error) {
      Toast.error({ content: t('加载账单失败') });
    } finally {
      setHistoryLoading(false);
    }
  };

  useEffect(() => {
    let mounted = true;
    setBillingSummaryLoading(true);
    let url = userIsAdmin
      ? `/api/bill?period=${billingPeriod}`
      : `/api/bill/self?period=${billingPeriod}`;

    API.get(url)
      .then((res) => {
        if (!mounted) return;

        const payload = res.data || {};
        const source =
          payload.data && typeof payload.data === 'object' ? payload.data : payload;

        if (payload.success === false) {
          Toast.error({ content: payload.message || t('加载失败') });
          return;
        }

        setBillingSummary({
          ...BILLING_SUMMARY_DEFAULTS,
          expense: source.expense,
          bonus: source.bonus,
          net_change: source.net_change,
          expense_trend: source.expense_trend,
          topup: source.topup,
        });
      })
      .catch(() => {
        if (!mounted) return;
        Toast.error({ content: t('加载账单失败') });
      })
      .finally(() => {
        if (mounted) {
          setBillingSummaryLoading(false);
        }
      });

    return () => {
      mounted = false;
    };
  }, [billingPeriod, t]);

  useEffect(() => {
    loadTopups(activePage, historyPageSize, historyKeyword);
  }, [activePage, historyPageSize, historyKeyword, userIsAdmin]);

  const handleAdminComplete = async (tradeNo) => {
    try {
      const res = await API.post('/api/user/topup/complete', {
        trade_no: tradeNo,
      });
      const { success, message } = res.data;
      if (success) {
        Toast.success({ content: t('补单成功') });
        await loadTopups(activePage, historyPageSize, historyKeyword);
      } else {
        Toast.error({ content: message || t('补单失败') });
      }
    } catch (error) {
      Toast.error({ content: t('补单失败') });
    }
  };

  const confirmAdminComplete = (tradeNo) => {
    Modal.confirm({
      title: t('确认补单'),
      content: t('是否将该订单标记为成功并为用户入账？'),
      onOk: () => handleAdminComplete(tradeNo),
    });
  };

  const getBizType = (record) => {
    if (record?.biz_type) return record.biz_type;
    const tradeNo = (record?.trade_no || '').toLowerCase();
    return Number(record?.amount || 0) === 0 && tradeNo.startsWith('sub')
      ? 'subscription'
      : 'payment';
  };

  const isSubscriptionTopup = (record) => getBizType(record) === 'subscription';

  const handleOpenDetail = (record) => {
    setSelectedRecord({
      ...record,
      biz_type: getBizType(record),
    });
    setDetailVisible(true);
  };

  const handleCopyDetailValue = async (value) => {
    if (!value || typeof navigator === 'undefined' || !navigator.clipboard?.writeText) {
      Toast.error({ content: t('复制失败') });
      return;
    }
    try {
      await navigator.clipboard.writeText(String(value));
      Toast.success({ content: t('复制成功') });
    } catch (error) {
      Toast.error({ content: t('复制失败') });
    }
  };

  const renderStatusBadge = (status, record) => {
      if (isInviteRebateTopup(record)) {
        return (
          <span className='inline-flex items-center justify-center gap-1 rounded-full bg-emerald-50 px-3 py-1 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-300'>
            <CheckCircle size={14} />
            <span className='font-medium'>{t('已入账')}</span>
          </span>
        );
      }
      if (!status) {
        return <Text type='tertiary'>-</Text>;
      }
      const config = STATUS_CONFIG[status] || { type: 'primary', key: status };
      return (
        <span className='flex items-center justify-center gap-2'>
          <Badge dot type={config.type} />
          <span>{t(config.key)}</span>
        </span>
      );
    };

    const renderPaymentMethod = (paymentMethod) => {
      const displayName = PAYMENT_METHOD_MAP[paymentMethod];
      return <Text>{displayName ? t(displayName) : paymentMethod || '-'}</Text>;
    };

    const renderBizTypeTag = (record) => {
      const config = getTopupBizTypeConfig(record);
      const inviteRebate = isInviteRebateTopup(record);
      return (
        <Tag color={config.color} shape='circle' size='small'>
          <span className='inline-flex items-center gap-1'>
            {inviteRebate ? <Gift size={12} /> : null}
            {t(config.label)}
          </span>
        </Tag>
      );
    };

    const periodOptionList = useMemo(
      () => BILL_PERIOD_OPTIONS.map((item) => ({ value: item.value, label: t(item.label) })),
      [t],
    );

    const summaryCards = useMemo(
      () => [
        {
          key: 'current_quota',
          title: t('支出/消费'),
          value: formatCurrency(billingSummary.expense),
          description: (
            <span className='inline-flex items-center gap-1'>
              <span className='text-slate-400'>{t('较上月')}</span>
              <span className={getPercentToneClassName(billingSummary.expense_trend)}>
                {formatPercent(billingSummary.expense_trend)}
              </span>
            </span>
          ),
          icon: BadgeDollarSign,
          iconClassName: 'text-slate-500',
          iconWrapClassName: 'bg-slate-100',
          valueClassName: 'text-slate-700',
        },
        {
          key: 'topup_amount',
          title: t('充值/本金'),
          value: formatCurrency(billingSummary.topup),
          description: t('实际支付充值的金额'),
          icon: CalendarCheck2,
          iconClassName: 'text-slate-500',
          iconWrapClassName: 'bg-slate-100',
          valueClassName: 'text-slate-700',
        },
        {
          key: 'redemption_amount',
          title: t('获赠/福利'),
          value: formatCurrency(billingSummary.bonus),
          description: t('获得的平台赠送或活动奖励'),
          icon: Coins,
          iconClassName: 'text-slate-500',
          iconWrapClassName: 'bg-slate-100',
          valueClassName: 'text-slate-700',
        },
        {
          key: 'net_change',
          title: t('净变动'),
          value: formatCurrency(billingSummary.net_change, { signed: true }),
          description: t('账户资金的净增减情况'),
          icon: BarChart3,
          iconClassName: 'text-slate-500',
          iconWrapClassName: 'bg-slate-100',
          valueClassName: 'text-slate-700',
        },
      ],
      [billingSummary, t],
    );

    const startIndex =
      historyTotal === 0 ? 0 : (activePage - 1) * historyPageSize + 1;
    const endIndex = Math.min(activePage * historyPageSize, historyTotal);
    const detailSections = useMemo(
      () => buildDetailSections(selectedRecord, t),
      [selectedRecord, t],
  
    );
    const hasInviteRebateRecords = useMemo(
      () => historyRows.some((record) => isInviteRebateTopup(record)),
      [historyRows],
    );
  const columns = useMemo(() => {
    const baseColumns = [
      {
        title: t('订单号'),
        dataIndex: 'trade_no',
        key: 'trade_no',
        render: (text) => <Text copyable>{text}</Text>,
      },
      {
        title: t('账单类型'),
        dataIndex: 'biz_type',
        key: 'biz_type',
        render: (_, record) => renderBizTypeTag(record),
      },
      {
        title: t('支付方式'),
        dataIndex: 'payment_method',
        key: 'payment_method',
        render: renderPaymentMethod,
      },
      {
        title: t('充值额度'),
        dataIndex: 'amount',
        key: 'amount',
        render: (_, record) =>
          isSubscriptionTopup(record) ? (
            <Tag color='purple' shape='circle' size='small'>
              {t('订阅套餐')}
            </Tag>
          ) : isInviteRebateTopup(record) ? (
            <span className='inline-flex items-center gap-1 rounded-full bg-emerald-50 px-3 py-1 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-300'>
              <Gift size={14} />
              <Text strong className='!text-emerald-600 dark:!text-emerald-300'>
                +{record.amount}
              </Text>
            </span>
          ) : (
            <span className='flex items-center gap-1'>
              <Coins size={16} />
              <Text>{record.amount}</Text>
            </span>
          ),
      },
      {
        title: t('支付金额'),
        dataIndex: 'money',
        key: 'money',
        render: (money, record) => {
          const normalizedMoney = Number(money || 0);
          if (normalizedMoney <= 0) {
            return <Text type="tertiary">-</Text>;
          }
          // 优先使用后端返回的币种符号，Stripe 默认 $，其他默认 ¥
          const paySymbol =
            record.display_symbol ||
            (record.payment_method === "stripe" ? "$" : "¥");
          return (
            <Text type="danger">
              {formatDisplayMoney(money, paySymbol)}
            </Text>
          );
        },
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        key: 'status',
        align: 'center',
        render: renderStatusBadge,
      },
      {
        title: t('创建时间'),
        dataIndex: 'create_time',
        key: 'create_time',
        render: (time) => (
          <span className='whitespace-nowrap text-slate-600'>
            {timestamp2string(time)}
          </span>
        ),
      },
      {
        title: t('操作'),
        key: 'action',
        align: 'left',
        render: (_, record) => (
          <div className='flex items-center justify-start gap-2'>
            {userIsAdmin && record.status === 'pending' ? (
              <Tooltip content={t('补单')}>
                <Button
                  size='small'
                  type='tertiary'
                  theme='borderless'
                  icon={<CheckCircle size={16} />}
                  onClick={() => confirmAdminComplete(record.trade_no)}
                  style={{ color: '#475569' }}
                />
              </Tooltip>
            ) : null}
            <Tooltip content={t('详情')}>
              <Button
                size='small'
                type='tertiary'
                theme='borderless'
                icon={<IconEyeOpened />}
                onClick={() => handleOpenDetail(record)}
                style={{ color: '#475569' }}
              />
            </Tooltip>
          </div>
        ),
      },
    ];

    if (userIsAdmin) {
      baseColumns.splice(1, 0, {
        title: t('用户昵称'),
        dataIndex: 'display_name',
        key: 'display_name',
        render: (text) => text || '-',
      });
    }

    return baseColumns;
  }, [t, userIsAdmin]);

  return (
    <div className='billing-page flex flex-col gap-4 pb-4'>
      <div className='billing-page__hero rounded-2xl bg-white px-6 py-5 dark:bg-slate-800'>
        <div className='flex flex-col gap-2 lg:flex-row lg:items-end lg:justify-between'>
          <div className='min-w-0'>
            <div className='flex items-center gap-3'>
              <div>
                <div className='text-[30px] font-medium text-[#475569] dark:text-slate-200'>
                  {billingPageTitle}
                </div>
                <div className='mt-2 text-[18px] font-medium text-[#94A3B8]'>
                  {billingPageDescription}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className='flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between'>
        <div className='inline-flex w-fit items-center rounded-2xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-800 p-1 shadow-sm'>
          <button
            type='button'
            className='rounded-xl bg-[#f3faf8] dark:bg-slate-400 px-6 py-3 text-sm font-semibold text-[#1f3b2d]'
          >
            {t('用户')}
          </button>
          <button
            type='button'
            disabled
            className='rounded-xl px-6 py-3 text-sm font-semibold text-slate-400 opacity-70'
          >
            {t('代理商')}
          </button>
        </div>

        <div className='flex w-full items-center justify-between dark:bg-slate-800 rounded-2xl border border-slate-200 bg-white px-4 py-3 dark:border-slate-700 shadow-sm sm:w-auto sm:gap-4'>
          <span className='text-sm font-medium text-slate-400'>{t('日期范围')}</span>
          <Select
            value={billingPeriod}
            optionList={periodOptionList}
            onChange={setBillingPeriod}
            loading={billingSummaryLoading}
            className='select-bg min-w-[120px]'
            size='small'
          />
        </div>
      </div>

      <div className='billing-page__stats grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-4'>
        {summaryCards.map((item) => {
          const Icon = item.icon;
          return (
            <Card
              key={item.key}
              bordered
              bodyStyle={{ padding: 0 }}
              className='billing-summary-card !rounded-[20px] transition-all duration-200 dark:!border-slate-700 dark:hover:!border-cyan-500/40'
            >
              <div
                className={`flex h-full flex-col justify-between gap-6 rounded-[20px] bg-white px-7 py-7 transition-opacity dark:bg-slate-800 ${billingSummaryLoading ? 'opacity-70' : 'opacity-100'}`}
              >
                <div className='flex items-start justify-between gap-6'>
                  <div className='min-w-0 text-[15px] font-semibold text-[#94A3B8]'>
                    {item.title}
                  </div>
                  <div
                    className={`icon-bg flex h-12 w-12 items-center justify-center dark:bg-slate-600 rounded-2xl ${item.iconWrapClassName}`}
                  >
                    <Icon size={22} className={`icon-color ${item.iconClassName}`} />
                  </div>
                </div>

                <div className='space-y-4'>
                  <div className={`text-val text-[24px] font-[900] md:text-[26px] ${item.valueClassName}`}>
                    {item.value}
                  </div>
                  <div className='text-sm text-slate-500'>{item.description}</div>
                </div>
              </div>
            </Card>
          );
        })}
      </div>

      <Card
        bodyStyle={{ padding: 0 }}
        bordered={false}
        className='billing-table-card !rounded-2xl'
      >
        <div className='flex items-center justify-between border-b border-slate-100 px-6 py-4'>
          <div className='flex flex-col gap-2 text-slate-800'>
            <span className='text-lg font-bold dark:text-slate-300'>
              {t('充值与返佣记录')}
            </span>
            <div className='flex flex-wrap items-center gap-2'>
              <Tag color='blue' shape='circle' size='small'>
                <span className='inline-flex items-center gap-1'>
                  <Wallet size={12} />
                  {t('在线充值')}
                </span>
              </Tag>
              <Tag color='green' shape='circle' size='small'>
                <span className='inline-flex items-center gap-1'>
                  <Gift size={12} />
                  {t('邀请返佣')}
                </span>
              </Tag>
              {hasInviteRebateRecords ? (
                <span className='text-xs text-emerald-600 dark:text-emerald-300'>
                  {t('邀请返佣已自动入账，无需手动处理。')}
                </span>
              ) : null}
            </div>
          </div>
          <div className='flex items-center gap-3'>
            <span className='text-xs text-slate-400'>
              {t('共 {{count}} 条记录', { count: historyTotal })}
            </span>
            <Input
              prefix={<IconSearch />}
              placeholder={t(
                userIsAdmin ? '搜索订单号或用户昵称' : '搜索订单号',
              )}
              value={historyKeyword}
              onChange={(value) => {
                setHistoryKeyword(value);
                setActivePage(1);
              }}
              showClear
              style={{ width: 220 }}
            />
          </div>
        </div>

        <Table
          className='billing-table'
          columns={columns}
          dataSource={historyRows}
          loading={historyLoading}
          rowKey='id'
          pagination={false}
          empty={
            <Empty
              image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
              }
              description={t('暂无账单记录')}
              style={{ padding: 30 }}
            />
          }
        />

        <div className='billing-pagination flex flex-col gap-3 pt-3 lg:flex-row lg:items-center lg:justify-between'>
          <Text type='tertiary'>
            {t('显示第 {{start}} - {{end}} 条，共 {{total}} 条', {
              start: startIndex,
              end: endIndex,
              total: historyTotal,
            })}
          </Text>
          <div className='flex items-center gap-3'>
            <Select
              value={historyPageSize}
              onChange={(value) => {
                setHistoryPageSize(value);
                setActivePage(1);
              }}
              optionList={HISTORY_PAGE_SIZE_OPTIONS.map((value) => ({
                label: t('{{count}} 条/页', { count: value }),
                value,
              }))}
              insetLabel={t('每页')}
              className='min-w-[120px]'
            />
            <Pagination
              total={historyTotal}
              pageSize={historyPageSize}
              currentPage={activePage}
              onPageChange={setActivePage}
              showSizeChanger={false}
            />
          </div>
        </div>
      </Card>

      <Modal
        title={
          <span className='text-[18px] font-[700] text-[#334155]'>{t('详情')}</span>
        }
        visible={detailVisible}
        onCancel={() => setDetailVisible(false)}
        width={isMobile ? undefined : DETAIL_MODAL_WIDTH}
        size={isMobile ? 'full-width' : undefined}
        closeOnEsc
        footer={
          <div className='flex items-center justify-end gap-4'>
            <Button
              theme='light'
              type='tertiary'
              onClick={() => setDetailVisible(false)}
              style={{
                minWidth: 78,
                height: 40,
                borderRadius: 10,
                color: '#475569',
                background: '#F8FAFC',
              }}
            >
              {t('取消')}
            </Button>
            <Button
              onClick={() => setDetailVisible(false)}
              style={{
                minWidth: 78,
                height: 40,
                borderRadius: 10,
                border: 'none',
                color: '#0F172A',
                background:
                  'linear-gradient(90deg, rgba(30, 235, 211, 1) 0%, rgba(176, 255, 45, 1) 100%)',
              }}
            >
              {t('确定')}
            </Button>
          </div>
        }
        centered={!isMobile}
        bodyStyle={{
          padding: isMobile ? '4px 16px 0' : '4px 0',
          height: isMobile ? 'calc(100vh - 220px)' : DETAIL_MODAL_CONTENT_HEIGHT,
          overflow: 'hidden',
        }}
      >
        <div
          className='border-t border-[#E9EEF5]'
          style={{
            height: '100%',
            overflowY: 'auto',
            overflowX: 'hidden',
            paddingTop: 20,
            paddingRight: 6,
          }}
        >
          <div className='pb-7'>
            <div className='mb-4 text-[16px] font-[700] text-[#475569]'>
              {t('主订单信息')}
            </div>
            <div className='space-y-2'>
              {detailSections.mainInfo.map((item) => (
                <DetailField
                  key={item.label}
                  label={item.label}
                  value={item.value}
                  copyValue={item.copyValue}
                  onCopy={handleCopyDetailValue}
                />
              ))}
            </div>
          </div>

          <div className='border-t border-[#E9EEF5] py-7'>
            <div className='mb-4 text-[16px] font-[700] text-[#475569]'>
              {t('分佣明细')}
            </div>

            <div className='space-y-7'>
              {detailSections.commissionGroups.map((group) => (
                <div key={group.title}>
                  <div className='mb-3 text-sm font-semibold text-[#94A3B8]'>
                    {group.title}
                  </div>
                  <div className='space-y-2'>
                    {group.items.map((item) => (
                      <DetailField
                        key={`${group.title}-${item.label}`}
                        label={item.label}
                        value={item.value}
                        copyValue={item.copyValue}
                        onCopy={handleCopyDetailValue}
                      />
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default Billing;
