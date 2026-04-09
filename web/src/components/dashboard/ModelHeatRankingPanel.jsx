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

import React, { useEffect, useRef, useState } from 'react';
import { TrendingUp } from 'lucide-react';
import { Empty } from '@douyinfe/semi-ui';
import {
  IllustrationConstruction,
  IllustrationConstructionDark,
} from '@douyinfe/semi-illustrations';
import { API, showError,isAdmin, timestamp2string } from '../../helpers';
import { getDefaultTime, getInitialTimestamp } from '../../helpers/dashboard';
import xxphIcon from '../../../public/board-xxph.svg';

import './index.scss';


const ModelHeatRankingPanel = ({ t }) => {
  const [rankingData, setRankingData] = useState([]);
  const fetched = useRef(false);
  const isAdminUser = isAdmin();

  useEffect(() => {
    if (fetched.current) return;
    fetched.current = true;

    const startTimestamp = Date.parse(getInitialTimestamp()) / 1000;
    const endTimestamp = Date.parse(timestamp2string(new Date().getTime() / 1000 + 3600)) / 1000;
    const defaultTime = getDefaultTime();
    const url = isAdminUser ?
      `/api/data/modelPopularRank/?start_timestamp=${startTimestamp}&end_timestamp=${endTimestamp}&default_time=${defaultTime}` :
      `/api/data/self/modelPopularRank/?start_timestamp=${startTimestamp}&end_timestamp=${endTimestamp}&default_time=${defaultTime}`;

    API.get(url).then((res) => {
      const { success, message, data } = res.data;
      if (success && data && data.length > 0) {
        const top3 = data.slice(0, 3).map((item, index) => ({
          ...item,
          rank: '0' + (index + 1),
        }));
        setRankingData(top3);
      } else if (!success) {
        showError(message);
      }
    });
  }, []);

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
        {rankingData.length > 0 ? (
        rankingData.map((item) => (
          <div key={item.rank} className='rank-item'>
            <div className='rank-info'>
              <div
                className='rank-number'
              >
                {item.rank}
              </div>
              <div>
                <div className='model-name'>{item.model_name}</div>
                {/* <div className='model-provider'>{item.vendor}</div> */}
              </div>
            </div>
            <div className='rank-item-right'>
              <div className='call-count'>{item.count} REQ</div>
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
