import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Empty,
  Form,
  SideSheet,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';
import {
  ArrowRight,
  Coins,
  CreditCard,
  FileText,
  Gift,
  Info,
  ReceiptText,
  RotateCcw,
  Search,
  WalletCards,
} from 'lucide-react';
import CardTable from '../../components/common/ui/CardTable';
import { DATE_RANGE_PRESETS } from '../../constants/console.constants';
import {
  API,
  getTodayStartTimestamp,
  renderQuota,
  showError,
  timestamp2string,
} from '../../helpers';
import { createCardProPagination } from '../../helpers/utils';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import '../Log/log-v2.css';

const { Text, Title } = Typography;

const ProviderProfitsPage = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [formApi, setFormApi] = useState(null);
  const [loading, setLoading] = useState(false);
  const [records, setRecords] = useState([]);
  const [summary, setSummary] = useState({});
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(
    parseInt(localStorage.getItem('page-size')) || 10,
  );
  const [total, setTotal] = useState(0);
  const [detailRecord, setDetailRecord] = useState(null);

  const now = new Date();
  const formInitValues = {
    provider_user_id: '',
    model_name: '',
    request_id: '',
    dateRange: [
      timestamp2string(getTodayStartTimestamp()),
      timestamp2string(now.getTime() / 1000 + 3600),
    ],
  };

  const getQueryValues = () => {
    const values = formApi ? formApi.getValues() : formInitValues;
    const dateRange = Array.isArray(values.dateRange)
      ? values.dateRange
      : formInitValues.dateRange;
    return {
      providerUserId: values.provider_user_id || '',
      modelName: values.model_name || '',
      requestId: values.request_id || '',
      startTimestamp: Date.parse(dateRange[0]) / 1000,
      endTimestamp: Date.parse(dateRange[1]) / 1000,
    };
  };

  const loadProfits = async (page = activePage, size = pageSize) => {
    setLoading(true);
    const query = getQueryValues();
    const url = encodeURI(
      `/api/provider/profits?p=${page}&page_size=${size}&provider_user_id=${query.providerUserId}&model_name=${query.modelName}&request_id=${query.requestId}&start_timestamp=${query.startTimestamp}&end_timestamp=${query.endTimestamp}`,
    );
    try {
      const res = await API.get(url);
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      const pageData = data?.page || {};
      setRecords(pageData.items || []);
      setSummary(data?.summary || {});
      setActivePage(pageData.page || page);
      setPageSize(pageData.page_size || size);
      setTotal(pageData.total || 0);
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadProfits(1, pageSize);
  }, [formApi]);

  const resetFilters = () => {
    formApi?.reset();
    setTimeout(() => loadProfits(1, pageSize), 100);
  };

  const quotaText = (value) => renderQuota(value || 0, 6);

  const percentText = (numerator, denominator) => {
    if (!denominator || denominator <= 0) return '0%';
    return `${((Number(numerator || 0) / Number(denominator)) * 100).toFixed(2)}%`;
  };

  const getGrossProfitQuota = (record) => {
    const explicit = Number(record?.gross_profit_quota || 0);
    if (explicit > 0) return explicit;
    const charge = Number(record?.provider_user_quota || 0);
    const baseCost = Number(record?.base_cost_quota || 0);
    const paid = Math.min(Math.max(Number(record?.paid_quota || 0), 0), charge);
    const gross = Math.max(charge - baseCost, 0);
    if (charge <= 0 || paid <= 0 || gross <= 0) return 0;
    return Math.floor((gross * paid) / charge);
  };

  const getRebateQuota = (record) => {
    const explicit = Number(record?.rebate_quota || 0);
    if (explicit > 0) return explicit;
    return Math.max(
      getGrossProfitQuota(record) - Number(record?.profit_quota || 0),
      0,
    );
  };

  const getPaymentInfo = (record) => {
    const total = Number(record?.provider_user_quota || 0);
    const cash = Math.min(Math.max(Number(record?.paid_quota || 0), 0), total);
    const reward = Math.max(total - cash, 0);
    if (total <= 0) {
      return {
        type: 'unknown',
        label: t('无法判断支付方式'),
        color: 'grey',
        icon: <Info size={14} />,
        total,
        cash,
        reward,
        ratio: '0%',
        explanation: t('本次应扣金额为 0，系统无法根据金额判断用户使用了哪类余额。'),
      };
    }
    if (cash <= 0) {
      return {
        type: 'reward_only',
        label: t('纯奖励余额支付'),
        color: 'orange',
        icon: <Gift size={14} />,
        total,
        cash,
        reward,
        ratio: '0%',
        explanation: t('用户这次没有使用充值余额，而是用奖励、赠送或活动额度完成调用。这类额度不是用户真实充值产生的现金收入，所以不会直接给服务商结算利润；但调用主站模型仍然会产生真实成本，通常由服务商主账号承担。'),
      };
    }
    if (cash >= total) {
      return {
        type: 'cash_only',
        label: t('纯充值余额支付'),
        color: 'green',
        icon: <CreditCard size={14} />,
        total,
        cash,
        reward,
        ratio: '100%',
        explanation: t('用户这次全部使用充值余额支付。这部分属于真实充值余额消耗，所以如果服务商售价高于主站成本，差额可以完整参与利润结算。'),
      };
    }
    return {
      type: 'mixed',
      label: t('奖励余额 + 充值余额混合支付'),
      color: 'blue',
      icon: <WalletCards size={14} />,
      total,
      cash,
      reward,
      ratio: percentText(cash, total),
      explanation: t('用户这次一部分用充值余额支付，一部分用奖励、赠送或活动额度抵扣。系统只按充值余额占比结算利润，奖励额度对应的部分不会产生可结算利润。'),
    };
  };

  const summaryCards = [
    { key: 'provider_user_quota', label: t('用户收费') },
    { key: 'base_cost_quota', label: t('基础成本') },
    { key: 'covered_cost_quota', label: t('用户覆盖成本') },
    { key: 'owner_cost_quota', label: t('服务商承担') },
    { key: 'gross_profit_quota', label: t('毛利润') },
    { key: 'rebate_quota', label: t('分佣') },
    { key: 'profit_quota', label: t('净利润') },
  ];

  const columns = useMemo(
    () => [
      {
        title: t('时间'),
        dataIndex: 'created_at',
        render: (value) => timestamp2string(value),
      },
      {
        title: t('服务商用户'),
        dataIndex: 'provider_user_id',
        render: (value) => <Tag shape='circle'>#{value}</Tag>,
      },
      {
        title: t('公开模型'),
        dataIndex: 'public_model_name',
      },
      {
        title: t('基础模型'),
        dataIndex: 'base_model_name',
      },
      {
        title: t('用户收费'),
        dataIndex: 'provider_user_quota',
        render: (value) => quotaText(value),
      },
      {
        title: t('基础成本'),
        dataIndex: 'base_cost_quota',
        render: (value) => quotaText(value),
      },
      {
        title: t('充值支付'),
        dataIndex: 'paid_quota',
        render: (value) => quotaText(value),
      },
      {
        title: t('服务商承担'),
        dataIndex: 'owner_cost_quota',
        render: (value) => quotaText(value),
      },
      {
        title: t('毛利润'),
        dataIndex: 'gross_profit_quota',
        render: (_, record) => quotaText(getGrossProfitQuota(record)),
      },
      {
        title: t('分佣'),
        dataIndex: 'rebate_quota',
        render: (_, record) => quotaText(getRebateQuota(record)),
      },
      {
        title: t('净利润'),
        dataIndex: 'profit_quota',
        render: (value) => <Text strong>{quotaText(value)}</Text>,
      },
      {
        title: 'Request ID',
        dataIndex: 'request_id',
        render: (value) => (
          <Text copyable ellipsis={{ showTooltip: true }} style={{ maxWidth: 220 }}>
            {value}
          </Text>
        ),
      },
      {
        title: t('明细'),
        dataIndex: 'detail',
        fixed: 'right',
        width: 120,
        render: (_, record) => (
          <Button
            size='small'
            type='tertiary'
            icon={<ReceiptText size={14} />}
            onClick={() => setDetailRecord(record)}
          >
            {t('查看明细')}
          </Button>
        ),
      },
    ],
    [t],
  );

  const paginationArea = createCardProPagination({
    currentPage: activePage,
    pageSize,
    total,
    onPageChange: (page) => loadProfits(page, pageSize),
    onPageSizeChange: (size) => {
      localStorage.setItem('page-size', `${size}`);
      loadProfits(1, size);
    },
    isMobile,
    t,
  });

  const renderMoneyRow = (label, value, help, tone = 'default') => {
    const color =
      tone === 'profit'
        ? 'var(--semi-color-success)'
        : tone === 'cost'
          ? 'var(--semi-color-warning)'
          : 'var(--semi-color-text-0)';
    return (
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: isMobile ? '1fr' : '180px 1fr',
          gap: isMobile ? 4 : 12,
          padding: '12px 0',
          borderBottom: '1px solid var(--semi-color-border)',
        }}
      >
        <Text type='secondary'>{label}</Text>
        <div>
          <div style={{ color, fontWeight: 700 }}>{quotaText(value)}</div>
          <Text type='tertiary' size='small'>
            {help}
          </Text>
        </div>
      </div>
    );
  };

  const renderInfoRow = (label, value) => (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: isMobile ? '1fr' : '180px 1fr',
        gap: isMobile ? 4 : 12,
        padding: '12px 0',
        borderBottom: '1px solid var(--semi-color-border)',
      }}
    >
      <Text type='secondary'>{label}</Text>
      <Text>{value || '-'}</Text>
    </div>
  );

  const detail = detailRecord || {};
  const paymentInfo = getPaymentInfo(detail);
  const paidRatio = paymentInfo.ratio;
  const theoreticalGrossProfit = Math.max(
    Number(detail.provider_user_quota || 0) - Number(detail.base_cost_quota || 0),
    0,
  );
  const grossProfit = getGrossProfitQuota(detail);
  const rebateQuota = getRebateQuota(detail);
  const netProfit = Number(detail.profit_quota || 0);

  return (
    <div className='log-v2-page mt-[10px] px-2'>
      <div className='log-v2 usage-logs-v2'>
        <div className='log-v2-shell'>
          <div className='log-v2-stack'>
            <section className='usage-logs-v2-header'>
              <div className='usage-logs-v2-title'>{t('服务商利润')}</div>
              <p className='usage-logs-v2-description'>
                {t('查看当前服务商下用户调用产生的收费、成本和利润流水。')}
              </p>
            </section>

            <section className='usage-logs-v2-stats'>
              <div className='usage-logs-v2-stat-grid'>
                {summaryCards.map((item) => (
                  <div
                    key={item.key}
                    className='usage-logs-v2-stat-card usage-logs-v2-stat-accent-quota'
                  >
                    <div className='usage-logs-v2-stat-head'>
                      <div className='usage-logs-v2-stat-label'>{item.label}</div>
                      <div className='usage-logs-v2-stat-icon usage-logs-v2-stat-icon-quota'>
                        <Coins size={16} />
                      </div>
                    </div>
                    <div className='usage-logs-v2-stat-value'>
                      {quotaText(summary[item.key])}
                    </div>
                  </div>
                ))}
              </div>
            </section>

            <section className='log-v2-filter-card usage-logs-v2-filter-card'>
              <Form
                initValues={formInitValues}
                getFormApi={setFormApi}
                onSubmit={() => loadProfits(1, pageSize)}
                allowEmpty
                layout='vertical'
                className='usage-logs-v2-filter-form'
              >
                <div className='usage-logs-v2-filter-header'>
                  <div className='usage-logs-v2-filter-title'>{t('筛选条件')}</div>
                  <div className='usage-logs-v2-filter-actions'>
                    <Button
                      htmlType='submit'
                      loading={loading}
                      icon={<Search size={15} />}
                      className='usage-logs-v2-button usage-logs-v2-button-primary'
                    >
                      {t('查询')}
                    </Button>
                    <Button
                      type='tertiary'
                      onClick={resetFilters}
                      icon={<RotateCcw size={15} />}
                      className='usage-logs-v2-button usage-logs-v2-button-secondary'
                    >
                      {t('重置')}
                    </Button>
                  </div>
                </div>
                <div className='usage-logs-v2-filter-grid'>
                  <div className='usage-logs-v2-filter-item usage-logs-v2-filter-item-wide'>
                    <div className='usage-logs-v2-filter-label'>{t('时间范围')}</div>
                    <Form.DatePicker
                      field='dateRange'
                      type='dateTimeRange'
                      showClear
                      pure
                      size='large'
                      className='usage-logs-v2-control usage-logs-v2-control-range'
                      presets={DATE_RANGE_PRESETS.map((preset) => ({
                        text: t(preset.text),
                        start: preset.start(),
                        end: preset.end(),
                      }))}
                    />
                  </div>
                  <div className='usage-logs-v2-filter-item'>
                    <div className='usage-logs-v2-filter-label'>{t('服务商用户 ID')}</div>
                    <Form.Input
                      field='provider_user_id'
                      prefix={<IconSearch />}
                      showClear
                      pure
                      size='large'
                      className='usage-logs-v2-control'
                    />
                  </div>
                  <div className='usage-logs-v2-filter-item'>
                    <div className='usage-logs-v2-filter-label'>{t('模型名称')}</div>
                    <Form.Input
                      field='model_name'
                      prefix={<IconSearch />}
                      showClear
                      pure
                      size='large'
                      className='usage-logs-v2-control'
                    />
                  </div>
                  <div className='usage-logs-v2-filter-item'>
                    <div className='usage-logs-v2-filter-label'>Request ID</div>
                    <Form.Input
                      field='request_id'
                      prefix={<IconSearch />}
                      showClear
                      pure
                      size='large'
                      className='usage-logs-v2-control'
                    />
                  </div>
                </div>
              </Form>
            </section>

            <section className='log-v2-table-card usage-logs-v2-table-card'>
              <div className='usage-logs-v2-table-wrap'>
                <CardTable
                  columns={columns}
                  dataSource={records}
                  rowKey='id'
                  loading={loading}
                  scroll={{ x: 'max-content' }}
                  hidePagination
                  empty={
                    <Empty
                      description={t('暂无服务商利润数据')}
                      style={{ padding: 40 }}
                    />
                  }
                  className='usage-logs-v2-table rounded-[24px] overflow-hidden'
                  size='small'
                />
              </div>
              {paginationArea && (
                <div className='usage-logs-v2-pagination'>{paginationArea}</div>
              )}
            </section>
          </div>
        </div>
      </div>

      <SideSheet
        title={t('利润明细')}
        visible={!!detailRecord}
        onCancel={() => setDetailRecord(null)}
        width={isMobile ? '100%' : 760}
        placement='right'
      >
        {detailRecord ? (
          <div style={{ paddingBottom: 24 }}>
            <Space vertical align='start' spacing={8} style={{ width: '100%' }}>
              <Tag color='blue' prefixIcon={<FileText size={14} />}>
                {t('一次调用，一条利润流水')}
              </Tag>
              <Title heading={5} style={{ margin: 0 }}>
                {detail.public_model_name || '-'}
                <ArrowRight size={15} style={{ margin: '0 8px' }} />
                {detail.base_model_name || '-'}
              </Title>
              <Text type='secondary'>
                {t('这条记录来自服务商用户的一次模型调用。系统会先按服务商售价向用户计费，再按主站真实模型成本计算服务商应承担的成本和可入账的利润。')}
              </Text>
            </Space>

            <div style={{ marginTop: 20 }}>
              <Title heading={6}>{t('基础信息')}</Title>
              {renderInfoRow(t('发生时间'), timestamp2string(detail.created_at))}
              {renderInfoRow(t('服务商用户'), `#${detail.provider_user_id}`)}
              <div
                style={{
                  display: 'grid',
                  gap: 8,
                  padding: '12px 0',
                  borderBottom: '1px solid var(--semi-color-border)',
                }}
              >
                <Text type='secondary'>Request ID</Text>
                <Text copyable ellipsis={{ showTooltip: true }}>
                  {detail.request_id}
                </Text>
              </div>
            </div>

            <div
              style={{
                marginTop: 22,
                padding: 16,
                borderRadius: 8,
                background: 'var(--semi-color-fill-0)',
                border: '1px solid var(--semi-color-border)',
              }}
            >
              <Space align='center' spacing={10} wrap>
                <Tag color={paymentInfo.color} prefixIcon={paymentInfo.icon}>
                  {paymentInfo.label}
                </Tag>
                <Text type='secondary'>
                  {t('充值支付占比')}：{paymentInfo.ratio}
                </Text>
              </Space>
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: isMobile ? '1fr' : 'repeat(3, 1fr)',
                  gap: 12,
                  marginTop: 14,
                }}
              >
                <div>
                  <Text type='secondary' size='small'>{t('本次应扣')}</Text>
                  <div style={{ fontWeight: 700 }}>{quotaText(paymentInfo.total)}</div>
                </div>
                <div>
                  <Text type='secondary' size='small'>{t('充值余额支付')}</Text>
                  <div style={{ fontWeight: 700, color: 'var(--semi-color-success)' }}>
                    {quotaText(paymentInfo.cash)}
                  </div>
                </div>
                <div>
                  <Text type='secondary' size='small'>{t('奖励/赠送抵扣')}</Text>
                  <div style={{ fontWeight: 700, color: 'var(--semi-color-warning)' }}>
                    {quotaText(paymentInfo.reward)}
                  </div>
                </div>
              </div>
              <div style={{ marginTop: 12 }}>
                <Text type='secondary'>{paymentInfo.explanation}</Text>
              </div>
            </div>

            <div style={{ marginTop: 22 }}>
              <Title heading={6}>{t('这笔钱是怎么算出来的')}</Title>
              {renderMoneyRow(
                t('用户收费'),
                detail.provider_user_quota,
                t('服务商用户看到并支付的价格，按服务商自己设置的模型售价计算。'),
              )}
              {renderMoneyRow(
                t('主站成本'),
                detail.base_cost_quota,
                t('这次调用主站真实模型产生的成本，也就是服务商使用主站资源的成本。'),
                'cost',
              )}
              {renderMoneyRow(
                t('充值余额支付'),
                detail.paid_quota,
                t('这里只统计用户用充值余额实际支付的部分；奖励余额、赠送额度、活动额度不会直接形成可结算利润。'),
              )}
              {renderMoneyRow(
                t('奖励/赠送抵扣'),
                paymentInfo.reward,
                t('这是用户本次应扣金额里，没有用充值余额支付的部分。它可以让用户完成调用，但不按现金收入结算利润。'),
                'cost',
              )}
              {renderMoneyRow(
                t('用户覆盖成本'),
                detail.covered_cost_quota,
                t('用户本次充值余额支付中，被用来抵扣主站成本的部分。'),
              )}
              {renderMoneyRow(
                t('服务商承担'),
                detail.owner_cost_quota,
                t('如果用户充值余额支付不足以覆盖主站成本，差额会由服务商主账号余额承担。'),
                'cost',
              )}
              {renderMoneyRow(
                t('毛利润'),
                grossProfit,
                t('服务商售价扣除主站成本后，按充值余额支付占比折算出的分佣前利润。'),
                'profit',
              )}
              {renderMoneyRow(
                t('一级/二级分佣'),
                rebateQuota,
                t('按服务商模型配置的一级、二级分佣比例，从毛利润中扣除后发给邀请人。'),
                'cost',
              )}
              {renderMoneyRow(
                t('净利润'),
                netProfit,
                t('服务商最终入账金额。净利润 = 毛利润 - 一级/二级分佣。'),
                'profit',
              )}
            </div>

            <div
              style={{
                marginTop: 22,
                padding: 16,
                borderRadius: 8,
                background: 'var(--semi-color-fill-0)',
                border: '1px solid var(--semi-color-border)',
              }}
            >
              <Space align='start' spacing={10}>
                <Info size={18} color='var(--semi-color-primary)' />
                <div>
                  <Text strong>{t('普通理解')}</Text>
                  <div style={{ marginTop: 8 }}>
                    <Text type='secondary'>
                      {t('可以把它理解成：用户这次应扣多少钱，先看里面有多少是真正的充值余额支付。充值余额支付代表真实现金消耗，可以参与利润结算；奖励、赠送或活动额度只是平台给用户的使用权益，不会直接变成服务商利润。')}
                    </Text>
                  </div>
                </div>
              </Space>
            </div>

            <div
              style={{
                marginTop: 16,
                padding: 16,
                borderRadius: 8,
                background: 'var(--semi-color-bg-1)',
                border: '1px solid var(--semi-color-border)',
              }}
            >
              <Title heading={6} style={{ marginTop: 0 }}>
                {t('计算公式')}
              </Title>
              <Space vertical align='start' spacing={8}>
                <Text>
                  {t('充值支付占比')} = {quotaText(detail.paid_quota)} /{' '}
                  {quotaText(detail.provider_user_quota)} = {paidRatio}
                </Text>
                <Text>
                  {t('奖励/赠送抵扣')} = {t('用户收费')} - {t('充值余额支付')} ={' '}
                  {quotaText(paymentInfo.reward)}
                </Text>
                <Text>
                  {t('理论毛利')} = {t('用户收费')} - {t('主站成本')} ={' '}
                  {quotaText(theoreticalGrossProfit)}
                </Text>
                <Text>
                  {t('毛利润')} = {t('理论毛利')} × {t('充值支付占比')} ={' '}
                  <Text strong>{quotaText(grossProfit)}</Text>
                </Text>
                <Text>
                  {t('分佣')} = {t('一级分佣')} + {t('二级分佣')} ={' '}
                  <Text strong>{quotaText(rebateQuota)}</Text>
                </Text>
                <Text>
                  {t('净利润')} = {t('毛利润')} - {t('分佣')} ={' '}
                  <Text strong>{quotaText(netProfit)}</Text>
                </Text>
                <Text>
                  {t('服务商承担')} = {t('主站成本')} - {t('用户覆盖成本')} ={' '}
                  <Text strong>{quotaText(detail.owner_cost_quota)}</Text>
                </Text>
              </Space>
            </div>
          </div>
        ) : null}
      </SideSheet>
    </div>
  );
};

export default ProviderProfitsPage;
