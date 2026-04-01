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
import { TrendingUp } from 'lucide-react';
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

const fallbackRankingData = [];

const ModelHeatRankingPanel = ({ t }) => {
  return (
    <div className='dashboard-card'>
      <div className='card-header'>
        <div className='header-title'>
          <img src={xxphIcon} alt="" />
          <span>{t('模型热度排行')}</span>
        </div>
        <TrendingUp size={20} />
        {/* <button className='header-link'>{t('查看全部')}</button> */}
      </div>

      <div className='rank-list'>
        {fallbackRankingData.length > 0 ? (
        fallbackRankingData.map((item) => (
          <div key={item.rank} className='rank-item'>
            <div className='rank-info'>
              <div
                className='rank-number rank-number--rounded'
                style={{color: item.color }}
              >
                {item.rank}
              </div>
              <div>
                <div className='model-name'>{item.model}</div>
                {/* <div className='model-provider'>{item.vendor}</div> */}
              </div>
            </div>
            <div className='rank-item-right'>
              <div className='call-count'>{item.vendor}</div>
              {/* <div className='call-label'>{t('请求量')}</div> */}
            </div>
          </div>
        ))
        ) : (
            <div className='flex justify-center items-center py-8'>
              <Empty
                image={<IllustrationConstruction style={{width:'60px', height:'60px'}} />}
                darkModeImage={
                  <IllustrationConstructionDark style={{width:'60px', height:'60px'}} />
                }
                description={t('暂无模型调用数据')}
              />
            </div>
        ) }
      </div>
    </div>
  );
};

export default ModelHeatRankingPanel;
