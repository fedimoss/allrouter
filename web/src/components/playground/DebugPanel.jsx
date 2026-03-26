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
import { Toast } from '@douyinfe/semi-ui';
import { ArrowLeft, Bug, Copy, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { copy } from '../../helpers';
import { DEBUG_TABS } from '../../constants/playground.constants';
import CodeViewer from './CodeViewer';

const stringifyContent = (value) => {
  if (!value) {
    return '';
  }

  if (typeof value === 'string') {
    return value;
  }

  try {
    return JSON.stringify(value, null, 2);
  } catch (error) {
    return String(value);
  }
};

const summarizeStreamEvent = (rawEvent, index) => {
  if (rawEvent === '[DONE]') {
    return {
      key: `done-${index}`,
      badge: '[DONE]',
      preview: 'Stream finished',
      raw: rawEvent,
    };
  }

  try {
    const parsed = JSON.parse(rawEvent);
    const delta = parsed?.choices?.[0]?.delta || {};
    const finishReason = parsed?.choices?.[0]?.finish_reason;
    const preview =
      delta.reasoning_content ||
      delta.reasoning ||
      delta.content ||
      finishReason ||
      parsed?.object ||
      'event';

    const badge = delta.reasoning_content || delta.reasoning
      ? 'reasoning'
      : delta.content
        ? 'content'
        : finishReason
          ? 'finish'
          : 'event';

    return {
      key: `event-${index}`,
      badge,
      preview,
      raw: JSON.stringify(parsed, null, 2),
    };
  } catch (error) {
    return {
      key: `raw-${index}`,
      badge: 'raw',
      preview: rawEvent,
      raw: rawEvent,
    };
  }
};

const DebugPanel = ({
  debugData,
  activeDebugTab,
  onActiveDebugTabChange,
  onCloseDebugPanel,
  customRequestMode,
  customRequestBody,
}) => {
  const { t } = useTranslation();
  const [primaryTab, setPrimaryTab] = useState(
    activeDebugTab === DEBUG_TABS.RESPONSE ? 'response' : 'request',
  );
  const [requestTab, setRequestTab] = useState(
    activeDebugTab === DEBUG_TABS.REQUEST ? 'actual' : 'preview',
  );

  useEffect(() => {
    if (activeDebugTab === DEBUG_TABS.RESPONSE) {
      setPrimaryTab('response');
      return;
    }

    setPrimaryTab('request');
    setRequestTab(activeDebugTab === DEBUG_TABS.REQUEST ? 'actual' : 'preview');
  }, [activeDebugTab]);

  const requestViews = useMemo(
    () => ({
      preview: {
        key: 'preview',
        label: t('预览请求体'),
        content: debugData.previewRequest,
      },
      custom: {
        key: 'custom',
        label: t('自定义'),
        content: stringifyContent(customRequestBody),
      },
      actual: {
        key: 'actual',
        label: t('实际发送'),
        content: debugData.request,
      },
    }),
    [customRequestBody, debugData.previewRequest, debugData.request, t],
  );

  const streamEvents = useMemo(
    () =>
      Array.isArray(debugData.sseMessages)
        ? debugData.sseMessages.map(summarizeStreamEvent)
        : [],
    [debugData.sseMessages],
  );

  const currentRequestView = requestViews[requestTab] || requestViews.preview;
  const currentCopyContent =
    primaryTab === 'response'
      ? streamEvents.length > 0
        ? streamEvents.map((event) => event.raw).join('\n\n')
        : stringifyContent(debugData.response)
      : stringifyContent(currentRequestView.content);

  const handleCopy = async () => {
    if (!currentCopyContent) {
      Toast.warning({
        content: t('暂无可复制的内容'),
        duration: 2,
      });
      return;
    }

    const success = await copy(currentCopyContent);
    if (success) {
      Toast.success({
        content: t('已复制到剪贴板'),
        duration: 2,
      });
      return;
    }

    Toast.error({
      content: t('复制失败'),
      duration: 2,
    });
  };

  const handlePrimaryChange = (tab) => {
    setPrimaryTab(tab);
    onActiveDebugTabChange(tab === 'response' ? DEBUG_TABS.RESPONSE : DEBUG_TABS.PREVIEW);
  };

  const handleRequestTabChange = (tab) => {
    setRequestTab(tab);
    if (tab === 'actual') {
      onActiveDebugTabChange(DEBUG_TABS.REQUEST);
      return;
    }

    if (tab === 'preview') {
      onActiveDebugTabChange(DEBUG_TABS.PREVIEW);
    }
  };

  const footerText =
    primaryTab === 'request'
      ? debugData.timestamp
        ? `${t('最后请求')}: ${new Date(debugData.timestamp).toLocaleString()}`
        : debugData.previewTimestamp
          ? `${t('预览更新')}: ${new Date(debugData.previewTimestamp).toLocaleString()}`
          : t('等待请求触发')
      : debugData.timestamp
        ? `${t('最后请求')}: ${new Date(debugData.timestamp).toLocaleString()}`
        : t('等待响应返回');

  return (
    <div className='playground-v2-drawer h-full'>
      <div className='playground-v2-drawer-header'>
        <div className='flex items-center gap-3'>
          <button
            type='button'
            className='playground-v2-icon-button'
            onClick={onCloseDebugPanel}
            aria-label={t('返回')}
          >
            <ArrowLeft size={16} />
          </button>

          <div>
            <p className='playground-v2-section-kicker !mb-2 !text-slate-500'>
              Debug Panel
            </p>
            <h2 className='playground-v2-panel-title !text-slate-100'>
              {t('调试信息')}
            </h2>
            <div className='playground-v2-panel-subtitle !text-slate-400'>
              {t('查看请求体、实际响应与流式事件。')}
            </div>
          </div>
        </div>

        <button
          type='button'
          className='playground-v2-icon-button'
          onClick={onCloseDebugPanel}
          aria-label={t('关闭')}
        >
          <X size={16} />
        </button>
      </div>

      <div className='playground-v2-drawer-body'>
        <div className='playground-v2-debug-tab-row'>
          <button
            type='button'
            className='playground-v2-debug-tab'
            data-active={primaryTab === 'request'}
            onClick={() => handlePrimaryChange('request')}
          >
            <Bug size={14} />
            {t('请求体')}
          </button>

          <button
            type='button'
            className='playground-v2-debug-tab'
            data-active={primaryTab === 'response'}
            onClick={() => handlePrimaryChange('response')}
          >
            {t('响应流')}
          </button>
        </div>

        {primaryTab === 'request' && (
          <div className='playground-v2-debug-card'>
            <div className='playground-v2-debug-card-header'>
              <div className='playground-v2-debug-subtabs'>
                <button
                  type='button'
                  className='playground-v2-debug-subtab'
                  data-active={requestTab === 'preview'}
                  onClick={() => handleRequestTabChange('preview')}
                >
                  {requestViews.preview.label}
                </button>

                <button
                  type='button'
                  className='playground-v2-debug-subtab'
                  data-active={requestTab === 'custom'}
                  onClick={() => handleRequestTabChange('custom')}
                >
                  {requestViews.custom.label}
                </button>

                <button
                  type='button'
                  className='playground-v2-debug-subtab'
                  data-active={requestTab === 'actual'}
                  onClick={() => handleRequestTabChange('actual')}
                >
                  {requestViews.actual.label}
                </button>
              </div>

              <button
                type='button'
                className='playground-v2-icon-button'
                onClick={handleCopy}
                aria-label={t('复制')}
              >
                <Copy size={14} />
              </button>
            </div>

            <div className='playground-v2-debug-card-body'>
              <CodeViewer
                content={currentRequestView.content}
                title={currentRequestView.key}
                language='json'
              />
            </div>
          </div>
        )}

        {primaryTab === 'response' && (
          <div className='playground-v2-debug-card'>
            <div className='playground-v2-debug-card-header'>
              <span className='playground-v2-outline-pill !bg-slate-900 !text-slate-300'>
                {streamEvents.length > 0
                  ? t('SSE 数据流（最近 {{count}} 条）', {
                      count: streamEvents.length,
                    })
                  : t('响应内容')}
              </span>

              <button
                type='button'
                className='playground-v2-icon-button'
                onClick={handleCopy}
                aria-label={t('复制')}
              >
                <Copy size={14} />
              </button>
            </div>

            <div className='playground-v2-debug-card-body'>
              {streamEvents.length > 0 ? (
                <div className='playground-v2-stream-list'>
                  {streamEvents.map((event, index) => (
                    <details
                      key={event.key}
                      className='playground-v2-stream-item'
                    >
                      <summary className='playground-v2-stream-summary'>
                        <div className='flex min-w-0 items-center gap-2'>
                          <span className='playground-v2-outline-pill !bg-slate-900 !text-slate-300'>
                            #{index + 1}
                          </span>
                          <span className='playground-v2-pill !bg-slate-800 !text-slate-200'>
                            {event.badge}
                          </span>
                          <span className='truncate text-sm text-slate-200'>
                            {event.preview}
                          </span>
                        </div>

                        <span className='text-xs text-slate-500'>JSON</span>
                      </summary>

                      <pre className='playground-v2-stream-pre'>{event.raw}</pre>
                    </details>
                  ))}
                </div>
              ) : (
                <CodeViewer
                  content={debugData.response}
                  title='response'
                  language='json'
                />
              )}
            </div>
          </div>
        )}

        <div className='playground-v2-debug-note'>
          <span>{footerText}</span>
          <span>
            {customRequestMode
              ? t('自定义请求体模式已启用')
              : t('当前面板展示真实请求与响应')}
          </span>
        </div>
      </div>
    </div>
  );
};

export default DebugPanel;
