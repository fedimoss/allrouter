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

import React, {
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { ArrowRight, Menu, X } from 'lucide-react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  API,
  getLogo,
  getSystemName,
  shouldShowProviderAgentPartner,
  withBrowserBaseUrl,
} from '../../../../helpers';
import { StatusContext } from '../../../../context/Status';
import { UserContext } from '../../../../context/User';
import {
  useActualTheme,
  useSetTheme,
} from '../../../../context/Theme';
import UserArea from '../../../../components/layout/headerbar/UserArea';
import HeaderBar from '../../../../components/layout/headerbar';
import brandLogo from '../../../../../public/theme/theme3/allrouter-logo.svg';

import './index.css';

const BrandLockup = ({ logo, name }) => (
  <Link className='brand-lockup' to='/' aria-label={`${name} 首页`}>
    <img src={logo} alt='' width='32' height='32' />
    <span>{name}</span>
  </Link>
);

const DesignThemeToggle = () => {
  const setTheme = useSetTheme();
  const actualTheme = useActualTheme();
  const isChecked = actualTheme === 'light';

  const toggle = useCallback(() => {
    setTheme(isChecked ? 'dark' : 'light');
  }, [isChecked, setTheme]);

  const handleKeyDown = useCallback(
    (event) => {
      if (
        event.key === 'Enter' ||
        event.key === ' ' ||
        event.key === 'Spacebar'
      ) {
        event.preventDefault();
        toggle();
      }
    },
    [toggle],
  );

  return (
    <div
      className={`theme-toggle ${isChecked ? 't-is-checked' : ''}`}
      role='switch'
      tabIndex={0}
      aria-checked={isChecked}
      aria-label='切换深色 / 浅色模式'
      title='切换深色 / 浅色模式'
      onClick={toggle}
      onKeyDown={handleKeyDown}
    >
      <div className='theme-toggle__handle'>
        <span
          className='theme-toggle__icon theme-toggle__icon--sun'
          aria-hidden='true'
        >
          <svg
            viewBox='0 0 24 24'
            fill='none'
            stroke='currentColor'
            strokeWidth='2'
            strokeLinecap='round'
          >
            <circle cx='12' cy='12' r='4' />
            <path d='M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4' />
          </svg>
        </span>
        <span
          className='theme-toggle__icon theme-toggle__icon--moon'
          aria-hidden='true'
        >
          <svg
            viewBox='0 0 24 24'
            fill='none'
            stroke='currentColor'
            strokeWidth='2'
            strokeLinecap='round'
            strokeLinejoin='round'
          >
            <path d='M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z' />
          </svg>
        </span>
      </div>
    </div>
  );
};

const LANG_OPTIONS = [
  { value: 'zh-CN', label: '中文' },
  { value: 'en', label: 'EN' },
];

const DesignLangToggle = ({ currentLang, onChange }) => {
  const [open, setOpen] = useState(false);
  const wrapRef = useRef(null);
  const isZh = String(currentLang || '').toLowerCase().startsWith('zh');
  const active = isZh ? 'zh-CN' : 'en';
  const displayLabel = isZh ? '中文' : 'EN';

  useEffect(() => {
    if (!open) return undefined;
    const onDocumentClick = (event) => {
      if (wrapRef.current && !wrapRef.current.contains(event.target)) {
        setOpen(false);
      }
    };
    document.addEventListener('click', onDocumentClick);
    return () => document.removeEventListener('click', onDocumentClick);
  }, [open]);

  const select = (value) => {
    setOpen(false);
    if (value !== active) onChange(value);
  };

  return (
    <div className='lang-toggle' ref={wrapRef}>
      <button
        type='button'
        className={`lang-toggle__trigger ${open ? 'lang-toggle__trigger--open' : ''}`}
        aria-haspopup='listbox'
        aria-expanded={open}
        onClick={(event) => {
          event.stopPropagation();
          setOpen((value) => !value);
        }}
      >
        <span className='lang-toggle__label'>{displayLabel}</span>
        <svg
          viewBox='0 0 24 24'
          fill='none'
          stroke='currentColor'
          strokeWidth='2'
          strokeLinecap='round'
          strokeLinejoin='round'
          aria-hidden='true'
        >
          <polyline points='6 9 12 15 18 9' />
        </svg>
      </button>
      <div
        className={`lang-toggle__dropdown ${open ? 'lang-toggle__dropdown--open' : ''}`}
        role='listbox'
      >
        {LANG_OPTIONS.map((option) => (
          <div
            key={option.value}
            data-lang={option.value}
            role='option'
            aria-selected={active === option.value}
            className={`lang-toggle__option ${active === option.value ? 'lang-toggle__option--active' : ''}`}
            onClick={(event) => {
              event.stopPropagation();
              select(option.value);
            }}
          >
            {option.label}
          </div>
        ))}
      </div>
    </div>
  );
};

