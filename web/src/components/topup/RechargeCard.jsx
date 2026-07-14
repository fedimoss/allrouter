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
import { useNavigate } from 'react-router-dom';
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
  Pagination,
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
  History,
  CheckCircle,
  Gift,
  Lightbulb,
  Clock3,
  Copy,
} from 'lucide-react';
import { IconGift, IconSearch } from '@douyinfe/semi-icons';
import { useMinimumLoadingTime } from '../../hooks/common/useMinimumLoadingTime';
import { API, timestamp2string, formatDisplayMoney } from '../../helpers';
import {
  getTopupBizTypeConfig,
  getEffectiveTopupMin,
  isInviteRebateTopup,
  isSubscriptionTopup,
} from '../../helpers/topup';
import SubscriptionPlansCard from './SubscriptionPlansCard';
import CryptoPaymentDrawer from './CryptoPaymentDrawer';
import balanceBgimg from '../../../public/wallet-balance.png';
import dateBgimg from '../../../public/wallet-date.png';
import dollarIcon from '../../../public/icon-dollar.svg';

const { Text } = Typography;

// 状态映射配置
const STATUS_CONFIG = {
  success: {
    type: 'success',
    key: '成功',
    color: 'rgb(10, 130, 54)',
    bgColor: 'green',
  },
  pending: {
    type: 'warning',
    key: '待支付',
    color: 'rgba(253, 184, 120, 1)',
    bgColor: 'orange',
  },
  failed: {
    type: 'danger',
    key: '失败',
    color: 'rgba(255, 107, 107, 1)',
    bgColor: 'red',
  },
  expired: {
    type: 'danger',
    key: '已过期',
    color: 'rgba(255, 107, 107, 1)',
    bgColor: 'red',
  },
};

// 支付方式映射
const PAYMENT_METHOD_MAP = {
  stripe: 'Stripe',
  creem: 'Creem',
  waffo: 'Waffo',
  alipay: '支付宝',
  wxpay: '微信',
  lakala: '微信',
  crypto: '加密货币',
  redemptionCode: '兑换码',
  redemption_code: '兑换码',
};

const EMPTY_TOPUP_GIFT_CONFIG = {
  enabled: false,
  rules: [],
  timed: {
    enabled: false,
    day: 0,
    endTime: 0,
  },
};

const getTopupPromotionCountdown = (endTime, now = Date.now()) => {
  const ended = !endTime || now >= endTime;
  const remainingMilliseconds = ended ? 0 : endTime - now;
  const totalSeconds = Math.max(0, Math.floor(remainingMilliseconds / 1000));

  return {
    ended,
    days: Math.floor(totalSeconds / 86400),
    hours: Math.floor((totalSeconds % 86400) / 3600),
    minutes: Math.floor((totalSeconds % 3600) / 60),
    seconds: totalSeconds % 60,
  };
};

const getTopupGiftEndTime = (timed) => {
  const configuredEndTime = Number(timed?.end_time);
  if (Number.isFinite(configuredEndTime) && configuredEndTime > 0) {
    return configuredEndTime < 1e12
      ? configuredEndTime * 1000
      : configuredEndTime;
  }

  const days = Number(timed?.day);
  if (!Number.isFinite(days) || days <= 0) return 0;

  const startOfToday = new Date();
  startOfToday.setHours(0, 0, 0, 0);
  return startOfToday.getTime() + days * 86400000;
};

