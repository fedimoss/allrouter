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

import React, { useEffect, useMemo, useState } from 'react';
import { Modal, Table, Tag, Pagination, Empty } from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { API, showError, timestamp2string } from '../../../helpers';

const safeFormatTimestamp = (value) => {
  if (value === null || value === undefined || value === '') {
    return '-';
  }
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) {
    return '-';
  }
  try {
    return timestamp2string(numeric);
  } catch {
    return '-';
  }
};

const InviteDetailModal = ({ t, visible, onClose, inviteeId, inviteeName }) => {
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState([]);
  const [displaySymbol, setDisplaySymbol] = useState('¥');
  const [totalRebateQuota, setTotalRebateQuota] = useState(0);
  const [secondLevelRebateQuota, setSecondLevelRebateQuota] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(10);
  const [total, setTotal] = useState(0);

  const loadDetailRecords = async (currentPage = 1) => {
    if (!inviteeId) return;
    setLoading(true);
    try {
      const res = await API.get(
        `/api/user/topup/rebate/records?user_id=${inviteeId}&p=${currentPage}&page_size=${pageSize}`,
      );
      const { success, message, data } = res.data;
      if (success) {
        setRows(data?.items || []);
        setTotal(data?.total || 0);
        setDisplaySymbol(data?.display_symbol || '¥');
        setTotalRebateQuota(data?.total_rebate_quota || 0);
        setSecondLevelRebateQuota(data?.level2_total_rebate_quota || 0);
      } else {
        showError(message || t('加载失败'));
      }
    } catch {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!visible) return;
    setPage(1);
    loadDetailRecords(1).then();
  }, [visible, inviteeId]);

  useEffect(() => {
    if (!visible) return;
    loadDetailRecords(page).then();
  }, [page]);

  const columns = useMemo(
    () => [
      {
        title: <span className='text-[#98A2B3] text-[14px] font-medium'>{t('时间')}</span>,
        dataIndex: 'created_at',
        key: 'created_at',
        width: '34%',
        render: (value) => {
          const full = safeFormatTimestamp(value);
          if (full === '-') {
            return <div className='text-[#475467] text-[14px] leading-[20px] font-semibold'>-</div>;
          }
          const [date, time] = String(full).split(' ');
          return (
            <div className='whitespace-pre-line text-[#475467] text-[14px] leading-[20px] font-semibold'>
              {`${time || ''}\n${date || ''}`}
            </div>
          );
        },
      },
      {
        title: <span className='text-[#98A2B3] text-[14px] font-medium'>{t('类型')}</span>,
        dataIndex: 'type',
        key: 'type',
        width: '33%',
        render: () => (
          <Tag className='!bg-[#DFFFF9] !text-[#22D3C5] !border !border-[#8BECE4] !rounded-[6px] !px-[10px] !py-[2px] !text-[12px] !leading-none'>
            {t('消费')}
          </Tag>
        ),
      },
      {
        title: <span className='text-[#98A2B3] text-[14px] font-medium'>{t('奖励')}</span>,
        dataIndex: 'rebate_quota',
        key: 'rebate_quota',
        width: '33%',
        render: (value) => (
          <span className='text-[#475467] text-[14px] font-semibold'>
            {displaySymbol} {value ?? 0}
          </span>
        ),
      },
    ],
    [displaySymbol, t],
  );

  const start = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const end = Math.min(page * pageSize, total);

  return (
    <Modal
      visible={visible}
      onCancel={onClose}
      footer={null}
      centered
      maskClosable
      closable
      width={600}
      title={<div className='text-[16px] leading-[24px] font-semibold text-[#344054]'>{t('消费返利')}</div>}
      bodyStyle={{ padding: '0 30px 24px' }}
      className='invite-detail-modal'
    >
      <div className='border-t border-[#E5E7EB] pt-5'>
        <div className='text-[16px] leading-[24px] font-semibold text-[#344054]'>{t('被邀请人')}</div>
        <div className='mt-4 space-y-3'>
          <div className='flex items-center'>
            <span className='w-[110px] text-[14px] leading-[20px] text-[#98A2B3]'>{t('被邀请人ID：')}</span>
            <span className='text-[14px] leading-[20px] text-[#475467] font-semibold'>{inviteeName || '-'}</span>
          </div>
          <div className='flex items-center'>
            <span className='w-[130px] text-[14px] leading-[20px] text-[#98A2B3]'>{t('一级累计贡献返利：')}</span>
            <span className='text-[14px] leading-[20px] text-[#475467] font-semibold'>
              {displaySymbol} {totalRebateQuota}
            </span>
          </div>
          <div className='flex items-center'>
            <span className='w-[130px] text-[14px] leading-[20px] text-[#98A2B3]'>{t('二级累计贡献返利：')}</span>
            <span className='text-[14px] leading-[20px] text-[#475467] font-semibold'>
              {displaySymbol} {secondLevelRebateQuota}
            </span>
          </div>
        </div>
      </div>

      <div className='mt-6 border-t border-[#E5E7EB] pt-5'>
        <div className='text-[16px] leading-[24px] font-semibold text-[#344054] mb-4'>{t('返利明细')}</div>
        <Table
          columns={columns}
          dataSource={rows}
          rowKey='id'
          pagination={false}
          size='default'
          loading={loading}
          empty={
            <Empty
              image={<IllustrationNoResult style={{ width: 120, height: 120 }} />}
              darkModeImage={<IllustrationNoResultDark style={{ width: 120, height: 120 }} />}
              description={t('暂无返利记录')}
            />
          }
        />

        <div className='mt-6 flex items-center justify-between'>
          <div className='text-[#475467] text-[14px] leading-[20px] font-semibold'>
            {t('显示 {{start}} - {{end}} 共 {{total}} 条记录', { start, end, total })}
          </div>
          <Pagination
            total={total}
            pageSize={pageSize}
            currentPage={page}
            onPageChange={setPage}
            size='small'
            showQuickJumper
            showSizeChanger={false}
          />
        </div>
      </div>
    </Modal>
  );
};

export default InviteDetailModal;
