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

import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { Toast } from '@douyinfe/semi-ui';
import {
  Copy,
  Download,
  Film,
  ListVideo,
  ImagePlus,
  LoaderCircle,
  Play,
  RefreshCw,
  RotateCcw,
  Send,
  Settings2,
  Video,
  X,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, copy } from '../../helpers';
import {
  API_ENDPOINTS,
  MINIMAX_H3_MODEL,
  MINIMAX_H3_REF2VA_MODEL,
  MINIMAX_H3_MODELS,
} from '../../constants/playground.constants';

const terminalStatuses = new Set(['completed', 'failed']);
const activeStatuses = new Set(['queued', 'in_progress']);

const normalizeVideoTask = (source) => {
  if (!source) return null;
  const taskID =
    source.task_id || (typeof source.id === 'string' ? source.id : '');
  if (!taskID) return null;

  const rawStatus = String(source.status || '').toLowerCase();
  const status =
    {
      not_start: 'queued',
      dispatching: 'queued',
      submitted: 'queued',
      queued: 'queued',
      in_progress: 'in_progress',
      processing: 'in_progress',
      success: 'completed',
      completed: 'completed',
      failure: 'failed',
      failed: 'failed',
    }[rawStatus] ||
    rawStatus ||
    'queued';
  const progressValue = Number.parseFloat(
    String(source.progress || '').replace('%', ''),
  );

  return {
    ...source,
    id: taskID,
    task_id: taskID,
    status,
    progress: Number.isFinite(progressValue) ? progressValue : 0,
    error:
      source.error ||
      (source.fail_reason ? { message: source.fail_reason } : undefined),
  };
};

const formatTaskTime = (timestamp) => {
  const value = Number(timestamp);
  if (!Number.isFinite(value) || value <= 0) return '';
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value * 1000));
};

const getErrorMessage = (error, fallback) =>
  error?.response?.data?.error?.message ||
  error?.response?.data?.message ||
  error?.message ||
  fallback;

const FramePicker = ({
  label,
  file,
  previewUrl,
  disabled,
  onChange,
  onClear,
}) => {
  const { t } = useTranslation();
  const inputRef = useRef(null);

  return (
    <div
      className='playground-v2-frame-picker'
      data-has-image={Boolean(previewUrl)}
    >
      <div className='playground-v2-frame-picker-header'>
        <span>{label}</span>
        {file && (
          <button
            type='button'
            className='playground-v2-action-button'
            onClick={onClear}
            disabled={disabled}
            aria-label={t('移除图片')}
            title={t('移除图片')}
          >
            <X size={15} />
          </button>
        )}
      </div>

      <button
        type='button'
        className='playground-v2-frame-picker-body'
        onClick={() => inputRef.current?.click()}
        disabled={disabled}
      >
        {previewUrl ? (
          <img src={previewUrl} alt={label} />
        ) : (
          <span className='playground-v2-frame-picker-empty'>
            <ImagePlus size={22} />
            <span>{t('选择图片')}</span>
            <small>{t('支持 JPG、PNG、WebP，最大 10 MB')}</small>
          </span>
        )}
      </button>

      <input
        ref={inputRef}
        type='file'
        accept='image/jpeg,image/png,image/webp'
        hidden
        disabled={disabled}
        onChange={(event) => {
          const nextFile = event.target.files?.[0] || null;
          if (nextFile) onChange(nextFile);
          event.target.value = '';
        }}
      />
    </div>
  );
};

