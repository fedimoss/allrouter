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
import { X } from 'lucide-react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { StatusContext } from '../../context/Status';
import { UserContext } from '../../context/User';
import LoginForm from './LoginForm';
import RegisterForm from './RegisterForm';

const AuthModal = () => {
  const location = useLocation();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [statusState] = React.useContext(StatusContext);
  const [userState] = React.useContext(UserContext);
  const registrationDisabled = Boolean(statusState?.status?.self_use_mode_enabled);
  const [visible, setVisible] = React.useState(
    () => location.pathname === '/login' || location.pathname === '/register',
  );
  const [mode, setMode] = React.useState(
    () => (location.pathname === '/register' ? 'register' : 'login'),
  );
  const originRef = React.useRef(
    location.pathname === '/login' || location.pathname === '/register'
      ? location.state?.from?.pathname || '/'
      : location.pathname,
  );

  React.useEffect(() => {
    // Directly visiting /register with registration disabled still shows the
    // modal, but in login mode. No navigation — the modal is route-agnostic.
    if (location.pathname === '/register' && registrationDisabled) {
      setMode('login');
    }
  }, [location.pathname, registrationDisabled]);

  React.useEffect(() => {
    const openAuth = (event) => {
      const requestedMode = event.detail?.mode === 'register' ? 'register' : 'login';
      if (requestedMode === 'register' && registrationDisabled) {
        setMode('login');
      } else {
        setMode(requestedMode);
      }
      originRef.current =
        location.pathname === '/login' || location.pathname === '/register'
          ? '/'
          : location.pathname;
      setVisible(true);
    };
    const interceptAuthLinks = (event) => {
      const anchor = event.target.closest?.('a');
      if (!anchor) return;
      const href = anchor.getAttribute('href');
      if (!href) return;
      let targetUrl;
      try {
        targetUrl = new URL(href, window.location.origin);
      } catch {
        return;
      }
      if (
        targetUrl.origin !== window.location.origin ||
        (targetUrl.pathname !== '/login' && targetUrl.pathname !== '/register')
      ) {
        return;
      }
      // Pure modal presentation: open the global auth dialog without touching
      // the route. Only a direct URL visit to /login|/register (deep link)
      // keeps the path in the address bar until the modal is closed.
      event.preventDefault();
      event.stopPropagation();
      const affCode = targetUrl.searchParams.get('aff');
      if (affCode) {
        localStorage.setItem('aff', affCode);
      }
      originRef.current =
        location.pathname === '/login' || location.pathname === '/register'
          ? location.state?.from?.pathname || '/'
          : location.pathname;
      setVisible(true);
      setMode(
        targetUrl.pathname === '/register' && !registrationDisabled
          ? 'register'
          : 'login',
      );
    };
    window.addEventListener('allrouter:open-auth', openAuth);
    document.addEventListener('click', interceptAuthLinks, true);
    return () => {
      window.removeEventListener('allrouter:open-auth', openAuth);
      document.removeEventListener('click', interceptAuthLinks, true);
    };
  }, [location.pathname, registrationDisabled]);

  React.useEffect(() => {
    if (location.pathname === '/login' || location.pathname === '/register') {
      if (location.state?.from?.pathname) {
        originRef.current = location.state.from.pathname;
      }
      setMode(location.pathname === '/register' && !registrationDisabled ? 'register' : 'login');
      setVisible(true);
    }
  }, [location.pathname, location.state, registrationDisabled]);

  React.useEffect(() => {
    if (userState?.user) {
      setVisible(false);
    }
  }, [userState?.user]);

  if (!visible) return null;

  const closeTarget = originRef.current || '/';

  const switchMode = (nextMode) => {
    if (nextMode === 'register' && registrationDisabled) return;
    setMode(nextMode === 'register' ? 'register' : 'login');
  };

  return (
    <div className='auth-modal-layer' role='presentation'>
      <section className='auth-modal' role='dialog' aria-modal='true'>
        <button
          type='button'
          className='auth-modal-close'
          onClick={() => {
            setVisible(false);
            if (location.pathname === '/login' || location.pathname === '/register') {
              navigate(closeTarget, { replace: true });
            }
          }}
          aria-label={t('关闭')}
        >
          <X size={18} />
        </button>
        <div className='auth-modal-copy'>
          <h1>{mode === 'login' ? t('欢迎回来') : t('创建账号')}</h1>
          <p>
            {mode === 'login'
              ? t('请输入您的凭据以访问控制台')
              : t('填写您的信息以开始使用控制台')}
          </p>
        </div>
        <div className='auth-modal-heading'>
          <div className='auth-modal-tabs' role='tablist'>
            <button
              type='button'
              className={mode === 'login' ? 'active' : ''}
              onClick={() => switchMode('login')}
            >
              {t('登录')}
            </button>
            {!registrationDisabled && (
              <button
                type='button'
                className={mode === 'register' ? 'active' : ''}
                onClick={() => switchMode('register')}
              >
                {t('注册')}
              </button>
            )}
          </div>
        </div>
        <div className='auth-modal-body'>
          {mode === 'login' ? (
            <LoginForm embedded onSwitch={switchMode} />
          ) : (
            <RegisterForm embedded onSwitch={switchMode} />
          )}
          <div className='auth-modal-terms'>
            {t(mode === 'login' ? '点击登录即代表您同意我们的' : '点击注册即代表您同意我们的')}{' '}
            <a href='/user-agreement' target='_blank' rel='noopener noreferrer'>{t('用户协议')}</a>{'、'}
            <a href='/service-clause' target='_blank' rel='noopener noreferrer'>{t('服务条款')}</a>{' '}
            {t('和')}{' '}
            <a href='/privacy-policy' target='_blank' rel='noopener noreferrer'>{t('隐私政策')}</a>{'。'}
          </div>
        </div>
      </section>
    </div>
  );
};

export default AuthModal;
