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
import { Popover } from '@douyinfe/semi-ui';
import qqLogo from '../../../public/qq.png';

// 客服弹层：支持每渠道多张二维码。
// 单张：二维码在上、描述在下（沿用原样式）。
// 多张：网格布局 —— 2 张两列、3 张三列、4 张 2×2，每张带独立描述。
const renderSupportPopover = (list, alt) => {
  const items = (list || []).filter((item) => item && (item.url || item.desc));
  if (items.length === 0) return null;
  if (items.length === 1) {
    const { url, desc } = items[0];
    return (
      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          gap: 8,
          maxWidth: 160,
        }}
      >
        {url ? (
          <img
            src={url}
            alt={alt}
            style={{
              width: 140,
              height: 140,
              objectFit: 'contain',
              borderRadius: 6,
              display: 'block',
            }}
          />
        ) : null}
        {desc ? (
          <span
            style={{
              fontSize: 13,
              color: 'var(--semi-color-text-1)',
              textAlign: 'center',
              wordBreak: 'break-word',
              whiteSpace: 'pre-wrap',
              lineHeight: 1.4,
            }}
          >
            {desc}
          </span>
        ) : null}
      </div>
    );
  }
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns:
          items.length === 3 ? 'repeat(3, 104px)' : 'repeat(2, 112px)',
        gap: '14px 10px',
        maxWidth: 380,
      }}
    >
      {items.slice(0, 4).map((item, index) => (
        <div
          key={index}
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: 6,
            minWidth: 0,
          }}
        >
          {item.url ? (
            <img
              src={item.url}
              alt={alt}
              style={{
                width: items.length === 3 ? 104 : 112,
                height: items.length === 3 ? 104 : 112,
                objectFit: 'contain',
                borderRadius: 6,
                background: '#fff',
                display: 'block',
              }}
            />
          ) : null}
          {item.desc ? (
            <span
              style={{
                fontSize: 12,
                color: 'var(--semi-color-text-1)',
                textAlign: 'center',
                wordBreak: 'break-word',
                lineHeight: 1.4,
                maxWidth: 130,
              }}
            >
              {item.desc}
            </span>
          ) : null}
        </div>
      ))}
    </div>
  );
};

const FloatingSupport = ({ wechatList, qqList, telegramList }) => {
  const wechatItems = (wechatList || []).filter(
    (item) => item && (item.url || item.desc),
  );
  const qqItems = (qqList || []).filter(
    (item) => item && (item.url || item.desc),
  );
  const telegramItems = (telegramList || []).filter(
    (item) => item && (item.url || item.desc),
  );

  const showWechat = wechatItems.length > 0;
  const showQQ = qqItems.length > 0;
  const showTelegram = telegramItems.length > 0;

  if (!showWechat && !showQQ && !showTelegram) {
    return null;
  }

  return (
    <div className='floating-support' aria-label='customer support'>
      {showWechat ? (
        <Popover
          content={renderSupportPopover(wechatItems, '微信客服二维码')}
          position='left'
          showArrow
          trigger='hover'
        >
          <div className='floating-support-icon floating-support-icon-wechat'>
            <i className='fab fa-weixin' />
          </div>
        </Popover>
      ) : null}
      {showTelegram ? (
        <Popover
          content={renderSupportPopover(telegramItems, 'Telegram客服二维码')}
          position='left'
          showArrow
          trigger='hover'
        >
          <div className='floating-support-icon floating-support-icon-telegram'>
            <i className='fab fa-telegram' />
          </div>
        </Popover>
      ) : null}
      {showQQ ? (
        <Popover
          content={renderSupportPopover(qqItems, 'QQ客服二维码')}
          position='left'
          showArrow
          trigger='hover'
        >
          <div className='floating-support-icon floating-support-icon-qq'>
            <img src={qqLogo} alt='QQ客服' />
          </div>
        </Popover>
      ) : null}
    </div>
  );
};

export default FloatingSupport;
