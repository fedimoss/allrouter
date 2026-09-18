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

import React, { useEffect, useMemo, useState } from 'react';
import { Badge, Empty, Modal, Pagination, Table, Tag, Tooltip, Typography } from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { IconCopy } from '@douyinfe/semi-icons';
import { CheckCircle, Coins, Gift } from 'lucide-react';
import { API, formatDisplayMoney, showError, timestamp2string } from '../../../helpers';
import {
  getTopupBizTypeConfig,
  getTopupDisplayAmount,
  isInviteRebateTopup,
  isSubscriptionTopup,
} from '../../../helpers/topup';

const { Text } = Typography;

const STATUS_CONFIG = {
  success: { type: 'success', key: '成功' },
  pending: { type: 'warning', key: '待支付' },
  failed: { type: 'danger', key: '失败' },
  expired: { type: 'danger', key: '已过期' },
};

const PAYMENT_METHOD_MAP = {
  stripe: 'Stripe',
  creem: 'Creem',
  waffo: 'Waffo',
  alipay: '支付宝',
  wxpay: '微信',
  redemptionCode: '兑换码',
  redemption_code: '兑换码',
};

const safeFormatTimestamp = (value) => {
  if (value === null || value === undefined || value === '') {
    return '-';
  }
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) {
    return '-';
  }
  try {
    return timestamp2string(numeric);
  } catch {
    return '-';
  }
};

