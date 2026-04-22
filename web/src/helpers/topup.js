export const TOPUP_BIZ_TYPE_CONFIG = {
  payment: { label: '在线充值', color: 'blue' },
  subscription: { label: '订阅套餐', color: 'purple' },
  redemption: { label: '兑换码', color: 'cyan' },
  invite_rebate: { label: '邀请返佣', color: 'green' },
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
  getTopupBizType(record) === 'invite_rebate';

export const getTopupBizTypeConfig = (record) => {
  const bizType = getTopupBizType(record);
  return (
    TOPUP_BIZ_TYPE_CONFIG[bizType] || {
      label: bizType || '-',
      color: 'grey',
    }
  );
};