export const useMiniMaxH3VideoGeneration = ({ enabled, group, userId, model }) => {
  const { t } = useTranslation();
  const isRef2va = model === MINIMAX_H3_REF2VA_MODEL;
  const storageKey = `minimax_h3_playground_task_${model || MINIMAX_H3_MODEL}_${userId || 'unknown'}`;
  const [prompt, setPrompt] = useState('');
  const [taskType, setTaskType] = useState('t2va');
  const [aspectRatio, setAspectRatio] = useState('9:16');
  const [seed, setSeed] = useState('');
  const [firstFrame, setFirstFrame] = useState(null);
  const [lastFrame, setLastFrame] = useState(null);
  const [referenceVideo, setReferenceVideo] = useState(null);
  const [startTimeSeconds, setStartTimeSeconds] = useState('0');
  const [tasks, setTasks] = useState([]);
  const [selectedTaskID, setSelectedTaskID] = useState('');
  const [historyLoading, setHistoryLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [pollError, setPollError] = useState('');
  const tasksRef = useRef([]);

  useEffect(() => {
    setTaskType('t2va');
    setAspectRatio('9:16');
    setSeed('');
    setFirstFrame(null);
    setLastFrame(null);
    setReferenceVideo(null);
    setStartTimeSeconds('0');
    setPrompt('');
  }, [model]);

  useEffect(() => {
    tasksRef.current = tasks;
  }, [tasks]);

  const task = useMemo(
    () => tasks.find((item) => item.id === selectedTaskID) || null,
    [selectedTaskID, tasks],
  );

  const firstPreview = useMemo(
    () => (firstFrame ? URL.createObjectURL(firstFrame) : ''),
    [firstFrame],
  );
  const lastPreview = useMemo(
    () => (lastFrame ? URL.createObjectURL(lastFrame) : ''),
    [lastFrame],
  );

  useEffect(
    () => () => firstPreview && URL.revokeObjectURL(firstPreview),
    [firstPreview],
  );
  useEffect(
    () => () => lastPreview && URL.revokeObjectURL(lastPreview),
    [lastPreview],
  );

  const pollTask = useCallback(async (taskID) => {
    const response = await API.get(`/v1/videos/${encodeURIComponent(taskID)}`, {
      skipErrorHandler: true,
    });
    return response.data;
  }, []);

  const mergeTask = useCallback(
    (nextTask, { select = false } = {}) => {
      const normalized = normalizeVideoTask(nextTask);
      if (!normalized) return;
      setTasks((previous) => {
        const existingIndex = previous.findIndex(
          (item) => item.id === normalized.id,
        );
        if (existingIndex < 0) return [normalized, ...previous];
        const merged = { ...previous[existingIndex], ...normalized };
        return previous.map((item, index) =>
          index === existingIndex ? merged : item,
        );
      });
      if (select) {
        setSelectedTaskID(normalized.id);
        localStorage.setItem(storageKey, normalized.id);
      }
    },
    [storageKey],
  );

  const loadTaskHistory = useCallback(async () => {
    if (!enabled || !userId) return;
    setHistoryLoading(true);
    try {
      const response = await API.get(
        `${API_ENDPOINTS.VIDEO_TASKS}?p=1&page_size=20&action=minimaxH3Generate`,
        { skipErrorHandler: true, disableDuplicate: true },
      );
      const payload = response.data?.success ? response.data.data : null;
      const history = (payload?.items || [])
        .map(normalizeVideoTask)
        .filter((item) => {
          if (!item) return false;
          const itemModel = item.model || item.properties?.origin_model_name;
          return itemModel === model;
        });
      const savedTaskID = localStorage.getItem(storageKey);
      setTasks(history);
      const restoredID = history.some((item) => item.id === savedTaskID)
        ? savedTaskID
        : history[0]?.id || '';
      setSelectedTaskID(restoredID);
      if (restoredID) localStorage.setItem(storageKey, restoredID);
    } catch (error) {
      setPollError(getErrorMessage(error, t('查询视频任务失败')));
    } finally {
      setHistoryLoading(false);
    }
  }, [enabled, model, storageKey, t, userId]);

  useEffect(() => {
    loadTaskHistory();
  }, [loadTaskHistory]);

  useEffect(() => {
    if (!enabled) return undefined;

    let cancelled = false;
    let timer = null;
    const refresh = async () => {
      const pendingTasks = tasksRef.current.filter((item) =>
        activeStatuses.has(item.status),
      );
      if (pendingTasks.length === 0) return;
      try {
        const results = await Promise.allSettled(
          pendingTasks.map((item) => pollTask(item.id)),
        );
        if (cancelled) return;
        let failed = false;
        results.forEach((result) => {
          if (result.status === 'fulfilled') mergeTask(result.value);
          else failed = true;
        });
        setPollError(failed ? t('查询视频任务失败') : '');
      } catch (error) {
        if (cancelled) return;
        const message = getErrorMessage(error, t('查询视频任务失败'));
        setPollError(message);
      }
      if (
        !cancelled &&
        tasksRef.current.some((item) => activeStatuses.has(item.status))
      ) {
        timer = window.setTimeout(refresh, 5000);
      }
    };

    refresh();
    return () => {
      cancelled = true;
      if (timer) window.clearTimeout(timer);
    };
  }, [enabled, historyLoading, mergeTask, pollTask, selectedTaskID, t]);

  const buildSubmissionForm = () => {
    const formData = new FormData();
    formData.append('model', model || MINIMAX_H3_MODEL);
    formData.append('prompt', prompt.trim());
    const effectiveTaskType = isRef2va ? 'ref2va' : taskType;
    formData.append('task', effectiveTaskType);
    formData.append('aspect_ratio', aspectRatio);
    if (seed.trim()) formData.append('seed', seed.trim());
    if (group) formData.append('group', group);
    if (!isRef2va && taskType === 'fl2va') {
      if (firstFrame) formData.append('first_frame', firstFrame);
      if (lastFrame) formData.append('last_frame', lastFrame);
    }
    if (isRef2va && referenceVideo) {
      formData.append('reference_video', referenceVideo);
      if (startTimeSeconds.trim()) formData.append('start_time_seconds', startTimeSeconds.trim());
    }
    return formData;
  };

  const handleSubmit = async () => {
    if (!enabled || !prompt.trim() || submitting) return;
    setSubmitting(true);
    setPollError('');
    try {
      const response = await API.post(
        API_ENDPOINTS.VIDEO_GENERATIONS,
        buildSubmissionForm(),
        { skipErrorHandler: true },
      );
      const createdTask = response.data;
      if (!createdTask?.id) {
        throw new Error(t('服务器未返回视频任务编号'));
      }
      mergeTask(createdTask, { select: true });
    } catch (error) {
      Toast.error(getErrorMessage(error, t('提交视频任务失败')));
    } finally {
      setSubmitting(false);
    }
  };

  const handleNewTask = () => {
    localStorage.removeItem(storageKey);
    setSelectedTaskID('');
    setPollError('');
  };

  const handleSelectTask = (taskID) => {
    setSelectedTaskID(taskID);
    localStorage.setItem(storageKey, taskID);
    setPollError('');
  };

  const isActive = submitting || (task && !terminalStatuses.has(task.status));
  const minimaxH3VideoURL =
    MINIMAX_H3_MODELS.includes(task?.model) && typeof task?.content?.url === 'string'
      ? task.content.url.trim()
      : '';
  const videoContentURL =
    minimaxH3VideoURL ||
    (task?.id ? `/v1/videos/${encodeURIComponent(task.id)}/content` : '');
  const videoDownloadURL = minimaxH3VideoURL
    ? minimaxH3VideoURL
    : `${videoContentURL}?download=1`;
  const videoShareURL = useMemo(() => {
    if (!videoContentURL) return '';
    try {
      return new URL(videoContentURL, window.location.origin).toString();
    } catch {
      return videoContentURL;
    }
  }, [videoContentURL]);
  const handleCopyVideoURL = async () => {
    if (!videoShareURL) return;
    if (await copy(videoShareURL)) {
      Toast.success(t('复制成功'));
      return;
    }
    Toast.error(t('复制失败，请手动复制'));
  };

  return {
    task,
    model,
    prompt,
    setPrompt,
    taskType,
    setTaskType,
    isRef2va,
    aspectRatio,
    setAspectRatio,
    seed,
    setSeed,
    firstFrame,
    setFirstFrame,
    lastFrame,
    setLastFrame,
    referenceVideo,
    setReferenceVideo,
    startTimeSeconds,
    setStartTimeSeconds,
    firstPreview,
    lastPreview,
    tasks,
    selectedTaskID,
    historyLoading,
    loadTaskHistory,
    handleSelectTask,
    submitting,
    pollError,
    isActive,
    minimaxH3VideoURL,
    videoContentURL,
    videoDownloadURL,
    videoShareURL,
    handleSubmit,
    handleNewTask,
    handleCopyVideoURL,
  };
};

export const MiniMaxH3VideoForm = ({ controller, compact = false }) => {
  const { t } = useTranslation();
  const {
    prompt,
    setPrompt,
    taskType,
    setTaskType,
    isRef2va,
    aspectRatio,
    setAspectRatio,
    seed,
    setSeed,
    firstFrame,
    setFirstFrame,
    lastFrame,
    setLastFrame,
    referenceVideo,
    setReferenceVideo,
    startTimeSeconds,
    setStartTimeSeconds,
    firstPreview,
    lastPreview,
    submitting,
    isActive,
    handleSubmit,
  } = controller;

  return (
    <div
      className={`playground-v2-video-form${compact ? ' playground-v2-video-form-compact playground-v2-settings-section' : ''}`}
    >
      <div className='playground-v2-video-model-note'>
        <Video size={15} />
        <span>{isRef2va ? MINIMAX_H3_REF2VA_MODEL : MINIMAX_H3_MODEL}</span>
      </div>
      {!isRef2va && (
        <div className='playground-v2-field'>
          <label className='playground-v2-field-label'>{t('Task type')}</label>
          <select className='playground-v2-text-input' value={taskType} disabled={isActive} onChange={(event) => setTaskType(event.target.value)}>
            <option value='t2va'>t2va</option>
            <option value='fl2va'>fl2va</option>
          </select>
        </div>
      )}
      <div className='playground-v2-field'>
        <label className='playground-v2-field-label'>{t('Aspect ratio')}</label>
        <select className='playground-v2-text-input' value={aspectRatio} disabled={isActive} onChange={(event) => setAspectRatio(event.target.value)}>
          <option value='16:9'>16:9</option>
          <option value='9:16'>9:16</option>
          <option value='1:1'>1:1</option>
          <option value='4:3'>4:3</option>
          <option value='3:4'>3:4</option>
          <option value='auto'>auto</option>
        </select>
      </div>
      <div className='playground-v2-field'>
        <label className='playground-v2-field-label'>{t('Random seed')}</label>
        <input className='playground-v2-text-input' value={seed} disabled={isActive} placeholder='default' onChange={(event) => setSeed(event.target.value)} />
      </div>
      <div className='playground-v2-field'>
        <label className='playground-v2-field-label'>{t('视频描述')}</label>
        <textarea
          className='playground-v2-video-prompt'
          rows={5}
          value={prompt}
          disabled={isActive}
          placeholder={t('描述希望生成的视频内容和运动过程...')}
          onChange={(event) => setPrompt(event.target.value)}
        />
      </div>

      {!isRef2va && taskType === 'fl2va' && <div className='playground-v2-frame-grid'>
        <FramePicker
          label={t('首帧（可选）')}
          file={firstFrame}
          previewUrl={firstPreview}
          disabled={isActive}
          onChange={setFirstFrame}
          onClear={() => setFirstFrame(null)}
        />
        <FramePicker
          label={t('尾帧（可选）')}
          file={lastFrame}
          previewUrl={lastPreview}
          disabled={isActive}
          onChange={setLastFrame}
          onClear={() => setLastFrame(null)}
        />
      </div>}
      {isRef2va && (
        <div className='playground-v2-reference-video-field'>
          <div className='playground-v2-field-label'>{t('Reference video (MP4)')}</div>
          <label className='playground-v2-reference-video-picker'>
            <Video size={20} />
            <span>{referenceVideo ? referenceVideo.name : t('Choose reference video')}</span>
            <small>{t('MP4 only, up to 512 MB')}</small>
            <input
              type='file'
              accept='video/mp4'
              disabled={isActive}
              onChange={(event) => setReferenceVideo(event.target.files?.[0] || null)}
            />
          </label>
          <label className='playground-v2-field-label'>{t('Reference start time (seconds)')}</label>
          <input className='playground-v2-text-input' type='number' min='0' step='0.1' value={startTimeSeconds} disabled={isActive} onChange={(event) => setStartTimeSeconds(event.target.value)} />
        </div>
      )}

      <div className='playground-v2-video-actions'>
        <button
          type='button'
          className='playground-v2-primary-command'
          disabled={!prompt.trim() || (isRef2va && !referenceVideo) || isActive}
          onClick={handleSubmit}
        >
          {submitting ? (
            <LoaderCircle className='playground-v2-spin' size={16} />
          ) : (
            <Send size={16} />
          )}
          {submitting ? t('正在上传并提交') : t('生成视频')}
        </button>
        <span>{t('任务提交后将在服务器队列中等待处理。')}</span>
      </div>
    </div>
  );
};

const VideoTaskHistory = ({
  tasks,
  selectedTaskID,
  loading,
  onRefresh,
  onSelect,
}) => {
  const { t } = useTranslation();
  const statusLabels = {
    queued: t('排队中'),
    in_progress: t('生成中'),
    completed: t('生成完成'),
    failed: t('生成失败'),
  };

  return (
    <section className='playground-v2-video-history'>
      <div className='playground-v2-video-history-header'>
        <div>
          <ListVideo size={16} />
          <strong>{t('任务记录')}</strong>
          <span>{tasks.length}</span>
        </div>
        <button
          type='button'
          className='playground-v2-icon-button'
          onClick={onRefresh}
          disabled={loading}
          aria-label={t('刷新列表')}
          title={t('刷新列表')}
        >
          <RefreshCw
            className={loading ? 'playground-v2-spin' : ''}
            size={15}
          />
        </button>
      </div>
      {tasks.length > 0 ? (
        <div className='playground-v2-video-history-list'>
          {tasks.map((item) => (
            <button
              type='button'
              key={item.id}
              className='playground-v2-video-history-item'
              data-selected={item.id === selectedTaskID}
              onClick={() => onSelect(item.id)}
            >
              <span className='playground-v2-video-history-item-main'>
                <span
                  className='playground-v2-video-history-status'
                  data-status={item.status}
                >
                  {statusLabels[item.status] || item.status}
                </span>
                <code>{item.id}</code>
              </span>
              <span className='playground-v2-video-history-item-meta'>
                {activeStatuses.has(item.status)
                  ? `${Math.round(item.progress || 0)}%`
                  : formatTaskTime(item.submit_time || item.created_at)}
              </span>
            </button>
          ))}
        </div>
      ) : (
        <div className='playground-v2-video-history-empty'>
          {loading ? t('加载中...') : t('当前筛选条件下没有匹配的任务记录。')}
        </div>
      )}
    </section>
  );
};

const VideoGenerationArea = ({ controller, styleState, onToggleSettings }) => {
  const { t } = useTranslation();
  const {
    task,
    model,
    pollError,
    isActive,
    minimaxH3VideoURL,
    videoContentURL,
    videoDownloadURL,
    videoShareURL,
    handleNewTask,
    handleCopyVideoURL,
    tasks,
    selectedTaskID,
    historyLoading,
    loadTaskHistory,
    handleSelectTask,
  } = controller;
  const statusLabel =
    {
      queued: t('排队中'),
      in_progress: t('生成中'),
      completed: t('生成完成'),
      failed: t('生成失败'),
    }[task?.status] || t('等待提交');

  return (
    <div className='playground-v2-chat playground-v2-video-workspace h-full'>
      <div className='playground-v2-chat-header'>
        <div className='playground-v2-chat-header-main'>
          <span className='playground-v2-chat-icon'>
            <Film size={16} />
          </span>
          <h2 className='playground-v2-panel-title'>{t('AI 视频')}</h2>
          <span className='playground-v2-outline-pill'>{model || MINIMAX_H3_MODEL}</span>
        </div>
        {styleState.isMobile && (
          <button
            type='button'
            className='playground-v2-icon-button playground-v2-hidden-desktop'
            onClick={onToggleSettings}
            aria-label={t('打开设置')}
            title={t('打开设置')}
          >
            <Settings2 size={16} />
          </button>
        )}
      </div>

      <div className='playground-v2-video-body model-settings-scroll'>
        {styleState.isMobile && <MiniMaxH3VideoForm controller={controller} />}

        <VideoTaskHistory
          tasks={tasks}
          selectedTaskID={selectedTaskID}
          loading={historyLoading}
          onRefresh={loadTaskHistory}
          onSelect={handleSelectTask}
        />

        <div className='playground-v2-video-result'>
          <div className='playground-v2-video-result-header'>
            <div>
              <span
                className='playground-v2-video-status'
                data-status={task?.status || 'idle'}
              >
                {isActive && (
                  <LoaderCircle className='playground-v2-spin' size={14} />
                )}
                {statusLabel}
              </span>
              {task?.id && <code>{task.id}</code>}
            </div>
            {task && terminalStatuses.has(task.status) && (
              <button
                type='button'
                className='playground-v2-secondary-command'
                onClick={handleNewTask}
              >
                <RotateCcw size={15} />
                {t('新建任务')}
              </button>
            )}
          </div>

          {task && !terminalStatuses.has(task.status) && (
            <div className='playground-v2-video-progress'>
              <span
                style={{
                  width: `${Math.max(2, Math.min(100, task.progress || 0))}%`,
                }}
              />
            </div>
          )}

          {pollError && (
            <div className='playground-v2-video-error'>{pollError}</div>
          )}
          {task?.status === 'failed' && (
            <div className='playground-v2-video-error'>
              {task.error?.message || t('视频生成失败')}
            </div>
          )}

          {task?.status === 'completed' ? (
            <div className='playground-v2-video-player-wrap'>
              <video
                controls
                playsInline
                preload='metadata'
                src={videoContentURL}
              />
              <div className='playground-v2-video-url-block'>
                <span className='playground-v2-field-label'>
                  {t('URL链接')}
                </span>
                <div className='playground-v2-video-url-row'>
                  <input
                    type='text'
                    readOnly
                    value={videoShareURL}
                    aria-label={t('URL链接')}
                    onFocus={(event) => event.target.select()}
                  />
                  <button
                    type='button'
                    className='playground-v2-secondary-command'
                    onClick={handleCopyVideoURL}
                  >
                    <Copy size={15} />
                    {t('复制链接')}
                  </button>
                </div>
              </div>
              <a
                className='playground-v2-primary-command'
                href={videoDownloadURL}
                download={
                  minimaxH3VideoURL && task?.id ? `${task.id}.mp4` : undefined
                }
              >
                <Download size={16} />
                {t('下载视频')}
              </a>
            </div>
          ) : !task ? (
            <div className='playground-v2-video-empty'>
              <Play size={28} />
              <span>{t('生成完成后，视频会显示在这里。')}</span>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
};

export default VideoGenerationArea;
