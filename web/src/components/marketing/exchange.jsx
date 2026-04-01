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

import React, { useState, useContext } from 'react';
import {
  Typography,
  Button,
  Input,
  Table,
  Modal,
  Select,
  DatePicker,
} from '@douyinfe/semi-ui';
import {
  ArrowRight
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';
import {
  API,
  showError,
  showSuccess,
  showInfo,
  renderQuota,
} from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import imgOne from '../../../public/one.png';
import imgTwo from '../../../public/two.png';
import imgThree from '../../../public/three.png';

const { Text, Title } = Typography;

// 静态兑换记录
const STATIC_RECORDS = [];

const STATUS_MAP = {
  redeemed: {
    color: 'bg-green-50 dark:bg-green-900/30 text-green-700 dark:text-green-400',
    dot: 'bg-green-500',
    label: '已兑换',
  },
  expired: {
    color: 'bg-slate-100 dark:bg-slate-700 text-slate-500 dark:text-slate-400',
    dot: 'bg-slate-400',
    label: '已过期',
  },
};

const ACCOUNT_STATUS_MAP = {
  arrived: {
    color: 'bg-green-50 dark:bg-green-900/30 text-green-700 dark:text-green-400',
    label: '已到账',
  },
  pending: {
    color: 'bg-amber-50 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400',
    label: '处理中',
  },
  failed: {
    color: 'bg-red-50 dark:bg-red-900/30 text-red-700 dark:text-red-400',
    label: '失败',
  },
};

const Exchange = () => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);
  const isMobile = useIsMobile();

  const [redemptionCode, setRedemptionCode] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const topUpLink = statusState?.status?.top_up_link || '';

  const openTopUpLink = () => {
    if (!topUpLink) {
      showError(t('超级管理员未设置充值链接！'));
      return;
    }
    window.open(topUpLink, '_blank');
  };

  const handleRedeem = async () => {
    if (redemptionCode === '') {
      showInfo(t('请输入兑换码！'));
      return;
    }
    setIsSubmitting(true);
    try {
      const res = await API.post('/api/user/topup', {
        key: redemptionCode,
      });
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(t('兑换成功！'));
        Modal.success({
          title: t('兑换成功！'),
          content: t('成功兑换额度：') + renderQuota(data),
          centered: true,
        });
        if (userState.user) {
          const updatedUser = {
            ...userState.user,
            quota: userState.user.quota + data,
          };
          userDispatch({ type: 'login', payload: updatedUser });
        }
        setRedemptionCode('');
      } else {
        showError(message);
      }
    } catch {
      showError(t('请求失败'));
    } finally {
      setIsSubmitting(false);
    }
  };

  // 表格列定义
  const columns = [
    {
      title: t('兑换时间'),
      dataIndex: 'redeem_time',
      key: 'redeem_time',
      render: (text) => (
        <span className='text-slate-600 dark:text-slate-300 whitespace-nowrap'>
          {text}
        </span>
      ),
    },
    {
      title: t('兑换码'),
      dataIndex: 'code',
      key: 'code',
      render: (text) => (
        <code className='text-xs font-mono bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 px-2 py-1 rounded'>
          {text}
        </code>
      ),
    },
    {
      title: t('类型'),
      dataIndex: 'type',
      key: 'type',
      render: (text) => (
        <span className='text-slate-600 dark:text-slate-300'>{t(text)}</span>
      ),
    },
    {
      title: t('面值/权益'),
      dataIndex: 'value',
      key: 'value',
      render: (text) => (
        <Text strong>{text}</Text>
      ),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      key: 'status',
      render: (status) => {
        const cfg = STATUS_MAP[status] || STATUS_MAP.redeemed;
        return (
          <span
            className={`inline-flex items-center px-2.5 py-1 ${cfg.color} rounded-full text-xs font-medium`}
          >
            <span className={`w-1.5 h-1.5 rounded-full ${cfg.dot} mr-1.5`} />
            {t(cfg.label)}
          </span>
        );
      },
    },
    {
      title: t('到账状态'),
      dataIndex: 'account_status',
      key: 'account_status',
      render: (status) => {
        const cfg = ACCOUNT_STATUS_MAP[status] || ACCOUNT_STATUS_MAP.pending;
        return (
          <span
            className={`inline-flex items-center px-2.5 py-1 ${cfg.color} rounded-full text-xs font-medium`}
          >
            {t(cfg.label)}
          </span>
        );
      },
    },
    {
      title: t('流水ID'),
      dataIndex: 'txn_id',
      key: 'txn_id',
      render: (text) => (
        <span className='text-xs font-mono text-slate-500 dark:text-slate-400'>
          {text}
        </span>
      ),
    },
  ];

  // 活动推荐卡片数据
  const promoCards = [
    {
      bgImgUrl: imgOne,
      title: t('邀请好友得兑换码'),
      subTitle: t('HOT EVENT'),
      desc: t('每成功邀请一位好友注册，可获得一张兑换码奖励'),
      link: '/console/invitation',
      linkText: t('立即邀请'),
    },
    {
      bgImgUrl: imgTwo,
      title: t('关注公众号领码'),
      subTitle: t('OFFICIAL'),
      desc: t('关注官方公众号，回复关键词领取兑换码'),
      link: null,
      linkText: t('去关注'),
    },
    {
      bgImgUrl: imgThree,
      title: t('社区贡献奖励'),
      subTitle: t('COMMUNITY'),
      desc: t('在社区发布教程、提交 Bug 或贡献代码，可获得额度奖励'),
      link: null,
      linkText: t('了解详情'),
    },
  ];

  return (
    <div className=''>
      <div className='mx-full'>
        {/* 页面标题 */}
        <div className='mb-6'>
          <div className='flex items-center gap-3'> 
            <div className='text-[30px] font-medium text-[#475569] dark:text-white'>
              {t('兑换码')}
            </div>
          </div>
          <div className='text-[16px] font-medium text-[#94A3B8] dark:text-slate-200 mt-2'>
            {t('输入您的专属兑换码，立即激活矩阵算力、扩充令牌余额或解锁高级模型使用权限。')}
          </div>
        </div>

        {/* 兑换码输入区 */}
        <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl dark:border-slate-700 p-6 md:p-6 mb-6 relative overflow-hidden'>
          {/* 装饰背景 */}
          <div className='absolute top-0 right-0 w-64 h-64 bg-gradient-to-bl from-cyan-50 to-transparent dark:from-cyan-950/30 rounded-bl-full -mr-16 -mt-16 pointer-events-none' />
          <div className='relative z-10'>
            <div className='flex items-center gap-2 mb-1'>
              <div className='text-[14px] font-medium text-[#94A3B8] dark:text-slate-200'>
                {t('输入您的兑换码')}
              </div>
            </div>
            <Text type='tertiary' size='small' className='block mb-6'>
              {t('兑换码由平台活动发放，可兑换余额额度或专属权益')}
            </Text>

            <div className='flex flex-col sm:flex-row gap-4'>
              <Input
                value={redemptionCode}
                onChange={setRedemptionCode}
                placeholder={t('请输入兑换码')}
                size='large'
                className='flex-1 !text-lg [&_.semi-input-wrapper]:!rounded-xl [&_.semi-input-wrapper]:!bg-[var(--semi-color-bg-0)] [&_.semi-input-wrapper]:!border-[var(--semi-color-border)] [&_.semi-input-wrapper_input]:!text-[var(--semi-color-text-0)] [&_.semi-input-clearBtn]:!text-[var(--semi-color-text-2)]'
                style={{ fontFamily: 'monospace', letterSpacing: '0.08em' }}
                onEnterPress={handleRedeem}
                showClear
              />
              <Button
                size='large'
                loading={isSubmitting}
                onClick={handleRedeem}
                className='!rounded-xl !min-w-[160px] !font-bold !text-base hover:!shadow-lg hover:!-translate-y-0.5 !transition-all'
                style={{
                  background:
                    'linear-gradient(135deg, #09fef7 0%, #f8ff15 100%)',
                  borderColor: 'transparent',
                  color: '#000',
                }}
              >
                {t('立即兑换')}
              </Button>
            </div>
            <div className='mt-5 flex items-center justify-between'>
              <div className='flex items-center gap-1.5'>
                <div className='text-[14px] text-[#64748B] dark:text-slate-200'>
                  {t('兑换成功后权益将实时发放至您的账户，请注意查看“到账状态”。')}
                </div>
              </div>
              {topUpLink && (
                <Text type='tertiary' size='small'>
                  {t('在找兑换码？')}
                  <Text
                    type='secondary'
                    underline
                    className='cursor-pointer'
                    onClick={openTopUpLink}
                    size='small'
                  >
                    {t('购买兑换码')}
                  </Text>
                </Text>
              )}
            </div>
          </div>
        </div>

        {/* 我的兑换记录 */}
        <div className='rounded-2xl border-slate-200 dark:border-slate-700 mb-6'>
          {/* 标题栏 */}
          <div className='px-4 py-5 border-b border-slate-100 dark:border-slate-700 flex justify-between items-center'>
            <div className='flex items-center gap-2'>
              <div className='text-[30px] text-[#475569] font-medium'>
                {t('我的兑换记录')}
              </div>
            </div>
            <span className='text-xs text-slate-500 dark:text-slate-400 bg-slate-100 dark:bg-slate-800 px-2.5 py-1 rounded-md'>
              {t('共')} {STATIC_RECORDS.length} {t('条记录')}
            </span>
          </div>

          {/* 筛选栏 */}
          {/* <div className='px-6 py-4 border-b border-slate-100 dark:border-slate-700 bg-slate-50/60 dark:bg-slate-900/60'>
            <div className='flex flex-col md:flex-row gap-3 items-center'>
              <DatePicker
                type='dateRange'
                density='compact'
                placeholder={[t('开始时间'), t('结束时间')]}
                className='md:!min-w-[260px] [&_.semi-datepicker-range-input]:!rounded-lg [&_.semi-datepicker-range-input]:!bg-[var(--semi-color-bg-0)] [&_.semi-datepicker-range-input]:!border-[var(--semi-color-border)] [&_.semi-datepicker-range-input_input]:!text-[var(--semi-color-text-0)] [&_.semi-datepicker-range-input_input]:!cursor-pointer [&_.semi-datepicker-range-input-separator]:!text-[var(--semi-color-text-2)] [&_.semi-input-suffix]:!text-[var(--semi-color-text-2)]'
              />
              <Select
                placeholder={t('全部状态')}
                className='md:!min-w-[130px] !cursor-pointer [&_.semi-select]:!rounded-lg [&_.semi-select]:!bg-[var(--semi-color-bg-0)] [&_.semi-select]:!border-[var(--semi-color-border)] [&_.semi-select-selection-text]:!text-[var(--semi-color-text-0)] [&_.semi-select-placeholder]:!text-[var(--semi-color-text-2)] [&_.semi-select-arrow]:!text-[var(--semi-color-text-2)]'
                optionList={[
                  { value: '', label: t('全部状态') },
                  { value: 'arrived', label: t('已到账') },
                  { value: 'pending', label: t('处理中') },
                  { value: 'failed', label: t('失败') },
                ]}
              />
              <Select
                placeholder={t('全部类型')}
                className='md:!min-w-[130px] !cursor-pointer [&_.semi-select]:!rounded-lg [&_.semi-select]:!bg-[var(--semi-color-bg-0)] [&_.semi-select]:!border-[var(--semi-color-border)] [&_.semi-select-selection-text]:!text-[var(--semi-color-text-0)] [&_.semi-select-placeholder]:!text-[var(--semi-color-text-2)] [&_.semi-select-arrow]:!text-[var(--semi-color-text-2)]'
                optionList={[
                  { value: '', label: t('全部类型') },
                  { value: 'quota', label: t('额度') },
                  { value: 'benefit', label: t('权益') },
                ]}
              />
              <Button
                icon={<Search size={14} />}
                className='!rounded-lg !font-semibold'
                style={{
                  background:
                    'linear-gradient(135deg, #09fef7 0%, #f8ff15 100%)',
                  borderColor: 'transparent',
                  color: '#000',
                }}
              >
                {t('查询')}
              </Button>
              <Button
                type='tertiary'
                theme='outline'
                icon={<RotateCcw size={14} />}
                className='!rounded-lg [&_.semi-button-content]:!text-[var(--semi-color-text-1)]' style={{ backgroundColor: 'var(--semi-color-bg-0)', borderColor: 'var(--semi-color-border)' }}
              >
                {t('重置')}
              </Button>
            </div>
          </div> */}

          {/* 表格 */}
          <Table
            columns={columns}
            dataSource={STATIC_RECORDS}
            rowKey='id'
          />
        </div>

        {/* 获取更多兑换码 */}
        <div className='mb-6'>
          <div className='flex items-center gap-2 mb-4'>
            <div className='text-[30px] text-[#475569] font-medium'>
              {t('获取更多兑换码')}
            </div>
          </div>
          <div className='grid grid-cols-1 md:grid-cols-3 gap-5'>
            {promoCards.map((card, index) => (
              <div
                key={index}
                className={`group bg-white dark:bg-[#FFFFFF08] dark:border-gray-800 rounded-2xl border p-6 cursor-pointer`}
                style={{backgroundImage:`url(${card.bgImgUrl})`, backgroundPosition: 'bottom right', backgroundRepeat: 'no-repeat'}}
                onClick={() => {
                  if (card.link) window.location.href = card.link;
                }}
              >
                <div className='text-[12px] text-[#1CDFD5] font-bold mb-2'>{card.subTitle}</div>
                <div className='text-[20px] text-[#475569] font-bold mb-2'>{card.title}</div>
                <div className='text-[14px] text-[#94A3B8] mb-4'>{card.desc}</div>
                <span className="text-[12px] text-[#1CDFD5] font-bold flex items-center">
                  {card.linkText} <ArrowRight size={14} className='inline-block ml-1 transition-transform group-hover:translate-x-1' />
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
};

export default Exchange;
