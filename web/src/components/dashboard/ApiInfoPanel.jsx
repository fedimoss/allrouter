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
import { Empty, Tag } from '@douyinfe/semi-ui';
import { Share2, CircleCheckBig } from 'lucide-react';
import { IllustrationConstruction } from '@douyinfe/semi-illustrations';

import './index.scss';

const ApiInfoPanel = ({ apiInfoData = [], handleCopyUrl, t }) => {
  return (
    <div className='dashboard-card'>
      <div className='card-header'>
        <div className='header-title'>
          <Share2 size={20} color='var(--semi-color-primary)' />
          <span>{t('API信息')}</span>
        </div>
        <button className='header-link'>{t('详情')}</button>
      </div>

      <div className='api-list'>
        {apiInfoData.length > 0 ? (
          apiInfoData.map((api, index) => (
            <div key={index} className='api-item'>
              <div>
                <div
                  className='api-endpoint'
                  onClick={() => handleCopyUrl?.(api.url)}
                >
                  {api.url}
                </div>
                <div className='api-status-text'>
                  {t('状态：')}
                  {api.label || t('可用')}
                </div>
              </div>
              <div className={`status-badge ${api.type || ''}`}>
                <Tag shape='circle' size='large' color='green'>
                  <CircleCheckBig size={16} />
                  &nbsp;&nbsp;OK
                </Tag>
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
              title={t('暂无API信息')}
              description={t('请联系管理员在系统设置中配置API信息')}
            />
          </div>
        )}
      </div>
    </div>
  );
};

export default ApiInfoPanel;
