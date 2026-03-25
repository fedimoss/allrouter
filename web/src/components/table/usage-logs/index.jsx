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
import { Empty, Modal } from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  copy,
  getLogOther,
  renderQuota,
  showError,
  showSuccess,
} from '../../../helpers';
import { useLogsData } from '../../../hooks/usage-logs/useUsageLogsData';
import UserInfoModal from './modals/UserInfoModal';
import ChannelAffinityUsageCacheModal from './modals/ChannelAffinityUsageCacheModal';
import ParamOverrideModal from './modals/ParamOverrideModal';
import {
  CalendarDays,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  CircleAlert,
  ClipboardList,
  Columns3,
  Copy as CopyIcon,
  DollarSign,
  Eye,
  FileText,
  RefreshCw,
  RotateCcw,
  Search,
  User,
  X,
  Zap,
} from 'lucide-react';

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];

const getPaginationItems = (currentPage, totalPages) => {
  if (totalPages <= 1) {
    return [1];
  }

  const items = [];
  const start = Math.max(1, currentPage - 1);
  const end = Math.min(totalPages, currentPage + 1);

  if (start > 1) {
    items.push(1);
  }

  if (start > 2) {
    items.push('left-ellipsis');
  }

  for (let page = start; page <= end; page += 1) {
    items.push(page);
  }

  if (end < totalPages - 1) {
    items.push('right-ellipsis');
  }

  if (end < totalPages) {
    items.push(totalPages);
  }

  return items;
};

const toDateTimeLocalValue = (value) => {
  if (!value) {
    return '';
  }
  return String(value).replace(' ', 'T');
};

const fromDateTimeLocalValue = (value) => {
  if (!value) {
    return '';
  }
  const normalized = String(value).replace('T', ' ');
  return normalized.length === 16 ? `${normalized}:00` : normalized;
};

const handleDateTimeInputClick = (event) => {
  const input = event.currentTarget;
  if (typeof input?.showPicker === 'function') {
    try {
      input.showPicker();
    } catch (_) {}
  }
};

const formatNumber = (value) => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return '0';
  }
  return parsed.toLocaleString();
};

const formatSeconds = (value, fractionDigits = 0) => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return '0s';
  }
  return `${parsed.toFixed(fractionDigits)}s`;
};

const formatFirstTokenSeconds = (value) => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return '—';
  }
  return `${(parsed / 1000).toFixed(2)}s`;
};

const formatRatioValue = (value) => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return '';
  }
  return `${parsed.toFixed(parsed >= 10 ? 1 : 2)}x`;
};

const renderQuotaValue = (value) => {
  const rendered = renderQuota(value, 6);
  return typeof rendered === 'string' ? rendered : String(rendered);
};

const getDisplayGroup = (record, t) => {
  if (record.group) {
    return record.group;
  }
  const other = getLogOther(record.other);
  if (other?.group) {
    return other.group;
  }
  return t('默认分组');
};

const getStatusMeta = (record, t) => {
  if (record.type === 5) {
    return {
      label: t('失败'),
      className: 'log-v2-status log-v2-status-error',
    };
  }

  if (record.type === 3 || record.type === 4) {
    return {
      label: t('处理中'),
      className: 'log-v2-status log-v2-status-pending',
    };
  }

  return {
    label: t('成功'),
    className: 'log-v2-status log-v2-status-success',
  };
};

const getErrorTimingLabel = (record, t) => {
  const other = getLogOther(record?.other) || {};
  const raw = String(other.reason || record?.content || '').trim().toLowerCase();

  if (raw.includes('timeout') || raw.includes('超时')) {
    return t('超时');
  }
  if (raw.includes('abort') || raw.includes('cancel') || raw.includes('中断')) {
    return t('中断');
  }
  if (raw.includes('limit') || raw.includes('频率')) {
    return t('限流');
  }
  return t('失败');
};

const getRequestTypeMeta = (record, t) => {
  const other = getLogOther(record.other) || {};
  const requestPath = String(other?.request_path || '').toLowerCase();
  const modelName = String(record.model_name || '').toLowerCase();

  if (requestPath.includes('/embeddings') || modelName.includes('embedding')) {
    return {
      label: 'Embedding',
      className: 'log-v2-chip log-v2-chip-emerald',
    };
  }

  if (requestPath.includes('/completions') && !requestPath.includes('/chat/')) {
    return {
      label: 'Completion',
      className: 'log-v2-chip log-v2-chip-violet',
    };
  }

  if (requestPath.includes('/images')) {
    return {
      label: 'Image',
      className: 'log-v2-chip log-v2-chip-orange',
    };
  }

  if (requestPath.includes('/audio')) {
    return {
      label: 'Audio',
      className: 'log-v2-chip log-v2-chip-indigo',
    };
  }

  switch (record.type) {
    case 1:
      return {
        label: t('充值'),
        className: 'log-v2-chip log-v2-chip-cyan',
      };
    case 3:
      return {
        label: t('管理'),
        className: 'log-v2-chip log-v2-chip-orange',
      };
    case 4:
      return {
        label: t('系统'),
        className: 'log-v2-chip log-v2-chip-slate',
      };
    case 5:
      return {
        label: t('错误'),
        className: 'log-v2-chip log-v2-chip-red',
      };
    case 6:
      return {
        label: t('退款'),
        className: 'log-v2-chip log-v2-chip-cyan',
      };
    default:
      return {
        label: 'Chat',
        className: 'log-v2-chip log-v2-chip-sky',
      };
  }
};

