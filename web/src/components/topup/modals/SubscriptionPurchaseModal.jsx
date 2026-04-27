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

import React from 'react';
import {
  Banner,
  Modal,
  Typography,
  Card,
  Button,
  Select,
  Divider,
  Tooltip,
} from '@douyinfe/semi-ui';
import { Crown, CalendarClock, Package } from 'lucide-react';
import { SiStripe } from 'react-icons/si';
import { IconCreditCard } from '@douyinfe/semi-icons';
import { renderQuota } from '../../../helpers';
// 使用统一的币种格式化工具，替代旧的 getCurrencyConfig 静态方法
import {
  formatDisplayMoney, // 将金额格式化为带币种符号的字符串（如 "$9.99"、"¥69.00"）
  normalizeDisplayCurrency, // 标准化币种配置对象，补全缺失字段并设置默认值
} from '../../../helpers/currency';
import {
  formatSubscriptionDuration,
  formatSubscriptionResetPeriod,
} from '../../../helpers/subscriptionFormat';

const { Text } = Typography;

// 根据展示币种选择对应的 Stripe Price ID
// CNY -> stripe_price_cny_id（人民币专用 Stripe Price）
// 其他币种 -> stripe_price_id（美元 Stripe Price）
const getStripePriceIdForDisplayCurrency = (plan, displayCurrency) => {
  if (!plan) return ''; // 套餐为空时返回空字符串
  const normalized = normalizeDisplayCurrency(displayCurrency); // 标准化币种配置
  if (normalized.currency === 'CNY') { // 如果是人民币
    return plan.stripe_price_cny_id || ''; // 返回人民币 Stripe Price ID
  }
  return plan.stripe_price_id || ''; // 否则返回美元 Stripe Price ID
};

