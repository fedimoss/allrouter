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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Dropdown } from '@douyinfe/semi-ui';
import { IconLoading } from '@douyinfe/semi-icons';
import {
  Activity,
  ArrowUpDown,
  BarChart3,
  CalendarDays,
  ChevronDown,
  Eye,
  Coins,
  Gift,
  Search,
  SlidersHorizontal,
  Users,
  WalletCards,
  X,
} from 'lucide-react';
import { API, showError } from '../../helpers';
import {
  DATE_RANGE_OPTIONS,
  TAB_CONFIG,
  blockPanelClassName,
  gradientButtonStyle,
  inputClassName,
  lightButtonClassName,
  pagePanelClassName,
} from './config';
import {
  buildDashboardCards,
  buildRecordsParams,
  countAppliedFilters,
  createInitialFilters,
  extractListPayload,
  extractResponsePayload,
  firstDefined,
  formatDateTime,
  formatInteger,
  formatQuotaValue,
  getDefaultVisibleColumns,
  getRegistrationSourceLabel,
  getRetentionMeta,
  getSortLabel,
  hasValue,
  joinClasses,
} from './utils';

const ICON_MAP = {
  users: Users,
  wallet: WalletCards,
  activity: Activity,
  topup: Coins,
  welfare: Gift,
  trend: BarChart3,
  calendar: CalendarDays,
};

const FOOTER_TONE_CLASS = {
  positive: 'text-cyan-500',
  negative: 'text-rose-500',
  neutral: 'text-slate-400 dark:text-slate-500',
};

function LoadingIcon({ className = 'h-5 w-5' }) {
  return <IconLoading className={joinClasses(className, 'animate-spin text-cyan-500')} />;
}

function normalizeRow(tabKey, item, index) {
  if (tabKey === 'user') {
    return {
      id: firstDefined(item, ['id', 'user_id', 'userId', 'uid'], `user-${index}`),
      userId: firstDefined(item, ['user_id', 'userId', 'uid', 'id'], ''),
      nickname: firstDefined(item, ['nickname', 'username', 'name', 'display_name'], ''),
      lastActiveTime: firstDefined(item, ['last_active_time', 'lastActiveTime'], ''),
      invited: firstDefined(item, ['invited'], false),
      retention: firstDefined(item, ['retention'], ''),
      quota: firstDefined(item, ['quota', 'balance'], ''),
      requestCount: firstDefined(item, ['request_count', 'requestCount'], ''),
      topupQuota: firstDefined(item, ['topup_quota', 'topupQuota'], ''),
      welfareQuota: firstDefined(item, ['welfare_quota', 'welfareQuota'], ''),
      usedQuota: firstDefined(
        item,
        ['used_quota', 'usedQuota', 'consume_quota', 'consumeQuota'],
        '',
      ),
      registerAt: firstDefined(
        item,
        ['register_at', 'registerAt', 'created_at', 'createdAt', 'created_time'],
        '',
      ),
    };
  }

  return {
    id: firstDefined(item, ['id'], `row-${index}`),
    ...item,
  };
}

