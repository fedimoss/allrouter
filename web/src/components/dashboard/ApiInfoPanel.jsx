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
import {
  IllustrationConstruction,
  IllustrationConstructionDark,
} from '@douyinfe/semi-illustrations';
import {
  ILLUSTRATION_SIZE
} from '../../constants/dashboard.constants';
import xxphIcon from '../../../public/board-xxph.svg';

import './index.scss';

const ApiInfoPanel = ({ apiInfoData = [], handleCopyUrl, t }) => {
  const displayData = apiInfoData

  return (
    <div className='dashboard-card'>
      <div className='card-header'>
        <div className='header-title'>
          <img src={xxphIcon} alt="" />
          <span>{t('API信息')}</span>
        </div>
        <button className='header-link'>{t('详情')}</button>
      </div>

      <div className='api-list'>
        {displayData.length > 0 ? (
          displayData.map((api, index) => (
            <div key={`${api.url}-${index}`} className='api-item'>
              <div className='api-item-main'>
                <div
                  className='api-endpoint'
                  onClick={() => handleCopyUrl?.(api.url)}
                >
                  {api.url || '--'}
                </div>
                {/* <div className='api-status-text'>
                  {api.label || '--'}
                </div> */}
              </div>
              <div className={`status-badge success`}>
                <div className='status-circle'></div>{'OK'}
              </div>
            </div>
          ))
        ): (
          <div className='flex justify-center items-center py-8'>
            <Empty
              image={<IllustrationConstruction style={{width:'60px', height:'60px'}} />}
              darkModeImage={
                <IllustrationConstructionDark style={{width:'60px', height:'60px'}} />
              }
              description={t('暂无API信息')}
            />
          </div>
        )}
      </div>
    </div>
  );
};

export default ApiInfoPanel;
