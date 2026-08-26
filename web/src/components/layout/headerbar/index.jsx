/* Copyright (C) 2025 QuantumNous */
import React, { useEffect, useMemo, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Menu, X } from 'lucide-react';
import { Button } from '@douyinfe/semi-ui';
import { useHeaderBar } from '../../../hooks/common/useHeaderBar';
import { useNotifications } from '../../../hooks/common/useNotifications';
import NoticeModal from '../NoticeModal';
import ActionButtons from './ActionButtons';
import UserArea from './UserArea';
import { shouldShowProviderAgentPartner, withBrowserBaseUrl } from '../../../helpers';

const HeaderBar = ({ onMobileMenuToggle, drawerOpen }) => {
  const { userState, statusState, isMobile, currentLang, isLoading, systemName, logo, isSelfUseMode, docsLink, theme, pricingRequireAuth, logout, handleLanguageChange, handleThemeToggle, navigate, t } = useHeaderBar({ onMobileMenuToggle, drawerOpen });
  const location = useLocation();
  const [mobileOpen, setMobileOpen] = useState(false);
  const { noticeVisible, unreadCount, handleNoticeOpen, handleNoticeClose, getUnreadKeys } = useNotifications(statusState);
  const loggedIn = Boolean(userState?.user);
  const docsLangPrefix = currentLang.startsWith('zh') ? 'zh' : 'en';
  const docsHref = docsLink || withBrowserBaseUrl(`/${docsLangPrefix}/docs`);
  const pricingTarget = !loggedIn && pricingRequireAuth ? '/login' : '/pricing';
  const showAgentPartner = shouldShowProviderAgentPartner(statusState?.status);
  const navLinks = useMemo(() => {
    if (loggedIn) {
      const links = [
        { label: t('数据看板'), to: '/console' },
        { label: t('令牌管理'), to: '/console/token' },
        { label: t('模型广场'), to: pricingTarget },
        { label: t('钱包'), to: '/console/topup' },
      ];
      if (showAgentPartner) links.push({ label: t('代理加盟'), to: '/agent-partner' });
      return links;
    }
    const links = [{ label: t('首页'), to: '/' }, { label: t('模型广场'), to: pricingTarget }];
    if (showAgentPartner) links.push({ label: t('代理加盟'), to: '/agent-partner' });
    links.push({ label: t('文档'), href: docsHref, external: true }, { label: t('关于'), to: '/about' });
    return links;
  }, [docsHref, loggedIn, pricingTarget, showAgentPartner, t]);
  useEffect(() => setMobileOpen(false), [location.pathname]);
  const isActive = (link) => link.to && (link.to === '/' ? location.pathname === '/' : location.pathname === link.to || location.pathname.startsWith(`${link.to}/`));
  const renderLink = (link, mobile = false) => {
    const className = `${isActive(link) ? 'app-header-link-active' : ''} ${mobile ? 'app-header-mobile-link' : ''}`;
    const content = <><span>{link.label}</span>{mobile && <span aria-hidden='true'>→</span>}</>;
    if (link.external) return <a key={link.label} href={link.href} target='_blank' rel='noreferrer' className={className} onClick={() => setMobileOpen(false)}>{content}</a>;
    return <Link key={link.label} to={link.to} className={className} onClick={() => setMobileOpen(false)}>{content}</Link>;
  };
  return (
    <>
      <header className='app-header'>
        <div className='app-header-inner'>
          <Link to='/' className='app-header-brand' aria-label={systemName}><span className='app-header-brand-logo'><img src={logo || '/logo.png'} alt='' /></span><span className='app-header-brand-name'>{systemName}</span></Link>
          <nav className='app-header-nav' aria-label={t('主导航')}>{navLinks.map((link) => renderLink(link))}</nav>
          <div className='app-header-actions'>
            <ActionButtons isNewYear={false} unreadCount={unreadCount} onNoticeOpen={handleNoticeOpen} theme={theme} onThemeToggle={handleThemeToggle} currentLang={currentLang} onLanguageChange={handleLanguageChange} t={t} />
            {loggedIn ? <UserArea userState={userState} isLoading={isLoading} isMobile={isMobile} isSelfUseMode={isSelfUseMode} logout={logout} navigate={navigate} t={t} /> : <Link className='app-header-login' to='/login'>{t('登录')}</Link>}
            <Button theme='borderless' type='tertiary' className='app-header-mobile-toggle' icon={mobileOpen ? <X size={22} /> : <Menu size={22} />} iconOnly onClick={() => setMobileOpen((open) => !open)} aria-label={mobileOpen ? t('关闭菜单') : t('打开菜单')} />
          </div>
        </div>
        <div className={`app-header-mobile-menu ${mobileOpen ? 'app-header-mobile-menu-open' : ''}`}><nav aria-label={t('移动端导航')}>{navLinks.map((link) => renderLink(link, true))}</nav></div>
      </header>
      <NoticeModal visible={noticeVisible} onClose={handleNoticeClose} isMobile={isMobile} defaultTab={unreadCount > 0 ? 'system' : 'inApp'} unreadKeys={getUnreadKeys()} />
    </>
  );
};
export default HeaderBar;
