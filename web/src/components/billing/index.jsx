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

import React, { useMemo, useState } from 'react';
import { Button, Card, Pagination, Select, Table, Tag, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import {
  BarChart3,
  BadgeDollarSign,
  CalendarCheck2,
  Clock3,
  Coins,
  Download,
  List,
  ReceiptText,
} from 'lucide-react';

const { Text, Title } = Typography;

const PAGE_SIZE = 6;

const SUMMARY_CARDS = [
  {
    key: 'monthCost',
    title: '本月消费',
    value: '$128.50',
    description: '较上月 +12.8%',
    icon: BadgeDollarSign,
    iconClassName: 'text-cyan-600',
    iconWrapClassName: 'bg-cyan-50',
  },
  {
    key: 'settled',
    title: '已结算',
    value: '$96.30',
    description: '2026 年 3 月',
    icon: CalendarCheck2,
    iconClassName: 'text-violet-600',
    iconWrapClassName: 'bg-violet-50',
  },
  {
    key: 'totalCost',
    title: '累计消费',
    value: '$1,245.80',
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

const BILLING_ROWS = [
  {
    key: '1',
    time: '2026-03-18 09:42',
    model: 'GPT-4-Turbo',
    provider: 'OpenAI 官方',
    inputTokens: 2340,
    outputTokens: 1856,
    cost: '$0.0580',
    status: '已结算',
    colorClassName: 'bg-emerald-400',
  },
  {
    key: '2',
    time: '2026-03-18 08:15',
    model: 'Claude 3.5 Sonnet',
    provider: 'Anthropic',
    inputTokens: 5120,
    outputTokens: 3472,
    cost: '$0.0412',
    status: '已结算',
    colorClassName: 'bg-orange-400',
  },
  {
    key: '3',
    time: '2026-03-17 22:30',
    model: 'GPT-4o',
    provider: 'OpenAI 官方',
    inputTokens: 8192,
    outputTokens: 2048,
    cost: '$0.0310',
    status: '已结算',
    colorClassName: 'bg-blue-400',
  },
  {
    key: '4',
    time: '2026-03-17 18:05',
    model: 'Gemini 1.5 Pro',
    provider: 'Google AI',
    inputTokens: 12800,
    outputTokens: 4096,
    cost: '$0.0720',
    status: '已结算',
    colorClassName: 'bg-violet-400',
  },
  {
    key: '5',
    time: '2026-03-17 14:52',
    model: 'DeepSeek-V3',
    provider: 'DeepSeek',
    inputTokens: 6400,
    outputTokens: 5120,
    cost: '$0.0086',
    status: '待结算',
    colorClassName: 'bg-cyan-400',
  },
  {
    key: '6',
    time: '2026-03-17 10:28',
    model: 'Claude 3 Opus',
    provider: 'Anthropic',
    inputTokens: 3200,
    outputTokens: 2816,
    cost: '$0.1320',
    status: '已结算',
    colorClassName: 'bg-orange-400',
  },
  {
    key: '7',
    time: '2026-03-16 21:10',
    model: 'GPT-4.1',
    provider: 'OpenAI 官方',
    inputTokens: 4890,
    outputTokens: 2604,
    cost: '$0.0440',
    status: '已结算',
    colorClassName: 'bg-sky-400',
  },
  {
    key: '8',
    time: '2026-03-16 14:36',
    model: 'Gemini 2.0 Flash',
    provider: 'Google AI',
    inputTokens: 2760,
    outputTokens: 1320,
    cost: '$0.0094',
    status: '已结算',
    colorClassName: 'bg-indigo-400',
  },
  {
    key: '9',
    time: '2026-03-16 09:12',
    model: 'DeepSeek-R1',
    provider: 'DeepSeek',
    inputTokens: 7140,
    outputTokens: 6540,
    cost: '$0.0158',
    status: '待结算',
    colorClassName: 'bg-teal-400',
  },
  {
    key: '10',
    time: '2026-03-15 19:44',
    model: 'Claude 3.7 Sonnet',
    provider: 'Anthropic',
    inputTokens: 5900,
    outputTokens: 4010,
    cost: '$0.0575',
    status: '已结算',
    colorClassName: 'bg-amber-400',
  },
  {
    key: '11',
    time: '2026-03-15 11:08',
    model: 'GPT-4o-mini',
    provider: 'OpenAI 官方',
    inputTokens: 9320,
    outputTokens: 4880,
    cost: '$0.0186',
    status: '已结算',
    colorClassName: 'bg-emerald-400',
  },
  {
    key: '12',
    time: '2026-03-15 08:23',
    model: 'Gemini 1.5 Pro',
    provider: 'Google AI',
    inputTokens: 4250,
    outputTokens: 2380,
    cost: '$0.0284',
    status: '已结算',
    colorClassName: 'bg-purple-400',
  },
];

const DAILY_COSTS = [
  { label: '3月2日', amount: '$15.20', percent: 60 },
  { label: '3月3日', amount: '$22.80', percent: 85 },
  { label: '3月4日', amount: '$8.50', percent: 35 },
  { label: '3月5日', amount: '$26.40', percent: 100 },
  { label: '3月6日', amount: '$19.70', percent: 75 },
  { label: '3月7日', amount: '$24.36', percent: 92 },
  { label: '今天', amount: '$11.54', percent: 45, highlight: true },
];

const TIME_RANGE_OPTIONS = [
  { value: 'current_month', label: '本月' },
  { value: 'last_month', label: '上月' },
  { value: 'last_three_months', label: '近三月' },
  { value: 'custom', label: '自定义' },
];

const MODEL_OPTIONS = [
  { value: 'all', label: '全部模型' },
  { value: 'GPT-4-Turbo', label: 'GPT-4-Turbo' },
  { value: 'GPT-4o', label: 'GPT-4o' },
  { value: 'Claude 3.5 Sonnet', label: 'Claude 3.5 Sonnet' },
  { value: 'Claude 3 Opus', label: 'Claude 3 Opus' },
  { value: 'Claude 3.7 Sonnet', label: 'Claude 3.7 Sonnet' },
  { value: 'Gemini 1.5 Pro', label: 'Gemini 1.5 Pro' },
  { value: 'Gemini 2.0 Flash', label: 'Gemini 2.0 Flash' },
  { value: 'DeepSeek-V3', label: 'DeepSeek-V3' },
  { value: 'DeepSeek-R1', label: 'DeepSeek-R1' },
  { value: 'GPT-4.1', label: 'GPT-4.1' },
  { value: 'GPT-4o-mini', label: 'GPT-4o-mini' },
];

const formatNumber = (value) => value.toLocaleString('en-US');

const Billing = () => {
  const { t } = useTranslation();
  const [activeRange, setActiveRange] = useState('current_month');
  const [selectedModel, setSelectedModel] = useState('all');
  const [activePage, setActivePage] = useState(1);

  const filteredRows = useMemo(() => {
    if (selectedModel === 'all') {
      return BILLING_ROWS;
    }
    return BILLING_ROWS.filter((row) => row.model === selectedModel);
  }, [selectedModel]);

  const pagedRows = useMemo(() => {
    const start = (activePage - 1) * PAGE_SIZE;
    return filteredRows.slice(start, start + PAGE_SIZE);
  }, [activePage, filteredRows]);

  const totalPages = Math.max(1, Math.ceil(filteredRows.length / PAGE_SIZE));
  const startIndex = filteredRows.length === 0 ? 0 : (activePage - 1) * PAGE_SIZE + 1;
  const endIndex = Math.min(activePage * PAGE_SIZE, filteredRows.length);

  const columns = [
    {
      title: t('时间'),
      dataIndex: 'time',
      render: (text) => <span className='whitespace-nowrap text-slate-600'>{text}</span>,
    },
    {
      title: t('模型名称'),
      dataIndex: 'model',
      render: (_, record) => (
        <div className='inline-flex items-center gap-2'>
          <span className={`h-2.5 w-2.5 rounded-full ${record.colorClassName}`} />
          <span className='font-medium text-slate-800'>{record.model}</span>
        </div>
      ),
    },
    {
      title: t('渠道'),
      dataIndex: 'provider',
      render: (text) => <span className='text-slate-500'>{text}</span>,
    },
    {
      title: t('输入 Tokens'),
      dataIndex: 'inputTokens',
      align: 'right',
      render: (value) => <span className='font-mono text-slate-600'>{formatNumber(value)}</span>,
    },
    {
      title: t('输出 Tokens'),
      dataIndex: 'outputTokens',
      align: 'right',
      render: (value) => <span className='font-mono text-slate-600'>{formatNumber(value)}</span>,
    },
    {
      title: t('费用'),
      dataIndex: 'cost',
      align: 'right',
      render: (value) => <span className='font-mono font-semibold text-slate-800'>{value}</span>,
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      align: 'center',
      render: (status) => (
        <Tag
          color={status === '已结算' ? 'green' : 'yellow'}
          shape='circle'
          size='small'
          className='billing-status-tag'
        >
          {status}
        </Tag>
      ),
    },
  ];

  const handleRangeChange = (value) => {
    setActiveRange(value);
    setActivePage(1);
  };

  const handleModelChange = (value) => {
    setSelectedModel(value);
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
          <div className='billing-page__hero-note rounded-xl border border-cyan-100 bg-cyan-50 px-4 py-3 text-sm text-cyan-700'>
            {t('当前展示为静态示例数据，后续可直接替换为接口返回结果。')}
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
                  <div className='mb-3 text-sm font-medium text-slate-500'>{t(item.title)}</div>
                  <div className='text-3xl font-bold leading-none text-slate-900'>{item.value}</div>
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
                <div className={`flex h-11 w-11 items-center justify-center rounded-xl ${item.iconWrapClassName}`}>
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
            <span className='text-sm font-medium text-slate-600'>{t('时间范围：')}</span>
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
              value={selectedModel}
              onChange={handleModelChange}
              optionList={MODEL_OPTIONS.map((item) => ({
                label: item.label,
                value: item.value,
              }))}
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
            <List size={18} className='text-cyan-500' />
            <span className='text-lg font-bold'>{t('消费明细')}</span>
          </div>
          <span className='text-xs text-slate-400'>
            {t('共 {{count}} 条记录', { count: filteredRows.length })}
          </span>
        </div>
        <Table
          className='billing-table'
          columns={columns}
          dataSource={pagedRows}
          pagination={false}
          empty={t('暂无账单记录')}
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
          {DAILY_COSTS.map((item) => (
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
                  style={{ height: `${item.percent}%`, marginTop: `${100 - item.percent}%` }}
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
            total: filteredRows.length,
          })}
        </Text>
        <Pagination
          total={filteredRows.length}
          pageSize={PAGE_SIZE}
          currentPage={activePage}
          onPageChange={setActivePage}
          showSizeChanger={false}
        />
      </div>
    </div>
  );
};

export default Billing;

