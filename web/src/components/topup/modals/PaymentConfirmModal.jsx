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
import { Modal, Typography, Card, Skeleton, Button, Toast } from '@douyinfe/semi-ui';
import { SiAlipay, SiWechat, SiStripe } from 'react-icons/si';
import { CreditCard } from 'lucide-react';

const { Text } = Typography;

const PaymentConfirmModal = ({
  t,
  open,
  onlineTopUp,
  handleCancel,
  confirmLoading,
  topUpCount,
  amountLoading,
  renderAmount,
  payWay,
  payMethods,
  // 新增：用于显示折扣明细
  amountNumber,
  discountRate,
  stripeSymbol, // Stripe 支付的币种符号（根据用户时区决定，如 ¥ 或 $）
  displayCurrency, // 展示币种信息 { currency, symbol, unitPrice }
}) => {
  // 判断是否有有效折扣（0 < 折扣率 < 1 且原价大于 0）
  const hasDiscount =
    discountRate && discountRate > 0 && discountRate < 1 && amountNumber > 0;
  // 根据折扣率反算原价
  const originalAmount = hasDiscount ? amountNumber / discountRate : 0;
  // 计算优惠金额
  const discountAmount = hasDiscount ? originalAmount - amountNumber : 0;

  // 判断是否为微信/支付宝等人民币支付方式
  const isCNYOnlyPayment = payWay === 'wxpay' || payWay === 'alipay';
  // 用户展示币种是否为美元
  const isUSDDisplay = displayCurrency?.currency !== 'CNY';

  // 将金额从美元换算为人民币（美元金额 × CNY 汇率）
  const convertUsdToCNY = (usdValue) => {
    const rate = displayCurrency?.cnyRate || 1;
    return (Number(usdValue) * rate).toFixed(1);
  };

  // 格式化"充值数量"：始终使用用户展示币种符号
  const formatChargeAmount = (value) => {
    if (payWay === 'stripe') {
      const sym = stripeSymbol || '$';
      return `${sym}${parseInt(value) || 0}`;
    }
    // 微信/支付宝：显示展示币种符号（美元用户显示 $，人民币用户显示 ¥）
    const sym = displayCurrency?.symbol || '$';
    return `${sym}${parseInt(value) || 0}`;
  };

  // 格式化"实付金额"：微信/支付宝始终显示人民币，美元用户需换算
  const formatPayAmount = (value, { negative = false } = {}) => {
    if (payWay === 'stripe') {
      const sym = stripeSymbol || '$';
      return `${negative ? '-' : ''}${sym}${parseInt(value) || 0}`;
    }
    // 微信/支付宝：美元用户换算为人民币，人民币用户直接显示
    if (isCNYOnlyPayment && isUSDDisplay) {
      const cnyAmount = convertUsdToCNY(value);
      return `${negative ? '- ' : ''}${cnyAmount} ${t('元')}`;
    }
    const numericValue = Number(value || 0).toFixed(2);
    return `${negative ? '- ' : ''}${numericValue} ${t('元')}`;
  };
  return (
    <Modal
      title={
        <div className='flex items-center'>
          <CreditCard className='mr-2' size={18} />
          {t('充值确认')}
        </div>
      }
      visible={open}
      // onOk={onlineTopUp}
      onCancel={handleCancel}
      maskClosable={false}
      size='small'
      centered
      confirmLoading={confirmLoading}
      footer={
        <>
          <div className='flex justify-end'>
            <Button onClick={handleCancel} style={{color:'#000'}}>{t('取消')}</Button>
            <Button style={{ color: '#000', background:'linear-gradient(90deg, #09FEF7 0%, #BAFF29 100%)'}}  onClick={onlineTopUp}>{t('充值')}</Button>
          </div>
        </>
      }
    >
      <div className='space-y-4'>
        <Card className='!rounded-xl !border-0 bg-slate-50 dark:bg-slate-800'>
          <div className='space-y-3'>
            <div className='flex justify-between items-center'>
              <Text strong className='text-slate-700 dark:text-slate-200'>
                {t('充值数量')}：
              </Text>
              <Text className='text-slate-900 dark:text-slate-100'>
                {formatChargeAmount(topUpCount)}
              </Text>
            </div>
            <div className='flex justify-between items-center'>
              <Text strong className='text-slate-700 dark:text-slate-200'>
                {t('实付金额')}：
              </Text>
              {amountLoading ? (
                <Skeleton.Title style={{ width: '60px', height: '16px' }} />
              ) : (
                <div className='flex items-baseline space-x-2'>
                  <Text strong className='font-bold' style={{ color: 'red' }}>
                    {formatPayAmount(topUpCount)}
                  </Text>
                  {hasDiscount && (
                    <Text size='small' className='text-rose-500'>
                      {Math.round(discountRate * 100)}%
                    </Text>
                  )}
                </div>
              )}
            </div>
            {hasDiscount && !amountLoading && (
              <>
                <div className='flex justify-between items-center'>
                  <Text className='text-slate-500 dark:text-slate-400'>
                    {t('原价')}：
                  </Text>
                  <Text delete className='text-slate-500 dark:text-slate-400'>
                      {formatPayAmount(originalAmount)}

                  </Text>
                </div>
                <div className='flex justify-between items-center'>
                  <Text className='text-slate-500 dark:text-slate-400'>
                    {t('优惠')}：
                  </Text>
                  <Text className='text-emerald-600 dark:text-emerald-400'>
                    {formatPayAmount(discountAmount, { negative: true })}
                  </Text>
                </div>
              </>
            )}
            <div className='flex justify-between items-center'>
              <Text strong className='text-slate-700 dark:text-slate-200'>
                {t('支付方式')}：
              </Text>
              <div className='flex items-center'>
                {(() => {
                  const payMethod = payMethods.find(
                    (method) => method.type === payWay,
                  );
                  if (payMethod) {
                    return (
                      <>
                        {payMethod.type === 'alipay' ? (
                          <SiAlipay
                            className='mr-2'
                            size={16}
                            color='#1677FF'
                          />
                        ) : payMethod.type === 'wxpay' ? (
                          <SiWechat
                            className='mr-2'
                            size={16}
                            color='#07C160'
                          />
                        ) : payMethod.type === 'stripe' ? (
                          <SiStripe
                            className='mr-2'
                            size={16}
                            color='#635BFF'
                          />
                        ) : (
                          <CreditCard
                            className='mr-2'
                            size={16}
                            color={
                              payMethod.color || 'var(--semi-color-text-2)'
                            }
                          />
                        )}
                        <Text className='text-slate-900 dark:text-slate-100'>
                          {payMethod.name}
                        </Text>
                      </>
                    );
                  } else {
                    // 默认充值方式
                    if (payWay === 'alipay') {
                      return (
                        <>
                          <SiAlipay
                            className='mr-2'
                            size={16}
                            color='#1677FF'
                          />
                          <Text className='text-slate-900 dark:text-slate-100'>
                            {t('支付宝')}
                          </Text>
                        </>
                      );
                    } else if (payWay === 'stripe') {
                      return (
                        <>
                          <SiStripe
                            className='mr-2'
                            size={16}
                            color='#635BFF'
                          />
                          <Text className='text-slate-900 dark:text-slate-100'>
                            Stripe
                          </Text>
                        </>
                      );
                    } else {
                      return (
                        <>
                          <SiWechat
                            className='mr-2'
                            size={16}
                            color='#07C160'
                          />
                          <Text className='text-slate-900 dark:text-slate-100'>
                            {t('微信')}
                          </Text>
                        </>
                      );
                    }
                  }
                })()}
              </div>
            </div>
          </div>
        </Card>
      </div>
    </Modal>
  );
};

export default PaymentConfirmModal;
