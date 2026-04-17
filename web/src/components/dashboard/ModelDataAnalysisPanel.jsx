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

import React, { useMemo, useState, useEffect, useRef } from 'react';
import { API, showError, timestamp2string, isAdmin } from '../../helpers';
import { Empty } from '@douyinfe/semi-ui';
import {
  IllustrationConstruction,
  IllustrationConstructionDark,
} from '@douyinfe/semi-illustrations';
import {
  ILLUSTRATION_SIZE
} from '../../constants/dashboard.constants';
import { getDefaultTime, getInitialTimestamp } from '../../helpers/dashboard';
import mxsjfxIcon from '../../../public/board-mxsjfx.svg';
import ModelAnalysisChartsPanel from './ModelAnalysisChartsPanel';

const fallbackTabs = [
  { label: '消耗趋势', value: '2' },
  { label: '调用次数分布', value: '3' },
  { label: '调用次数排行', value: '4' },
];

const quotaRadioColors = ['#25dfe0', '#4f8ef7', '#6d6af8'];

const ModelDataAnalysisPanel = ({
  t,
  spec_model_line,
  spec_pie,
  spec_rank_bar,
  CHART_CONFIG,
}) => {
  const fetched = useRef(false);
  const isAdminUser = isAdmin();
  const [activeTab, setActiveTab] = useState('2');
  const [quotaRadioData, setQuotaRadioData] = useState([]);

  const tabs = useMemo(
    () => fallbackTabs.map((item) => ({ ...item, label: t(item.label) })),
    [t],
  );

  const activeTabLabel =
    tabs.find((item) => item.value === activeTab)?.label || tabs[0]?.label;

  useEffect(() => {
    if (fetched.current) return;
    fetched.current = true;

    const startTimestamp = Date.parse(getInitialTimestamp()) / 1000;
    const endTimestamp =
      Date.parse(timestamp2string(new Date().getTime() / 1000 + 3600)) / 1000;
    const defaultTime = getDefaultTime();
    const url = isAdminUser ?
      `/api/data/modelQuotaRadio/?start_timestamp=${startTimestamp}&end_timestamp=${endTimestamp}&default_time=${defaultTime}` :
      `/api/data/self/modelQuotaRadio/?start_timestamp=${startTimestamp}&end_timestamp=${endTimestamp}&default_time=${defaultTime}`;

    API.get(url).then((res) => {
      const { success, message, data } = res.data;
      if (success && data && data.length > 0) {
        const top3 = data.slice(0, 3).map((item, index) => ({
          ...item,
          color: quotaRadioColors[index],
        }));
        setQuotaRadioData(top3);
      } else if (!success) {
        showError(message);
      }
    });
  }, []);

  return (
    <section className='dashboard-model-analysis'>
      <div className='dashboard-model-analysis__header'>
        <div className='dashboard-model-analysis__title-wrap'>
          <img src={mxsjfxIcon} alt='' />
          <div>
            <h3 className='dashboard-model-analysis__title'>
              {t('模型数据分析')}
            </h3>
          </div>
        </div>
        <div className='dashboard-model-analysis__tabs'>
          {tabs.map((tab) => (
            <button
              key={tab.value}
              type='button'
              className={`dashboard-model-analysis__tab ${activeTab === tab.value ? 'is-active' : ''
                }`}
              onClick={() => setActiveTab(tab.value)}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      <div className='dashboard-model-analysis__body'>
        <div className='dashboard-model-analysis__left'>
          <div className='dashboard-model-analysis__section-title'>
            {t('额度占比')}
          </div>
          <div className='dashboard-model-analysis__bar-list'>
            {quotaRadioData.length > 0 ? quotaRadioData.map((item) => (
              <div key={item.model} className='dashboard-model-analysis__bar-item'>
                <div className='dashboard-model-analysis__bar-head'>
                  <span>{t(item.model_name)}</span>
                  <strong>{(item.quota * 100).toFixed(2)}%</strong>
                </div>
                <div className='dashboard-model-analysis__bar-track'>
                  <div
                    className='dashboard-model-analysis__bar-fill'
                    style={{
                      width: `${(item.quota * 100).toFixed(2)}%`,
                      backgroundColor: item.color,
                    }}
                  />
                </div>
              </div>
            )) : (
                <div className='h-full flex items-center justify-center'>
                  <Empty
                    image={<IllustrationConstruction style={ILLUSTRATION_SIZE} />}
                    darkModeImage={
                      <IllustrationConstructionDark style={ILLUSTRATION_SIZE} />
                    }
                    title={t('暂无数据')}
                  />
              </div>
            )
            }
          </div>
        </div>

        <div className='dashboard-model-analysis__right'>
          <div className='dashboard-model-analysis__section-title'>
            {activeTabLabel}
          </div>
          <div className='dashboard-model-analysis__summary-list'>
            <ModelAnalysisChartsPanel
              activeChartTab={activeTab}
              spec_model_line={spec_model_line}
              spec_pie={spec_pie}
              spec_rank_bar={spec_rank_bar}
              CHART_CONFIG={CHART_CONFIG}
            />
          </div>
        </div>
      </div>
    </section>
  );
};

export default ModelDataAnalysisPanel;
