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
import { Empty, ImagePreview, Modal } from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  Columns3,
  Copy as CopyIcon,
  FileText,
  Image as ImageIcon,
  Plus,
  RefreshCw,
  RotateCcw,
  Search,
  Sparkles,
  X,
} from 'lucide-react';
import { timestamp2string } from '../../../helpers';
import { useMjLogsData } from '../../../hooks/mj-logs/useMjLogsData';

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];

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
  mj_id: filters?.mj_id || '',
  dateRange: Array.isArray(filters?.dateRange) ? [...filters.dateRange] : ['', ''],
});

const createDefaultFilters = (formInitValues) =>
  cloneFilters({
    channel_id: formInitValues?.channel_id || '',
    mj_id: formInitValues?.mj_id || '',
    dateRange: Array.isArray(formInitValues?.dateRange) ? formInitValues.dateRange : ['', ''],
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

const toMilliseconds = (value) => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) return 0;
  return parsed > 1e12 ? parsed : parsed * 1000;
};

const formatTimestamp = (value) => {
  const seconds = toSeconds(value);
  return seconds ? timestamp2string(seconds) : '—';
};

const formatDuration = (start, end) => {
  const startMs = toMilliseconds(start);
  const endMs = toMilliseconds(end);
  if (!startMs || !endMs || endMs < startMs) return '—';
  const seconds = (endMs - startMs) / 1000;
  if (seconds < 60) return `${seconds.toFixed(seconds >= 10 ? 0 : 1)}s`;
  const minutes = seconds / 60;
  if (minutes < 60) return `${minutes.toFixed(minutes >= 10 ? 0 : 1)}m`;
  return `${(minutes / 60).toFixed(1)}h`;
};

const parseProps = (record) => {
  if (!record?.properties) return {};
  if (typeof record.properties === 'object') return record.properties;
  try {
    return JSON.parse(record.properties);
  } catch (_) {
    return {};
  }
};

const getPrompt = (record) => {
  const props = parseProps(record);
  return String(record?.prompt || props?.finalZhPrompt || props?.finalPrompt || '').trim() || '—';
};

const getPromptEn = (record) => {
  const props = parseProps(record);
  return String(record?.prompt_en || props?.finalPrompt || '').trim() || '—';
};

const getRatio = (record) => {
  const props = parseProps(record);
  const texts = [
    record?.prompt,
    record?.prompt_en,
    props?.finalPrompt,
    props?.finalZhPrompt,
    record?.description,
  ];
  for (const text of texts) {
    const match =
      String(text || '').match(/--ar\s+(\d+\s*:\s*\d+)/i) ||
      String(text || '').match(/aspect[_\s-]?ratio[:=]\s*(\d+\s*:\s*\d+)/i);
    if (match?.[1]) return match[1].replace(/\s+/g, '');
  }
  return '—';
};

const getProgress = (value) => {
  const parsed = Number(String(value || '').replace('%', '').trim());
  return Number.isFinite(parsed) && parsed > 0 ? Math.min(100, parsed) : 0;
};

const getSourceMeta = (record) => {
  const action = String(record?.action || '').toUpperCase();
  return action === 'GENERATE' || action.includes('DALLE')
    ? { label: 'DALL-E 3', className: 'mjlog-v2-pill mjlog-v2-pill-indigo' }
    : { label: 'Midjourney', className: 'mjlog-v2-pill mjlog-v2-pill-violet' };
};

const MODE_META = {
  IMAGINE: ['Imagine', 'mjlog-v2-pill-sky'],
  UPSCALE: ['Upscale', 'mjlog-v2-pill-orange'],
  VIDEO: ['Video', 'mjlog-v2-pill-indigo'],
  EDITS: ['Edit', 'mjlog-v2-pill-amber'],
  VARIATION: ['Variation', 'mjlog-v2-pill-rose'],
  HIGH_VARIATION: ['High Variation', 'mjlog-v2-pill-rose'],
  LOW_VARIATION: ['Low Variation', 'mjlog-v2-pill-rose'],
  PAN: ['Pan', 'mjlog-v2-pill-cyan'],
  DESCRIBE: ['Describe', 'mjlog-v2-pill-amber'],
  BLEND: ['Blend', 'mjlog-v2-pill-emerald'],
  UPLOAD: ['Upload', 'mjlog-v2-pill-slate'],
  SHORTEN: ['Shorten', 'mjlog-v2-pill-amber'],
  REROLL: ['Reroll', 'mjlog-v2-pill-indigo'],
  INPAINT: ['Inpaint', 'mjlog-v2-pill-violet'],
  ZOOM: ['Zoom', 'mjlog-v2-pill-teal'],
  CUSTOM_ZOOM: ['Custom Zoom', 'mjlog-v2-pill-teal'],
  MODAL: ['Modal', 'mjlog-v2-pill-emerald'],
  SWAP_FACE: ['Swap Face', 'mjlog-v2-pill-emerald'],
};

