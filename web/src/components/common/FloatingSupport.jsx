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

const normalize = (value = '') => String(value).trim();

// 客服弹层：二维码图片在上、文本描述在下；两者皆空时返回 null。
const renderSupportPopover = (image, text, alt) => {
  if (!image && !text) return null;
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
      {image ? (
        <img
          src={image}
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
      {text ? (
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
          {text}
        </span>
      ) : null}
    </div>
  );
};

const FloatingSupport = ({
  wechatQRCode,
  wechatDesc,
  qqQrcode,
  qqSupport,
  telegramQRCode,
  telegramDesc,
}) => {
  const wechatImage = normalize(wechatQRCode);
  const wechatText = normalize(wechatDesc);
  const qqImage = normalize(qqQrcode);
  const qqText = normalize(qqSupport);
  const telegramImage = normalize(telegramQRCode);
  const telegramText = normalize(telegramDesc);

  const showWechat = Boolean(wechatImage || wechatText);
  const showQQ = Boolean(qqImage || qqText);
  const showTelegram = Boolean(telegramImage || telegramText);

  if (!showWechat && !showQQ && !showTelegram) {
    return null;
  }

  return (
    <div className='floating-support' aria-label='customer support'>
      {showWechat ? (
        <Popover
          content={renderSupportPopover(wechatImage, wechatText, '微信客服二维码')}
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
          content={renderSupportPopover(telegramImage, telegramText, 'Telegram客服二维码')}
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
          content={renderSupportPopover(qqImage, qqText, 'QQ客服二维码')}
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