const RechargeCard = ({
  t,
  enableOnlineTopUp,
  enableStripeTopUp,
  stripeCurrency,
  displayCurrency,
  enableCreemTopUp,
  creemProducts,
  creemPreTopUp,
  presetAmounts,
  selectedPreset,
  selectPresetAmount,
  formatLargeNumber,
  topUpCount,
  minTopUp,
  renderQuotaWithAmount,
  getAmount,
  getStripeAmount,
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
  const navigate = useNavigate();
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
  const historyPageSize = 10;
  const [historyKeyword, setHistoryKeyword] = useState('');
  const [selectedPayMethod, setSelectedPayMethod] = useState('');
  const [cryptoDrawerVisible, setCryptoDrawerVisible] = useState(false);
  const [topupGiftConfig, setTopupGiftConfig] = useState(
    EMPTY_TOPUP_GIFT_CONFIG,
  );
  const [topupPromotionCountdown, setTopupPromotionCountdown] = useState(() =>
    getTopupPromotionCountdown(0),
  );
  // 当未选择支付方式且仅 Stripe 可用时，回退为 stripe，用于输入框的最低金额计算
  const fallbackInputPaymentType =
    !selectedPayMethod && enableStripeTopUp && !enableOnlineTopUp
      ? 'stripe'
      : selectedPayMethod;
  // 根据支付方式和币种计算实际生效的最低充值金额（Stripe CNY 有额外下限要求）
  const inputMinTopUp = getEffectiveTopupMin({
    paymentType: fallbackInputPaymentType,
    minTopup: minTopUp,
    stripeCurrency,
    fallback: 1,
  });
  // 是否为 Stripe 且有时区币种配置（用于决定输入框 placeholder 的金额展示格式）
  const isStripeCurrencyInput =
    fallbackInputPaymentType === 'stripe' && !!stripeCurrency;
  const stripeCurrencySymbol =
    stripeCurrency?.symbol || (stripeCurrency?.currency === 'CNY' ? '¥' : '$');
  const displayCurrencySymbol =
    displayCurrency?.symbol ||
    (displayCurrency?.currency === 'CNY' ? '¥' : '$');
  const presetCurrencySymbol = stripeCurrency?.symbol || displayCurrencySymbol;
  const topupGiftRules = topupGiftConfig.enabled ? topupGiftConfig.rules : [];
  const hasDynamicGiftPresets = topupGiftRules.length > 0;
  const displayedPresetAmounts = hasDynamicGiftPresets
    ? topupGiftRules.map((rule) => ({
        value: rule.threshold,
        bonus: rule.bonus,
        giftRuleId: rule.id,
      }))
    : presetAmounts;
  const formatTopupGiftAmount = (amount) =>
    `${presetCurrencySymbol} ${Number(Number(amount).toFixed(10)).toString()}`;

  useEffect(() => {
    if (initialTabSetRef.current) return;
    if (subscriptionLoading) return;
    setActiveTab(shouldShowSubscription ? 'topup' : 'subscription');
    initialTabSetRef.current = true;
  }, [shouldShowSubscription, subscriptionLoading]);

  useEffect(() => {
    if (!shouldShowSubscription && activeTab !== 'topup') {
      setActiveTab('topup');
    }
  }, [shouldShowSubscription, activeTab]);

  useEffect(() => {
    let active = true;

    const loadTopupGiftConfig = async () => {
      try {
        const res = await API.get('/api/topup/gift_config');
        const { success, data } = res.data || {};
        if (!active || !success || !data) return;

        const rules = Array.isArray(data.rules)
          ? data.rules
              .map((rule) => ({
                id: rule.id,
                threshold: Number(rule.threshold),
                bonus: Number(rule.bonus),
              }))
              .filter(
                (rule) =>
                  Number.isFinite(rule.threshold) &&
                  rule.threshold > 0 &&
                  Number.isFinite(rule.bonus) &&
                  rule.bonus > 0,
              )
          : [];
        const timed = data.timed || {};

        setTopupGiftConfig({
          enabled: data.enabled === true,
          rules,
          timed: {
            enabled: timed.enabled === true,
            day: Number(timed.day) || 0,
            endTime: getTopupGiftEndTime(timed),
          },
        });
      } catch (error) {
        // Keep the existing preset amounts when gift configuration is unavailable.
      }
    };

    loadTopupGiftConfig();
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (!topupGiftConfig.timed.enabled) return undefined;

    const updateCountdown = () => {
      setTopupPromotionCountdown(
        getTopupPromotionCountdown(topupGiftConfig.timed.endTime),
      );
    };

    updateCountdown();
    const countdownTimer = window.setInterval(updateCountdown, 1000);
    return () => window.clearInterval(countdownTimer);
  }, [topupGiftConfig.timed.enabled, topupGiftConfig.timed.endTime]);

  useEffect(() => {
    if (selectedPayMethod) return;
    const firstMethod = (payMethods || [])
      .filter((m) => m.type !== 'waffo')
      .find((m) => {
        const minTopupVal = Number(m.min_topup) || 0;
        const isStripe = m.type === 'stripe';
        if (
          (!enableOnlineTopUp && !isStripe) ||
          (!enableStripeTopUp && isStripe)
        ) {
          return false;
        }
        return minTopupVal <= Number(topUpCount || 0);
      });

    if (firstMethod?.type) {
      setSelectedPayMethod(firstMethod.type);
      return;
    }

    if (
      enableWaffoTopUp &&
      Array.isArray(waffoPayMethods) &&
      waffoPayMethods.length > 0
    ) {
      setSelectedPayMethod('waffo:0');
    }
  }, [
    selectedPayMethod,
    payMethods,
    enableOnlineTopUp,
    enableStripeTopUp,
    topUpCount,
    enableWaffoTopUp,
    waffoPayMethods,
    stripeCurrency,
  ]);

  const prevInputMinTopUpRef = useRef(inputMinTopUp);
  // 支付方式切换时，根据最低充值金额自动修正当前输入值
  useEffect(() => {
    const prevMin = prevInputMinTopUpRef.current;
    const currentValue = Number(topUpCount || 0);

    if (inputMinTopUp > prevMin) {
      // 最低金额变大了（如切换到 Stripe CNY），当前值低于新下限则拉高
      if (currentValue && currentValue < inputMinTopUp) {
        setTopUpCount(inputMinTopUp);
        setSelectedPreset(null);
        onlineFormApiRef.current?.setValue('topUpCount', inputMinTopUp);
      }
    } else if (inputMinTopUp < prevMin) {
      // 最低金额变小了（如从 Stripe 切换到微信），当前值恰好等于旧下限则重置到新下限
      if (currentValue && currentValue === prevMin) {
        setTopUpCount(inputMinTopUp);
        setSelectedPreset(null);
        onlineFormApiRef.current?.setValue('topUpCount', inputMinTopUp);
      }
    }

    prevInputMinTopUpRef.current = inputMinTopUp;
  }, [inputMinTopUp, setSelectedPreset, setTopUpCount]);

  // 跳转邀请详情
  const toInvitationDetail = () => {
    navigate('/console/invitation');
  };

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

  const handleHistoryKeywordChange = (value) => {
    setHistoryKeyword(value);
    setHistoryPage(1);
  };

  const isPayMethodDisabled = (payMethod) => {
    const minTopupVal = Number(payMethod.min_topup) || 0;
    const isStripe = payMethod.type === 'stripe';
    return (
      (!enableOnlineTopUp && !isStripe) ||
      (!enableStripeTopUp && isStripe) ||
      minTopupVal > Number(topUpCount || 0)
    );
  };

  const renderPayMethodIcon = (payMethod) => {
    if (payMethod.type === 'alipay')
      return <SiAlipay size={20} color='#1677FF' />;
    if (payMethod.type === 'wxpay')
      return <SiWechat size={20} color='#07C160' />;
    if (payMethod.type === 'stripe')
      return <SiStripe size={20} color='#635BFF' />;
    if (payMethod.type === 'crypto')
      return <img src={dollarIcon} alt='crypto' style={{ width: 16 }} />;
    return (
      <CreditCard
        size={20}
        color={payMethod.color || 'var(--semi-color-text-2)'}
      />
    );
  };

  const handlePrimaryTopUp = () => {
    if (!selectedPayMethod) return;

    if (selectedPayMethod.startsWith('waffo:')) {
      const idx = Number(selectedPayMethod.split(':')[1] || 0);
      waffoTopUp(Number.isNaN(idx) ? 0 : idx);
      return;
    }

    if (selectedPayMethod === 'crypto') {
      setCryptoDrawerVisible(true);
      return;
    }

    preTopUp(selectedPayMethod);
  };

  const renderStatusBadge = (status, record) => {
    if (isInviteRebateTopup(record)) {
      return (
        <span className='flex items-center gap-2'>
          <Tag
            color='green'
            style={{ padding: '0 10px', height: 26, lineHeight: '24px' }}
          >
            <span
              className='inline-flex items-center gap-1 font-medium'
              style={{ color: 'rgb(10, 130, 54)' }}
            >
              <CheckCircle size={14} />
              {t('已入账')}
            </span>
          </Tag>
        </span>
      );
    }
    if (!status) {
      return <Text type='tertiary'>-</Text>;
    }
    const config = STATUS_CONFIG[status] || { type: 'primary', key: status };
    return (
      <span className='flex items-center gap-2'>
        <Tag
          color={config.bgColor}
          style={{ padding: '0 6px', height: 22, lineHeight: '22px' }}
        >
          <span style={{ color: config.color }}>{t(config.key)}</span>
        </Tag>
      </span>
    );
  };

  const renderPaymentMethod = (pm) => {
    const displayName = PAYMENT_METHOD_MAP[pm];
    return <Text>{displayName ? t(displayName) : pm || '-'}</Text>;
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

  const historyColumns = useMemo(() => {
    return [
      {
        title: t('流水号'),
        dataIndex: 'trade_no',
        key: 'trade_no',
        render: (text) => <Text>{text}</Text>,
      },
      {
        title: t('账单类型'),
        dataIndex: 'biz_type',
        key: 'biz_type',
        render: (_, record) => renderBizTypeTag(record),
      },
      {
        title: t('充值时间'),
        dataIndex: 'create_time',
        key: 'create_time',
        render: (time) => timestamp2string(time),
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
        render: (amount, record) =>
          isSubscriptionTopup(record) ? (
            <Tag color='purple' shape='circle' size='small'>
              {t('订阅套餐')}
            </Tag>
          ) : isInviteRebateTopup(record) ? (
            <span className='inline-flex items-center gap-1 rounded-full bg-emerald-50 px-3 py-1 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-300'>
              <Gift size={14} />
              <Text strong className='!text-emerald-600 dark:!text-emerald-300'>
                +{amount}
              </Text>
            </span>
          ) : (
            <span className='flex items-center gap-1'>
              <Coins size={16} />
              <Text>{amount}</Text>
            </span>
          ),
      },
      {
        title: t('充值金额'),
        dataIndex: 'money',
        key: 'money',
        render: (money, record) => {
          const normalizedMoney = Number(money || 0);
          if (normalizedMoney <= 0) {
            return <Text type='tertiary'>-</Text>;
          }
          // 优先使用后端返回的币种符号，回退到用户默认展示币种
          const isCrypto = record.payment_method === 'crypto';
          const paySymbol = isCrypto
            ? ''
            : record.display_symbol || displayCurrency?.symbol || '$';
          const suffix = isCrypto ? ' USDT' : '';
          return (
            <Text
              className='text-xl dark:!text-cyan-300'
              style={{
                fontWeight: '800',
                color: 'var(--theme-primary)',
              }}
            >
              {isCrypto
                ? `${normalizedMoney}${suffix}`
                : formatDisplayMoney(money, paySymbol)}
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
      {
        title: t('操作'),
        key: 'action',
        render: (_, record) => (
          <Tooltip content={t('复制')}>
            <Button
              type='tertiary'
              theme='borderless'
              icon={<Copy size={14} style={{ color: '#999' }} />}
              size='small'
              onClick={() => {
                navigator.clipboard.writeText(record?.trade_no || '');
                Toast.success({ content: t('复制成功') });
              }}
            />
          </Tooltip>
        ),
      },
    ];
  }, [displayCurrency?.symbol, t]);

  const topupContent = (
    <div className='space-y-6'>
      {statusLoading ? (
        <div className='py-10 flex justify-center'>
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
              <div className='rounded-2xl 900/60'>
                <Form.InputNumber
                  field='topUpCount'
                  hideButtons
                  label={t('请输入充值金额')}
                  disabled={
                    !enableOnlineTopUp &&
                    !enableStripeTopUp &&
                    !enableWaffoTopUp
                  }
                  placeholder={
                    isStripeCurrencyInput
                      ? t('充值数量，最低 ') +
                        stripeCurrencySymbol +
                        inputMinTopUp
                      : t('充值数量，最低 ') +
                        renderQuotaWithAmount(inputMinTopUp)
                  }
                  className='charge-input'
                  value={topUpCount}
                  min={inputMinTopUp}
                  max={999999999}
                  step={1}
                  precision={0}
                  onChange={async (value) => {
                    if (value && value >= 1) {
                      setTopUpCount(value);
                      setSelectedPreset(null);
                      // 有时区币种配置时，renderAmount 直接用 topUpCount 显示，无需调后端金额接口
                      if (!stripeCurrency) {
                        if (selectedPayMethod === 'stripe' && getStripeAmount) {
                          await getStripeAmount(value);
                        } else {
                          await getAmount(value);
                        }
                      }
                    }
                  }}
                  onBlur={(e) => {
                    // 输入框失焦时校验，无效值回退到最低充值数量
                    const parsed = parseInt(e.target.value);
                    if (!parsed || parsed < inputMinTopUp) {
                      setTopUpCount(inputMinTopUp);
                      setSelectedPreset(null);
                      onlineFormApiRef.current?.setValue(
                        'topUpCount',
                        inputMinTopUp,
                      );
                      // 无时区币种配置时需重新请求金额
                      if (!stripeCurrency) {
                        if (selectedPayMethod === 'stripe' && getStripeAmount) {
                          getStripeAmount(inputMinTopUp);
                        } else {
                          getAmount(inputMinTopUp);
                        }
                      }
                    }
                  }}
                  formatter={(value) => (value ? `${value}` : '')}
                  parser={(value) => {
                    if (!value) return 0;
                    return parseInt(value.replace(/[^\d]/g, '')) || 0;
                  }}
                  extraText={
                    <Skeleton
                      loading={showAmountSkeleton}
                      active
                      placeholder={
                        <Skeleton.Title
                          style={{
                            width: 160,
                            height: 20,
                            borderRadius: 6,
                          }}
                        />
                      }
                    >
                      <Text
                        type='secondary'
                        className='text-slate-600 dark:text-slate-300'
                      >
                        {t('实付金额')}：
                        <span className='text-[color:var(--theme-primary)] font-semibold'>
                          {renderAmount()}
                        </span>
                      </Text>
                    </Skeleton>
                  }
                  style={{ width: '100%' }}
                />

                <Form.Slot
                  label={
                    <div className='flex items-center gap-2'>
                      {/* <span>{t('选择充值额度')}</span> */}
                    </div>
                  }
                >
                  <div className='grid grid-cols-2 md:grid-cols-6 gap-3'>
                    {displayedPresetAmounts.map((preset, index) => {
                      const discount =
                        preset.discount ||
                        topupInfo?.discount?.[preset.value] ||
                        1.0;
                      const hasDiscount = discount < 1.0;

                      const presetButton = (
                        <button
                          type='button'
                          className={`h-12 w-full rounded-xl text-l font-semibold transition-all ${
                            selectedPreset === preset.value
                              ? 'text-[color:var(--theme-primary)] border border-[color:var(--theme-primary)] dark:bg-cyan-900/10 dark:text-[color:var(--theme-primary)]'
                              : 'bg-[#F8FAFC] text-slate-700 dark:bg-gray-800 dark:text-slate-200'
                          }`}
                          onClick={() => {
                            selectPresetAmount(preset);
                            onlineFormApiRef.current?.setValue(
                              'topUpCount',
                              preset.value,
                            );
                          }}
                        >
                          {presetCurrencySymbol} {preset.value}
                          {hasDiscount && (
                            <Tag
                              style={{ marginLeft: 6 }}
                              color='green'
                              size='small'
                            >
                              {t('折')}
                            </Tag>
                          )}
                        </button>
                      );

                      return hasDynamicGiftPresets ? (
                        <Tooltip
                          key={preset.giftRuleId || `${preset.value}-${index}`}
                          position='top'
                          content={
                            <div className='space-y-1'>
                              <div>
                                {t('赠送')}：
                                {formatTopupGiftAmount(preset.bonus)}
                              </div>
                              <div>
                                {t('到账总额')}：
                                {formatTopupGiftAmount(
                                  preset.value + preset.bonus,
                                )}
                              </div>
                            </div>
                          }
                        >
                          {presetButton}
                        </Tooltip>
                      ) : (
                        <React.Fragment key={`${preset.value}-${index}`}>
                          {presetButton}
                        </React.Fragment>
                      );
                    })}
                  </div>
                </Form.Slot>

                {payMethods &&
                  payMethods.filter((m) => m.type !== 'waffo').length > 0 && (
                    <Form.Slot label={t('选择支付方式')}>
                      <div className='grid grid-cols-2 md:grid-cols-4 gap-3'>
                        {payMethods
                          .filter((m) => m.type !== 'waffo')
                          .map((payMethod) => {
                            const disabled = isPayMethodDisabled(payMethod);
                            const selected =
                              selectedPayMethod === payMethod.type;
                            const minTopupVal =
                              Number(payMethod.min_topup) || 0;

                            const card = (
                              <button
                                type='button'
                                key={payMethod.type}
                                disabled={disabled}
                                onClick={() =>
                                  setSelectedPayMethod(payMethod.type)
                                }
                                className={`h-20 rounded-xl border transition-all px-3 ${
                                  selected
                                    ? 'border-[color:var(--theme-primary)] bg-[color:var(--theme-primary-12)] text-[color:var(--theme-primary)] dark:border-[color:var(--theme-primary)] dark:bg-cyan-900/30'
                                    : 'border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900'
                                } ${disabled ? 'opacity-45 cursor-not-allowed' : 'cursor-pointer'}`}
                              >
                                <div className='h-full flex flex-col items-center justify-center gap-2'>
                                  {renderPayMethodIcon(payMethod)}
                                  <span
                                    className={`text-sm font-medium ${selected ? 'text-[color:var(--theme-primary)]' : 'dark:text-slate-200'}`}
                                  >
                                    {t(payMethod.name)}
                                  </span>
                                </div>
                              </button>
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
                                {card}
                              </Tooltip>
                            ) : (
                              <React.Fragment key={payMethod.type}>
                                {card}
                              </React.Fragment>
                            );
                          })}
                      </div>
                    </Form.Slot>
                  )}

                {enableWaffoTopUp &&
                  waffoPayMethods &&
                  waffoPayMethods.length > 0 && (
                    <Form.Slot label={t('Waffo 充值')}>
                      <div className='grid grid-cols-2 md:grid-cols-3 gap-3'>
                        {waffoPayMethods.map((method, index) => {
                          const methodKey = `waffo:${index}`;
                          const selected = selectedPayMethod === methodKey;
                          return (
                            <button
                              type='button'
                              key={methodKey}
                              onClick={() => setSelectedPayMethod(methodKey)}
                              className={`h-20 rounded-xl border transition-all px-3 ${
                                selected
                                  ? 'border-cyan-500 dark:border-cyan-400 dark:bg-cyan-900/30'
                                  : 'border-slate-200 bg-white hover:border-cyan-300 dark:border-slate-700 dark:bg-slate-900 dark:hover:border-cyan-600'
                              }`}
                            >
                              <div className='h-full flex flex-col items-center justify-center gap-2'>
                                {method.icon ? (
                                  <img
                                    src={method.icon}
                                    alt={method.name}
                                    style={{
                                      width: 22,
                                      height: 22,
                                      objectFit: 'contain',
                                    }}
                                  />
                                ) : (
                                  <CreditCard
                                    size={20}
                                    color='var(--semi-color-text-2)'
                                  />
                                )}
                                <span className='text-sm font-medium text-slate-700 dark:text-slate-200'>
                                  {method.name}
                                </span>
                              </div>
                            </button>
                          );
                        })}
                      </div>
                    </Form.Slot>
                  )}

                <Button
                  onClick={handlePrimaryTopUp}
                  disabled={!selectedPayMethod}
                  loading={
                    selectedPayMethod?.startsWith('waffo:')
                      ? paymentLoading
                      : paymentLoading && payWay === selectedPayMethod
                  }
                  className='common-theme !w-full !h-14 !text-base !font-semibold !rounded-xl !border-0 !from-cyan-500 !to-emerald-400 mt-4 mb-2'
                >
                  {t('立即充值')}
                </Button>
              </div>
            )}

            {enableCreemTopUp && creemProducts.length > 0 && (
              <Form.Slot label={t('Creem 充值')}>
                <div className='grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3'>
                  {creemProducts.map((product, index) => (
                    <Card
                      key={index}
                      onClick={() => creemPreTopUp(product)}
                      className='cursor-pointer !rounded-2xl transition-all hover:shadow-md border-gray-200 hover:border-cyan-300 dark:border-slate-700 dark:hover:border-cyan-500'
                      bodyStyle={{ textAlign: 'center', padding: '16px' }}
                    >
                      <div className='font-medium text-lg mb-2'>
                        {product.name}
                      </div>
                      <div className='text-sm text-gray-600 dark:text-slate-300 mb-2'>
                        {t('充值额度')}: {product.quota}
                      </div>
                      <div className='text-lg font-semibold text-cyan-600 dark:text-cyan-400'>
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

      {/* <div className='pt-4 border-t border-slate-100 dark:border-slate-800'>
        <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
          <div className='rounded-xl bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 p-3'>
            <p className='text-xs text-slate-500 dark:text-slate-400 mb-1'>{t('到账时效')}</p>
            <p className='text-sm font-medium text-slate-700 dark:text-slate-300'>
              {t('微信/Stripe：通常 1-3 分钟内到账')}
            </p>
          </div>
          <div className='rounded-xl dark:bg-cyan-900/20 border border-cyan-100 dark:border-cyan-800 p-3'>
            <p className='text-xs text-cyan-700 dark:text-cyan-300 mb-1'>{t('温馨提示')}</p>
            <p className='text-sm font-medium text-cyan-800 dark:text-cyan-200'>
              {t('大额档位可享折扣，建议按需统一充值更划算')}
            </p>
          </div>
        </div>
      </div> */}
    </div>
  );

  return (
    <div className='space-y-5 md:space-y-6'>
      {/* 页面标题 */}
      {/* <div className='mb-1 text-center md:text-left'>
        <h1 className='text-3xl font-bold text-slate-900 dark:text-white mb-2 flex items-center justify-center md:justify-start'>
          <Wallet className='w-8 h-8 mr-3 text-cyan-600 dark:text-cyan-400' />
          {t('钱包管理')}
        </h1>
        <p className='text-slate-500 dark:text-slate-400 max-w-3xl'>
          {t('管理您的账户余额与充值，查看账单记录，确保 API 服务不中断。')}
        </p>
      </div> */}

      {/* 顶部概览卡片 */}
      <div className='grid grid-cols-1 md:grid-cols-3 xl:grid-cols-5 gap-4'>
        <div className='rounded-2xl from-cyan-50 bg-white dark:border-cyan-900/50 dark:from-slate-900 dark:bg-slate-800 dark:via-slate-900 dark:to-slate-950 p-5'>
          <div className='flex items-center justify-between mb-2'>
            <h3 className='text-sm font-medium text-[#94A3B8] dark:text-slate-400'>
              {t('当前余额')}
            </h3>
            <div className=''></div>
          </div>
          <p
            className='text-[24px] text-[#475569] dark:text-cyan-400'
            style={{ fontWeight: '900' }}
          >
            {formatDisplayMoney(
              userState?.user?.quota,
              displayCurrency?.symbol,
            )}
          </p>
          <p className='text-[12px] text-[#64748B] dark:text-slate-400 mt-2 flex items-center gap-1'>
            {t('当前账户剩余的全部金额')}
          </p>
        </div>

        <div className='rounded-2xl from-emerald-50 bg-white dark:bg-slate-800 dark:from-slate-900 dark:via-slate-900 dark:to-slate-950 p-5'>
          <div className='flex items-center justify-between mb-2'>
            <h3 className='text-sm font-medium text-[#94A3B8] dark:text-slate-400'>
              {t('历史消费')}
            </h3>
          </div>
          <p
            className='text-[24px] text-[#475569] dark:text-white'
            style={{ fontWeight: '900' }}
          >
            {formatDisplayMoney(
              userState?.user?.used_quota,
              displayCurrency?.symbol,
            )}
          </p>
          <p className='text-[12px] text-[#64748B] dark:text-slate-400 mt-2'>
            {t('历史全部的消耗金额')}
          </p>
        </div>
        <div className='rounded-2xl from-emerald-50 bg-white dark:bg-slate-800 dark:from-slate-900 dark:via-slate-900 dark:to-slate-950 p-5'>
          <div className='flex items-center justify-between mb-2'>
            <h3 className='text-sm font-medium text-[#94A3B8] dark:text-slate-400'>
              {t('历史充值')}
            </h3>
          </div>
          <p
            className='text-[24px] text-[#475569] dark:text-white'
            style={{ fontWeight: '900' }}
          >
            {formatDisplayMoney(
              userState?.user?.total_topup_quota,
              displayCurrency?.symbol,
            )}
          </p>
          <p className='text-[12px] text-[#64748B] dark:text-slate-400 mt-2'>
            {t('历史充值的全部金额')}
          </p>
        </div>
        <div className='rounded-2xl from-emerald-50 bg-white dark:bg-slate-800 dark:from-slate-900 dark:via-slate-900 dark:to-slate-950 p-5'>
          <div className='flex items-center justify-between mb-2'>
            <h3 className='text-sm font-medium text-[#94A3B8] dark:text-slate-400'>
              {t('历史奖励/获赠')}
            </h3>
          </div>
          <p
            className='text-[24px] text-[#475569] dark:text-white'
            style={{ fontWeight: '900' }}
          >
            {formatDisplayMoney(
              userState?.user?.welfare_quota,
              displayCurrency?.symbol,
            )}
          </p>
          <div className='flex items-center justify-between'>
            <span className='text-[12px] text-[#64748B] dark:text-slate-400 mt-2'>
              {t('平台赠送或活动奖励')}
            </span>
            <span
              className='text-xs text-[color:var(--theme-primary)] underline cursor-pointer mt-2'
              onClick={toInvitationDetail}
            >
              {t('查看收益详情')}
            </span>
          </div>
        </div>
        <div className='rounded-2xl from-emerald-50 bg-white dark:bg-slate-800 dark:from-slate-900 dark:via-slate-900 dark:to-slate-950 p-5'>
          <div className='flex items-center justify-between mb-2'>
            <h3 className='text-sm font-medium text-[#94A3B8] dark:text-slate-400'>
              {t('请求次数')}
            </h3>
          </div>
          <p
            className='text-[24px] text-[#475569] dark:text-white'
            style={{ fontWeight: '900' }}
          >
            {userState?.user?.request_count || 0}
          </p>
          <div className='flex items-center justify-between'>
            <span className='text-[12px] text-[color:var(--theme-primary)] flex items-center  mt-2'>
              <TrendingUp size={16} className='mr-1' />{' '}
              {t('较昨日') + (userState?.user?.request_count_change || 0)}
            </span>
          </div>
        </div>
      </div>

      {/* 主体内容 */}
      <div className='grid grid-cols-1 xl:grid-cols-3 gap-5'>
        <div className='xl:col-span-2 space-y-5'>
          <div className='bg-white dark:bg-slate-900 rounded-2xl p-4 md:p-6'>
            <h2 className='text-lg font-bold text-slate-800 dark:text-white flex items-center mb-4'>
              {t('账户充值')}
            </h2>

            {shouldShowSubscription ? (
              <Tabs type='card' activeKey={activeTab} onChange={setActiveTab}>
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
                      displayCurrency={displayCurrency}
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
              </Tabs>
            ) : (
              topupContent
            )}
          </div>
        </div>

        {/* 右侧：充值小贴士（已移除额度预警） */}
        <div className='space-y-4'>
          <div className='rounded-2xl from-cyan-50 bg-white dark:bg-slate-800 to-emerald-50 dark:from-slate-900 dark:via-slate-900 dark:to-slate-950 p-5'>
            <h3 className='font-bold text-slate-800 dark:text-white mb-4 flex items-center gap-2'>
              <Lightbulb style={{ color: '#FDB878' }} /> {t('充值小贴士')}
            </h3>
            <ul className='space-y-3 text-sm text-slate-600 dark:text-slate-300'>
              <li className='flex items-center gap-2 pl-1'>
                <span className='text-lg font-bold text-[color:var(--theme-primary)] dark:text-cyan-400'>
                  01
                </span>
                <span>{t('如需查看消费明细，请到「账单中心」页面。')}</span>
              </li>
              <li className='flex items-center gap-2 pl-1'>
                <span className='text-lg font-bold text-[color:var(--theme-primary)] dark:text-cyan-400'>
                  02
                </span>
                <span>{t('设置合适充值档位，可减少频繁操作。')}</span>
              </li>
              <li className='flex items-center gap-2 pl-1'>
                <span className='text-lg font-bold text-[color:var(--theme-primary)] dark:text-cyan-400'>
                  03
                </span>
                <span>{t('如遇支付问题，请通过帮助中心联系支持。')}</span>
              </li>
            </ul>
          </div>

          {hasDynamicGiftPresets && (
            <div className='rounded-2xl bg-white p-5 dark:bg-slate-800'>
              <h3 className='mb-4 flex items-center gap-2 font-bold text-slate-800 dark:text-white'>
                <Gift size={20} className='text-amber-500' /> 充值赠送活动
              </h3>
              <div className='overflow-x-auto rounded-lg border border-slate-300'>
                <table className='w-full min-w-[360px] border-collapse text-sm text-slate-950'>
                  <thead className='bg-[#1f4e78] text-white'>
                    <tr>
                      {['充值', '赠送', '到账总额'].map((title) => (
                        <th
                          key={title}
                          className='border-r border-white/50 px-2 py-2 text-left font-bold last:border-r-0'
                        >
                          {title}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {topupGiftRules.map((rule, index) => {
                      const values = [
                        formatTopupGiftAmount(rule.threshold),
                        `+${formatTopupGiftAmount(rule.bonus)}`,
                        formatTopupGiftAmount(rule.threshold + rule.bonus),
                      ];

                      return (
                        <tr
                          key={rule.id || `${rule.threshold}-${index}`}
                          className={
                            index === topupGiftRules.length - 1
                              ? 'bg-[#fff2cc] font-bold'
                              : 'bg-white'
                          }
                        >
                          {values.map((value, valueIndex) => (
                            <td
                              key={`${rule.id || rule.threshold}-${valueIndex}`}
                              className='border-r border-t border-slate-300 px-2 py-2 text-left last:border-r-0'
                            >
                              {value}
                            </td>
                          ))}
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {topupGiftConfig.timed.enabled && (
            <div className='rounded-2xl bg-gradient-to-br m-5 from-slate-900 to-[#1f4e78] p-5 text-white shadow-sm'>
              <div className='mb-4 flex items-center justify-between gap-3'>
                <h3 className='flex items-center gap-2 font-bold'>
                  <Clock3 size={20} className='text-amber-300' /> 活动倒计时
                </h3>
                {!topupPromotionCountdown.ended && (
                  <span className='rounded-full bg-emerald-400/20 px-2.5 py-1 text-xs font-medium text-emerald-200'>
                    活动进行中
                  </span>
                )}
              </div>

              {topupPromotionCountdown.ended ? (
                <div className='rounded-xl border border-white/15 bg-white/10 px-4 py-8 text-center'>
                  <p className='text-xl font-bold text-amber-200'>活动已结束</p>
                </div>
              ) : (
                <div className='grid grid-cols-4 gap-2'>
                  {[
                    ['天', topupPromotionCountdown.days],
                    ['时', topupPromotionCountdown.hours],
                    ['分', topupPromotionCountdown.minutes],
                    ['秒', topupPromotionCountdown.seconds],
                  ].map(([unit, value]) => (
                    <div
                      key={unit}
                      className='rounded-xl border border-white/15 bg-white/10 px-1 py-3 text-center backdrop-blur-sm'
                    >
                      <strong className='block text-xl font-black tabular-nums sm:text-2xl'>
                        {String(value).padStart(2, '0')}
                      </strong>
                      <span className='mt-1 block text-xs text-blue-100'>
                        {unit}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* 充值记录 */}
      <div className='bg-white dark:bg-slate-900 rounded-2xl p-4 md:p-6'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-4'>
          <h2 className='text-lg font-bold text-slate-800 dark:text-white flex items-center'>
            {t('充值记录')}
          </h2>
          <Input
            prefix={<IconSearch />}
            placeholder={t('搜索订单号')}
            value={historyKeyword}
            onChange={handleHistoryKeywordChange}
            showClear
            style={{ width: '100%', maxWidth: 260 }}
          />
        </div>
        <Table
          columns={historyColumns}
          dataSource={topups}
          loading={historyLoading}
          rowKey='id'
          pagination={false}
          size='small'
          empty={
            <Empty
              image={
                <IllustrationNoResult style={{ width: 150, height: 150 }} />
              }
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
              }
              description={t('暂无账单记录')}
              style={{ padding: 30 }}
            />
          }
        />
        <div
          style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 12 }}
        >
          <Pagination
            total={historyTotal}
            hideOnSinglePage
            onPageChange={handleHistoryPageChange}
          />
        </div>
      </div>

      <CryptoPaymentDrawer
        visible={cryptoDrawerVisible}
        onClose={() => setCryptoDrawerVisible(false)}
        amount={topUpCount}
        currency={stripeCurrency?.currency || 'USD'}
        t={t}
        onSuccess={() => loadTopups(historyPage, historyPageSize)}
      />
    </div>
  );
};

export default RechargeCard;
