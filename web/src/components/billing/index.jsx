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
import {
  BarChart3,
  BadgeDollarSign,
  CalendarCheck2,
  Clock3,
  Coins,
  Download,
  History,
  ReceiptText,
} from 'lucide-react';
import { IconSearch } from '@douyinfe/semi-icons';
import { API, timestamp2string } from '../../helpers';
import { isAdmin } from '../../helpers/utils';

const { Text, Title } = Typography;

const SUMMARY_CARDS = [
  {
    key: 'monthCost',
    title: '本月消费',
    value: '$0.00',
    description: '较上月 +0%',
    icon: BadgeDollarSign,
    iconClassName: 'text-cyan-600',
    iconWrapClassName: 'bg-cyan-50',
  },
  {
    key: 'settled',
    title: '已结算',
    value: '$0.00',
    description: '2026 年 3 月',
    icon: CalendarCheck2,
    iconClassName: 'text-violet-600',
    iconWrapClassName: 'bg-violet-50',
  },
  {
    key: 'totalCost',
    title: '累计消费',
    value: '$0.00',
    description: '自 2025-06 至今',
    icon: Coins,
    iconClassName: 'text-amber-600',
    iconWrapClassName: 'bg-amber-50',
  },
  {
    key: 'pending',
    title: '待结算',
    value: '$0.00',
    description: '已全部结清',
    icon: Clock3,
    iconClassName: 'text-emerald-600',
    iconWrapClassName: 'bg-emerald-50',
  },
];

const DAILY_COST_VALUES = [
  { amount: '$15.20', percent: 60 },
  { amount: '$22.80', percent: 85 },
  { amount: '$8.50', percent: 35 },
  { amount: '$26.40', percent: 100 },
  { amount: '$19.70', percent: 75 },
  { amount: '$24.36', percent: 92 },
  { amount: '$11.54', percent: 45 },
];

const TIME_RANGE_OPTIONS = [
  { value: 'current_month', label: '本月' },
  { value: 'last_month', label: '上月' },
  { value: 'last_three_months', label: '近三个月' },
  { value: 'custom', label: '自定义' },
];

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