function DateRangeDropdown({ activeRange, onChange }) {
  const activeLabel =
    DATE_RANGE_OPTIONS.find((option) => option.value === activeRange)?.label || '今日';

  return (
    <Dropdown
      clickToHide
      position='bottomRight'
      trigger='click'
      render={
        <Dropdown.Menu className='!min-w-[140px] !rounded-[18px] !border !border-slate-200/90 !bg-white !p-1.5 !shadow-[0_18px_40px_rgba(148,163,184,0.18)] dark:!border-slate-700 dark:!bg-slate-900'>
          {DATE_RANGE_OPTIONS.map((option) => {
            const active = option.value === activeRange;
            return (
              <Dropdown.Item
                key={option.value}
                onClick={() => onChange(option.value)}
                className={joinClasses(
                  '!rounded-[14px] !px-3.5 !py-2.5 !text-sm !font-medium !text-slate-600 transition dark:!text-slate-200',
                  active
                    ? '!bg-slate-100 !text-slate-900 dark:!bg-slate-800 dark:!text-white'
                    : 'hover:!bg-slate-50 dark:hover:!bg-slate-800/70',
                )}
              >
                <div className='flex items-center justify-between gap-3'>
                  <span>{option.label}</span>
                  {active ? (
                    <span className='h-2 w-2 rounded-full bg-cyan-400 shadow-[0_0_0_4px_rgba(34,211,238,0.12)]' />
                  ) : null}
                </div>
              </Dropdown.Item>
            );
          })}
        </Dropdown.Menu>
      }
    >
      <button
        className='inline-flex h-[50px] items-center justify-between rounded-[20px] border border-slate-200/90 bg-white px-4 text-left shadow-[0_8px_20px_rgba(148,163,184,0.12)] transition hover:border-slate-300 hover:shadow-[0_10px_24px_rgba(148,163,184,0.16)] focus:border-cyan-300 focus:outline-none focus:ring-4 focus:ring-cyan-100 dark:border-slate-700 dark:bg-slate-900 dark:hover:border-slate-600 dark:focus:border-cyan-500 dark:focus:ring-cyan-900/30'
        type='button'
      >
        <div className='flex  justify-between leading-none'>
          <span className='text-[12px] font-medium leading-none mr-4 text-slate-400 dark:text-slate-500'>日期选择</span>
          <span className='text-[15px] font-semibold p-2 leading-none text-slate-700 dark:text-slate-100'>{activeLabel}</span>
        </div>
        <ChevronDown className='h-4 w-4 shrink-0 text-slate-400' />
      </button>
    </Dropdown>
  );
}
function formatMetricValue(metric) {
  if (metric.valueType === 'quota') {
    return formatQuotaValue(metric.value);
  }
  if (metric.valueType === 'count') {
    return formatInteger(metric.value);
  }
  return hasValue(metric.value) ? String(metric.value) : '--';
}

function MetricCard({ metric, loading }) {
  const Icon = ICON_MAP[metric.icon] || BarChart3;
  const footerToneClass = FOOTER_TONE_CLASS[metric.footer?.tone] || FOOTER_TONE_CLASS.neutral;

  return (
    <article className={joinClasses(blockPanelClassName, 'p-5 sm:p-6')}>
      <div className='flex items-start justify-between gap-4'>
        <div className='min-w-0'>
          <p className='text-sm text-slate-500 dark:text-slate-400'>{metric.title}</p>
          <div className='mt-4 min-h-[44px] text-[24px] font-semibold tracking-tight text-slate-900 dark:text-white sm:text-[28px]'>
            {loading ? <LoadingIcon className='h-6 w-6' /> : formatMetricValue(metric)}
          </div>
        </div>
        <div className='flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-[#f8fafc] text-slate-400 dark:bg-slate-800 dark:text-slate-300'>
          <Icon className='h-5 w-5' />
        </div>
      </div>
      <div className='mt-5 min-h-[22px] text-sm'>
        {metric.footer?.trend ? <span className={joinClasses('font-semibold', footerToneClass)}>{metric.footer.trend}</span> : null}
        {metric.footer?.text ? <span className={joinClasses(metric.footer?.trend ? 'ml-2' : '', 'text-slate-400 dark:text-slate-500')}>{metric.footer.text}</span> : null}
      </div>
    </article>
  );
}

