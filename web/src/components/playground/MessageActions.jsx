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

import React from 'react';
import { Tooltip } from '@douyinfe/semi-ui';
import { Copy, Edit, RefreshCw, Trash2, UserCheck } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const ActionButton = ({
  icon,
  label,
  onClick,
  disabled = false,
  danger = false,
}) => {
  return (
    <Tooltip content={label} position='top'>
      <button
        type='button'
        className='playground-v2-action-button'
        data-danger={danger}
        onClick={onClick}
        disabled={disabled}
        aria-label={label}
      >
        {icon}
      </button>
    </Tooltip>
  );
};

const MessageActions = ({
  message,
  onMessageReset,
  onMessageCopy,
  onMessageDelete,
  onRoleToggle,
  onMessageEdit,
  isAnyMessageGenerating = false,
  isEditing = false,
}) => {
  const { t } = useTranslation();

  const isLoading =
    message.status === 'loading' || message.status === 'incomplete';
  const shouldDisableActions = isAnyMessageGenerating || isEditing;
  const canToggleRole =
    message.role === 'assistant' || message.role === 'system';
  const canEdit =
    !isLoading &&
    message.content &&
    typeof onMessageEdit === 'function' &&
    !isEditing;

  return (
    <div className='playground-v2-message-actions'>
      {!isLoading && (
        <ActionButton
          icon={<RefreshCw size={14} />}
          label={shouldDisableActions ? t('操作暂时被禁用') : t('重试')}
          disabled={shouldDisableActions}
          onClick={() => !shouldDisableActions && onMessageReset(message)}
        />
      )}

      {message.content && (
        <ActionButton
          icon={<Copy size={14} />}
          label={t('复制')}
          onClick={() => onMessageCopy(message)}
        />
      )}

      {canEdit && (
        <ActionButton
          icon={<Edit size={14} />}
          label={shouldDisableActions ? t('操作暂时被禁用') : t('编辑')}
          disabled={shouldDisableActions}
          onClick={() => !shouldDisableActions && onMessageEdit(message)}
        />
      )}

      {canToggleRole && !isLoading && (
        <ActionButton
          icon={<UserCheck size={14} />}
          label={
            shouldDisableActions
              ? t('操作暂时被禁用')
              : message.role === 'assistant'
                ? t('切换为System角色')
                : t('切换为Assistant角色')
          }
          disabled={shouldDisableActions}
          onClick={() =>
            !shouldDisableActions && onRoleToggle && onRoleToggle(message)
          }
        />
      )}

      {!isLoading && (
        <ActionButton
          icon={<Trash2 size={14} />}
          label={shouldDisableActions ? t('操作暂时被禁用') : t('删除')}
          disabled={shouldDisableActions}
          danger
          onClick={() => !shouldDisableActions && onMessageDelete(message)}
        />
      )}
    </div>
  );
};

export default MessageActions;
