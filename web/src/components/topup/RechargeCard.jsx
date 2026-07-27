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

import React, { useEffect, useRef, useState } from 'react';
import {
  Card,
  Banner,
  Skeleton,
  Form,
  Spin,
  Tooltip,
  Tabs,
  TabPane,
  Input,
  Empty,
  Toast,
  Pagination,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { Wallet, Sparkles } from 'lucide-react';
import { IconSearch } from '@douyinfe/semi-icons';
import { useMinimumLoadingTime } from '../../hooks/common/useMinimumLoadingTime';
import { API, timestamp2string, formatDisplayMoney } from '../../helpers';
import {
  getTopupBizTypeConfig,
  getEffectiveTopupMin,
  isInviteRebateTopup,
} from '../../helpers/topup';
import SubscriptionPlansCard from './SubscriptionPlansCard';
import CryptoPaymentDrawer from './CryptoPaymentDrawer';

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
    end_time: 0,
  },
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

  // 同一接口同时提供赠送规则和倒计时配置。
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
            end_time: Number(timed.end_time) || 0,
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
      }
    } else if (inputMinTopUp < prevMin) {
      // 最低金额变小了（如从 Stripe 切换到微信），当前值恰好等于旧下限则重置到新下限
      if (currentValue && currentValue === prevMin) {
        setTopUpCount(inputMinTopUp);
        setSelectedPreset(null);
      }
    }

    prevInputMinTopUpRef.current = inputMinTopUp;
  }, [inputMinTopUp, setSelectedPreset, setTopUpCount]);

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

  const submitLoading = selectedPayMethod?.startsWith('waffo:')
    ? paymentLoading
    : paymentLoading && payWay === selectedPayMethod;

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
        <Form>
          <div className='space-y-6'>
            {(enableOnlineTopUp || enableStripeTopUp || enableWaffoTopUp) && (
              <div className='topup-recharge'>
                {/* 1. 充值方式 */}
                {payMethods &&
                  payMethods.filter((m) => m.type !== 'waffo').length > 0 && (
                    <>
                      <label className='tr-label'>{t('充值方式')}</label>
                      <div className='pay-methods'>
                        {payMethods
                          .filter((m) => m.type !== 'waffo')
                          .map((payMethod) => {
                            const disabled = isPayMethodDisabled(payMethod);
                            const selected =
                              selectedPayMethod === payMethod.type;
                            const minTopupVal =
                              Number(payMethod.min_topup) || 0;
                            const chip = (
                              <label
                                key={payMethod.type}
                                className={`pay-chip${selected ? ' is-checked' : ''}${disabled ? ' is-disabled' : ''}`}
                              >
                                <input
                                  type='radio'
                                  name='topupPayMethod'
                                  checked={selected}
                                  disabled={disabled}
                                  onChange={() =>
                                    setSelectedPayMethod(payMethod.type)
                                  }
                                />
                                {t(payMethod.name)}
                              </label>
                            );
                            return disabled &&
                              minTopupVal > Number(topUpCount || 0) ? (
                              <Tooltip
                                key={payMethod.type}
                                content={
                                  t('此支付方式最低充值金额为') +
                                  ' ' +
                                  minTopupVal
                                }
                              >
                                {chip}
                              </Tooltip>
                            ) : (
                              <React.Fragment key={payMethod.type}>
                                {chip}
                              </React.Fragment>
                            );
                          })}
                      </div>
                    </>
                  )}

                {enableWaffoTopUp &&
                  waffoPayMethods &&
                  waffoPayMethods.length > 0 && (
                    <>
                      <label className='tr-label'>{t('Waffo 充值')}</label>
                      <div className='pay-methods'>
                        {waffoPayMethods.map((method, index) => {
                          const methodKey = `waffo:${index}`;
                          const selected = selectedPayMethod === methodKey;
                          return (
                            <label
                              key={methodKey}
                              className={`pay-chip${selected ? ' is-checked' : ''}`}
                            >
                              <input
                                type='radio'
                                name='topupPayMethod'
                                checked={selected}
                                onChange={() =>
                                  setSelectedPayMethod(methodKey)
                                }
                              />
                              {method.name}
                            </label>
                          );
                        })}
                      </div>
                    </>
                  )}

                {/* 2. 充值数量 */}
                <label className='tr-label'>
                  {t('充值数量')}（{presetCurrencySymbol}）
                </label>
                <div
                  className={`amount-input${
                    !enableOnlineTopUp &&
                    !enableStripeTopUp &&
                    !enableWaffoTopUp
                      ? ' is-disabled'
                      : ''
                  }`}
                >
                  <span>{presetCurrencySymbol}</span>
                  <input
                    type='number'
                    inputMode='numeric'
                    min={inputMinTopUp}
                    max={999999999}
                    step={1}
                    disabled={
                      !enableOnlineTopUp &&
                      !enableStripeTopUp &&
                      !enableWaffoTopUp
                    }
                    value={topUpCount || ''}
                    placeholder={
                      isStripeCurrencyInput
                        ? t('充值数量，最低 ') +
                          stripeCurrencySymbol +
                          inputMinTopUp
                        : t('充值数量，最低 ') +
                          renderQuotaWithAmount(inputMinTopUp)
                    }
                    onChange={async (e) => {
                      const parsed =
                        parseInt(
                          String(e.target.value).replace(/[^\d]/g, ''),
                        ) || 0;
                      if (parsed && parsed >= 1) {
                        setTopUpCount(parsed);
                        setSelectedPreset(null);
                        // 有时区币种配置时，renderAmount 直接用 topUpCount 显示，无需调后端金额接口
                        if (!stripeCurrency) {
                          if (
                            selectedPayMethod === 'stripe' &&
                            getStripeAmount
                          ) {
                            await getStripeAmount(parsed);
                          } else {
                            await getAmount(parsed);
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
                        // 无时区币种配置时需重新请求金额
                        if (!stripeCurrency) {
                          if (
                            selectedPayMethod === 'stripe' &&
                            getStripeAmount
                          ) {
                            getStripeAmount(inputMinTopUp);
                          } else {
                            getAmount(inputMinTopUp);
                          }
                        }
                      }
                    }}
                  />
                </div>

                {/* 3. 选择充值额度 */}
                <label className='tr-label'>{t('选择充值额度')}</label>
                <div className='tier-grid'>
                  {displayedPresetAmounts.map((preset, index) => {
                    const discount =
                      preset.discount ||
                      topupInfo?.discount?.[preset.value] ||
                      1.0;
                    const hasDiscount = discount < 1.0;
                    const offPct = Math.round(discount * 100);
                    const offLabel =
                      offPct % 10 === 0 ? offPct / 10 : offPct;
                    const selected = selectedPreset === preset.value;
                    return (
                      <button
                        key={preset.giftRuleId || `${preset.value}-${index}`}
                        type='button'
                        className={`tier${selected ? ' is-active' : ''}`}
                        onClick={() => selectPresetAmount(preset)}
                      >
                        {hasDiscount && (
                          <span className='off'>
                            {offLabel}
                            {t('折')}
                          </span>
                        )}
                        <b>
                          {presetCurrencySymbol} {preset.value}
                        </b>
                        {hasDynamicGiftPresets ? (
                          <small>
                            {t('实际到账：')}
                            {formatTopupGiftAmount(preset.value + preset.bonus)}
                          </small>
                        ) : (
                          <small>
                            {t('实付')}{' '}
                            {formatTopupGiftAmount(preset.value * discount)}
                          </small>
                        )}
                      </button>
                    );
                  })}
                </div>

                {/* 4. 实付金额汇总 */}
                <div className='pay-summary'>
                  <span>{t('实付金额')}</span>
                  <b>
                    <Skeleton
                      loading={showAmountSkeleton}
                      active
                      placeholder={
                        <Skeleton.Title
                          style={{ width: 80, height: 24, borderRadius: 6 }}
                        />
                      }
                    >
                      {renderAmount()}
                    </Skeleton>
                  </b>
                </div>

                {/* 5. 确认充值 */}
                <button
                  type='button'
                  className='pay-submit'
                  onClick={handlePrimaryTopUp}
                  disabled={!selectedPayMethod || submitLoading}
                >
                  {submitLoading && <Spin size='small' />}
                  <span>{t('立即充值')}</span>
                </button>

                {/* 6. 充值小贴士（hint-card） */}
                <div className='hint-card'>
                  <b>{t('充值小贴士')}：</b>
                  {t('如需查看消费明细，请到「账单中心」页面。')}
                  <br />
                  {t('设置合适充值档位，可减少频繁操作。')}
                  <br />
                  {t('如遇支付问题，请通过帮助中心联系支持。')}
                </div>
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
      <div className='grid grid-cols-1 gap-4 md:grid-cols-3'>
        <div className='rounded-[18px] border border-transparent bg-[linear-gradient(135deg,#0e1a15,#0a110d)] px-6 py-[22px] text-white transition-[transform,box-shadow] duration-200 hover:-translate-y-0.5 hover:shadow-[0_12px_32px_rgba(12,25,20,0.08)]'>
          <h3 className='text-sm font-medium text-white/60'>{t('当前余额')}</h3>
          <p className='mt-2 text-[30px] font-extrabold tracking-normal'>
            {formatDisplayMoney(
              userState?.user?.quota,
              displayCurrency?.symbol,
            )}
          </p>
          <p className='mt-1.5 text-xs text-[rgba(50,254,165,0.85)]'>
            {t('当前账户剩余的全部金额')}
          </p>
        </div>

        <div className='rounded-[18px] border border-[rgba(12,25,20,0.1)] bg-white px-6 py-[22px] transition-[transform,box-shadow] duration-200 hover:-translate-y-0.5 hover:shadow-[0_12px_32px_rgba(12,25,20,0.08)] dark:border-white/10 dark:bg-slate-800'>
          <h3 className='text-sm font-medium text-[rgba(14,25,21,0.55)] dark:text-slate-400'>
            {t('历史消费')}
          </h3>
          <p className='mt-2 text-[30px] font-extrabold tracking-normal text-[#0e1915] dark:text-white'>
            {formatDisplayMoney(
              userState?.user?.used_quota,
              displayCurrency?.symbol,
            )}
          </p>
          <p className='mt-1.5 text-xs text-[rgba(14,25,21,0.45)] dark:text-slate-400'>
            {t('历史全部的消耗金额')}
          </p>
        </div>

        <div className='rounded-[18px] border border-[rgba(12,25,20,0.1)] bg-white px-6 py-[22px] transition-[transform,box-shadow] duration-200 hover:-translate-y-0.5 hover:shadow-[0_12px_32px_rgba(12,25,20,0.08)] dark:border-white/10 dark:bg-slate-800'>
          <h3 className='text-sm font-medium text-[rgba(14,25,21,0.55)] dark:text-slate-400'>
            {t('历史充值')}
          </h3>
          <p className='mt-2 text-[30px] font-extrabold tracking-normal text-[#0e1915] dark:text-white'>
            {formatDisplayMoney(
              userState?.user?.total_topup_quota,
              displayCurrency?.symbol,
            )}
          </p>
          <p className='mt-1.5 text-xs text-[rgba(14,25,21,0.45)] dark:text-slate-400'>
            {t('历史充值的全部金额')}
          </p>
        </div>
      </div>

      {/* 主体内容 */}
      <div className='space-y-5'>
        <div className='space-y-5'>
          <div className='bg-white dark:bg-slate-900 rounded-[18px] p-5 md:p-[26px] border border-[rgba(12,25,20,0.1)] dark:border-white/10'>
            <h2 className='text-lg font-bold text-slate-800 dark:text-white flex items-center mb-5'>
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
      </div>

      {/* 充值记录 */}
      <section className='topup-records'>
        <div className='records-head'>
          <h2>{t('充值记录')}</h2>
          <Input
            prefix={<IconSearch />}
            placeholder={t('搜索订单号')}
            value={historyKeyword}
            onChange={handleHistoryKeywordChange}
            showClear
            style={{ width: '100%', maxWidth: 260 }}
          />
        </div>
        <div className='table-wrap'>
          <table>
            <thead>
              <tr>
                <th>{t('订单号')}</th>
                <th>{t('来源')}</th>
                <th>{t('时间')}</th>
                <th>{t('渠道')}</th>
                <th>{t('金额')}</th>
                <th>{t('状态')}</th>
              </tr>
            </thead>
            <tbody>
              {topups.length === 0 ? (
                historyLoading ? (
                <tr>
                  <td colSpan={6}>
                    <div
                      className='flex justify-center'
                      style={{ padding: '32px 0' }}
                    >
                      <Spin />
                    </div>
                  </td>
                </tr>
                ) : (
                <tr>
                  <td colSpan={6}>
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
                      description={t('暂无账单记录')}
                      style={{ padding: 24 }}
                    />
                  </td>
                </tr>
                )
              ) : (
                topups.map((record) => {
                  const money = Number(record.money || 0);
                  const isCrypto = record.payment_method === 'crypto';
                  const channelName = PAYMENT_METHOD_MAP[record.payment_method];
                  const bizConfig = getTopupBizTypeConfig(record);
                  const statusConfig = STATUS_CONFIG[record.status];
                  const statusCls =
                    record.status === 'success'
                      ? 'ok'
                      : record.status === 'pending'
                        ? 'pending'
                        : 'failed';
                  return (
                    <tr key={record.id}>
                      <td className='trade-no' title={record.trade_no}>
                        {record.trade_no}
                      </td>
                      <td>{t(bizConfig.label)}</td>
                      <td>{timestamp2string(record.create_time)}</td>
                      <td>
                        <span className='method-tag'>
                          {channelName
                            ? t(channelName)
                            : record.payment_method || '-'}
                        </span>
                      </td>
                      <td>
                        {money <= 0 ? (
                          <span className='muted'>-</span>
                        ) : (
                          <span className='amount-pos'>
                            {isCrypto
                              ? `+${money} USDT`
                              : `+${formatDisplayMoney(
                                  money,
                                  record.display_symbol ||
                                    displayCurrency?.symbol ||
                                    '$',
                                )}`}
                          </span>
                        )}
                      </td>
                      <td>
                        {isInviteRebateTopup(record) ? (
                          <span className='status status--ok'>
                            {t('已入账')}
                          </span>
                        ) : !record.status ? (
                          <span className='muted'>-</span>
                        ) : statusConfig ? (
                          <span className={`status status--${statusCls}`}>
                            {t(statusConfig.key)}
                          </span>
                        ) : (
                          <span>{record.status}</span>
                        )}
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
          {historyLoading && topups.length > 0 && (
            <div className='records-loading-overlay'>
              <Spin />
            </div>
          )}
        </div>
        <div className='records-pagination'>
          <Pagination
            total={historyTotal}
            hideOnSinglePage
            onPageChange={handleHistoryPageChange}
          />
        </div>
      </section>

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
