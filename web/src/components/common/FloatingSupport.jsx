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

const normalizeQQ = (qqSupport = '') => String(qqSupport).replace(/\D/g, '');
const normalizeWechatQRCode = (wechatQRCode = '') =>
  String(wechatQRCode).trim();

const FloatingSupport = ({ wechatQRCode, qqSupport }) => {
  const displayWechatQRCode = normalizeWechatQRCode(wechatQRCode);
  const qqNumber = normalizeQQ(qqSupport);

  if (!displayWechatQRCode && !qqNumber) {
    return null;
  }

  const qqHref = qqNumber
    ? `tencent://message/?uin=${qqNumber}&Site=&Menu=yes`
    : undefined;

  const qqPopoverContent = (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
      <img src={qqLogo} alt='QQ客服' style={{ width: 20 }} />
      <a
        href={qqHref}
        style={{
          color: 'var(--semi-color-text-0)',
          textDecoration: 'none',
          fontWeight: 600,
          fontSize: 14,
        }}
      >
        {qqNumber}
      </a>
    </div>
  );

  const wechatPopoverContent = displayWechatQRCode ? (
    <img
      src={displayWechatQRCode}
      alt='微信客服二维码'
      style={{ width: 140, height: 140, objectFit: 'contain', borderRadius: 6, display: 'block' }}
    />
  ) : null;

  return (
    <div className='floating-support' aria-label='customer support'>
      {displayWechatQRCode ? (
        <Popover
          content={wechatPopoverContent}
          position='left'
          showArrow
          trigger='hover'
        >
          <div className='floating-support-icon floating-support-icon-wechat'>
            <i className='fab fa-weixin' />
          </div>
        </Popover>
      ) : null}
      {qqNumber ? (
        <Popover
          content={qqPopoverContent}
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
