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
  ImagePlus,
  LoaderCircle,
  Play,
  RotateCcw,
  Send,
  Settings2,
  X,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, copy } from '../../helpers';
import {
  API_ENDPOINTS,
  MINIMAX_H3_MODEL,
} from '../../constants/playground.constants';

const terminalStatuses = new Set(['completed', 'failed']);

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

export const useMiniMaxH3VideoGeneration = ({ enabled, group, userId }) => {
  const { t } = useTranslation();
  const storageKey = `minimax_h3_playground_task_${userId || 'unknown'}`;
  const [prompt, setPrompt] = useState('');
  const [firstFrame, setFirstFrame] = useState(null);
  const [lastFrame, setLastFrame] = useState(null);
  const [task, setTask] = useState(null);
  const [submitting, setSubmitting] = useState(false);
  const [pollError, setPollError] = useState('');

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

  useEffect(() => {
    if (!enabled) return;
    const savedTaskID = localStorage.getItem(storageKey);
    if (savedTaskID) {
      setTask({ id: savedTaskID, status: 'queued', progress: 0 });
    }
  }, [enabled, storageKey]);

  const pollTask = useCallback(async (taskID) => {
    const response = await API.get(`/v1/videos/${encodeURIComponent(taskID)}`, {
      skipErrorHandler: true,
    });
    return response.data;
  }, []);

  useEffect(() => {
    if (!enabled || !task?.id || terminalStatuses.has(task.status)) {
      return undefined;
    }

    let cancelled = false;
    let timer = null;
    const refresh = async () => {
      try {
        const nextTask = await pollTask(task.id);
        if (cancelled) return;
        setTask(nextTask);
        setPollError('');
        if (!terminalStatuses.has(nextTask.status)) {
          timer = window.setTimeout(refresh, 5000);
        }
      } catch (error) {
        if (cancelled) return;
        const message = getErrorMessage(error, t('查询视频任务失败'));
        setPollError(message);
        timer = window.setTimeout(refresh, 5000);
      }
    };

    refresh();
    return () => {
      cancelled = true;
      if (timer) window.clearTimeout(timer);
    };
  }, [enabled, pollTask, t, task?.id, task?.status]);

  const uploadFrames = async () => {
    if (!firstFrame && !lastFrame) return {};
    const formData = new FormData();
    if (firstFrame) formData.append('first_frame', firstFrame);
    if (lastFrame) formData.append('last_frame', lastFrame);
    const response = await API.post(
      API_ENDPOINTS.VIDEO_FRAME_UPLOADS,
      formData,
      {
        skipErrorHandler: true,
      },
    );
    return response.data?.data || {};
  };

  const handleSubmit = async () => {
    if (!enabled || !prompt.trim() || submitting) return;
    setSubmitting(true);
    setPollError('');
    try {
      const frames = await uploadFrames();
      const response = await API.post(
        API_ENDPOINTS.VIDEO_GENERATIONS,
        {
          model: MINIMAX_H3_MODEL,
          group,
          prompt: prompt.trim(),
          ...frames,
        },
        { skipErrorHandler: true },
      );
      const createdTask = response.data;
      if (!createdTask?.id) {
        throw new Error(t('服务器未返回视频任务编号'));
      }
      localStorage.setItem(storageKey, createdTask.id);
      setTask(createdTask);
    } catch (error) {
      Toast.error(getErrorMessage(error, t('提交视频任务失败')));
    } finally {
      setSubmitting(false);
    }
  };

  const handleNewTask = () => {
    localStorage.removeItem(storageKey);
    setTask(null);
    setPollError('');
  };

  const isActive = submitting || (task && !terminalStatuses.has(task.status));
  const minimaxH3VideoURL =
    task?.model === 'MiniMax-H3' && typeof task?.content?.url === 'string'
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
    prompt,
    setPrompt,
    firstFrame,
    setFirstFrame,
    lastFrame,
    setLastFrame,
    firstPreview,
    lastPreview,
    task,
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
    firstFrame,
    setFirstFrame,
    lastFrame,
    setLastFrame,
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

      <div className='playground-v2-frame-grid'>
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
      </div>

      <div className='playground-v2-video-actions'>
        <button
          type='button'
          className='playground-v2-primary-command'
          disabled={!prompt.trim() || isActive}
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

const VideoGenerationArea = ({ controller, styleState, onToggleSettings }) => {
  const { t } = useTranslation();
  const {
    task,
    pollError,
    isActive,
    minimaxH3VideoURL,
    videoContentURL,
    videoDownloadURL,
    videoShareURL,
    handleNewTask,
    handleCopyVideoURL,
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
          <span className='playground-v2-outline-pill'>{MINIMAX_H3_MODEL}</span>
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
