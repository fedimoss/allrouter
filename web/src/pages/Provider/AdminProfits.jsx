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

import React, { useContext, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Descriptions,
  Empty,
  Form,
  Modal,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { Calculator, CalendarClock, Coins, RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  API,
  showError,
  timestamp2string,
  userRawQuotaToDisplay,
} from '../../helpers';
import { UserContext } from '../../context/User';
import { createCardProPagination } from '../../helpers/utils';
import CardPro from '../../components/common/ui/CardPro';
import { DATE_RANGE_PRESETS } from '../../constants/console.constants';
import '../Log/log-v2.css';
import './provider-profits.css';

const { Text, Title } = Typography;

// 默认查询当天（与日志页一致）：本地当天零点起，至当前时间 +1 小时
const getCurrentDayRange = () => {
  const now = new Date();
  const dayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  return [
    timestamp2string(dayStart.getTime() / 1000),
    timestamp2string(now.getTime() / 1000 + 3600),
  ];
};

const AdminProviderProfitsPage = () => {
  const { t } = useTranslation();
  const [userState] = useContext(UserContext);
  const user = userState?.user;
  const [formApi, setFormApi] = useState(null);
  const [items, setItems] = useState([]);
  const [summary, setSummary] = useState({});
  const [loading, setLoading] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [total, setTotal] = useState(0);
  const [formulaVisible, setFormulaVisible] = useState(false);
  const [expandedRowKeys, setExpandedRowKeys] = useState([]);
  const [detailItems, setDetailItems] = useState({});
  const [detailLoaded, setDetailLoaded] = useState({});
  const [detailLoading, setDetailLoading] = useState({});
  const formInitValues = useMemo(
    () => ({ dateRange: getCurrentDayRange() }),
    [],
  );

  const getQuery = () => {
    const values = formApi?.getValues() || formInitValues;
    const range = Array.isArray(values.dateRange)
      ? values.dateRange
      : formInitValues.dateRange;
    return {
      start: Date.parse(range[0]) / 1000,
      end: Date.parse(range[1]) / 1000,
    };
  };

  const pageSize = 10;

  const loadOverview = async (page = activePage) => {
    setLoading(true);
    const query = getQuery();
    try {
      const res = await API.get(
        `/api/provider/admin/profits?p=${page}&page_size=${pageSize}&start_timestamp=${query.start}&end_timestamp=${query.end}`,
      );
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      const pageData = res.data.data?.page || {};
      setItems(pageData.items || []);
      setSummary(res.data.data?.summary || {});
      setActivePage(pageData.page || page);
      setTotal(pageData.total || 0);
      setExpandedRowKeys([]);
      setDetailItems({});
      setDetailLoaded({});
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (formApi) loadOverview(1);
  }, [formApi]);

  const paginationArea = createCardProPagination({
    currentPage: activePage,
    pageSize,
    total,
    onPageChange: (page) => loadOverview(page),
    t,
  });

  const loadProviderDetails = async (record) => {
    const providerId = Number(record?.provider_id || 0);
    if (!providerId || detailItems[providerId]) return;

    setDetailLoading((current) => ({ ...current, [providerId]: true }));
    const query = getQuery();
    try {
      const pageSize = 100;
      let page = 1;
      let total = 0;
      const records = [];
      do {
        const res = await API.get(
          `/api/provider/admin/${providerId}/profits?p=${page}&page_size=${pageSize}&start_timestamp=${query.start}&end_timestamp=${query.end}`,
        );
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        const pageData = res.data.data?.page || {};
        records.push(...(pageData.items || []));
        total = Number(pageData.total || records.length);
        page += 1;
      } while (records.length < total);
      setDetailItems((current) => ({ ...current, [providerId]: records }));
      setDetailLoaded((current) => ({ ...current, [providerId]: true }));
    } catch (error) {
      showError(error);
    } finally {
      setDetailLoading((current) => ({ ...current, [providerId]: false }));
    }
  };

  const quotaPerUnit =
    parseFloat(localStorage.getItem('quota_per_unit')) || 500000;
  const displaySymbol = user?.display_symbol || '$';
  const displayRate = Number(user?.display_rate) || 1;
  const quotaNumber = (value) =>
    ((Number(value) || 0) / quotaPerUnit) * displayRate;
  const quotaText = (value) =>
    `${displaySymbol}${quotaNumber(value).toFixed(6)}`;
  const renderProfit = (value) => (
    <Text strong type={Number(value) >= 0 ? 'success' : 'danger'}>
      {quotaText(value)}
    </Text>
  );
  const columns = [
    {
      title: t('服务商'),
      dataIndex: 'provider_name',
      width: 260,
      render: (value, record) => (
        <span>
          <strong>{value || '-'}</strong>{' '}
          <Tag shape='circle'>#{record.provider_id}</Tag>
        </span>
      ),
    },
    {
      title: t('收入'),
      dataIndex: 'income_quota',
      width: 180,
      render: quotaText,
    },
    {
      title: t('支出'),
      dataIndex: 'expense_quota',
      width: 180,
      render: quotaText,
    },
    {
      title: t('利润'),
      dataIndex: 'profit_quota',
      width: 180,
      render: renderProfit,
    },
  ];

  const getDetailDescriptions = (record, providerRecord) => {
    const query = getQuery();
    const userCharge = Number(record?.provider_user_quota || 0);
    const baseCost = Number(record?.base_cost_quota || 0);
    const paidQuota = Number(record?.paid_quota || 0);
    const coveredCost = Number(record?.covered_cost_quota || 0);
    const ownerCost = Number(record?.owner_cost_quota || 0);
    const rebate = Number(record?.rebate_quota || 0);
    const netProfit = Number(record?.profit_quota || 0);
    const grossProfit = Number(
      record?.gross_profit_quota || netProfit + rebate,
    );
    const paidRatio = userCharge > 0 ? (paidQuota / userCharge) * 100 : 0;
    const rewardCredit = Math.max(userCharge - paidQuota, 0);
    const amount = (value) => quotaText(value);
    // 与服务商"利润明细"弹窗一致的语义色：利润=绿，成本/支出=橙
    const money = (value, tone) => (
      <span
        style={{
          color:
            tone === 'cost'
              ? 'var(--semi-color-warning)'
              : tone === 'profit'
                ? 'var(--semi-color-success)'
                : 'var(--semi-color-text-0)',
          fontWeight: 700,
        }}
      >
        {amount(value)}
      </span>
    );

    return [
      {
        key: t('服务商'),
        value: providerRecord?.provider_name || record?.provider_name || '-',
      },
      {
        key: t('时间'),
        value: `${timestamp2string(query.start)} ~ ${timestamp2string(query.end)}`,
      },
      { key: t('用户收费'), value: amount(userCharge) },
      { key: t('基础成本'), value: amount(baseCost) },
      { key: t('充值支付'), value: amount(paidQuota) },
      { key: t('奖励/赠送抵扣'), value: amount(rewardCredit) },
      { key: t('用户覆盖成本'), value: amount(coveredCost) },
      { key: t('服务商承担'), value: amount(ownerCost) },
      { key: t('毛利润'), value: renderProfit(grossProfit) },
      { key: t('分佣'), value: amount(rebate) },
      { key: t('净利润'), value: renderProfit(netProfit) },
      {
        key: t('计算过程'),
        value: (
          <article>
            <p>
              {t('币种换算')} = {t('内部额度')} ÷ {quotaPerUnit} ×{' '}
              {displayRate}（{t('先换算后计算')}）
            </p>
            <p>
              {t('充值支付占比')} = {t('充值支付')} ÷ {t('用户收费')} ={' '}
              {money(paidQuota, 'profit')} ÷ {money(userCharge)} ={' '}
              {paidRatio.toFixed(2)}%
            </p>
            <p>
              {t('用户覆盖成本')} = {t('基础成本')} × {t('充值支付占比')} ={' '}
              {money(baseCost, 'cost')} × {paidRatio.toFixed(2)}% ={' '}
              {money(coveredCost, 'cost')}
            </p>
            <p>
              {t('服务商承担')} = {t('基础成本')} - {t('用户覆盖成本')} ={' '}
              {money(baseCost, 'cost')} - {money(coveredCost, 'cost')} ={' '}
              {money(ownerCost, 'cost')}
            </p>
            <p>
              {t('毛利润')} = ({t('用户收费')} - {t('基础成本')}) ×{' '}
              {t('充值支付占比')} = ({money(userCharge)} -{' '}
              {money(baseCost, 'cost')}) × {paidRatio.toFixed(2)}% ={' '}
              {money(grossProfit, 'profit')}
            </p>
            <p>
              {t('净利润')} = {t('毛利润')} - {t('分佣')} ={' '}
              {money(grossProfit, 'profit')} - {money(rebate)} ={' '}
              {money(netProfit, 'profit')}
            </p>
            <p>
              {t('收入')} = {t('净利润')}：{money(netProfit, 'profit')}
            </p>
            <p>
              {t('支出')} = {t('服务商承担')}：{money(ownerCost, 'cost')}
            </p>
            <p>
              {t('利润')} = {t('收入')} - {t('支出')} ={' '}
              {money(netProfit, 'profit')} - {money(ownerCost, 'cost')} ={' '}
              {money(netProfit - ownerCost, 'profit')}
            </p>
          </article>
        ),
      },
    ];
  };

  const expandedRowRender = (record) => {
    const providerId = Number(record.provider_id || 0);
    const rows = detailItems[providerId] || [];
    const hasLoaded = Boolean(detailLoaded[providerId]);
    const displayRows =
      rows.length > 0
        ? rows
        : hasLoaded
          ? [
              {
                id: `empty-${providerId}`,
                created_at: 0,
                provider_user_quota: 0,
                base_cost_quota: 0,
                paid_quota: 0,
                covered_cost_quota: 0,
                owner_cost_quota: 0,
                gross_profit_quota: 0,
                rebate_quota: 0,
                profit_quota: 0,
              },
            ]
          : [];
    return (
      <Spin spinning={Boolean(detailLoading[providerId])}>
        <div className='space-y-4'>
          {displayRows.map((detail) => (
            <Descriptions
              key={detail.id}
              data={getDetailDescriptions(detail, record)}
            />
          ))}
        </div>
      </Spin>
    );
  };

  const statCards = [
    { key: 'income_quota', label: t('收入') },
    { key: 'expense_quota', label: t('支出') },
    { key: 'profit_quota', label: t('利润') },
  ];

  const formulaRows = [
    {
      field: t('用户收费'),
      meaning: t('服务商用户看到并支付的本次消费金额。'),
      formula: t('服务商模型售价'),
      sample: '100',
    },
    {
      field: t('基础成本'),
      meaning: t('本次调用主站真实模型产生的成本。'),
      formula: t('主站模型原价'),
      sample: '60',
    },
    {
      field: t('充值支付'),
      meaning: t('用户本次消费中实际使用充值余额支付的部分。'),
      formula: t('用户充值余额实际支付金额'),
      sample: '80',
    },
    {
      field: t('充值支付占比'),
      meaning: t('充值余额支付金额占用户收费的比例。'),
      formula: t('充值支付 ÷ 用户收费'),
      sample: '80 ÷ 100 = 80%',
    },
    {
      field: t('奖励/赠送抵扣'),
      meaning: t('用户收费中由奖励、赠送或活动额度抵扣的部分。'),
      formula: t('用户收费 - 充值支付'),
      sample: '20',
    },
    {
      field: t('用户覆盖成本'),
      meaning: t('用户充值余额支付按比例覆盖的主站成本。'),
      formula: t('基础成本 × 充值支付占比'),
      sample: '60 × 80% = 48',
    },
    {
      field: t('服务商承担'),
      meaning: t('未被用户充值余额覆盖、由服务商主账号承担的成本。'),
      formula: t('基础成本 - 用户覆盖成本'),
      sample: '60 - 48 = 12',
    },
    {
      field: t('理论毛利'),
      meaning: t('用户收费扣除基础成本、尚未按充值支付占比折算的毛利。'),
      formula: t('用户收费 - 基础成本'),
      sample: '100 - 60 = 40',
    },
    {
      field: t('毛利润'),
      meaning: t('扣除主站成本后、分佣前并按充值支付占比折算的利润。'),
      formula: t('(用户收费 - 基础成本) × 充值支付占比'),
      sample: '(100 - 60) × 80% = 32',
    },
    {
      field: t('分佣'),
      meaning: t('按邀请关系从毛利润中分出的佣金。'),
      formula: t('一级分佣 + 二级分佣'),
      sample: '4',
    },
    {
      field: t('净利润'),
      meaning: t('扣除分佣后最终结算给服务商的利润。'),
      formula: t('毛利润 - 分佣'),
      sample: '32 - 4 = 28',
    },
  ];

  const aggregateRows = [
    {
      field: t('收入'),
      meaning: t('指定时间范围内所有调用结算给服务商的净利润。'),
      formula: t('SUM(净利润)'),
      sample: '28',
    },
    {
      field: t('支出'),
      meaning: t('指定时间范围内由服务商承担的主站成本。'),
      formula: t('SUM(服务商承担)'),
      sample: '12',
    },
    {
      field: t('利润'),
      meaning: t('管理员视角扣除服务商承担成本后的利润。'),
      formula: t('收入 - 支出'),
      sample: '28 - 12 = 16',
    },
  ];

  return (
    <div className='log-v2-page provider-profit-page mt-[10px] px-2'>
      <div className='log-v2 usage-logs-v2'>
        <div className='log-v2-shell'>
          <div className='log-v2-stack'>
            <section className='usage-logs-v2-header'>
              <div className='flex flex-wrap items-center justify-between gap-3'>
                <div>
                  <div className='usage-logs-v2-title'>{t('服务商利润')}</div>
                  <p className='usage-logs-v2-description'>
                    {t('查看所有服务商在指定时间范围内的收入、支出和利润。')}
                  </p>
                </div>
                <Button
                  icon={<Calculator size={15} />}
                  onClick={() => setFormulaVisible(true)}
                >
                  {t('查看计算公式')}
                </Button>
              </div>
            </section>

            <section className='log-v2-filter-card usage-logs-v2-filter-card'>
              <Form
                initValues={formInitValues}
                getFormApi={setFormApi}
                onSubmit={() => loadOverview(1)}
                allowEmpty
                layout='vertical'
                className='usage-logs-v2-filter-form provider-profit-filter-form'
              >
                <div className='provider-profit-filter-row'>
                  <div className='provider-profit-filter-title'>
                    <span className='provider-profit-filter-label'>
                      <CalendarClock size={14} />
                      {t('时间范围')}
                    </span>
                    <span className='provider-profit-filter-hint'>
                      {t('下方统计与明细均按此时间范围')}
                    </span>
                  </div>
                  <Form.DatePicker
                    field='dateRange'
                    type='dateTimeRange'
                    showClear
                    pure
                    size='large'
                    className='usage-logs-v2-control usage-logs-v2-control-range provider-profit-filter-picker'
                    presets={DATE_RANGE_PRESETS.map((preset) => ({
                      text: t(preset.text),
                      start: preset.start(),
                      end: preset.end(),
                    }))}
                  />
                  <Button
                    htmlType='submit'
                    loading={loading}
                    icon={<RefreshCw size={15} />}
                    className='usage-logs-v2-button usage-logs-v2-button-primary provider-profit-filter-button'
                  >
                    {t('查询')}
                  </Button>
                </div>
              </Form>
            </section>

            <section className='usage-logs-v2-stats'>
              <div className='grid grid-cols-1 sm:grid-cols-3 gap-3'>
                {statCards.map((item) => (
                  <div
                    key={item.key}
                    className='rounded-xl border border-[var(--lg-border)] bg-[var(--lg-surface)] p-4'
                  >
                    <div className='flex items-center justify-between mb-2'>
                      <div className='text-xs text-[var(--lg-text-muted)] font-medium'>
                        {item.label}
                      </div>
                      <Coins size={14} className='text-[var(--lg-text-soft)]' />
                    </div>
                    <div className='text-lg font-bold text-[var(--lg-text)] tracking-tight'>
                      {quotaText(summary[item.key])}
                    </div>
                  </div>
                ))}
              </div>
            </section>

            <CardPro
              className='provider-profit-table-card'
              paginationArea={paginationArea}
            >
              <Spin spinning={loading}>
                <Table
                  columns={columns}
                  dataSource={items}
                  rowKey='provider_id'
                  expandedRowKeys={expandedRowKeys}
                  expandedRowRender={expandedRowRender}
                  expandRowByClick
                  onExpand={(expanded, record) => {
                    const providerId = Number(record.provider_id || 0);
                    setExpandedRowKeys((current) =>
                      expanded
                        ? [...current, providerId]
                        : current.filter((key) => key !== providerId),
                    );
                    if (expanded) loadProviderDetails(record);
                  }}
                  pagination={false}
                  empty={
                    <Empty
                      description={t('暂无服务商利润数据')}
                      style={{ padding: 40 }}
                    />
                  }
                  className='usage-logs-v2-table rounded-xl overflow-hidden'
                  size='middle'
                />
              </Spin>
            </CardPro>
          </div>
        </div>
      </div>

      <Modal
        title={t('计算公式与样例')}
        visible={formulaVisible}
        onCancel={() => setFormulaVisible(false)}
        footer={null}
        width={720}
        bodyStyle={{ maxHeight: '55vh', overflowY: 'auto', paddingRight: 12 }}
      >
        <div className='space-y-4'>
          <div>
            <Title heading={6}>{t('字段含义与计算逻辑')}</Title>
            <div className='overflow-x-auto'>
              <table className='w-full text-sm'>
                <thead>
                  <tr>
                    <th className='text-left p-2'>{t('字段')}</th>
                    <th className='text-left p-2'>{t('通俗解释')}</th>
                    <th className='text-left p-2'>{t('计算逻辑')}</th>
                    <th className='text-right p-2'>{t('样例')}</th>
                  </tr>
                </thead>
                <tbody>
                  {formulaRows.map((row) => (
                    <tr
                      key={row.field}
                      className='border-t border-[var(--lg-border)]'
                    >
                      <td className='p-2 font-medium whitespace-nowrap'>
                        {row.field}
                      </td>
                      <td className='p-2'>{row.meaning}</td>
                      <td className='p-2'>{row.formula}</td>
                      <td className='p-2 text-right whitespace-nowrap'>
                        {row.sample}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          <div>
            <Title heading={6}>{t('管理员汇总指标')}</Title>
            <div className='overflow-x-auto'>
              <table className='w-full text-sm'>
                <thead>
                  <tr>
                    <th className='text-left p-2'>{t('字段')}</th>
                    <th className='text-left p-2'>{t('含义')}</th>
                    <th className='text-left p-2'>{t('计算公式')}</th>
                    <th className='text-right p-2'>{t('样例')}</th>
                  </tr>
                </thead>
                <tbody>
                  {aggregateRows.map((row) => (
                    <tr
                      key={row.field}
                      className='border-t border-[var(--lg-border)]'
                    >
                      <td className='p-2 font-medium'>{row.field}</td>
                      <td className='p-2'>{row.meaning}</td>
                      <td className='p-2'>{row.formula}</td>
                      <td className='p-2 text-right whitespace-nowrap'>
                        {row.sample}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          <div>
            <Title heading={6}>{t('完整样例')}</Title>
            <Text>
              {t(
                '假设用户收费为 100，其中充值支付 80，奖励或赠送额度抵扣 20，基础成本为 60，分佣为 4。',
              )}
            </Text>
            <ol className='list-decimal pl-5 mt-2 space-y-1 text-sm'>
              <li>{t('充值支付占比')}：80 ÷ 100 = 80%</li>
              <li>{t('用户覆盖成本')}：60 × 80% = 48</li>
              <li>{t('服务商承担')}：60 - 48 = 12</li>
              <li>{t('毛利润')}：(100 - 60) × 80% = 32</li>
              <li>{t('分佣')}：4</li>
              <li>{t('净利润')}：32 - 4 = 28</li>
              <li>{t('收入')}：28</li>
              <li>{t('支出')}：12</li>
              <li>{t('利润')}：28 - 12 = 16</li>
            </ol>
          </div>

          <Text type='tertiary'>
            {t(
              '奖励、赠送或活动额度不会直接形成可结算收入；用户收费也不等同于服务商收入。',
            )}
          </Text>
        </div>
      </Modal>
    </div>
  );
};

export default AdminProviderProfitsPage;
