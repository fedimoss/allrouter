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

import React, { useMemo, useState } from 'react';
import { BarChart3 } from 'lucide-react';
import mxsjfxIcon from '../../../public/board-mxsjfx.svg';

const fallbackTabs = [
  { label: '额度分布', value: 'quota' },
  { label: '消耗趋势', value: 'consume' },
  { label: '调用次数分布', value: 'calls' },
  { label: '调用/占比/统计', value: 'share' },
];

const fallbackModelShare = [
  { model: 'GPT-4-Turbo', percent: 0, color: '#25dfe0' },
  { model: 'Midjourney v6', percent: 0, color: '#4f8ef7' },
  { model: 'DALL-E 3', percent: 0, color: '#6d6af8' },
];

const fallbackQuotaOverview = [
  { label: '已分配', value: '0' },
  { label: '可用', value: '0' },
  { label: '冻结', value: '0' },
];

const ModelDataAnalysisPanel = ({ t }) => {
  const [activeTab, setActiveTab] = useState('quota');

  const tabs = useMemo(
    () => fallbackTabs.map((item) => ({ ...item, label: t(item.label) })),
    [t],
  );

  return (
    <section className='dashboard-model-analysis'>
      <div className='dashboard-model-analysis__header'>
        <div className='dashboard-model-analysis__title-wrap'>
          <img src={mxsjfxIcon} alt="" />
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
              className={`dashboard-model-analysis__tab ${
                activeTab === tab.value ? 'is-active' : ''
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
            {t('额度占比（模拟）')}
          </div>
          <div className='dashboard-model-analysis__bar-list'>
            {fallbackModelShare.map((item) => (
              <div
                key={item.model}
                className='dashboard-model-analysis__bar-item'
              >
                <div className='dashboard-model-analysis__bar-head'>
                  <span>{t(item.model)}</span>
                  <strong>{item.percent}%</strong>
                </div>
                <div className='dashboard-model-analysis__bar-track'>
                  <div
                    className='dashboard-model-analysis__bar-fill'
                    style={{
                      width: `${item.percent}%`,
                      backgroundColor: item.color,
                    }}
                  />
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className='dashboard-model-analysis__right'>
          <div className='dashboard-model-analysis__section-title'>
            {t('配额概览（模拟）')}
          </div>
          <div className='dashboard-model-analysis__summary-list'>
            {fallbackQuotaOverview.map((item) => (
              <div
                key={item.label}
                className='dashboard-model-analysis__summary-row'
              >
                <span>{t(item.label)}</span>
                <strong>{item.value}</strong>
              </div>
            ))}
          </div>
          <p className='dashboard-model-analysis__footnote'>
            {t('用于展示“额度分布”视图的占比结构。')}
          </p>
        </div>
      </div>
    </section>
  );
};

export default ModelDataAnalysisPanel;
