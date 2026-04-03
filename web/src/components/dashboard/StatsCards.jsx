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
import { Card, Progress, Skeleton, Tag } from '@douyinfe/semi-ui';
import { useNavigate } from 'react-router-dom';
import {
  CircleCheck,
  Wallet,
  BadgePercent,
} from 'lucide-react';
import xfjeIcon from '../../../public/board-xfje.svg';
import qqcsIcon from '../../../public/board-qqcs.svg';
import ycqqIcon from '../../../public/board-ycqq.svg';
import pjxyIcon from '../../../public/board-pjxy.svg';
import kyyeIcon from '../../../public/board-kyye.svg';
import jfedIcon from '../../../public/board-jfed.svg';

const StatsCards = ({
  groupedStatsData,
  loading,
  CARD_PROPS,
  displayMode = 'all',
  t
}) => {
  const navigate = useNavigate();

  const toNum = (value) => {
    if (typeof value === 'number') return value;
    if (typeof value === 'string') {
      const num = Number(value.replace(/[^0-9.-]/g, ''));
      return Number.isFinite(num) ? num : 0;
    }
    return 0;
  };

  const formatValue = (value, fallback = '--') => {
    if (value === null || value === undefined || value === '') return fallback;
    return value;
  };

  const account = groupedStatsData?.[0]?.items || [];
  const usage = groupedStatsData?.[1]?.items || [];
  const resource = groupedStatsData?.[2]?.items || [];
  const performance = groupedStatsData?.[3]?.items || [];

  const currentBalance = formatValue(account?.[0]?.value, '$19.99');
  const historyCost = formatValue(account?.[1]?.value, '$2.13');
  const todayRequests = formatValue(usage?.[0]?.value, 3);
  const totalTokens = formatValue(resource?.[1]?.value, 0);
  const totalQuota = formatValue(resource?.[0]?.value, '$0.00');
  const avgRPM = formatValue(performance?.[0]?.value, 0);
  const avgTPM = formatValue(performance?.[1]?.value, 0);

  const todayReqNum = Math.max(0, Math.floor(toNum(todayRequests)));
  const successCount = todayReqNum;
  const failedCount = 0;
  const successRate = todayReqNum > 0 ? 100 : 0;
  const anomalyCount = 0;
  const errorRate = 0;
  const hourAnomaly = 0;

  const budgetUsagePct = 0;
  const pointsUsedPct = 0;

  const cards = [
    {
      key: 'today-cost',
      title: '今日消耗金额',
      value: totalQuota,
      imgUrl: xfjeIcon,
      note: '总计模型费',
      noteValue: '$12,345.67',
      footerLabel: '本月预算使用率',
      footerValue: `${budgetUsagePct}%`,
      progress: budgetUsagePct,
      progressColor: '#2ed8c3',
      onClick: resource?.[0]?.onClick,
      compact: false,
    },
    {
      key: 'today-requests',
      title: '今日请求次数',
      value: todayRequests,
      imgUrl: qqcsIcon,
      statRows: [
        { label: '成功次数', value: successCount },
        { label: '失败次数', value: failedCount },
      ],
      badgeText: `成功率`,
      badgeValue: successRate,
      badgeColor: 'green',
      onClick: usage?.[0]?.onClick,
      compact: false,
    },
    {
      key: 'anomaly-requests',
      title: '异常请求数',
      value: anomalyCount,
      imgUrl: ycqqIcon,
      statRows: [
        { label: '错误率', value: `${errorRate.toFixed(1)}%` },
        { label: '近 1 小时异常', value: hourAnomaly },
      ],
      badgeText: '系统健康',
      badgeValue: '100',
      badgeColor: 'green',
      compact: false,
    },
    {
      key: 'avg-latency',
      title: '平均响应时间',
      value: '--',
      valueSuffix: 'ms',
      imgUrl: pjxyIcon,
      statRows: [
        { label: '平均 RPM', value: avgRPM },
        { label: '平均 TPM', value: avgTPM },
      ],
      badgeText: '响应正常',
      badgeValue: '',
      badgeColor: 'green',
      onClick: performance?.[0]?.onClick,
      compact: false,
    },
    {
      key: 'available-balance',
      title: '可用余额',
      value: currentBalance,
      imgUrl: kyyeIcon,
      statRows: [{ label: '历史消耗', value: historyCost }],
      actionText: '充值',
      onActionClick: (event) => {
        event.stopPropagation();
        navigate('/console/topup');
      },
      compact: true,
      onClick: account?.[0]?.onClick,
    },
    {
      key: 'points-quota',
      title: '积分额度',
      value: '0',
      imgUrl: jfedIcon,
      statRows: [{ label: '已使用率', value: `${pointsUsedPct}%` }],
      actionText: '使用说明',
      compact: true,
      onClick: usage?.[1]?.onClick,
    },
  ];

  const primaryCards = cards.filter((card) => !card.compact);
  const compactCards = cards.filter((card) => card.compact);

  const renderValue = (card) => (
    <Skeleton
      loading={loading}
      active
      placeholder={
        <Skeleton.Paragraph
          active
          rows={1}
          style={{ width: '120px', height: '32px', marginTop: 0 }}
        />
      }
    >
      <span>{formatValue(card.value)}</span>
      {card.valueSuffix ? (
        <span className='dashboard-stats-v2__value-suffix'>
          {card.valueSuffix}
        </span>
      ) : null}
    </Skeleton>
  );

  const renderPrimaryCard = (card) => (
    <Card
      key={card.key}
      {...CARD_PROPS}
      className='dashboard-stats-v2__card dashboard-stats-v2__card--primary'
    >
      <div
        className='dashboard-stats-v2__card-inner'
        onClick={card.onClick}
        role={card.onClick ? 'button' : undefined}
      >
        <div className='dashboard-stats-v2__card-header'>
          <div>
            <div className='dashboard-stats-v2__label'>{t(card.title)}</div>
            <div className='dashboard-stats-v2__value'>{renderValue(card)}</div>
          </div>
          <div className='dashboard-stats-v2__icon'>
            <img src={card.imgUrl} />
          </div>
        </div>

        {card.note ? (
          <div className='dashboard-stats-v2__single-note'>
            <span>{t(card.note)}</span>
            <span>{card.noteValue}</span>
          </div>
        ) : null}

        {card.statRows ? (
          <div className='dashboard-stats-v2__meta-list'>
            {card.statRows.map((row) => (
              <div
                key={`${card.key}-${row.label}`}
                className='dashboard-stats-v2__meta-row'
              >
                <span>{t(row.label)}</span>
                <strong>{formatValue(row.value, 0)}</strong>
              </div>
            ))}
          </div>
        ) : null}

        {card.footerLabel ? (
          <div className='dashboard-stats-v2__progress-wrap'>
            <div className='dashboard-stats-v2__progress-head'>
              <span>{t(card.footerLabel)}</span>
              <strong>{card.footerValue}</strong>
            </div>
            <Progress
              percent={card.progress}
              stroke={card.progressColor}
              showInfo={false}
              aria-label={card.footerLabel}
              style={{ height: '8px' }}
            />
          </div>
        ) : null}

        {card.badgeText ? (
          <div className='dashboard-stats-v2__badge-wrap'>
            <Tag color={card.badgeColor} shape='circle' size='large'>
              <CircleCheck size={14} />&nbsp;{t(card.badgeText)}{card.badgeValue ? ` ${card.badgeValue}%` : ''}
            </Tag>
          </div>
        ) : null}
      </div>
    </Card>
  );

  const renderCompactCard = (card) => (
    <Card
      key={card.key}
      {...CARD_PROPS}
      className='dashboard-stats-v2__card dashboard-stats-v2__card--compact'
    >
      <div
        className='dashboard-stats-v2__compact-inner'
        onClick={card.onClick}
        role={card.onClick ? 'button' : undefined}
      >
        <div className='dashboard-stats-v2__compact-copy'>
          <div className='dashboard-stats-v2__label'>{t(card.title)}</div>
          <div className='dashboard-stats-v2__compact-value'>
            {renderValue(card)}
          </div>
          {/* <div className='dashboard-stats-v2__meta-row dashboard-stats-v2__meta-row--compact'>
            <span>{card.statRows?.[0]?.label || '--'}</span>
            <strong>{formatValue(card.statRows?.[0]?.value, '--')}</strong>
          </div> */}
        </div>
        <div className='dashboard-stats-v2__compact-side'>
          <img src={card.imgUrl} className='iconImg' />
          {/* <button
            type='button'
            className='dashboard-stats-v2__mini-action'
            onClick={card.onActionClick}
          >
            {card.actionText}
          </button> */}
        </div>
      </div>
    </Card>
  );

  if (displayMode === 'primary') {
    return (
      <section className='dashboard-stats-v2'>
        <div className='dashboard-stats-v2__primary-grid'>
          {primaryCards.map(renderPrimaryCard)}
        </div>
      </section>
    );
  }

  if (displayMode === 'compact') {
    return (
      <section className='dashboard-stats-v2 dashboard-stats-v2--compact-only'>
        <div className='dashboard-stats-v2__compact-grid'>
          {compactCards.map(renderCompactCard)}
        </div>
      </section>
    );
  }

  return (
    <section className='dashboard-stats-v2'>
      <div className='dashboard-stats-v2__primary-grid'>
        {primaryCards.map(renderPrimaryCard)}
      </div>
      <div className='dashboard-stats-v2__compact-grid'>
        {compactCards.map(renderCompactCard)}
      </div>
    </section>
  );
};

export default StatsCards;
