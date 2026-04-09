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
import { VChart } from '@visactor/react-vchart';

const ModelAnalysisChartsPanel = ({
  activeChartTab,
  spec_model_line,
  spec_pie,
  spec_rank_bar,
  CHART_CONFIG,
}) => {
  const currentTab = ['2', '3', '4'].includes(String(activeChartTab))
    ? String(activeChartTab)
    : '2';

  return (
    <div className='dashboard-trend-body' style={{ height: 240, padding: 0 }}>
      {currentTab === '2' && (
        <VChart spec={spec_model_line} option={CHART_CONFIG} />
      )}
      {currentTab === '3' && (
        <VChart spec={spec_pie} option={CHART_CONFIG} />
      )}
      {currentTab === '4' && (
        <VChart spec={spec_rank_bar} option={CHART_CONFIG} />
      )}
    </div>
  );
};

export default ModelAnalysisChartsPanel;
