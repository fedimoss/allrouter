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

import React, { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  API,
  getLogo,
  showError,
  showInfo,
  showSuccess,
  getSystemName,
} from '../../helpers';
import Turnstile from 'react-turnstile';
import { Button, Input } from '@douyinfe/semi-ui';
import { IconMail } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import channelImg from '../../../public/channel.svg';
import modelImg from '../../../public/model.svg';
import safeImg from '../../../public/safe.svg';
import avatarImg from '../../../public/avatar.svg';
import '../../pages/Login/auth-v2.css';

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

const PasswordResetForm = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [inputs, setInputs] = useState({
    email: '',
  });
  const { email } = inputs;

  const [loading, setLoading] = useState(false);
  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');
  const [disableButton, setDisableButton] = useState(false);
  const [countdown, setCountdown] = useState(30);

  const logo = getLogo();
  const systemName = getSystemName();

  useEffect(() => {
    let status = localStorage.getItem('status');
    if (status) {
      status = JSON.parse(status);
      if (status.turnstile_check) {
        setTurnstileEnabled(true);
        setTurnstileSiteKey(status.turnstile_site_key);
      }
    }
  }, []);

  useEffect(() => {
    let countdownInterval = null;
    if (disableButton && countdown > 0) {
      countdownInterval = setInterval(() => {
        setCountdown(countdown - 1);
      }, 1000);
    } else if (countdown === 0) {
      setDisableButton(false);
      setCountdown(30);
    }
    return () => clearInterval(countdownInterval);
  }, [disableButton, countdown]);

  function handleChange(value) {
    setInputs((inputs) => ({ ...inputs, email: value }));
  }

  async function handleSubmit(e) {
    e.preventDefault();
    const trimmedEmail = String(email || '').trim();
    if (!trimmedEmail) {
      showError(t('请输入邮箱地址'));
      return;
    }
    if (turnstileEnabled && turnstileToken === '') {
      showInfo(t('请稍后几秒重试，Turnstile 正在检查用户环境！'));
      return;
    }
    setDisableButton(true);
    setLoading(true);
    try {
      const res = await API.get(
        `/api/reset_password?email=${encodeURIComponent(
          trimmedEmail,
        )}&turnstile=${turnstileToken}`,
      );
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('重置邮件发送成功，请检查邮箱！'));
        setInputs({ ...inputs, email: '' });
      } else {
        showError(message);
      }
    } catch {
      showError('重置邮件发送失败，请重试');
    }
    setLoading(false);
  }

  return (
    <div className='auth-v2 password-reset-page'>
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

          <h1 className='brand-tagline'>{t('智能API聚合')}</h1>
          <p className='brand-desc'>
            {t(
              '一站式 AI 模型路由市场，自营渠道 + 第三方商家生态，为每次 API 调用找到最优路线。',
            )}
          </p>

          <div className='brand-features'>
            {brandFeatureItems.map((item) => (
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
            ))}
          </div>
        </div>

        <div className='brand-quote-card'>
          <p>
            {systemName}&nbsp;
            {t(
              '彻底改变了我们团队调用多模型的方式。它不再是单纯的技术工具，而是我们决策流中的核心。',
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
        <div className='form-container password-reset-container'>
          <div className='mobile-copy'>
            <h1>{t('密码重置')}</h1>
            <p>{t('输入邮箱，我们将向您发送密码重置链接')}</p>
          </div>

          <div className='auth-heading password-reset-heading'>
            <h2>{t('密码重置')}</h2>
            <p>{t('输入邮箱，我们将向您发送密码重置链接')}</p>
          </div>

          <p className='password-reset-helper'>
            {t('请使用您注册 {{systemName}} 的邮箱地址。', { systemName })}
          </p>

          <form className='password-reset-form' onSubmit={handleSubmit}>
            <div className='form-group'>
              <label className='form-label'>{t('邮箱')}</label>
              <Input
                className='form-input'
                size='large'
                prefix={<IconMail />}
                placeholder={t('请输入您的邮箱地址')}
                value={email}
                onChange={handleChange}
                autoComplete='email'
              />
            </div>

            <Button
              htmlType='submit'
              type='primary'
              theme='solid'
              className='btn-submit password-reset-submit theme-btn-color'
              size='large'
              loading={loading}
              disabled={disableButton}
            >
              {disableButton ? `${t('重试')} (${countdown})` : t('提交')}
            </Button>
          </form>

          <div className='password-reset-login-link'>
            <span>{t('想起来了？')}</span>
            <Link to='/login'>{t('登录')}</Link>
          </div>

          {turnstileEnabled && (
            <div className='turnstile-wrap'>
              <Turnstile
                sitekey={turnstileSiteKey}
                onVerify={(token) => {
                  setTurnstileToken(token);
                }}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default PasswordResetForm;