const InviteBillingModal = ({ t, visible, onClose, inviteeId, inviteeName }) => {
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState([]);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(10);
  const [total, setTotal] = useState(0);

  const loadBillingRecords = async (currentPage = 1) => {
    if (!inviteeId) return;
    setLoading(true);
    try {
      const res = await API.get(
        `/api/user/topup/invitee?user_id=${inviteeId}&p=${currentPage}&page_size=${pageSize}`,
      );
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
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!visible) return;
    setRows([]);
    setTotal(0);
    setPage(1);
    loadBillingRecords(1).then();
  }, [visible, inviteeId]);

  useEffect(() => {
    if (!visible) return;
    loadBillingRecords(page).then();
  }, [page]);

  const renderStatusBadge = (status, record) => {
    // 充值返佣记录没有真实支付状态，统一展示为「已入账」
    if (isInviteRebateTopup(record)) {
      return (
        <span className='inline-flex items-center justify-center gap-1 rounded-full bg-emerald-50 px-3 py-1 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-300'>
          <CheckCircle size={14} />
          <span className='font-medium'>{t('已入账')}</span>
        </span>
      );
    }
    if (!status) {
      return <Text type='tertiary'>-</Text>;
    }
    const config = STATUS_CONFIG[status] || { type: 'primary', key: status };
    return (
      <span className='flex items-center justify-center gap-2'>
        <Badge dot type={config.type} />
        <span>{t(config.key)}</span>
      </span>
    );
  };

  const renderPaymentMethod = (paymentMethod) => {
    const displayName = PAYMENT_METHOD_MAP[paymentMethod];
    return (
      <Text>
        {displayName
          ? t(displayName)
          : paymentMethod === 'provider_profit'
            ? t('服务商分润')
            : paymentMethod || '-'}
      </Text>
    );
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

  const columns = useMemo(
    () => [
      {
        title: <span className='text-[#98A2B3] text-[14px] font-medium'>{t('订单号')}</span>,
        dataIndex: 'trade_no',
        key: 'trade_no',
        // 订单号过长时省略显示，悬停展示完整内容。
        // 不用 <Text copyable ellipsis>：copyable 会让 Semi 走 JS 测量路径，
        // 首帧渲染完整文本、下一帧才收成省略号，弹窗打开时能看到换行→省略的跳变；
        // 纯 CSS truncate 首帧即是省略号，无跳变。
        render: (text) => (
          <div className='flex min-w-0 items-center gap-1'>
            <Tooltip content={text} position='top'>
              <span className='truncate text-[14px] text-[#475467]'>
                {text}
              </span>
            </Tooltip>
            <IconCopy
              size='small'
              className='flex-shrink-0 cursor-pointer text-[#475467]'
              onClick={() => {
                navigator.clipboard?.writeText(String(text || ''));
              }}
            />
          </div>
        ),
      },
      {
        title: <span className='text-[#98A2B3] text-[14px] font-medium'>{t('账单类型')}</span>,
        dataIndex: 'biz_type',
        key: 'biz_type',
        render: (_, record) => renderBizTypeTag(record),
      },
      {
        title: <span className='text-[#98A2B3] text-[14px] font-medium'>{t('支付方式')}</span>,
        dataIndex: 'payment_method',
        key: 'payment_method',
        render: renderPaymentMethod,
      },
      {
        title: <span className='text-[#98A2B3] text-[14px] font-medium'>{t('充值额度')}</span>,
        dataIndex: 'amount',
        key: 'amount',
        render: (_, record) =>
          isSubscriptionTopup(record) ? (
            <Tag color='purple' shape='circle' size='small'>
              {t('订阅套餐')}
            </Tag>
          ) : isInviteRebateTopup(record) ? (
            <span className='inline-flex items-center gap-1 rounded-full bg-emerald-50 px-3 py-1 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-300'>
              <Gift size={14} />
              <Text strong className='!text-emerald-600 dark:!text-emerald-300'>
                +{record.amount}
              </Text>
            </span>
          ) : (
            <span className='flex items-center gap-1'>
              <Coins size={16} />
              <Text>{getTopupDisplayAmount(record)}</Text>
            </span>
          ),
      },
      {
        title: <span className='text-[#98A2B3] text-[14px] font-medium'>{t('支付金额')}</span>,
        dataIndex: 'money',
        key: 'money',
        render: (money, record) => {
          const normalizedMoney = Number(money || 0);
          if (normalizedMoney <= 0) {
            return <Text type='tertiary'>-</Text>;
          }
          // 优先使用后端返回的币种符号，Stripe 默认 $，其他默认 ¥
          const paySymbol =
            record.display_symbol ||
            (record.payment_method === 'stripe' ? '$' : '¥');
          return (
            <Text type='danger'>{formatDisplayMoney(money, paySymbol)}</Text>
          );
        },
      },
      {
        title: <span className='text-[#98A2B3] text-[14px] font-medium'>{t('状态')}</span>,
        dataIndex: 'status',
        key: 'status',
        align: 'center',
        // 防止状态标签被挤压换行
        render: (status, record) => (
          <div className='whitespace-nowrap'>
            {renderStatusBadge(status, record)}
          </div>
        ),
      },
      {
        title: <span className='text-[#98A2B3] text-[14px] font-medium'>{t('创建时间')}</span>,
        dataIndex: 'create_time',
        key: 'create_time',
        render: (time) => (
          <span className='whitespace-nowrap text-[#475467] text-[14px] font-semibold'>
            {safeFormatTimestamp(time)}
          </span>
        ),
      },
    ],
    [t],
  );

  const start = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const end = Math.min(page * pageSize, total);

  return (
    <Modal
      visible={visible}
      onCancel={onClose}
      footer={null}
      centered
      maskClosable
      closable
      width={1000}
      title={<div className='text-[16px] leading-[24px] font-semibold text-[#344054]'>{t('账单详情')}</div>}
      bodyStyle={{ padding: '0 0 10px 0' }}
      className='invite-billing-modal'
    >
      <div className='border-t border-[#E5E7EB] pt-5'>
        <Table
          columns={columns}
          dataSource={rows}
          rowKey='id'
          pagination={false}
          size='default'
          loading={loading}
          scroll={{ x: 900 }}
          empty={
            <Empty
              image={<IllustrationNoResult style={{ width: 120, height: 120 }} />}
              darkModeImage={<IllustrationNoResultDark style={{ width: 120, height: 120 }} />}
              description={t('暂无账单记录')}
            />
          }
        />

        <div className='mt-6 flex items-center justify-between'>
          <div className='text-[#475467] text-[14px] leading-[20px] font-semibold'>
            {t('显示')} {start} - {end} {t('共')} {total} {t('条记录')}
          </div>
          <Pagination
            total={total}
            pageSize={pageSize}
            currentPage={page}
            onPageChange={setPage}
            size='small'
            showQuickJumper
            showSizeChanger={false}
          />
        </div>
      </div>
    </Modal>
  );
};

export default InviteBillingModal;
