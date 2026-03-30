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

import React, { useEffect, useRef, useState } from 'react';
import { Empty, Modal } from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  ClipboardList,
  Columns3,
  Copy as CopyIcon,
  Download,
  ExternalLink,
  Music,
  RefreshCw,
  RotateCcw,
  Search,
  Video,
  X,
} from 'lucide-react';
import { timestamp2string } from '../../../helpers';
import { useTaskLogsData } from '../../../hooks/task-logs/useTaskLogsData';
import ContentModal from './modals/ContentModal';
import AudioPreviewModal from './modals/AudioPreviewModal';
import {
  TASK_ACTION_FIRST_TAIL_GENERATE,
  TASK_ACTION_GENERATE,
  TASK_ACTION_REFERENCE_GENERATE,
  TASK_ACTION_REMIX_GENERATE,
  TASK_ACTION_TEXT_GENERATE,
} from '../../../constants/common.constant';

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];
const VIDEO_ACTIONS = new Set([
  TASK_ACTION_GENERATE,
  TASK_ACTION_TEXT_GENERATE,
  TASK_ACTION_FIRST_TAIL_GENERATE,
  TASK_ACTION_REFERENCE_GENERATE,
  TASK_ACTION_REMIX_GENERATE,
]);

const getPaginationItems = (currentPage, totalPages) => {
  if (totalPages <= 1) return [1];
  const items = [];
  const start = Math.max(1, currentPage - 1);
  const end = Math.min(totalPages, currentPage + 1);
  if (start > 1) items.push(1);
  if (start > 2) items.push('left-ellipsis');
  for (let page = start; page <= end; page += 1) items.push(page);
  if (end < totalPages - 1) items.push('right-ellipsis');
  if (end < totalPages) items.push(totalPages);
  return items;
};

const cloneFilters = (filters) => ({
  channel_id: filters?.channel_id || '',
  task_id: filters?.task_id || '',
  dateRange: Array.isArray(filters?.dateRange) ? [...filters.dateRange] : ['', ''],
});

const createDefaultFilters = (formInitValues) =>
  cloneFilters({
    channel_id: formInitValues?.channel_id || '',
    task_id: formInitValues?.task_id || '',
    dateRange: Array.isArray(formInitValues?.dateRange)
      ? formInitValues.dateRange
      : ['', ''],
  });

const toDateTimeLocalValue = (value) => (value ? String(value).replace(' ', 'T') : '');

const fromDateTimeLocalValue = (value) => {
  if (!value) return '';
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

const toSeconds = (value) => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) return 0;
  return parsed > 1e12 ? Math.floor(parsed / 1000) : Math.floor(parsed);
};

const formatTimestamp = (value) => {
  const seconds = toSeconds(value);
  return seconds ? timestamp2string(seconds) : '—';
};

const formatDuration = (start, end) => {
  const startSeconds = toSeconds(start);
  const endSeconds = toSeconds(end);
  if (!startSeconds || !endSeconds || endSeconds < startSeconds) return '—';
  const diff = endSeconds - startSeconds;
  if (diff < 60) return `${diff}s`;
  if (diff < 3600) return `${(diff / 60).toFixed(diff >= 600 ? 0 : 1)}m`;
  return `${(diff / 3600).toFixed(1)}h`;
};

