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
import { Button, Spin, Tabs, TabPane, Tag,Empty } from '@douyinfe/semi-ui';
import { Gauge, RefreshCw } from 'lucide-react';
import ScrollableContainer from '../common/ui/ScrollableContainer';
import {
  IllustrationConstruction,
  IllustrationConstructionDark,
} from '@douyinfe/semi-illustrations';
import sjjkIcon from '../../../public/board-xxph.svg';

import './index.scss';

const fallbackMonitorCards = [];

const UptimePanel = ({
  uptimeData,
  uptimeLoading,
  activeUptimeTab,
  setActiveUptimeTab,
  loadUptimeData,
  uptimeLegendData,
  renderMonitorList,
  CARD_PROPS,
  ILLUSTRATION_SIZE,
  t,
}) => {
  return (
    <section className='custom-card dashboard-v2-bottom-card'>
      <div className='custom-card__header'>
        <div className='header-left'>
          <img src={sjjkIcon} alt="" />
          <span>{t('数据监控')}</span>
        </div>
        <Button
          icon={<RefreshCw size={14} />}
          onClick={loadUptimeData}
          loading={uptimeLoading}
          size='small'
          theme='borderless'
          type='tertiary'
          className='dashboard-v2-bottom-refresh'
        />
      </div>
      <div className='flex1-content'>
        <div className='relative'>
          <Spin spinning={uptimeLoading}>
            {uptimeData.length > 0 ? (
              uptimeData.length === 1 ? (
                <ScrollableContainer maxHeight='24rem'>
                  {renderMonitorList(uptimeData[0].monitors)}
                </ScrollableContainer>
              ) : (
                <Tabs
                  type='card'
                  collapsible
                  activeKey={activeUptimeTab}
                  onChange={setActiveUptimeTab}
                  size='small'
                >
                  {uptimeData.map((group, groupIdx) => (
                    <TabPane
                      tab={
                        <span className='flex items-center gap-2'>
                          <Gauge size={14} />
                          {group.categoryName}
                          <Tag
                            color={
                              activeUptimeTab === group.categoryName
                                ? 'red'
                                : 'grey'
                            }
                            size='small'
                            shape='circle'
                          >
                            {group.monitors ? group.monitors.length : 0}
                          </Tag>
                        </span>
                      }
                      itemKey={group.categoryName}
                      key={groupIdx}
                    >
                      <ScrollableContainer maxHeight='21.5rem'>
                        {renderMonitorList(group.monitors)}
                      </ScrollableContainer>
                    </TabPane>
                  ))}
                </Tabs>
              )
            ) : (
              <div className='monitor-placeholder-list'>
                {fallbackMonitorCards.map((item) => (
                  <div key={item.title} className='monitor-placeholder-card'>
                    <div className='monitor-placeholder-copy'>
                      <div className='monitor-placeholder-title'>
                        {t(item.title)}
                      </div>
                      <div className='monitor-placeholder-desc'>
                        {t(item.desc)}
                      </div>
                    </div>
                    <div className={item.badgeClass}>{t(item.badge)}</div>
                  </div>
                ))}
              </div>
            )}
          </Spin>
        </div>

        {uptimeData.length > 0 ? (
          <div
            className='rounded-b-2xl p-3'
            style={{ backgroundColor: 'var(--semi-color-fill-0)' }}
          >
            <div className='flex flex-wrap gap-3 text-xs justify-center'>
              {uptimeLegendData.map((legend, index) => (
                <div key={index} className='flex items-center gap-1'>
                  <div
                    className='w-2 h-2 rounded-full'
                    style={{ backgroundColor: legend.color }}
                  />
                  <span style={{ color: 'var(--semi-color-text-1)' }}>
                    {legend.label}
                  </span>
                </div>
              ))}
            </div>
          </div>
        ) : (
          <div className='flex justify-center items-center py-8'>
            <Empty
              image={<IllustrationConstruction style={ILLUSTRATION_SIZE} />}
              darkModeImage={
                <IllustrationConstructionDark style={ILLUSTRATION_SIZE} />
              }
              title={t('暂无监控数据')}
              description={t('请联系管理员在系统设置中配置Uptime')}
            />
          </div>
        )}
      </div>
    </section>
  );
};

export default UptimePanel;
