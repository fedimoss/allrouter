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

import React, { useContext, useEffect,useState } from 'react';
import { useTranslation } from 'react-i18next';
import { getRelativeTime } from '../../helpers';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';

import DashboardHeader from './DashboardHeader';
import StatsCards from './StatsCards';
import ChartsPanel from './ChartsPanel';
import ApiInfoPanel from './ApiInfoPanel';
import ModelHeatRankingPanel from './ModelHeatRankingPanel';
import ModelDataAnalysisPanel from './ModelDataAnalysisPanel';
import AnnouncementsPanel from './AnnouncementsPanel';
import FaqPanel from './FaqPanel';
import UptimePanel from './UptimePanel';
import SearchModal from './modals/SearchModal';

import { useDashboardData } from '../../hooks/dashboard/useDashboardData';
import { useDashboardStats } from '../../hooks/dashboard/useDashboardStats';
import { useDashboardCharts } from '../../hooks/dashboard/useDashboardCharts';

import {
  CHART_CONFIG,
  CARD_PROPS,
  FLEX_CENTER_GAP2,
  ILLUSTRATION_SIZE,
  ANNOUNCEMENT_LEGEND_DATA,
  UPTIME_STATUS_MAP,
} from '../../constants/dashboard.constants';
import {
  getTrendSpec,
  getUptimeStatusColor,
  getUptimeStatusText,
  renderMonitorList,
} from '../../helpers/dashboard';
import { Button, Select } from '@douyinfe/semi-ui';
import { CalendarDays } from 'lucide-react';