const Billing = () => {
  const { t } = useTranslation();
  const [activeRange, setActiveRange] = useState('current_month');
  const [selectedStatus, setSelectedStatus] = useState('all');
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

  const statusOptions = useMemo(
    () => [
      { value: 'all', label: t('全部状态') },
      { value: 'success', label: t('成功') },
      { value: 'pending', label: t('待支付') },
      { value: 'failed', label: t('失败') },
      { value: 'expired', label: t('已过期') },
    ],
    [t],
  );

  const filteredRows = useMemo(() => {
    if (selectedStatus === 'all') {
      return historyRows;
    }
    return historyRows.filter((row) => row.status === selectedStatus);
  }, [historyRows, selectedStatus]);

  const dailyCosts = useMemo(() => {
    const today = new Date();
    return DAILY_COST_VALUES.map((item, index) => {
      const date = new Date(today);
      date.setDate(today.getDate() - (DAILY_COST_VALUES.length - 1 - index));
      const isToday = index === DAILY_COST_VALUES.length - 1;
      return {
        ...item,
        label: isToday ? t('今天') : `${date.getMonth() + 1}月${date.getDate()}日`,
        highlight: isToday,
      };
    });
  }, [t]);

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
        render: (money, record) => {
          const normalizedMoney = Number(money || 0);
          const prefix =
            normalizedMoney <= 0
              ? ''
              : record.payment_method === 'stripe'
                ? '$'
                : '￥';
          return (
            <Text type='danger'>
              {prefix}
              {normalizedMoney.toFixed(2)}
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

  const handleRangeChange = (value) => {
    setActiveRange(value);
    setActivePage(1);
  };

  const handleStatusChange = (value) => {
    setSelectedStatus(value);
    setActivePage(1);
  };

  return (
    <div className='billing-page flex flex-col gap-4 pb-4'>
      <div className='billing-page__hero rounded-2xl border border-slate-200 bg-white px-6 py-5 shadow-sm'>
        <div className='flex flex-col gap-2 lg:flex-row lg:items-end lg:justify-between'>
          <div className='min-w-0'>
            <div className='flex items-center gap-3'>
              <div className='flex h-11 w-11 items-center justify-center rounded-2xl bg-cyan-50 text-cyan-600'>
                <ReceiptText size={22} />
              </div>
              <div>
                <Title heading={3} style={{ margin: 0 }}>
                  {t('账单中心')}
                </Title>
                <Text type='tertiary'>{t('查看消费记录与账单明细')}</Text>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className='billing-page__stats grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4'>
        {SUMMARY_CARDS.map((item) => {
          const Icon = item.icon;
          return (
            <Card
              key={item.key}
              bodyStyle={{ padding: 20 }}
              className='billing-summary-card !rounded-2xl border border-slate-200 shadow-sm'
            >
              <div className='flex items-start justify-between gap-4'>
                <div className='min-w-0'>
                  <div className='mb-3 text-sm font-medium text-slate-500'>
                    {t(item.title)}
                  </div>
                  <div className='text-3xl font-bold leading-none text-slate-900'>
                    {item.value}
                  </div>
                  <div className='mt-3 text-xs text-slate-400'>
                    {item.key === 'pending' ? (
                      <span className='inline-flex items-center gap-1 text-emerald-500'>
                        <Clock3 size={14} />
                        {t(item.description)}
                      </span>
                    ) : (
                      t(item.description)
                    )}
                  </div>
                </div>
                <div
                  className={`flex h-11 w-11 items-center justify-center rounded-xl ${item.iconWrapClassName}`}
                >
                  <Icon size={20} className={item.iconClassName} />
                </div>
              </div>
            </Card>
          );
        })}
      </div>

      <Card
        bodyStyle={{ padding: 16 }}
        className='billing-filter-card !rounded-2xl border border-slate-200 shadow-sm'
      >
        <div className='flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
          <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
            <span className='text-sm font-medium text-slate-600'>
              {t('时间范围')}
            </span>
            <div className='billing-filter-card__range inline-flex flex-wrap rounded-xl border border-slate-200 bg-slate-50 p-1'>
              {TIME_RANGE_OPTIONS.map((option) => {
                const active = activeRange === option.value;
                return (
                  <Button
                    key={option.value}
                    theme='borderless'
                    type='primary'
                    className={`billing-range-button !rounded-lg !px-4 !py-2 !text-sm ${
                      active
                        ? '!bg-cyan-600 !text-white hover:!bg-cyan-600'
                        : '!text-slate-600 hover:!bg-white hover:!text-slate-900'
                    }`}
                    onClick={() => handleRangeChange(option.value)}
                  >
                    {t(option.label)}
                  </Button>
                );
              })}
            </div>
          </div>

          <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
            <Select
              value={selectedStatus}
              onChange={handleStatusChange}
              optionList={statusOptions}
              className='billing-filter-card__select min-w-[220px]'
            />
            <Button
              icon={<Download size={16} />}
              className='billing-filter-card__export !rounded-xl'
              theme='light'
              type='tertiary'
            >
              {t('导出账单')}
            </Button>
          </div>
        </div>
      </Card>

      <Card
        bodyStyle={{ padding: 0 }}
        className='billing-table-card !rounded-2xl border border-slate-200 shadow-sm'
      >
        <div className='flex items-center justify-between border-b border-slate-100 px-6 py-4'>
          <div className='flex items-center gap-2 text-slate-800'>
            <History size={18} className='text-cyan-500' />
            <span className='text-lg font-bold'>{t('消费明细')}</span>
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
          dataSource={filteredRows}
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
      </Card>

      <Card
        bodyStyle={{ padding: 24 }}
        className='billing-daily-card !rounded-2xl border border-slate-200 shadow-sm'
      >
        <div className='mb-5 flex items-center justify-between'>
          <div className='flex items-center gap-2 text-slate-800'>
            <BarChart3 size={18} className='text-cyan-500' />
            <span className='text-lg font-bold'>{t('按日消费汇总')}</span>
          </div>
          <span className='text-xs text-slate-400'>2026 年 3 月</span>
        </div>
        <div className='grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-7'>
          {dailyCosts.map((item) => (
            <div
              key={item.label}
              className={`billing-daily-card__item rounded-xl border p-3 text-center ${
                item.highlight
                  ? 'border-cyan-200 bg-cyan-50'
                  : 'border-slate-100 bg-slate-50'
              }`}
            >
              <div
                className={`mb-1 text-xs ${
                  item.highlight
                    ? 'font-medium text-cyan-600'
                    : 'text-slate-400'
                }`}
              >
                {item.label}
              </div>
              <div
                className={`text-sm font-bold ${
                  item.highlight ? 'text-cyan-700' : 'text-slate-800'
                }`}
              >
                {item.amount}
              </div>
              <div className='mt-3 h-10 overflow-hidden rounded-sm bg-cyan-100'>
                <div
                  className={`billing-daily-card__bar mt-auto w-full rounded-sm ${
                    item.highlight ? 'bg-cyan-600' : 'bg-cyan-500'
                  }`}
                  style={{
                    height: `${item.percent}%`,
                    marginTop: `${100 - item.percent}%`,
                  }}
                />
              </div>
            </div>
          ))}
        </div>
      </Card>

      <div className='billing-pagination flex flex-col gap-3 pt-1 lg:flex-row lg:items-center lg:justify-between'>
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
    </div>
  );
};

export default Billing;