const getChannelLabel = (record, t) => {
  const other = getLogOther(record.other) || {};
  const chain = other?.admin_info?.use_channel;
  if (Array.isArray(chain) && chain.length > 0) {
    return chain.join(' -> ');
  }
  if (record.channel_name && record.channel) {
    return `${record.channel_name} (${record.channel})`;
  }
  if (record.channel_name) {
    return record.channel_name;
  }
  if (record.channel) {
    return `${t('渠道')} ${record.channel}`;
  }
  return '-';
};

const getModelDisplayMeta = (record) => {
  const other = getLogOther(record.other) || {};
  const mappedModel = String(other?.upstream_model_name || '').trim();
  const modelName = record.model_name || '-';

  if (other?.is_model_mapped && mappedModel && mappedModel !== modelName) {
    return {
      primary: modelName,
      secondary: mappedModel,
    };
  }

  return {
    primary: modelName,
    secondary: '',
  };
};

const getPromptCacheText = (record, t) => {
  const other = getLogOther(record.other) || {};
  const cacheReadTokens = Number(other.cache_tokens) || 0;
  const cacheCreationTokens = Number(other.cache_creation_tokens) || 0;
  const cacheCreationTokens5m = Number(other.cache_creation_tokens_5m) || 0;
  const cacheCreationTokens1h = Number(other.cache_creation_tokens_1h) || 0;
  const cacheWriteTokens =
    cacheCreationTokens5m + cacheCreationTokens1h || cacheCreationTokens;

  if (cacheReadTokens > 0 && cacheWriteTokens > 0) {
    return `${t('缓存读')} ${formatNumber(cacheReadTokens)} · ${t('写')} ${formatNumber(cacheWriteTokens)}`;
  }
  if (cacheReadTokens > 0) {
    return `${t('缓存读')} ${formatNumber(cacheReadTokens)}`;
  }
  if (cacheWriteTokens > 0) {
    return `${t('缓存写')} ${formatNumber(cacheWriteTokens)}`;
  }
  return '';
};

const getCostDisplayMeta = (record, billingDisplayMode, t) => {
  const other = getLogOther(record.other) || {};
  const primary = renderQuotaValue(record.quota);

  if (other?.billing_source === 'subscription') {
    return {
      primary: t('订阅抵扣'),
      secondary: primary,
      subscription: true,
    };
  }

  if (billingDisplayMode !== 'ratio') {
    return {
      primary,
      secondary: '',
      subscription: false,
    };
  }

  const ratioSegments = [];
  const modelRatio = Number(other.model_ratio);
  const completionRatio = Number(other.completion_ratio);
  const userGroupRatio = Number(other.user_group_ratio);
  const groupRatio =
    Number.isFinite(userGroupRatio) && userGroupRatio !== -1
      ? userGroupRatio
      : Number(other.group_ratio);

  if (Number.isFinite(modelRatio) && modelRatio > 0) {
    ratioSegments.push(`${t('模型')} ${formatRatioValue(modelRatio)}`);
  }
  if (
    Number.isFinite(completionRatio) &&
    completionRatio > 0 &&
    completionRatio !== 1
  ) {
    ratioSegments.push(`${t('补全')} ${formatRatioValue(completionRatio)}`);
  }
  if (Number.isFinite(groupRatio) && groupRatio > 0 && groupRatio !== 1) {
    ratioSegments.push(`${t('分组')} ${formatRatioValue(groupRatio)}`);
  }

  return {
    primary,
    secondary: ratioSegments.join(' · '),
    subscription: false,
  };
};

const getDetailText = (record, expandData, t) => {
  const content = String(record?.content || '').trim();
  if (content) {
    return content;
  }

  const other = getLogOther(record?.other) || {};
  if (typeof other?.reason === 'string' && other.reason.trim()) {
    return other.reason.trim();
  }

  const detailRows = expandData?.[record?.key] || [];
  const normalized = detailRows
    .map((item) => {
      if (!item?.key) {
        return '';
      }
      if (typeof item.value === 'string' || typeof item.value === 'number') {
        return `${item.key}: ${item.value}`;
      }
      return '';
    })
    .filter(Boolean)
    .join('\n');

  return normalized || t('暂无内容');
};

const buildCopyPayload = (record, expandData, t) => {
  const other = getLogOther(record?.other) || {};
  const lines = [
    `${t('Request ID')}: ${record?.request_id || '-'}`,
    `${t('状态')}: ${getStatusMeta(record || {}, t).label}`,
    `${t('名称')}: ${record?.token_name || '-'}`,
    `${t('用户')}: ${record?.username || '-'}`,
    `${t('模型')}: ${record?.model_name || '-'}`,
    `${t('分组')}: ${record ? getDisplayGroup(record, t) : '-'}`,
    `${t('渠道')}: ${record ? getChannelLabel(record, t) : '-'}`,
    `${t('输入')}: ${formatNumber(record?.prompt_tokens)}`,
    `${t('输出')}: ${formatNumber(record?.completion_tokens)}`,
    `${t('花费')}: ${record ? renderQuotaValue(record.quota) : '-'}`,
    `${t('总耗时')}: ${formatSeconds(record?.use_time)}`,
    `${t('首字时间')}: ${formatFirstTokenSeconds(other?.frt)}`,
    `${t('IP')}: ${record?.ip || '-'}`,
    '',
    `${t('请求内容 (Prompt)')}:`,
    getDetailText(record, expandData, t),
  ];

  return lines.join('\n');
};

