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
import { Dropdown, Modal, Pagination } from '@douyinfe/semi-ui';
import { IconLoading } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import {
  Activity,
  ArrowUpDown,
  BarChart3,
  Building2,
  CalendarDays,
  Check,
  ChevronDown,
  Coins,
  Gift,
  Search,
  SlidersHorizontal,
  Users,
  WalletCards,
  X,
} from 'lucide-react';
import { API, showError, isProviderOwner, getProviderId } from '../../helpers';
import {
  DATE_RANGE_OPTIONS,
  OPERATIONAL_PERIOD_COPY,
  OPERATIONAL_PREVIEW_CARDS,
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

const PROVIDER_PAGE_SIZE = 10;

function LoadingIcon({ className = 'h-5 w-5' }) {
  return <IconLoading className={joinClasses(className, 'animate-spin text-cyan-500')} />;
}

function normalizeRow(recordType, item, index) {
  if (recordType === 'user') {
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

function DateRangeDropdown ({ activeRange, onChange }) {
  const { t } = useTranslation();
  const activeLabel =
    DATE_RANGE_OPTIONS.find((option) => option.value === activeRange)?.label || t('今日');

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
                  <span>{t(option.label)}</span>
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
        <div className='flex justify-between leading-none items-center'>
          <span className='text-[12px] font-medium leading-none mr-4 text-slate-400 dark:text-slate-500'>{t('日期选择')}</span>
          <span className='text-[15px] font-semibold p-2 leading-none text-slate-700 dark:text-slate-100'>{t(activeLabel)}</span>
        </div>
        <ChevronDown className='h-4 w-4 shrink-0 text-slate-400' />
      </button>
    </Dropdown>
  );
}
function formatMetricValue(metric, displaySymbol) {
  if (metric.valueType === 'quota') {
    return formatQuotaValue(metric.value, displaySymbol);
  }
  if (metric.valueType === 'count') {
    return formatInteger(metric.value);
  }
  return hasValue(metric.value) ? String(metric.value) : '--';
}

function MetricCard({ metric, loading, displaySymbol }) {
  const { t } = useTranslation();
  const Icon = ICON_MAP[metric.icon] || BarChart3;
  const footerClassName =
    metric.footer?.className ||
    FOOTER_TONE_CLASS[metric.footer?.tone] ||
    FOOTER_TONE_CLASS.neutral;

  return (
    <article className={joinClasses(blockPanelClassName, 'p-5 sm:p-6')}>
      <div className='flex items-start justify-between gap-4'>
        <div className='min-w-0'>
          <p className='text-sm text-slate-500 dark:text-slate-400'>{t(metric.title)}</p>
          <div className='mt-4 min-h-[44px] text-[24px] font-semibold tracking-tight text-slate-900 dark:text-white sm:text-[28px]'>
            {loading ? (
              <LoadingIcon className='h-6 w-6' />
            ) : (
              formatMetricValue(metric, displaySymbol)
            )}
          </div>
        </div>
        <div className='flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-[#f8fafc] text-slate-400 dark:bg-slate-800 dark:text-slate-300'>
          <Icon className='h-5 w-5' />
        </div>
      </div>
      <div className={joinClasses('mt-5 min-h-[22px] text-sm', footerClassName)}>
        {metric.footer?.text ? <span>{t(metric.footer.text)}</span> : null}
        {metric.footer?.trend ? (
          <span
            className={joinClasses(
              'font-semibold',
              metric.footer?.text ? 'ml-2' : '',
            )}
          >
            {t(metric.footer.trend)}
          </span>
        ) : null}
      </div>
    </article>
  );
}

function ProviderSelectorModal({
  loading,
  onClose,
  onPageChange,
  onSelect,
  open,
  page,
  providers,
  selectedProviderId,
  total,
}) {
  const { t } = useTranslation();

  return (
    <Modal
      centered
      footer={null}
      onCancel={onClose}
      title={t('选择服务商')}
      visible={open}
      width={520}
    >
      <div className='min-h-[260px] pb-4'>
        {loading ? (
          <div className='flex min-h-[260px] items-center justify-center'>
            <LoadingIcon className='h-7 w-7' />
          </div>
        ) : providers.length === 0 ? (
          <div className='flex min-h-[260px] items-center justify-center text-sm text-slate-500 dark:text-slate-400'>
            {t('暂无服务商')}
          </div>
        ) : (
          <div className='overflow-hidden rounded-2xl border border-slate-200 dark:border-slate-700'>
            {providers.map((provider) => {
              const providerId = Number(provider.provider_id);
              const selected = providerId === selectedProviderId;

              return (
                <button
                  key={provider.id || providerId}
                  className={joinClasses(
                    'flex w-full items-center gap-3 border-b border-slate-100 px-4 py-3 text-left transition last:border-b-0 dark:border-slate-800',
                    selected
                      ? 'bg-cyan-50 text-slate-900 dark:bg-cyan-950/30 dark:text-white'
                      : 'hover:bg-slate-50 dark:hover:bg-slate-800/70',
                  )}
                  onClick={() => onSelect(provider)}
                  type='button'
                >
                  <span className='flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-300'>
                    <Building2 className='h-4 w-4' />
                  </span>
                  <span className='min-w-0 flex-1'>
                    <span className='block truncate text-sm font-semibold'>
                      {provider.site_name || `#${providerId}`}
                    </span>
                    <span className='mt-1 block text-xs text-slate-400 dark:text-slate-500'>
                      ID: {providerId}
                    </span>
                  </span>
                  {selected ? <Check className='h-4 w-4 shrink-0 text-cyan-500' /> : null}
                </button>
              );
            })}
          </div>
        )}

        {total > PROVIDER_PAGE_SIZE ? (
          <div className='mt-4 flex justify-end'>
            <Pagination
              currentPage={page}
              hideOnSinglePage
              onPageChange={onPageChange}
              pageSize={PROVIDER_PAGE_SIZE}
              total={total}
            />
          </div>
        ) : null}
      </div>
    </Modal>
  );
}

function ColumnMenu({ columns, visibleColumnKeys, onToggle }) {
  const { t } = useTranslation();

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
                    ? 'border-transparent theme-btn-color'
                    : 'border-slate-300 text-transparent dark:border-slate-600',
                )}
                style={checked ? gradientButtonStyle : undefined}
              >
                ✓
              </span>
              <span>{t(column.label)}</span>
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

function renderCell (column, row, displaySymbol, t) {
  const displayName = row.userId || '--';
  const subTitle = row.nickname || '';

  switch (column.key) {
    case 'user':
      return (
        <span className='whitespace-nowrap text-sm font-semibold text-slate-800 dark:text-slate-100'>
          {displayName}
        </span>
      );
    case 'source':
      return t(getRegistrationSourceLabel(row.invited));
    case 'retention': {
      const retentionMeta = getRetentionMeta(row.retention);
      return (
        <span
          className={joinClasses(
            'inline-flex rounded-full px-2.5 py-1 text-xs font-semibold',
            retentionMeta.className,
          )}
        >
          {t(retentionMeta.label)}
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
      return formatQuotaValue(row.quota, displaySymbol);
    case 'requestCount':
      return formatInteger(row.requestCount);
    case 'topupQuota':
      return formatQuotaValue(row.topupQuota, displaySymbol);
    case 'welfareQuota':
      return formatQuotaValue(row.welfareQuota, displaySymbol);
    case 'usedQuota':
      return formatQuotaValue(row.usedQuota, displaySymbol);
    case 'registerAt':
      return formatDateTime(row.registerAt);
    default:
      return hasValue(row[column.key]) ? String(row[column.key]) : '--';
  }
}
function DesktopTable({ columns, rows, sortState, displaySymbol, onSortChange }) {
  const { t } = useTranslation();
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
                    <span>{t(column.title)}</span>
                    <ArrowUpDown className='h-3.5 w-3.5' />
                    {getSortLabel(sortState, column.sortField) ? (
                      <span className='text-[11px]'>
                        {t(getSortLabel(sortState, column.sortField))}
                      </span>
                    ) : null}
                  </button>
                ) : (
                  <span>{t(column.title)}</span>
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
                    {renderCell(column, row, displaySymbol, t)}
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

function MobileCards({ columns, rows, displaySymbol }) {
  const { t } = useTranslation();
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
                      {t(column.title)}
                    </p>
                    <div className='text-sm font-medium text-slate-700 dark:text-slate-200'>
                      {renderCell(column, row, displaySymbol, t)}
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
function AdvancedFilterModal ({ open, fields, values, onChange, onClose, onReset, onSubmit }) {
  const { t } = useTranslation();
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
            <h3 className='text-lg font-semibold text-slate-900 dark:text-white'>{t('高级筛选')}</h3>
            <p className='mt-1 text-sm text-slate-400 dark:text-slate-500'>
              {t('按当前标签页配置的条件组合筛选数据。')}
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
                {t(field.label)}
              </label>
              <div className='grid gap-3 sm:grid-cols-2'>
                <input
                  className={inputClassName}
                  onChange={(event) => onChange(field.startKey, event.target.value)}
                  placeholder={t(field.startPlaceholder)}
                  step={field.inputType === 'datetime-local' ? 60 : 'any'}
                  type={field.inputType}
                  value={values[field.startKey] || ''}
                />
                <input
                  className={inputClassName}
                  onChange={(event) => onChange(field.endKey, event.target.value)}
                  placeholder={t(field.endPlaceholder)}
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
            {t('重置')}
          </button>
          <button
            className='rounded-2xl px-5 py-3 text-sm font-semibold theme-btn-color'
            onClick={onSubmit}
            style={gradientButtonStyle}
            type='button'
          >
            {t('应用筛选')}
          </button>
        </div>
      </div>
    </div>
  );
}

export default function Operational () {
  const { t } = useTranslation();
  const isProvider = isProviderOwner();
  const providerId = getProviderId();
  const [activeTab, setActiveTab] = useState('selfHosted');
  const [activeRange, setActiveRange] = useState('day');
  const [dashboardLoading, setDashboardLoading] = useState(false);
  const [tableLoading, setTableLoading] = useState(false);
  const [dashboardPayload, setDashboardPayload] = useState({});
  const [rows, setRows] = useState([]);
  const [total, setTotal] = useState(0);
  const [displaySymbol, setDisplaySymbol] = useState('');
  const [page, setPage] = useState(1);
  const pageSize = 10;
  const [searchInput, setSearchInput] = useState('');
  const [keyword, setKeyword] = useState('');
  const [sortState, setSortState] = useState({ key: '', order: '' });
  const [showColumnMenu, setShowColumnMenu] = useState(false);
  const [showAdvancedFilter, setShowAdvancedFilter] = useState(false);
  const [showProviderSelector, setShowProviderSelector] = useState(false);
  const [selectedProvider, setSelectedProvider] = useState(null);
  const [providers, setProviders] = useState([]);
  const [providersLoading, setProvidersLoading] = useState(false);
  const [providerPage, setProviderPage] = useState(1);
  const [providerTotal, setProviderTotal] = useState(0);

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
  const selectedProviderId = Number(selectedProvider?.provider_id) || null;
  const providerSelectionPending = activeTab === 'agent' && selectedProviderId === null;
  const effectiveProviderId = activeTab === 'agent' ? selectedProviderId : providerId;
  const hasDashboardApi =
    !providerSelectionPending && hasValue(activeConfig.api?.dashboard);
  const hasRecordsApi =
    !providerSelectionPending && hasValue(activeConfig.api?.records);
  const dashboardCards = useMemo(
    () => buildDashboardCards(activeConfig.cards, dashboardPayload || {}),
    [activeConfig.cards, dashboardPayload],
  );
  const dashboardDisplaySymbol = dashboardPayload?.display_symbol || '';
  const previewCards = useMemo(() => {
    const periodCopy = OPERATIONAL_PERIOD_COPY[activeRange];
    const cardsConfig = OPERATIONAL_PREVIEW_CARDS.map((card) => {
      if (card.key === 'totalUsers') {
        return card;
      }

      return {
        ...card,
        title:
          card.key === 'periodNewUsers'
            ? periodCopy.newUsersTitle
            : periodCopy.depositTitle,
        footer: {
          ...card.footer,
          text: periodCopy.comparisonText,
        },
        footerText: periodCopy.comparisonText,
      };
    });

    return buildDashboardCards(cardsConfig, dashboardPayload || {});
  }, [activeRange, dashboardPayload]);
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
    if (!showProviderSelector || isProvider) {
      return undefined;
    }

    let active = true;

    const loadProviders = async () => {
      setProvidersLoading(true);
      try {
        const res = await API.get('/api/operation/providers', {
          params: { p: providerPage, page_size: PROVIDER_PAGE_SIZE },
        });
        const { success, message, data } = extractResponsePayload(res?.data);
        if (!active) {
          return;
        }
        if (!success) {
          showError(message || t('获取服务商列表失败'));
          setProviders([]);
          setProviderTotal(0);
          return;
        }

        const payload = extractListPayload(data);
        setProviders(
          payload.list.filter((provider) => Number(provider.provider_id) > 0),
        );
        setProviderTotal(payload.total || 0);
      } catch (error) {
        if (active) {
          showError(error?.message || t('获取服务商列表失败'));
          setProviders([]);
          setProviderTotal(0);
        }
      } finally {
        if (active) {
          setProvidersLoading(false);
        }
      }
    };

    loadProviders();

    return () => {
      active = false;
    };
  }, [isProvider, providerPage, showProviderSelector, t]);

  useEffect(() => {
    const loadDashboard = async () => {
      if (!hasDashboardApi) {
        setDashboardPayload({});
        setDashboardLoading(false);
        return;
      }

      setDashboardLoading(true);
      try {
        const res = await API.get(activeConfig.api.dashboard, {
          params: { period: activeRange, provider_id: effectiveProviderId },
        });
        const { success, message, data } = extractResponsePayload(res?.data);
        if (!success) {
          showError(message || t('获取看板数据失败'));
          setDashboardPayload({});
          return;
        }
        setDashboardPayload(data || {});
      } catch (error) {
        showError(error?.message || t('获取看板数据失败'));
        setDashboardPayload({});
      } finally {
        setDashboardLoading(false);
      }
    };

    loadDashboard();
  }, [activeConfig.api?.dashboard, activeRange, effectiveProviderId, hasDashboardApi, t]);

  useEffect(() => {
    const loadRecords = async () => {
      if (!hasRecordsApi) {
        setRows([]);
        setTotal(0);
        setDisplaySymbol('');
        setTableLoading(false);
        return;
      }

      setTableLoading(true);
      try {
        const params = {
          ...buildRecordsParams(
            page,
            pageSize,
            keyword,
            sortState,
            appliedFilters,
            activeConfig.advancedFilters,
          ),
          provider_id: effectiveProviderId,
        };
        const res = await API.get(activeConfig.api.records, {
          params,
        });
        const { success, message, data } = extractResponsePayload(res?.data);
        if (!success) {
          showError(message || t('获取列表数据失败'));
          setRows([]);
          setTotal(0);
          return;
        }

        const payload = extractListPayload(data);
        setDisplaySymbol(data?.display_symbol || '');
        setRows(
          payload.list.map((item, index) =>
            normalizeRow(activeConfig.recordType, item, index),
          ),
        );
        setTotal(payload.total || 0);
      } catch (error) {
        showError(error?.message || t('获取列表数据失败'));
        setRows([]);
        setTotal(0);
      } finally {
        setTableLoading(false);
      }
    };

    loadRecords();
  }, [activeConfig.api?.records, activeConfig.advancedFilters, activeConfig.recordType, activeTab, appliedFilters, effectiveProviderId, hasRecordsApi, keyword, page, pageSize, sortState, t]);

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

  const handleTabChange = (tabKey) => {
    setActiveTab(tabKey);
    if (tabKey === 'agent') {
      setProviderPage(1);
      setShowProviderSelector(true);
    }
  };

  const handleProviderSelect = (provider) => {
    setSelectedProvider(provider);
    setShowProviderSelector(false);
    setPage(1);
  };

  return (
    <div className='min-h-scree'>
      <div className='mx-auto flex w-full max-w-[1520px] flex-col gap-6'>
        <section className='px-1 py-2 sm:px-0'>
          <div>
            <h1 className='text-3xl font-semibold tracking-tight text-slate-900 dark:text-white sm:text-4xl'>
              {t('运营数据')}
            </h1>
            <p className='mt-3 max-w-3xl text-sm leading-6 text-slate-400 dark:text-slate-500 sm:text-base'>
              {t('监控系统全局运营数据。')}
            </p>
          </div>
          <div
            className={joinClasses(
              'mt-6 flex flex-col gap-4 xl:flex-row xl:items-center',
              isProvider ? 'xl:justify-end' : 'xl:justify-between',
            )}
          >
            {!isProvider && (
              <div className='inline-flex w-full flex-wrap items-center gap-1 rounded-2xl bg-white p-1 shadow-[0_6px_20px_rgba(148,163,184,0.12)] dark:bg-slate-900 xl:w-auto'>
                {Object.entries(TAB_CONFIG)
                  .filter(([key]) => key === 'agent' || key === 'selfHosted')
                  .map(([key, config]) => {
                    const active = key === activeTab;
                    return (
                      <button
                        key={key}
                        className={joinClasses(
                          'rounded-xl px-5 py-2 text-sm font-medium transition',
                          active
                            ? 'bg-[#f8fafc] text-slate-900 shadow-[0_2px_10px_rgba(148,163,184,0.12)] dark:bg-slate-800 dark:text-white'
                            : 'text-slate-400 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200',
                        )}
                        onClick={() => handleTabChange(key)}
                        type='button'
                      >
                        {t(config.label)}
                      </button>
                    );
                  })}
              </div>
            )}
            <div className='flex w-full flex-col gap-3 sm:flex-row sm:items-center sm:justify-end xl:w-auto'>
              {activeTab === 'agent' ? (
                <button
                  className='inline-flex h-[50px] w-full items-center justify-between rounded-[20px] border border-slate-200/90 bg-white px-4 text-left shadow-[0_8px_20px_rgba(148,163,184,0.12)] transition hover:border-slate-300 hover:shadow-[0_10px_24px_rgba(148,163,184,0.16)] focus:border-cyan-300 focus:outline-none focus:ring-4 focus:ring-cyan-100 dark:border-slate-700 dark:bg-slate-900 dark:hover:border-slate-600 dark:focus:border-cyan-500 dark:focus:ring-cyan-900/30 sm:w-[260px]'
                  onClick={() => setShowProviderSelector(true)}
                  type='button'
                >
                  <span className='flex min-w-0 items-center gap-3'>
                    <Building2 className='h-4 w-4 shrink-0 text-slate-400' />
                    <span className='min-w-0'>
                      <span className='block truncate text-[15px] font-semibold leading-none text-slate-700 dark:text-slate-100'>
                        {selectedProvider?.site_name || t('选择服务商')}
                      </span>
                    </span>
                  </span>
                  <ChevronDown className='h-4 w-4 shrink-0 text-slate-400' />
                </button>
              ) : null}
              <div className='w-full sm:w-[180px] [&>button]:w-full'>
                <DateRangeDropdown activeRange={activeRange} onChange={setActiveRange} />
              </div>
            </div>
          </div>
        </section>

        <section>
          <div className='mb-4 flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between'>
            <div>
              <h2 className='text-[22px] font-semibold tracking-tight text-slate-900 dark:text-white'>
                {t(activeConfig.title)}
              </h2>
              <p className='mt-1 text-sm text-slate-500 dark:text-slate-400'>
                {t(activeConfig.subtitle)}
              </p>
            </div>
          </div>
          {(activeTab === 'selfHosted' || activeTab === 'agent') && (
            <div className='mb-4 grid grid-cols-1 gap-4 sm:grid-cols-3'>
              {previewCards.map((metric) => (
                <MetricCard
                  displaySymbol={dashboardDisplaySymbol}
                  key={metric.key}
                  loading={dashboardLoading && hasDashboardApi}
                  metric={metric}
                />
              ))}
            </div>
          )}
          <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4'>
            {dashboardCards.map((metric) => (
              <MetricCard
                displaySymbol={dashboardDisplaySymbol}
                key={metric.key}
                loading={dashboardLoading && hasDashboardApi}
                metric={metric}
              />
            ))}
          </div>
        </section>

        <section className={joinClasses(pagePanelClassName, 'px-4 py-5 sm:px-6 sm:py-6')}>
          <div className='flex flex-col gap-4 border-b border-slate-100 pb-5 dark:border-slate-800'>
            <div className='flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between'>
              <div>
                <h3 className='text-[20px] font-semibold text-slate-900 dark:text-white'>
                  {t(activeConfig.tableTitle)}
                </h3>
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
                    placeholder={t(activeConfig.searchPlaceholder)}
                    type='text'
                    value={searchInput}
                  />
                  <button
                    className='absolute right-2 top-1/2 -translate-y-1/2 rounded-xl px-3 py-2 text-xs font-semibold theme-btn-color disabled:opacity-50'
                    disabled={!hasRecordsApi}
                    onClick={handleSearchSubmit}
                    style={gradientButtonStyle}
                    type='button'
                  >
                    {t('搜索')}
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
                      {t('列设置')}
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
                    {t('高级筛选')}
                  </button>
                ) : null}
                {appliedFilterCount > 0 ? (
                  <button
                    className={joinClasses(lightButtonClassName, 'w-full lg:w-auto')}
                    onClick={handleResetAppliedFilters}
                    type='button'
                  >
                    {t('清空筛选')}
                  </button>
                ) : null}
              </div>
            </div>
          </div>

          <div className='mt-4'>
            {providerSelectionPending ? (
              <div className='flex min-h-[220px] items-center justify-center rounded-2xl bg-slate-50 dark:bg-slate-950'>
                <button
                  className={lightButtonClassName}
                  onClick={() => setShowProviderSelector(true)}
                  type='button'
                >
                  <Building2 className='h-4 w-4' />
                  {t('选择服务商')}
                </button>
              </div>
            ) : !hasRecordsApi ? (
              <div className='flex min-h-[220px] items-center justify-center rounded-2xl bg-slate-50 text-sm text-slate-500 dark:bg-slate-950 dark:text-slate-400'>
                {t('当前标签页表格接口暂未接入')}
              </div>
            ) : visibleColumns.length === 0 ? (
              <div className='flex min-h-[220px] items-center justify-center rounded-2xl bg-slate-50 text-sm text-slate-500 dark:bg-slate-950 dark:text-slate-400'>
                {t('当前未选择任何表格列')}
              </div>
            ) : tableLoading ? (
              <div className='flex min-h-[220px] items-center justify-center rounded-2xl bg-slate-50 dark:bg-slate-950'>
                <LoadingIcon className='h-7 w-7' />
              </div>
            ) : rows.length === 0 ? (
              <div className='flex min-h-[220px] items-center justify-center rounded-2xl bg-slate-50 text-sm text-slate-500 dark:bg-slate-950 dark:text-slate-400'>
                {t('暂无匹配数据')}
              </div>
            ) : (
              <>
                <div className='hidden xl:block'>
                  <DesktopTable
                    columns={visibleColumns}
                    displaySymbol={displaySymbol}
                    onSortChange={handleSortChange}
                    rows={rows}
                    sortState={sortState}
                  />
                </div>
                <div className='xl:hidden'>
                  <MobileCards columns={visibleColumns} rows={rows} displaySymbol={displaySymbol} />
                </div>
              </>
            )}

            {hasRecordsApi ? (
              <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 20 }}>
                <Pagination
                  total={total}
                  onPageChange={(p) => setPage(p)}
                />
              </div>
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
      <ProviderSelectorModal
        loading={providersLoading}
        onClose={() => setShowProviderSelector(false)}
        onPageChange={setProviderPage}
        onSelect={handleProviderSelect}
        open={showProviderSelector}
        page={providerPage}
        providers={providers}
        selectedProviderId={selectedProviderId}
        total={providerTotal}
      />
    </div>
  );
}