const getModeMeta = (record) => {
  const action = String(record?.action || '').toUpperCase();
  const [label, variant] = MODE_META[action] || [action || 'Unknown', 'mjlog-v2-pill-slate'];
  return { label, className: `mjlog-v2-pill ${variant}` };
};

const getDefaultVisibleColumns = (isAdminUser) => ({
  submit_time: true,
  finish_time: true,
  source: true,
  task_id: true,
  task_status: true,
  channel: isAdminUser,
  draw_mode: true,
  submit_result: false,
  progress: false,
  prompt: true,
  proportion: true,
  fail_reason: true,
  image: true,
  prompt_en: true,
});

const MjLogsPage = () => {
  const logsData = useMjLogsData();
  const initialFilters = useRef(createDefaultFilters(logsData.formInitValues));
  const [filters, setFilters] = useState(initialFilters.current);
  const [previewTitle, setPreviewTitle] = useState(logsData.t('内容详情'));
  const [columnSelectorOpen, setColumnSelectorOpen] = useState(false);
  const filtersRef = useRef(initialFilters.current);
  const formBridgeRef = useRef(null);
  const storageKey = logsData.isAdminUser ? 'mj-log-v2-columns-admin' : 'mj-log-v2-columns-user';
  const [visibleColumns, setVisibleColumns] = useState(getDefaultVisibleColumns(logsData.isAdminUser));

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

  const onFieldChange = (field, value) => {
    setFiltersAndSync({ ...filtersRef.current, [field]: value });
  };

  const onDateChange = (index, value) => {
    const nextRange = Array.isArray(filtersRef.current.dateRange)
      ? [...filtersRef.current.dateRange]
      : ['', ''];
    nextRange[index] = fromDateTimeLocalValue(value);
    setFiltersAndSync({ ...filtersRef.current, dateRange: nextRange });
  };

  const openTextPreview = (title, content) => {
    if (!content || content === '—') return;
    setPreviewTitle(title);
    logsData.openContentModal(content);
  };

  const getStatusMeta = (record) => {
    const status = String(record?.status || '').toUpperCase();
    const map = {
      SUCCESS: ['成功', 'mjlog-v2-status mjlog-v2-status-success'],
      NOT_START: ['未启动', 'mjlog-v2-status mjlog-v2-status-neutral'],
      SUBMITTED: ['队列中', 'mjlog-v2-status mjlog-v2-status-warning'],
      IN_PROGRESS: ['进行中', 'mjlog-v2-status mjlog-v2-status-info'],
      FAILURE: ['失败', 'mjlog-v2-status mjlog-v2-status-error'],
      MODAL: ['窗口等待', 'mjlog-v2-status mjlog-v2-status-warning'],
    };
    const [label, className] = map[status] || ['未知', 'mjlog-v2-status mjlog-v2-status-neutral'];
    return { label: logsData.t(label), className };
  };

  const getSubmitMeta = (record) => {
    const code = Number(record?.code);
    const map = {
      1: ['已提交', 'mjlog-v2-pill mjlog-v2-pill-emerald'],
      21: ['等待中', 'mjlog-v2-pill mjlog-v2-pill-amber'],
      22: ['重复提交', 'mjlog-v2-pill mjlog-v2-pill-orange'],
      0: ['未提交', 'mjlog-v2-pill mjlog-v2-pill-slate'],
    };
    const [label, className] = map[code] || ['未知', 'mjlog-v2-pill mjlog-v2-pill-slate'];
    return { label: logsData.t(label), className };
  };

  const selectableColumns = [
    { key: 'submit_time', label: logsData.t('提交时间') },
    { key: 'finish_time', label: logsData.t('完成时间') },
    { key: 'source', label: logsData.t('来源') },
    { key: 'task_id', label: logsData.t('任务ID') },
    { key: 'task_status', label: logsData.t('任务状态') },
    ...(logsData.isAdminUser ? [{ key: 'channel', label: logsData.t('渠道') }] : []),
    { key: 'draw_mode', label: logsData.t('绘图模式') },
    ...(logsData.isAdminUser ? [{ key: 'submit_result', label: logsData.t('提交结果') }] : []),
    { key: 'progress', label: logsData.t('进度') },
    { key: 'prompt', label: 'Prompt' },
    { key: 'proportion', label: 'Proportion' },
    { key: 'fail_reason', label: logsData.t('失败原因') },
    { key: 'image', label: logsData.t('任务图片') },
    { key: 'prompt_en', label: 'PromptEn' },
  ];

  const resetColumns = () => {
    setVisibleColumns(getDefaultVisibleColumns(logsData.isAdminUser));
  };

  const tableColumns = [
    {
      key: 'submit_time',
      label: logsData.t('提交时间'),
      className: 'mjlog-v2-col-time',
      render: (record) => <span className='mjlog-v2-mono-text'>{formatTimestamp(record.submit_time)}</span>,
    },
    {
      key: 'finish_time',
      label: logsData.t('完成时间'),
      className: 'mjlog-v2-col-time',
      render: (record) => (
        <div className='mjlog-v2-time-stack'>
          <span className='mjlog-v2-mono-text'>{formatTimestamp(record.finish_time)}</span>
          <span className='mjlog-v2-subtle-text'>{formatDuration(record.submit_time, record.finish_time)}</span>
        </div>
      ),
    },
    {
      key: 'source',
      label: logsData.t('来源'),
      className: 'mjlog-v2-col-source',
      render: (record) => {
        const meta = getSourceMeta(record);
        return <span className={meta.className}>{meta.label}</span>;
      },
    },
    {
      key: 'task_id',
      label: logsData.t('任务ID'),
      className: 'mjlog-v2-col-task',
      render: (record) => (
        <div className='mjlog-v2-task-cell'>
          <button
            type='button'
            className='mjlog-v2-inline-link mjlog-v2-inline-link-mono'
            onClick={() => logsData.copyText(record.mj_id || '')}
            title={logsData.t('点击复制任务 ID')}
          >
            {record.mj_id || '—'}
          </button>
          {record.mj_id ? (
            <button
              type='button'
              className='mjlog-v2-copy-button'
              onClick={() => logsData.copyText(record.mj_id)}
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
      className: 'mjlog-v2-col-status',
      render: (record) => {
        const meta = getStatusMeta(record);
        return <span className={meta.className}>{meta.label}</span>;
      },
    },
    {
      key: 'channel',
      label: logsData.t('渠道'),
      className: 'mjlog-v2-col-channel',
      render: (record) =>
        record.channel_id ? (
          <button
            type='button'
            className='mjlog-v2-pill mjlog-v2-pill-slate mjlog-v2-pill-clickable'
            onClick={() => logsData.copyText(String(record.channel_id))}
            title={logsData.t('点击复制渠道 ID')}
          >
            #{record.channel_id}
          </button>
        ) : (
          <span className='mjlog-v2-empty-value'>—</span>
        ),
    },
    {
      key: 'draw_mode',
      label: logsData.t('绘图模式'),
      className: 'mjlog-v2-col-mode',
      render: (record) => {
        const meta = getModeMeta(record);
        return <span className={meta.className}>{meta.label}</span>;
      },
    },
    {
      key: 'submit_result',
      label: logsData.t('提交结果'),
      className: 'mjlog-v2-col-result',
      render: (record) => {
        const meta = getSubmitMeta(record);
        return <span className={meta.className}>{meta.label}</span>;
      },
    },
    {
      key: 'progress',
      label: logsData.t('进度'),
      className: 'mjlog-v2-col-progress',
      render: (record) => (
        <div className='mjlog-v2-progress-wrap'>
          <div className='mjlog-v2-progress-track'>
            <span className='mjlog-v2-progress-fill' style={{ width: `${getProgress(record.progress)}%` }} />
          </div>
          <span className='mjlog-v2-progress-text'>{record.progress || '0%'}</span>
        </div>
      ),
    },
    {
      key: 'prompt',
      label: 'Prompt',
      className: 'mjlog-v2-col-prompt',
      render: (record) => {
        const prompt = getPrompt(record);
        return prompt === '—' ? (
          <span className='mjlog-v2-empty-value'>—</span>
        ) : (
          <button
            type='button'
            className='mjlog-v2-text-trigger'
            title={prompt}
            onClick={() => openTextPreview('Prompt', prompt)}
          >
            {prompt}
          </button>
        );
      },
    },
    {
      key: 'proportion',
      label: 'Proportion',
      className: 'mjlog-v2-col-proportion',
      render: (record) => <span className='mjlog-v2-mono-text'>{getRatio(record)}</span>,
    },
    {
      key: 'fail_reason',
      label: logsData.t('失败原因'),
      className: 'mjlog-v2-col-fail',
      render: (record) =>
        record.fail_reason ? (
          <button
            type='button'
            className='mjlog-v2-text-trigger mjlog-v2-text-trigger-fail'
            title={record.fail_reason}
            onClick={() => openTextPreview(logsData.t('失败原因'), record.fail_reason)}
          >
            {record.fail_reason}
          </button>
        ) : (
          <span className='mjlog-v2-empty-value'>—</span>
        ),
    },
    {
      key: 'image',
      label: logsData.t('任务图片'),
      className: 'mjlog-v2-col-image',
      center: true,
      render: (record) =>
        record.image_url ? (
          <button
            type='button'
            className='mjlog-v2-image-button'
            onClick={() => logsData.openImageModal(record.image_url)}
            title={logsData.t('查看图片')}
          >
            <img
              className='mjlog-v2-image-thumb'
              src={record.image_url}
              alt={record.mj_id || 'preview'}
              loading='lazy'
            />
          </button>
        ) : (
          <span className='mjlog-v2-empty-value'>—</span>
        ),
    },
    {
      key: 'prompt_en',
      label: 'PromptEn',
      className: 'mjlog-v2-col-prompt-en',
      render: (record) => {
        const promptEn = getPromptEn(record);
        return promptEn === '—' ? (
          <span className='mjlog-v2-empty-value'>—</span>
        ) : (
          <button
            type='button'
            className='mjlog-v2-text-trigger'
            title={promptEn}
            onClick={() => openTextPreview('PromptEn', promptEn)}
          >
            {promptEn}
          </button>
        );
      },
    },
  ];

  const visibleTableColumns = tableColumns.filter((column) => visibleColumns[column.key]);
  const totalPages = Math.max(1, Math.ceil(logsData.logCount / Math.max(logsData.pageSize || 1, 1)));
  const paginationItems = getPaginationItems(logsData.activePage, totalPages);
  const rangeStart = logsData.logCount === 0 ? 0 : (logsData.activePage - 1) * logsData.pageSize + 1;
  const rangeEnd = Math.min(logsData.logCount, logsData.activePage * logsData.pageSize);
  const hasActiveFilters =
    filters.mj_id ||
    (logsData.isAdminUser && filters.channel_id) ||
    filters.dateRange?.[0] !== initialFilters.current.dateRange?.[0] ||
    filters.dateRange?.[1] !== initialFilters.current.dateRange?.[1];

  return (
    <>
      <Modal
        centered
        visible={logsData.isModalOpen}
        onCancel={() => logsData.setIsModalOpen(false)}
        footer={null}
        width='min(760px, calc(100vw - 24px))'
        closable={false}
        closeIcon={null}
        maskClosable
        className='mjlog-v2-modal'
      >
        <div className='mjlog-v2-modal-card'>
          <div className='mjlog-v2-modal-header'>
            <h3 className='mjlog-v2-modal-title'>
              <FileText size={18} />
              {previewTitle}
            </h3>
            <button
              type='button'
              className='mjlog-v2-icon-button'
              onClick={() => logsData.setIsModalOpen(false)}
              aria-label={logsData.t('关闭')}
            >
              <X size={18} />
            </button>
          </div>
          <div className='mjlog-v2-modal-body'>
            <pre className='mjlog-v2-modal-pre'>{logsData.modalContent || '—'}</pre>
          </div>
          <div className='mjlog-v2-modal-footer'>
            <button
              type='button'
              className='mjlog-v2-secondary-button'
              onClick={() => logsData.setIsModalOpen(false)}
            >
              {logsData.t('关闭')}
            </button>
            <button
              type='button'
              className='mjlog-v2-primary-button'
              onClick={() => logsData.copyText(logsData.modalContent || '')}
            >
              <CopyIcon size={16} />
              {logsData.t('复制内容')}
            </button>
          </div>
        </div>
      </Modal>

      <Modal
        centered
        visible={columnSelectorOpen}
        onCancel={() => setColumnSelectorOpen(false)}
        footer={null}
        width='min(520px, calc(100vw - 24px))'
        closable={false}
        closeIcon={null}
        maskClosable
        className='mjlog-v2-modal'
      >
        <div className='mjlog-v2-modal-card'>
          <div className='mjlog-v2-modal-header'>
            <h3 className='mjlog-v2-modal-title'>
              <Columns3 size={18} />
              {logsData.t('列设置')}
            </h3>
            <button
              type='button'
              className='mjlog-v2-icon-button'
              onClick={() => setColumnSelectorOpen(false)}
              aria-label={logsData.t('关闭')}
            >
              <X size={18} />
            </button>
          </div>
          <div className='mjlog-v2-column-toolbar'>
            <label className='mjlog-v2-checkbox-card mjlog-v2-checkbox-card-strong'>
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
          <div className='mjlog-v2-column-grid'>
            {selectableColumns.map((column) => (
              <label key={column.key} className='mjlog-v2-checkbox-card'>
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
          <div className='mjlog-v2-modal-footer'>
            <button type='button' className='mjlog-v2-secondary-button' onClick={resetColumns}>
              {logsData.t('恢复默认')}
            </button>
            <button
              type='button'
              className='mjlog-v2-primary-button'
              onClick={() => setColumnSelectorOpen(false)}
            >
              {logsData.t('完成')}
            </button>
          </div>
        </div>
      </Modal>

      <ImagePreview
        src={logsData.modalImageUrl}
        visible={logsData.isModalOpenurl}
        onVisibleChange={(visible) => logsData.setIsModalOpenurl(visible)}
      />

      <div className='mjlog-v2'>
        <div className='mjlog-v2-shell'>
          <div className='mjlog-v2-stack'>
            <section className='mjlog-v2-header'>
              <div className='mjlog-v2-header-main'>
                {/* <div className='mjlog-v2-header-icon'>
                  <ImageIcon size={20} />
                </div> */}
                <div className='mjlog-v2-header-copy'>
                  <div className='text-[30px] font-medium text-[#000000] font-medium dark:text-slate-200'>
                    {logsData.t('绘图日志')}
                  </div>
                  <p className='mjlog-v2-header-description'>
                    {logsData.t('查看所有图像生成任务的调用记录与结果预览')}
                  </p>
                </div>
              </div>
              {/* <div className='mjlog-v2-header-tags'>
                <span className='mjlog-v2-header-tag mjlog-v2-header-tag-violet'>
                  <Sparkles size={14} />
                  Midjourney
                </span>
                <span className='mjlog-v2-header-tag mjlog-v2-header-tag-emerald'>
                  <ImageIcon size={14} />
                  DALL-E 3
                </span>
                <span className='mjlog-v2-header-tag mjlog-v2-header-tag-dashed'>
                  <Plus size={14} />
                  {logsData.t('更多渠道')}
                </span>
              </div> */}
            </section>

            <section className='mjlog-v2-filter-card'>
              <form
                className='mjlog-v2-filter-form'
                onSubmit={(event) => {
                  event.preventDefault();
                  logsData.refresh();
                }}
              >
                <div className='mjlog-v2-filter-row'>
                  <div className='mjlog-v2-filter-grid'>
                    <label className='mjlog-v2-filter-field mjlog-v2-filter-field-range'>
                      <CalendarDays size={16} />
                      <div className='mjlog-v2-range-wrap'>
                        <input
                          type='datetime-local'
                          step='1'
                          value={toDateTimeLocalValue(filters.dateRange?.[0])}
                          onClick={handleDateTimeInputClick}
                          onChange={(event) => onDateChange(0, event.target.value)}
                        />
                        <span className='mjlog-v2-range-separator'>→</span>
                        <input
                          type='datetime-local'
                          step='1'
                          value={toDateTimeLocalValue(filters.dateRange?.[1])}
                          onClick={handleDateTimeInputClick}
                          onChange={(event) => onDateChange(1, event.target.value)}
                        />
                      </div>
                    </label>
                    <label className='mjlog-v2-filter-field mjlog-v2-filter-field-search'>
                      <Search size={16} />
                      <input
                        type='text'
                        value={filters.mj_id}
                        placeholder={logsData.t('任务 ID')}
                        onChange={(event) => onFieldChange('mj_id', event.target.value)}
                      />
                    </label>
                    {logsData.isAdminUser ? (
                      <label className='mjlog-v2-filter-field mjlog-v2-filter-field-search'>
                        <Search size={16} />
                        <input
                          type='text'
                          value={filters.channel_id}
                          placeholder={logsData.t('渠道 ID')}
                          onChange={(event) => onFieldChange('channel_id', event.target.value)}
                        />
                      </label>
                    ) : null}
                  </div>
                  <div className='mjlog-v2-filter-actions'>
                    <button
                      type='submit'
                      className='mjlog-v2-primary-button'
                      disabled={logsData.loading}
                    >
                      {logsData.loading ? (
                        <RefreshCw size={16} className='mjlog-v2-spin' />
                      ) : (
                        <Search size={16} />
                      )}
                      {logsData.t('查询')}
                    </button>
                    <button
                      type='button'
                      className='mjlog-v2-secondary-button'
                      onClick={() => {
                        const next = cloneFilters(initialFilters.current);
                        setFiltersAndSync(next);
                        logsData.refresh();
                      }}
                    >
                      <RotateCcw size={16} />
                      {logsData.t('重置')}
                    </button>
                    <button
                      type='button'
                      className='mjlog-v2-secondary-button'
                      onClick={() => setColumnSelectorOpen(true)}
                    >
                      <Columns3 size={16} />
                      {logsData.t('列设置')}
                    </button>
                  </div>
                </div>
              </form>
            </section>

            <section className='mjlog-v2-table-card'>
              <div className='mjlog-v2-table-scroll'>
                {logsData.loading && logsData.logs.length === 0 ? (
                  <div className='mjlog-v2-loading-state'>
                    <div className='mjlog-v2-loading-spinner' />
                    <div className='mjlog-v2-loading-text'>{logsData.t('加载中...')}</div>
                  </div>
                ) : visibleTableColumns.length === 0 ? (
                  <div className='mjlog-v2-no-columns'>
                    <p className='mjlog-v2-empty-hint'>{logsData.t('当前未选择任何表格列。')}</p>
                    <button type='button' className='mjlog-v2-primary-button' onClick={resetColumns}>
                      {logsData.t('恢复默认')}
                    </button>
                  </div>
                ) : logsData.logs.length > 0 ? (
                  <table className='mjlog-v2-table'>
                    <thead>
                      <tr>
                        {visibleTableColumns.map((column) => (
                          <th
                            key={column.key}
                            className={[
                              column.className,
                              column.center ? 'mjlog-v2-align-center' : '',
                            ].filter(Boolean).join(' ')}
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
                                column.className,
                                column.center ? 'mjlog-v2-align-center' : '',
                              ].filter(Boolean).join(' ')}
                            >
                              {column.render(record)}
                            </td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                ) : (
                  <div className='mjlog-v2-empty-state'>
                    <Empty
                      image={<IllustrationNoResult style={{ width: 140, height: 140 }} />}
                      darkModeImage={<IllustrationNoResultDark style={{ width: 140, height: 140 }} />}
                      title={logsData.t('暂无绘图日志')}
                      description={
                        hasActiveFilters
                          ? logsData.t('当前筛选条件下没有匹配的绘图任务。')
                          : logsData.t('当前时间范围内还没有可展示的绘图日志。')
                      }
                    />
                    {hasActiveFilters ? (
                      <button
                        type='button'
                        className='mjlog-v2-secondary-button'
                        onClick={() => {
                          const next = cloneFilters(initialFilters.current);
                          setFiltersAndSync(next);
                          logsData.refresh();
                        }}
                      >
                        <RotateCcw size={16} />
                        {logsData.t('清空筛选')}
                      </button>
                    ) : null}
                  </div>
                )}
              </div>
              <div className='mjlog-v2-footer'>
                <div className='mjlog-v2-footer-summary'>
                  {logsData.t('显示第 {{start}} 到 {{end}} 条，共 {{total}} 条结果', {
                    start: rangeStart,
                    end: rangeEnd,
                    total: logsData.logCount,
                  })}
                </div>
                <div className='mjlog-v2-footer-actions'>
                  <label className='mjlog-v2-page-size'>
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
                  <nav className='mjlog-v2-pagination' aria-label='Pagination'>
                    <button
                      type='button'
                      className='mjlog-v2-page-button'
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
                              ? 'mjlog-v2-page-button mjlog-v2-page-current'
                              : 'mjlog-v2-page-button'
                          }
                          onClick={() => logsData.handlePageChange(item)}
                        >
                          {item}
                        </button>
                      ) : (
                        <span key={item} className='mjlog-v2-page-ellipsis'>
                          ...
                        </span>
                      ),
                    )}
                    <button
                      type='button'
                      className='mjlog-v2-page-button'
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

export default MjLogsPage;
