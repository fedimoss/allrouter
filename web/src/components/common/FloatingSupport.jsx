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
import { useTranslation } from 'react-i18next';
import ConfigurationTutorial from '../layout/headerbar/ConfigurationTutorial';

const normalize = (value = '') => String(value).trim();

// 实心书本图标，与静态页 global-components 的配置教程按钮保持一致
const BookIcon = () => (
  <svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'>
    <path d='M20 5h-8a3 3 0 0 0-3-3H4a2 2 0 0 0-2 2v15a2 2 0 0 0 2 2h5.5c1.1 0 2.1.45 2.83 1.17A2.83 2.83 0 0 1 13.5 19h6.5a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2zM5 3h4a1 1 0 0 1 1 1v9.5H5V3zm12 14h-2v-2h2v2zm0-4h-2V7h2v6z' />
  </svg>
);

const TelegramIcon = () => (
  <svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'>
    <path d='M21.9 4.4 2.8 11.9c-.95.38-.9 1.66.08 1.98l4.76 1.52 1.83 5.72c.28.86 1.37 1.03 1.93.34l2.53-3.18 4.98 3.67c.74.55 1.8.14 2.01-.75l3.06-15.8c.2-1.03-.84-1.86-1.78-1.44ZM8.6 14.1l8.7-7.3c.22-.18.45.16.27.36l-7.2 6.9-.3 3.06-1.45-3.02Z' />
  </svg>
);

const QQIcon = () => (
  <svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'>
    <path d='M21.395 15.035a39.548 39.548 0 0 0-.803-2.264l-1.079-2.695c.001-.032.014-.562.014-.836C19.526 4.632 17.351 0 12 0S4.474 4.632 4.474 9.241c0 .274.013.804.014.836l-1.08 2.695a38.97 38.97 0 0 0-.802 2.264c-1.021 3.283-.69 4.643-.438 4.673.54.065 2.103-2.472 2.103-2.472 0 1.469.756 3.387 2.394 4.771-.612.188-1.363.479-1.845.835-.434.32-.379.646-.301.778.343.578 5.883.369 7.482.189 1.6.18 7.14.389 7.483-.189.078-.132.132-.458-.301-.778-.483-.356-1.233-.646-1.846-.836 1.637-1.384 2.393-3.302 2.393-4.771 0 0 1.563 2.537 2.103 2.472.251-.03.581-1.39-.438-4.673zM12.662 4.846c.039-1.052.659-1.878 1.385-1.846s1.281.912 1.242 1.964c-.039 1.051-.659 1.878-1.385 1.846s-1.282-.912-1.242-1.964zM9.954 3c.725-.033 1.345.794 1.384 1.846.04 1.052-.517 1.931-1.242 1.963-.726.033-1.346-.794-1.385-1.845C8.672 3.912 9.228 3.033 9.954 3zM7.421 8.294c.194-.43 2.147-.908 4.566-.908h.026c2.418 0 4.372.479 4.566.908a.14.14 0 0 1 .014.061c0 .031-.01.059-.026.083-.163.238-2.333 1.416-4.553 1.416h-.026c-2.221 0-4.39-1.178-4.553-1.416a.136.136 0 0 1-.014-.144zm10.422 8.622c-.22 3.676-2.403 5.987-5.774 6.021h-.137c-3.37-.033-5.554-2.345-5.773-6.021-.081-1.35.001-2.496.147-3.43.318.063.638.122.958.176v3.506s1.658.334 3.318.103v-3.225c.488.027.96.04 1.406.034h.025c1.678.021 3.714-.204 5.683-.594.146.934.227 2.08.147 3.43zM10.48 5.804c.313-.041.542-.409.508-.825-.033-.415-.314-.72-.629-.679-.313.04-.541.409-.508.824.034.417.315.72.629.68zM14.479 5.156c.078.037.221.042.289-.146.035-.095.025-.165-.009-.214-.023-.033-.133-.118-.371-.176-.904-.22-1.341.384-1.405.499-.04.072-.012.176.056.227.067.051.139.037.179-.006.58-.628 1.21-.208 1.261-.184z' />
  </svg>
);

