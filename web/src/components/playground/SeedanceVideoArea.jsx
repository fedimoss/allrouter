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
  useId,
  useMemo,
  useRef,
  useState,
} from 'react';
import { Toast } from '@douyinfe/semi-ui';
import {
  Copy,
  Download,
  Film,
  ImagePlus,
  ListVideo,
  LoaderCircle,
  Music,
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
import { API_ENDPOINTS } from '../../constants/playground.constants';

// 逻辑：Seedance 独立的操练场面板与任务状态机，不复用 H3 的控制器。
// 注意：参考素材只在当前会话展示，刷新后依赖后端历史任务。
const terminalStatuses = new Set(['completed', 'failed']);
const activeStatuses = new Set(['queued', 'in_progress']);
const seedancePlatforms = new Set(['54', '45']);

// 兼容：能力接口异常时按 2.0 保守回退，避免放开发送不支持的参数。
const defaultSeedanceCapabilities = {
  panel: 'seedance',
  family: '2.0',
  max_images: 9,
  max_videos: 3,
  max_audios: 3,
  max_video_seconds: 15,
  max_audio_seconds: 15,
  allow_audio_only: false,
  min_duration: 4,
  max_duration: 15,
  resolutions: ['480p', '720p', '1080p', '4k'],
  output_formats: [],
};

const resolveCapabilities = (capabilities) => {
  // 逻辑：后端能力优先，缺失字段用默认值补齐，数组字段单独兜底。
  const merged = { ...defaultSeedanceCapabilities, ...(capabilities || {}) };
  if (!Array.isArray(merged.resolutions) || merged.resolutions.length === 0) {
    merged.resolutions = defaultSeedanceCapabilities.resolutions;
  }
  if (!Array.isArray(merged.output_formats)) {
    merged.output_formats = [];
  }
  return merged;
};

const readMediaDuration = (file, kind) =>
  new Promise((resolve) => {
    // 逻辑：用浏览器 loadedmetadata 预读本地素材时长，失败返回 null。
    // 注意：5 秒超时和 onerror 都按“未知时长”处理，最终由后端/上游校验。
    const element = document.createElement(kind);
    const objectURL = URL.createObjectURL(file);
    let timer = null;
    let finished = false;
    const finish = (duration) => {
      if (finished) return;
      finished = true;
      if (timer) clearTimeout(timer);
      element.removeAttribute('src');
      element.load();
      URL.revokeObjectURL(objectURL);
      resolve(duration);
    };
    timer = setTimeout(() => finish(null), 5000);
    element.preload = 'metadata';
    element.onloadedmetadata = () => {
      const duration = Number(element.duration);
      finish(Number.isFinite(duration) && duration > 0 ? duration : null);
    };
    element.onerror = () => finish(null);
    element.src = objectURL;
  });

const isVideoURL = (value) =>
  typeof value === 'string' &&
  (value.startsWith('http://') ||
    value.startsWith('https://') ||
    value.startsWith('data:') ||
    value.startsWith('/'));

const normalizeVideoTask = (source) => {
  if (!source) return null;
  const taskID =
    source.task_id || (typeof source.id === 'string' ? source.id : '');
  if (!taskID) return null;

  const model = source.model || source.properties?.origin_model_name || '';
  const rawStatus = String(source.status || '').toLowerCase();
  // 兼容：内部任务状态、上游状态和旧接口状态统一映射到面板状态。
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

  let dataPrompt = '';
  if (source.data && typeof source.data === 'object') {
    dataPrompt = source.data.prompt || '';
  } else if (typeof source.data === 'string') {
    try {
      dataPrompt = JSON.parse(source.data)?.prompt || '';
    } catch {
      dataPrompt = '';
    }
  }

  const promptCandidates = [
    // 兼容：查询接口、历史任务和旧数据可能把提示词放在不同字段。
    source.prompt,
    source.metadata?.prompt,
    source.properties?.input,
    source.properties?.prompt,
    dataPrompt,
  ];
  const prompt =
    promptCandidates.find(
      (value) => typeof value === 'string' && value.trim(),
    ) || '';

  const urlCandidates =
    // 注意：失败任务不展示旧结果 URL，避免用户误以为视频可用。
    status === 'failed'
      ? []
      : [
          source.metadata?.url,
          source.url,
          source.result_url,
          source.content?.video_url,
        ];
  const publicURL = urlCandidates.find(isVideoURL) || '';
  const progressValue = Number.parseFloat(
    String(source.progress || '').replace('%', ''),
  );

  return {
    ...source,
    id: taskID,
    task_id: taskID,
    model,
    prompt,
    url: publicURL,
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

const formatFileSize = (bytes) => {
  const value = Number(bytes) || 0;
  if (value < 1024 * 1024) return `${Math.max(1, Math.round(value / 1024))} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
};

const getErrorMessage = (error, fallback) =>
  error?.response?.data?.error?.message ||
  error?.response?.data?.message ||
  error?.message ||
  fallback;

const buildTaskReferences = (images, videos, audios) => [
  // 逻辑：保存本次提交的 File 引用，供当前会话结果区预览参考素材。
  ...images.map((file, index) => ({
    id: `image-${index}-${file.name}-${file.lastModified}`,
    kind: 'image',
    name: file.name,
    file,
  })),
  ...videos.map((file, index) => ({
    id: `video-${index}-${file.name}-${file.lastModified}`,
    kind: 'video',
    name: file.name,
    file,
  })),
  ...audios.map((file, index) => ({
    id: `audio-${index}-${file.name}-${file.lastModified}`,
    kind: 'audio',
    name: file.name,
    file,
  })),
];

const ToggleSwitch = ({ checked, disabled, onChange }) => (
  <button
    type='button'
    className='playground-v2-switch'
    data-active={checked}
    data-disabled={disabled}
    disabled={disabled}
    onClick={() => !disabled && onChange(!checked)}
    aria-pressed={checked}
  />
);

const ImagePicker = ({ files, max, disabled, onChange }) => {
  const { t } = useTranslation();
  const inputId = useId();
  const limit = Number(max) > 0 ? Number(max) : 9;
  const previews = useMemo(
    // 注意：objectURL 必须随 files 变化或卸载时释放，避免内存泄漏。
    () => files.map((file) => URL.createObjectURL(file)),
    [files],
  );

  useEffect(
    () => () => previews.forEach((preview) => URL.revokeObjectURL(preview)),
    [previews],
  );

  return (
    <div className='playground-v2-field'>
      <label className='playground-v2-field-label'>
        {t('Reference images')}
        <span className='playground-v2-field-count'>
          {files.length}/{limit}
        </span>
      </label>
      <div className='playground-v2-seedance-image-grid'>
        {files.map((file, index) => (
          <div
            className='playground-v2-seedance-image-card'
            key={file.name + index}
          >
            <img src={previews[index]} alt={file.name} />
            <button
              type='button'
              className='playground-v2-seedance-media-remove'
              onClick={() => onChange(files.filter((_, i) => i !== index))}
              disabled={disabled}
              aria-label={t('Remove')}
              title={t('Remove')}
            >
              <X size={14} />
            </button>
          </div>
        ))}
        {files.length < limit && (
          <label
            className='playground-v2-seedance-image-add'
            htmlFor={inputId}
            aria-disabled={disabled}
            onClick={(event) => {
              if (disabled) event.preventDefault();
            }}
          >
            <ImagePlus size={20} />
            <span>{t('Add images')}</span>
          </label>
        )}
      </div>
      <div className='playground-v2-field-hint'>
        {t('JPEG/PNG/WebP, up to 30 MB each')}
      </div>
      <input
        id={inputId}
        className='playground-v2-seedance-file-input'
        type='file'
        accept='image/jpeg,image/png,image/webp'
        multiple
        disabled={disabled}
        onChange={(event) => {
          const selected = Array.from(event.target.files || []);
          const next = [...files, ...selected];
          if (next.length > limit) {
            Toast.warning(t('At most {{count}} images', { count: limit }));
          }
          onChange(next.slice(0, limit));
          event.target.value = '';
        }}
      />
    </div>
  );
};

const FilePicker = ({
  label,
  hint,
  accept,
  files,
  max,
  disabled,
  icon: Icon,
  onChange,
}) => {
  const { t } = useTranslation();
  const inputId = useId();
  const limit = Number(max) > 0 ? Number(max) : 3;

  return (
    <div className='playground-v2-field'>
      <label className='playground-v2-field-label'>
        {label}
        <span className='playground-v2-field-count'>
          {files.length}/{limit}
        </span>
      </label>
      {files.length > 0 && (
        <div className='playground-v2-seedance-file-list'>
          {files.map((file, index) => (
            <div
              className='playground-v2-seedance-file-row'
              key={file.name + index}
            >
              <Icon size={16} />
              <span className='playground-v2-seedance-file-name'>
                {file.name}
              </span>
              <span className='playground-v2-seedance-file-size'>
                {formatFileSize(file.size)}
              </span>
              <button
                type='button'
                className='playground-v2-seedance-media-remove'
                onClick={() => onChange(files.filter((_, i) => i !== index))}
                disabled={disabled}
                aria-label={t('Remove')}
                title={t('Remove')}
              >
                <X size={14} />
              </button>
            </div>
          ))}
        </div>
      )}
      {files.length < limit && (
        /* 逻辑：用原生 label 触发隐藏 input，避免脚本 click 被浏览器拦截。 */
        <label
          className='playground-v2-seedance-file-add'
          htmlFor={inputId}
          aria-disabled={disabled}
          onClick={(event) => {
            if (disabled) event.preventDefault();
          }}
        >
          <Icon size={18} />
          <span>{t('Add files')}</span>
        </label>
      )}
      <div className='playground-v2-field-hint'>{hint}</div>
      <input
        id={inputId}
        className='playground-v2-seedance-file-input'
        type='file'
        accept={accept}
        multiple
        disabled={disabled}
        onChange={(event) => {
          const selected = Array.from(event.target.files || []);
          const next = [...files, ...selected];
          if (next.length > limit) {
            Toast.warning(t('Too many files'));
          }
          onChange(next.slice(0, limit));
          event.target.value = '';
        }}
      />
    </div>
  );
};

export const useSeedanceVideoGeneration = ({
  enabled,
  group,
  userId,
  model,
  capabilities,
}) => {
  // 逻辑：该 hook 负责表单状态、历史加载、轮询合并、校验和提交。
  // 注意：H3 使用独立 hook，这里不要引入 H3 的任务字段或清理逻辑。
  const { t } = useTranslation();
  const resolvedCapabilities = useMemo(
    () => resolveCapabilities(capabilities),
    [capabilities],
  );
  const storageKey = `seedance_playground_task_${model || 'seedance'}_${userId || 'unknown'}`;
  const [prompt, setPrompt] = useState('');
  const [ratio, setRatio] = useState('adaptive');
  const [resolution, setResolution] = useState('720p');
  const [duration, setDuration] = useState('5');
  const [generateAudio, setGenerateAudio] = useState(true);
  const [outputFormat, setOutputFormat] = useState('mp4');
  const [images, setImages] = useState([]);
  const [videos, setVideos] = useState([]);
  const [audios, setAudios] = useState([]);
  const [tasks, setTasks] = useState([]);
  const [selectedTaskID, setSelectedTaskID] = useState('');
  const [historyLoading, setHistoryLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [pollError, setPollError] = useState('');
  const [durationCheck, setDurationCheck] = useState({
    videoSeconds: 0,
    audioSeconds: 0,
    unknownVideo: false,
    unknownAudio: false,
    loading: false,
  });
  const tasksRef = useRef([]);

  useEffect(() => {
    // 逻辑：切换模型时清空表单和素材，避免把旧模型参数带到新模型。
    setPrompt('');
    setRatio('adaptive');
    setResolution('720p');
    setDuration('5');
    setGenerateAudio(true);
    setOutputFormat('mp4');
    setImages([]);
    setVideos([]);
    setAudios([]);
  }, [model]);

  useEffect(() => {
    tasksRef.current = tasks;
  }, [tasks]);

  useEffect(() => {
    // 逻辑：能力变化后收敛非法选项，保证 UI 不会提交超出模型范围的值。
    if (!resolvedCapabilities.resolutions.includes(resolution)) {
      setResolution(resolvedCapabilities.resolutions[0] || '720p');
    }
    if (duration !== '-1') {
      const numericDuration = Number(duration);
      if (
        !Number.isFinite(numericDuration) ||
        numericDuration < resolvedCapabilities.min_duration ||
        numericDuration > resolvedCapabilities.max_duration
      ) {
        setDuration(String(Math.min(5, resolvedCapabilities.max_duration)));
      }
    }
    if (
      resolvedCapabilities.output_formats.length > 0 &&
      !resolvedCapabilities.output_formats.includes(outputFormat)
    ) {
      setOutputFormat(resolvedCapabilities.output_formats[0]);
    }
  }, [duration, outputFormat, resolution, resolvedCapabilities]);

  useEffect(() => {
    // 逻辑：本地读取参考视频/音频总时长；读取失败的素材只提示、不阻断。
    if (videos.length === 0 && audios.length === 0) {
      setDurationCheck({
        videoSeconds: 0,
        audioSeconds: 0,
        unknownVideo: false,
        unknownAudio: false,
        loading: false,
      });
      return undefined;
    }
    let cancelled = false;
    setDurationCheck((previous) => ({ ...previous, loading: true }));
    Promise.all([
      Promise.all(videos.map((file) => readMediaDuration(file, 'video'))),
      Promise.all(audios.map((file) => readMediaDuration(file, 'audio'))),
    ]).then(([videoDurations, audioDurations]) => {
      if (cancelled) return;
      const knownVideos = videoDurations.filter((value) => value !== null);
      const knownAudios = audioDurations.filter((value) => value !== null);
      setDurationCheck({
        videoSeconds: knownVideos.reduce((sum, value) => sum + value, 0),
        audioSeconds: knownAudios.reduce((sum, value) => sum + value, 0),
        unknownVideo: knownVideos.length !== videoDurations.length,
        unknownAudio: knownAudios.length !== audioDurations.length,
        loading: false,
      });
    });
    return () => {
      cancelled = true;
    };
  }, [audios, videos]);

  const task = useMemo(
    () => tasks.find((item) => item.id === selectedTaskID) || null,
    [selectedTaskID, tasks],
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
        const merged = {
          ...previous[existingIndex],
          ...normalized,
          // 注意：轮询结果可能缺 prompt，必须保留提交时写入的值。
          prompt: normalized.prompt || previous[existingIndex].prompt || '',
        };
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
        `${API_ENDPOINTS.VIDEO_TASKS}?p=1&page_size=20&action=generate`,
        { skipErrorHandler: true, disableDuplicate: true },
      );
      const payload = response.data?.success ? response.data.data : null;
      const history = (payload?.items || [])
        .map(normalizeVideoTask)
        .filter((item) => {
          // 逻辑：只保留当前模型和 Seedance 平台的任务，避免混入其他渠道。
          if (!item || item.model !== model) return false;
          return !item.platform || seedancePlatforms.has(String(item.platform));
        })
        .sort(
          (left, right) =>
            Number(right.created_at || 0) - Number(left.created_at || 0),
        );
      const savedTaskID = localStorage.getItem(storageKey);
      setTasks(history);
      const restoredID = history.some((item) => item.id === savedTaskID)
        ? savedTaskID
        : history[0]?.id || '';
      setSelectedTaskID(restoredID);
      if (restoredID) localStorage.setItem(storageKey, restoredID);
    } catch (error) {
      setPollError(getErrorMessage(error, t('Failed to query video task')));
    } finally {
      setHistoryLoading(false);
    }
  }, [enabled, model, storageKey, t, userId]);

  useEffect(() => {
    loadTaskHistory();
  }, [loadTaskHistory]);

  useEffect(() => {
    if (!enabled) return undefined;

    // 逻辑：仅轮询活跃任务，5 秒一轮；组件卸载或任务终态后停止。
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
        setPollError(failed ? t('Failed to query video task') : '');
      } catch (error) {
        if (cancelled) return;
        setPollError(getErrorMessage(error, t('Failed to query video task')));
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

  const videoDurationExceeded =
    durationCheck.videoSeconds > resolvedCapabilities.max_video_seconds;
  const audioDurationExceeded =
    durationCheck.audioSeconds > resolvedCapabilities.max_audio_seconds;
  const durationBlocked = videoDurationExceeded || audioDurationExceeded;

  const buildSubmissionForm = () => {
    // 逻辑：Playground 使用 multipart，参考文件由后端落盘后转成公网 URL。
    const formData = new FormData();
    formData.append('model', model);
    formData.append('prompt', prompt.trim());
    if (group) formData.append('group', group);
    formData.append('ratio', ratio);
    formData.append('resolution', resolution);
    formData.append('duration', duration);
    formData.append('generate_audio', String(generateAudio));
    if (resolvedCapabilities.output_formats.includes(outputFormat)) {
      formData.append('output_format', outputFormat);
    }
    images.forEach((file) => formData.append('reference_images', file));
    videos.forEach((file) => formData.append('reference_videos', file));
    audios.forEach((file) => formData.append('reference_audios', file));
    return formData;
  };

  const handleSubmit = async () => {
    if (!enabled || !prompt.trim() || submitting) return;
    if (
      !resolvedCapabilities.allow_audio_only &&
      audios.length > 0 &&
      images.length === 0 &&
      videos.length === 0
    ) {
      Toast.error(t('Audio-only requests are not supported'));
      return;
    }
    if (durationBlocked) {
      Toast.error(t('Reference media duration exceeds the model limit'));
      return;
    }
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
        throw new Error(t('Server did not return a video task id'));
      }
      mergeTask(
        {
          ...createdTask,
          prompt: prompt.trim(),
          // 注意：references 只存在前端内存，刷新页面后不会恢复。
          references: buildTaskReferences(images, videos, audios),
        },
        { select: true },
      );
    } catch (error) {
      Toast.error(getErrorMessage(error, t('Failed to submit video task')));
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
  // 逻辑：优先使用持久化结果/上游 URL，缺失时回退到网关内容代理。
  const directVideoURL =
    (typeof task?.url === 'string' && task.url.trim()) ||
    (typeof task?.result_url === 'string' && task.result_url.trim()) ||
    (typeof task?.metadata?.url === 'string' && task.metadata.url.trim()) ||
    '';
  const videoContentURL =
    directVideoURL ||
    (task?.id ? `/v1/videos/${encodeURIComponent(task.id)}/content` : '');
  const videoDownloadURL = directVideoURL
    ? directVideoURL
    : videoContentURL
      ? `${videoContentURL}?download=1`
      : '';
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
      Toast.success(t('Copied successfully'));
      return;
    }
    Toast.error(t('Copy failed, please copy manually'));
  };

  const handleCopyVideoPrompt = async () => {
    if (!task?.prompt) return;
    if (await copy(task.prompt)) {
      Toast.success(t('Copied successfully'));
      return;
    }
    Toast.error(t('Copy failed, please copy manually'));
  };

  return {
    task,
    model,
    prompt,
    setPrompt,
    ratio,
    setRatio,
    resolution,
    setResolution,
    duration,
    setDuration,
    generateAudio,
    setGenerateAudio,
    outputFormat,
    setOutputFormat,
    images,
    setImages,
    videos,
    setVideos,
    audios,
    setAudios,
    tasks,
    selectedTaskID,
    historyLoading,
    loadTaskHistory,
    handleSelectTask,
    submitting,
    pollError,
    isActive,
    videoContentURL,
    videoDownloadURL,
    videoShareURL,
    handleSubmit,
    handleNewTask,
    handleCopyVideoURL,
    handleCopyVideoPrompt,
    capabilities: resolvedCapabilities,
    durationCheck,
    videoDurationExceeded,
    audioDurationExceeded,
    durationBlocked,
  };
};

// 逻辑：桌面端在左侧设置栏以 compact 模式渲染，移动端渲染在结果区顶部。
export const SeedanceVideoForm = ({ controller, compact = false }) => {
  const { t } = useTranslation();
  const {
    model,
    prompt,
    setPrompt,
    ratio,
    setRatio,
    resolution,
    setResolution,
    duration,
    setDuration,
    generateAudio,
    setGenerateAudio,
    outputFormat,
    setOutputFormat,
    images,
    setImages,
    videos,
    setVideos,
    audios,
    setAudios,
    submitting,
    handleSubmit,
    capabilities,
    durationCheck,
    videoDurationExceeded,
    audioDurationExceeded,
    durationBlocked,
  } = controller;

  const resolutionOptions = capabilities.resolutions;
  const durationDisabled = submitting || duration === '-1';

  return (
    <div
      className={`playground-v2-video-form playground-v2-seedance-form${compact ? ' playground-v2-video-form-compact playground-v2-settings-section' : ''}`}
    >
      <div className='playground-v2-video-model-note'>
        <Video size={15} />
        <span>{model}</span>
      </div>

      <div className='playground-v2-field'>
        <label className='playground-v2-field-label'>{t('Video prompt')}</label>
        <textarea
          className='playground-v2-video-prompt playground-v2-seedance-prompt'
          rows={5}
          value={prompt}
          disabled={submitting}
          placeholder={t('Describe the video you want to generate...')}
          onChange={(event) => setPrompt(event.target.value)}
        />
      </div>

      <div className='playground-v2-field'>
        <label className='playground-v2-field-label'>{t('Aspect ratio')}</label>
        <select
          className='playground-v2-text-input'
          value={ratio}
          disabled={submitting}
          onChange={(event) => setRatio(event.target.value)}
        >
          <option value='adaptive'>adaptive</option>
          <option value='16:9'>16:9</option>
          <option value='4:3'>4:3</option>
          <option value='1:1'>1:1</option>
          <option value='3:4'>3:4</option>
          <option value='9:16'>9:16</option>
          <option value='21:9'>21:9</option>
        </select>
      </div>

      <div className='playground-v2-field'>
        <label className='playground-v2-field-label'>{t('Resolution')}</label>
        <select
          className='playground-v2-text-input'
          value={resolution}
          disabled={submitting}
          onChange={(event) => setResolution(event.target.value)}
        >
          {resolutionOptions.map((option) => (
            <option value={option} key={option}>
              {option}
            </option>
          ))}
        </select>
      </div>

      <div className='playground-v2-field'>
        <label className='playground-v2-field-label'>{t('Duration')}</label>
        <div className='playground-v2-seedance-duration-row'>
          <input
            className='playground-v2-text-input'
            type='number'
            min={capabilities.min_duration}
            max={capabilities.max_duration}
            value={duration === '-1' ? '' : duration}
            disabled={durationDisabled}
            placeholder='-1'
            onChange={(event) => setDuration(event.target.value)}
          />
          <label className='playground-v2-seedance-auto-duration'>
            <input
              type='checkbox'
              checked={duration === '-1'}
              disabled={submitting}
              onChange={(event) =>
                setDuration(event.target.checked ? '-1' : '5')
              }
            />
            <span>{t('Auto')}</span>
          </label>
        </div>
        <div className='playground-v2-field-hint'>
          {t('Seconds; use Auto to let the model choose')}
        </div>
      </div>

      <div className='playground-v2-field'>
        <div className='playground-v2-seedance-toggle-row'>
          <span className='playground-v2-field-label'>
            {t('Generate audio')}
          </span>
          <ToggleSwitch
            checked={generateAudio}
            disabled={submitting}
            onChange={setGenerateAudio}
          />
        </div>
      </div>

      {capabilities.output_formats.length > 0 && (
        <div className='playground-v2-field'>
          <label className='playground-v2-field-label'>
            {t('Output format')}
          </label>
          <select
            className='playground-v2-text-input'
            value={outputFormat}
            disabled={submitting}
            onChange={(event) => setOutputFormat(event.target.value)}
          >
            {capabilities.output_formats.map((format) => (
              <option value={format} key={format}>
                {format}
              </option>
            ))}
          </select>
        </div>
      )}

      <ImagePicker
        files={images}
        max={capabilities.max_images}
        disabled={submitting}
        onChange={setImages}
      />

      <FilePicker
        label={t('Reference videos')}
        hint={`${t('MP4/MOV, up to 200 MB each')} · ${t('Reference video total duration must not exceed {{seconds}} seconds', { seconds: capabilities.max_video_seconds })} · ${durationCheck.videoSeconds.toFixed(1)}s`}
        accept='video/mp4,video/quicktime'
        files={videos}
        max={capabilities.max_videos}
        disabled={submitting}
        icon={Film}
        onChange={setVideos}
      />

      <FilePicker
        label={t('Reference audio')}
        hint={`${t('MP3/WAV, up to 15 MB each; requires an image or video')} · ${t('Reference audio total duration must not exceed {{seconds}} seconds', { seconds: capabilities.max_audio_seconds })} · ${durationCheck.audioSeconds.toFixed(1)}s`}
        accept='audio/mpeg,audio/wav,audio/x-wav'
        files={audios}
        max={capabilities.max_audios}
        disabled={submitting}
        icon={Music}
        onChange={setAudios}
      />

      {videoDurationExceeded && (
        <div className='playground-v2-video-error'>
          {t('Reference video duration exceeds the model limit')}
        </div>
      )}
      {audioDurationExceeded && (
        <div className='playground-v2-video-error'>
          {t('Reference audio duration exceeds the model limit')}
        </div>
      )}
      {(durationCheck.unknownVideo || durationCheck.unknownAudio) && (
        <div className='playground-v2-field-hint'>
          {t('Some reference media duration could not be checked locally.')}
        </div>
      )}
      {!capabilities.allow_audio_only &&
        audios.length > 0 &&
        images.length === 0 &&
        videos.length === 0 && (
          <div className='playground-v2-video-error'>
            {t('Audio-only requests are not supported')}
          </div>
        )}

      <div className='playground-v2-video-actions'>
        <button
          type='button'
          className='playground-v2-primary-command'
          disabled={
            !prompt.trim() ||
            submitting ||
            durationBlocked ||
            (!capabilities.allow_audio_only &&
              audios.length > 0 &&
              images.length === 0 &&
              videos.length === 0)
          }
          onClick={handleSubmit}
        >
          {submitting ? (
            <LoaderCircle className='playground-v2-spin' size={16} />
          ) : (
            <Send size={16} />
          )}
          {submitting ? t('Submitting...') : t('Generate video')}
        </button>
        <span>{t('The task will be queued on the server.')}</span>
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
    queued: t('Queued'),
    in_progress: t('Generating'),
    completed: t('Completed'),
    failed: t('Failed'),
  };

  return (
    <section className='playground-v2-video-history'>
      <div className='playground-v2-video-history-header'>
        <div>
          <ListVideo size={16} />
          <strong>{t('Task history')}</strong>
          <span>{tasks.length}</span>
        </div>
        <button
          type='button'
          className='playground-v2-icon-button'
          onClick={onRefresh}
          disabled={loading}
          aria-label={t('Refresh list')}
          title={t('Refresh list')}
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
          {loading ? t('Loading...') : t('No matching task records.')}
        </div>
      )}
    </section>
  );
};

const ReferenceMediaDisplay = ({ references }) => {
  const { t } = useTranslation();
  const previews = useMemo(
    // 注意：历史任务没有 File，只能展示后端可能返回的 url。
    () =>
      (references || []).map((reference) =>
        reference.file
          ? URL.createObjectURL(reference.file)
          : reference.url || '',
      ),
    [references],
  );

  useEffect(
    () => () => previews.forEach((preview) => URL.revokeObjectURL(preview)),
    [previews],
  );

  if (!references?.length) return null;

  return (
    <div className='playground-v2-video-reference-display'>
      <span className='playground-v2-field-label'>{t('Reference media')}</span>
      <div className='playground-v2-video-reference-grid'>
        {references.map((reference, index) => (
          <div
            className='playground-v2-video-reference-item'
            key={reference.id || index}
          >
            {reference.kind === 'image' && (
              <img src={previews[index]} alt={reference.name} />
            )}
            {reference.kind === 'video' && (
              <video controls preload='metadata' src={previews[index]} />
            )}
            {reference.kind === 'audio' && (
              <audio controls src={previews[index]} />
            )}
            <span title={reference.name}>{reference.name}</span>
          </div>
        ))}
      </div>
    </div>
  );
};

const directDownloadName = (task) =>
  task?.url && task?.id ? `${task.id}.mp4` : undefined;

const SeedanceVideoArea = ({ controller, styleState, onToggleSettings }) => {
  const { t } = useTranslation();
  const {
    task,
    model,
    pollError,
    isActive,
    videoContentURL,
    videoDownloadURL,
    videoShareURL,
    handleNewTask,
    handleCopyVideoURL,
    handleCopyVideoPrompt,
    tasks,
    selectedTaskID,
    historyLoading,
    loadTaskHistory,
    handleSelectTask,
  } = controller;
  const statusLabel =
    // 逻辑：结果区在排队、生成中、完成和失败阶段都显示提示词与状态。
    {
      queued: t('Queued'),
      in_progress: t('Generating'),
      completed: t('Completed'),
      failed: t('Failed'),
    }[task?.status] || t('Waiting to submit');

  return (
    <div className='playground-v2-chat playground-v2-video-workspace h-full'>
      <div className='playground-v2-chat-header'>
        <div className='playground-v2-chat-header-main'>
          <span className='playground-v2-chat-icon'>
            <Film size={16} />
          </span>
          <h2 className='playground-v2-panel-title'>{t('AI video')}</h2>
          <span className='playground-v2-outline-pill'>{model}</span>
        </div>
        {styleState.isMobile && (
          <button
            type='button'
            className='playground-v2-icon-button playground-v2-hidden-desktop'
            onClick={onToggleSettings}
            aria-label={t('Open settings')}
            title={t('Open settings')}
          >
            <Settings2 size={16} />
          </button>
        )}
      </div>

      <div className='playground-v2-video-body model-settings-scroll'>
        {styleState.isMobile && <SeedanceVideoForm controller={controller} />}

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
                {t('New task')}
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
              {task.error?.message || t('Video generation failed')}
            </div>
          )}

          {task?.prompt && (
            <div className='playground-v2-video-prompt-display'>
              <div className='playground-v2-video-prompt-header'>
                <span className='playground-v2-field-label'>
                  {t('Video prompt')}
                </span>
                <button
                  type='button'
                  className='playground-v2-secondary-command'
                  onClick={handleCopyVideoPrompt}
                >
                  <Copy size={14} />
                  {t('Copy prompt')}
                </button>
              </div>
              <div>{task.prompt}</div>
            </div>
          )}

          <ReferenceMediaDisplay references={task?.references} />

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
                  {t('URL link')}
                </span>
                <div className='playground-v2-video-url-row'>
                  <input
                    type='text'
                    readOnly
                    value={videoShareURL}
                    aria-label={t('URL link')}
                    onFocus={(event) => event.target.select()}
                  />
                  <button
                    type='button'
                    className='playground-v2-secondary-command'
                    onClick={handleCopyVideoURL}
                  >
                    <Copy size={15} />
                    {t('Copy link')}
                  </button>
                </div>
              </div>
              <a
                className='playground-v2-primary-command'
                href={videoDownloadURL}
                download={directDownloadName(task)}
              >
                <Download size={16} />
                {t('Download video')}
              </a>
            </div>
          ) : !task ? (
            <div className='playground-v2-video-empty'>
              <Play size={28} />
              <span>{t('The generated video will appear here.')}</span>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
};

export default SeedanceVideoArea;
