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
import { Button, Card, Skeleton, Tag,Progress } from '@douyinfe/semi-ui';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {BadgeDollarSign,MousePointer2,ShieldAlert,Zap,Wallet,BadgePercent} from 'lucide-react';

const StatsCards = ({ groupedStatsData, loading, CARD_PROPS }) => {
  const navigate = useNavigate();
  const { t } = useTranslation();

  const toNum = (value) => {
    if (typeof value === 'number') return value;
    if (typeof value === 'string') {
      const num = Number(value.replace(/[^0-9.-]/g, ''));
      return Number.isFinite(num) ? num : 0;
    }
    return 0;
  };

  const account = groupedStatsData?.[0]?.items || [];
  const usage = groupedStatsData?.[1]?.items || [];
  const resource = groupedStatsData?.[2]?.items || [];
  const performance = groupedStatsData?.[3]?.items || [];

  const currentBalance = account?.[0]?.value ?? '$0.00';
  const historyCost = account?.[1]?.value ?? '$0.00';
  const todayRequests = usage?.[0]?.value ?? 0;
  const totalTokens = resource?.[1]?.value ?? 0;
  const totalQuota = resource?.[0]?.value ?? '$0.00';
  const avgRPM = performance?.[0]?.value ?? 0;
  const avgTPM = performance?.[1]?.value ?? 0;

  const todayReqNum = Math.max(0, Math.floor(toNum(todayRequests)));
  const successCount = todayReqNum;
  const failedCount = 0;
  const successRate = todayReqNum > 0 ? 100 : 0;
  const anomalyCount = 0;
  const errorRate = 0;
  const hourAnomaly = 0;

  const budgetUsagePct = 24;
  const pointsUsedPct = 50;

  const cards = [
    {
      key: 'today-cost',
      title: t('今日消耗金额'),
      value: totalQuota,
      icon: <BadgeDollarSign />,
      iconColor:'#22c55e',
      iconBg: 'rgba(240, 253, 244, 1)',
      accent: '#22c55e',
      lines: [
        { label: t('总计模型费'), value: '$12,345.67' },
        { label: t('较昨日'), value: '--' },
      ],
      footer: (
        <div className='mt-2'>
          <div className='flex items-center justify-between text-[12px] text-slate-500'>
            <span style={{color:'rgb(100 116 139 / 100%)'}}>{t('本月预算使用率')}</span>
            <span className='font-semibold text-slate-600'>{budgetUsagePct}%</span>
          </div>
          <div className='mt-1 overflow-hidden'>
            <Progress percent={budgetUsagePct} aria-label="disk usage" />
          </div>
        </div>
      ),
      onClick: resource?.[0]?.onClick,
    },
    {
      key: 'today-requests',
      title: t('今日请求次数'),
      value: todayRequests,
      icon: <MousePointer2 />,
      iconColor: '#3b82f6',
      iconBg: 'rgba(239, 246, 255, 1)',
      lines: [
        { label: t('成功次数'), value: successCount },
        { label: t('失败次数'), value: failedCount },
        { label: t('总计 Tokens'), value: totalTokens },
      ],
      footer: (
        <Tag color='green' shape='circle' size='large'>
          {t('成功率')} {successRate}%
        </Tag>
      ),
      onClick: usage?.[0]?.onClick,
    },
    {
      key: 'anomaly-requests',
      title: t('异常请求数'),
      value: anomalyCount,
      icon: <ShieldAlert />,
      iconColor: '#8b5cf6',
      iconBg: 'rgba(250, 245, 255, 1)',
      lines: [
        { label: t('错误率'), value: `${errorRate.toFixed(1)}%` },
        { label: t('近 1 小时异常'), value: hourAnomaly },
      ],
      footer: (
        <Tag color='green' shape='circle' size='large'>
          {t('系统健康')}
        </Tag>
      ),
    },
    {
      key: 'avg-latency',
      title: t('平均响应时间'),
      value: '0ms',
      icon: <Zap />,
      iconColor: '#f59e0b',
      iconBg: 'rgba(255, 247, 237, 1)',
      lines: [
        { label: t('平均 RPM'), value: avgRPM },
        { label: t('平均 TPM'), value: avgTPM },
        { label: t('负载状态'), value: t('正常') },
      ],
      onClick: performance?.[0]?.onClick,
    },
    {
      key: 'available-balance',
      title: t('可用余额'),
      value: currentBalance,
      icon: <Wallet />,
      iconColor: '#10b981',
      iconBg: 'rgba(230, 255, 254, 1)',
      lines: [
        { label: t('历史消耗'), value: historyCost },
        { label: t('上次充值值'), value: '2026-03-15 10:30' },
      ],
      footer: (
        <Tag
          color='white'
          shape='circle'
          size='large'
          onClick={(e) => {
            e.stopPropagation();
            navigate('/console/topup');
          }}
        >
          {t('充值')}
        </Tag>
      ),
      onClick: account?.[0]?.onClick,
    },
    {
      key: 'points-quota',
      title: t('积分额度'),
      value: '1,000 / 2,000',
      icon: <BadgePercent />,
      iconColor: '#a855f7',
      iconBg: 'rgba(250, 245, 255, 1)',
      accent: '#a855f7',
      lines: [{ label: t('已使用率'), value: `${pointsUsedPct}%` }],
      footer: (
        <div className='mt-2'>
          <div className='h-2 rounded-full bg-slate-200 overflow-hidden'>
            <div
              className='h-full rounded-full bg-violet-500'
              style={{ width: `${pointsUsedPct}%` }}
            />
          </div>
          <div className='text-[12px] text-slate-600 mt-2' style={{color:'rgb(100 116 139 / 100%)'}}>
            {t('预计可用')} 30 {t('天')}
          </div>
        </div>
      ),
      onClick: usage?.[1]?.onClick,
    },
  ];

  return (
    <div className='mb-4'>
      <div className='grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-6 gap-4'>
        {cards.map((card) => (
          <Card
            key={card.key}
            {...CARD_PROPS}
            className='metric-card bg-white p-5 !rounded-xl shadow-sm border border-slate-200 relative overflow-hidden'
            style={{ borderRight: `4px solid ${card.accent}` }}
          >
            <div
              className='h-full flex flex-col'
              onClick={card.onClick}
              role={card.onClick ? 'button' : undefined}
            >
              <div className='flex items-start justify-between mb-3'>
                <div>
                  <div className='text-[14px] text-gray-800'>{card.title}</div>
                  <div className='text-2xl font-bold text-slate-800 mt-1'>
                    <Skeleton
                      loading={loading}
                      active
                      placeholder={
                        <Skeleton.Paragraph
                          active
                          rows={1}
                          style={{ width: '120px', height: '30px', marginTop: 0 }}
                        />
                      }
                    >
                      {card.value}
                    </Skeleton>
                  </div>
                </div>
                <Button
                  type='tertiary'
                  theme='light'
                  icon={card.icon}
                  style={{backgroundColor:card.iconBg, color: card.iconColor}}
                >
                </Button>
              </div>
              <div className='text-[12px]'>
                {card.lines.map((line, lineIdx) => (
                  <div key={`${card.key}-${lineIdx}`} className='flex items-center gap-2'>
                    <span style={{color:'rgb(148 163 184 / 100%)'}}>{line.label}</span>
                    <span className='font-semibold text-slate-600'>{line.value}</span>
                  </div>
                ))}
              </div>

              <div className='mt-auto pt-1'>{card.footer || null}</div>
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
};

export default StatsCards;