const Dashboard = () => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState, statusDispatch] = useContext(StatusContext);
  const [showScreenModal, setShowScreenModal] = useState(false);
  const [dateRangeValue, setDateRangeValue] = useState('24h');
  const DATE_RANGE_OPTIONS = [
    { label: '最近24小时', value: '24h' },
    { label: '最近一周', value: '7d' },
    { label: '最近一个月', value: '30d' },
  ];

  // ========== 主要数据管理 ==========
  const dashboardData = useDashboardData(userState, userDispatch, statusState);

  // ========== 图表管理 ==========
  const dashboardCharts = useDashboardCharts(
    dashboardData.dataExportDefaultTime,
    dashboardData.setTrendData,
    dashboardData.setConsumeQuota,
    dashboardData.setTimes,
    dashboardData.setConsumeTokens,
    dashboardData.setPieData,
    dashboardData.setLineData,
    dashboardData.setModelColors,
    dashboardData.t,
  );

  // ========== 统计数据 ==========
  const { groupedStatsData } = useDashboardStats(
    userState,
    dashboardData.consumeQuota,
    dashboardData.consumeTokens,
    dashboardData.times,
    dashboardData.trendData,
    dashboardData.performanceMetrics,
    dashboardData.displayCurrency,
    dashboardData.isAdminUser,
    dashboardData.navigate,
    dashboardData.t,
  );

  // ========== 数据处理 ==========
  const initChart = async () => {
    const [data] = await Promise.all([
      dashboardData.loadQuotaData(),
      dashboardData.loadModelData(),
    ]);
    if (data) {
      dashboardCharts.updateChartData(data);
    }
    await dashboardData.loadUptimeData();
  };

  const handleRefresh = async () => {
    const data = await dashboardData.refresh();
    if (data) {
      dashboardCharts.updateChartData(data);
    }
  };

  const handleSearchConfirm = async () => {
    await dashboardData.handleSearchConfirm(dashboardCharts.updateChartData);
    setShowScreenModal(false);
  };

  // 下拉快捷区间切换：与自定义弹窗一样调用三个接口刷新数据
  const handleDateRangeChange = async (value) => {
    setDateRangeValue(value);
    await dashboardData.handleDateRangeChange(
      value,
      dashboardCharts.updateChartData,
    );
  };

  // ========== 数据准备 ==========
  const apiInfoData = statusState?.status?.api_info || [];
  const announcementData = (statusState?.status?.announcements || []).map(
    (item) => {
      const pubDate = item?.publishDate ? new Date(item.publishDate) : null;
      const absoluteTime =
        pubDate && !isNaN(pubDate.getTime())
          ? `${pubDate.getFullYear()}-${String(pubDate.getMonth() + 1).padStart(2, '0')}-${String(pubDate.getDate()).padStart(2, '0')} ${String(pubDate.getHours()).padStart(2, '0')}:${String(pubDate.getMinutes()).padStart(2, '0')}`
          : item?.publishDate || '';
      const relativeTime = getRelativeTime(item.publishDate);
      return {
        ...item,
        time: absoluteTime,
        relative: relativeTime,
      };
    },
  );
  const faqData = statusState?.status?.faq || [];

  const uptimeLegendData = Object.entries(UPTIME_STATUS_MAP).map(
    ([status, info]) => ({
      status: Number(status),
      color: info.color,
      label: dashboardData.t(info.label),
    }),
  );

  // ========== Effects ==========
  useEffect(() => {
    initChart();
  }, []);

  const dashboardChartPanelHeight = dashboardData.isMobile
    ? undefined
    : dashboardData.hasApiInfoPanel
      ? 520
      : 520;

  return (
    <div className='dashboard-v2-shell'>
      <DashboardHeader
        getGreeting={dashboardData.getGreeting}
        greetingVisible={dashboardData.greetingVisible}
        showSearchModal={dashboardData.showSearchModal}
        refresh={handleRefresh}
        loading={dashboardData.loading}
        dataExportDefaultTime={dashboardData.dataExportDefaultTime}
        t={dashboardData.t}
      />

      {/* 搜索弹窗 */}
      <SearchModal
        // searchModalVisible={dashboardData.searchModalVisible}
        // handleCloseModal={dashboardData.handleCloseModal}
        searchModalVisible={showScreenModal}
        handleSearchConfirm={handleSearchConfirm}
        handleCloseModal={() => setShowScreenModal(false)}
        isMobile={dashboardData.isMobile}
        isAdminUser={dashboardData.isAdminUser}
        inputs={dashboardData.inputs}
        dataExportDefaultTime={dashboardData.dataExportDefaultTime}
        timeOptions={dashboardData.timeOptions}
        handleInputChange={dashboardData.handleInputChange}
        t={dashboardData.t}
      />

      {/* 面板数据统计 */}
      <StatsCards
        groupedStatsData={groupedStatsData}
        loading={dashboardData.loading}
        getTrendSpec={getTrendSpec}
        CARD_PROPS={CARD_PROPS}
        CHART_CONFIG={CHART_CONFIG}
        displayMode='primary'
        t={dashboardData.t}
      />

      <section className='dashboard-dataAnalys'>
        <div className="dashboard-dataAnalys-header">
          <div className='dataAnalys-header-left'>
            <p>{t('数据分析')}</p>
            <span>{t('统一按筛选周期查看消耗、请求与模型表现')}</span>
          </div>
          <div className='dataAnalys-header-right'>
            <Button theme='outline' type='tertiary' onClick={() => setShowScreenModal(true)}>
              <CalendarDays size={14} />&nbsp;{t('自定义')}
            </Button>
            <Select
              value={dateRangeValue}
              style={{ width: 120 }}
              onChange={handleDateRangeChange}
              optionList={DATE_RANGE_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.label),
              }))}
            />
          </div>
        </div>
        <div className="dashboard-dataAnalys-content">
          <div className="dashboard-dataAnalys-content-top">
            <ChartsPanel
              activeChartTab={dashboardData.activeChartTab}
              setActiveChartTab={dashboardData.setActiveChartTab}
              spec_line={dashboardCharts.spec_line}
              spec_model_line={dashboardCharts.spec_model_line}
              spec_pie={dashboardCharts.spec_pie}
              spec_rank_bar={dashboardCharts.spec_rank_bar}
              CARD_PROPS={CARD_PROPS}
              CHART_CONFIG={CHART_CONFIG}
              FLEX_CENTER_GAP2={FLEX_CENTER_GAP2}
              hasApiInfoPanel={dashboardData.hasApiInfoPanel}
              t={dashboardData.t}
              panelHeight={dashboardChartPanelHeight}
            />
            <ModelHeatRankingPanel
              t={dashboardData.t}
              rankingData={dashboardData.modelPopularRank}
              className='h-full'
            />
          </div>
          <div className="dashboard-dataAnalys-content-bottom">
            <ModelDataAnalysisPanel
              t={dashboardData.t}
              quotaRadioData={dashboardData.modelQuotaRadio}
              spec_model_line={dashboardCharts.spec_model_line}
              spec_pie={dashboardCharts.spec_pie}
              spec_rank_bar={dashboardCharts.spec_rank_bar}
              CHART_CONFIG={CHART_CONFIG}
            />
          </div>
        </div>
      </section>
      
      {/* {dashboardData.hasApiInfoPanel && (
        <div className='dashboard-v2-overview-side-item dashboard-v2-overview-side-item--api'>
          <ApiInfoPanel
            apiInfoData={apiInfoData}
            CARD_PROPS={CARD_PROPS}
            t={dashboardData.t}
            className='h-full'
          />
        </div>
      )} */}
      {/* 配置教程卡片 */}
      {/* <ConfigurationTutorial className='h-full' /> */}

      {/* ========== 底部信息面板 ========== */}
      {dashboardData.hasInfoPanels && (
        <section className='dashboard-v2-bottom-grid'>
          {dashboardData.announcementsEnabled && (
            <AnnouncementsPanel
              announcementData={announcementData}
              announcementLegendData={ANNOUNCEMENT_LEGEND_DATA.map((item) => ({
                ...item,
                label: dashboardData.t(item.label),
              }))}
              CARD_PROPS={CARD_PROPS}
              ILLUSTRATION_SIZE={ILLUSTRATION_SIZE}
              t={dashboardData.t}
            />
          )}

          {dashboardData.faqEnabled && (
            <FaqPanel
              faqData={faqData}
              CARD_PROPS={CARD_PROPS}
              FLEX_CENTER_GAP2={FLEX_CENTER_GAP2}
              ILLUSTRATION_SIZE={ILLUSTRATION_SIZE}
              t={dashboardData.t}
            />
          )}

          {dashboardData.uptimeEnabled && (
            <UptimePanel
              uptimeData={dashboardData.uptimeData}
              uptimeLoading={dashboardData.uptimeLoading}
              activeUptimeTab={dashboardData.activeUptimeTab}
              setActiveUptimeTab={dashboardData.setActiveUptimeTab}
              loadUptimeData={dashboardData.loadUptimeData}
              uptimeLegendData={uptimeLegendData}
              renderMonitorList={(monitors) =>
                renderMonitorList(
                  monitors,
                  (status) => getUptimeStatusColor(status, UPTIME_STATUS_MAP),
                  (status) =>
                    getUptimeStatusText(
                      status,
                      UPTIME_STATUS_MAP,
                      dashboardData.t,
                    ),
                  dashboardData.t,
                )
              }
              CARD_PROPS={CARD_PROPS}
              ILLUSTRATION_SIZE={ILLUSTRATION_SIZE}
              t={dashboardData.t}
            />
          )}
        </section>
      )}
    </div>
  );
};

export default Dashboard;
