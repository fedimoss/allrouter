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
import { Toast } from '@douyinfe/semi-ui';
import {
  Bug,
  ImagePlus,
  MessageSquare,
  Send,
  Settings2,
  Square,
  Trash2,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { MESSAGE_ROLES } from '../../constants/playground.constants';
import { usePlayground } from '../../contexts/PlaygroundContext';
import logoImg from '../../../public/logo-white.svg'

const composerRoleOptions = [
  { key: MESSAGE_ROLES.USER, label: 'User', dotColor: '#34d399' },
  { key: MESSAGE_ROLES.SYSTEM, label: 'System', dotColor: '#94a3b8' },
];

const ChatArea = ({
  chatRef,
  message,
  inputs,
  styleState,
  roleInfo,
  onMessageSend,
  onStopGenerator,
  onClearMessages,
  onToggleDebugPanel,
  onToggleSettings,
  renderCustomChatContent,
  renderChatBoxAction,
}) => {
  const { t } = useTranslation();
  const { onPasteImage, imageUrls, imageEnabled } = usePlayground();
  const [draft, setDraft] = useState('');
  const [composerRole, setComposerRole] = useState(MESSAGE_ROLES.USER);
  const listRef = useRef(null);

  const validImages = (imageUrls || []).filter((url) => url && url.trim());
  const isGenerating = message.some(
    (item) => item.status === 'loading' || item.status === 'incomplete',
  );
  const canSend =
    draft.trim().length > 0 ||
    (composerRole === MESSAGE_ROLES.USER && validImages.length > 0);

  useEffect(() => {
    if (listRef.current) {
      listRef.current.scrollTop = listRef.current.scrollHeight;
      if (chatRef) {
        chatRef.current = listRef.current;
      }
    }
  }, [chatRef, message]);

  const handleSend = () => {
    if (!canSend || isGenerating) {
      return;
    }

    onMessageSend({
      content: draft.trim(),
      role: composerRole,
    });
    setDraft('');
  };

  const handlePaste = async (event) => {
    const items = event.clipboardData?.items;
    if (!items) {
      return;
    }

    for (let index = 0; index < items.length; index += 1) {
      const item = items[index];

      if (!item.type.includes('image')) {
        continue;
      }

      event.preventDefault();

      if (!imageEnabled) {
        Toast.warning({
          content: t('请先在设置中启用图片功能'),
          duration: 3,
        });
        return;
      }

      const file = item.getAsFile();
      if (!file) {
        return;
      }

      try {
        const reader = new FileReader();
        reader.onload = (loadEvent) => {
          const base64 = loadEvent.target?.result;
          if (base64 && onPasteImage) {
            onPasteImage(base64);
            Toast.success({
              content: t('图片已添加'),
              duration: 2,
            });
          }
        };
        reader.onerror = () => {
          Toast.error({
            content: t('粘贴图片失败'),
            duration: 2,
          });
        };
        reader.readAsDataURL(file);
      } catch (error) {
        console.error('Failed to paste image:', error);
        Toast.error({
          content: t('粘贴图片失败'),
          duration: 2,
        });
      }

      break;
    }
  };

  const handleKeyDown = (event) => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      if (isGenerating) {
        onStopGenerator();
        return;
      }
      handleSend();
    }
  };

  return (
    <div className='playground-v2-chat h-full'>
      <div className='playground-v2-chat-header'>
        <div className='playground-v2-chat-header-main'>
          <span className='playground-v2-chat-icon'>
            <MessageSquare size={16} />
          </span>

          <h2 className='playground-v2-panel-title'>{t('AI 对话')}</h2>
          <span className='playground-v2-outline-pill'>
            {inputs.model || t('未选择模型')}
          </span>
        </div>

        <div className='playground-v2-chat-header-actions'>
          <div className='playground-v2-chat-role-group'>
            <span className='playground-v2-chat-role-label'>
              {t('当前角色')}
            </span>
            <div className='playground-v2-role-toggle'>
              {composerRoleOptions.map((option) => (
                <button
                  key={option.key}
                  type='button'
                  className='playground-v2-role-pill'
                  data-active={composerRole === option.key}
                  onClick={() => setComposerRole(option.key)}
                >
                  <span
                    className='playground-v2-role-dot'
                    style={{ background: option.dotColor }}
                  />
                  {option.label}
                </button>
              ))}
            </div>
          </div>

          <div className='playground-v2-chat-tools'>
            <button
              type='button'
              className='playground-v2-icon-button'
              onClick={onToggleDebugPanel}
              aria-label={t('打开调试')}
              title={t('打开调试')}
            >
              <Bug size={15} />
            </button>

          {styleState.isMobile && (
            <button
              type='button'
              className='playground-v2-icon-button playground-v2-hidden-desktop'
              onClick={onToggleSettings}
              aria-label={t('打开设置')}
            >
              <Settings2 size={16} />
            </button>
          )}
          </div>
        </div>
      </div>

      <div className='playground-v2-message-list model-settings-scroll' ref={listRef}>
        {message.map((currentMessage) => {
          const isUser = currentMessage.role === MESSAGE_ROLES.USER;
          const roleMeta = roleInfo[currentMessage.role] || roleInfo.assistant;

          return (
            <div
              key={currentMessage.id}
              className='playground-v2-message-row'
              data-role={currentMessage.role}
            >
              {!isUser && (
                <span className='playground-v2-avatar'>
                  {roleMeta?.avatar ? (
                    <img src={logoImg} alt={roleMeta.name} />
                  ) : (
                    roleMeta?.name?.[0] || 'A'
                  )}
                </span>
              )}

              <div className='playground-v2-message-stack'>
                <div
                  className='playground-v2-bubble'
                  data-role={currentMessage.role}
                >
                  {renderCustomChatContent({
                    message: currentMessage,
                    className: 'playground-v2-bubble-content',
                  })}
                </div>

                <div
                  className='playground-v2-message-actions-row'
                  data-align={isUser ? 'end' : 'start'}
                >
                  {renderChatBoxAction({
                    message: currentMessage,
                  })}
                </div>
              </div>
            </div>
          );
        })}
      </div>

      <div className='playground-v2-composer-wrap'>
        <div className='playground-v2-composer'>
          {composerRole === MESSAGE_ROLES.USER && validImages.length > 0 && (
            <div className='playground-v2-attachment-strip'>
              <span className='playground-v2-attachment-chip'>
                <ImagePlus size={14} />
                {t('已附加 {{count}} 张图片', { count: validImages.length })}
              </span>
            </div>
          )}

          <div className='playground-v2-composer-frame'>
            <textarea
              className='playground-v2-composer-textarea'
              value={draft}
              rows={2}
              placeholder={
                composerRole === MESSAGE_ROLES.SYSTEM
                  ? t('以 System 角色输入指令，例如：你是一个严格的代码审查助手...')
                  : t('请输入您的问题...')
              }
              onChange={(event) => setDraft(event.target.value)}
              onPaste={handlePaste}
              onKeyDown={handleKeyDown}
            />

            <button
              type='button'
              className='playground-v2-send-button'
              data-stop={isGenerating}
              disabled={!isGenerating && !canSend}
              onClick={isGenerating ? onStopGenerator : handleSend}
              aria-label={isGenerating ? t('停止生成') : t('发送')}
            >
              {isGenerating ? <Square size={18} /> : <Send size={18} />}
            </button>
          </div>

          <div className='playground-v2-composer-footer'>
            <button
              type='button'
              className='playground-v2-link-button'
              onClick={onClearMessages}
            >
              <Trash2 size={14} />
              {t('清除本次会话')}
            </button>

            <span>{t('AI 生成的内容可能不准确，请仔细甄别。')}</span>
          </div>
        </div>
      </div>
    </div>
  );
};

export default ChatArea;