const getSelectableColumns = (columnKeys, t) => [
  { key: columnKeys.TIME, label: t('时间') },
  { key: columnKeys.TOKEN, label: t('名称') },
  { key: columnKeys.GROUP, label: t('分组') },
  { key: columnKeys.TYPE, label: t('类型') },
  { key: columnKeys.MODEL, label: t('模型') },
  { key: columnKeys.USE_TIME, label: t('用时/首字') },
  { key: columnKeys.PROMPT, label: t('输入') },
  { key: columnKeys.COMPLETION, label: t('输出') },
  { key: columnKeys.COST, label: t('花费') },
  { key: columnKeys.IP, label: t('IP') },
  { key: columnKeys.DETAILS, label: t('详情') },
];

function UsageLogDetailModal({
  log,
  visible,
  onClose,
  onCopy,
  onOpenParamOverride,
  onOpenChannelAffinity,
  billingDisplayMode,
  expandData,
  isAdminUser,
  t,
}) {
  const currentLog = log || {};
  const other = getLogOther(currentLog.other) || {};
  const statusMeta = log
    ? getStatusMeta(currentLog, t)
    : { label: '', className: 'log-v2-status log-v2-status-pending' };
  const detailText = getDetailText(currentLog, expandData, t);
  const requestBodyLabel =
    currentLog.type === 5 ? t('错误内容') : t('请求内容 (Prompt)');
  const costMeta = log
    ? getCostDisplayMeta(currentLog, billingDisplayMode, t)
    : { primary: '-', secondary: '', subscription: false };
  const paramOverrideCount = Array.isArray(other?.po)
    ? other.po.filter(Boolean).length
    : 0;
  const hasChannelAffinity =
    isAdminUser && !!other?.admin_info?.channel_affinity;

  return (
    <Modal
      centered
      visible={visible}
      onCancel={onClose}
      footer={null}
      width='min(560px, calc(100vw - 24px))'
      closable={false}
      closeIcon={null}
      maskClosable
      className='log-v2-detail-modal'
    >
      <div className='log-v2-detail-card'>
        <div className='log-v2-detail-header'>
          <h3 className='log-v2-detail-title'>
            <FileText size={18} />
            {t('请求详情')}
          </h3>
          <button
            type='button'
            className='log-v2-icon-button'
            onClick={onClose}
            aria-label={t('关闭')}
          >
            <X size={18} />
          </button>
        </div>

        <div className='log-v2-detail-grid'>
          <div className='log-v2-detail-item'>
            <span className='log-v2-detail-label'>{t('Request ID')}</span>
            <span className='log-v2-detail-mono'>
              {currentLog.request_id || '-'}
            </span>
          </div>
          <div className='log-v2-detail-item'>
            <span className='log-v2-detail-label'>{t('状态')}</span>
            {log ? (
              <span className={statusMeta.className}>{statusMeta.label}</span>
            ) : (
              <span className='log-v2-detail-value'>-</span>
            )}
          </div>
          <div className='log-v2-detail-item'>
            <span className='log-v2-detail-label'>{t('模型')}</span>
            <span className='log-v2-detail-value'>
              {currentLog.model_name || '-'}
            </span>
          </div>
          <div className='log-v2-detail-item'>
            <span className='log-v2-detail-label'>{t('渠道')}</span>
            <span className='log-v2-detail-value'>
              {log ? getChannelLabel(currentLog, t) : '-'}
            </span>
          </div>
          <div className='log-v2-detail-item'>
            <span className='log-v2-detail-label'>{t('输入 Tokens')}</span>
            <span className='log-v2-detail-mono'>
              {formatNumber(currentLog.prompt_tokens)}
            </span>
          </div>
          <div className='log-v2-detail-item'>
            <span className='log-v2-detail-label'>{t('输出 Tokens')}</span>
            <span className='log-v2-detail-mono'>
              {formatNumber(currentLog.completion_tokens)}
            </span>
          </div>
          <div className='log-v2-detail-item'>
            <span className='log-v2-detail-label'>{t('总耗时')}</span>
            <span className='log-v2-detail-value'>
              {formatSeconds(currentLog.use_time, 2)}
            </span>
          </div>
          <div className='log-v2-detail-item'>
            <span className='log-v2-detail-label'>{t('首字时间')}</span>
            <span className='log-v2-detail-value log-v2-detail-highlight'>
              {formatFirstTokenSeconds(other?.frt)}
            </span>
          </div>
          <div className='log-v2-detail-item'>
            <span className='log-v2-detail-label'>{t('花费')}</span>
            <div className='log-v2-detail-cost-stack'>
              <span className='log-v2-detail-cost'>{costMeta.primary}</span>
              {costMeta.secondary ? (
                <span className='log-v2-detail-subvalue'>{costMeta.secondary}</span>
              ) : null}
            </div>
          </div>
          <div className='log-v2-detail-item'>
            <span className='log-v2-detail-label'>{t('IP 地址')}</span>
            <span className='log-v2-detail-mono'>{currentLog.ip || '-'}</span>
          </div>
          <div className='log-v2-detail-item'>
            <span className='log-v2-detail-label'>{t('名称')}</span>
            <span className='log-v2-detail-value'>
              {currentLog.token_name || '-'}
            </span>
          </div>
          <div className='log-v2-detail-item'>
            <span className='log-v2-detail-label'>{t('分组')}</span>
            <span className='log-v2-detail-value'>
              {log ? getDisplayGroup(currentLog, t) : '-'}
            </span>
          </div>
          {isAdminUser && (
            <>
              <div className='log-v2-detail-item'>
                <span className='log-v2-detail-label'>{t('用户')}</span>
                <span className='log-v2-detail-value'>
                  {currentLog.username || '-'}
                </span>
              </div>
              <div className='log-v2-detail-item'>
                <span className='log-v2-detail-label'>{t('日志类型')}</span>
                <span className='log-v2-detail-value'>
                  {log ? getRequestTypeMeta(currentLog, t).label : '-'}
                </span>
              </div>
            </>
          )}
          {paramOverrideCount > 0 ? (
            <div className='log-v2-detail-item'>
              <span className='log-v2-detail-label'>{t('参数覆盖')}</span>
              <button
                type='button'
                className='log-v2-text-link'
                onClick={onOpenParamOverride}
              >
                {t('查看 {{count}} 项操作', { count: paramOverrideCount })}
              </button>
            </div>
          ) : null}
          {hasChannelAffinity ? (
            <div className='log-v2-detail-item'>
              <span className='log-v2-detail-label'>{t('渠道亲和性')}</span>
              <button
                type='button'
                className='log-v2-text-link'
                onClick={onOpenChannelAffinity}
              >
                {t('查看缓存命中')}
              </button>
            </div>
          ) : null}
        </div>

        <div className='log-v2-detail-section'>
          <span className='log-v2-detail-label'>{requestBodyLabel}</span>
          <div className='log-v2-detail-prompt'>{detailText}</div>
        </div>

        <div className='log-v2-detail-footer'>
          <button
            type='button'
            className='log-v2-secondary-button'
            onClick={onClose}
          >
            {t('关闭')}
          </button>
          <button
            type='button'
            className='log-v2-primary-button'
            onClick={onCopy}
          >
            <CopyIcon size={16} />
            {t('复制请求体')}
          </button>
        </div>
      </div>
    </Modal>
  );
}

