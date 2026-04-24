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
import LogsTable from './UsageLogsTable';
import LogsActions from './UsageLogsActions';
import LogsFilters from './UsageLogsFilters';
import ColumnSelectorModal from './modals/ColumnSelectorModal';
import UserInfoModal from './modals/UserInfoModal';
import ChannelAffinityUsageCacheModal from './modals/ChannelAffinityUsageCacheModal';
import ParamOverrideModal from './modals/ParamOverrideModal';
import { useLogsData } from '../../../hooks/usage-logs/useUsageLogsData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

const LogsPage = () => {
  const logsData = useLogsData();
  const isMobile = useIsMobile();

  const paginationArea = createCardProPagination({
    currentPage: logsData.activePage,
    pageSize: logsData.pageSize,
    total: logsData.logCount,
    onPageChange: logsData.handlePageChange,
    onPageSizeChange: logsData.handlePageSizeChange,
    isMobile,
    t: logsData.t,
  });

  return (
    <>
      <ColumnSelectorModal {...logsData} />
      <UserInfoModal {...logsData} />
      <ChannelAffinityUsageCacheModal {...logsData} />
      <ParamOverrideModal {...logsData} />

      <div className='log-v2 usage-logs-v2'>
        <div className='log-v2-shell'>
          <div className='log-v2-stack'>
            <section className='usage-logs-v2-header'>
              <div className='usage-logs-v2-title'>{logsData.t('使用日志')}</div>
              <p className='usage-logs-v2-description'>
                {logsData.t('查看并分析您的 API 调用详细数据和实时状态。')}
              </p>
            </section>

            <section>
              <LogsActions {...logsData} />
            </section>

            <section className='log-v2-filter-card usage-logs-v2-filter-card'>
              <LogsFilters {...logsData} />
            </section>

            <section className='log-v2-table-card usage-logs-v2-table-card'>
              <div className='usage-logs-v2-table-wrap'>
                <LogsTable {...logsData} />
              </div>
              {paginationArea && (
                <div className='usage-logs-v2-pagination'>{paginationArea}</div>
              )}
            </section>
          </div>
        </div>
      </div>
    </>
  );
};

export default LogsPage;
