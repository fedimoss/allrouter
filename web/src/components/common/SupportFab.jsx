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

import React, { useContext, useEffect, useMemo, useState } from 'react';
import { ArrowUp } from 'lucide-react';
import { useLocation } from 'react-router-dom';
import { StatusContext } from '../../context/Status';
import { useActualTheme } from '../../context/Theme';
import { buildSupportConfig } from '../../helpers';
import './SupportFab.css';

const TG_ICON = (
  <svg
    viewBox='0 0 24 24'
    fill='currentColor'
    aria-hidden='true'
    width='20'
    height='20'
  >
    <path d='M21.9 4.4 2.8 11.9c-.95.38-.9 1.66.08 1.98l4.76 1.52 1.83 5.72c.28.86 1.37 1.03 1.93.34l2.53-3.18 4.98 3.67c.74.55 1.8.14 2.01-.75l3.06-15.8c.2-1.03-.84-1.86-1.78-1.44ZM8.6 14.1l8.7-7.3c.22-.18.45.16.27.36l-7.2 6.9-.3 3.06-1.45-3.02Z' />
  </svg>
);

// 仅填描述无图时，用描述文本生成占位二维码（沿用原 t3 逻辑）
const qr = (url) =>
  `https://api.qrserver.com/v1/create-qr-code/?size=160x160&margin=8&data=${encodeURIComponent(
    url,
  )}`;

// 全局悬浮客服按钮（Telegram / Wechat / QQ / back-to-top）。
// 由 PageLayout 全局挂载；数据取自 StatusContext，视觉跟随 actualTheme：
//   - t3 首页（'/' + home_page_theme=style_c）加 support-fab--t3 变体（保留底部偏移）
//   - 其余页面为控制台视觉（浅色/深色）
// 每渠道支持多张二维码：1 张单列、2 张两列、3 张三列、4 张 2×2（见 SupportFab.css）。
const SupportFab = () => {
  const [statusState] = useContext(StatusContext);
  const actualTheme = useActualTheme();
  const { pathname } = useLocation();
  const [showTop, setShowTop] = useState(false);

  useEffect(() => {
    const onScroll = () => setShowTop(window.scrollY > 360);
    window.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
    return () => window.removeEventListener('scroll', onScroll);
  }, []);

  const support = useMemo(
    () => buildSupportConfig(statusState?.status),
    [statusState?.status],
  );

  const isT3Home =
    pathname === '/' && statusState?.status?.home_page_theme === 'style_c';
  const fabClass = [
    'support-fab',
    `support-fab--${actualTheme === 'light' ? 'light' : 'dark'}`,
    isT3Home ? 'support-fab--t3' : '',
  ]
    .filter(Boolean)
    .join(' ');

  const scrollTop = () => {
    const reduced = window.matchMedia(
      '(prefers-reduced-motion: reduce)',
    ).matches;
    window.scrollTo({ top: 0, behavior: reduced ? 'auto' : 'smooth' });
  };

  const tgList = support?.telegramList || [];
  const wxList = support?.wechatList || [];
  const qqList = support?.qqList || [];

  const showTg = tgList.length > 0;
  const showWx = wxList.length > 0;
  const showQq = qqList.length > 0;
  const hasAny = showTg || showWx || showQq;

  // 渲染单个渠道的多二维码弹窗：单张无标题，多张网格布局。
  // Fallback：仅填描述无图时用描述文本生成占位二维码。
  const renderQrPopup = (list, makeFallbackQr) => {
    if (list.length === 1) {
      const { url, desc } = list[0];
      const qrSrc = url || (desc && makeFallbackQr ? makeFallbackQr(desc) : '');
      return (
        <div className='fab-qr'>
          {qrSrc ? <img src={qrSrc} loading='lazy' /> : null}
          {desc ? <span>{desc}</span> : <span></span>}
        </div>
      );
    }
    return (
      <div className='fab-qr fab-qr--multi'>
        <div className='fab-qr-grid' data-count={Math.min(list.length, 4)}>
          {list.slice(0, 4).map((item, i) => (
            <div className='fab-qr-item' key={i}>
              {item.url ? (
                <img src={item.url} loading='lazy' />
              ) : item.desc && makeFallbackQr ? (
                <img src={makeFallbackQr(item.desc)} loading='lazy' />
              ) : null}
              {item.desc ? <span>{item.desc}</span> : <span></span>}
            </div>
          ))}
        </div>
      </div>
    );
  };

  if (!hasAny && !showTop) {
    // 仅渲染返回顶部按钮
  }

  return (
    <div className={fabClass}>
      <div className='fab'>
        {showTg && (
          <div className='fab-contact'>
            <button className='fab-btn fab-tg' type='button'>
              {TG_ICON}
            </button>
            {renderQrPopup(tgList, (text) => qr(text))}
          </div>
        )}
        {showQq && (
          <div className='fab-contact'>
            <button className='fab-btn fab-qq' type='button'>
              QQ
            </button>
            {renderQrPopup(qqList, (text) => qr(text))}
          </div>
        )}
        {showWx && (
          <div className='fab-contact'>
            <button className='fab-btn fab-wx' type='button'>
              <svg
                viewBox='0 0 24 24'
                fill='currentColor'
                aria-hidden='true'
                width='20'
                height='20'
              >
                <path d='M9.5 4C5.36 4 2 6.91 2 10.5c0 1.86.95 3.53 2.46 4.67L4 18l2.86-1.64c.82.23 1.7.35 2.64.35.28 0 .56-.01.83-.04A6.5 6.5 0 0 1 10 14.5c0-3.59 3.36-6.5 7.5-6.5.24 0 .48.01.71.03C17.19 5.6 13.72 4 9.5 4zM7 8a1 1 0 1 1 0 2 1 1 0 0 1 0-2zm5 0a1 1 0 1 1 0 2 1 1 0 0 1 0-2zm5.5 3c-3.04 0-5.5 2.02-5.5 4.5s2.46 4.5 5.5 4.5c.73 0 1.42-.1 2.06-.28L22 21l-.43-1.87C22.79 18.34 23.5 17 23.5 15.5c0-2.48-2.46-4.5-6-4.5zm-2 2.25a.88.88 0 1 1 0 1.75.88.88 0 0 1 0-1.75zm4 0a.88.88 0 1 1 0 1.75.88.88 0 0 1 0-1.75z' />
              </svg>
            </button>
            {renderQrPopup(wxList)}
          </div>
        )}
        <button
          className={`fab-btn fab-top ${showTop ? 'fab-top--show' : ''}`}
          type='button'
          onClick={scrollTop}
        >
          <ArrowUp size={20} aria-hidden='true' />
        </button>
      </div>
    </div>
  );
};

export default SupportFab;
