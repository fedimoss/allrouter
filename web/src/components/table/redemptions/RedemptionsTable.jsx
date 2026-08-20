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
import { Empty } from '@douyinfe/semi-ui';
import CardTable from '../../common/ui/CardTable';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { getRedemptionsColumns, isExpired } from './RedemptionsColumnDefs';
import DeleteRedemptionModal from './modals/DeleteRedemptionModal';
import SendRedemptionModal from './modals/SendRedemptionModal';

const RedemptionsTable = (redemptionsData) => {
  const {
    redemptions,
    loading,
    activePage,
    pageSize,
    tokenCount,
    compactMode,
    displaySymbol,
    handlePageChange,
    rowSelection,
    handleRow,
    manageRedemption,
    copyText,
    setEditingRedemption,
    setShowEdit,
    refresh,
    sentFilter,
    setSentFilter,
    toggleSent,
    sendingRecord,
    setSendingRecord,
    sendingEmail,
    confirmSendRedemption,
    t,
  } = redemptionsData;

  // Modal states
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deletingRecord, setDeletingRecord] = useState(null);

  // Handle show delete modal
  const showDeleteRedemptionModal = (record) => {
    setDeletingRecord(record);
    setShowDeleteModal(true);
  };

  // Handle filter change from table header (sent filter)
  // 表头筛选变化回调：从 Semi Table 的 onChange 中取出「发放」列的筛选值，
  // 更新 sentFilter 后由 useEffect 触发服务端重新查询
  const handleTableChange = ({ filters }) => {
    const sentColumn = filters?.find((f) => f.dataIndex === 'sent_time');
    const value = sentColumn?.filteredValue?.[0];
    if (value !== undefined && value !== sentFilter) {
      setSentFilter(value);
    }
  };

  // Get all columns
  const columns = useMemo(() => {
    return getRedemptionsColumns({
      t,
      manageRedemption,
      copyText,
      setEditingRedemption,
      setShowEdit,
      refresh,
      redemptions,
      activePage,
      displaySymbol,
      showDeleteRedemptionModal,
      sentFilter,
      toggleSent,
    });
  }, [
    t,
    manageRedemption,
    copyText,
    setEditingRedemption,
    setShowEdit,
    refresh,
    redemptions,
    activePage,
    displaySymbol,
    showDeleteRedemptionModal,
    sentFilter,
    toggleSent,
  ]);

  // Handle compact mode by removing fixed positioning
  const tableColumns = useMemo(() => {
    return compactMode
      ? columns.map((col) => {
          if (col.dataIndex === 'operate') {
            const { fixed, ...rest } = col;
            return rest;
          }
          return col;
        })
      : columns;
  }, [compactMode, columns]);

  return (
    <>
      <CardTable
        columns={tableColumns}
        dataSource={redemptions}
        scroll={compactMode ? undefined : { x: 'max-content' }}
        pagination={{
          currentPage: activePage,
          pageSize: pageSize,
          total: tokenCount,
          onPageChange: handlePageChange,
        }}
        hidePagination={true}
        loading={loading}
        rowSelection={rowSelection}
        onRow={handleRow}
        // 表头筛选变化（发放状态）由此回调通知，驱动服务端查询
        onChange={handleTableChange}
        empty={
          <Empty
            image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
            darkModeImage={
              <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
            }
            description={t('搜索无结果')}
            style={{ padding: 30 }}
          />
        }
        className='rounded-xl overflow-hidden'
        size='middle'
      />

      <DeleteRedemptionModal
        visible={showDeleteModal}
        onCancel={() => setShowDeleteModal(false)}
        record={deletingRecord}
        manageRedemption={manageRedemption}
        refresh={refresh}
        redemptions={redemptions}
        activePage={activePage}
        t={t}
      />

      <SendRedemptionModal
        visible={!!sendingRecord}
        onCancel={() => setSendingRecord(null)}
        onOk={confirmSendRedemption}
        loading={sendingEmail}
        t={t}
      />
    </>
  );
};

export default RedemptionsTable;
