import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  DatePicker,
  Input,
  Pagination,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconFilter, IconRefresh, IconSearch } from '@douyinfe/semi-icons';
import {
  BadgeCheck,
  CircleAlert,
  CircleCheckBig,
  Coins,
  SquareKanban,
  ShieldCheck,
} from 'lucide-react';
import { SiAlipay, SiStripe } from 'react-icons/si';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import ReconciliationDetailSheet from './ReconciliationDetailSheet';

import weChatImg from '../../../public/WeChat.png';
import stripeImg from '../../../public/stripe.png';

const { Text } = Typography;

const PAYMENT_METHODS = {
  wxpay: 'wxpay',
  alipay: 'alipay',
  stripe: 'stripe',
  crypto: 'crypto',
};

const PAYMENT_METHOD_TABS = [
  { key: PAYMENT_METHODS.wxpay, label: '微信支付' },
  // 支付宝对账暂不开放，后续启用时取消注释即可
  // { key: PAYMENT_METHODS.alipay, label: '支付宝' },
  { key: PAYMENT_METHODS.stripe, label: 'Stripe' },
  { key: PAYMENT_METHODS.crypto, label: '加密货币' },
];

const CARD_META = [
  {
    key: 'total',
    title: '充值订单总额',
    desc: '系统生成订单的总金额',
    tone: 'normal',
    icon: SquareKanban,
  },
  {
    key: 'success',
    title: '支付成功总数',
    desc: '支付渠道确认收到单子',
    tone: 'normal',
    icon: ShieldCheck,
  },
  {
    key: 'matched',
    title: '对账一致（平账）',
    desc: '正常',
    tone: 'success',
    icon: ShieldCheck,
  },
  {
    key: 'abnormal',
    title: '对账异常',
    desc: '需要人工处理',
    tone: 'danger',
    icon: CircleAlert,
    dangerIconText: '!',
  },
  {
    key: 'abnormalAmount',
    title: '异常涉及总金额',
    desc: '',
    tone: 'danger-soft',
    icon: CircleAlert,
    dangerIconText: '¥',
  },
];

const statusMap = {
  matched: {
    text: '一致',
    bg: '#D9F3E8',
    color: '#09CC73',
    icon: CircleCheckBig,
  },
  abnormal: {
    text: '异常',
    bg: '#F9E7E7',
    color: '#FF4D4F',
    icon: CircleAlert,
  },
};

const cardClass = {
  normal: 'border-transparent bg-white dark:bg-slate-800',
  success: 'border-transparent bg-white dark:bg-slate-800',
  danger: 'border-[#FF4D4F] bg-[#FDF2F2] dark:bg-rose-900/10',
  'danger-soft': 'border-[#FF4D4F] bg-[#FDF2F2] dark:bg-rose-900/10',
};

const HeaderText = ({ children }) => (
  <span className='text-[13px] font-medium text-[#93A5BD]'>{children}</span>
);

const formatDate = (value) => dayjs(value).format('YYYY-MM-DD');

const buildDateQuery = (dateValue) => {
  if (!dateValue) return '';
  return `&bill_date=${encodeURIComponent(formatDate(dateValue))}`;
};

const buildPaymentMethodQuery = (paymentMethod) =>
  `payment_method=${encodeURIComponent(paymentMethod || PAYMENT_METHODS.wxpay)}`;

const renderLocalType = (localType, t) => {
  if (localType === 'subscription') return t('订阅');
  if (localType === 'topup') return t('充值');
  return '-';
};

