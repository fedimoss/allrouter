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
import { Card, Skeleton, Tag, Tooltip } from '@douyinfe/semi-ui';
import { useNavigate } from 'react-router-dom';
import { Hourglass, DollarSign, Hash, Info, Orbit } from 'lucide-react';
import i18next from 'i18next';

const COMPACT_NUMBER_THRESHOLD = 10_000;

const formatCompactValue = (value) => {
  const fullValue = String(value);
  const normalizedValue = fullValue.trim().replaceAll(',', '');
  const valueParts = normalizedValue.match(
    /^([^0-9+-]*)([+-]?\d+(?:\.\d+)?)(.*)$/,
  );

  if (!valueParts) {
    return {
      displayValue: value,
      compactUnit: '',
      trailingValue: '',
      fullValue,
      isCompact: false,
    };
  }

  const numericValue = Number(valueParts[2]);
  if (
    !Number.isFinite(numericValue) ||
    Math.abs(numericValue) < COMPACT_NUMBER_THRESHOLD
  ) {
    return {
      displayValue: value,
      compactUnit: '',
      trailingValue: '',
      fullValue,
      isCompact: false,
    };
  }

  const locale = i18next.resolvedLanguage || i18next.language || 'zh-CN';
  let compactParts;
  try {
    compactParts = new Intl.NumberFormat(locale, {
      notation: 'compact',
      compactDisplay: 'short',
      maximumFractionDigits: 2,
    }).formatToParts(numericValue);
  } catch {
    compactParts = new Intl.NumberFormat('zh-CN', {
      notation: 'compact',
      compactDisplay: 'short',
      maximumFractionDigits: 2,
    }).formatToParts(numericValue);
  }

  const compactUnit = compactParts
    .filter((part) => part.type === 'compact')
    .map((part) => part.value)
    .join('');
  const compactNumber = compactParts
    .filter((part) => part.type !== 'compact')
    .map((part) => part.value)
    .join('');

  return {
    displayValue: `${valueParts[1]}${compactNumber}`,
    compactUnit,
    trailingValue: valueParts[3],
    fullValue,
    isCompact: true,
  };
};

