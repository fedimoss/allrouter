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
import { Card, Select, Button } from '@douyinfe/semi-ui';
import { ChartNoAxesColumn, Download } from 'lucide-react';
import { VChart } from '@visactor/react-vchart';

const ChartsPanel = ({
  activeChartTab,
  setActiveChartTab,
  spec_line,
  spec_model_line,
  spec_pie,
  spec_rank_bar,
  CARD_PROPS,
  CHART_CONFIG,
  FLEX_CENTER_GAP2,
  t,
  panelHeight,
}) => {
  const [timeRange, setTimeRange] = useState('24h');

  const chartTypeOptions = useMemo(
    () => [
      { label: t('消耗分布'), value: '1' },
      { label: t('消耗趋势'), value: '2' },
      { label: t('调用次数分布'), value: '3' },
      { label: t('调用次数排行'), value: '4' },
    ],
    [t],
  );

  const timeRangeOptions = useMemo(
    () => [
      { label: t('最近 24 小时'), value: '24h' },
      { label: t('最近 7 天'), value: '7d' },
      { label: t('最近 30 天'), value: '30d' },
    ],
    [t],
  );

  return (
    <Card
      {...CARD_PROPS}
      className='!rounded-2xl dashboard-trend-card'
      style={panelHeight ? { height: panelHeight } : undefined}
      title={
        <div className='dashboard-trend-header'>
          <div className={FLEX_CENTER_GAP2}>
            <ChartNoAxesColumn size={24} className='dashboard-trend-title-icon' />
            <span style={{fontSize:'18px',fontWeight:'700'}}>{t('消耗与请求趋势')}</span>
          </div>
          <div className='dashboard-trend-toolbar'>
            <Select
              value={activeChartTab}
              onChange={(value) => setActiveChartTab(String(value))}
              optionList={chartTypeOptions}
              style={{ width: 148,backgroundColor: 'rgb(248 250 252 / 100%)',border:'1px solid rgb(203 213 225 / 100%)' }}
            />
            <Select
              value={timeRange}
              onChange={(value) => setTimeRange(String(value))}
              optionList={timeRangeOptions}
              style={{ width: 148,backgroundColor: 'rgb(248 250 252 / 100%)',border:'1px solid rgb(203 213 225 / 100%)' }}
            />
            <Button
              theme='borderless'
              type='tertiary'
              icon={<Download size={14} />}
            />
          </div>
        </div>
      }
      bodyStyle={panelHeight ? { padding: 0, height: 'calc(100% - 56px)' } : { padding: 0 }}
    >
      <div className='dashboard-trend-body'>
        {activeChartTab === '1' && (
          <VChart spec={spec_line} option={CHART_CONFIG} />
        )}
        {activeChartTab === '2' && (
          <VChart spec={spec_model_line} option={CHART_CONFIG} />
        )}
        {activeChartTab === '3' && (
          <VChart spec={spec_pie} option={CHART_CONFIG} />
        )}
        {activeChartTab === '4' && (
          <VChart spec={spec_rank_bar} option={CHART_CONFIG} />
        )}
      </div>
    </Card>
  );
};

export default ChartsPanel;