const formatDatePart = (value) => {
  const seconds = toSeconds(value);
  if (!seconds) return '00000000';
  const date = new Date(seconds * 1000);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}${month}${day}`;
};

const getTaskBatchCode = (record) => {
  const suffix = String(record?.id || record?.task_id || '0').replace(/\D/g, '');
  return `BATCH-${formatDatePart(record?.submit_time || record?.created_at)}-${String(
    suffix || 0,
  ).padStart(3, '0')}`;
};

const safeJsonParse = (value, fallback = null) => {
  if (!value || typeof value !== 'string') return fallback;
  try {
    return JSON.parse(value);
  } catch (_) {
    return fallback;
  }
};

const getAudioClips = (record) => {
  if (Array.isArray(record?.data)) return record.data.filter(Boolean);
  const parsed = safeJsonParse(record?.data, null);
  return Array.isArray(parsed) ? parsed.filter(Boolean) : [];
};

const hasAudioPreview = (record) =>
  record?.platform === 'suno' &&
  String(record?.status || '').toUpperCase() === 'SUCCESS' &&
  getAudioClips(record).some((clip) => clip?.audio_url);

const getResultUrl = (record) =>
  typeof record?.result_url === 'string' && /^https?:\/\//i.test(record.result_url)
    ? record.result_url
    : '';

const hasVideoPreview = (record) =>
  VIDEO_ACTIONS.has(record?.action) &&
  String(record?.status || '').toUpperCase() === 'SUCCESS' &&
  !!getResultUrl(record);

const getProgressMeta = (value) => {
  if (value === null || value === undefined || value === '') return null;
  const rawText = String(value).trim();
  if (!rawText) return null;
  const ratioMatch = rawText.match(/(\d+)\s*\/\s*(\d+)/);
  if (ratioMatch) {
    const current = Number(ratioMatch[1]);
    const total = Number(ratioMatch[2]);
    if (total > 0) {
      return {
        percent: Math.min(100, Math.max(0, Math.round((current / total) * 100))),
        label: rawText,
      };
    }
  }
  const percentMatch = rawText.match(/(\d+(?:\.\d+)?)\s*%/);
  if (percentMatch) {
    return {
      percent: Math.min(100, Math.max(0, Math.round(Number(percentMatch[1])))),
      label: rawText,
    };
  }
  const numericValue = Number(rawText);
  if (Number.isFinite(numericValue) && numericValue > 0) {
    return {
      percent: Math.min(100, Math.max(0, Math.round(numericValue))),
      label: `${Math.round(numericValue)}%`,
    };
  }
  return null;
};

const getSourceMeta = (record, t) => {
  const key = String(record?.platform || '').toLowerCase();
  const map = {
    suno: ['Suno', 'tasklog-v2-pill tasklog-v2-pill-emerald'],
    kling: ['Kling', 'tasklog-v2-pill tasklog-v2-pill-sky'],
    runway: ['Runway', 'tasklog-v2-pill tasklog-v2-pill-violet'],
    luma: ['Luma', 'tasklog-v2-pill tasklog-v2-pill-indigo'],
    minimax: ['MiniMax', 'tasklog-v2-pill tasklog-v2-pill-amber'],
    jimeng: ['即梦', 'tasklog-v2-pill tasklog-v2-pill-orange'],
    veo: ['Veo', 'tasklog-v2-pill tasklog-v2-pill-cyan'],
    pika: ['Pika', 'tasklog-v2-pill tasklog-v2-pill-rose'],
  };
  if (map[key]) {
    const [label, className] = map[key];
    return { label, className };
  }
  if (key) {
    return {
      label: key.length <= 10 ? key.toUpperCase() : key,
      className: 'tasklog-v2-pill tasklog-v2-pill-slate',
    };
  }
  return { label: t('未知'), className: 'tasklog-v2-pill tasklog-v2-pill-slate' };
};

const getTypeMeta = (record, t) => {
  const map = {
    MUSIC: [t('生成音乐'), 'tasklog-v2-pill tasklog-v2-pill-slate'],
    LYRICS: [t('生成歌词'), 'tasklog-v2-pill tasklog-v2-pill-rose'],
    [TASK_ACTION_GENERATE]: [t('图生视频'), 'tasklog-v2-pill tasklog-v2-pill-violet'],
    [TASK_ACTION_TEXT_GENERATE]: [t('文生视频'), 'tasklog-v2-pill tasklog-v2-pill-indigo'],
    [TASK_ACTION_FIRST_TAIL_GENERATE]: [
      t('首尾生视频'),
      'tasklog-v2-pill tasklog-v2-pill-cyan',
    ],
    [TASK_ACTION_REFERENCE_GENERATE]: [
      t('参照生视频'),
      'tasklog-v2-pill tasklog-v2-pill-orange',
    ],
    [TASK_ACTION_REMIX_GENERATE]: [
      t('视频 Remix'),
      'tasklog-v2-pill tasklog-v2-pill-amber',
    ],
  };
  const action = record?.action;
  if (map[action]) {
    const [label, className] = map[action];
    return { label, className };
  }
  return { label: action || t('未知'), className: 'tasklog-v2-pill tasklog-v2-pill-slate' };
};

const getStatusMeta = (record, t) => {
  const status = String(record?.status || '').toUpperCase();
  const map = {
    SUCCESS: [t('已完成'), 'tasklog-v2-status tasklog-v2-status-success'],
    FAILURE: [t('失败'), 'tasklog-v2-status tasklog-v2-status-error'],
    IN_PROGRESS: [t('进行中'), 'tasklog-v2-status tasklog-v2-status-info'],
    SUBMITTED: [t('队列中'), 'tasklog-v2-status tasklog-v2-status-warning'],
    QUEUED: [t('排队中'), 'tasklog-v2-status tasklog-v2-status-warning'],
    NOT_START: [t('未启动'), 'tasklog-v2-status tasklog-v2-status-neutral'],
    CANCELLED: [t('已取消'), 'tasklog-v2-status tasklog-v2-status-neutral'],
    CANCELED: [t('已取消'), 'tasklog-v2-status tasklog-v2-status-neutral'],
    UNKNOWN: [t('未知'), 'tasklog-v2-status tasklog-v2-status-neutral'],
    '': [t('提交中'), 'tasklog-v2-status tasklog-v2-status-warning'],
  };
  const [label, className] = map[status] || [
    status || t('未知'),
    'tasklog-v2-status tasklog-v2-status-neutral',
  ];
  return { label, className };
};

const getInfoText = (record, t) => {
  if (hasAudioPreview(record)) return t('点击预览音乐');
  if (hasVideoPreview(record)) return t('点击预览视频');
  if (String(record?.status || '').toUpperCase() === 'FAILURE') {
    return record?.fail_reason || t('执行失败');
  }
  const progressMeta = getProgressMeta(record?.progress);
  if (progressMeta) return progressMeta.label;
  if (String(record?.status || '').toUpperCase() === 'SUCCESS') return t('任务已完成');
  return getStatusMeta(record, t).label;
};

const getDefaultVisibleColumns = (isAdminUser) => ({
  submit_time: true,
  batch_code: true,
  finish_time: true,
  source: true,
  type: true,
  task_id: true,
  task_status: true,
  channel: isAdminUser,
  info: true,
});

const exportCsv = (filename, headers, rows) => {
  const escapeCell = (value) => {
    const normalized = String(value ?? '');
    if (normalized.includes(',') || normalized.includes('"') || normalized.includes('\n')) {
      return `"${normalized.replace(/"/g, '""')}"`;
    }
    return normalized;
  };
  const content = [
    headers.map(escapeCell).join(','),
    ...rows.map((row) => row.map(escapeCell).join(',')),
  ].join('\n');
  const blob = new Blob([`${String.fromCharCode(0xfeff)}${content}`], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.setTimeout(() => URL.revokeObjectURL(url), 0);
};

