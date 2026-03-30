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
import { Avatar, Card, Tag } from '@douyinfe/semi-ui';
import { isRoot, isAdmin, renderQuota, stringToColor } from '../../../../helpers';
import { Coins, BarChart2, Users, Wallet } from 'lucide-react';

const UserInfoHeader = ({ t, userState }) => {
  const getUsername = () => {
    if (userState.user) {
      return userState.user.username;
    }
    return 'null';
  };

  const getAvatarText = () => {
    const username = getUsername();
    if (username && username.length > 0) {
      return username.slice(0, 2).toUpperCase();
    }
    return 'NA';
  };

  const roleLabel = isRoot() ? t('超级管理员') : isAdmin() ? t('管理员') : t('普通用户');

  const metricItems = [
    {
      key: 'quota',
      label: t('当前余额'),
      value: renderQuota(userState?.user?.quota),
      trend: t('可用余额'),
      icon: Wallet,
      tone: 'emerald',
    },
    {
      key: 'used',
      label: t('历史消耗'),
      value: renderQuota(userState?.user?.used_quota),
      trend: t('累计消费'),
      icon: Coins,
      tone: 'cyan',
    },
    {
      key: 'request',
      label: t('请求次数'),
      value: userState?.user?.request_count || 0,
      trend: t('累计请求'),
      icon: BarChart2,
      tone: 'violet',
    },
    {
      key: 'group',
      label: t('用户分组'),
      value: userState?.user?.group || t('默认'),
      trend: `ID ${userState?.user?.id || '-'}`,
      icon: Users,
      tone: 'amber',
    },
  ];

  return (
    <Card className='personal-v2-panel personal-v2-header-card !rounded-2xl'>
      <section className='personal-v2-profile-shell'>
        <div className='personal-v2-profile-main'>
          <Avatar size='large' color={stringToColor(getUsername())}>
            {getAvatarText()}
          </Avatar>
          <div className='min-w-0'>
            <div className='personal-v2-hero-name'>
              {getUsername()}
            </div>
            <div className='flex flex-wrap items-center gap-2 mt-2'>
              <Tag size='large' shape='circle' className='personal-v2-profile-tag'>
                {roleLabel}
              </Tag>
              <Tag size='large' shape='circle' className='personal-v2-profile-tag'>
                ID: {userState?.user?.id}
              </Tag>
            </div>
          </div>
        </div>
      </section>

      <section className='personal-v2-metric-grid'>
        {metricItems.map((item) => {
          const Icon = item.icon;
          return (
            <article
              key={item.key}
              className={`personal-v2-metric-card personal-v2-metric-${item.tone}`}
            >
              <div className='personal-v2-metric-main'>
                <div>
                  <p className='personal-v2-metric-label'>{item.label}</p>
                  <p className='personal-v2-metric-value'>{item.value}</p>
                  <p className='personal-v2-metric-trend'>{item.trend}</p>
                </div>
                <span className='personal-v2-metric-icon'>
                  <Icon size={18} />
                </span>
              </div>
            </article>
          );
        })}
      </section>
    </Card>
  );
};

export default UserInfoHeader;