const PayReconciliationList = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const yesterday = dayjs().subtract(1, 'day').toDate();

  const [channelTab, setChannelTab] = useState(PAYMENT_METHODS.wxpay);
  const [billDate, setBillDate] = useState(yesterday);
  const [keyword, setKeyword] = useState('');
  const [keywordInput, setKeywordInput] = useState('');

  const [statLoading, setStatLoading] = useState(false);
  const [stat, setStat] = useState({
    total_order_count: 0,
    payment_success_count: 0,
    matched_count: 0,
    abnormal_count: 0,
    total_amount: 0,
    abnormal_amount: 0,
    latest_sync_at_text: '',
  });

  const [listLoading, setListLoading] = useState(false);
  const [rows, setRows] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(10);

  const [detailVisible, setDetailVisible] = useState(false);
  const [activeRow, setActiveRow] = useState(null);

  const loadStats = async () => {
    setStatLoading(true);
    try {
      const qs = buildDateQuery(billDate);
      const url = `/api/wechat_trade_bill/stat?${buildPaymentMethodQuery(channelTab)}${qs}`;
      const res = await API.get(url);
      const { success, message, data } = res.data;
      if (success) {
        setStat((prev) => ({ ...prev, ...(data || {}) }));
      } else {
        showError(message || t('加载失败'));
      }
    } catch {
      showError(t('加载失败'));
    } finally {
      setStatLoading(false);
    }
  };

  const loadList = async (targetPage = page) => {
    setListLoading(true);
    try {
      const qs =
        `p=${targetPage}&page_size=${pageSize}&${buildPaymentMethodQuery(channelTab)}` +
        buildDateQuery(billDate) +
        (keyword ? `&keyword=${encodeURIComponent(keyword)}` : '');
      const res = await API.get(`/api/wechat_trade_bill/list?${qs}`);
      const { success, message, data } = res.data;
      if (success) {
        setRows(data?.items || []);
        setTotal(data?.total || 0);
      } else {
        showError(message || t('加载失败'));
      }
    } catch {
      showError(t('加载失败'));
    } finally {
      setListLoading(false);
    }
  };

  const runSyncBill = async () => {
    if (!billDate) {
      showError(t('请选择日期'));
      return;
    }
    setListLoading(true);
    try {
      const res = await API.post('/api/wechat_trade_bill/run', {
        bill_date: formatDate(billDate),
        payment_method: channelTab || PAYMENT_METHODS.wxpay,
      });
      const { success, message, data } = res.data;
      if (success) {
        // 支付宝"当天无账单"：后端返回 has_bill=false，前端按当前语言本地化提示，
        // 避免直接展示后端英文 message（如 "no bill for bill_date=..."）。
        if (channelTab === PAYMENT_METHODS.alipay && data?.has_bill === false) {
          showSuccess(
            t('当天无账单：{{date}}', { date: formatDate(billDate) }),
          );
        } else {
          showSuccess(data?.message || message || t('同步成功'));
        }
        loadStats().then();
        loadList(page).then();
      } else {
        showError(message || t('同步失败'));
      }
    } catch {
      showError(t('同步失败'));
    } finally {
      setListLoading(false);
    }
  };

  useEffect(() => {
    setPage(1);
    loadStats().then();
    loadList(1).then();
  }, [billDate, keyword, channelTab]);

  useEffect(() => {
    loadList(page).then();
  }, [page]);

  const summaryCards = useMemo(() => {
    const formatAmount = (value) => {
      const rawValue = value ?? 0;
      if (channelTab === PAYMENT_METHODS.crypto) {
        return `${rawValue}`;
      }
      return Number(rawValue).toFixed(2);
    };

    return CARD_META.map((m) => {
      if (m.key === 'total')
        return {
          ...m,
          value: `${stat.currency_symbol} ${formatAmount(stat.total_amount)}`,
        };
      if (m.key === 'success')
        return {
          ...m,
          value: `${stat.payment_success_count || 0}`,
          unit: t('笔'),
        };
      if (m.key === 'matched')
        return { ...m, value: `${stat.matched_count || 0}`, unit: t('笔') };
      if (m.key === 'abnormal')
        return { ...m, value: `${stat.abnormal_count || 0}`, unit: t('笔') };
      if (m.key === 'abnormalAmount')
        return {
          ...m,
          value: `${stat.currency_symbol} ${formatAmount(stat.abnormal_amount)}`,
        };
      return m;
    });
  }, [channelTab, stat, t]);

  const columns = useMemo(
    () => [
      {
        title: <HeaderText>{t('对账状态')}</HeaderText>,
        dataIndex: 'status',
        key: 'status',
        width: 130,
        render: (status, record) => {
          const normalized = status === 'matched' ? 'matched' : 'abnormal';
          const cfg = statusMap[normalized] || statusMap.matched;
          const Icon = cfg.icon;
          return (
            <Tag
              className='!border-0 !rounded-[10px] !px-[12px] !h-[34px] !inline-flex !items-center'
              style={{ background: cfg.bg, color: cfg.color }}
            >
              <span className='inline-flex items-center gap-1.5 text-[13px] leading-none font-semibold'>
                <Icon size={13} />
                {t(record?.status_text) || t(cfg.text)}
              </span>
            </Tag>
          );
        },
      },
      {
        title: <HeaderText>{t('用户ID')}</HeaderText>,
        dataIndex: 'user_id',
        key: 'user_id',
        width: 110,
        render: (v) => `${v === 0 ? '-' : v}`,
      },
      {
        title: <HeaderText>{t('系统订单号')}</HeaderText>,
        dataIndex: 'local_id',
        key: 'local_id',
        width: 100,
        render: (v) => {
          if (v === 0 || v === '0' || v === null || typeof v === 'undefined')
            return '-';
          return v;
        },
      },
      {
        title: <HeaderText>{t('商户订单号')}</HeaderText>,
        dataIndex: 'merchant_trade_no',
        key: 'merchant_trade_no',
        width: 220,
        render: (v) => v || '-',
      },
      {
        title: <HeaderText>{t('充值金额')}</HeaderText>,
        dataIndex: 'amount_text',
        key: 'amount_text',
        width: 110,
        render: (v, record) => `${record.local_currency} ${v ?? 0}`,
      },
      {
        title: <HeaderText>{t('支付渠道')}</HeaderText>,
        dataIndex: 'payment_method_text',
        key: 'payment_method_text',
        width: 145,
        render: (text, record) => {
          const isStripe =
            record?.payment_method === PAYMENT_METHODS.stripe ||
            text === 'Stripe';
          const isCrypto =
            record?.payment_method === PAYMENT_METHODS.crypto ||
            text === 'Crypto' ||
            text === '加密货币';
          const isAlipay =
            record?.payment_method === PAYMENT_METHODS.alipay ||
            text === '支付宝';
          if (isCrypto) {
            return (
              <span className='inline-flex items-center gap-2 text-[16px] leading-[22px] text-[#475569] font-medium'>
                <Coins size={16} className='text-[#F59E0B]' />
                {t(text) || t('加密货币')}
              </span>
            );
          }
          if (isAlipay) {
            return (
              <span className='inline-flex items-center gap-2 text-[16px] leading-[22px] text-[#475569] font-medium'>
                <SiAlipay size={16} className='text-[#1677FF]' />
                {t(text) || t('支付宝')}
              </span>
            );
          }
          return (
            <span className='inline-flex items-center gap-2 text-[16px] leading-[22px] text-[#475569] font-medium'>
              {isStripe ? (
                <img src={stripeImg} alt='Stripe' className='w-4 h-4' />
              ) : (
                <img src={weChatImg} alt='微信支付' className='w-4 h-4' />
              )}
              {t(text) || (isStripe ? t('Stripe') : t('微信支付'))}
            </span>
          );
        },
      },
      {
        title: <HeaderText>{t('支付时间')}</HeaderText>,
        dataIndex: 'trade_time',
        key: 'trade_time',
        width: 170,
        render: (v) => v || '-',
      },
      {
        title: <HeaderText>{t('订单类型')}</HeaderText>,
        dataIndex: 'local_type',
        key: 'local_type',
        width: 80,
        render: (localType) => renderLocalType(localType, t),
      },
      {
        title: <HeaderText>{t('详情')}</HeaderText>,
        key: 'action',
        align: 'right',
        width: 110,
        render: (_, record) => (
          <button
            className='text-[color:var(--theme-primary)] text-[14px] font-medium'
            onClick={() => {
              setActiveRow(record);
              setDetailVisible(true);
            }}
          >
            {t('查看详情')}
          </button>
        ),
      },
    ],
    [t],
  );

  const start = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const end = Math.min(page * pageSize, total);

  return (
    <div className='min-h-full bg-[#F8FAFC] dark:bg-slate-900'>
      <div className='dark:bg-slate-800'>
        <h1 className='text-[30px] leading-[45px] font-semibold text-[#475569] dark:text-slate-100'>
          {t('支付对账')}
        </h1>
        <Text className='!text-[16px] !text-[#94A3B8]'>
          {t('系统账单与支付渠道的账单对比核对')}
        </Text>

        <div className='mt-4 flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between'>
          <div className='flex flex-wrap items-center gap-4'>
            <div className='inline-flex items-center rounded-[16px] bg-[#fff] p-[5px] gap-[6px] dark:bg-slate-900'>
              {PAYMENT_METHOD_TABS.map((tab) => {
                const active = channelTab === tab.key;
                return (
                  <Button
                    key={tab.key}
                    theme='borderless'
                    onClick={() => setChannelTab(tab.key)}
                    style={{
                      minWidth: 108,
                      height: 40,
                      borderRadius: 10,
                      color: active ? '#082E1A' : '#64748B',
                      background: active ? '#F8FAFC' : 'transparent',
                      fontSize: 14,
                      fontWeight: active ? 600 : 500,
                    }}
                  >
                    {t(tab.label)}
                  </Button>
                );
              })}
            </div>
            <DatePicker
              size='large'
              type='date'
              insetLabel={t('日期选择')}
              value={billDate}
              onChange={(value) => setBillDate(value || null)}
            />
          </div>

          <div className='flex flex-wrap items-center gap-3'>
            {/* <span className='inline-flex items-center gap-2 text-[14px] font-medium text-slate-600 dark:text-slate-300'>
              {t('今日对账状态：')}
              <span className='inline-flex items-center gap-1.5 font-semibold' style={{ color: '#06CE73' }}>
                <span className='inline-flex w-4 h-4 items-center justify-center rounded-full' style={{ border: '1px solid #A7EBC8', background: '#ECFBF3' }}>
                  <span className='block w-2 h-2 rounded-full' style={{ background: '#06CE73' }} />
                </span>
                {t('已完成')}
              </span>
            </span> */}
            <Button
              icon={<IconRefresh />}
              size='small'
              loading={statLoading || listLoading}
              onClick={runSyncBill}
              style={{
                height: 40,
                padding: '0 16px',
                borderRadius: 10,
                background: '#DDF7FA',
                borderColor: 'var(--theme-primary-20)',
                borderWidth: 1,
                color: 'var(--theme-primary)',
                fontWeight: 600,
                fontSize: 14,
              }}
            >
              {t('立即同步账单')}
            </Button>
          </div>
        </div>
      </div>

      <div className='grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-5 gap-4 mt-8'>
        {summaryCards.map((card) => {
          const Icon = card.icon;
          return (
            <div
              key={card.key}
              className={`min-h-[148px] rounded-2xl border p-5 ${cardClass[card.tone]}`}
              style={
                card.tone.includes('danger')
                  ? { borderColor: '#FF4D4F', background: '#FDF2F2' }
                  : undefined
              }
            >
              <div className='flex items-center justify-between'>
                <span
                  className='text-[14px] font-semibold'
                  style={{
                    color: card.tone.includes('danger') ? '#FF4D4F' : '#94A3B8',
                  }}
                >
                  {t(card.title)}
                </span>
                <span
                  className='w-9 h-9 rounded-xl flex items-center justify-center'
                  style={
                    card.tone.includes('danger')
                      ? { background: '#F8DCDD', color: '#FF4D4F' }
                      : card.key === 'total'
                        ? { background: '#E8F0FF', color: '#3B82F6' }
                        : { background: '#DFF2E8', color: '#09CC73' }
                  }
                >
                  {card.tone.includes('danger') ? (
                    <span className='text-[16px] leading-none font-bold'>
                      {card.dangerIconText}
                    </span>
                  ) : (
                    <Icon size={20} style={{ color: 'currentColor' }} />
                  )}
                </span>
              </div>
              <div
                className='mt-5 text-[24px] leading-[38px] font-[900]'
                style={{
                  color: card.tone.includes('danger') ? '#FF4D4F' : '#475569',
                }}
              >
                {statLoading ? '--' : card.value}{' '}
                <span
                  className='text-[12px] font-[700]'
                  style={{
                    color: card.tone.includes('danger') ? '#FF4D4F' : '#64748B',
                  }}
                >
                  {t(card.unit)}
                </span>
              </div>
              {card.desc ? (
                <div
                  className={`mt-3 inline-flex items-center gap-1 text-[12px] ${card.tone === 'danger' ? 'text-[#FF4D4F]' : card.tone === 'success' ? 'text-[#09CC73]' : 'text-[#64748B]'}`}
                >
                  {card.key === 'matched' ? <CircleCheckBig size={12} /> : null}
                  {card.key === 'abnormal' ? <CircleAlert size={12} /> : null}
                  {t(card.desc)}
                </div>
              ) : null}
            </div>
          );
        })}
      </div>

      <div className='rounded-2xl bg-white dark:bg-slate-800 p-8 mt-8'>
        <div className='mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
          <h2 className='text-[20px] leading-[36px] font-[500] text-[#475569] dark:text-slate-100'>
            {t('对账结果明细列表')}
          </h2>
          <div className='flex items-center gap-2'>
            <Input
              prefix={<IconSearch />}
              placeholder={t('搜索用户ID...')}
              value={keywordInput}
              onChange={(v) => setKeywordInput(v)}
              onEnterPress={() => {
                setPage(1);
                setKeyword((keywordInput || '').trim());
              }}
              style={{ width: isMobile ? 180 : 220 }}
              size='large'
              showClear
              onClear={() => {
                setKeywordInput('');
                setPage(1);
                setKeyword('');
              }}
            />
            {/* <Button icon={<IconFilter />} size='small' type='tertiary'>
              {t('高级筛选')}
            </Button> */}
          </div>
        </div>

        <div className='overflow-x-auto'>
          <div className='min-w-[1200px]'>
            <Table
              columns={columns}
              dataSource={rows}
              rowKey='id'
              pagination={false}
              loading={listLoading}
            />
          </div>
        </div>

        <div className='mt-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-end'>
          <Pagination
            total={total}
            hideOnSinglePage
            onPageChange={setPage}
          />
        </div>
      </div>

      <ReconciliationDetailSheet
        visible={detailVisible}
        onClose={() => setDetailVisible(false)}
        row={activeRow}
      />
    </div>
  );
};

export default PayReconciliationList;
