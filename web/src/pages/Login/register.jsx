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
import { Link, useNavigate } from 'react-router-dom';
import Turnstile from 'react-turnstile';
import { Button, Checkbox, Input } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { FaBolt, FaShieldHalved, FaStore } from 'react-icons/fa6';
import {
  IconKey,
  IconLock,
  IconMail,
  IconUser,
} from '@douyinfe/semi-icons';

import { StatusContext } from '../../context/Status';
import {
  API,
  getLogo,
  getSystemName,
  showError,
  showInfo,
  showSuccess,
} from '../../helpers';
import './auth-v2.css';

function scorePasswordStrength(password) {
  if (!password) return { score: 0, levelClass: '', text: '', color: '' };

  let score = 0;
  if (password.length >= 8) score++;
  if (/[a-z]/.test(password) && /[A-Z]/.test(password)) score++;
  if (/\d/.test(password)) score++;
  if (/[^a-zA-Z0-9]/.test(password)) score++;

  const levels = [
    { cls: 'weak', text: '弱 · 建议增加长度和复杂度', color: '#ef4444' },
    { cls: 'medium', text: '一般 · 建议添加特殊字符', color: '#f59e0b' },
    { cls: 'strong', text: '强', color: '#10b981' },
    { cls: 'strong', text: '非常强', color: '#10b981' },
  ];

  const idx = Math.min(score, levels.length) - 1;
  if (idx < 0) return { score: 0, levelClass: '', text: '', color: '' };

  return {
    score,
    levelClass: levels[idx].cls,
    text: levels[idx].text,
    color: levels[idx].color,
  };
}

