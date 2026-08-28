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

import HeaderBar from './headerbar';
import { Layout } from '@douyinfe/semi-ui';
import SiderBar from './SiderBar';
import App from '../../App';
import AuthModal from '../auth/AuthModal';
import FloatingSupport from '../common/FloatingSupport';
import SiteFooter from '../common/SiteFooter';
import { ToastContainer } from 'react-toastify';
import React, { useContext, useEffect, useMemo, useState } from 'react';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { useSidebarCollapsed } from '../../hooks/common/useSidebarCollapsed';
import { useTranslation } from 'react-i18next';
import {
  API,
  applyBranding,
  buildSupportConfig,
  showError,
  setStatusData,
  applyThemeColors,
  extractThemeColors,
} from '../../helpers';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';
import { useLocation } from 'react-router-dom';
const { Sider, Content, Header } = Layout;

const PageLayout = () => {
  const [, userDispatch] = useContext(UserContext);
  const [statusState, statusDispatch] = useContext(StatusContext);
  const isMobile = useIsMobile();
  const [collapsed, , setCollapsed] = useSidebarCollapsed();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const { i18n } = useTranslation();
  const location = useLocation();

  const selfContainedPages = [
    '/agent-partner',
    '/about',
    '/landing',
    '/payment/lakala/qrcode',
    '/userQuestion',
  ];
  const shouldInnerPadding =
    location.pathname.includes('/console') &&
    !location.pathname.startsWith('/console/chat') &&
    location.pathname !== '/console/playground';

  const isConsoleRoute = location.pathname.startsWith('/console');
  const isPlaygroundRoute = location.pathname === '/console/playground';
  const isChatRoute = location.pathname.startsWith('/console/chat');
  const useConstrainedConsoleContent =
    isConsoleRoute && !isPlaygroundRoute && !isChatRoute;
  const isDocsRoute =
    location.pathname === '/docs' || location.pathname.startsWith('/docs/');
  // Console navigation is hosted in the authenticated user dropdown.
  const showSider = false;
  const authRoutesWithoutHeader = ['/reset', '/user/reset'];
  const shouldShowHeader =
    !authRoutesWithoutHeader.includes(location.pathname) &&
    !selfContainedPages.includes(location.pathname);
  const shouldSplitConsoleLayout = false;

  // 客服/配置教程悬浮球：全局显示，所有页面共用
  const supportConfig = useMemo(
    () => buildSupportConfig(statusState?.status),
    [statusState?.status],
  );
  const floatingSupport = (
    <FloatingSupport
      wechatQRCode={supportConfig.wechatQRCode}
      wechatDesc={supportConfig.wechatDesc}
      qqQrcode={supportConfig.qqQrcode}
      qqSupport={supportConfig.qqSupport}
      telegramQRCode={supportConfig.telegramQRCode}
      telegramDesc={supportConfig.telegramDesc}
    />
  );

  useEffect(() => {
    if (isMobile && drawerOpen && collapsed) {
      setCollapsed(false);
    }
  }, [isMobile, drawerOpen, collapsed, setCollapsed]);

  const loadUser = () => {
    let user = localStorage.getItem('user');
    if (user) {
      let data = JSON.parse(user);
      userDispatch({ type: 'login', payload: data });
    }
  };

  const loadStatus = async () => {
    try {
      const [statusRes, webColorsRes] = await Promise.allSettled([
        API.get('/api/status'),
        API.get('/api/web_colors'),
      ]);
      if (webColorsRes.status === 'fulfilled') {
        const { primaryColor, secondaryColor, buttonTextColor } = extractThemeColors(
          webColorsRes.value,
        );
        if (primaryColor || secondaryColor || buttonTextColor) {
          applyThemeColors(primaryColor, secondaryColor, buttonTextColor);
        }
      }
      if (statusRes.status !== 'fulfilled') {
        throw statusRes.reason;
      }
      const { success, data } = statusRes.value.data;
      if (success) {
        statusDispatch({ type: 'set', payload: data });
        setStatusData(data);
      } else {
        showError('Unable to connect to server');
      }
    } catch (error) {
      showError('Failed to load status');
    }
  };

  useEffect(() => {
    applyBranding();
    loadUser();
    loadStatus()
      .then(() => applyBranding())
      .catch(console.error);
    const savedLang = localStorage.getItem('i18nextLng');
    if (savedLang) {
      i18n.changeLanguage(savedLang);
    }
  }, [i18n]);

  if (shouldSplitConsoleLayout) {
    return (
      <Layout
        className='app-layout'
        style={{
          display: 'flex',
          flexDirection: 'row',
          overflow: 'hidden',
        }}
      >
        {showSider && (
          <Sider
            className='app-sider'
            style={{
              position: 'relative',
              left: 'auto',
              top: 0,
              zIndex: 2,
              border: 'none',
              paddingRight: '0',
              width: 'var(--sidebar-current-width)',
              flex: '0 0 var(--sidebar-current-width)',
              height: '100vh',
            }}
          >
            <SiderBar onNavigate={() => {}} />
          </Sider>
        )}
        <Layout
          style={{
            minWidth: 0,
            flex: '1 1 auto',
            display: 'flex',
            flexDirection: 'column',
            overflow: 'hidden',
          }}
        >
          {shouldShowHeader && (
            <Header
              style={{
                padding: 0,
                height: 'auto',
                lineHeight: 'normal',
                position: 'relative',
                width: '100%',
                top: 'auto',
                zIndex: 3,
                flex: '0 0 auto',
              }}
            >
              <HeaderBar
                onMobileMenuToggle={() => setDrawerOpen((prev) => !prev)}
                drawerOpen={drawerOpen}
              />
            </Header>
          )}
          <Content
            style={{
              flex: '1 1 auto',
              overflowY: isDocsRoute ? 'visible' : 'auto',
              WebkitOverflowScrolling: 'touch',
              padding: shouldInnerPadding ? '24px' : '0',
              position: 'relative',
              minWidth: 0,
            }}
          >
            <div
              className={
                useConstrainedConsoleContent ? 'console-content-shell' : undefined
              }
              style={{
                minHeight: isPlaygroundRoute ? undefined : '100%',
                height: isPlaygroundRoute ? '100%' : undefined,
                overflow: isPlaygroundRoute ? 'hidden' : undefined,
                display: 'flex',
                flexDirection: 'column',
              }}
            >
              <div
                style={{
                  flex: '1 0 auto',
                  height: isPlaygroundRoute ? '100%' : undefined,
                  overflow: isPlaygroundRoute ? 'hidden' : undefined,
                }}
              >
                <App />
              </div>
              {/* 全站统一页脚：所有页面展示；CTA 卡片仅首页显示 */}
              <SiteFooter showCta={location.pathname === '/'} />
            </div>
          </Content>
        </Layout>
        <ToastContainer />
        <AuthModal />
        {floatingSupport}
      </Layout>
    );
  }

  return (
    <Layout
      className='app-layout'
      style={{
        display: 'flex',
        flexDirection: 'column',
        overflow: isMobile ? 'visible' : 'hidden',
      }}
    >
      {shouldShowHeader && (
        <Header
          style={{
            padding: 0,
            height: 'auto',
            lineHeight: 'normal',
            position: 'fixed',
            width: '100%',
            left: 0,
            top: 0,
            zIndex: 100,
          }}
        >
          <HeaderBar
            onMobileMenuToggle={() => setDrawerOpen((prev) => !prev)}
            drawerOpen={drawerOpen}
          />
        </Header>
      )}
      <Layout
        style={{
          overflow: isMobile ? 'visible' : 'auto',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        {showSider && (
          <Sider
            className='app-sider'
            style={{
              position: 'fixed',
              left: 0,
              top: '64px',
              zIndex: 99,
              border: 'none',
              paddingRight: '0',
              width: 'var(--sidebar-current-width)',
            }}
          >
            <SiderBar
              onNavigate={() => {
                if (isMobile) setDrawerOpen(false);
              }}
            />
          </Sider>
        )}
        <Layout
          style={{
            marginLeft: isMobile
              ? '0'
              : showSider
                ? 'var(--sidebar-current-width)'
                : '0',
            flex: '1 1 auto',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          <Content
            style={{
              flex: '1 0 auto',
              overflowY: isMobile || isDocsRoute ? 'visible' : 'hidden',
              WebkitOverflowScrolling: 'touch',
              padding: useConstrainedConsoleContent
                ? '0'
                : shouldInnerPadding
                  ? isMobile
                    ? '5px'
                    : '24px'
                  : '0',
              paddingTop: shouldShowHeader
                ? isMobile
                  ? '68px'
                  : shouldInnerPadding
                    ? '88px'
                    : '68px'
                : undefined,
              position: 'relative',
            }}
          >
            <div
              className={
                useConstrainedConsoleContent ? 'console-content-shell' : undefined
              }
              style={{
                minHeight: isPlaygroundRoute ? undefined : '100%',
                height: isPlaygroundRoute ? '100%' : undefined,
                overflow: isPlaygroundRoute ? 'hidden' : undefined,
                display: 'flex',
                flexDirection: 'column',
              }}
            >
              <div
                style={{
                  flex: '1 0 auto',
                  height: isPlaygroundRoute ? '100%' : undefined,
                  overflow: isPlaygroundRoute ? 'hidden' : undefined,
                }}
              >
                <App />
              </div>
              {/* 全站统一页脚：所有页面展示；CTA 卡片仅首页显示 */}
              <SiteFooter showCta={location.pathname === '/'} />
            </div>
          </Content>
        </Layout>
      </Layout>
      <ToastContainer />
      <AuthModal />
      {floatingSupport}
    </Layout>
  );
};

export default PageLayout;
