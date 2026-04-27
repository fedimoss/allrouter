export const TOPUP_BIZ_TYPE_CONFIG = {
  payment: { label: '在线充值', color: 'blue' },
  subscription: { label: '订阅套餐', color: 'purple' },
  redemption: { label: '兑换码', color: 'cyan' },
  topup_rebate: { label: '充值返佣', color: 'green' },
};

// Stripe 人民币最低充值金额（单位：元），Stripe 要求 CNY 最低 3 元，这里留余量设为 4
export const STRIPE_CNY_MIN_TOPUP = 4;

/**
 * 计算实际生效的最低充值金额
 * 当支付方式为 Stripe 且币种为人民币时，取系统配置与 Stripe CNY 最低限额的较大值
 */
export const getEffectiveTopupMin = ({
  paymentType,
  minTopup,
  stripeCurrency,
  fallback = 0,
}) => {
  const normalizedMinTopup = Number(minTopup);
  const baseMinTopup = Number.isFinite(normalizedMinTopup)
    ? normalizedMinTopup
    : fallback;

  // Stripe + 人民币场景：确保最低充值金额不低于 Stripe CNY 最低限额
  if (paymentType === 'stripe' && stripeCurrency?.currency === 'CNY') {
    return Math.max(baseMinTopup, STRIPE_CNY_MIN_TOPUP);
  }

  return baseMinTopup;
};

export const getTopupBizType = (record) => {
  if (record?.biz_type) {
    return record.biz_type;
  }
  const tradeNo = String(record?.trade_no || '').toLowerCase();
  return Number(record?.amount || 0) === 0 && tradeNo.startsWith('sub')
    ? 'subscription'
    : 'payment';
};

export const isSubscriptionTopup = (record) =>
  getTopupBizType(record) === 'subscription';

export const isInviteRebateTopup = (record) =>
  getTopupBizType(record) === 'topup_rebate';

export const getTopupBizTypeConfig = (record) => {
  const bizType = getTopupBizType(record);
  return (
    TOPUP_BIZ_TYPE_CONFIG[bizType] || {
      label: bizType || '-',
      color: 'grey',
    }
  );
};
