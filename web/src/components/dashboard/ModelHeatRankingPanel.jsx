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
import { Empty } from '@douyinfe/semi-ui';
import { Trophy } from 'lucide-react';
import { IllustrationConstruction } from '@douyinfe/semi-illustrations';

import './index.scss';

const mockRankingData = [];

const ModelHeatRankingPanel = ({ t }) => {
  return (
    <div className='dashboard-card'>
      <div className='card-header'>
        <div className='header-title'>
          <Trophy size={20} color='var(--semi-color-warning)' />
          <span>{t('模型热度排行')}</span>
        </div>
        <button className='header-link'>{t('查看全部')}</button>
      </div>

      <div className='rank-list'>
        {mockRankingData.length > 0 ? (
          mockRankingData.map((item) => (
            <div key={item.rank} className='rank-item'>
              <div className='rank-info'>
                <div
                  className='rank-number'
                  style={{ backgroundColor: item.bg, color: item.color }}
                >
                  {item.rank}
                </div>
                <div>
                  <div className='model-name'>{item.model}</div>
                  <div className='model-provider'>{item.vendor}</div>
                </div>
              </div>
              <div>
                <div className='call-count'>{item.calls}</div>
                <div className='call-label'>{t('次调用')}</div>
              </div>
            </div>
          ))
        ) : (
          <div className='flex h-full w-full items-center justify-center'>
            <Empty
              image={
                <IllustrationConstruction
                  style={{ width: '90px', height: '90px' }}
                />
              }
              title={t('暂无模型调用数据')}
              description={t('请联系管理员在系统设置中配置模型调用数据')}
            />
          </div>
        )}
      </div>
    </div>
  );
};

export default ModelHeatRankingPanel;
