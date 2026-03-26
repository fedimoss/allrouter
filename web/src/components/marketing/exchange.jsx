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
  Ticket,
  Gift,
  Zap,
  Info,
  History,
  Search,
  RotateCcw,
  Users,
  QrCode,
  Trophy,
  ArrowRight,
  Sparkles,
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
      icon: <Users size={24} className='text-purple-600 dark:text-purple-400' />,
      title: t('邀请好友得兑换码'),
      desc: t('每成功邀请一位好友注册，可获得一张兑换码奖励'),
      link: '/console/invitation',
      linkText: t('立即邀请'),
      gradient:
        'from-purple-50 to-pink-50 dark:from-purple-950/40 dark:to-pink-950/40',
      border: 'border-purple-100 dark:border-purple-800/50',
      linkColor: 'text-purple-600 dark:text-purple-400',
    },
    {
      icon: <QrCode size={24} className='text-green-600 dark:text-green-400' />,
      title: t('关注公众号领码'),
      desc: t('关注官方公众号，回复关键词领取兑换码'),
      link: null,
      linkText: t('去关注'),
      gradient:
        'from-green-50 to-emerald-50 dark:from-green-950/40 dark:to-emerald-950/40',
      border: 'border-green-100 dark:border-green-800/50',
      linkColor: 'text-green-600 dark:text-green-400',
    },
    {
      icon: <Trophy size={24} className='text-orange-600 dark:text-orange-400' />,
      title: t('社区贡献奖励'),
      desc: t('在社区发布教程、提交 Bug 或贡献代码，可获得额度奖励'),
      link: null,
      linkText: t('了解详情'),
      gradient:
        'from-orange-50 to-amber-50 dark:from-orange-950/40 dark:to-amber-950/40',
      border: 'border-orange-100 dark:border-orange-800/50',
      linkColor: 'text-orange-600 dark:text-orange-400',
    },
  ];

  return (
    <div className='p-4 md:p-6'>
      <div className='max-w-6xl mx-auto'>
        {/* 页面标题 */}
        <div className='mb-6'>
          <div className='flex items-center gap-3'> 
            <Ticket size={30} style={{color:'rgb(9 254 247 / 100%)'}} />
            <Title heading={2} className='!mb-0 text-xl'>
              {t('兑换码')}
            </Title>
          </div>
          <Text type='tertiary' size='small'>
              {t('使用兑换码领取额度与权益')}
            </Text>
        </div>

        {/* 兑换码输入区 */}
        <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl border border-slate-200 dark:border-slate-700 p-6 md:p-8 mb-6 relative overflow-hidden'>
          {/* 装饰背景 */}
          <div className='absolute top-0 right-0 w-64 h-64 bg-gradient-to-bl from-cyan-50 to-transparent dark:from-cyan-950/30 rounded-bl-full -mr-16 -mt-16 pointer-events-none' />
          <div className='relative z-10'>
            <div className='flex items-center gap-2 mb-1'>
              <Gift size={20} className='text-purple-500' />
              <Text strong className='text-lg'>
                {t('输入兑换码')}
              </Text>
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
                className='flex-1 !rounded-xl !bg-slate-50 dark:!bg-slate-800 !border-slate-300 dark:!border-slate-700 !text-lg'
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
                icon={<Zap size={18} />}
              >
                {t('立即兑换')}
              </Button>
            </div>
            <div className='mt-3 flex items-center justify-between'>
              <div className='flex items-center gap-1.5'>
                <Info
                  size={14}
                  className='text-slate-400 flex-shrink-0'
                />
                <Text type='tertiary' size='small'>
                  {t('兑换码不区分大小写，输入后点击立即兑换即可领取对应权益')}
                </Text>
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
        <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl border border-slate-200 dark:border-slate-700 mb-6'>
          {/* 标题栏 */}
          <div className='px-6 py-5 border-b border-slate-100 dark:border-slate-700 flex justify-between items-center'>
            <div className='flex items-center gap-2'>
              <History size={18} className='text-cyan-500' />
              <Text strong className='text-lg'>
                {t('我的兑换记录')}
              </Text>
            </div>
            <span className='text-xs text-slate-500 dark:text-slate-400 bg-slate-100 dark:bg-slate-800 px-2.5 py-1 rounded-md'>
              {t('共')} {STATIC_RECORDS.length} {t('条记录')}
            </span>
          </div>

          {/* 筛选栏 */}
          <div className='px-6 py-4 border-b border-slate-100 dark:border-slate-700 bg-slate-50/60 dark:bg-slate-900/60'>
            <div className='flex flex-col md:flex-row gap-3 items-center'>
              <DatePicker
                type='dateRange'
                density='compact'
                placeholder={[t('开始时间'), t('结束时间')]}
                className='md:!min-w-[260px] [&_.semi-datepicker-range-input]:!bg-white dark:[&_.semi-datepicker-range-input]:!bg-slate-800 [&_.semi-datepicker-range-input]:!border-slate-200 dark:[&_.semi-datepicker-range-input]:!border-slate-700 [&_.semi-datepicker-range-input]:!rounded-lg [&_input]:!cursor-pointer'
              />
              <Select
                placeholder={t('全部状态')}
                className='!bg-white dark:!bg-slate-800 !border-slate-200 dark:!border-slate-700 !rounded-lg !cursor-pointer md:!min-w-[130px]'
                optionList={[
                  { value: '', label: t('全部状态') },
                  { value: 'arrived', label: t('已到账') },
                  { value: 'pending', label: t('处理中') },
                  { value: 'failed', label: t('失败') },
                ]}
              />
              <Select
                placeholder={t('全部类型')}
                className='!bg-white dark:!bg-slate-800 !border-slate-200 dark:!border-slate-700 !rounded-lg !cursor-pointer md:!min-w-[130px]'
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
                className='!rounded-lg !bg-white dark:!bg-slate-800 !border-slate-200 dark:!border-slate-600 !text-slate-600 dark:!text-slate-300'
              >
                {t('重置')}
              </Button>
            </div>
          </div>

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
            <Sparkles size={18} className='text-amber-500' />
            <Text strong className='text-base'>
              {t('获取更多兑换码')}
            </Text>
          </div>
          <div className='grid grid-cols-1 md:grid-cols-3 gap-5'>
            {promoCards.map((card, index) => (
              <div
                key={index}
                className={`group bg-gradient-to-br ${card.gradient} rounded-2xl border ${card.border} p-6 hover:shadow-lg hover:-translate-y-1 transition-all cursor-pointer`}
                onClick={() => {
                  if (card.link) window.location.href = card.link;
                }}
              >
                <div className='w-12 h-12 rounded-xl bg-white dark:bg-slate-800 shadow-sm flex items-center justify-center mb-4 group-hover:scale-110 transition-transform'>
                  {card.icon}
                </div>
                <Text strong className='block mb-1 text-slate-800 dark:text-slate-100'>
                  {card.title}
                </Text>
                <Text type='tertiary' size='small' className='block mb-3'>
                  {card.desc}
                </Text>
                <span
                  className={`text-xs font-semibold ${card.linkColor} flex items-center group-hover:translate-x-1 transition-transform`}
                >
                  {card.linkText}
                  <ArrowRight size={14} className='ml-1' />
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
