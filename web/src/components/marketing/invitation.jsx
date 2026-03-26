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

import React, { useEffect, useState, useContext, useRef } from 'react';
import {
  Typography,
  Button,
  Input,
  Table,
  Empty,
  Tag,
  Collapsible,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  Copy,
  Users,
  TrendingUp,
  Gift,
  Wallet,
  HelpCircle,
  ChevronDown,
  ArrowLeftRight,
  Link as LinkIcon,
  Mail,
  Info,
  List,
} from 'lucide-react';
import { SiWechat, SiX } from 'react-icons/si';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../../context/User';
import {
  API,
  showError,
  showSuccess,
  renderQuota,
  copy,
  getQuotaPerUnit,
} from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import TransferModal from '../topup/modals/TransferModal';

const { Text, Title } = Typography;

const Invitation = () => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const isMobile = useIsMobile();

  const [affLink, setAffLink] = useState('');
  const [openTransfer, setOpenTransfer] = useState(false);
  const [transferAmount, setTransferAmount] = useState(getQuotaPerUnit());
  const [expandedFaq, setExpandedFaq] = useState(null);

  const affFetchedRef = useRef(false);

  const getAffLink = async () => {
    try {
      const res = await API.get('/api/user/aff');
      const { success, message, data } = res.data;
      if (success) {
        setAffLink(`${window.location.origin}/register?aff=${data}`);
      } else {
        showError(message);
      }
    } catch {
      showError(t('获取邀请链接失败'));
    }
  };

  const getUserQuota = async () => {
    const res = await API.get('/api/user/self');
    const { success, message, data } = res.data;
    if (success) {
      userDispatch({ type: 'login', payload: data });
    } else {
      showError(message);
    }
  };

  const transfer = async () => {
    if (transferAmount < getQuotaPerUnit()) {
      showError(t('划转金额最低为') + ' ' + renderQuota(getQuotaPerUnit()));
      return;
    }
    const res = await API.post('/api/user/aff_transfer', {
      quota: transferAmount,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(message);
      setOpenTransfer(false);
      getUserQuota().then();
    } else {
      showError(message);
    }
  };

  const handleTransferCancel = () => {
    setOpenTransfer(false);
  };

  const handleAffLinkClick = async () => {
    await copy(affLink);
    showSuccess(t('邀请链接已复制到剪切板'));
  };

  useEffect(() => {
    getUserQuota().then();
    setTransferAmount(getQuotaPerUnit());
  }, []);

  useEffect(() => {
    if (affFetchedRef.current) return;
    affFetchedRef.current = true;
    getAffLink().then();
  }, []);

  const affQuota = userState?.user?.aff_quota || 0;
  const affHistoryQuota = userState?.user?.aff_history_quota || 0;
  const affCount = userState?.user?.aff_count || 0;

  // 邀请明细表格列
  const inviteColumns = [
    {
      title: t('用户'),
      dataIndex: 'username',
      key: 'username',
      render: (text) => <Text strong>{text}</Text>,
    },
    {
      title: t('注册时间'),
      dataIndex: 'register_time',
      key: 'register_time',
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      key: 'status',
      render: (status) => {
        if (status === 'active') {
          return (
            <Tag color='green' size='small'>
              {t('已激活')}
            </Tag>
          );
        }
        return (
          <Tag color='orange' size='small'>
            {t('待激活')}
          </Tag>
        );
      },
    },
    {
      title: t('贡献奖励'),
      dataIndex: 'reward',
      key: 'reward',
      render: (reward) => (
        <Text type={reward > 0 ? 'success' : 'tertiary'}>
          {reward > 0 ? renderQuota(reward) : '-'}
        </Text>
      ),
    },
  ];

  // 奖励规则 (静态)
  const rewardRules = [
    {
      num: 1,
      color: 'text-blue-600 bg-blue-50 dark:bg-blue-900/30',
      title: t('基础奖励'),
      desc: t(
        '每成功邀请一位好友注册并完成首次充值（任意金额），您即可获得 $5.00 奖励额度。',
      ),
    },
    {
      num: 2,
      color: 'text-purple-600 bg-purple-50 dark:bg-purple-900/30',
      title: t('奖励发放'),
      desc: t(
        '奖励将在好友充值成功后的 5 分钟内自动发放到您的"待使用收益"账户。',
      ),
    },
    {
      num: 3,
      color: 'text-green-600 bg-green-50 dark:bg-green-900/30',
      title: t('如何使用'),
      desc: t(
        '点击"划转到余额"按钮，即可将奖励额度转入主钱包，用于抵扣 API 调用费用。',
      ),
    },
  ];

  // FAQ (静态)
  const faqItems = [
    {
      q: t('好友必须充值多少才能获奖？'),
      a: t('好友完成任意金额的首次充值即可触发奖励，没有最低充值金额限制。'),
    },
    {
      q: t('邀请奖励的比例是多少？'),
      a: t('具体奖励比例由平台设置，请参考平台公告或联系客服了解详情。'),
    },
    {
      q: t('奖励额度可以提现吗？'),
      a: t(
        '奖励额度不支持直接提现，但可以划转到账户余额中用于API调用消费。',
      ),
    },
    {
      q: t('邀请人数有上限吗？'),
      a: t('没有上限，您可以邀请任意数量的好友，邀请越多奖励越多。'),
    },
  ];

  const toggleFaq = (index) => {
    setExpandedFaq(expandedFaq === index ? null : index);
  };

  return (
    <div className='p-4 md:p-6'>
      {/* 页面标题区域 */}
      <div className='mb-6'>
        <div className='flex items-center gap-3 mb-2'>
          <div className='w-10 h-10 rounded-xl bg-gradient-to-br from-pink-400 to-purple-500 flex items-center justify-center'>
            <Gift size={20} className='text-white' />
          </div>
          <div>
            <Title heading={4} className='!mb-0'>
              {t('邀请奖励计划')}
            </Title>
          </div>
        </div>
        <Text type='tertiary' className='mt-1 block'>
          {t(
            '分享您的专属链接，邀请好友加入。好友充值后您将获得丰厚奖励，多邀多得，上不封顶！',
          )}
        </Text>
      </div>

      {/* 顶部统计卡片 */}
      <div className='grid grid-cols-1 md:grid-cols-3 gap-4 mb-6'>
        {/* 待使用收益 */}
        <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl p-5 border border-slate-200 dark:border-slate-700'>
          <div className='flex items-start justify-between mb-3'>
            <span className='text-sm text-slate-500 dark:text-slate-400'>
              {t('待使用收益')}
            </span>
            <div className='w-8 h-8 rounded-lg bg-blue-50 dark:bg-blue-900/30 flex items-center justify-center'>
              <Wallet size={16} className='text-blue-500' />
            </div>
          </div>
          <div className='text-2xl md:text-3xl font-bold text-slate-900 dark:text-semi-color-text-0 mb-1'>
            {renderQuota(affQuota)}
          </div>
          <Text type='tertiary' size='small'>
            {t('可立即划转至余额')}
          </Text>
          <Button
            block
            type='tertiary'
            theme='outline'
            className='!mt-4 !rounded-lg'
            disabled={!affQuota || affQuota <= 0}
            onClick={() => setOpenTransfer(true)}
            icon={<ArrowLeftRight size={14} />}
          >
            {t('划转到余额')}
          </Button>
        </div>

        {/* 累计总收益 */}
        <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl p-5 border border-slate-200 dark:border-slate-700'>
          <div className='flex items-start justify-between mb-3'>
            <span className='text-sm text-slate-500 dark:text-slate-400'>
              {t('累计总收益')}
            </span>
            <div className='w-8 h-8 rounded-lg bg-green-50 dark:bg-green-900/30 flex items-center justify-center'>
              <TrendingUp size={16} className='text-green-500' />
            </div>
          </div>
          <div className='text-2xl md:text-3xl font-bold text-slate-900 dark:text-semi-color-text-0 mb-1'>
            {renderQuota(affHistoryQuota)}
          </div>
          <Text type='tertiary' size='small'>
            {t('历史所有奖励总和')}
          </Text>
        </div>

        {/* 成功邀请人数 */}
        <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl p-5 border border-slate-200 dark:border-slate-700'>
          <div className='flex items-start justify-between mb-3'>
            <span className='text-sm text-slate-500 dark:text-slate-400'>
              {t('成功邀请人数')}
            </span>
            <div className='w-8 h-8 rounded-lg bg-purple-50 dark:bg-purple-900/30 flex items-center justify-center'>
              <Users size={16} className='text-purple-500' />
            </div>
          </div>
          <div className='text-2xl md:text-3xl font-bold text-slate-900 dark:text-semi-color-text-0 mb-1'>
            {affCount}
          </div>
          <Text type='tertiary' size='small'>
            {t('已注册并完成首充的好友')}
          </Text>
        </div>
      </div>

      {/* 主内容区：左右布局 */}
      <div
        className={`grid gap-6 ${isMobile ? 'grid-cols-1' : 'grid-cols-3'}`}
      >
        {/* 左侧：邀请链接 + 邀请明细 */}
        <div className={isMobile ? '' : 'col-span-2'}>
          {/* 邀请链接卡片 */}
          <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl border border-slate-200 dark:border-slate-700 p-5 mb-6'>
            <div className='flex items-center gap-2 mb-4'>
              <LinkIcon size={18} className='text-cyan-500' />
              <Text strong className='text-base'>
                {t('您的专属邀请链接')}
              </Text>
            </div>
            <div className='flex gap-2'>
              <Input
                value={affLink}
                readonly
                className='flex-1 !rounded-lg'
                placeholder={t('加载中...')}
              />
              <Button
                theme='solid'
                onClick={handleAffLinkClick}
                icon={<Copy size={14} />}
                className='!rounded-lg'
                style={{
                  background: 'linear-gradient(135deg, #34d399, #a3e635)',
                  borderColor: 'transparent',
                  color: '#fff',
                }}
              >
                {t('复制链接')}
              </Button>
            </div>
            <div className='mt-3 flex items-start gap-2'>
              <Info
                size={14}
                className='text-slate-400 mt-0.5 flex-shrink-0'
              />
              <Text type='tertiary' size='small'>
                {t('好友通过此链接注册并首次充值后，奖励将自动发放。')}
              </Text>
            </div>

            {/* 分享按钮 */}
            <div className='grid grid-cols-3 gap-3 mt-4'>
              <Button
                block
                type='tertiary'
                theme='outline'
                className='!rounded-lg'
                icon={<SiWechat size={14} />}
              >
                {t('微信分享')}
              </Button>
              <Button
                block
                type='tertiary'
                theme='outline'
                className='!rounded-lg'
                icon={<SiX size={14} />}
              >
                {t('Twitter 分享')}
              </Button>
              <Button
                block
                type='tertiary'
                theme='outline'
                className='!rounded-lg'
                icon={<Mail size={14} />}
              >
                {t('邮件邀请')}
              </Button>
            </div>
          </div>

          {/* 邀请明细 */}
          <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl border border-slate-200 dark:border-slate-700 p-5'>
            <div className='flex items-center justify-between mb-4'>
              <div className='flex items-center gap-2'>
                <List size={18} className='text-blue-500' />
                <Text strong className='text-base'>
                  {t('邀请明细')}
                </Text>
              </div>
              <Tag color='blue' size='small'>
                {t('最近 30 天')}
              </Tag>
            </div>
            <Table
              columns={inviteColumns}
              dataSource={[]}
              rowKey='id'
              pagination={false}
              size='small'
              empty={
                <Empty
                  image={
                    <IllustrationNoResult
                      style={{ width: 120, height: 120 }}
                    />
                  }
                  darkModeImage={
                    <IllustrationNoResultDark
                      style={{ width: 120, height: 120 }}
                    />
                  }
                  description={t('暂无邀请记录')}
                  style={{ padding: 30 }}
                />
              }
            />
          </div>
        </div>

        {/* 右侧：奖励规则 + 常见问题 */}
        <div className={isMobile ? '' : 'col-span-1'}>
          {/* 奖励规则 */}
          <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl border border-slate-200 dark:border-slate-700 p-5 mb-6'>
            <div className='flex items-center gap-2 mb-5'>
              <HelpCircle size={18} className='text-blue-500' />
              <Text strong className='text-base'>
                {t('奖励规则')}
              </Text>
            </div>
            <div className='space-y-5'>
              {rewardRules.map((rule) => (
                <div key={rule.num}>
                  <div className='flex items-center gap-2 mb-2'>
                    <span
                      className={`w-6 h-6 rounded-full ${rule.color} flex items-center justify-center text-xs font-bold flex-shrink-0`}
                    >
                      {rule.num}
                    </span>
                    <Text strong>{rule.title}</Text>
                  </div>
                  <Text
                    type='tertiary'
                    size='small'
                    className='block pl-8'
                  >
                    {rule.desc}
                  </Text>
                </div>
              ))}
            </div>
          </div>

          {/* 常见问题 */}
          <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl border border-slate-200 dark:border-slate-700 p-5'>
            <div className='flex items-center gap-2 mb-4'>
              <HelpCircle size={18} className='text-amber-500' />
              <Text strong className='text-base'>
                {t('常见问题')}
              </Text>
            </div>
            <div className='space-y-0'>
              {faqItems.map((faq, index) => (
                <div
                  key={index}
                  className='border-b border-slate-100 dark:border-slate-700 last:border-b-0'
                >
                  <div
                    className='flex items-center justify-between py-3 cursor-pointer'
                    onClick={() => toggleFaq(index)}
                  >
                    <Text size='small'>{faq.q}</Text>
                    <ChevronDown
                      size={16}
                      className={`text-slate-400 flex-shrink-0 ml-2 transition-transform duration-200 ${expandedFaq === index ? 'rotate-180' : ''}`}
                    />
                  </div>
                  <Collapsible isOpen={expandedFaq === index}>
                    <Text
                      type='tertiary'
                      size='small'
                      className='block pb-3'
                    >
                      {faq.a}
                    </Text>
                  </Collapsible>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* 划转弹窗 */}
      <TransferModal
        t={t}
        openTransfer={openTransfer}
        transfer={transfer}
        handleTransferCancel={handleTransferCancel}
        userState={userState}
        renderQuota={renderQuota}
        getQuotaPerUnit={getQuotaPerUnit}
        transferAmount={transferAmount}
        setTransferAmount={setTransferAmount}
      />
    </div>
  );
};

export default Invitation;
