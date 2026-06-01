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

import React, { useContext, useEffect, useState } from 'react';
import { Button, Input, Modal, Table, Typography } from '@douyinfe/semi-ui';
import { ArrowRight } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';
import {
  API,
  renderQuota,
  showError,
  showInfo,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import imgOne from '../../../public/one.png';
import imgTwo from '../../../public/two.png';
import imgThree from '../../../public/three.png';

const { Text } = Typography;

const statusBadgeClass =
  'inline-flex items-center px-2.5 py-1 bg-green-50 dark:bg-green-900/30 text-green-700 dark:text-green-400 rounded-full text-xs font-medium';

const Exchange = () => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);
  const [redemptionCode, setRedemptionCode] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [records, setRecords] = useState([]);
  const [recordsTotal, setRecordsTotal] = useState(0);
  const [recordsLoading, setRecordsLoading] = useState(false);

  const topUpLink = statusState?.status?.top_up_link || '';

  const loadRecords = async () => {
    setRecordsLoading(true);
    try {
      const res = await API.get('/api/user/redemption/self?p=1&page_size=20');
      const { success, message, data } = res.data;
      if (success) {
        setRecords(data.items || []);
        setRecordsTotal(data.total || 0);
      } else {
        showError(message);
      }
    } catch {
      showError(t('加载兑换记录失败'));
    } finally {
      setRecordsLoading(false);
    }
  };

  useEffect(() => {
    loadRecords();
  }, []);

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
          okText: t('确定'),
          cancelText: t('取消'),
        });
        if (userState.user) {
          userDispatch({
            type: 'login',
            payload: {
              ...userState.user,
              quota: userState.user.quota + data,
            },
          });
        }
        setRedemptionCode('');
        loadRecords();
      } else {
        showError(message);
      }
    } catch {
      showError(t('请求失败'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const columns = [
    {
      title: t('兑换时间'),
      dataIndex: 'redeemed_time',
      key: 'redeemed_time',
      render: (time) => (
        <span className='text-slate-600 dark:text-slate-300 whitespace-nowrap'>
          {time ? timestamp2string(time) : '-'}
        </span>
      ),
    },
    {
      title: t('兑换码'),
      dataIndex: 'key',
      key: 'key',
      render: (text) => (
        <code className='text-xs font-mono bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 px-2 py-1 rounded'>
          {text}
        </code>
      ),
    },
    {
      title: t('类型'),
      dataIndex: 'name',
      key: 'name',
      render: (text) => (
        <span className='text-slate-600 dark:text-slate-300'>
          {text || t('兑换码')}
        </span>
      ),
    },
    {
      title: t('面值 / 权益'),
      dataIndex: 'quota',
      key: 'quota',
      render: (text) => <Text strong>{text}</Text>,
    },
    {
      title: t('状态'),
      key: 'status',
      render: () => (
        <span className={statusBadgeClass}>
          <span className='w-1.5 h-1.5 rounded-full bg-green-500 mr-1.5' />
          {t('已兑换')}
        </span>
      ),
    },
    {
      title: t('到账状态'),
      key: 'account_status',
      render: () => <span className={statusBadgeClass}>{t('已到账')}</span>,
    },
    {
      title: t('流水ID'),
      dataIndex: 'id',
      key: 'id',
      render: (text) => (
        <span className='text-xs font-mono text-slate-500 dark:text-slate-400'>
          {text}
        </span>
      ),
    },
  ];

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
    <div>
      <div className='mx-full'>
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

        <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl dark:border-slate-700 p-6 md:p-6 mb-6 relative overflow-hidden'>
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
                className='flex-1 !h-[56px] !text-lg [&_.semi-input-wrapper]:!rounded-xl [&_.semi-input-wrapper]:!bg-[var(--semi-color-bg-0)] [&_.semi-input-wrapper]:!border-[var(--semi-color-border)] [&_.semi-input-wrapper_input]:!text-[var(--semi-color-text-0)] [&_.semi-input-clearBtn]:!text-[var(--semi-color-text-2)]'
                style={{ fontFamily: 'monospace', letterSpacing: '0.08em' }}
                onEnterPress={handleRedeem}
                showClear
              />
              <Button
                size='large'
                loading={isSubmitting}
                onClick={handleRedeem}
                className='!rounded-xl !h-[56px] !min-w-[160px] !font-bold !text-base hover:!shadow-lg hover:!-translate-y-0.5 !transition-all'
                style={{
                  background:
                    'var(--theme-gradient-135)',
                  borderColor: 'transparent',
                  color: 'var(--theme-primary-btn-color)',
                }}
              >
                {t('立即兑换')}
              </Button>
            </div>
            <div className='mt-5 flex items-center justify-between'>
              <div className='text-[14px] text-[#64748B] dark:text-slate-200'>
                {t('兑换成功后权益将实时发放至您的账户，请注意查看到账状态。')}
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

        <div className='rounded-2xl border-slate-200 dark:border-slate-700 mb-6'>
          <div className='px-4 py-5 border-b border-slate-100 dark:border-slate-700 flex justify-between items-center'>
            <div className='text-[30px] text-[#475569] font-medium'>
              {t('我的兑换记录')}
            </div>
            <span className='text-xs text-slate-500 dark:text-slate-400 bg-slate-100 dark:bg-slate-800 px-2.5 py-1 rounded-md'>
              {t('共')} {recordsTotal} {t('条记录')}
            </span>
          </div>

          <Table
            columns={columns}
            dataSource={records}
            loading={recordsLoading}
            rowKey='id'
          />
        </div>

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
                <div className='text-[12px] text-[color:var(--theme-primary)] font-bold mb-2'>{card.subTitle}</div>
                <div className='text-[20px] text-[#475569] font-bold mb-2'>{card.title}</div>
                <div className='text-[14px] text-[#94A3B8] mb-4'>{card.desc}</div>
                <span className="text-[12px] text-[color:var(--theme-primary)] font-bold flex items-center">
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
