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
  Wallet,
  ChevronDown,
  Link as LinkIcon,
  Mail,
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
import { isAdmin } from '../../helpers/utils';
import TransferModal from '../topup/modals/TransferModal';
import InviteDetailModal from './modals/InviteDetailModal';
import './invitation.css';

const { Text } = Typography;

const Invitation = () => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);

  const [affLink, setAffLink] = useState('');
  const [openTransfer, setOpenTransfer] = useState(false);
  const [openInviteDetail, setOpenInviteDetail] = useState(false);
  const [selectedInvite, setSelectedInvite] = useState(null);
  const [expandedFaq, setExpandedFaq] = useState(null);
  const [inviteList, setInviteList] = useState([]);

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

  // 前端只触发全额划转，实际金额由后端在事务锁内读取，避免提交过期额度。
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

  // 统计卡片（图标 + 数值 + 说明）
  const statCards = [
    {
      key: 'count',
      title: t('成功邀请人数'),
      value: affCount,
      label: t('累计邀请人数'),
      icon: Users,
      iconClass: 'inv-stat-card-icon--green',
    },
    {
      key: 'history',
      title: t('累计总收益'),
      value: formatDisplayMoney(affHistoryQuotaDisplay, displaySymbol),
      label: t('历史奖励总和'),
      icon: TrendingUp,
      iconClass: 'inv-stat-card-icon--violet',
    },
    {
      key: 'withdrawable',
      title: t('待提取收益'),
      value: formatDisplayMoney(affQuotaDisplay, displaySymbol),
      label: t('待提现余额'),
      icon: Wallet,
      iconClass: 'inv-stat-card-icon--orange',
    },
  ];

  // 邀请明细表格列
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
        <div className='inv-reward-cell'>
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
          className='inv-view-btn'
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

  // 奖励规则 (静态)
  const rewardRules = [
    {
      num: 1,
      title: t('邀新激励'),
      desc: t('邀请新用户完成注册,即可获得单笔邀新激励。'),
    },
    {
      num: 2,
      title: t('可持续增收'),
      desc: t('加入合伙人计划,可获得受邀用户算力消耗额外激励。'),
    },
    {
      num: 3,
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

  return (
    <div className='invitation-page'>
      <div className='inv-container'>
        {/* 页头 */}
        <div className='inv-page-head'>
          <div>
            <h1>{t('邀请奖励')}</h1>
            <p className='sub'>{t('邀请好友注册消费，获得现金奖励')}</p>
          </div>
        </div>

        {/* 推广链接卡片 */}
        <div className='inv-promo-card'>
          <div className='inv-promo-card-head'>
            <LinkIcon size={20} />
            <h3>{t('您的专属邀请链接')}</h3>
          </div>
          <div className='inv-promo-link-row'>
            <Input
              value={affLink}
              size='large'
              readonly
              className='inv-promo-link-input'
              placeholder={t('加载中...')}
            />
            <Button
              type='primary'
              theme='solid'
              size='large'
              onClick={handleAffLinkClick}
              icon={<Copy size={15} />}
            >
              {t('复制链接')}
            </Button>
          </div>
          <div className='inv-share-row'>
            <span className='inv-share-label'>{t('快速分享：')}</span>
            <button
              type='button'
              className='inv-share-btn'
              aria-label={t('分享到微信')}
            >
              <SiWechat size={18} />
            </button>
            <button
              type='button'
              className='inv-share-btn'
              aria-label={t('分享到 X')}
            >
              <SiX size={18} />
            </button>
            <button
              type='button'
              className='inv-share-btn'
              aria-label={t('通过邮件分享')}
            >
              <Mail size={18} />
            </button>
          </div>
        </div>

        {/* 统计概览 */}
        <section className='inv-stats-grid'>
          {statCards.map((card) => {
            const Icon = card.icon;
            return (
              <div key={card.key} className='inv-stat-card'>
                <div className='inv-stat-card-head'>
                  <span className='inv-stat-card-title'>{card.title}</span>
                  <span className={`inv-stat-card-icon ${card.iconClass}`}>
                    <Icon size={18} />
                  </span>
                </div>
                <div className='inv-stat-card-value'>{card.value}</div>
                <div className='inv-stat-card-change'>
                  <span className='inv-change-label'>{card.label}</span>
                </div>
              </div>
            );
          })}
        </section>

        {/* 提现到钱包 */}
        <div className='inv-withdraw-row'>
          <Button
            type='primary'
            theme='solid'
            className='inv-withdraw-btn'
            icon={<Wallet size={16} />}
            disabled={!affQuota || affQuota <= 0}
            onClick={() => setOpenTransfer(true)}
          >
            {t('提现')} {formatDisplayMoney(affQuotaDisplay, displaySymbol)}{' '}
            {t('到钱包')}
          </Button>
        </div>

        {/* 奖励规则步骤 */}
        <div className='inv-rule-steps'>
          {rewardRules.map((rule) => (
            <div key={rule.num} className='inv-rule-step'>
              <div className='inv-rule-step-num'>0{rule.num}</div>
              <h4>{rule.title}</h4>
              <p>{rule.desc}</p>
            </div>
          ))}
        </div>

        {/* 邀请明细表格 */}
        <div className='inv-table-card'>
          <div className='inv-card-title'>{t('邀请明细')}</div>
          <div className='inv-table-wrap'>
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
          </div>
          <div className='inv-pagination'>
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

        {/* 常见问题 */}
        <div className='inv-faq-card'>
          <div className='inv-card-title'>{t('常见问题')}</div>
          <div className='inv-faq-list'>
            {faqItems.map((faq, index) => (
              <div key={index} className='inv-faq-item'>
                <div className='inv-faq-q' onClick={() => toggleFaq(index)}>
                  <span>{faq.q}</span>
                  <ChevronDown
                    size={16}
                    className={expandedFaq === index ? 'is-open' : ''}
                  />
                </div>
                <Collapsible isOpen={expandedFaq === index}>
                  <p className='inv-faq-a'>{faq.a}</p>
                </Collapsible>
              </div>
            ))}
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
