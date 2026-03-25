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

import React, { useCallback, useContext, useEffect } from 'react';
import { Toast } from '@douyinfe/semi-ui';
import { useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

import './playground-v2.css';

import { UserContext } from '../../context/User';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { usePlaygroundState } from '../../hooks/playground/usePlaygroundState';
import { useMessageActions } from '../../hooks/playground/useMessageActions';
import { useApiRequest } from '../../hooks/playground/useApiRequest';
import { useSyncMessageAndCustomBody } from '../../hooks/playground/useSyncMessageAndCustomBody';
import { useMessageEdit } from '../../hooks/playground/useMessageEdit';
import { useDataLoader } from '../../hooks/playground/useDataLoader';
import { MESSAGE_ROLES, ERROR_MESSAGES } from '../../constants/playground.constants';
import {
  getLogo,
  stringToColor,
  buildMessageContent,
  createMessage,
  createLoadingAssistantMessage,
  getTextContent,
  buildApiPayload,
  encodeToBase64,
  formatMessageForAPI,
} from '../../helpers';
import {
  OptimizedSettingsPanel,
  OptimizedDebugPanel,
  OptimizedMessageContent,
  OptimizedMessageActions,
} from '../../components/playground/OptimizedComponents';
import ChatArea from '../../components/playground/ChatArea';
import { PlaygroundProvider } from '../../contexts/PlaygroundContext';

// 生成头像
const generateAvatarDataUrl = (username) => {
  if (!username) {
    return 'https://lf3-static.bytednsdoc.com/obj/eden-cn/ptlz_zlp/ljhwZthlaukjlkulzlp/docs-icon.png';
  }

  const firstLetter = username[0].toUpperCase();
  const bgColor = stringToColor(username);
  const svg = `
    <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32">
      <circle cx="16" cy="16" r="16" fill="${bgColor}" />
      <text x="50%" y="50%" dominant-baseline="central" text-anchor="middle" font-size="16" fill="#ffffff" font-family="sans-serif">${firstLetter}</text>
    </svg>
  `;

  return `data:image/svg+xml;base64,${encodeToBase64(svg)}`;
};

const normalizeOutgoingMessage = (payload) => {
  if (typeof payload === 'string') {
    return {
      content: payload,
      role: MESSAGE_ROLES.USER,
    };
  }

  return {
    content: payload?.content || '',
    role: payload?.role || MESSAGE_ROLES.USER,
  };
};

const Playground = () => {
  const { t } = useTranslation();
  const [userState] = useContext(UserContext);
  const isMobile = useIsMobile();
  const styleState = { isMobile };
  const [searchParams] = useSearchParams();

  const state = usePlaygroundState();
  const {
    inputs,
    parameterEnabled,
    showDebugPanel,
    customRequestMode,
    customRequestBody,
    showSettings,
    models,
    groups,
    message,
    debugData,
    activeDebugTab,
    previewPayload,
    sseSourceRef,
    chatRef,
    handleInputChange,
    handleParameterToggle,
    debouncedSaveConfig,
    saveMessagesImmediately,
    handleConfigImport,
    handleConfigReset,
    setShowSettings,
    setModels,
    setGroups,
    setMessage,
    setDebugData,
    setActiveDebugTab,
    setPreviewPayload,
    setShowDebugPanel,
    setCustomRequestMode,
    setCustomRequestBody,
  } = state;

  // API 请求相关
  const { sendRequest, onStopGenerator } = useApiRequest(
    setMessage,
    setDebugData,
    setActiveDebugTab,
    sseSourceRef,
    saveMessagesImmediately,
  );

  // 数据加载
  useDataLoader(userState, inputs, handleInputChange, setModels, setGroups);

  // 消息编辑
  const {
    editingMessageId,
    editValue,
    setEditValue,
    handleMessageEdit,
    handleEditSave,
    handleEditCancel,
  } = useMessageEdit(
    setMessage,
    inputs,
    parameterEnabled,
    sendRequest,
    saveMessagesImmediately,
  );

  // 消息和自定义请求体同步
  const { syncMessageToCustomBody, syncCustomBodyToMessage } =
    useSyncMessageAndCustomBody(
      customRequestMode,
      customRequestBody,
      message,
      inputs,
      setCustomRequestBody,
      setMessage,
      debouncedSaveConfig,
    );

  // 角色信息
  const roleInfo = {
    user: {
      name: userState?.user?.username || 'User',
      avatar: generateAvatarDataUrl(userState?.user?.username),
    },
    assistant: {
      name: 'Assistant',
      avatar: getLogo(),
    },
    system: {
      name: 'System',
      avatar: getLogo(),
    },
  };

  function onMessageSend(payload) {
    const { content, role } = normalizeOutgoingMessage(payload);
    const trimmedContent = content.trim();
    const validImageUrls = (inputs.imageUrls || []).filter(
      (url) => url && url.trim() !== '',
    );
    const canAttachImages =
      role === MESSAGE_ROLES.USER &&
      inputs.imageEnabled &&
      validImageUrls.length > 0;

    if (!trimmedContent && !canAttachImages) {
      return;
    }

    const outgoingContent =
      role === MESSAGE_ROLES.USER
        ? buildMessageContent(trimmedContent, validImageUrls, inputs.imageEnabled)
        : trimmedContent;

    const outgoingMessage = createMessage(role, outgoingContent);
    const loadingMessage = createLoadingAssistantMessage();

    if (customRequestMode && customRequestBody) {
      try {
        const customPayload = JSON.parse(customRequestBody);

        setMessage((prevMessage) => {
          const newMessages = [...prevMessage, outgoingMessage, loadingMessage];
          const payloadToSend = {
            ...customPayload,
            messages: newMessages
              .filter((item) => item.status !== 'loading')
              .map(formatMessageForAPI),
          };

          if (payloadToSend.stream === undefined) {
            payloadToSend.stream = inputs.stream;
          }

          sendRequest(payloadToSend, payloadToSend.stream !== false);
          setTimeout(() => saveMessagesImmediately(newMessages), 0);
          return newMessages;
        });
        return;
      } catch (error) {
        console.error('自定义请求体JSON解析失败:', error);
        Toast.error(ERROR_MESSAGES.JSON_PARSE_ERROR);
        return;
      }
    }

    setMessage((prevMessage) => {
      const newMessages = [...prevMessage, outgoingMessage];
      const payloadToSend = buildApiPayload(
        newMessages,
        null,
        inputs,
        parameterEnabled,
      );

      sendRequest(payloadToSend, inputs.stream);

      if (canAttachImages) {
        setTimeout(() => {
          handleInputChange('imageEnabled', false);
        }, 100);
      }

      const messagesWithLoading = [...newMessages, loadingMessage];
      setTimeout(() => saveMessagesImmediately(messagesWithLoading), 0);
      return messagesWithLoading;
    });
  }

  const messageActions = useMessageActions(
    message,
    setMessage,
    onMessageSend,
    saveMessagesImmediately,
  );

  // 构建预览请求体
  const constructPreviewPayload = useCallback(() => {
    try {
      // 如果是自定义请求体模式且有自定义内容，直接返回解析后的自定义请求体
      if (customRequestMode && customRequestBody && customRequestBody.trim()) {
        try {
          return JSON.parse(customRequestBody);
        } catch (parseError) {
          console.warn('自定义请求体JSON解析失败，回退到默认预览:', parseError);
        }
      }

      let previewMessages = [...message];

      // 如果存在用户消息
      if (
        !(
          previewMessages.length === 0 ||
          previewMessages.every((item) => item.role !== MESSAGE_ROLES.USER)
        )
      ) {
        for (let index = previewMessages.length - 1; index >= 0; index -= 1) {
          if (previewMessages[index].role === MESSAGE_ROLES.USER) {
            if (inputs.imageEnabled && inputs.imageUrls) {
              const validImageUrls = inputs.imageUrls.filter(
                (url) => url.trim() !== '',
              );
              if (validImageUrls.length > 0) {
                const textContent = getTextContent(previewMessages[index]) || '示例消息';
                const contentWithImages = buildMessageContent(
                  textContent,
                  validImageUrls,
                  true,
                );
                previewMessages[index] = {
                  ...previewMessages[index],
                  content: contentWithImages,
                };
              }
            }
            break;
          }
        }
      }

      return buildApiPayload(previewMessages, null, inputs, parameterEnabled);
    } catch (error) {
      console.error('构造预览请求体失败:', error);
      return null;
    }
  }, [customRequestBody, customRequestMode, inputs, message, parameterEnabled]);

  const toggleReasoningExpansion = useCallback(
    (messageId) => {
      setMessage((prevMessages) =>
        prevMessages.map((item) =>
          item.id === messageId && item.role === MESSAGE_ROLES.ASSISTANT
            ? {
                ...item,
                isReasoningExpanded: !item.isReasoningExpanded,
              }
            : item,
        ),
      );
    },
    [setMessage],
  );

  // 渲染函数
  const renderCustomChatContent = useCallback(
    ({ message: currentMessage, className }) => {
      const isCurrentlyEditing = editingMessageId === currentMessage.id;

      return (
        <OptimizedMessageContent
          message={currentMessage}
          className={className}
          styleState={styleState}
          onToggleReasoningExpansion={toggleReasoningExpansion}
          isEditing={isCurrentlyEditing}
          onEditSave={handleEditSave}
          onEditCancel={handleEditCancel}
          editValue={editValue}
          onEditValueChange={setEditValue}
        />
      );
    },
    [
      editingMessageId,
      editValue,
      handleEditCancel,
      handleEditSave,
      setEditValue,
      styleState,
      toggleReasoningExpansion,
    ],
  );

  const renderChatBoxAction = useCallback(
    ({ message: currentMessage }) => {
      const isAnyMessageGenerating = message.some(
        (item) => item.status === 'loading' || item.status === 'incomplete',
      );
      const isCurrentlyEditing = editingMessageId === currentMessage.id;

      return (
        <OptimizedMessageActions
          message={currentMessage}
          styleState={styleState}
          onMessageReset={messageActions.handleMessageReset}
          onMessageCopy={messageActions.handleMessageCopy}
          onMessageDelete={messageActions.handleMessageDelete}
          onRoleToggle={messageActions.handleRoleToggle}
          onMessageEdit={handleMessageEdit}
          isAnyMessageGenerating={isAnyMessageGenerating}
          isEditing={isCurrentlyEditing}
        />
      );
    },
    [editingMessageId, handleMessageEdit, message, messageActions, styleState],
  );

  // Effects

  // 同步消息和自定义请求体
  useEffect(() => {
    syncMessageToCustomBody();
  }, [message, syncMessageToCustomBody]);

  useEffect(() => {
    syncCustomBodyToMessage();
  }, [customRequestBody, syncCustomBodyToMessage]);

  // 处理URL参数
  useEffect(() => {
    if (searchParams.get('expired')) {
      Toast.warning(t('登录过期，请重新登录！'));
    }
  }, [searchParams, t]);

  // Playground 组件无需再监听窗口变化，isMobile 由 useIsMobile Hook 自动更新

  // 构建预览payload
  useEffect(() => {
    const timer = setTimeout(() => {
      const preview = constructPreviewPayload();
      setPreviewPayload(preview);
      setDebugData((prev) => ({
        ...prev,
        previewRequest: preview ? JSON.stringify(preview, null, 2) : null,
        previewTimestamp: preview ? new Date().toISOString() : null,
      }));
    }, 300);

    return () => clearTimeout(timer);
  }, [
    constructPreviewPayload,
    customRequestBody,
    customRequestMode,
    inputs,
    message,
    parameterEnabled,
    setDebugData,
    setPreviewPayload,
  ]);

  // 自动保存配置
  useEffect(() => {
    debouncedSaveConfig();
  }, [
    customRequestBody,
    customRequestMode,
    debouncedSaveConfig,
    inputs,
    parameterEnabled,
    showDebugPanel,
  ]);

  const handleClearMessages = useCallback(() => {
    setMessage([]);
    setTimeout(() => saveMessagesImmediately([]), 0);
  }, [saveMessagesImmediately, setMessage]);

  const handlePasteImage = useCallback(
    (base64Data) => {
      if (!inputs.imageEnabled) {
        return;
      }

      const newUrls = [...(inputs.imageUrls || []), base64Data];
      handleInputChange('imageUrls', newUrls);
    },
    [handleInputChange, inputs.imageEnabled, inputs.imageUrls],
  );

  const playgroundContextValue = {
    onPasteImage: handlePasteImage,
    imageUrls: inputs.imageUrls || [],
    imageEnabled: inputs.imageEnabled || false,
  };

  return (
    <PlaygroundProvider value={playgroundContextValue}>
      <div className='playground-v2 h-full'>
        <div className={`playground-v2-shell ${isMobile ? 'pt-[72px]' : ''}`}>
          <div className='playground-v2-content'>
            {!isMobile && (
              <div className='min-h-0'>
                <OptimizedSettingsPanel
                  inputs={inputs}
                  parameterEnabled={parameterEnabled}
                  models={models}
                  groups={groups}
                  styleState={styleState}
                  showSettings={showSettings}
                  showDebugPanel={showDebugPanel}
                  customRequestMode={customRequestMode}
                  customRequestBody={customRequestBody}
                  onInputChange={handleInputChange}
                  onParameterToggle={handleParameterToggle}
                  onCloseSettings={() => setShowSettings(false)}
                  onConfigImport={handleConfigImport}
                  onConfigReset={handleConfigReset}
                  onCustomRequestModeChange={setCustomRequestMode}
                  onCustomRequestBodyChange={setCustomRequestBody}
                  previewPayload={previewPayload}
                  messages={message}
                />
              </div>
            )}

            <div className='min-h-0'>
              <ChatArea
                chatRef={chatRef}
                message={message}
                inputs={inputs}
                styleState={styleState}
                roleInfo={roleInfo}
                onMessageSend={onMessageSend}
                onStopGenerator={onStopGenerator}
                onClearMessages={handleClearMessages}
                onToggleDebugPanel={() => setShowDebugPanel((prev) => !prev)}
                onToggleSettings={() => setShowSettings(true)}
                renderCustomChatContent={renderCustomChatContent}
                renderChatBoxAction={renderChatBoxAction}
              />
            </div>
          </div>
        </div>

        {isMobile && showSettings && (
          <>
            <div
              className='playground-v2-overlay'
              onClick={() => setShowSettings(false)}
            />
            <div className='playground-v2-mobile-sheet'>
              <OptimizedSettingsPanel
                inputs={inputs}
                parameterEnabled={parameterEnabled}
                models={models}
                groups={groups}
                styleState={styleState}
                showSettings={showSettings}
                showDebugPanel={showDebugPanel}
                customRequestMode={customRequestMode}
                customRequestBody={customRequestBody}
                onInputChange={handleInputChange}
                onParameterToggle={handleParameterToggle}
                onCloseSettings={() => setShowSettings(false)}
                onConfigImport={handleConfigImport}
                onConfigReset={handleConfigReset}
                onCustomRequestModeChange={setCustomRequestMode}
                onCustomRequestBodyChange={setCustomRequestBody}
                previewPayload={previewPayload}
                messages={message}
              />
            </div>
          </>
        )}

        {showDebugPanel && (
          <>
            <div
              className='playground-v2-overlay'
              onClick={() => setShowDebugPanel(false)}
            />
            <OptimizedDebugPanel
              debugData={debugData}
              activeDebugTab={activeDebugTab}
              onActiveDebugTabChange={setActiveDebugTab}
              customRequestMode={customRequestMode}
              customRequestBody={customRequestBody}
              onCloseDebugPanel={() => setShowDebugPanel(false)}
            />
          </>
        )}
      </div>
    </PlaygroundProvider>
  );
};

export default Playground;