function UsageLogColumnSelectorModal({
  visible,
  onClose,
  columns,
  visibleColumns,
  billingDisplayMode,
  onToggleColumn,
  onToggleAll,
  onReset,
  onChangeBillingMode,
  t,
}) {
  const allChecked = columns.every((column) => visibleColumns[column.key]);
  const partialChecked =
    columns.some((column) => visibleColumns[column.key]) && !allChecked;

  return (
    <Modal
      centered
      visible={visible}
      onCancel={onClose}
      footer={null}
      width='min(460px, calc(100vw - 24px))'
      closable={false}
      closeIcon={null}
      maskClosable
      className='log-v2-column-modal'
    >
      <div className='log-v2-column-card'>
        <div className='log-v2-column-header'>
          <h3 className='log-v2-column-title'>{t('列设置')}</h3>
          <button
            type='button'
            className='log-v2-icon-button'
            onClick={onClose}
            aria-label={t('关闭')}
          >
            <X size={18} />
          </button>
        </div>

        <div className='log-v2-column-section'>
          <span className='log-v2-column-label'>{t('计费显示模式')}</span>
          <div className='log-v2-billing-switch'>
            <button
              type='button'
              className={
                billingDisplayMode === 'price'
                  ? 'log-v2-billing-option log-v2-billing-option-active'
                  : 'log-v2-billing-option'
              }
              onClick={() => onChangeBillingMode('price')}
            >
              {t('价格模式')}
            </button>
            <button
              type='button'
              className={
                billingDisplayMode === 'ratio'
                  ? 'log-v2-billing-option log-v2-billing-option-active'
                  : 'log-v2-billing-option'
              }
              onClick={() => onChangeBillingMode('ratio')}
            >
              {t('倍率模式')}
            </button>
          </div>
        </div>

        <div className='log-v2-column-toolbar'>
          <label className='log-v2-checkbox-item log-v2-checkbox-item-strong'>
            <input
              type='checkbox'
              checked={allChecked}
              ref={(node) => {
                if (node) {
                  node.indeterminate = partialChecked;
                }
              }}
              onChange={(event) => onToggleAll(event.target.checked)}
            />
            <span>{t('全选')}</span>
          </label>
        </div>

        <div className='log-v2-column-grid'>
          {columns.map((column) => (
            <label key={column.key} className='log-v2-checkbox-item'>
              <input
                type='checkbox'
                checked={!!visibleColumns[column.key]}
                onChange={(event) =>
                  onToggleColumn(column.key, event.target.checked)
                }
              />
              <span>{column.label}</span>
            </label>
          ))}
        </div>

        <div className='log-v2-column-footer'>
          <button
            type='button'
            className='log-v2-secondary-button'
            onClick={onReset}
          >
            {t('恢复默认')}
          </button>
          <button
            type='button'
            className='log-v2-primary-button'
            onClick={onClose}
          >
            {t('完成')}
          </button>
        </div>
      </div>
    </Modal>
  );
}