const StatsCards = ({
  groupedStatsData,
  loading,
  CARD_PROPS,
  displayMode = 'all',
  t,
}) => {
  const navigate = useNavigate();

  const formatValue = (value, fallback = '--') => {
    if (value === null || value === undefined || value === '') {
      return fallback;
    }
    return value;
  };

  const account = groupedStatsData?.[0]?.items || [];
  const usage = groupedStatsData?.[1]?.items || [];
  const resource = groupedStatsData?.[2]?.items || [];
  const performance = groupedStatsData?.[3]?.items || [];

  const currentBalance = formatValue(account?.[0]?.value, '0');
  const historyCost = formatValue(account?.[1]?.value, '0');
  const requestCount24H = formatValue(usage?.[0]?.value, 0);
  const totalCount24H = formatValue(usage?.[1]?.value, 0);
  const totalQuota = formatValue(resource?.[0]?.value, '0');
  const totalTokens = formatValue(resource?.[1]?.value, '0');
  const userTotalTokenUsed = formatValue(resource?.[2]?.value, '0');
  const allUsersTotalTokenUsed = formatValue(resource?.[3]?.value, null);
  const userTotalTokenUsedTitle = resource?.[2]?.title || `${t('已用')} Tokens`;
  const avgRPM = formatValue(performance?.[0]?.value, '0');
  const avgTPM = formatValue(performance?.[1]?.value, '0');

  const cards = [
    {
      key: 'balance',
      title: '24H 消费余额',
      value: currentBalance,
      icon: DollarSign,
      iconClassName: 'dashboard-stats-v3__icon dashboard-stats-v3__icon--green',
      rows: [
        {
          label: '历史消耗',
          value: historyCost,
        },
      ],
      actionText: '充值',
      onActionClick: () => navigate('/console/topup'),
      onClick: account?.[0]?.onClick,
    },
    {
      key: 'usage',
      title: '24H 使用统计',
      icon: Orbit,
      iconClassName: 'dashboard-stats-v3__icon dashboard-stats-v3__icon--blue',
      metricBlocks: [
        { label: '请求次数', value: requestCount24H },
        { label: '统计次数', value: totalCount24H },
      ],
      onClick: usage?.[0]?.onClick,
    },
    {
      key: 'resource',
      title: '24H 资源消耗',
      icon: Info,
      iconClassName:
        'dashboard-stats-v3__icon dashboard-stats-v3__icon--violet',
      metricBlocks: [
        { label: '统计额度', value: totalQuota, compact: true },
        { label: '统计 Tokens', value: totalTokens, compact: true },
      ],
      onClick: resource?.[0]?.onClick,
    },
    {
      key: 'performance',
      title: '性能指标',
      icon: Hourglass,
      iconClassName:
        'dashboard-stats-v3__icon dashboard-stats-v3__icon--orange',
      metricBlocks: [
        { label: '平均RPM', value: avgRPM, compact: true },
        { label: '平均TPM', value: avgTPM, compact: true },
      ],
      onClick: performance?.[0]?.onClick,
    },
    {
      key: 'user-token-used',
      title: 'Token 消耗总量',
      titleTranslated: true,
      value: userTotalTokenUsed,
      icon: Hash,
      iconClassName: 'dashboard-stats-v3__icon dashboard-stats-v3__icon--blue',
      onClick: resource?.[2]?.onClick,
    },
    ...(allUsersTotalTokenUsed !== null
      ? [
          {
            key: 'all-users-token-used',
            title: '全站 Token 消耗总量',
            titleTranslated: true,
            value: allUsersTotalTokenUsed,
            icon: Hash,
            iconClassName:
              'dashboard-stats-v3__icon dashboard-stats-v3__icon--violet',
            onClick: resource?.[3]?.onClick,
          },
        ]
      : []),
  ];

  const renderValue = (value, width = 118) => {
    const { displayValue, compactUnit, trailingValue, fullValue, isCompact } =
      formatCompactValue(value);
    const valueNode = (
      <span
        className='dashboard-stats-v3__display-value'
        aria-label={isCompact ? fullValue : undefined}
        tabIndex={isCompact ? 0 : undefined}
      >
        {displayValue}
        {compactUnit ? (
          <span className='dashboard-stats-v3__compact-unit'>
            {compactUnit}
          </span>
        ) : null}
        {trailingValue}
      </span>
    );

    return (
      <Skeleton
        loading={loading}
        active
        placeholder={
          <Skeleton.Paragraph
            active
            rows={1}
            style={{ width: `${width}px`, height: '34px', marginTop: 0 }}
          />
        }
      >
        {isCompact ? (
          <Tooltip content={fullValue}>{valueNode}</Tooltip>
        ) : (
          valueNode
        )}
      </Skeleton>
    );
  };

  const renderCard = (card) => {
    const Icon = card.icon;

    return (
      <Card
        key={card.key}
        {...CARD_PROPS}
        className='dashboard-stats-v2__card dashboard-stats-v3__card'
      >
        <div
          className='dashboard-stats-v3__inner'
          onClick={card.onClick}
          role={card.onClick ? 'button' : undefined}
        >
          <div className='dashboard-stats-v3__header'>
            <div className='dashboard-stats-v3__title'>
              {card.titleTranslated ? t(card.title) : t(card.title)}
            </div>
            <div className={card.iconClassName}>
              <Icon size={18} strokeWidth={2.1} />
            </div>
          </div>

          {card.value ? (
            <div className='dashboard-stats-v3__hero'>
              {renderValue(card.value, 140)}
            </div>
          ) : null}

          {card.actionText ? (
            <div className='dashboard-stats-v3__action-wrap'>
              <Tag
                color='green'
                size='large'
                className='dashboard-stats-v3__action-tag'
                onClick={(event) => {
                  event.stopPropagation();
                  card.onActionClick?.();
                }}
              >
                {t(card.actionText)}
              </Tag>
            </div>
          ) : null}

          {card.metricBlocks ? (
            <div className='dashboard-stats-v3__metrics'>
              {card.metricBlocks.map((item) => (
                <div
                  key={`${card.key}-${item.label}`}
                  className='dashboard-stats-v3__metric-block'
                >
                  <div className='dashboard-stats-v3__metric-label'>
                    {t(item.label)}
                  </div>
                  <div
                    className={
                      item.compact
                        ? 'dashboard-stats-v3__metric-value dashboard-stats-v3__metric-value--compact'
                        : 'dashboard-stats-v3__metric-value'
                    }
                  >
                    {renderValue(item.value, item.compact ? 120 : 92)}
                  </div>
                </div>
              ))}
            </div>
          ) : null}

          {card.rows ? (
            <div className='dashboard-stats-v3__footer'>
              {card.rows.map((row) => (
                <div
                  key={`${card.key}-${row.label}`}
                  className='dashboard-stats-v3__footer-row'
                >
                  <span>{t(row.label)}</span>
                  <strong>{renderValue(row.value, 74)}</strong>
                </div>
              ))}
            </div>
          ) : null}
        </div>
      </Card>
    );
  };

  if (displayMode === 'compact') {
    return null;
  }

  return (
    <section className='dashboard-stats-v2 dashboard-stats-v3'>
      <div
        className={`dashboard-stats-v2__primary-grid dashboard-stats-v3__grid${
          cards.length > 5 ? ' dashboard-stats-v3__grid--six' : ''
        }`}
      >
        {cards.map(renderCard)}
      </div>
    </section>
  );
};

export default StatsCards;