export default function RegisterPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [statusState] = useContext(StatusContext);

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
    if (selfUseMode) {
      navigate('/login', { replace: true });
    }
  }, [selfUseMode, navigate]);

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

  // ==================== Register ====================
  const [showEmailVerification, setShowEmailVerification] = useState(false);
  useEffect(() => {
    setShowEmailVerification(Boolean(status?.email_verification));
  }, [status?.email_verification]);

  const [registerInputs, setRegisterInputs] = useState({
    username: '',
    email: '',
    verification_code: '',
    password: '',
    password2: '',
  });
  const [registerLoading, setRegisterLoading] = useState(false);

  const [verificationCodeLoading, setVerificationCodeLoading] = useState(false);
  const [disableSendCode, setDisableSendCode] = useState(false);
  const [countdown, setCountdown] = useState(60);
  const [regEmailFlashError, setRegEmailFlashError] = useState(false);

  useEffect(() => {
    let timer = null;
    if (disableSendCode && countdown > 0) {
      timer = setInterval(() => setCountdown((s) => s - 1), 1000);
    } else if (disableSendCode && countdown <= 0) {
      setDisableSendCode(false);
      setCountdown(60);
    }
    return () => {
      if (timer) clearInterval(timer);
    };
  }, [disableSendCode, countdown]);

  const sendVerificationCode = async () => {
    const email = String(registerInputs.email || '').trim();
    if (!email) return;
    if (!email.includes('@')) {
      setRegEmailFlashError(true);
      setTimeout(() => setRegEmailFlashError(false), 2000);
      return;
    }
    if (!ensureTurnstileReady()) return;

    setVerificationCodeLoading(true);
    try {
      const res = await API.get(
        `/api/verification?email=${encodeURIComponent(
          email,
        )}&turnstile=${turnstileToken}`,
      );
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('验证码已发送，请查收'));
        setDisableSendCode(true);
      } else {
        showError(message);
      }
    } catch {
      showError(t('验证码发送失败，请重试'));
    } finally {
      setVerificationCodeLoading(false);
    }
  };

  const handleRegisterSubmit = async (e) => {
    e.preventDefault();
    if (!ensureTermsAccepted()) return;
    if (!ensureTurnstileReady()) return;

    const username = String(registerInputs.username || '').trim();
    const email = String(registerInputs.email || '').trim();
    const password = String(registerInputs.password || '');
    const password2 = String(registerInputs.password2 || '');
    const verificationCode = String(
      registerInputs.verification_code || '',
    ).trim();

    if (!username) return showInfo(t('请输入用户名'));
    if (!email || !email.includes('@')) {
      return showInfo(t('请输入正确的邮箱地址'));
    }
    if (showEmailVerification && !verificationCode) {
      return showInfo(t('请输入邮箱验证码'));
    }
    if (!password) return showInfo(t('请输入密码'));
    if (password !== password2) return showInfo(t('两次输入的密码不一致'));

    setRegisterLoading(true);
    try {
      const affCode = localStorage.getItem('aff') || '';
      const payload = {
        ...registerInputs,
        aff_code: affCode || undefined,
      };
      const res = await API.post(
        `/api/user/register?turnstile=${turnstileToken}`,
        payload,
      );
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('注册成功'));
        navigate('/login');
      } else {
        showError(message);
      }
    } catch {
      showError(t('注册失败，请重试'));
    } finally {
      setRegisterLoading(false);
    }
  };

  const strength = useMemo(
    () => scorePasswordStrength(registerInputs.password),
    [registerInputs.password],
  );

  return (
    <div className='auth-v2'>
      <div className='brand-panel'>
        <div className='floating-orb orb-1' />
        <div className='floating-orb orb-2' />
        <div className='floating-orb orb-3' />
        <div className='floating-orb orb-4' />

        <div className='brand-content'>
          <div className='brand-logo'>
            {logo ? <img src={logo} alt={systemName} /> : null}
            <span>{systemName}</span>
          </div>

          <h1 className='brand-tagline'>
            {t('AI API 路由市场')}
            <br />
            <span>{t('为每一次调用找到最优路径')}</span>
          </h1>
          <p className='brand-desc'>
            {t(
              '一站式 AI 模型路由市场，自营渠道 + 第三方商户生态，为每一次 API 调用找到最优路线。',
            )}
          </p>

          <div className='brand-features'>
            <div className='brand-feature'>
              <div className='brand-feature-icon'>
                <FaBolt />
              </div>
              <span>{t('50+ 模型，OpenAI 兼容接入')}</span>
            </div>
            <div className='brand-feature'>
              <div className='brand-feature-icon'>
                <FaStore />
              </div>
              <span>{t('多渠道比价，自动路由最优')}</span>
            </div>
            <div className='brand-feature'>
              <div className='brand-feature-icon'>
                <FaShieldHalved />
              </div>
              <span>{t('自营品质保障，99.9% 可用性')}</span>
            </div>
          </div>
        </div>
      </div>

      <div className='form-panel'>
        <div className='form-container'>
          <Link to='/' className='mobile-logo'>
            {logo ? <img src={logo} alt={systemName} /> : null}
            <span>{systemName}</span>
          </Link>

          <div className='auth-tabs'>
            <Button
              type='tertiary'
              theme='borderless'
              className='auth-tab'
              onClick={() => navigate('/login')}
            >
              {t('登录')}
            </Button>
            <Button
              type='tertiary'
              theme='borderless'
              className='auth-tab active'
            >
              {t('注册')}
            </Button>
          </div>

          <div className='auth-view active' id='register'>
            <form onSubmit={handleRegisterSubmit}>
              <div className='form-group'>
                <label className='form-label'>{t('用户名')}</label>
                <Input
                  className='form-input'
                  size='large'
                  prefix={<IconUser />}
                  placeholder={t('请输入用户名')}
                  value={registerInputs.username}
                  onChange={(value) =>
                    setRegisterInputs((s) => ({ ...s, username: value }))
                  }
                  autoComplete='username'
                />
              </div>

              <div className='form-group'>
                <label className='form-label'>{t('邮箱地址')}</label>
                <Input
                  className={`form-input ${
                    regEmailFlashError ? 'error-flash' : ''
                  }`}
                  size='large'
                  prefix={<IconMail />}
                  placeholder={t('请输入邮箱地址')}
                  value={registerInputs.email}
                  onChange={(value) =>
                    setRegisterInputs((s) => ({ ...s, email: value }))
                  }
                  autoComplete='email'
                />
              </div>

              {showEmailVerification && (
                <div className='form-group'>
                  <label className='form-label'>{t('邮箱验证码')}</label>
                  <div className='code-input-group'>
                    <Input
                      className='form-input'
                      size='large'
                      prefix={<IconKey />}
                      placeholder={t('请输入 6 位验证码')}
                      maxLength={6}
                      value={registerInputs.verification_code}
                      onChange={(value) =>
                        setRegisterInputs((s) => ({
                          ...s,
                          verification_code: value,
                        }))
                      }
                      autoComplete='one-time-code'
                    />
                    <Button
                      type='primary'
                      theme='outline'
                      className='btn-send-code'
                      size='large'
                      onClick={sendVerificationCode}
                      loading={verificationCodeLoading}
                      disabled={disableSendCode || verificationCodeLoading}
                    >
                      {disableSendCode
                        ? `${countdown}s`
                        : t('发送验证码')}
                    </Button>
                  </div>
                </div>
              )}

              <div className='form-group'>
                <label className='form-label'>{t('设置密码')}</label>
                <Input
                  className='form-input'
                  size='large'
                  mode='password'
                  prefix={<IconLock />}
                  placeholder={t('至少 8 位，包含字母和数字')}
                  value={registerInputs.password}
                  onChange={(value) =>
                    setRegisterInputs((s) => ({ ...s, password: value }))
                  }
                  autoComplete='new-password'
                />

                <div className='password-strength'>
                  {[0, 1, 2, 3].map((idx) => (
                    <div
                      key={idx}
                      className={`strength-bar ${
                        strength.score > idx ? strength.levelClass : ''
                      }`}
                    />
                  ))}
                </div>
                <div
                  className='strength-text'
                  style={{ color: strength.color || undefined }}
                >
                  {strength.text ? t(strength.text) : ''}
                </div>
              </div>

              <div className='form-group'>
                <label className='form-label'>{t('确认密码')}</label>
                <Input
                  className='form-input'
                  size='large'
                  mode='password'
                  prefix={<IconLock />}
                  placeholder={t('请再次输入密码')}
                  value={registerInputs.password2}
                  onChange={(value) =>
                    setRegisterInputs((s) => ({ ...s, password2: value }))
                  }
                  autoComplete='new-password'
                />
              </div>

              {renderTermsInline()}

              <Button
                htmlType='submit'
                type='primary'
                theme='solid'
                className='btn-submit'
                size='large'
                loading={registerLoading}
                disabled={
                  registerLoading ||
                  ((hasUserAgreement || hasPrivacyPolicy) && !agreedToTerms)
                }
              >
                {t('注册')}
              </Button>
            </form>

            <div className='terms'>
              {t('注册即表示您同意')}
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
            </div>
          </div>

          {turnstileEnabled && (
            <div
              style={{
                display: 'flex',
                justifyContent: 'center',
                marginTop: 18,
              }}
            >
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