const WeChatIcon = () => (
  <svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'>
    <path d='M9.5 4C5.36 4 2 6.91 2 10.5c0 1.86.95 3.53 2.46 4.67L4 18l2.86-1.64c.82.23 1.7.35 2.64.35.28 0 .56-.01.83-.04A6.5 6.5 0 0 1 10 14.5c0-3.59 3.36-6.5 7.5-6.5.24 0 .48.01.71.03C17.19 5.6 13.72 4 9.5 4zM7 8a1 1 0 1 1 0 2 1 1 0 0 1 0-2zm5 0a1 1 0 1 1 0 2 1 1 0 0 1 0-2zm5.5 3c-3.04 0-5.5 2.02-5.5 4.5s2.46 4.5 5.5 4.5c.73 0 1.42-.1 2.06-.28L22 21l-.43-1.87C22.79 18.34 23.5 17 23.5 15.5c0-2.48-2.46-4.5-6-4.5zm-2 2.25a.88.88 0 1 1 0 1.75.88.88 0 0 1 0-1.75zm4 0a.88.88 0 1 1 0 1.75.88.88 0 0 1 0-1.75z' />
  </svg>
);

const SupportChannel = ({ channel, open, onToggle }) => (
  <div className={`floating-support-item ${open ? 'open' : ''}`}>
    <button
      type='button'
      className={`floating-support-button floating-support-button--${channel.key}`}
      aria-label={channel.label}
      aria-expanded={open}
      onClick={(event) => {
        event.stopPropagation();
        onToggle(channel.key);
      }}
    >
      {channel.icon}
    </button>
    <div className='floating-support-card'>
      {channel.image ? (
        <img src={channel.image} alt={channel.alt} loading='lazy' />
      ) : null}
      <span>{channel.text}</span>
    </div>
  </div>
);

const FloatingSupport = ({
  wechatQRCode,
  wechatDesc,
  qqQrcode,
  qqSupport,
  telegramQRCode,
  telegramDesc,
}) => {
  const { t } = useTranslation();
  const containerRef = useRef(null);
  const [openChannel, setOpenChannel] = useState(null);
  const wechatImage = normalize(wechatQRCode);
  const wechatText = normalize(wechatDesc);
  const qqImage = normalize(qqQrcode);
  const qqText = normalize(qqSupport);
  const telegramImage = normalize(telegramQRCode);
  const telegramText = normalize(telegramDesc);

  const showWechat = Boolean(wechatImage || wechatText);
  const showQQ = Boolean(qqImage || qqText);
  const showTelegram = Boolean(telegramImage || telegramText);

  useEffect(() => {
    const close = (event) => {
      if (containerRef.current && !containerRef.current.contains(event.target)) {
        setOpenChannel(null);
      }
    };
    const closeOnEscape = (event) => {
      if (event.key === 'Escape') setOpenChannel(null);
    };
    document.addEventListener('click', close);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('click', close);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, []);

  const channels = [
    showTelegram && {
      key: 'telegram',
      label: t('Telegram客服'),
      alt: t('Telegram二维码'),
      image: telegramImage,
      text: telegramText || t('联系我们'),
      icon: <TelegramIcon />,
    },
    showWechat && {
      key: 'wechat',
      label: t('微信客服'),
      alt: t('微信客服二维码'),
      image: wechatImage,
      text: wechatText || t('联系我们'),
      icon: <WeChatIcon />,
    },
    showQQ && {
      key: 'qq',
      label: t('QQ客服'),
      alt: t('QQ二维码'),
      image: qqImage,
      text: qqText || t('联系我们'),
      icon: <QQIcon />,
    },
  ].filter(Boolean);

  const toggleChannel = (key) => {
    setOpenChannel((current) => (current === key ? null : key));
  };

  return (
    <div ref={containerRef} className='floating-support' aria-label={t('联系我们')}>
      {channels.map((channel) => (
        <SupportChannel
          key={channel.key}
          channel={channel}
          open={openChannel === channel.key}
          onToggle={toggleChannel}
        />
      ))}
      <ConfigurationTutorial
        renderTrigger={(openTutorial) => (
          <div className='floating-support-item floating-support-item--tutorial'>
            <button
              type='button'
              className='floating-support-button floating-support-button--tutorial'
              onClick={() => {
                setOpenChannel(null);
                openTutorial();
              }}
              aria-label={t('配置教程')}
              title={t('配置教程')}
            >
              <BookIcon />
            </button>
            <div className='floating-support-card'>
              <span>{t('配置教程')}</span>
            </div>
          </div>
        )}
      />
    </div>
  );
};

export default FloatingSupport;