const HeaderView = ({
  logo,
  name,
  menuOpen,
  setMenuOpen,
  isLoggedIn,
  isSelfUseMode,
  currentLang,
  currentUser,
  handleLanguageChange,
  logout,
  navigate,
  t,
  navLinks,
  currentPath,
}) => {
  useEffect(() => {
    document.body.classList.toggle('menu-open', menuOpen);
    const onEscape = (event) => {
      if (event.key === 'Escape') setMenuOpen(false);
    };
    window.addEventListener('keydown', onEscape);
    return () => {
      document.body.classList.remove('menu-open');
      window.removeEventListener('keydown', onEscape);
    };
  }, [menuOpen, setMenuOpen]);

  const isLinkActive = (link) => {
    if (!link.to) return false;
    if (link.to === '/') return currentPath === '/';
    return currentPath === link.to || currentPath.startsWith(`${link.to}/`);
  };

  return (
    <>
      <header className='site-header'>
        <BrandLockup logo={logo} name={name} />
        <nav className='desktop-nav' aria-label='主导航'>
          {navLinks.map((link) => {
            const isActive = isLinkActive(link);
            return link.external ? (
              <a
                key={link.label}
                href={link.href}
                target='_blank'
                rel='noreferrer'
              >
                {link.label}
              </a>
            ) : (
              <Link
                key={link.label}
                to={link.to}
                className={isActive ? 'site-nav-link--active' : undefined}
                aria-current={isActive ? 'page' : undefined}
              >
                {link.label}
              </Link>
            );
          })}
        </nav>
        <div className='header-actions'>
          <DesignLangToggle
            currentLang={currentLang}
            onChange={handleLanguageChange}
          />
          <DesignThemeToggle />
          {isLoggedIn ? (
            <UserArea
              userState={{ user: currentUser }}
              isLoading={false}
              isMobile={false}
              isSelfUseMode={isSelfUseMode}
              logout={logout}
              navigate={navigate}
              t={t}
            />
          ) : (
            <Link className='login-btn' to='/login'>
              {t('登录')}
            </Link>
          )}
        </div>

        <button
          type='button'
          className='mobile-menu-button'
          aria-label={menuOpen ? '关闭菜单' : '打开菜单'}
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen((value) => !value)}
          title={menuOpen ? '关闭菜单' : '打开菜单'}
        >
          {menuOpen ? <X aria-hidden='true' /> : <Menu aria-hidden='true' />}
        </button>
      </header>

      <div
        className={`mobile-menu ${menuOpen ? 'mobile-menu--open' : ''}`}
        aria-hidden={!menuOpen}
      >
        <div className='mobile-menu-tools'>
          <DesignLangToggle
            currentLang={currentLang}
            onChange={handleLanguageChange}
          />
          <DesignThemeToggle />
        </div>
        <nav aria-label='移动端导航'>
          {navLinks.map((link, index) => {
            const isActive = isLinkActive(link);
            return link.external ? (
              <a
                key={link.label}
                href={link.href}
                onClick={() => setMenuOpen(false)}
                style={{ '--menu-index': index }}
                tabIndex={menuOpen ? 0 : -1}
                target='_blank'
                rel='noreferrer'
              >
                <span>{`0${index + 1}`}</span>
                {link.label}
                <ArrowRight size={22} aria-hidden='true' />
              </a>
            ) : (
              <Link
                key={link.label}
                to={link.to}
                className={isActive ? 'site-nav-link--active' : undefined}
                aria-current={isActive ? 'page' : undefined}
                onClick={() => setMenuOpen(false)}
                style={{ '--menu-index': index }}
                tabIndex={menuOpen ? 0 : -1}
              >
                <span>{`0${index + 1}`}</span>
                {link.label}
                <ArrowRight size={22} aria-hidden='true' />
              </Link>
            );
          })}
        </nav>
        <div className='mobile-menu-footer'>
          <span>ALL MODELS. ONE ROUTE.</span>
          <Link
            to='/console'
            tabIndex={menuOpen ? 0 : -1}
            onClick={() => setMenuOpen(false)}
          >
            {t('免费开始构建')}
          </Link>
        </div>
      </div>
    </>
  );
};

