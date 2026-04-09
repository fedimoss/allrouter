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
import { Skeleton } from '@douyinfe/semi-ui';
import { Wallet, Gauge, Zap } from 'lucide-react';
import { renderQuota } from '../../../helpers';
import { useMinimumLoadingTime } from '../../../hooks/common/useMinimumLoadingTime';

const STAT_CARDS = [
  {
    key: 'quota',
    label: '消耗额度',
    icon: Wallet,
    accentClassName: 'usage-logs-v2-stat-accent-quota',
    iconClassName: 'usage-logs-v2-stat-icon-quota',
  },
  {
    key: 'rpm',
    label: 'RPM',
    icon: Gauge,
    accentClassName: 'usage-logs-v2-stat-accent-rpm',
    iconClassName: 'usage-logs-v2-stat-icon-rpm',
  },
  {
    key: 'tpm',
    label: 'TPM',
    icon: Zap,
    accentClassName: 'usage-logs-v2-stat-accent-tpm',
    iconClassName: 'usage-logs-v2-stat-icon-tpm',
  },
];

const LogsActions = ({ stat, loadingStat, showStat, t }) => {
  const showSkeleton = useMinimumLoadingTime(loadingStat);
  const needSkeleton = !showStat || showSkeleton;

  const values = {
    quota: renderQuota(stat?.quota ?? 0),
    rpm: stat?.rpm ?? 0,
    tpm: stat?.tpm ?? 0,
  };

  return (
    <div className='usage-logs-v2-stats'>
      <div className='usage-logs-v2-stat-grid'>
        {STAT_CARDS.map((item) => {
          const Icon = item.icon;

          return (
            <article
              key={item.key}
              className={`usage-logs-v2-stat-card ${item.accentClassName}`}
            >
              <div className='usage-logs-v2-stat-head'>
                <div className='usage-logs-v2-stat-label'>{t(item.label)}</div>
                <span
                  className={`usage-logs-v2-stat-icon ${item.iconClassName}`}
                  aria-hidden='true'
                >
                  <Icon size={18} strokeWidth={2.2} />
                </span>
              </div>

              {needSkeleton ? (
                <div className='usage-logs-v2-stat-skeleton'>
                  <Skeleton.Title
                    style={{ width: 112, height: 34, borderRadius: 10 }}
                  />
                </div>
              ) : (
                <div className='usage-logs-v2-stat-value'>{values[item.key]}</div>
              )}
            </article>
          );
        })}
      </div>
    </div>
  );
};

export default LogsActions;