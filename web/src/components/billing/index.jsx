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
  Typography,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { useTranslation } from 'react-i18next';
import { BarChart3, BadgeDollarSign, CalendarCheck2, Coins } from 'lucide-react';
import { IconSearch } from '@douyinfe/semi-icons';
import { API, timestamp2string, formatDisplayMoney } from '../../helpers';
import { isAdmin } from '../../helpers/utils';

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

const Billing = () => {
  const { t } = useTranslation();
  const [billingPeriod, setBillingPeriod] = useState('day');
  const [billingSummary, setBillingSummary] = useState(BILLING_SUMMARY_DEFAULTS);
  const [billingSummaryLoading, setBillingSummaryLoading] = useState(false);

  const [activePage, setActivePage] = useState(1);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyRows, setHistoryRows] = useState([]);
  const [historyTotal, setHistoryTotal] = useState(0);
  const [historyPageSize, setHistoryPageSize] = useState(10);
  const [historyKeyword, setHistoryKeyword] = useState('');

  const userIsAdmin = useMemo(() => isAdmin(), []);

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

  const renderStatusBadge = (status) => {
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

  const columns = useMemo(() => {
    const baseColumns = [
      {
        title: t('订单号'),
        dataIndex: 'trade_no',
        key: 'trade_no',
        render: (text) => <Text copyable>{text}</Text>,
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
        render: (money, record) => (
          <Text type='danger'>
            {formatDisplayMoney(
              money,
              // 优先使用后端返回的币种符号，Stripe 默认 $，其他默认 ¥
              record.display_symbol ||
                (record.payment_method === 'stripe' ? '$' : '\u00A5'),
            )}
          </Text>
        ),
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
    ];

    if (userIsAdmin) {
      baseColumns.splice(1, 0, {
        title: t('用户昵称'),
        dataIndex: 'display_name',
        key: 'display_name',
        render: (text) => text || '-',
      });
      baseColumns.push({
        title: t('操作'),
        key: 'action',
        align: 'center',
        render: (_, record) =>
          record.status === 'pending' ? (
            <Button
              size='small'
              type='primary'
              theme='outline'
              onClick={() => confirmAdminComplete(record.trade_no)}
            >
              {t('补单')}
            </Button>
          ) : null,
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
                  {t('账单中心')}
                </div>
                <div className='mt-2 text-[18px] font-medium text-[#94A3B8]'>
                  {t('实时监控您的 API 消耗、Token 使用明细以及账户余额。基于矩阵算力引擎提供精确到毫秒级的计费服务。')}
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
          <div className='flex items-center gap-2 text-slate-800'>
            <span className='text-lg font-bold dark:text-slate-300'>{t('消费明细')}</span>
          </div>
          <div className='flex items-center gap-3'>
            <span className='text-xs text-slate-400'>
              {t('共 {{count}} 条记录', { count: historyTotal })}
            </span>
            <Input
              prefix={<IconSearch />}
              placeholder={t('搜索订单号')}
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
              description={t('暂无充值记录')}
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
    </div>
  );
};

export default Billing;