const LegacyTheme3Header = () => {
  const { t, i18n } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);
  const actualTheme = useActualTheme();
  const navigate = useNavigate();
  const location = useLocation();
  const [menuOpen, setMenuOpen] = useState(false);

  const currentUser = userState?.user || null;
  const isLoggedIn = Boolean(currentUser?.id);
  const isSelfUseMode = statusState?.status?.self_use_mode_enabled || false;
  const logo = getLogo() || brandLogo;
  const systemName = getSystemName() || 'AllRouter.AI';
  const docsLink = statusState?.status?.docs_link || '';
  const docsLangPrefix = i18n.language.startsWith('zh') ? 'zh' : 'en';
  const docsHref = docsLink || withBrowserBaseUrl(`/${docsLangPrefix}/docs`);

  const headerNavModules = useMemo(() => {
    const config = statusState?.status?.HeaderNavModules;
    if (!config) return null;
    try {
      const modules = typeof config === 'string' ? JSON.parse(config) : config;
      if (typeof modules.pricing === 'boolean') {
        modules.pricing = {
          enabled: modules.pricing,
          requireAuth: false,
        };
      }
      return modules;
    } catch {
      return null;
    }
  }, [statusState?.status?.HeaderNavModules]);

  const pricingRequireAuth = useMemo(() => {
    if (!headerNavModules?.pricing) return false;
    return typeof headerNavModules.pricing === 'object'
      ? headerNavModules.pricing.requireAuth
      : false;
  }, [headerNavModules]);

  const consoleNavTarget = isLoggedIn ? '/console' : '/login';
  const pricingNavTarget =
    !isLoggedIn && pricingRequireAuth ? '/login' : '/pricing';
  const showAgentPartnerNav = shouldShowProviderAgentPartner(
    statusState?.status,
  );

  const navLinks = useMemo(() => {
    const links = [
      { label: t('首页'), to: '/' },
      { label: t('控制台'), to: consoleNavTarget },
      { label: t('模型广场'), to: pricingNavTarget },
    ];
    if (showAgentPartnerNav) {
      links.push({ label: t('代理加盟'), to: '/agent-partner' });
    }
    links.push({ label: t('文档'), href: docsHref, external: true });
    links.push({ label: t('关于'), to: '/about' });
    return links;
  }, [
    consoleNavTarget,
    docsHref,
    pricingNavTarget,
    showAgentPartnerNav,
    t,
  ]);

  const handleLanguageChange = useCallback(
    async (language) => {
      i18n.changeLanguage(language);
      try {
        localStorage.setItem('i18nextLng', language);
      } catch {
        // Ignore unavailable browser storage.
      }
      if (!currentUser?.id) return;
      try {
        const response = await API.put('/api/user/self', {
          language,
        });
        if (response.data.success && currentUser?.setting) {
          const settings = JSON.parse(currentUser.setting);
          settings.language = language;
          const nextUser = {
            ...currentUser,
            setting: JSON.stringify(settings),
          };
          userDispatch({ type: 'login', payload: nextUser });
          localStorage.setItem('user', JSON.stringify(nextUser));
        }
      } catch (error) {
        console.error('Failed to save language preference:', error);
      }
    },
    [currentUser, i18n, userDispatch],
  );

  const logout = useCallback(async () => {
    await API.get('/api/user/logout');
    userDispatch({ type: 'logout' });
    localStorage.removeItem('user');
    navigate('/login');
  }, [navigate, userDispatch]);

  return (
    <div
      className={`t3-app theme3-header-host ${
        actualTheme === 'light' ? 't3-app--light' : ''
      }`}
    >
      <HeaderView
        logo={logo}
        name={systemName}
        menuOpen={menuOpen}
        setMenuOpen={setMenuOpen}
        isLoggedIn={isLoggedIn}
        isSelfUseMode={isSelfUseMode}
        currentLang={i18n.language}
        currentUser={currentUser}
        handleLanguageChange={handleLanguageChange}
        logout={logout}
        navigate={navigate}
        t={t}
        navLinks={navLinks}
        currentPath={location.pathname}
      />
    </div>
  );
};

const Theme3Header = () => <HeaderBar />;

export default Theme3Header;
