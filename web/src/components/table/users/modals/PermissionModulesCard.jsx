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
import { useTranslation } from 'react-i18next';
import { Avatar, Card, Typography } from '@douyinfe/semi-ui';
import { IconLock } from '@douyinfe/semi-icons';
import { getPermissionModules } from '../../../../helpers';

const { Text } = Typography;

// 与 new-api 一致的勾选图标：圆角描边对勾
const CheckIcon = () => (
  <svg
    xmlns='http://www.w3.org/2000/svg'
    width='24'
    height='24'
    viewBox='0 0 24 24'
    fill='none'
    stroke='currentColor'
    strokeWidth='2'
    strokeLinecap='round'
    strokeLinejoin='round'
    className='h-3 w-3'
  >
    <path d='M5 14L8.5 17.5L19 6.5' />
  </svg>
);

// 视觉勾选框：未选中为灰色圆环，选中填充主题色圆并显示白色对勾（与 new-api 一致）
const CheckboxIndicator = ({ checked }) => (
  <span
    aria-hidden='true'
    className='mt-[1px] flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded-full border transition-colors'
    style={
      checked
        ? {
            borderColor: 'var(--semi-color-primary)',
            backgroundColor: 'var(--semi-color-primary)',
            color: '#fff',
          }
        : {
            borderColor: 'var(--semi-color-fill-2)',
            backgroundColor: 'var(--semi-color-bg-0)',
          }
    }
  >
    {checked && <CheckIcon />}
  </span>
);

/**
 * 模块权限设置卡片：为普通用户勾选可访问的管理页面。
 * 仅在"操作者可授权 + 目标用户为普通用户"时由父组件渲染。
 * 受控组件：value 为已勾选的模块 key 数组。
 */
const PermissionModulesCard = ({ providerMode = false, value = [], onChange }) => {
  const { t } = useTranslation();
  const modules = getPermissionModules(providerMode);
  const groupTitle = providerMode ? t('服务商') : t('管理员');

  const toggle = (key, checked) => {
    const next = checked
      ? [...value, key]
      : value.filter((k) => k !== key);
    onChange?.(next);
  };

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      <div className='flex items-center mb-2'>
        <Avatar size='small' color='orange' className='mr-2 shadow-md'>
          <IconLock size={16} />
        </Avatar>
        <div>
          <Text className='text-lg font-medium'>{t('模块权限')}</Text>
          <div className='text-xs text-gray-600'>
            {t('勾选允许该普通用户访问的管理页面')}
          </div>
        </div>
      </div>

      <div className='rounded-md border p-3 space-y-2'>
        <div className='text-sm font-medium'>
          {groupTitle}
          <span className='text-xs text-gray-500 ml-2'>
            {t('导航页面')}
          </span>
        </div>
        {/* 列表定高滚动，避免模块过多时弹窗被撑得过长 */}
        <div className='max-h-72 overflow-y-auto pr-1 space-y-2'>
          {modules.map((m) => {
            const checked = value.includes(m.key);
            return (
              <div
                key={m.key}
                role='checkbox'
                aria-checked={checked}
                tabIndex={0}
                onClick={() => toggle(m.key, !checked)}
                onKeyDown={(e) => {
                  if (e.key === ' ' || e.key === 'Enter') {
                    e.preventDefault();
                    toggle(m.key, !checked);
                  }
                }}
                className='flex items-start gap-3 px-2 py-1 rounded-md cursor-pointer select-none transition-colors hover:bg-[color:var(--semi-color-fill-0)]'
              >
                <CheckboxIndicator checked={checked} />
                <div className='flex flex-col gap-0.5'>
                  <span className='text-sm font-medium'>{t(m.label)}</span>
                  <span className='text-xs text-gray-500'>
                    {t(m.description)}
                  </span>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </Card>
  );
};

export default PermissionModulesCard;
