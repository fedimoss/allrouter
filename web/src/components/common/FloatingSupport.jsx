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

  return (
    <div className='floating-support' aria-label='customer support'>
      {displayWechatQRCode ? (
        <div className='floating-support-item floating-support-wechat'>
          <button
            className='floating-support-icon floating-support-icon-wechat'
            type='button'
            aria-label='微信客服'
          >
            <i className='fab fa-weixin' />
          </button>
          <div className='floating-support-qrcode'>
            <img src={displayWechatQRCode} alt='微信客服二维码' />
          </div>
        </div>
      ) : null}
      {qqNumber ? (
        <a
          className='floating-support-icon floating-support-icon-qq'
          href={qqHref}
          aria-label='QQ客服'
        >
          <img src={qqLogo} alt='QQ客服' />
        </a>
      ) : null}
    </div>
  );
};

export default FloatingSupport;