const TaskLogsPage = () => {
  const logsData = useTaskLogsData();
  const initialFilters = useRef(createDefaultFilters(logsData.formInitValues));
  const filtersRef = useRef(initialFilters.current);
  const formBridgeRef = useRef(null);
  const storageKey = logsData.isAdminUser
    ? 'task-log-v2-columns-admin'
    : 'task-log-v2-columns-user';

  const [filters, setFilters] = useState(initialFilters.current);
  const [columnSelectorOpen, setColumnSelectorOpen] = useState(false);
  const [visibleColumns, setVisibleColumns] = useState(
    getDefaultVisibleColumns(logsData.isAdminUser),
  );

  if (!formBridgeRef.current) {
    formBridgeRef.current = {
      getValues: () => filtersRef.current,
      reset: () => {
        const next = cloneFilters(initialFilters.current);
        filtersRef.current = next;
        setFilters(next);
      },
    };
  }

  useEffect(() => {
    logsData.setFormApi(formBridgeRef.current);
  }, [logsData.setFormApi]);

  useEffect(() => {
    filtersRef.current = filters;
  }, [filters]);

  useEffect(() => {
    const defaults = getDefaultVisibleColumns(logsData.isAdminUser);
    const saved = localStorage.getItem(storageKey);
    if (!saved) {
      setVisibleColumns(defaults);
      return;
    }
    try {
      setVisibleColumns({ ...defaults, ...JSON.parse(saved) });
    } catch (_) {
      setVisibleColumns(defaults);
    }
  }, [logsData.isAdminUser, storageKey]);

  useEffect(() => {
    localStorage.setItem(storageKey, JSON.stringify(visibleColumns));
  }, [storageKey, visibleColumns]);

  const setFiltersAndSync = (next) => {
    filtersRef.current = next;
    setFilters(next);
  };

  const handleFieldChange = (field, value) => {
    setFiltersAndSync({ ...filtersRef.current, [field]: value });
  };

  const handleDateChange = (index, value) => {
    const nextRange = Array.isArray(filtersRef.current.dateRange)
      ? [...filtersRef.current.dateRange]
      : ['', ''];
    nextRange[index] = fromDateTimeLocalValue(value);
    setFiltersAndSync({ ...filtersRef.current, dateRange: nextRange });
  };

  const handleOpenDetail = (record) => {
    logsData.openContentModal(JSON.stringify(record, null, 2));
  };

  const selectableColumns = [
    { key: 'submit_time', label: logsData.t('提交时间') },
    { key: 'batch_code', label: logsData.t('批次编码') },
    { key: 'finish_time', label: logsData.t('完成时间') },
    { key: 'source', label: logsData.t('来源') },
    { key: 'type', label: logsData.t('类型') },
    { key: 'task_id', label: logsData.t('任务ID') },
    { key: 'task_status', label: logsData.t('任务状态') },
    ...(logsData.isAdminUser
      ? [{ key: 'channel', label: logsData.t('渠道') }]
      : []),
    { key: 'info', label: logsData.t('信息') },
  ];

  const tableColumns = [
    {
      key: 'submit_time',
      label: logsData.t('提交时间'),
      className: 'tasklog-v2-col-time',
      render: (record) => (
        <span className='tasklog-v2-mono-text'>
          {formatTimestamp(record.submit_time)}
        </span>
      ),
    },
    {
      key: 'batch_code',
      label: logsData.t('批次编码'),
      className: 'tasklog-v2-col-batch',
      render: (record) => (
        <span className='tasklog-v2-batch-code'>{getTaskBatchCode(record)}</span>
      ),
    },
    {
      key: 'finish_time',
      label: logsData.t('完成时间'),
      className: 'tasklog-v2-col-time',
      render: (record) => (
        <div className='tasklog-v2-time-stack'>
          <span className='tasklog-v2-mono-text'>
            {formatTimestamp(record.finish_time)}
          </span>
          <span className='tasklog-v2-subtle-text'>
            {formatDuration(record.submit_time, record.finish_time)}
          </span>
        </div>
      ),
    },
    {
      key: 'source',
      label: logsData.t('来源'),
      className: 'tasklog-v2-col-source',
      render: (record) => {
        const meta = getSourceMeta(record, logsData.t);
        return <span className={meta.className}>{meta.label}</span>;
      },
    },
    {
      key: 'type',
      label: logsData.t('类型'),
      className: 'tasklog-v2-col-type',
      render: (record) => {
        const meta = getTypeMeta(record, logsData.t);
        return <span className={meta.className}>{meta.label}</span>;
      },
    },
    {
      key: 'task_id',
      label: logsData.t('任务ID'),
      className: 'tasklog-v2-col-task',
      render: (record) => (
        <div className='tasklog-v2-task-cell'>
          <button
            type='button'
            className='tasklog-v2-inline-link tasklog-v2-inline-link-mono'
            onClick={() => handleOpenDetail(record)}
            title={logsData.t('查看任务详情')}
          >
            {record.task_id || '—'}
          </button>
          {record.task_id ? (
            <button
              type='button'
              className='tasklog-v2-copy-button'
              onClick={() => logsData.copyText(record.task_id)}
              aria-label={logsData.t('复制任务 ID')}
            >
              <CopyIcon size={14} />
            </button>
          ) : null}
        </div>
      ),
    },
    {
      key: 'task_status',
      label: logsData.t('任务状态'),
      className: 'tasklog-v2-col-status',
      render: (record) => {
        const meta = getStatusMeta(record, logsData.t);
        return <span className={meta.className}>{meta.label}</span>;
      },
    },
    {
      key: 'channel',
      label: logsData.t('渠道'),
      className: 'tasklog-v2-col-channel',
      render: (record) =>
        record.channel_id ? (
          <button
            type='button'
            className='tasklog-v2-pill tasklog-v2-pill-slate tasklog-v2-pill-clickable'
            onClick={() => logsData.copyText(String(record.channel_id))}
            title={logsData.t('点击复制渠道 ID')}
          >
            #{record.channel_id}
          </button>
        ) : (
          <span className='tasklog-v2-empty-value'>—</span>
        ),
    },
    {
      key: 'info',
      label: logsData.t('信息'),
      className: 'tasklog-v2-col-info',
      render: (record) => {
        if (hasAudioPreview(record)) {
          return (
            <button
              type='button'
              className='tasklog-v2-link-button'
              onClick={() => logsData.openAudioModal(getAudioClips(record))}
            >
              <Music size={14} />
              {logsData.t('预览音乐')}
            </button>
          );
        }

        if (hasVideoPreview(record)) {
          const resultUrl = getResultUrl(record);
          return (
            <div className='tasklog-v2-info-actions'>
              <button
                type='button'
                className='tasklog-v2-link-button'
                onClick={() => logsData.openVideoModal(resultUrl)}
              >
                <Video size={14} />
                {logsData.t('预览视频')}
              </button>
              <button
                type='button'
                className='tasklog-v2-ghost-icon'
                onClick={() => window.open(resultUrl, '_blank', 'noopener,noreferrer')}
                aria-label={logsData.t('在新标签页打开')}
              >
                <ExternalLink size={14} />
              </button>
            </div>
          );
        }

        if (String(record?.status || '').toUpperCase() === 'FAILURE') {
          const failureText = record?.fail_reason || logsData.t('执行失败');
          return (
            <button
              type='button'
              className='tasklog-v2-text-trigger tasklog-v2-text-trigger-error'
              title={failureText}
              onClick={() => logsData.openContentModal(failureText)}
            >
              {failureText}
            </button>
          );
        }

        const progressMeta = getProgressMeta(record?.progress);
        if (progressMeta) {
          return (
            <div className='tasklog-v2-progress-wrap'>
              <div className='tasklog-v2-progress-track'>
                <span
                  className='tasklog-v2-progress-fill'
                  style={{ width: `${progressMeta.percent}%` }}
                />
              </div>
              <span className='tasklog-v2-progress-text'>{progressMeta.label}</span>
            </div>
          );
        }

        return (
          <span className='tasklog-v2-subtle-text'>
            {getInfoText(record, logsData.t)}
          </span>
        );
      },
    },
  ];

  const visibleTableColumns = tableColumns.filter(
    (column) => visibleColumns[column.key],
  );
  const totalPages = Math.max(
    1,
    Math.ceil(logsData.logCount / Math.max(logsData.pageSize || 1, 1)),
  );
  const paginationItems = getPaginationItems(logsData.activePage, totalPages);
  const rangeStart =
    logsData.logCount === 0
      ? 0
      : (logsData.activePage - 1) * logsData.pageSize + 1;
  const rangeEnd = Math.min(logsData.logCount, logsData.activePage * logsData.pageSize);
  const hasActiveFilters =
    !!filters.task_id ||
    (logsData.isAdminUser && !!filters.channel_id) ||
    filters.dateRange?.[0] !== initialFilters.current.dateRange?.[0] ||
    filters.dateRange?.[1] !== initialFilters.current.dateRange?.[1];

  const handleSearchSubmit = (event) => {
    event.preventDefault();
    logsData.refresh();
  };

  const handleResetFilters = () => {
    const next = cloneFilters(initialFilters.current);
    setFiltersAndSync(next);
    logsData.refresh();
  };

  const handleExportLogs = () => {
    if (!logsData.logs.length || !visibleTableColumns.length) return;
    const headers = visibleTableColumns.map((column) => column.label);
    const rows = logsData.logs.map((record) =>
      visibleTableColumns.map((column) => {
        switch (column.key) {
          case 'submit_time':
            return formatTimestamp(record.submit_time);
          case 'batch_code':
            return getTaskBatchCode(record);
          case 'finish_time':
            return formatTimestamp(record.finish_time);
          case 'source':
            return getSourceMeta(record, logsData.t).label;
          case 'type':
            return getTypeMeta(record, logsData.t).label;
          case 'task_id':
            return record.task_id || '';
          case 'task_status':
            return getStatusMeta(record, logsData.t).label;
          case 'channel':
            return record.channel_id ? `#${record.channel_id}` : '—';
          case 'info':
            return getInfoText(record, logsData.t);
          default:
            return '';
        }
      }),
    );
    exportCsv(
      `task-logs-${formatDatePart(Date.now())}-${logsData.activePage}.csv`,
      headers,
      rows,
    );
  };

  return (
    <>
      <ContentModal {...logsData} isVideo={false} />
      <ContentModal
        isModalOpen={logsData.isVideoModalOpen}
        setIsModalOpen={logsData.setIsVideoModalOpen}
        modalContent={logsData.videoUrl}
        isVideo={true}
      />
      <AudioPreviewModal
        isModalOpen={logsData.isAudioModalOpen}
        setIsModalOpen={logsData.setIsAudioModalOpen}
        audioClips={logsData.audioClips}
      />

      <Modal
        centered
        visible={columnSelectorOpen}
        onCancel={() => setColumnSelectorOpen(false)}
        footer={null}
        width='min(460px, calc(100vw - 24px))'
        closable={false}
        closeIcon={null}
        maskClosable
        className='tasklog-v2-modal'
      >
        <div className='tasklog-v2-modal-card'>
          <div className='tasklog-v2-modal-header'>
            <h3 className='tasklog-v2-modal-title'>
              <Columns3 size={18} />
              {logsData.t('列设置')}
            </h3>
            <button
              type='button'
              className='tasklog-v2-icon-button'
              onClick={() => setColumnSelectorOpen(false)}
              aria-label={logsData.t('关闭')}
            >
              <X size={18} />
            </button>
          </div>
          <div className='tasklog-v2-column-toolbar'>
            <label className='tasklog-v2-checkbox-card tasklog-v2-checkbox-card-strong'>
              <input
                type='checkbox'
                checked={
                  selectableColumns.length > 0 &&
                  selectableColumns.every((column) => visibleColumns[column.key])
                }
                onChange={(event) =>
                  setVisibleColumns((current) => {
                    const next = { ...current };
                    selectableColumns.forEach((column) => {
                      next[column.key] = event.target.checked;
                    });
                    return next;
                  })
                }
              />
              <span>{logsData.t('全选')}</span>
            </label>
          </div>
          <div className='tasklog-v2-column-grid'>
            {selectableColumns.map((column) => (
              <label key={column.key} className='tasklog-v2-checkbox-card'>
                <input
                  type='checkbox'
                  checked={!!visibleColumns[column.key]}
                  onChange={(event) =>
                    setVisibleColumns((current) => ({
                      ...current,
                      [column.key]: event.target.checked,
                    }))
                  }
                />
                <span>{column.label}</span>
              </label>
            ))}
          </div>
          <div className='tasklog-v2-modal-footer'>
            <button
              type='button'
              className='tasklog-v2-secondary-button'
              onClick={() => setVisibleColumns(getDefaultVisibleColumns(logsData.isAdminUser))}
            >
              {logsData.t('恢复默认')}
            </button>
            <button
              type='button'
              className='tasklog-v2-primary-button'
              onClick={() => setColumnSelectorOpen(false)}
            >
              {logsData.t('完成')}
            </button>
          </div>
        </div>
      </Modal>

      <div className='tasklog-v2'>
        <div className='tasklog-v2-shell'>
          <div className='tasklog-v2-stack'>
            <section className='tasklog-v2-page-header'>
              <div className='tasklog-v2-header-main'>
                <div className='tasklog-v2-header-icon'>
                  <ClipboardList size={20} />
                </div>
                <div className='tasklog-v2-header-copy'>
                  <h2 className='tasklog-v2-header-title'>{logsData.t('任务记录')}</h2>
                  <p className='tasklog-v2-header-description'>
                    {logsData.t('查看所有异步批量任务的执行记录与状态追踪')}
                  </p>
                </div>
              </div>
              <button
                type='button'
                className='tasklog-v2-export-button'
                onClick={handleExportLogs}
                disabled={!logsData.logs.length || !visibleTableColumns.length}
              >
                <Download size={14} />
                {logsData.t('导出记录')}
              </button>
            </section>

            <section className='tasklog-v2-filter-card'>
              <form className='tasklog-v2-filter-form' onSubmit={handleSearchSubmit}>
                <div className='tasklog-v2-filter-row'>
                  <div className='tasklog-v2-filter-grid'>
                    <label className='tasklog-v2-filter-field tasklog-v2-filter-field-range'>
                      <CalendarDays size={16} />
                      <div className='tasklog-v2-filter-range'>
                        <input
                          type='datetime-local'
                          step='1'
                          value={toDateTimeLocalValue(filters.dateRange?.[0])}
                          onClick={handleDateTimeInputClick}
                          onChange={(event) => handleDateChange(0, event.target.value)}
                        />
                        <span className='tasklog-v2-range-separator'>→</span>
                        <input
                          type='datetime-local'
                          step='1'
                          value={toDateTimeLocalValue(filters.dateRange?.[1])}
                          onClick={handleDateTimeInputClick}
                          onChange={(event) => handleDateChange(1, event.target.value)}
                        />
                      </div>
                    </label>

                    <label className='tasklog-v2-filter-field tasklog-v2-filter-field-search'>
                      <Search size={16} />
                      <input
                        type='text'
                        value={filters.task_id}
                        placeholder={logsData.t('任务 ID')}
                        onChange={(event) => handleFieldChange('task_id', event.target.value)}
                      />
                    </label>

                    {logsData.isAdminUser ? (
                      <label className='tasklog-v2-filter-field tasklog-v2-filter-field-search'>
                        <Search size={16} />
                        <input
                          type='text'
                          value={filters.channel_id}
                          placeholder={logsData.t('渠道 ID')}
                          onChange={(event) => handleFieldChange('channel_id', event.target.value)}
                        />
                      </label>
                    ) : null}
                  </div>

                  <div className='tasklog-v2-filter-actions'>
                    <button
                      type='submit'
                      className='tasklog-v2-primary-button'
                      disabled={logsData.loading}
                    >
                      {logsData.loading ? (
                        <RefreshCw size={16} className='tasklog-v2-spin' />
                      ) : (
                        <Search size={16} />
                      )}
                      {logsData.t('查询')}
                    </button>
                    <button
                      type='button'
                      className='tasklog-v2-secondary-button'
                      onClick={handleResetFilters}
                    >
                      <RotateCcw size={16} />
                      {logsData.t('重置')}
                    </button>
                    <button
                      type='button'
                      className='tasklog-v2-secondary-button'
                      onClick={() => setColumnSelectorOpen(true)}
                    >
                      <Columns3 size={16} />
                      {logsData.t('列设置')}
                    </button>
                  </div>
                </div>
              </form>
            </section>

            <section className='tasklog-v2-table-card'>
              <div className='tasklog-v2-table-scroll'>
                {logsData.loading && logsData.logs.length === 0 ? (
                  <div className='tasklog-v2-loading-state'>
                    <div className='tasklog-v2-loading-spinner' />
                    <div className='tasklog-v2-loading-text'>{logsData.t('加载中...')}</div>
                  </div>
                ) : visibleTableColumns.length === 0 ? (
                  <div className='tasklog-v2-no-columns'>
                    <p className='tasklog-v2-empty-hint'>{logsData.t('当前未选择任何表格列。')}</p>
                    <button
                      type='button'
                      className='tasklog-v2-primary-button'
                      onClick={() => setVisibleColumns(getDefaultVisibleColumns(logsData.isAdminUser))}
                    >
                      {logsData.t('恢复默认')}
                    </button>
                  </div>
                ) : logsData.logs.length > 0 ? (
                  <table className='tasklog-v2-table'>
                    <thead>
                      <tr>
                        {visibleTableColumns.map((column) => (
                          <th key={column.key} className={column.className}>
                            {column.label}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {logsData.logs.map((record) => (
                        <tr key={record.key || record.id}>
                          {visibleTableColumns.map((column) => (
                            <td key={column.key} className={column.className}>
                              {column.render(record)}
                            </td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                ) : (
                  <div className='tasklog-v2-empty-state'>
                    <Empty
                      image={<IllustrationNoResult style={{ width: 140, height: 140 }} />}
                      darkModeImage={
                        <IllustrationNoResultDark style={{ width: 140, height: 140 }} />
                      }
                      title={logsData.t('暂无任务日志')}
                      description={
                        hasActiveFilters
                          ? logsData.t('当前筛选条件下没有匹配的任务记录。')
                          : logsData.t('当前时间范围内还没有可展示的任务日志。')
                      }
                    />
                    {hasActiveFilters ? (
                      <button
                        type='button'
                        className='tasklog-v2-secondary-button'
                        onClick={handleResetFilters}
                      >
                        <RotateCcw size={16} />
                        {logsData.t('清空筛选')}
                      </button>
                    ) : null}
                  </div>
                )}
              </div>
              <div className='tasklog-v2-footer'>
                <div className='tasklog-v2-footer-summary'>
                  {logsData.t('显示第 {{start}} 到 {{end}} 条，共 {{total}} 条结果', {
                    start: rangeStart,
                    end: rangeEnd,
                    total: logsData.logCount,
                  })}
                </div>
                <div className='tasklog-v2-footer-actions'>
                  <label className='tasklog-v2-page-size'>
                    <span>{logsData.t('每页')}</span>
                    <select
                      value={logsData.pageSize}
                      onChange={(event) => logsData.handlePageSizeChange(Number(event.target.value))}
                    >
                      {PAGE_SIZE_OPTIONS.map((size) => (
                        <option key={size} value={size}>
                          {size}
                        </option>
                      ))}
                    </select>
                  </label>

                  <nav className='tasklog-v2-pagination' aria-label='Pagination'>
                    <button
                      type='button'
                      className='tasklog-v2-page-button'
                      disabled={logsData.activePage <= 1}
                      onClick={() => logsData.handlePageChange(logsData.activePage - 1)}
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
                              ? 'tasklog-v2-page-button tasklog-v2-page-current'
                              : 'tasklog-v2-page-button'
                          }
                          onClick={() => logsData.handlePageChange(item)}
                        >
                          {item}
                        </button>
                      ) : (
                        <span key={item} className='tasklog-v2-page-ellipsis'>
                          ...
                        </span>
                      ),
                    )}
                    <button
                      type='button'
                      className='tasklog-v2-page-button'
                      disabled={logsData.activePage >= totalPages}
                      onClick={() => logsData.handlePageChange(logsData.activePage + 1)}
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

export default TaskLogsPage;