const SubscriptionPurchaseModal = ({
  t,
  visible,
  onCancel,
  selectedPlan,
  paying,
  selectedEpayMethod,
  setSelectedEpayMethod,
  epayMethods = [], // 易支付方式列表
  displayCurrency, // 从父组件传入的标准化币种配置（含 symbol、currency、unitPrice 等）
  enableOnlineTopUp = false,
  enableStripeTopUp = false,
  enableCreemTopUp = false,
  purchaseLimitInfo = null,
  onPayStripe,
  onPayCreem,
  onPayEpay,
}) => {
  const plan = selectedPlan?.plan; // 获取当前选中的套餐对象
  const totalAmount = Number(plan?.total_amount || 0); // 套餐总额度（0 表示不限）
  // 标准化币种配置，确保 symbol / currency / unitPrice 等字段均有合理默认值
  const normalizedDisplayCurrency = normalizeDisplayCurrency(displayCurrency);
  const price = plan ? Number(plan.price_amount || 0) : 0; // 套餐原价（USD）
  // 根据展示币种选择对应的 Stripe Price ID（USD 或 CNY）
  const stripePriceId = getStripePriceIdForDisplayCurrency(
    plan, // 当前套餐
    normalizedDisplayCurrency, // 标准化后的币种配置
  );
  // 只有管理员开启 Stripe 支付 且 套餐配置了对应币种的 Stripe Price ID 时才显示 Stripe 按钮
  const hasStripe = enableStripeTopUp && !!stripePriceId;
  const hasCreem = enableCreemTopUp && !!plan?.creem_product_id; // 是否启用 Creem 支付
  const hasEpay = enableOnlineTopUp && epayMethods.length > 0; // 是否启用易支付
  const hasAnyPayment = hasStripe || hasCreem || hasEpay; // 是否有任意一种支付方式可用
  // 是否仅有易支付可用（无 Stripe / Creem），用于后续决定价格显示逻辑
  const isEpayOnly = hasEpay && !hasStripe && !hasCreem;
  // 从币种配置中读取人民币汇率，兼容驼峰和下划线两种命名
  const cnyRate = Number(
    displayCurrency?.cnyRate || displayCurrency?.cny_rate || 0,
  );
  // 计算展示金额：仅易支付且有 CNY 汇率时，直接用 cnyRate 换算人民币；否则走通用币种逻辑
  const displayAmount =
    isEpayOnly && cnyRate > 0 // 仅易支付且有人民币汇率
      ? price * cnyRate // 原价 × 汇率 = 人民币金额
      : normalizedDisplayCurrency.currency === 'CNY' // 非仅易支付，判断是否为人民币
        ? price * normalizedDisplayCurrency.unitPrice // 原价 × unitPrice（汇率）= 人民币金额
        : price; // 非人民币直接使用原价（USD）
  // 决定显示的币种符号：仅易支付走 CNY 时显示 ¥，其他场景使用标准化后的币种符号
  const displaySymbol =
    isEpayOnly && cnyRate > 0 ? '¥' : normalizedDisplayCurrency.symbol;
  // 使用 formatDisplayMoney 将金额和符号格式化为最终显示的价格字符串
  const displayPrice = formatDisplayMoney(displayAmount, displaySymbol);
  const purchaseLimit = Number(purchaseLimitInfo?.limit || 0);
  const purchaseCount = Number(purchaseLimitInfo?.count || 0);
  const purchaseLimitReached =
    purchaseLimit > 0 && purchaseCount >= purchaseLimit;

  return (
    <Modal
      title={
        <div className='flex items-center'>
          <Crown className='mr-2' size={18} />
          {t('购买订阅套餐')}
        </div>
      }
      visible={visible}
      onCancel={onCancel}
      footer={null}
      size='small'
      centered
    >
      {plan ? (
        <div className='space-y-4 pb-10'>
          {/* 套餐信息 */}
          <Card className='!rounded-xl !border-0 bg-slate-50 dark:bg-slate-800'>
            <div className='space-y-3'>
              <div className='flex justify-between items-center'>
                <Text strong className='text-slate-700 dark:text-slate-200'>
                  {t('套餐名称')}：
                </Text>
                <Typography.Text
                  ellipsis={{ rows: 1, showTooltip: true }}
                  className='text-slate-900 dark:text-slate-100'
                  style={{ maxWidth: 200 }}
                >
                  {plan.title}
                </Typography.Text>
              </div>
              <div className='flex justify-between items-center'>
                <Text strong className='text-slate-700 dark:text-slate-200'>
                  {t('有效期')}：
                </Text>
                <div className='flex items-center'>
                  <CalendarClock size={14} className='mr-1 text-slate-500' />
                  <Text className='text-slate-900 dark:text-slate-100'>
                    {formatSubscriptionDuration(plan, t)}
                  </Text>
                </div>
              </div>
              {formatSubscriptionResetPeriod(plan, t) !== t('不重置') && (
                <div className='flex justify-between items-center'>
                  <Text strong className='text-slate-700 dark:text-slate-200'>
                    {t('重置周期')}：
                  </Text>
                  <Text className='text-slate-900 dark:text-slate-100'>
                    {formatSubscriptionResetPeriod(plan, t)}
                  </Text>
                </div>
              )}
              <div className='flex justify-between items-center'>
                <Text strong className='text-slate-700 dark:text-slate-200'>
                  {t('总额度')}：
                </Text>
                <div className='flex items-center'>
                  <Package size={14} className='mr-1 text-slate-500' />
                  {totalAmount > 0 ? (
                    <Tooltip content={`${t('原生额度')}：${totalAmount}`}>
                      <Text className='text-slate-900 dark:text-slate-100'>
                        {renderQuota(totalAmount)}
                      </Text>
                    </Tooltip>
                  ) : (
                    <Text className='text-slate-900 dark:text-slate-100'>
                      {t('不限')}
                    </Text>
                  )}
                </div>
              </div>
              {plan?.upgrade_group ? (
                <div className='flex justify-between items-center'>
                  <Text strong className='text-slate-700 dark:text-slate-200'>
                    {t('升级分组')}：
                  </Text>
                  <Text className='text-slate-900 dark:text-slate-100'>
                    {plan.upgrade_group}
                  </Text>
                </div>
              ) : null}
              <Divider margin={8} />
              <div className='flex justify-between items-center'>
                <Text strong className='text-slate-700 dark:text-slate-200'>
                  {t('应付金额')}：
                </Text>
                <Text strong className='text-xl text-purple-600'>
                  {displayPrice}
                </Text>
              </div>
            </div>
          </Card>

          {/* 支付方式 */}
          {purchaseLimitReached && (
            <Banner
              type='warning'
              description={`${t('已达到购买上限')} (${purchaseCount}/${purchaseLimit})`}
              className='!rounded-xl'
              closeIcon={null}
            />
          )}

          {hasAnyPayment ? (
            <div className='space-y-3'>
              <Text size='small' type='tertiary'>
                {t('选择支付方式')}：
              </Text>

              {/* Stripe / Creem */}
              {(hasStripe || hasCreem) && (
                <div className='flex gap-2'>
                  {hasStripe && (
                    <Button
                      theme='light'
                      className='flex-1'
                      icon={<SiStripe size={14} color='#635BFF' />}
                      onClick={onPayStripe}
                      loading={paying}
                      disabled={purchaseLimitReached}
                    >
                      Stripe
                    </Button>
                  )}
                  {hasCreem && (
                    <Button
                      theme='light'
                      className='flex-1'
                      icon={<IconCreditCard />}
                      onClick={onPayCreem}
                      loading={paying}
                      disabled={purchaseLimitReached}
                    >
                      Creem
                    </Button>
                  )}
                </div>
              )}

              {/* 易支付 */}
              {hasEpay && (
                <div className='flex gap-2'>
                  <Select
                    value={selectedEpayMethod}
                    onChange={setSelectedEpayMethod}
                    style={{ flex: 1 }}
                    size='default'
                    placeholder={t('选择支付方式')}
                    optionList={epayMethods.map((m) => ({
                      value: m.type,
                      label: m.name || m.type,
                    }))}
                    disabled={purchaseLimitReached}
                  />
                  <Button
                    theme='solid'
                    type='primary'
                    onClick={onPayEpay}
                    loading={paying}
                    disabled={!selectedEpayMethod || purchaseLimitReached}
                  >
                    {t('支付')}
                  </Button>
                </div>
              )}
            </div>
          ) : (
            <Banner
              type='info'
              description={t('管理员未开启在线支付功能，请联系管理员配置。')}
              className='!rounded-xl'
              closeIcon={null}
            />
          )}
        </div>
      ) : null}
    </Modal>
  );
};

export default SubscriptionPurchaseModal;
