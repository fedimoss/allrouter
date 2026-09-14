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
import {
  Button,
  Input,
  Modal,
  Table,
  Empty,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { ArrowRight, Check } from 'lucide-react';
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
import './exchange.css';

const Exchange = () => {
  const { t } = useTranslation();
  const [, userDispatch] = useContext(UserContext);
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

  const refreshUser = async () => {
    const res = await API.get('/api/user/self');
    const { success, data } = res.data;
    if (success) {
      localStorage.setItem('user', JSON.stringify(data));
      userDispatch({ type: 'login', payload: data });
    }
  };

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
        await refreshUser();
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
        <span className='ex-date-cell'>
          {time ? timestamp2string(time) : '-'}
        </span>
      ),
    },
    {
      title: t('兑换码'),
      dataIndex: 'key',
      key: 'key',
      render: (text) => <code className='ex-code'>{text}</code>,
    },
    {
      title: t('类型'),
      dataIndex: 'name',
      key: 'name',
      render: (text) => (
        <span className='ex-type-cell'>{text || t('兑换码')}</span>
      ),
    },
    {
      title: t('面值 / 权益'),
      dataIndex: 'quota',
      key: 'quota',
      render: (text) => <span className='ex-quota-cell'>{text}</span>,
    },
    {
      title: t('状态'),
      key: 'status',
      render: () => (
        <span className='ex-status ex-status--active'>{t('已兑换')}</span>
      ),
    },
    {
      title: t('到账状态'),
      key: 'account_status',
      render: () => (
        <span className='ex-status ex-status--active'>{t('已到账')}</span>
      ),
    },
    {
      title: t('流水ID'),
      dataIndex: 'id',
      key: 'id',
      render: (text) => <span className='ex-mono-cell'>{text}</span>,
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
    <div className='exchange-page'>
      <div className='ex-container'>
        {/* 页头 */}
        <div className='ex-page-head'>
          <div>
            <h1>{t('兑换码')}</h1>
            <p className='sub'>{t('输入兑换码充值账户余额')}</p>
          </div>
        </div>

        {/* 兑换输入卡片 */}
        <div className='ex-hero'>
          <div className='ex-steps'>
            <div className='ex-step'>
              <div className='ex-step-icon'>1</div>
              <span>{t('输入兑换码')}</span>
            </div>
            <span className='ex-step-arrow'>→</span>
            <div className='ex-step'>
              <div className='ex-step-icon'>2</div>
              <span>{t('点击兑换')}</span>
            </div>
            <span className='ex-step-arrow'>→</span>
            <div className='ex-step'>
              <div className='ex-step-icon'>3</div>
              <span>{t('余额到账')}</span>
            </div>
          </div>
          <div className='ex-input-row'>
            <Input
              value={redemptionCode}
              onChange={setRedemptionCode}
              placeholder={t('请输入兑换码')}
              size='large'
              onEnterPress={handleRedeem}
              showClear
            />
            <Button
              size='large'
              loading={isSubmitting}
              onClick={handleRedeem}
              icon={<Check size={15} />}
            >
              {t('立即兑换')}
            </Button>
          </div>
          <p className='ex-hint'>
            {t('兑换码由平台活动发放，兑换成功后余额将实时到账')}
          </p>
          {topUpLink && (
            <p className='ex-hint'>
              {t('在找兑换码？')}
              <span className='ex-link' onClick={openTopUpLink}>
                {t('购买兑换码')}
              </span>
            </p>
          )}
        </div>

        {/* 兑换记录表格 */}
        <div className='ex-table-card'>
          <div className='ex-card-head'>
            <div className='ex-card-title'>{t('我的兑换记录')}</div>
            <span className='ex-count-badge'>
              {t('共')} {recordsTotal} {t('条记录')}
            </span>
          </div>
          <div className='ex-table-wrap'>
            <Table
              columns={columns}
              dataSource={records}
              loading={recordsLoading}
              rowKey='id'
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
                  description={t('暂无兑换记录')}
                  style={{ padding: 30 }}
                />
              }
            />
          </div>
        </div>

        {/* 获取更多兑换码 */}
        <div className='ex-promo-section'>
          <div className='ex-section-title'>{t('获取更多兑换码')}</div>
          <div className='ex-promo-grid'>
            {promoCards.map((card, index) => (
              <div
                key={index}
                className='ex-promo-card'
                style={{ backgroundImage: `url(${card.bgImgUrl})` }}
                onClick={() => {
                  if (card.link) window.location.href = card.link;
                }}
              >
                <div className='ex-promo-sub'>{card.subTitle}</div>
                <div className='ex-promo-title'>{card.title}</div>
                <div className='ex-promo-desc'>{card.desc}</div>
                <span className='ex-promo-link'>
                  {card.linkText}
                  <ArrowRight size={14} />
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
