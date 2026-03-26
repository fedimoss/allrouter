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
          <Trophy size={20} color='#eab308' />
          <span>{t('\u6a21\u578b\u70ed\u5ea6\u6392\u884c')}</span>
        </div>
        <button className='header-link'>{t('\u67e5\u770b\u5168\u90e8')}</button>
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
                <div className='call-label'>{t('\u6b21\u8c03\u7528')}</div>
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
              title={t('\u6682\u65e0\u6a21\u578b\u8c03\u7528\u6570\u636e')}
              description={t('\u8bf7\u8054\u7cfb\u7ba1\u7406\u5458\u5728\u7cfb\u7edf\u8bbe\u7f6e\u4e2d\u914d\u7f6e\u6a21\u578b\u8c03\u7528\u6570\u636e')}
            />
          </div>
        )}
      </div>
    </div>
  );
};

export default ModelHeatRankingPanel;
