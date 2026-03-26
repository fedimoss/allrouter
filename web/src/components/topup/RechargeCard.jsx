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

import React, { useEffect, useRef, useState, useMemo } from 'react';
import {
  Typography,
  Tag,
  Card,
  Button,
  Banner,
  Skeleton,
  Form,
  Space,
  Spin,
  Tooltip,
  Tabs,
  TabPane,
  Table,
  Badge,
  Input,
  Empty,
  Toast,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { SiAlipay, SiWechat, SiStripe } from 'react-icons/si';
import {
  CreditCard,
  Coins,
  Wallet,
  TrendingUp,
  Sparkles,
  Bell,
  History,
  CheckCircle,
} from 'lucide-react';
import { IconGift, IconSearch } from '@douyinfe/semi-icons';
import { useMinimumLoadingTime } from '../../hooks/common/useMinimumLoadingTime';
import { getCurrencyConfig } from '../../helpers/render';
import { API, timestamp2string } from '../../helpers';
import SubscriptionPlansCard from './SubscriptionPlansCard';

const { Text } = Typography;

// 状态映射配置
const STATUS_CONFIG = {
  success: { type: 'success', key: '成功' },
  pending: { type: 'warning', key: '待支付' },
  failed: { type: 'danger', key: '失败' },
  expired: { type: 'danger', key: '已过期' },
};

// 支付方式映射
const PAYMENT_METHOD_MAP = {
  stripe: 'Stripe',
  creem: 'Creem',
  waffo: 'Waffo',
  alipay: '支付宝',
  wxpay: '微信',
  redemptionCode: '兑换码',
  redemption_code: '兑换码',
};

const RechargeCard = ({
  t,
  enableOnlineTopUp,
  enableStripeTopUp,
  enableCreemTopUp,
  creemProducts,
  creemPreTopUp,
  presetAmounts,
  selectedPreset,
  selectPresetAmount,
  formatLargeNumber,
  priceRatio,
  topUpCount,
  minTopUp,
  renderQuotaWithAmount,
  getAmount,
  setTopUpCount,
  setSelectedPreset,
  renderAmount,
  amountLoading,
  payMethods,
  preTopUp,
  paymentLoading,
  payWay,
  redemptionCode,
  setRedemptionCode,
  topUp,
  isSubmitting,
  topUpLink,
  openTopUpLink,
  userState,
  renderQuota,
  statusLoading,
  topupInfo,
  enableWaffoTopUp,
  waffoTopUp,
  waffoPayMethods,
  subscriptionLoading = false,
  subscriptionPlans = [],
  billingPreference,
  onChangeBillingPreference,
  activeSubscriptions = [],
  allSubscriptions = [],
  reloadSubscriptionSelf,
}) => {
  const onlineFormApiRef = useRef(null);
  const redeemFormApiRef = useRef(null);
  const initialTabSetRef = useRef(false);
  const showAmountSkeleton = useMinimumLoadingTime(amountLoading);
  const [activeTab, setActiveTab] = useState('topup');
  const shouldShowSubscription =
    !subscriptionLoading && subscriptionPlans.length > 0;

  // 充值记录相关状态
  const [historyLoading, setHistoryLoading] = useState(false);
  const [topups, setTopups] = useState([]);
  const [historyTotal, setHistoryTotal] = useState(0);
  const [historyPage, setHistoryPage] = useState(1);
  const [historyPageSize, setHistoryPageSize] = useState(10);
  const [historyKeyword, setHistoryKeyword] = useState('');

  useEffect(() => {
    if (initialTabSetRef.current) return;
    if (subscriptionLoading) return;
    setActiveTab(shouldShowSubscription ? 'subscription' : 'topup');
    initialTabSetRef.current = true;
  }, [shouldShowSubscription, subscriptionLoading]);

  useEffect(() => {
    if (!shouldShowSubscription && activeTab !== 'topup') {
      setActiveTab('topup');
    }
  }, [shouldShowSubscription, activeTab]);

  // 加载充值记录
  const loadTopups = async (currentPage, currentPageSize) => {
    setHistoryLoading(true);
    try {
      const qs =
        `p=${currentPage}&page_size=${currentPageSize}` +
        (historyKeyword
          ? `&keyword=${encodeURIComponent(historyKeyword)}`
          : '');
      const res = await API.get(`/api/user/topup/self?${qs}`);
      const { success, message, data } = res.data;
      if (success) {
        setTopups(data.items || []);
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
    loadTopups(historyPage, historyPageSize);
  }, [historyPage, historyPageSize, historyKeyword]);

  const handleHistoryPageChange = (currentPage) => {
    setHistoryPage(currentPage);
  };

  const handleHistoryPageSizeChange = (currentPageSize) => {
    setHistoryPageSize(currentPageSize);
    setHistoryPage(1);
  };

  const handleHistoryKeywordChange = (value) => {
    setHistoryKeyword(value);
    setHistoryPage(1);
  };

  const renderStatusBadge = (status) => {
    const config = STATUS_CONFIG[status] || { type: 'primary', key: status };
    return (
      <span className='flex items-center gap-2'>
        <Badge dot type={config.type} />
        <span>{t(config.key)}</span>
      </span>
    );
  };

  const renderPaymentMethod = (pm) => {
    const displayName = PAYMENT_METHOD_MAP[pm];
    return <Text>{displayName ? t(displayName) : pm || '-'}</Text>;
  };

  const getBizType = (record) => {
    if (record?.biz_type) return record.biz_type;
    const tradeNo = (record?.trade_no || '').toLowerCase();
    return Number(record?.amount || 0) === 0 && tradeNo.startsWith('sub')
      ? 'subscription'
      : 'payment';
  };

  const isSubscriptionTopup = (record) =>
    getBizType(record) === 'subscription';

  const historyColumns = useMemo(() => {
    const cols = [
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
        render: (amount, record) => {
          if (isSubscriptionTopup(record)) {
            return (
              <Tag color='purple' shape='circle' size='small'>
                {t('订阅套餐')}
              </Tag>
            );
          }
          return (
            <span className='flex items-center gap-1'>
              <Coins size={16} />
              <Text>{amount}</Text>
            </span>
          );
        },
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
                : '¥';
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
        render: renderStatusBadge,
      },
    ];

    cols.push({
      title: t('创建时间'),
      dataIndex: 'create_time',
      key: 'create_time',
      render: (time) => timestamp2string(time),
    });
    return cols;
  }, [t]);

  const topupContent = (
    <div className='space-y-6'>
      {/* 在线充值表单 */}
      {statusLoading ? (
        <div className='py-8 flex justify-center'>
          <Spin size='large' />
        </div>
      ) : enableOnlineTopUp ||
        enableStripeTopUp ||
        enableCreemTopUp ||
        enableWaffoTopUp ? (
        <Form
          getFormApi={(api) => (onlineFormApiRef.current = api)}
          initValues={{ topUpCount: topUpCount }}
        >
          <div className='space-y-6'>
            {(enableOnlineTopUp || enableStripeTopUp || enableWaffoTopUp) && (
              <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
                <div>
                  <Form.InputNumber
                    field='topUpCount'
                    label={t('充值数量')}
                    disabled={
                      !enableOnlineTopUp &&
                      !enableStripeTopUp &&
                      !enableWaffoTopUp
                    }
                    placeholder={
                      t('充值数量，最低 ') + renderQuotaWithAmount(minTopUp)
                    }
                    value={topUpCount}
                    min={minTopUp}
                    max={999999999}
                    step={1}
                    precision={0}
                    onChange={async (value) => {
                      if (value && value >= 1) {
                        setTopUpCount(value);
                        setSelectedPreset(null);
                        await getAmount(value);
                      }
                    }}
                    onBlur={(e) => {
                      const value = parseInt(e.target.value);
                      if (!value || value < 1) {
                        setTopUpCount(1);
                        getAmount(1);
                      }
                    }}
                    formatter={(value) => (value ? `${value}` : '')}
                    parser={(value) =>
                      value ? parseInt(value.replace(/[^\d]/g, '')) : 0
                    }
                    extraText={
                      <Skeleton
                        loading={showAmountSkeleton}
                        active
                        placeholder={
                          <Skeleton.Title
                            style={{
                              width: 120,
                              height: 20,
                              borderRadius: 6,
                            }}
                          />
                        }
                      >
                        <Text type='secondary' className='text-red-600'>
                          {t('实付金额：')}
                          <span style={{ color: 'red' }}>
                            {renderAmount()}
                          </span>
                        </Text>
                      </Skeleton>
                    }
                    style={{ width: '100%' }}
                  />
                </div>
                {payMethods &&
                  payMethods.filter((m) => m.type !== 'waffo').length > 0 && (
                    <div>
                      <Form.Slot label={t('选择支付方式')}>
                        <Space wrap>
                          {payMethods
                            .filter((m) => m.type !== 'waffo')
                            .map((payMethod) => {
                              const minTopupVal =
                                Number(payMethod.min_topup) || 0;
                              const isStripe = payMethod.type === 'stripe';
                              const disabled =
                                (!enableOnlineTopUp && !isStripe) ||
                                (!enableStripeTopUp && isStripe) ||
                                minTopupVal > Number(topUpCount || 0);

                              const buttonEl = (
                                <Button
                                  key={payMethod.type}
                                  theme='outline'
                                  type='tertiary'
                                  onClick={() => preTopUp(payMethod.type)}
                                  disabled={disabled}
                                  loading={
                                    paymentLoading &&
                                    payWay === payMethod.type
                                  }
                                  icon={
                                    payMethod.type === 'alipay' ? (
                                      <SiAlipay size={18} color='#1677FF' />
                                    ) : payMethod.type === 'wxpay' ? (
                                      <SiWechat size={18} color='#07C160' />
                                    ) : payMethod.type === 'stripe' ? (
                                      <SiStripe size={18} color='#635BFF' />
                                    ) : (
                                      <CreditCard
                                        size={18}
                                        color={
                                          payMethod.color ||
                                          'var(--semi-color-text-2)'
                                        }
                                      />
                                    )
                                  }
                                  className='!rounded-lg !px-4 !py-2'
                                >
                                  {payMethod.name}
                                </Button>
                              );

                              return disabled &&
                                minTopupVal > Number(topUpCount || 0) ? (
                                <Tooltip
                                  content={
                                    t('此支付方式最低充值金额为') +
                                    ' ' +
                                    minTopupVal
                                  }
                                  key={payMethod.type}
                                >
                                  {buttonEl}
                                </Tooltip>
                              ) : (
                                <React.Fragment key={payMethod.type}>
                                  {buttonEl}
                                </React.Fragment>
                              );
                            })}
                        </Space>
                      </Form.Slot>
                    </div>
                  )}
              </div>
            )}

            {/* 选择充值额度 */}
            {(enableOnlineTopUp || enableStripeTopUp || enableWaffoTopUp) && (
              <Form.Slot
                label={
                  <div className='flex items-center gap-2'>
                    <span>{t('选择充值额度')}</span>
                    {(() => {
                      const { symbol, rate, type } = getCurrencyConfig();
                      if (type === 'USD') return null;

                      return (
                        <span
                          style={{
                            color: 'var(--semi-color-text-2)',
                            fontSize: '12px',
                            fontWeight: 'normal',
                          }}
                        >
                          (1 $ = {rate.toFixed(2)} {symbol})
                        </span>
                      );
                    })()}
                  </div>
                }
              >
                <div className='grid grid-cols-2 md:grid-cols-3 gap-3'>
                  {presetAmounts.map((preset, index) => {
                    const discount =
                      preset.discount ||
                      topupInfo?.discount?.[preset.value] ||
                      1.0;
                    const originalPrice = preset.value * priceRatio;
                    const discountedPrice = originalPrice * discount;
                    const hasDiscount = discount < 1.0;
                    const actualPay = discountedPrice;
                    const save = originalPrice - discountedPrice;

                    const { symbol, rate, type } = getCurrencyConfig();
                    const statusStr = localStorage.getItem('status');
                    let usdRate = 7;
                    try {
                      if (statusStr) {
                        const s = JSON.parse(statusStr);
                        usdRate = s?.usd_exchange_rate || 7;
                      }
                    } catch (e) {}

                    let displayValue = preset.value;
                    let displayActualPay = actualPay;
                    let displaySave = save;

                    if (type === 'USD') {
                      displayActualPay = actualPay / usdRate;
                      displaySave = save / usdRate;
                    } else if (type === 'CNY') {
                      displayValue = preset.value * usdRate;
                    } else if (type === 'CUSTOM') {
                      displayValue = preset.value * rate;
                      displayActualPay = (actualPay / usdRate) * rate;
                      displaySave = (save / usdRate) * rate;
                    }

                    return (
                      <button
                        type='button'
                        key={index}
                        className={`border rounded-lg p-3 text-left transition-all hover:border-cyan-500 hover:bg-cyan-50 dark:hover:bg-cyan-900/20 ${
                          selectedPreset === preset.value
                            ? 'border-cyan-500 bg-cyan-50 dark:bg-cyan-900/20'
                            : 'border-slate-200 dark:border-slate-700'
                        }`}
                        onClick={() => {
                          selectPresetAmount(preset);
                          onlineFormApiRef.current?.setValue(
                            'topUpCount',
                            preset.value,
                          );
                        }}
                      >
                        <p className='font-bold text-slate-800 dark:text-white flex items-center gap-1'>
                          {formatLargeNumber(displayValue)} {symbol}
                          {hasDiscount && (
                            <Tag
                              style={{ marginLeft: 4 }}
                              color='green'
                              size='small'
                            >
                              {t('折').includes('off')
                                ? (
                                    (1 - parseFloat(discount)) *
                                    100
                                  ).toFixed(1)
                                : (discount * 10).toFixed(1)}
                              {t('折')}
                            </Tag>
                          )}
                        </p>
                        <p className='text-xs text-slate-500 mt-1'>
                          {t('实付')} {symbol}
                          {displayActualPay.toFixed(2)}
                          {hasDiscount &&
                            ` · ${t('节省')} ${symbol}${displaySave.toFixed(2)}`}
                        </p>
                      </button>
                    );
                  })}
                </div>
              </Form.Slot>
            )}

            {/* Waffo 充值区域 */}
            {enableWaffoTopUp &&
              waffoPayMethods &&
              waffoPayMethods.length > 0 && (
                <Form.Slot label={t('Waffo 充值')}>
                  <Space wrap>
                    {waffoPayMethods.map((method, index) => (
                      <Button
                        key={index}
                        theme='outline'
                        type='tertiary'
                        onClick={() => waffoTopUp(index)}
                        loading={paymentLoading}
                        icon={
                          method.icon ? (
                            <img
                              src={method.icon}
                              alt={method.name}
                              style={{
                                width: 36,
                                height: 36,
                                objectFit: 'contain',
                              }}
                            />
                          ) : (
                            <CreditCard
                              size={18}
                              color='var(--semi-color-text-2)'
                            />
                          )
                        }
                        className='!rounded-lg !px-4 !py-2'
                      >
                        {method.name}
                      </Button>
                    ))}
                  </Space>
                </Form.Slot>
              )}

            {/* Creem 充值区域 */}
            {enableCreemTopUp && creemProducts.length > 0 && (
              <Form.Slot label={t('Creem 充值')}>
                <div className='grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3'>
                  {creemProducts.map((product, index) => (
                    <Card
                      key={index}
                      onClick={() => creemPreTopUp(product)}
                      className='cursor-pointer !rounded-2xl transition-all hover:shadow-md border-gray-200 hover:border-gray-300'
                      bodyStyle={{ textAlign: 'center', padding: '16px' }}
                    >
                      <div className='font-medium text-lg mb-2'>
                        {product.name}
                      </div>
                      <div className='text-sm text-gray-600 mb-2'>
                        {t('充值额度')}: {product.quota}
                      </div>
                      <div className='text-lg font-semibold text-blue-600'>
                        {product.currency === 'EUR' ? '€' : '$'}
                        {product.price}
                      </div>
                    </Card>
                  ))}
                </div>
              </Form.Slot>
            )}
          </div>
        </Form>
      ) : (
        <Banner
          type='info'
          description={t(
            '管理员未开启在线充值功能，请联系管理员开启或使用兑换码充值。',
          )}
          className='!rounded-xl'
          closeIcon={null}
        />
      )}

      {/* 底部信息区 */}
      <div className='pt-5 border-t border-slate-100 dark:border-slate-800'>
        <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
          <div className='rounded-lg bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 p-3'>
            <p className='text-xs text-slate-500 dark:text-slate-400 mb-1'>
              {t('到账时效')}
            </p>
            <p className='text-sm font-medium text-slate-700 dark:text-slate-300'>
              {t('微信/Stripe：通常 1-3 分钟内到账')}
            </p>
          </div>
          <div className='rounded-lg bg-cyan-50 dark:bg-cyan-900/20 border border-cyan-100 dark:border-cyan-800 p-3'>
            <p className='text-xs text-cyan-700 dark:text-cyan-300 mb-1'>
              {t('温馨提示')}
            </p>
            <p className='text-sm font-medium text-cyan-800 dark:text-cyan-200'>
              {t('大额档位可享折扣，建议按需统一充值更划算')}
            </p>
          </div>
        </div>
      </div>

      {/* 兑换码充值 */}
      {/* <div className='pt-6 border-t border-slate-100 dark:border-slate-800'>
        <h3 className='text-sm font-medium text-slate-700 dark:text-slate-300 mb-3'>
          {t('兑换码充值')}
        </h3>
        <Form
          getFormApi={(api) => (redeemFormApiRef.current = api)}
          initValues={{ redemptionCode: redemptionCode }}
        >
          <Form.Input
            field='redemptionCode'
            noLabel={true}
            placeholder={t('请输入兑换码')}
            value={redemptionCode}
            onChange={(value) => setRedemptionCode(value)}
            prefix={<IconGift />}
            suffix={
              <div className='flex items-center gap-2'>
                <Button
                  type='primary'
                  theme='solid'
                  onClick={topUp}
                  loading={isSubmitting}
                >
                  {t('兑换额度')}
                </Button>
              </div>
            }
            showClear
            style={{ width: '100%' }}
            extraText={
              topUpLink && (
                <Text type='tertiary'>
                  {t('在找兑换码？')}
                  <Text
                    type='secondary'
                    underline
                    className='cursor-pointer'
                    onClick={openTopUpLink}
                  >
                    {t('购买兑换码')}
                  </Text>
                </Text>
              )
            }
          />
        </Form>
      </div> */}
    </div>
  );

  return (
    <div className='space-y-6'>
      {/* 页面标题 */}
      <div className='mb-2 text-center md:text-left'>
        <h1 className='text-3xl font-bold text-slate-900 dark:text-white mb-2 flex items-center justify-center md:justify-start'>
          <Wallet className='w-8 h-8 mr-3 text-cyan-600' />
          {t('钱包管理')}
        </h1>
        <p className='text-slate-500 dark:text-slate-400 max-w-2xl'>
          {t('管理您的账户余额与充值，设置额度预警，确保 API 服务不中断。')}
        </p>
      </div>

      {/* 顶部概览卡片 */}
      <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
        {/* 当前余额 */}
        <div className='bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 p-5'>
          <div className='flex items-center justify-between mb-2'>
            <h3 className='text-sm font-medium text-slate-500 dark:text-slate-400'>
              {t('当前余额')}
            </h3>
            <Wallet className='w-5 h-5 text-cyan-600' />
          </div>
          <p className='text-2xl font-bold text-slate-800 dark:text-white'>
            {renderQuota(userState?.user?.quota)}
          </p>
          <p className='text-xs text-green-600 mt-1'>
            &#10003; {t('可用额度')}
          </p>
        </div>

        {/* 历史消耗 */}
        <div className='bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 p-5'>
          <div className='flex items-center justify-between mb-2'>
            <h3 className='text-sm font-medium text-slate-500 dark:text-slate-400'>
              {t('历史消耗')}
            </h3>
            <TrendingUp className='w-5 h-5 text-green-500' />
          </div>
          <p className='text-2xl font-bold text-slate-800 dark:text-white'>
            {renderQuota(userState?.user?.used_quota)}
          </p>
          <p className='text-xs text-slate-400 mt-1'>
            {t('请求次数')}：{userState?.user?.request_count || 0}
          </p>
        </div>
      </div>

      {/* 主体内容区 */}
      <div className='grid grid-cols-1 lg:grid-cols-1 gap-6'>
        {/* 左侧：充值模块 */}
        <div className='lg:col-span-2 space-y-6'>
          <div className='bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 p-6 flex flex-col'>
            <h2 className='text-lg font-bold text-slate-800 dark:text-white flex items-center mb-4'>
              <CreditCard className='w-5 h-5 mr-2 text-cyan-600' />
              {t('账户充值')}
            </h2>

            {shouldShowSubscription ? (
              <Tabs
                type='card'
                activeKey={activeTab}
                onChange={setActiveTab}
              >
                <TabPane
                  tab={
                    <div className='flex items-center gap-2'>
                      <Sparkles size={16} />
                      {t('订阅套餐')}
                    </div>
                  }
                  itemKey='subscription'
                >
                  <div className='py-2'>
                    <SubscriptionPlansCard
                      t={t}
                      loading={subscriptionLoading}
                      plans={subscriptionPlans}
                      payMethods={payMethods}
                      enableOnlineTopUp={enableOnlineTopUp}
                      enableStripeTopUp={enableStripeTopUp}
                      enableCreemTopUp={enableCreemTopUp}
                      billingPreference={billingPreference}
                      onChangeBillingPreference={onChangeBillingPreference}
                      activeSubscriptions={activeSubscriptions}
                      allSubscriptions={allSubscriptions}
                      reloadSubscriptionSelf={reloadSubscriptionSelf}
                      withCard={false}
                    />
                  </div>
                </TabPane>
                <TabPane
                  tab={
                    <div className='flex items-center gap-2'>
                      <Wallet size={16} />
                      {t('额度充值')}
                    </div>
                  }
                  itemKey='topup'
                >
                  <div className='py-2'>{topupContent}</div>
                </TabPane>
              </Tabs>
            ) : (
              topupContent
            )}
          </div>
        </div>

        {/* 右侧：额度预警 & 小贴士 */}
        <div className='space-y-6' style={{display:'none'}}>
          {/* 额度预警 */}
          <div className='bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 p-6'>
            <h2 className='text-lg font-bold text-slate-800 dark:text-white mb-4 flex items-center'>
              <Bell className='w-5 h-5 mr-2 text-cyan-600' />
              {t('额度预警')}
            </h2>
            <p className='text-sm text-slate-600 dark:text-slate-400 mb-4'>
              {t('当余额低于此值时，系统将发送通知提醒。')}
            </p>
            <div className='flex items-center space-x-2 mb-4'>
              <input
                type='number'
                defaultValue='1.00'
                className='w-24 border border-slate-300 dark:border-slate-600 rounded-lg px-3 py-2 focus:ring-2 focus:ring-cyan-500 outline-none bg-white dark:bg-slate-800 text-slate-800 dark:text-white'
              />
              <span className='text-slate-500 dark:text-slate-400'>
                {t('美元')}
              </span>
            </div>
            <div className='flex items-center space-x-2 mb-4'>
              <input
                type='checkbox'
                defaultChecked
                className='rounded text-cyan-600 focus:ring-cyan-500'
              />
              <label className='text-sm text-slate-700 dark:text-slate-300'>
                {t('邮件通知')}
              </label>
            </div>
            <div className='flex items-center space-x-2'>
              <input
                type='checkbox'
                className='rounded text-cyan-600 focus:ring-cyan-500'
              />
              <label className='text-sm text-slate-700 dark:text-slate-300'>
                Webhook {t('通知')}
              </label>
            </div>
            <button className='w-full mt-4 py-2 bg-cyan-600 text-white rounded-lg font-medium hover:bg-cyan-700 transition-colors'>
              {t('保存设置')}
            </button>
          </div>

          {/* 小贴士 */}
          <div className='bg-gradient-to-br from-purple-50 to-pink-50 dark:from-purple-900/20 dark:to-pink-900/20 rounded-xl border border-purple-100 dark:border-purple-800 p-6'>
            <h3 className='font-bold text-slate-800 dark:text-white mb-3'>
              {t('小贴士')}
            </h3>
            <ul className='space-y-2 text-sm text-slate-600 dark:text-slate-400'>
              <li className='flex items-start'>
                <CheckCircle className='w-4 h-4 text-green-500 mr-2 mt-0.5 flex-shrink-0' />
                <span>{t('大额充值享折扣，最高节省 20%')}</span>
              </li>
              <li className='flex items-start'>
                <CheckCircle className='w-4 h-4 text-green-500 mr-2 mt-0.5 flex-shrink-0' />
                <span>{t('邀请好友得奖励，每成功邀一人赠 $5')}</span>
              </li>
              <li className='flex items-start'>
                <CheckCircle className='w-4 h-4 text-green-500 mr-2 mt-0.5 flex-shrink-0' />
                <span>{t('设置自动充值，避免服务中断')}</span>
              </li>
            </ul>
          </div>
        </div>
      </div>

      {/* 充值记录 */}
      <div className='bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 p-6'>
        <div className='flex items-center justify-between mb-4'>
          <h2 className='text-lg font-bold text-slate-800 dark:text-white flex items-center'>
            <History className='w-5 h-5 mr-2 text-cyan-600' />
            {t('充值记录')}
          </h2>
          <Input
            prefix={<IconSearch />}
            placeholder={t('搜索订单号')}
            value={historyKeyword}
            onChange={handleHistoryKeywordChange}
            showClear
            style={{ width: 220 }}
          />
        </div>
        <Table
          columns={historyColumns}
          dataSource={topups}
          loading={historyLoading}
          rowKey='id'
          pagination={{
            currentPage: historyPage,
            pageSize: historyPageSize,
            total: historyTotal,
            showSizeChanger: true,
            pageSizeOpts: [10, 20, 50, 100],
            onPageChange: handleHistoryPageChange,
            onPageSizeChange: handleHistoryPageSizeChange,
          }}
          size='small'
          empty={
            <Empty
              image={
                <IllustrationNoResult
                  style={{ width: 150, height: 150 }}
                />
              }
              darkModeImage={
                <IllustrationNoResultDark
                  style={{ width: 150, height: 150 }}
                />
              }
              description={t('暂无充值记录')}
              style={{ padding: 30 }}
            />
          }
        />
      </div>
    </div>
  );
};

export default RechargeCard;
