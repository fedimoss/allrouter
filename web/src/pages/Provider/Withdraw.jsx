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
import {
  Button,
  Card,
  Empty,
  InputNumber,
  Modal,
  Pagination,
  Select,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconPlus } from '@douyinfe/semi-icons';
import { ArrowUpCircle, Calendar, Info, Wallet } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, timestamp2string, isAdmin, isProviderOwner, showError, showSuccess } from '../../helpers';

const { Text } = Typography;

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];

const STATUS_CONFIG = {
  1: { bgColor: 'orange', textColor: 'rgb(180, 83, 9)',  key: '待审核' },
  2: { bgColor: 'green',  textColor: 'rgb(10, 130, 54)',  key: '已通过' },
  3: { bgColor: 'red',    textColor: 'rgb(185, 28, 28)',  key: '已拒绝' },
  4: { bgColor: 'grey',   textColor: 'rgb(100, 116, 139)', key: '已取消' },
};

const WithdrawPage = () => {
  const { t } = useTranslation();
  const adminMode = isAdmin();
  const ownerMode = !adminMode && isProviderOwner();
  const canAccessWithdraw = adminMode || ownerMode;

  // --- dashboard ---
  const [dashboard, setDashboard] = useState(null);
  const [dashboardLoading, setDashboardLoading] = useState(false);
  const [adminStats, setAdminStats] = useState(null);

  // --- list ---
  const [records, setRecords] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

  // --- admin filters ---
  const [providerName, setProviderName] = useState('');
  const [statusFilter, setStatusFilter] = useState(0);

  // --- apply modal ---
  const [modalVisible, setModalVisible] = useState(false);
  const [withdrawAmount, setWithdrawAmount] = useState(null);
  const [submitting, setSubmitting] = useState(false);

  // --- confirm modal ---
  const [confirmModal, setConfirmModal] = useState(null); // { id, action, title, content }

  // ==================== API calls ====================

  const loadDashboard = async () => {
    if (!ownerMode) return;
    setDashboardLoading(true);
    try {
      const res = await API.get('/api/provider/withdraw/dashboard');
      if (res.data.success) {
        setDashboard(res.data.data);
      }
    } catch {
      // silent
    } finally {
      setDashboardLoading(false);
    }
  };

  const loadAdminDashboard = async () => {
    if (!adminMode) return;
    try {
      const res = await API.get('/api/provider/admin/withdraw/dashboard');
      if (res.data.success) {
        setAdminStats(res.data.data);
      }
    } catch {
      // silent
    }
  };

  const loadList = async () => {
    if (!canAccessWithdraw) return;
    setLoading(true);
    try {
      let url;
      if (adminMode) {
        const params = new URLSearchParams();
        params.set('p', page);
        params.set('page_size', pageSize);
        if (providerName) params.set('provider_name', providerName);
        if (statusFilter > 0) params.set('status', statusFilter);
        url = `/api/provider/admin/withdraw/list?${params.toString()}`;
      } else if (ownerMode) {
        url = `/api/provider/withdraw/list?p=${page}&page_size=${pageSize}`;
      } else {
        return;
      }
      const res = await API.get(url);
      if (res.data.success) {
        const data = res.data.data;
        setRecords(data.items || []);
        setTotal(data.total || 0);
      }
    } catch {
      // silent
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (ownerMode) loadDashboard();
    else setDashboard(null);
    if (adminMode) loadAdminDashboard();
  }, [ownerMode, adminMode]);

  useEffect(() => {
    if (!canAccessWithdraw) {
      setRecords([]);
      setTotal(0);
      setLoading(false);
      return;
    }
    loadList();
  }, [page, pageSize, adminMode, ownerMode, statusFilter]);

  // ==================== actions ====================

  const handleSearch = () => {
    if (!adminMode) return;
    setPage(1);
    loadList();
  };

  const handleSubmit = async () => {
    if (!ownerMode) return;
    const amt = parseFloat(withdrawAmount);
    if (!amt || amt <= 0) {
      showError(t('请输入有效的提现金额'));
      return;
    }
    if (dashboard && amt > dashboard.available_balance) {
      showError(t('提现金额不能超过可用余额'));
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.post(`/api/provider/withdraw/request?amount=${amt}`);
      if (res.data.success) {
        showSuccess(t('提现申请已提交'));
        setModalVisible(false);
        setWithdrawAmount(null);
        loadList();
        loadDashboard();
      } else {
        showError(res.data.message || t('提交失败'));
      }
    } catch {
      showError(t('网络错误'));
    } finally {
      setSubmitting(false);
    }
  };

  const handleApprove = async (id, action) => {
    if (!adminMode) return;
    try {
      const res = await API.post(`/api/provider/admin/withdraw/approve?id=${id}&action=${action}`);
      if (res.data.success) {
        showSuccess(t('操作成功'));
        loadList();
      } else {
        showError(res.data.message || t('操作失败'));
      }
    } catch {
      showError(t('网络错误'));
    }
  };

  const handleCancelRequest = async (id) => {
    if (!ownerMode) return;
    try {
      const res = await API.post(`/api/provider/withdraw/cancel?id=${id}`);
      if (res.data.success) {
        showSuccess(t('提现申请已取消'));
        loadList();
        loadDashboard();
      } else {
        showError(res.data.message || t('操作失败'));
      }
    } catch {
      showError(t('网络错误'));
    }
  };

  // ==================== columns ====================

  const formatNumber = (val, maxDigits) => {
    if (typeof val !== 'number') return val;
    return val.toFixed(maxDigits).replace(/\.?0+$/, '');
  };

  const columns = useMemo(() => {
    const moneyRender = (val, symbol, digits) => (
      <span className='tabular-nums text-slate-700 dark:text-slate-300'>
        {symbol}{formatNumber(val, digits)}
      </span>
    );

    const base = [
      { title: 'ID', dataIndex: 'id', key: 'id', width: 60, align: 'center' },
      {
        title: t('提现金额'),
        dataIndex: 'amount',
        key: 'amount',
        width: 120,
        align: 'right',
        render: (_, record) => (
          <span className='inline-block tabular-nums font-semibold text-slate-800 dark:text-slate-200'>
            {record.currency}{formatNumber(record.amount, 8)}
          </span>
        ),
      },
      {
        title: t('美元金额'),
        dataIndex: 'usd_amount',
        key: 'usd_amount',
        width: 130,
        align: 'right',
        render: (val) => moneyRender(val, '$', 8),
      },
      {
        title: t('人民币金额'),
        dataIndex: 'cny_amount',
        key: 'cny_amount',
        width: 130,
        align: 'right',
        render: (val) => moneyRender(val, '¥', 8),
      },
      {
        title: t('汇率'),
        dataIndex: 'usd_to_cny_rate',
        key: 'usd_to_cny_rate',
        width: 85,
        align: 'right',
        render: (val) => (
          <span className='tabular-nums text-sm text-slate-500 dark:text-slate-400'>
            {formatNumber(val, 8)}
          </span>
        ),
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        key: 'status',
        width: 90,
        align: 'center',
        render: (status) => {
          const config = STATUS_CONFIG[status] || { bgColor: 'grey', textColor: 'rgb(100, 116, 139)', key: String(status) };
          return (
            <Tag
              color={config.bgColor}
              size='small'
              style={{ padding: '0 6px', height: 22, lineHeight: '22px' }}
            >
              <span style={{ color: config.textColor }}>{t(config.key)}</span>
            </Tag>
          );
        },
      },
      {
        title: t('申请时间'),
        dataIndex: 'created_at',
        key: 'created_at',
        width: 160,
        render: (val) => <Text className='text-sm'>{timestamp2string(val)}</Text>,
      },
    ];

    if (adminMode) {
      base.splice(1, 0, {
        title: t('服务商名称'),
        dataIndex: 'provider_name',
        key: 'provider_name',
        width: 140,
        render: (val) => (
          <Text className='truncate' style={{ maxWidth: 120 }}>
            {val || '--'}
          </Text>
        ),
      });
      base.splice(2, 0, {
        title: t('服务商ID'),
        dataIndex: 'provider_id',
        key: 'provider_id',
        width: 80,
        align: 'center',
      });
      base.push({
        title: t('操作'),
        key: 'actions',
        width: 140,
        render: (_, record) => {
          if (record.status !== 1) return <Text type='tertiary'>--</Text>;
          return (
            <div className='flex items-center gap-2'>
              <Button
                size='small'
                theme='light'
                type='primary'
                onClick={() => setConfirmModal({ id: record.id, action: 'approve', title: t('批准提现'), content: t('确认批准该提现申请？') })}
              >
                {t('批准')}
              </Button>
              <Button
                size='small'
                theme='light'
                type='danger'
                onClick={() => setConfirmModal({ id: record.id, action: 'reject', title: t('拒绝提现'), content: t('确认拒绝该提现申请？') })}
              >
                {t('拒绝')}
              </Button>
            </div>
          );
        },
      });
    }

    if (ownerMode) {
      base.push({
        title: t('操作'),
        key: 'actions',
        width: 80,
        render: (_, record) => {
          if (record.status !== 1) return <Text type='tertiary'>--</Text>;
          return (
            <Button
              size='small'
              theme='light'
              type='danger'
              onClick={() => setConfirmModal({ id: record.id, action: 'cancel', title: t('取消提现'), content: t('确认取消该提现申请？') })}
            >
              {t('取消')}
            </Button>
          );
        },
      });
    }

    return base;
  }, [t, adminMode, ownerMode]);

  // ==================== guards ====================

  if (!canAccessWithdraw) {
    return (
      <div className='flex h-64 items-center justify-center'>
        <Empty title={t('无权访问')} />
      </div>
    );
  }

  // ==================== render ====================

  const statusOptions = [
    { value: 0, label: t('全部状态') },
    { value: 1, label: t('待审核') },
    { value: 2, label: t('已通过') },
    { value: 3, label: t('已拒绝') },
    { value: 4, label: t('已取消') },
  ];

  const start = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const end = Math.min(page * pageSize, total);

  return (
    <div className='flex flex-col gap-4 pb-4'>

      {/* ======== Header cards ======== */}
      <div className={`grid grid-cols-1 gap-6 md:grid-cols-2`}>
        {/* Balance card (owner) or Title card (admin) */}
        {ownerMode ? (
          <Card
            bordered
            bodyStyle={{ padding: 0 }}
            className='!rounded-[20px] dark:!border-slate-700'
          >
            <div
              className={`flex h-full flex-col justify-between gap-4 rounded-[20px] bg-white px-7 py-6 dark:bg-slate-800 ${
                dashboardLoading ? 'opacity-70' : 'opacity-100'
              }`}
            >
              <div className='flex items-start justify-between gap-6'>
                <div className='text-[15px] font-semibold text-slate-400'>
                  {t('可用余额')}
                </div>
                <div className='flex h-12 w-12 items-center justify-center rounded-2xl bg-emerald-50 dark:bg-emerald-500/10'>
                  <ArrowUpCircle size={22} className='text-emerald-500' />
                </div>
              </div>
              <div className='space-y-2'>
                <div className='text-[26px] font-black text-slate-800 dark:text-slate-100'>
                  {dashboard
                    ? `${dashboard.currency}${Number(dashboard.available_balance).toFixed(4)}`
                    : '--'}
                </div>
                <div className='text-sm text-slate-400'>
                  {dashboard?.currency === '￥' ? t('人民币余额') : t('美元余额')}
                </div>
              </div>
            </div>
          </Card>
        ) : (
          <>
            <Card
              bordered
              bodyStyle={{ padding: 0 }}
              className='!rounded-[20px] dark:!border-slate-700'
            >
              <div className='flex h-full flex-col justify-between gap-4 rounded-[20px] bg-white px-7 py-6 dark:bg-slate-800'>
                <div className='flex items-start justify-between gap-6'>
                  <div className='text-[15px] font-semibold text-slate-400'>
                    {t('总提现次数')}
                  </div>
                  <div className='flex h-12 w-12 items-center justify-center rounded-2xl bg-blue-50 dark:bg-blue-500/10'>
                    <Wallet size={22} className='text-blue-500' />
                  </div>
                </div>
                <div className='space-y-2'>
                  <div className='text-[26px] font-black text-slate-800 dark:text-slate-100'>
                    {adminStats ? adminStats.total_count : '--'}
                  </div>
                  <div className='text-sm text-slate-400'>
                    {t('全部提现申请')}
                  </div>
                </div>
              </div>
            </Card>
            <Card
              bordered
              bodyStyle={{ padding: 0 }}
              className='!rounded-[20px] dark:!border-slate-700'
            >
              <div className='flex h-full flex-col justify-between gap-4 rounded-[20px] bg-white px-7 py-6 dark:bg-slate-800'>
                <div className='flex items-start justify-between gap-6'>
                  <div className='text-[15px] font-semibold text-slate-400'>
                    {t('今日提现次数')}
                  </div>
                  <div className='flex h-12 w-12 items-center justify-center rounded-2xl bg-orange-50 dark:bg-orange-500/10'>
                    <Calendar size={22} className='text-orange-500' />
                  </div>
                </div>
                <div className='space-y-2'>
                  <div className='text-[26px] font-black text-slate-800 dark:text-slate-100'>
                    {adminStats ? adminStats.today_count : '--'}
                  </div>
                  <div className='text-sm text-slate-400'>
                    {t('今日提交的申请')}
                  </div>
                </div>
              </div>
            </Card>
          </>
        )}

        {/* Description card (owner) */}
        {ownerMode && (
          <Card
            bordered
            bodyStyle={{ padding: 0 }}
            className='!rounded-[20px] dark:!border-slate-700'
          >
            <div className='flex h-full flex-col justify-between gap-4 rounded-[20px] bg-white px-7 py-6 dark:bg-slate-800'>
              <div className='flex items-start justify-between gap-6'>
                <div className='text-[15px] font-semibold text-slate-400'>
                  {t('提现说明')}
                </div>
                <div className='flex h-12 w-12 items-center justify-center rounded-2xl bg-amber-50 dark:bg-amber-500/10'>
                  <Info size={22} className='text-amber-500' />
                </div>
              </div>
              <div className='space-y-3'>
                <div className='rounded-xl bg-slate-50 px-4 py-3 dark:bg-slate-700/50'>
                  <div className='flex items-center gap-2 text-sm text-slate-500 dark:text-slate-400'>
                    <span className='font-medium text-slate-600 dark:text-slate-300'>{t('可提现余额')}</span>
                    <span>=</span>
                    <span>{t('总余额')}</span>
                    <span className='text-slate-300'>−</span>
                    <span>{t('奖励余额')}</span>
                  </div>
                </div>
                <div className='flex items-start gap-2 text-xs text-slate-400'>
                  <span className='mt-0.5 shrink-0'>·</span>
                  <span>{t('提现金额不能超过可用余额')}</span>
                </div>
              </div>
            </div>
          </Card>
        )}
      </div>

      {/* ======== Filter bar (admin only) ======== */}
      {adminMode && (
        <div className='flex flex-wrap items-center justify-end gap-3'>
          <div className='flex items-center gap-2 rounded-2xl border border-slate-200 bg-white px-4 py-2 dark:border-slate-700 dark:bg-slate-800'>
            <span className='text-sm font-medium text-slate-400'>{t('服务商')}</span>
            <input
              type='text'
              placeholder={t('输入名称搜索')}
              value={providerName}
              onChange={(e) => setProviderName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
              className='min-w-[160px] border-none bg-transparent text-sm text-slate-700 outline-none dark:text-slate-300'
            />
          </div>
          <Select
            value={statusFilter}
            optionList={statusOptions}
            onChange={(v) => {
              setStatusFilter(v);
              setPage(1);
            }}
            className='min-w-[120px]'
          />
          <Button onClick={handleSearch}>{t('搜索')}</Button>
        </div>
      )}

      {/* ======== Table card ======== */}
      <Card bodyStyle={{ padding: 0 }} bordered={false} className='!rounded-2xl'>
        <div className='flex items-center justify-between border-b border-slate-100 px-6 py-4 dark:border-slate-700'>
          <span className='text-lg font-bold text-slate-800 dark:text-slate-300'>
            {t('提现记录')}
          </span>
          {ownerMode && (
            <Button
              type='primary'
              icon={<IconPlus />}
              onClick={() => setModalVisible(true)}
            >
              {t('申请提现')}
            </Button>
          )}
        </div>

        <Table
          columns={columns}
          dataSource={records}
          loading={loading}
          rowKey='id'
          pagination={false}
          empty={<Empty title={t('暂无记录')} />}
        />

        <div className='flex items-center justify-between border-t border-slate-100 px-6 py-4 dark:border-slate-700'>
          <Text type='tertiary' className='text-sm'>
            {t('显示第 {{start}} - {{end}} 条，共 {{total}} 条', { start, end, total })}
          </Text>
          <div className='flex items-center gap-3'>
            <Select
              value={pageSize}
              optionList={PAGE_SIZE_OPTIONS.map((v) => ({ value: v, label: `${v} ${t('条/页')}` }))}
              onChange={(v) => {
                setPageSize(v);
                setPage(1);
              }}
              size='small'
            />
            <Pagination
              total={total}
              pageSize={pageSize}
              currentPage={page}
              onPageChange={setPage}
              showSizeChanger={false}
              size='small'
            />
          </div>
        </div>
      </Card>

      {/* ======== Confirm modal ======== */}
      <Modal
        title={confirmModal?.title || ''}
        visible={!!confirmModal}
        onCancel={() => setConfirmModal(null)}
        onOk={async () => {
          if (!confirmModal) return;
          const { id, action } = confirmModal;
          setConfirmModal(null);
          if (action === 'cancel') {
            await handleCancelRequest(id);
          } else {
            await handleApprove(id, action);
          }
        }}
        okText={t('确认')}
        cancelText={t('取消')}
      >
        <div className='py-4'>
          <Text>{confirmModal?.content || ''}</Text>
        </div>
      </Modal>

      {/* ======== Apply modal ======== */}
      <Modal
        title={t('申请提现')}
        visible={modalVisible}
        onCancel={() => {
          setModalVisible(false);
          setWithdrawAmount(null);
        }}
        onOk={handleSubmit}
        confirmLoading={submitting}
        okText={t('确认提现')}
        cancelText={t('取消')}
      >
        <div className='py-4'>
          {dashboard && (
            <div className='mb-4 rounded-xl bg-slate-50 px-4 py-3 dark:bg-slate-700'>
              <div className='text-sm text-slate-500 dark:text-slate-400'>
                {t('当前可用余额')}
              </div>
              <div className='mt-1 text-xl font-bold text-slate-800 dark:text-slate-100'>
                {dashboard.currency}{Number(dashboard.available_balance).toFixed(4)}
              </div>
            </div>
          )}
          <div className='text-sm font-medium text-slate-600 dark:text-slate-300 mb-2'>
            {t('提现金额')}
          </div>
          <InputNumber
            value={withdrawAmount}
            onChange={setWithdrawAmount}
            placeholder={t('请输入提现金额')}
            min={0}
            className='w-full'
            size='large'
          />
        </div>
      </Modal>
    </div>
  );
};

export default WithdrawPage;
