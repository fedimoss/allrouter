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
  Pagination,
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
  Hammer,
} from 'lucide-react';
import { SiWechat, SiX } from 'react-icons/si';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../../context/User';
import {
  API,
  showError,
  showSuccess,
  formatDisplayMoney, // 用于将后端转换后的金额按用户币种格式化展示
  copy,
  timestamp2string,
} from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { isAdmin } from '../../helpers/utils';
import TransferModal from '../topup/modals/TransferModal';
import InviteDetailModal from './modals/InviteDetailModal';
import bannerImg from '../../../public/invite-banner.png';
import walletImg from '../../../public/wallet-balance.png';
import houseImg from '../../../public/house.png';
import invitePeopleImg from '../../../public/invite-people.png';

const { Text, Title } = Typography;

const Invitation = () => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const isMobile = useIsMobile();

  const [affLink, setAffLink] = useState('');
  const [openTransfer, setOpenTransfer] = useState(false);
  const [openInviteDetail, setOpenInviteDetail] = useState(false);
  const [selectedInvite, setSelectedInvite] = useState(null);
  const [expandedFaq, setExpandedFaq] = useState(null);
  const [inviteList, setInviteList] = useState([
    {
      username: 'testuser',
      reward: 1000,
      register_time: '2024-01-01 12:00:00',
      status: '查看',
    },
  ]);

  const [inviteLoading, setInviteLoading] = useState(false);
  const [invitePage, setInvitePage] = useState(1);
  const [invitePageSize] = useState(10);
  const [inviteTotal, setInviteTotal] = useState(0);
  const [inviteDisplaySymbol, setInviteDisplaySymbol] = useState('');
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

  const loadInviteRecords = async (page = 1, pageSize = invitePageSize) => {
    setInviteLoading(true);
    try {
      const base = isAdmin()
        ? '/api/user/aff/records'
        : '/api/user/self/aff/records';
      const res = await API.get(`${base}?p=${page}&page_size=${pageSize}`);
      const { success, message, data } = res.data;
      if (success) {
        setInviteList(data?.items || []);
        setInviteTotal(data?.total || 0);
        setInviteDisplaySymbol(data?.display_symbol || '');
      } else {
        showError(message || t('加载失败'));
      }
    } catch {
      showError(t('加载失败'));
    } finally {
      setInviteLoading(false);
    }
  };

  const transfer = async () => {
    if (!userState?.user?.aff_quota) {
      return;
    }
    const res = await API.post('/api/user/aff_transfer');
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
  }, []);

  useEffect(() => {
    if (affFetchedRef.current) return;
    affFetchedRef.current = true;
    getAffLink().then();
  }, []);

  useEffect(() => {
    loadInviteRecords(invitePage, invitePageSize).then();
  }, [invitePage, invitePageSize]);

  const affQuota = userState?.user?.aff_quota || 0; // 原始邀请额度，用于划转接口和按钮禁用判断
  const affQuotaDisplay = userState?.user?.aff_quota_display || 0; // 后端转换后的展示金额
  const affHistoryQuota = userState?.user?.aff_history_quota || 0; // 原始历史邀请收益
  const affHistoryQuotaDisplay =
    userState?.user?.aff_history_quota_display || 0; // 后端转换后的历史展示金额
  const displaySymbol = userState?.user?.display_symbol; // 用户当前币种符号（如 $ 或 ¥）
  const affCount = userState?.user?.aff_count || 0;

  // 邀请明细表格列
  const inviteColumns = [
    {
      title: t('被邀请人'),
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
      title: t('注册奖励'),
      dataIndex: 'reward',
      key: 'reward',
      render: (reward) => (
        <div className='text-[color:var(--theme-primary)] text-[14px] font-bold'>
          +{reward}
        </div>
      ),
    },
    {
      title: t('充值返利'),
      dataIndex: 'status',
      key: 'status',
      render: (row, record) => {
        return (
          <span
            className='text-[color:var(--theme-primary)] bg-[color:var(--theme-primary-20)] border border-[color:var(--theme-primary)] rounded-md px-3 py-1 text-[14px] font-bold cursor-pointer'
            onClick={() => {
              setSelectedInvite(record);
              setOpenInviteDetail(true);
            }}
          >
            {t('查看')}
          </span>
        );
      },
    },
  ];

  // 奖励规则 (静态)
  const rewardRules = [
    {
      num: 1,
      color: 'text-blue-600 bg-blue-50 dark:bg-blue-900/30',
      title: t('邀新激励'),
      desc: t('邀请新用户完成注册,即可获得单笔邀新激励。'),
    },
    {
      num: 2,
      color: 'text-purple-600 bg-purple-50 dark:bg-purple-900/30',
      title: t('可持续增收'),
      desc: t('加入合伙人计划,可获得受邀用户算力消耗额外激励。'),
    },
    {
      num: 3,
      color: 'text-green-600 bg-green-50 dark:bg-green-900/30',
      title: t('即时结算'),
      desc: t('收益实时同步至您的待领取账户,兑付及时清晰。'),
    },
  ];

  // FAQ (静态)
  const faqItems = [
    {
      q: t('如何提取奖励收益？'),
      a: t(
        '当待提取收益达到 10 USDT 时即可申请提现，系统将在 24 小时内自动处理至您的关联钱包。',
      ),
    },
    {
      q: t('邀请人数是否有上限？'),
      a: t(
        '没有上限。您可以邀请无限数量的伙伴，且所有符合规则的收益均受系统算法保护。',
      ),
    },
    {
      q: t('奖励为何没有到账？'),
      a: t(
        '请确认被邀请人是否已完成实名认证并成功启动首个算力节点。如有异常请联系技术支持。',
      ),
    },
  ];

  const toggleFaq = (index) => {
    setExpandedFaq(expandedFaq === index ? null : index);
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

  const inviteRecordsColumns = [
    {
      title: t('被邀请人'),
      dataIndex: 'invitee_name',
      key: 'invitee_name',
      render: (text) => <Text strong>{text || '-'}</Text>,
    },
    {
      title: t('注册时间'),
      dataIndex: 'register_time',
      key: 'register_time',
      render: (_, record) => safeFormatTimestamp(record?.register_time),
    },
    {
      title: t('注册奖励'),
      dataIndex: 'reward_quota',
      key: 'reward_quota',
      render: (rewardQuota) => (
        <div className='text-[color:var(--theme-primary)] text-[14px] font-bold'>
          {inviteDisplaySymbol}
          {rewardQuota ?? 0}
        </div>
      ),
    },
    {
      title: t('消费返利'),
      dataIndex: 'status',
      key: 'status',
      render: (_, record) => (
        <span
          className='text-[color:var(--theme-primary-btn-color)] bg-[color:var(--theme-primary-20)] border border-[color:var(--theme-primary)] rounded-md px-3 py-1 text-[14px] font-bold cursor-pointer'
          onClick={() => {
            setSelectedInvite(record);
            setOpenInviteDetail(true);
          }}
        >
          {t('查看')}
        </span>
      ),
    },
  ];

  return (
    <div className='p-4 md:p-6'>
      {/* 页面标题区域 */}
      <div
        className='mb-6 rounded-2xl px-4 py-6 shadow-sm'
        style={{
          backgroundImage: `url(${bannerImg})`,
          backgroundSize: 'cover',
          backgroundPosition: 'center',
        }}
      >
        <div className='p-4 text-[48px] font-500 text-white'>
          {t('邀请奖励计划')}
        </div>
        <div
          className='px-2 text-[18px] font-medium text-slate-500 dark:text-slate-400'
          style={{ width: 'min(657px,100%)' }}
        >
          {t('每成功邀请一位新用户完成注册,您将获得额外积分奖励。')}
        </div>
      </div>

      {/* 顶部统计卡片 */}
      <div className='grid grid-cols-1 md:grid-cols-3 gap-4 mb-6'>
        {/* 待提取收益 */}
        <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl p-5 border border-slate-200 dark:border-slate-700 relative'>
          <img
            src={walletImg}
            alt='wallet'
            className='absolute bottom-0 right-0 z-0 h-[70px]'
          />
          <div className='flex items-start justify-between mb-3'>
            <span className='text-[14px] text-[#94A3B8] dark:text-slate-400'>
              {t('待提取收益')}
            </span>
          </div>
          <div className='dark:text-cyan-300 mb-1 text-[color:var(--theme-primary)] font-[900] text-[30px] leading-[30px]'>
            {/* 使用后端转换值 + 用户币种符号展示待提取收益 */}
            {formatDisplayMoney(affQuotaDisplay, displaySymbol)}
          </div>
          {/* <Text type='tertiary' className='mt-2' size='small' style={{display:'block'}}>
            {t('可立即划转至余额')}
          </Text> */}
          <Button
            block
            className='!mt-4 !rounded-lg relative z-10 dark:text-cyan-300 dark:border-cyan-300'
            style={{
              width: '60%',
              border: '1px solid var(--theme-primary-20)',
              background: 'var(--theme-primary-10)',
              color: 'var(--theme-primary-btn-color)',
              padding: '18px',
              fontSize: '12px',
            }}
            disabled={!affQuota || affQuota <= 0}
            onClick={() => setOpenTransfer(true)}
          >
            {t('可立即划转至余额')}
          </Button>
        </div>

        {/* 累计总收益 */}
        <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl p-5 border border-slate-200 dark:border-slate-700 relative'>
          <img
            src={houseImg}
            alt='house'
            className='absolute bottom-0 right-0 z-0 h-[70px]'
          />
          <div className='flex items-start justify-between mb-3'>
            <span className='text-[14px] text-[#94A3B8] dark:text-slate-400'>
              {t('累计总收益')}
            </span>
          </div>
          <div className='dark:text-semi-color-text-0 mb-1 font-[900] text-[#475569] text-[30px] leading-[30px]'>
            {/* 使用后端转换值 + 用户币种符号展示累计总收益 */}
            {formatDisplayMoney(affHistoryQuotaDisplay, displaySymbol)}
          </div>
          {/* <Text type='tertiary' size='small'>
            {t('历史所有奖励总和')}
          </Text> */}
        </div>

        {/* 成功邀请人数 */}
        <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl p-5 border border-slate-200 dark:border-slate-700 relative'>
          <img
            src={invitePeopleImg}
            alt='invitePeople'
            className='absolute bottom-0 right-0 z-0 h-[70px]'
          />
          <div className='flex items-start justify-between mb-3'>
            <span className='text-[14px] text-[#94A3B8] dark:text-slate-400'>
              {t('成功邀请人数')}
            </span>
          </div>
          <div className='dark:text-semi-color-text-0 mb-1 font-[900] text-[#475569] text-[30px] leading-[30px]'>
            {affCount}
          </div>
          {/* <Text type='tertiary' size='small'>
            {t('已注册并完成首充的好友')}
          </Text> */}
        </div>
      </div>

      {/* 主内容区：左右布局 */}
      <div className={`grid gap-6 ${isMobile ? 'grid-cols-1' : 'grid-cols-3'}`}>
        {/* 左侧：邀请链接 + 邀请明细 */}
        <div className={isMobile ? '' : 'col-span-2'}>
          {/* 邀请链接卡片 */}
          <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl border border-slate-200 dark:border-slate-700 p-5 mb-6'>
            <div className='flex items-center gap-2 mb-4'>
              <div strong className='font-bold text-[20px]'>
                {t('您的专属邀请链接')}
              </div>
            </div>
            <div className='flex gap-2'>
              <Input
                value={affLink}
                size='large'
                readonly
                className='flex-1 !rounded-lg dark:bg-slate-900 dark:border-slate-700'
                placeholder={t('加载中...')}
                style={{ fontSize: '16px' }}
              />
              <Button
                theme='solid'
                size='large'
                onClick={handleAffLinkClick}
                icon={<Copy size={14} />}
                className='!rounded-lg'
                style={{
                  background: 'var(--theme-gradient-135)',
                  borderColor: 'transparent',
                  color: 'var(--theme-primary-btn-color)',
                }}
              >
                {t('复制链接')}
              </Button>
            </div>
            {/* <div className='mt-3 flex items-start gap-2'>
              <Info
                size={14}
                className='text-slate-400 mt-0.5 flex-shrink-0'
              />
              <Text type='tertiary' size='small'>
                {t('好友通过此链接注册并首次充值后，奖励将自动发放。')}
              </Text>
            </div> */}

            {/* 分享按钮 */}
            <div className='flex items-center gap-3 mt-8'>
              <span className='text-sm text-slate-500 dark:text-slate-400'>
                {t('快速分享：')}
              </span>
              <Button
                size='large'
                type='tertiary'
                className='!rounded-lg'
                icon={<SiWechat size={18} />}
              ></Button>
              <Button
                size='large'
                type='tertiary'
                className='!rounded-lg'
                icon={<SiX size={18} />}
              ></Button>
              <Button
                size='large'
                type='tertiary'
                className='!rounded-lg'
                icon={<Mail size={18} />}
              ></Button>
            </div>
          </div>

          {/* 邀请明细 */}
          <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl border border-slate-200 dark:border-slate-700 p-5'>
            <div className='flex items-center justify-between mb-4'>
              <div className='flex items-center gap-2'>
                <div className='text-[18px] font-medium'>{t('邀请明细')}</div>
              </div>
            </div>
            <Table
              columns={inviteRecordsColumns}
              dataSource={inviteList}
              rowKey='id'
              pagination={false}
              loading={inviteLoading}
              empty={
                <Empty
                  image={
                    <IllustrationNoResult style={{ width: 120, height: 120 }} />
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
            <div className='mt-4 flex justify-end'>
              <Pagination
                total={inviteTotal}
                pageSize={invitePageSize}
                currentPage={invitePage}
                onPageChange={setInvitePage}
                showSizeChanger={false}
                size='small'
              />
            </div>
          </div>
        </div>

        {/* 右侧：奖励规则 + 常见问题 */}
        <div className={isMobile ? '' : 'col-span-1'}>
          {/* 奖励规则 */}
          <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl border border-slate-200 dark:border-slate-700 p-5 mb-6'>
            <div className='flex items-center gap-2 mb-5'>
              <Hammer
                className='w-5 h-5'
                style={{ color: 'var(--theme-primary)' }}
              />
              <div className='font-medium text-[18px]'>{t('奖励规则')}</div>
            </div>
            <div className='space-y-5'>
              {rewardRules.map((rule) => (
                <div key={rule.num}>
                  <div className='flex items-center gap-2 mb-2'>
                    <span
                      className={`w-6 h-6 flex text-cyan-300 items-center justify-center text-xs font-[900] flex-shrink-0`}
                    >
                      0{rule.num}
                    </span>
                    <Text strong>{rule.title}</Text>
                  </div>
                  <Text type='tertiary' size='small' className='block pl-8'>
                    {rule.desc}
                  </Text>
                </div>
              ))}
            </div>
          </div>

          {/* 常见问题 */}
          <div className='bg-white dark:bg-semi-color-bg-1 rounded-2xl border border-slate-200 dark:border-slate-700 p-5'>
            <div className='flex items-center gap-2 mb-4'>
              <div className='font-medium text-[18px]'>{t('常见问题')}</div>
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
                    <Text type='tertiary' size='small' className='block pb-3'>
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
      />

      {/* 邀请明细弹窗 */}
      <InviteDetailModal
        t={t}
        visible={openInviteDetail}
        onClose={() => setOpenInviteDetail(false)}
        inviteeId={selectedInvite?.invitee_id}
        inviteeName={selectedInvite?.invitee_name}
      />
    </div>
  );
};

export default Invitation;