const UsageLogsPage = () => {
  const logsData = useLogsData();
  const initialFiltersRef = useRef(null);

  if (!initialFiltersRef.current) {
    initialFiltersRef.current = {
      ...logsData.formInitValues,
      dateRange: Array.isArray(logsData.formInitValues.dateRange)
        ? [...logsData.formInitValues.dateRange]
        : ['', ''],
    };
  }

  const initialFilters = initialFiltersRef.current;
  const [filters, setFilters] = useState(initialFilters);
  const filtersRef = useRef(initialFilters);
  const [advancedOpen, setAdvancedOpen] = useState(() => {
    try {
      return localStorage.getItem('usageLogAdvancedOpen') === '1';
    } catch (_) {
      return false;
    }
  });
  const [detailLog, setDetailLog] = useState(null);
  const [detailVisible, setDetailVisible] = useState(false);

  useEffect(() => {
    filtersRef.current = filters;
  }, [filters]);

  useEffect(() => {
    logsData.setFormApi({
      getValues: () => filtersRef.current,
      reset: () => {
        filtersRef.current = initialFilters;
        setFilters(initialFilters);
      },
    });
  }, []);

  useEffect(() => {
    try {
      localStorage.setItem('usageLogAdvancedOpen', advancedOpen ? '1' : '0');
    } catch (_) {}
  }, [advancedOpen]);

  const selectableColumns = useMemo(
    () => getSelectableColumns(logsData.COLUMN_KEYS, logsData.t),
    [logsData.COLUMN_KEYS, logsData.t],
  );

  const handleFilterChange = (field, value) => {
    setFilters((prev) => ({ ...prev, [field]: value }));
  };

  const handleDateChange = (index, value) => {
    setFilters((prev) => {
      const nextDateRange = [...prev.dateRange];
      nextDateRange[index] = fromDateTimeLocalValue(value);
      return {
        ...prev,
        dateRange: nextDateRange,
      };
    });
  };

  const handleSearchSubmit = async (event) => {
    event.preventDefault();
    await logsData.refresh();
  };

  const handleResetFilters = async () => {
    const nextFilters = {
      ...initialFilters,
      dateRange: [...initialFilters.dateRange],
    };
    filtersRef.current = nextFilters;
    setFilters(nextFilters);
    await logsData.refresh();
  };

  const handleOpenDetail = (record) => {
    setDetailLog(record);
    setDetailVisible(true);
  };

  const handleCopyDetail = async () => {
    const payload = buildCopyPayload(detailLog, logsData.expandData, logsData.t);
    if (!payload) {
      return;
    }
    if (await copy(payload)) {
      showSuccess(logsData.t('已复制到剪贴板！'));
      return;
    }
    showError(logsData.t('无法复制到剪贴板，请手动复制'));
  };

  const handleToggleAllColumns = (checked) => {
    selectableColumns.forEach((column) => {
      logsData.handleColumnVisibilityChange(column.key, checked);
    });
  };

  const handleOpenParamOverride = (record) => {
    if (!record) {
      return;
    }
    setDetailVisible(false);
    logsData.openParamOverrideModal(record, getLogOther(record.other) || {});
  };

  const handleOpenChannelAffinity = (record) => {
    const affinity = getLogOther(record?.other)?.admin_info?.channel_affinity;
    if (!affinity) {
      return;
    }
    logsData.openChannelAffinityUsageCacheModal(affinity);
  };

  const totalPages = Math.max(
    1,
    Math.ceil(logsData.logCount / logsData.pageSize),
  );
  const paginationItems = getPaginationItems(logsData.activePage, totalPages);
  const rangeStart =
    logsData.logCount === 0
      ? 0
      : (logsData.activePage - 1) * logsData.pageSize + 1;
  const rangeEnd = Math.min(
    logsData.activePage * logsData.pageSize,
    logsData.logCount,
  );

  const hasActiveFilters = useMemo(
    () =>
      Boolean(
        filters.username ||
          filters.token_name ||
          filters.model_name ||
          filters.group ||
          filters.request_id ||
          filters.channel ||
          (filters.logType && filters.logType !== '0'),
      ),
    [filters],
  );

  const summaryCards = useMemo(() => {
    const showStats = logsData.showStat || !logsData.loadingStat;
    const tpsValue = Number(logsData.stat?.tpm || 0) / 60;
    return [
      {
        key: 'quota',
        label: logsData.t('成功消费'),
        value: showStats ? renderQuotaValue(logsData.stat?.quota) : '...',
        subtitle: logsData.t('当前筛选时段累计'),
        cardClassName: 'log-v2-stat-card log-v2-stat-success',
        iconClassName: 'log-v2-stat-icon log-v2-stat-icon-success',
        icon: <DollarSign size={16} />,
      },
      {
        key: 'errors',
        label: logsData.t('错误次数'),
        value: showStats ? formatNumber(logsData.errorCount) : '...',
        subtitle: logsData.t('失败请求数量'),
        cardClassName: 'log-v2-stat-card log-v2-stat-error',
        iconClassName: 'log-v2-stat-icon log-v2-stat-icon-error',
        icon: <CircleAlert size={16} />,
      },
      {
        key: 'tps',
        label: logsData.t('当前 TPS'),
        value: showStats ? tpsValue.toFixed(1) : '...',
        subtitle: 'tokens/sec (avg)',
        cardClassName: 'log-v2-stat-card log-v2-stat-tps',
        iconClassName: 'log-v2-stat-icon log-v2-stat-icon-tps',
        icon: <Zap size={16} />,
      },
      {
        key: 'requests',
        label: logsData.t('总请求'),
        value: formatNumber(logsData.logCount),
        subtitle: logsData.t('当前时段请求数'),
        cardClassName: 'log-v2-stat-card log-v2-stat-cost',
        iconClassName: 'log-v2-stat-icon log-v2-stat-icon-cost',
        icon: <ClipboardList size={16} />,
      },
    ];
  }, [
    logsData.errorCount,
    logsData.loadingStat,
    logsData.logCount,
    logsData.showStat,
    logsData.stat,
    logsData.t,
  ]);

  const tableColumns = [
    {
      key: logsData.COLUMN_KEYS.TIME,
      label: logsData.t('时间'),
      headerClassName: 'log-v2-col-time',
      cellClassName: 'log-v2-cell-time',
      render: (record) => (
        <span className='log-v2-time-text'>{record.timestamp2string || '-'}</span>
      ),
    },
    {
      key: logsData.COLUMN_KEYS.TOKEN,
      label: logsData.t('名称'),
      headerClassName: 'log-v2-col-name',
      cellClassName: 'log-v2-cell-name',
      render: (record) => (
        <div className='log-v2-name-cell'>
          <span className='log-v2-name-text'>{record.token_name || '-'}</span>
        </div>
      ),
    },
    {
      key: logsData.COLUMN_KEYS.GROUP,
      label: logsData.t('分组'),
      headerClassName: 'log-v2-col-group',
      cellClassName: 'log-v2-cell-group',
      render: (record) => (
        <span className='log-v2-chip log-v2-chip-slate'>
          {getDisplayGroup(record, logsData.t)}
        </span>
      ),
    },
    {
      key: logsData.COLUMN_KEYS.TYPE,
      label: logsData.t('类型'),
      headerClassName: 'log-v2-col-type',
      cellClassName: 'log-v2-cell-type',
      render: (record) => {
        const meta = getRequestTypeMeta(record, logsData.t);
        return <span className={meta.className}>{meta.label}</span>;
      },
    },
    {
      key: logsData.COLUMN_KEYS.MODEL,
      label: logsData.t('模型'),
      headerClassName: 'log-v2-col-model',
      cellClassName: 'log-v2-cell-model',
      render: (record) => {
        const meta = getModelDisplayMeta(record);
        return <span className='log-v2-model-text'>{meta.primary}</span>;
      },
    },
    {
      key: logsData.COLUMN_KEYS.USE_TIME,
      label: logsData.t('用时/首字'),
      headerClassName: 'log-v2-col-timing',
      cellClassName: 'log-v2-cell-timing',
      render: (record) => {
        const other = getLogOther(record.other) || {};
        if (record.type === 5) {
          return (
            <span className='log-v2-inline-status log-v2-status-error'>
              {getErrorTimingLabel(record, logsData.t)}
            </span>
          );
        }
        if (record.type === 3 || record.type === 4) {
          return (
            <span className='log-v2-inline-status log-v2-status-pending'>
              {logsData.t('处理中')}
            </span>
          );
        }
        return (
          <div className='log-v2-timing-cell'>
            <span className='log-v2-timing-primary'>
              {formatSeconds(record.use_time, 2)}
            </span>
            <span className='log-v2-timing-divider'>/</span>
            <span className='log-v2-timing-highlight'>
              {formatFirstTokenSeconds(other?.frt)}
            </span>
          </div>
        );
      },
    },
    {
      key: logsData.COLUMN_KEYS.PROMPT,
      label: logsData.t('输入'),
      headerClassName: 'log-v2-col-number',
      cellClassName: 'log-v2-cell-number',
      align: 'right',
      render: (record) => {
        const cacheText = getPromptCacheText(record, logsData.t);
        return (
          <span className='log-v2-number-primary' title={cacheText || undefined}>
            {formatNumber(record.prompt_tokens)}
          </span>
        );
      },
    },
    {
      key: logsData.COLUMN_KEYS.COMPLETION,
      label: logsData.t('输出'),
      headerClassName: 'log-v2-col-number',
      cellClassName: 'log-v2-cell-number',
      align: 'right',
      render: (record) => (
        <span className='log-v2-number-primary'>
          {Number(record.completion_tokens) > 0
            ? formatNumber(record.completion_tokens)
            : '—'}
        </span>
      ),
    },
    {
      key: logsData.COLUMN_KEYS.COST,
      label: logsData.t('花费'),
      headerClassName: 'log-v2-col-cost',
      cellClassName: 'log-v2-cell-cost',
      align: 'right',
      render: (record) => {
        const meta = getCostDisplayMeta(
          record,
          logsData.billingDisplayMode,
          logsData.t,
        );
        return (
          <div
            className='log-v2-cost-cell'
            title={meta.secondary || undefined}
          >
            {meta.subscription ? (
              <span className='log-v2-chip log-v2-chip-emerald'>
                {meta.primary}
              </span>
            ) : (
              <span className='log-v2-cost-primary'>{meta.primary}</span>
            )}
          </div>
        );
      },
    },
    {
      key: logsData.COLUMN_KEYS.IP,
      label: logsData.t('IP'),
      headerClassName: 'log-v2-col-ip',
      cellClassName: 'log-v2-cell-ip',
      render: (record) => (
        <span className='log-v2-mono-text'>{record.ip || '-'}</span>
      ),
    },
    {
      key: logsData.COLUMN_KEYS.DETAILS,
      label: logsData.t('详情'),
      headerClassName: 'log-v2-col-detail',
      cellClassName: 'log-v2-cell-detail',
      align: 'center',
      render: (record) => (
        <button
          type='button'
          className={
            record.type === 5
              ? 'log-v2-detail-action log-v2-detail-action-error'
              : 'log-v2-detail-action'
          }
          onClick={() => handleOpenDetail(record)}
          aria-label={logsData.t('详情')}
          title={logsData.t('详情')}
        >
          <Eye size={16} />
        </button>
      ),
    },
  ];

  const visibleTableColumns = tableColumns.filter(
    (column) => logsData.visibleColumns[column.key],
  );

  return (
    <>
      <UserInfoModal {...logsData} />
      <ChannelAffinityUsageCacheModal {...logsData} />
      <ParamOverrideModal {...logsData} />

      <UsageLogDetailModal
        log={detailLog}
        visible={detailVisible}
        onClose={() => setDetailVisible(false)}
        onCopy={handleCopyDetail}
        onOpenParamOverride={() => handleOpenParamOverride(detailLog)}
        onOpenChannelAffinity={() => handleOpenChannelAffinity(detailLog)}
        billingDisplayMode={logsData.billingDisplayMode}
        expandData={logsData.expandData}
        isAdminUser={logsData.isAdminUser}
        t={logsData.t}
      />

      <UsageLogColumnSelectorModal
        visible={logsData.showColumnSelector}
        onClose={() => logsData.setShowColumnSelector(false)}
        columns={selectableColumns}
        visibleColumns={logsData.visibleColumns}
        billingDisplayMode={logsData.billingDisplayMode}
        onToggleColumn={logsData.handleColumnVisibilityChange}
        onToggleAll={handleToggleAllColumns}
        onReset={logsData.initDefaultColumns}
        onChangeBillingMode={logsData.setBillingDisplayMode}
        t={logsData.t}
      />

      <div className='log-v2'>
        <div className='log-v2-shell'>
          <div className='log-v2-stack'>
            <section className='log-v2-stat-grid'>
              {summaryCards.map((card) => (
                <article key={card.key} className={card.cardClassName}>
                  <div className='log-v2-stat-head'>
                    <span className='log-v2-stat-label'>{card.label}</span>
                    <span className={card.iconClassName}>{card.icon}</span>
                  </div>
                  <div className='log-v2-stat-value'>{card.value}</div>
                  <div className='log-v2-stat-subtitle'>{card.subtitle}</div>
                </article>
              ))}
            </section>

            <section className='log-v2-filter-card'>
              <form className='log-v2-filter-form' onSubmit={handleSearchSubmit}>
                <div className='log-v2-filter-top'>
                  <div className='log-v2-filter-grid'>
                    <label className='log-v2-filter-field log-v2-filter-field-range'>
                      <CalendarDays size={16} />
                      <div className='log-v2-filter-range-inner'>
                        <input
                          type='datetime-local'
                          step='1'
                          value={toDateTimeLocalValue(filters.dateRange?.[0])}
                          onClick={handleDateTimeInputClick}
                          onChange={(event) =>
                            handleDateChange(0, event.target.value)
                          }
                        />
                        <span className='log-v2-filter-range-separator'>→</span>
                        <input
                          type='datetime-local'
                          step='1'
                          value={toDateTimeLocalValue(filters.dateRange?.[1])}
                          onClick={handleDateTimeInputClick}
                          onChange={(event) =>
                            handleDateChange(1, event.target.value)
                          }
                        />
                      </div>
                    </label>

                    <label className='log-v2-filter-field'>
                      <Search size={16} />
                      <input
                        type='text'
                        value={filters.model_name}
                        placeholder={logsData.t('模型名称')}
                        onChange={(event) =>
                          handleFilterChange('model_name', event.target.value)
                        }
                      />
                    </label>

                    <label className='log-v2-filter-field'>
                      {logsData.isAdminUser ? <User size={16} /> : <Search size={16} />}
                      <input
                        type='text'
                        value={
                          logsData.isAdminUser
                            ? filters.username
                            : filters.token_name
                        }
                        placeholder={
                          logsData.isAdminUser
                            ? logsData.t('用户名称')
                            : logsData.t('令牌名称')
                        }
                        onChange={(event) =>
                          handleFilterChange(
                            logsData.isAdminUser ? 'username' : 'token_name',
                            event.target.value,
                          )
                        }
                      />
                    </label>
                  </div>

                  <div className='log-v2-filter-actions'>
                    <button
                      type='submit'
                      className='log-v2-primary-button'
                      disabled={logsData.loading}
                    >
                      {logsData.loading ? (
                        <RefreshCw size={16} className='log-v2-spin' />
                      ) : (
                        <Search size={16} />
                      )}
                      {logsData.t('查询')}
                    </button>
                    <button
                      type='button'
                      className='log-v2-secondary-button'
                      onClick={handleResetFilters}
                    >
                      <RotateCcw size={16} />
                      {logsData.t('重置')}
                    </button>
                    <button
                      type='button'
                      className='log-v2-secondary-button'
                      onClick={() => setAdvancedOpen((prev) => !prev)}
                      aria-expanded={advancedOpen}
                      aria-controls='log-v2-advanced-filters'
                    >
                      <Columns3 size={16} />
                      {logsData.t('高级筛选')}
                      <ChevronDown
                        size={14}
                        className={
                          advancedOpen
                            ? 'log-v2-chevron log-v2-chevron-open'
                            : 'log-v2-chevron'
                        }
                      />
                    </button>
                    <button
                      type='button'
                      className='log-v2-secondary-button'
                      onClick={() => logsData.setShowColumnSelector(true)}
                    >
                      <Columns3 size={16} />
                      {logsData.t('列设置')}
                    </button>
                  </div>
                </div>

                {advancedOpen ? (
                  <div
                    id='log-v2-advanced-filters'
                    className='log-v2-advanced-panel'
                  >
                    <div className='log-v2-advanced-caption'>
                      {logsData.t('更多条件（令牌、分组、请求追踪、渠道与日志类型）')}
                    </div>

                    <div className='log-v2-advanced-grid'>
                      {logsData.isAdminUser ? (
                        <label className='log-v2-filter-field'>
                          <Search size={16} />
                          <input
                            type='text'
                            value={filters.token_name}
                            placeholder={logsData.t('令牌名称')}
                            onChange={(event) =>
                              handleFilterChange('token_name', event.target.value)
                            }
                          />
                        </label>
                      ) : null}
                      <label className='log-v2-filter-field'>
                        <Search size={16} />
                        <input
                          type='text'
                          value={filters.group}
                          placeholder={logsData.t('分组')}
                          onChange={(event) =>
                            handleFilterChange('group', event.target.value)
                          }
                        />
                      </label>
                      <label className='log-v2-filter-field'>
                        <FileText size={16} />
                        <input
                          type='text'
                          value={filters.request_id}
                          placeholder='Request ID'
                          onChange={(event) =>
                            handleFilterChange('request_id', event.target.value)
                          }
                        />
                      </label>
                      {logsData.isAdminUser ? (
                        <label className='log-v2-filter-field'>
                          <Columns3 size={16} />
                          <input
                            type='text'
                            value={filters.channel}
                            placeholder={logsData.t('渠道 ID')}
                            onChange={(event) =>
                              handleFilterChange('channel', event.target.value)
                            }
                          />
                        </label>
                      ) : null}
                      <label className='log-v2-filter-field'>
                        <ClipboardList size={16} />
                        <select
                          value={filters.logType}
                          onChange={(event) =>
                            handleFilterChange('logType', event.target.value)
                          }
                        >
                          <option value='0'>{logsData.t('全部')}</option>
                          <option value='1'>{logsData.t('充值')}</option>
                          <option value='2'>{logsData.t('消费')}</option>
                          <option value='3'>{logsData.t('管理')}</option>
                          <option value='4'>{logsData.t('系统')}</option>
                          <option value='5'>{logsData.t('错误')}</option>
                          <option value='6'>{logsData.t('退款')}</option>
                        </select>
                      </label>
                    </div>
                  </div>
                ) : null}
              </form>
            </section>

            <section className='log-v2-table-card'>
              <div className='log-v2-table-scroll'>
                {logsData.loading && logsData.logs.length === 0 ? (
                  <div className='log-v2-loading-state'>
                    <div className='log-v2-loading-spinner' />
                    <div className='log-v2-loading-text'>
                      {logsData.t('加载中...')}
                    </div>
                  </div>
                ) : visibleTableColumns.length === 0 ? (
                  <div className='log-v2-no-columns'>
                    <p className='log-v2-empty-hint'>
                      {logsData.t('当前未选择任何表格列。')}
                    </p>
                    <button
                      type='button'
                      className='log-v2-primary-button log-v2-empty-button'
                      onClick={logsData.initDefaultColumns}
                    >
                      {logsData.t('恢复默认')}
                    </button>
                  </div>
                ) : logsData.logs.length > 0 ? (
                  <table className='log-v2-table'>
                    <thead>
                      <tr>
                        {visibleTableColumns.map((column) => (
                          <th
                            key={column.key}
                            className={[
                              column.headerClassName,
                              column.align === 'right'
                                ? 'log-v2-align-right'
                                : '',
                              column.align === 'center'
                                ? 'log-v2-align-center'
                                : '',
                            ]
                              .filter(Boolean)
                              .join(' ')}
                          >
                            {column.label}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {logsData.logs.map((record) => (
                        <tr key={record.key || record.id}>
                          {visibleTableColumns.map((column) => (
                            <td
                              key={column.key}
                              className={[
                                column.cellClassName,
                                column.align === 'right'
                                  ? 'log-v2-align-right'
                                  : '',
                                column.align === 'center'
                                  ? 'log-v2-align-center'
                                  : '',
                              ]
                                .filter(Boolean)
                                .join(' ')}
                            >
                              {column.render(record)}
                            </td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                ) : (
                  <div className='log-v2-empty-state'>
                    <Empty
                      image={
                        <IllustrationNoResult style={{ width: 140, height: 140 }} />
                      }
                      darkModeImage={
                        <IllustrationNoResultDark
                          style={{ width: 140, height: 140 }}
                        />
                      }
                      title={logsData.t('暂无使用日志')}
                      description={
                        hasActiveFilters
                          ? logsData.t('当前筛选条件下没有匹配的日志。')
                          : logsData.t('当前时间范围内还没有可展示的使用日志。')
                      }
                    />
                    {hasActiveFilters ? (
                      <button
                        type='button'
                        className='log-v2-secondary-button log-v2-empty-button'
                        onClick={handleResetFilters}
                      >
                        <RotateCcw size={16} />
                        {logsData.t('清空筛选')}
                      </button>
                    ) : null}
                  </div>
                )}
              </div>
              <div className='log-v2-footer'>
                <div className='log-v2-footer-summary'>
                  {logsData.t('显示第 {{start}} 到 {{end}} 条，共 {{total}} 条结果', {
                    start: rangeStart,
                    end: rangeEnd,
                    total: logsData.logCount,
                  })}
                </div>

                <div className='log-v2-footer-actions'>
                  <label className='log-v2-page-size'>
                    <span>{logsData.t('每页')}</span>
                    <select
                      value={logsData.pageSize}
                      onChange={(event) =>
                        logsData.handlePageSizeChange(Number(event.target.value))
                      }
                    >
                      {PAGE_SIZE_OPTIONS.map((size) => (
                        <option key={size} value={size}>
                          {size}
                        </option>
                      ))}
                    </select>
                  </label>

                  <nav className='log-v2-pagination' aria-label='Pagination'>
                    <button
                      type='button'
                      className='log-v2-page-button'
                      disabled={logsData.activePage <= 1}
                      onClick={() =>
                        logsData.handlePageChange(logsData.activePage - 1)
                      }
                    >
                      <ChevronLeft size={14} />
                    </button>
                    {paginationItems.map((item) =>
                      typeof item === 'number' ? (
                        <button
                          key={item}
                          type='button'
                          className={
                            item === logsData.activePage
                              ? 'log-v2-page-button log-v2-page-current'
                              : 'log-v2-page-button'
                          }
                          onClick={() => logsData.handlePageChange(item)}
                        >
                          {item}
                        </button>
                      ) : (
                        <span key={item} className='log-v2-page-ellipsis'>
                          ...
                        </span>
                      ),
                    )}
                    <button
                      type='button'
                      className='log-v2-page-button'
                      disabled={logsData.activePage >= totalPages}
                      onClick={() =>
                        logsData.handlePageChange(logsData.activePage + 1)
                      }
                    >
                      <ChevronRight size={14} />
                    </button>
                  </nav>
                </div>
              </div>
            </section>
          </div>
        </div>
      </div>
    </>
  );
};

export default UsageLogsPage;