function ColumnMenu({ columns, visibleColumnKeys, onToggle }) {
  return (
    <div
      className={joinClasses(
        blockPanelClassName,
        'absolute right-0 top-14 z-30 w-[220px] p-3 sm:w-[240px]',
      )}
    >
      <div className='space-y-2'>
        {columns.map((column) => {
          const checked = visibleColumnKeys.includes(column.key);
          return (
            <label
              key={column.key}
              className='flex cursor-pointer items-center gap-3 rounded-xl px-2 py-2 text-sm text-slate-600 transition hover:bg-slate-50 dark:text-slate-200 dark:hover:bg-slate-800/70'
            >
              <span
                className={joinClasses(
                  'flex h-4 w-4 items-center justify-center rounded-[4px] border text-[10px] font-semibold',
                  checked
                    ? 'border-transparent text-slate-950'
                    : 'border-slate-300 text-transparent dark:border-slate-600',
                )}
                style={checked ? gradientButtonStyle : undefined}
              >
                ✓
              </span>
              <span>{column.label}</span>
              <input
                checked={checked}
                className='sr-only'
                onChange={() => onToggle(column.key)}
                type='checkbox'
              />
            </label>
          );
        })}
      </div>
    </div>
  );
}

function renderCell(column, row) {
  const displayName = row.userId || '--';
  const subTitle = row.nickname || '';

  switch (column.key) {
    case 'user':
      return (
        <div className='min-w-[180px]'>
          <div className='text-sm font-semibold text-slate-800 dark:text-slate-100'>
            {displayName}
          </div>
        </div>
      );
    case 'source':
      return getRegistrationSourceLabel(row.invited);
    case 'retention': {
      const retentionMeta = getRetentionMeta(row.retention);
      return (
        <span
          className={joinClasses(
            'inline-flex rounded-full px-2.5 py-1 text-xs font-semibold',
            retentionMeta.className,
          )}
        >
          {retentionMeta.label}
        </span>
      );
    }
    case 'detail':
      return (
        <span className='inline-flex h-9 w-9 items-center justify-center rounded-full text-slate-400 ring-1 ring-inset ring-slate-200 dark:text-slate-300 dark:ring-slate-700'>
          <Eye className='h-4 w-4' />
        </span>
      );
    case 'lastActiveTime':
      return formatDateTime(row.lastActiveTime);
    case 'quota':
      return formatQuotaValue(row.quota);
    case 'requestCount':
      return formatInteger(row.requestCount);
    case 'topupQuota':
      return formatQuotaValue(row.topupQuota);
    case 'welfareQuota':
      return formatQuotaValue(row.welfareQuota);
    case 'usedQuota':
      return formatQuotaValue(row.usedQuota);
    case 'registerAt':
      return formatDateTime(row.registerAt);
    default:
      return hasValue(row[column.key]) ? String(row[column.key]) : '--';
  }
}
function DesktopTable({ columns, rows, sortState, onSortChange }) {
  return (
    <div className='overflow-x-auto'>
      <table className='min-w-full'>
        <thead>
          <tr className='border-b border-slate-200 dark:border-slate-800'>
            {columns.map((column) => (
              <th
                key={column.key}
                className='px-4 py-4 text-left text-xs font-semibold text-slate-400 dark:text-slate-500'
              >
                {column.sortable ? (
                  <button
                    className='inline-flex items-center gap-1.5 transition hover:text-slate-700 dark:hover:text-slate-200'
                    onClick={() => onSortChange(column.sortField)}
                    type='button'
                  >
                    <span>{column.title}</span>
                    <ArrowUpDown className='h-3.5 w-3.5' />
                    {getSortLabel(sortState, column.sortField) ? (
                      <span className='text-[11px]'>
                        {getSortLabel(sortState, column.sortField)}
                      </span>
                    ) : null}
                  </button>
                ) : (
                  <span>{column.title}</span>
                )}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr
              key={row.id}
              className='border-b border-slate-100 last:border-b-0 dark:border-slate-800/80'
            >
              {columns.map((column) => (
                <td key={column.key} className='px-4 py-5 align-middle'>
                  <div className='text-sm font-medium text-slate-700 dark:text-slate-200'>
                    {renderCell(column, row)}
                  </div>
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function MobileCards({ columns, rows }) {
  return (
    <div className='space-y-4'>
      {rows.map((row) => {
        const displayName = row.userId || '--';
        const subTitle = row.nickname || '';

        return (
          <article key={`${row.id}-mobile`} className={joinClasses(blockPanelClassName, 'p-4')}>
            <div className='rounded-2xl bg-slate-50 px-3 py-3 dark:bg-slate-950'>
              <div className='text-sm font-semibold text-slate-900 dark:text-white'>
                {displayName}
              </div>
            </div>
            <div className='mt-3 grid gap-3 sm:grid-cols-2'>
              {columns
                .filter((column) => column.key !== 'user')
                .map((column) => (
                  <div
                    key={column.key}
                    className='space-y-1 rounded-2xl bg-slate-50 px-3 py-3 dark:bg-slate-950'
                  >
                    <p className='text-xs font-medium text-slate-400 dark:text-slate-500'>
                      {column.title}
                    </p>
                    <div className='text-sm font-medium text-slate-700 dark:text-slate-200'>
                      {renderCell(column, row)}
                    </div>
                  </div>
                ))}
            </div>
          </article>
        );
      })}
    </div>
  );
}
function PaginationBar({ page, pageSize, total, loading, onPageChange, onPageSizeChange }) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const start = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const end = total === 0 ? 0 : Math.min(page * pageSize, total);

  return (
    <div className='mt-5 flex flex-col gap-3 border-t border-slate-100 pt-5 text-sm text-slate-500 dark:border-slate-800 dark:text-slate-400 lg:flex-row lg:items-center lg:justify-between'>
      <div>
        显示第 {start} 条 - 第 {end} 条，共 {total} 条
      </div>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-center'>
        <label className='inline-flex items-center gap-2'>
          <span>每页</span>
          <select
            className='rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200'
            onChange={(event) => onPageSizeChange(Number(event.target.value))}
            value={pageSize}
          >
            {[10, 20, 50, 100].map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </label>
        <div className='inline-flex items-center gap-2'>
          <button
            className={lightButtonClassName}
            disabled={loading || page <= 1}
            onClick={() => onPageChange(page - 1)}
            type='button'
          >
            上一页
          </button>
          <span className='min-w-[88px] text-center'>
            第 {page} / {totalPages} 页
          </span>
          <button
            className={lightButtonClassName}
            disabled={loading || page >= totalPages}
            onClick={() => onPageChange(page + 1)}
            type='button'
          >
            下一页
          </button>
        </div>
      </div>
    </div>
  );
}

function AdvancedFilterModal({ open, fields, values, onChange, onClose, onReset, onSubmit }) {
  if (!open) {
    return null;
  }

  return (
    <div
      className='fixed inset-0 z-50 flex items-center justify-center bg-slate-950/35 px-4 py-6 backdrop-blur-sm'
      onClick={onClose}
    >
      <div
        className={joinClasses(pagePanelClassName, 'w-full max-w-[560px] p-5 sm:p-6')}
        onClick={(event) => event.stopPropagation()}
      >
        <div className='flex items-center justify-between gap-4'>
          <div>
            <h3 className='text-lg font-semibold text-slate-900 dark:text-white'>高级筛选</h3>
            <p className='mt-1 text-sm text-slate-400 dark:text-slate-500'>
              按当前标签页配置的条件组合筛选数据。
            </p>
          </div>
          <button
            className='flex h-9 w-9 items-center justify-center rounded-full border border-slate-200 text-slate-500 transition hover:border-cyan-300 hover:text-slate-900 dark:border-slate-700 dark:text-slate-300 dark:hover:border-cyan-500'
            onClick={onClose}
            type='button'
          >
            <X className='h-4 w-4' />
          </button>
        </div>
        <div className='mt-6 space-y-4'>
          {fields.map((field) => (
            <div key={field.key}>
              <label className='mb-2 block text-sm font-medium text-slate-600 dark:text-slate-300'>
                {field.label}
              </label>
              <div className='grid gap-3 sm:grid-cols-2'>
                <input
                  className={inputClassName}
                  onChange={(event) => onChange(field.startKey, event.target.value)}
                  placeholder={field.startPlaceholder}
                  step={field.inputType === 'datetime-local' ? 60 : 'any'}
                  type={field.inputType}
                  value={values[field.startKey] || ''}
                />
                <input
                  className={inputClassName}
                  onChange={(event) => onChange(field.endKey, event.target.value)}
                  placeholder={field.endPlaceholder}
                  step={field.inputType === 'datetime-local' ? 60 : 'any'}
                  type={field.inputType}
                  value={values[field.endKey] || ''}
                />
              </div>
            </div>
          ))}
        </div>
        <div className='mt-8 flex flex-col-reverse gap-3 sm:flex-row sm:justify-end'>
          <button className={lightButtonClassName} onClick={onReset} type='button'>
            重置
          </button>
          <button
            className='rounded-2xl px-5 py-3 text-sm font-semibold text-slate-950 shadow-[0_8px_20px_rgba(9,254,247,0.2)]'
            onClick={onSubmit}
            style={gradientButtonStyle}
            type='button'
          >
            应用筛选
          </button>
        </div>
      </div>
    </div>
  );
}

export default function Operational() {
  const [activeTab, setActiveTab] = useState('user');
  const [activeRange, setActiveRange] = useState('day');
  const [dashboardLoading, setDashboardLoading] = useState(false);
  const [tableLoading, setTableLoading] = useState(false);
  const [dashboardPayload, setDashboardPayload] = useState({});
  const [rows, setRows] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [searchInput, setSearchInput] = useState('');
  const [keyword, setKeyword] = useState('');
  const [sortState, setSortState] = useState({ key: '', order: '' });
  const [showColumnMenu, setShowColumnMenu] = useState(false);
  const [showAdvancedFilter, setShowAdvancedFilter] = useState(false);

  const activeConfig = TAB_CONFIG[activeTab];
  const [appliedFilters, setAppliedFilters] = useState(() =>
    createInitialFilters(activeConfig.advancedFilters),
  );
  const [draftFilters, setDraftFilters] = useState(() =>
    createInitialFilters(activeConfig.advancedFilters),
  );
  const [visibleColumnKeys, setVisibleColumnKeys] = useState(
    getDefaultVisibleColumns(activeConfig.columns),
  );

  const columnMenuRef = useRef(null);
  const hasDashboardApi = hasValue(activeConfig.api?.dashboard);
  const hasRecordsApi = hasValue(activeConfig.api?.records);
  const dashboardCards = useMemo(
    () => buildDashboardCards(activeConfig.cards, dashboardPayload || {}),
    [activeConfig.cards, dashboardPayload],
  );
  const visibleColumns = useMemo(
    () => activeConfig.columns.filter((column) => visibleColumnKeys.includes(column.key)),
    [activeConfig.columns, visibleColumnKeys],
  );
  const appliedFilterCount = useMemo(
    () => countAppliedFilters(appliedFilters),
    [appliedFilters],
  );

  useEffect(() => {
    setSearchInput('');
    setKeyword('');
    setPage(1);
    setPageSize(10);
    setSortState({ key: '', order: '' });
    setShowColumnMenu(false);
    setShowAdvancedFilter(false);
    setVisibleColumnKeys(getDefaultVisibleColumns(activeConfig.columns));
    setAppliedFilters(createInitialFilters(activeConfig.advancedFilters));
    setDraftFilters(createInitialFilters(activeConfig.advancedFilters));
  }, [activeConfig]);

  useEffect(() => {
    const handlePointerDown = (event) => {
      if (
        columnMenuRef.current &&
        !columnMenuRef.current.contains(event.target)
      ) {
        setShowColumnMenu(false);
      }
    };

    const handleEscape = (event) => {
      if (event.key === 'Escape') {
        setShowColumnMenu(false);
        setShowAdvancedFilter(false);
      }
    };

    document.addEventListener('mousedown', handlePointerDown);
    document.addEventListener('keydown', handleEscape);

    return () => {
      document.removeEventListener('mousedown', handlePointerDown);
      document.removeEventListener('keydown', handleEscape);
    };
  }, []);

  useEffect(() => {
    const loadDashboard = async () => {
      if (!hasValue(activeConfig.api?.dashboard)) {
        setDashboardPayload({});
        return;
      }

      setDashboardLoading(true);
      try {
        const res = await API.get(activeConfig.api.dashboard, {
          params: { period: activeRange },
          disableDuplicate: true,
        });
        const { success, message, data } = extractResponsePayload(res?.data);
        if (!success) {
          showError(message || '获取看板数据失败');
          setDashboardPayload({});
          return;
        }
        setDashboardPayload(data || {});
      } catch (error) {
        showError(error?.message || '获取看板数据失败');
        setDashboardPayload({});
      } finally {
        setDashboardLoading(false);
      }
    };

    loadDashboard();
  }, [activeConfig.api?.dashboard, activeRange]);

  useEffect(() => {
    const loadRecords = async () => {
      if (!hasValue(activeConfig.api?.records)) {
        setRows([]);
        setTotal(0);
        return;
      }

      setTableLoading(true);
      try {
        const params = buildRecordsParams(
          page,
          pageSize,
          keyword,
          sortState,
          appliedFilters,
          activeConfig.advancedFilters,
        );
        const res = await API.get(activeConfig.api.records, {
          params,
          disableDuplicate: true,
        });
        const { success, message, data } = extractResponsePayload(res?.data);
        if (!success) {
          showError(message || '获取列表数据失败');
          setRows([]);
          setTotal(0);
          return;
        }

        const payload = extractListPayload(data);
        setRows(payload.list.map((item, index) => normalizeRow(activeTab, item, index)));
        setTotal(payload.total || 0);
        if (payload.pageSize && payload.pageSize !== pageSize) {
          setPageSize(payload.pageSize);
        }
      } catch (error) {
        showError(error?.message || '获取列表数据失败');
        setRows([]);
        setTotal(0);
      } finally {
        setTableLoading(false);
      }
    };

    loadRecords();
  }, [activeConfig.api?.records, activeConfig.advancedFilters, activeTab, appliedFilters, keyword, page, pageSize, sortState]);

  useEffect(() => {
    const totalPages = Math.max(1, Math.ceil(total / pageSize));
    if (page > totalPages) {
      setPage(totalPages);
    }
  }, [page, pageSize, total]);

  const handleToggleColumn = (columnKey) => {
    setVisibleColumnKeys((previous) => {
      if (previous.includes(columnKey)) {
        return previous.filter((item) => item !== columnKey);
      }

      const nextKeys = [...previous, columnKey];
      return activeConfig.columns
        .map((column) => column.key)
        .filter((key) => nextKeys.includes(key));
    });
  };

  const handleSortChange = (sortField) => {
    setPage(1);
    setSortState((previous) => {
      if (previous.key !== sortField) {
        return { key: sortField, order: 'desc' };
      }
      if (previous.order === 'desc') {
        return { key: sortField, order: 'asc' };
      }
      if (previous.order === 'asc') {
        return { key: '', order: '' };
      }
      return { key: sortField, order: 'desc' };
    });
  };

  const handleSearchSubmit = () => {
    setPage(1);
    setKeyword(searchInput.trim());
  };

  const handleFilterDraftChange = (field, value) => {
    setDraftFilters((previous) => ({
      ...previous,
      [field]: value,
    }));
  };

  const handleResetDraftFilters = () => {
    setDraftFilters(createInitialFilters(activeConfig.advancedFilters));
  };

  const handleOpenAdvancedFilter = () => {
    setDraftFilters(appliedFilters);
    setShowAdvancedFilter(true);
  };

  const handleApplyFilters = () => {
    setPage(1);
    setAppliedFilters(draftFilters);
    setShowAdvancedFilter(false);
  };

  const handleResetAppliedFilters = () => {
    const nextFilters = createInitialFilters(activeConfig.advancedFilters);
    setDraftFilters(nextFilters);
    setAppliedFilters(nextFilters);
    setPage(1);
  };

  return (
    <div className='min-h-screen bg-[#f4f7fb] px-3 py-4 dark:bg-slate-950 sm:px-5 lg:px-6'>
      <div className='mx-auto flex w-full max-w-[1520px] flex-col gap-6'>
        <section className='px-1 py-2 sm:px-0'>
          <div>
            <h1 className='text-3xl font-semibold tracking-tight text-slate-900 dark:text-white sm:text-4xl'>
              运营数据
            </h1>
            <p className='mt-3 max-w-3xl text-sm leading-6 text-slate-400 dark:text-slate-500 sm:text-base'>
              监控系统全局运营数据，用户、代理商、入驻商家、平台自营等维度统一展示。
            </p>
          </div>
          <div className='mt-6 flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between'>
            <div className='inline-flex w-full flex-wrap items-center gap-1 rounded-2xl bg-white p-1 shadow-[0_6px_20px_rgba(148,163,184,0.12)] dark:bg-slate-900 xl:w-auto'>
              {Object.entries(TAB_CONFIG).map(([key, config]) => {
                const active = key === activeTab;
                const disabled = key !== 'user';
                return (
                  <button
                    key={key}
                    className={joinClasses(
                      'rounded-xl px-5 py-2 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-45',
                      active
                        ? 'bg-[#f8fafc] text-slate-900 shadow-[0_2px_10px_rgba(148,163,184,0.12)] dark:bg-slate-800 dark:text-white'
                        : 'text-slate-400 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200',
                      disabled ? 'pointer-events-none' : '',
                    )}
                    disabled={disabled}
                    onClick={() => setActiveTab(key)}
                    type='button'
                  >
                    {config.label}
                  </button>
                );
              })}            </div>
            <DateRangeDropdown activeRange={activeRange} onChange={setActiveRange} />
          </div>
        </section>

        <section>
          <div className='mb-4 flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between'>
            <div>
              <h2 className='text-[22px] font-semibold tracking-tight text-slate-900 dark:text-white'>
                {activeConfig.title}
              </h2>
              <p className='mt-1 text-sm text-slate-500 dark:text-slate-400'>
                {activeConfig.subtitle}
              </p>
            </div>
          </div>
          <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4'>
            {dashboardCards.map((metric) => (
              <MetricCard key={metric.key} loading={dashboardLoading && hasDashboardApi} metric={metric} />
            ))}
          </div>
        </section>

        <section className={joinClasses(pagePanelClassName, 'px-4 py-5 sm:px-6 sm:py-6')}>
          <div className='flex flex-col gap-4 border-b border-slate-100 pb-5 dark:border-slate-800'>
            <div className='flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between'>
              <div>
                <h3 className='text-[20px] font-semibold text-slate-900 dark:text-white'>
                  {activeConfig.tableTitle}
                </h3>
                <p className='mt-1 text-sm text-slate-500 dark:text-slate-400'>
                  {hasRecordsApi
                    ? `共 ${total} 条数据，支持搜索、分页、排序和高级筛选。${appliedFilterCount > 0 ? ` 当前已启用 ${appliedFilterCount} 个筛选条件。` : ''}`
                    : '该标签页表格接口暂未接入。'}
                </p>
              </div>
              <div className='flex flex-col gap-3 lg:flex-row lg:items-center'>
                <label className='relative block min-w-[220px] lg:min-w-[320px]'>
                  <Search className='pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400' />
                  <input
                    className={joinClasses(inputClassName, 'pl-11 pr-24')}
                    disabled={!hasRecordsApi}
                    onChange={(event) => setSearchInput(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') {
                        handleSearchSubmit();
                      }
                    }}
                    placeholder={activeConfig.searchPlaceholder}
                    type='text'
                    value={searchInput}
                  />
                  <button
                    className='absolute right-2 top-1/2 -translate-y-1/2 rounded-xl px-3 py-2 text-xs font-semibold text-slate-950 disabled:opacity-50'
                    disabled={!hasRecordsApi}
                    onClick={handleSearchSubmit}
                    style={gradientButtonStyle}
                    type='button'
                  >
                    搜索
                  </button>
                </label>
                {activeConfig.columns.length > 0 ? (
                  <div className='relative' ref={columnMenuRef}>
                    <button
                      className={joinClasses(lightButtonClassName, 'w-full lg:w-auto')}
                      disabled={!hasRecordsApi}
                      onClick={() => setShowColumnMenu((previous) => !previous)}
                      type='button'
                    >
                      <ChevronDown className='h-4 w-4' />
                      列设置
                    </button>
                    {showColumnMenu ? (
                      <ColumnMenu
                        columns={activeConfig.columns}
                        onToggle={handleToggleColumn}
                        visibleColumnKeys={visibleColumnKeys}
                      />
                    ) : null}
                  </div>
                ) : null}
                {activeConfig.advancedFilters.length > 0 ? (
                  <button
                    className={joinClasses(lightButtonClassName, 'w-full lg:w-auto')}
                    disabled={!hasRecordsApi}
                    onClick={handleOpenAdvancedFilter}
                    type='button'
                  >
                    <SlidersHorizontal className='h-4 w-4' />
                    高级筛选
                  </button>
                ) : null}
                {appliedFilterCount > 0 ? (
                  <button
                    className={joinClasses(lightButtonClassName, 'w-full lg:w-auto')}
                    onClick={handleResetAppliedFilters}
                    type='button'
                  >
                    清空筛选
                  </button>
                ) : null}
              </div>
            </div>
          </div>

          <div className='mt-4'>
            {!hasRecordsApi ? (
              <div className='flex min-h-[220px] items-center justify-center rounded-2xl bg-slate-50 text-sm text-slate-500 dark:bg-slate-950 dark:text-slate-400'>
                当前标签页表格接口暂未接入。
              </div>
            ) : visibleColumns.length === 0 ? (
              <div className='flex min-h-[220px] items-center justify-center rounded-2xl bg-slate-50 text-sm text-slate-500 dark:bg-slate-950 dark:text-slate-400'>
                当前未选择任何表格列
              </div>
            ) : tableLoading ? (
              <div className='flex min-h-[220px] items-center justify-center rounded-2xl bg-slate-50 dark:bg-slate-950'>
                <LoadingIcon className='h-7 w-7' />
              </div>
            ) : rows.length === 0 ? (
              <div className='flex min-h-[220px] items-center justify-center rounded-2xl bg-slate-50 text-sm text-slate-500 dark:bg-slate-950 dark:text-slate-400'>
                暂无匹配数据
              </div>
            ) : (
              <>
                <div className='hidden xl:block'>
                  <DesktopTable
                    columns={visibleColumns}
                    onSortChange={handleSortChange}
                    rows={rows}
                    sortState={sortState}
                  />
                </div>
                <div className='xl:hidden'>
                  <MobileCards columns={visibleColumns} rows={rows} />
                </div>
              </>
            )}

            {hasRecordsApi ? (
              <PaginationBar
                loading={tableLoading}
                onPageChange={setPage}
                onPageSizeChange={(value) => {
                  setPage(1);
                  setPageSize(value);
                }}
                page={page}
                pageSize={pageSize}
                total={total}
              />
            ) : null}
          </div>
        </section>
      </div>

      <AdvancedFilterModal
        fields={activeConfig.advancedFilters}
        onChange={handleFilterDraftChange}
        onClose={() => setShowAdvancedFilter(false)}
        onReset={handleResetDraftFilters}
        onSubmit={handleApplyFilters}
        open={showAdvancedFilter}
        values={draftFilters}
      />
    </div>
  );
}





















