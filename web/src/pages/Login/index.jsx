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

import React, { useContext, useEffect, useMemo, useRef, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import Turnstile from 'react-turnstile';
import { Button, Checkbox, Input, Modal } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { FaBolt, FaShieldHalved, FaStore } from 'react-icons/fa6';
import { SiGoogle } from 'react-icons/si';
import { IconKey, IconLock, IconMail } from '@douyinfe/semi-icons';

import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';
import TwoFAVerification from '../../components/auth/TwoFAVerification';
import WeChatIcon from '../../components/common/logo/WeChatIcon';
import channelImg from '../../../public/channel.svg';
import modelImg from '../../../public/model.svg';
import safeImg from '../../../public/safe.svg';
import avatarImg from '../../../public/avatar.svg';
import {
  API,
  getLogo,
  getSystemName,
  setUserData,
  showError,
  showInfo,
  showSuccess,
  updateAPI,
} from '../../helpers';
import './auth-v2.css';

const brandFeatureItems = [
  {
    imgUrl: modelImg,
    title: '50+ 模型，OpenAI 兼容接入',
    description: '集成全球领先模型，通过单一接口实现智能路由。',
  },
  {
    imgUrl: channelImg,
    title: '多渠道比价，自动路由最优',
    description: '毫秒级账单同步，深度优化您的 Token 使用效率。',
  },
  {
    imgUrl: safeImg,
    title: '自营品质保障，99.9% 可用性',
    description: '端到端加密通信，确保您的核心业务数据隐私无虞。',
  },
];

export default function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);
  const expiredNoticeShownRef = useRef(false);

  const logo = getLogo();
  const systemName = getSystemName();

  const status = useMemo(() => {
    if (statusState?.status) return statusState.status;
    const savedStatus = localStorage.getItem('status');
    if (!savedStatus) return {};
    try {
      return JSON.parse(savedStatus) || {};
    } catch {
      return {};
    }
  }, [statusState?.status]);

  const selfUseMode = Boolean(status?.self_use_mode_enabled);

  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');

  const [hasUserAgreement, setHasUserAgreement] = useState(false);
  const [hasPrivacyPolicy, setHasPrivacyPolicy] = useState(false);
  const [agreedToTerms, setAgreedToTerms] = useState(false);

  useEffect(() => {
    if (status?.turnstile_check) {
      setTurnstileEnabled(true);
      setTurnstileSiteKey(status.turnstile_site_key);
    } else {
      setTurnstileEnabled(false);
      setTurnstileSiteKey('');
      setTurnstileToken('');
    }
    setHasUserAgreement(status?.user_agreement_enabled || false);
    setHasPrivacyPolicy(status?.privacy_policy_enabled || false);
  }, [status]);

  useEffect(() => {
    const affCode = new URLSearchParams(window.location.search).get('aff');
    if (affCode) localStorage.setItem('aff', affCode);
  }, []);

  useEffect(() => {
    if (searchParams.get('expired') && !expiredNoticeShownRef.current) {
      expiredNoticeShownRef.current = true;
      showError(t('登录已过期，请重新登录'));
    }
  }, [searchParams, t]);

  const ensureTermsAccepted = () => {
    if ((hasUserAgreement || hasPrivacyPolicy) && !agreedToTerms) {
      showInfo(t('请先阅读并同意用户协议与隐私政策'));
      return false;
    }
    return true;
  };

  const ensureTurnstileReady = () => {
    if (turnstileEnabled && turnstileToken === '') {
      showInfo(t('Turnstile 校验未完成，请先完成验证'));
      return false;
    }
    return true;
  };

  const renderTermsInline = () => {
    if (!hasUserAgreement && !hasPrivacyPolicy) return null;
    return (
      <div className='terms-check'>
        <Checkbox
          checked={agreedToTerms}
          onChange={(e) => setAgreedToTerms(e.target.checked)}
        >
          <span>
            {t('我已阅读并同意')}
            {hasUserAgreement && (
              <>
                {' '}
                <a
                  href='/user-agreement'
                  target='_blank'
                  rel='noopener noreferrer'
                >
                  {t('用户协议')}
                </a>
              </>
            )}
            {hasUserAgreement && hasPrivacyPolicy && t('和')}
            {hasPrivacyPolicy && (
              <>
                {' '}
                <a
                  href='/privacy-policy'
                  target='_blank'
                  rel='noopener noreferrer'
                >
                  {t('隐私政策')}
                </a>
              </>
            )}
          </span>
        </Checkbox>
      </div>
    );
  };

  const [loginInputs, setLoginInputs] = useState({
    username: '',
    password: '',
  });
  const [loginLoading, setLoginLoading] = useState(false);

  const [showTwoFA, setShowTwoFA] = useState(false);
  const handle2FASuccess = (data) => {
    userDispatch({ type: 'login', payload: data });
    setUserData(data);
    updateAPI();
    setShowTwoFA(false);
    navigate('/console');
  };

  const handleLoginSubmit = async (e) => {
    e.preventDefault();
    if (!ensureTermsAccepted()) return;
    if (!ensureTurnstileReady()) return;

    const username = String(loginInputs.username || '').trim();
    const password = String(loginInputs.password || '');
    if (!username) {
      showInfo(t('请输入邮箱或用户名'));
      return;
    }
    if (!password) {
      showInfo(t('请输入密码'));
      return;
    }

    setLoginLoading(true);
    try {
      const res = await API.post(
        `/api/user/login?turnstile=${turnstileToken}`,
        {
          username,
          password,
        },
      );
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      if (data?.require_2fa) {
        setShowTwoFA(true);
        return;
      }
      userDispatch({ type: 'login', payload: data });
      setUserData(data);
      updateAPI();
      showSuccess(t('登录成功'));
      navigate('/console');
    } catch {
      showError(t('登录失败，请重试'));
    } finally {
      setLoginLoading(false);
    }
  };

  const handleGoogleLogin = () => {
    showInfo(t('Google 登录功能暂未开放'));
  };

  const [showWeChatLoginModal, setShowWeChatLoginModal] = useState(false);
  const [wechatLoading, setWechatLoading] = useState(false);
  const [wechatCodeSubmitLoading, setWechatCodeSubmitLoading] = useState(false);
  const [wechatVerificationCode, setWechatVerificationCode] = useState('');

  const onWeChatLoginClicked = () => {
    if (!ensureTermsAccepted()) return;
    setWechatLoading(true);
    setShowWeChatLoginModal(true);
    setWechatLoading(false);
  };

  const onSubmitWeChatVerificationCode = async () => {
    const code = String(wechatVerificationCode || '').trim();
    if (!code) {
      showInfo(t('请输入验证码'));
      return;
    }
    if (!ensureTurnstileReady()) return;
    setWechatCodeSubmitLoading(true);
    try {
      const res = await API.get(
        `/api/oauth/wechat?code=${encodeURIComponent(code)}`,
      );
      const { success, message, data } = res.data;
      if (success) {
        userDispatch({ type: 'login', payload: data });
        localStorage.setItem('user', JSON.stringify(data));
        setUserData(data);
        updateAPI();
        showSuccess(t('登录成功'));
        setShowWeChatLoginModal(false);
        navigate('/console');
      } else {
        showError(message);
      }
    } catch {
      showError(t('登录失败，请重试'));
    } finally {
      setWechatCodeSubmitLoading(false);
    }
  };

  const socialButtons = useMemo(
    () => [
      <Button
        key='google'
        type='tertiary'
        theme='outline'
        className='btn-social btn-google'
        size='large'
        onClick={handleGoogleLogin}
      >
        <span className='social-icon'>
          <SiGoogle size={18} />
        </span>
        {t('Google')}
      </Button>,
      <Button
        key='wechat'
        type='tertiary'
        theme='outline'
        className='btn-social btn-wechat'
        size='large'
        onClick={onWeChatLoginClicked}
        disabled={wechatLoading}
      >
        <span className='social-icon'>
          <WeChatIcon />
        </span>
        {t('微信登录')}
      </Button>,
    ],
    [handleGoogleLogin, onWeChatLoginClicked, wechatLoading, t],
  );

  return (
    <div className='auth-v2'>
      <div className='brand-panel'>
        <div className='floating-orb orb-1' />
        <div className='floating-orb orb-2' />
        <div className='floating-orb orb-3' />
        <div className='floating-orb orb-4' />

        <div className='brand-content'>
          <div className='brand-logo' onClick={() => navigate('/')}>
            <div className='logo-img'>
              {logo ? <img src={logo} alt={systemName} /> : null}
            </div>
            <span>{systemName}</span>
          </div>

          <div className='brand-tagline'>{t('智能API聚合')}</div>
          <p className='brand-desc'>
            {t(
              '一站式 AI 模型路由市场，自营渠道 + 第三方商家生态，为每次 API 调用找到最优路线。',
            )}
          </p>

          <div className='brand-features'>
            {brandFeatureItems.map((item) => {
              return (
                <div key={item.title} className='brand-feature'>
                  <div className='brand-feature-icon'>
                    <img src={item.imgUrl} alt={item.title} />
                  </div>
                  <div className='brand-feature-body'>
                    <div className='brand-feature-title'>{t(item.title)}</div>
                    <div className='brand-feature-desc'>
                      {t(item.description)}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        <div className='brand-quote-card'>
          <p>
            {t(
              'AllRouter.AI 彻底改变了我们团队调用多模型的方式。它不再是单纯的技术工具，而是我们决策流中的核心。',
            )}
          </p>
          <div className='brand-quote-author'>
            <span className='brand-quote-avatar'>
              <img src={avatarImg} alt='Avatar' />
            </span>
            <span>{t('技术负责人')} @ Visionary Lab</span>
          </div>
        </div>
      </div>

      <div className='form-panel'>
        <div className='form-container'>
          <Link to='/' className='mobile-logo'>
            {logo ? <img src={logo} alt={systemName} /> : null}
            <span>{systemName}</span>
          </Link>

          <div className='mobile-copy'>
            <h1>{t('智能API聚合')}</h1>
            <p>{t('统一接入主流模型与路由能力，在手机端也能快速完成登录。')}</p>
          </div>

          <div className='auth-heading'>
            <h2>{t('欢迎回来')}</h2>
            <p>{t('请输入您的凭据以访问控制台')}</p>
          </div>

          <div className='auth-tabs'>
            <Button
              type='tertiary'
              theme='borderless'
              className='auth-tab active'
              onClick={() => navigate('/login')}
            >
              {t('登录')}
            </Button>
            {!selfUseMode && (
              <Button
                type='tertiary'
                theme='borderless'
                className='auth-tab'
                onClick={() => navigate('/register')}
              >
                {t('注册账号')}
              </Button>
            )}
          </div>

          <div className='auth-view active' id='login'>
            <form onSubmit={handleLoginSubmit}>
              <div className='form-group'>
                <label className='form-label'>{t('电子邮箱')}</label>
                <Input
                  className='form-input'
                  size='large'
                  prefix={<IconMail />}
                  placeholder={t('name@company.com')}
                  value={loginInputs.username}
                  onChange={(value) =>
                    setLoginInputs((s) => ({ ...s, username: value }))
                  }
                  autoComplete='username'
                />
              </div>

              <div className='form-group'>
                <div className='form-label-row'>
                  <label className='form-label'>{t('密码')}</label>
                  <a
                    className='form-inline-link'
                    href='/reset'
                    onClick={(e) => {
                      e.preventDefault();
                      navigate('/reset');
                    }}
                  >
                    {t('忘记密码？')}
                  </a>
                </div>
                <Input
                  className='form-input'
                  size='large'
                  mode='password'
                  prefix={<IconLock />}
                  placeholder={t('请输入密码')}
                  value={loginInputs.password}
                  onChange={(value) =>
                    setLoginInputs((s) => ({ ...s, password: value }))
                  }
                  autoComplete='current-password'
                />
              </div>

              {renderTermsInline()}

              <Button
                htmlType='submit'
                type='primary'
                theme='solid'
                className='btn-submit'
                size='large'
                loading={loginLoading}
                disabled={
                  loginLoading ||
                  ((hasUserAgreement || hasPrivacyPolicy) && !agreedToTerms)
                }
              >
                <span className='btn-submit-text'>
                  {t('立即登录')}
                  <span className='submit-arrow'>→</span>
                </span>
              </Button>
            </form>

            {/* <div className='divider'>{t('第三方快速登录')}</div> */}
            {/* <div className='social-logins'>{socialButtons}</div> */}

            <div className='terms'>
              {t('点击登录即代表您同意我们的')}{' '}
              <a
                href='/user-agreement'
                target='_blank'
                rel='noopener noreferrer'
              >
                {t('服务条款')}
              </a>{' '}
              {t('和')}{' '}
              <a
                href='/privacy-policy'
                target='_blank'
                rel='noopener noreferrer'
              >
                {t('隐私政策')}
              </a>
              。
            </div>
          </div>

          <Modal
            title={t('微信扫码登录')}
            visible={showWeChatLoginModal}
            maskClosable={true}
            onOk={onSubmitWeChatVerificationCode}
            onCancel={() => setShowWeChatLoginModal(false)}
            okText={t('登录')}
            centered={true}
            okButtonProps={{ loading: wechatCodeSubmitLoading }}
          >
            <div style={{ textAlign: 'center' }}>
              {status.wechat_qrcode ? (
                <img
                  src={status.wechat_qrcode}
                  alt='wechat qrcode'
                  style={{ margin: '0 auto 12px', maxWidth: '100%' }}
                />
              ) : null}
              <div style={{ marginBottom: 12 }}>
                {t(
                  '微信扫码关注公众号，输入“验证码”获取验证码（3 分钟内有效）',
                )}
              </div>
            </div>

            <div className='form-group' style={{ marginBottom: 0 }}>
              <label className='form-label'>{t('验证码')}</label>
              <Input
                className='form-input'
                size='large'
                prefix={<IconKey />}
                placeholder={t('请输入验证码')}
                value={wechatVerificationCode}
                onChange={(value) => setWechatVerificationCode(value)}
              />
            </div>
          </Modal>

          <Modal
            title={t('两步验证')}
            visible={showTwoFA}
            onCancel={() => setShowTwoFA(false)}
            footer={null}
            width={450}
            centered
          >
            <TwoFAVerification
              onSuccess={handle2FASuccess}
              onBack={() => setShowTwoFA(false)}
              isModal={true}
            />
          </Modal>

          {turnstileEnabled && (
            <div className='turnstile-wrap'>
              <Turnstile
                sitekey={turnstileSiteKey}
                onVerify={(token) => setTurnstileToken(token)}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
