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

/*
 * 微信登录确认页
 *
 * 手机上扫码后打开的页面：由 /wechat/login/confirm?state=xxx 进入，
 * 用户点击「确认在此设备登录」后调用后端完成确认，PC 端轮询随即登录成功。
 *
 * 样式说明（移动端优先）：
 *   - 用 min-h-[100dvh] 而非 100vh，避免移动端浏览器地址栏导致的抖动；
 *     本页已在 PageLayout 的 selfContainedPages 中登记，不渲染站点头尾，
 *     页面高度正好是一屏，不会再出现多于视口的外层滚动条；
 *   - 卡片走自然高度 + 内边距，不再写死高度，多语言长文案不会溢出；
 *   - 内边距与背景兜住 iOS 刘海/Home 指示条（safe-area-inset）；
 *   - 主按钮 block 撑满宽度，手机上更好点按。
 */

import React, { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { Button } from '@douyinfe/semi-ui';
import { IconAlertTriangle, IconTick } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { FaWeixin } from 'react-icons/fa6';
import { API, showError, showSuccess } from '../../helpers';

// 页面外壳：渐变底 + 顶部一抹微信绿光晕 + 居中卡片。
//
// 光晕用元素自身的背景渐变实现，而不是另起一个 blur 的绝对定位 div：
// blur 属于「墨水溢出」，在窄屏上会被计入可滚动区域，凭空多出横向滚动条；
// 背景渐变不参与布局，永远不会有这个问题。
const ConfirmShell = ({ badge, children }) => (
  <div
    className='flex w-full items-center justify-center overflow-y-auto overflow-x-hidden px-4'
    style={{
      paddingTop: 'max(2.5rem, env(safe-area-inset-top))',
      paddingBottom: 'max(2.5rem, env(safe-area-inset-bottom))',
    }}
  >
    <div className='w-full max-w-[400px] rounded-2xl border border-semi-color-border bg-semi-color-bg-2 px-6 py-9 text-center shadow-xl sm:px-8 sm:py-10'>
      {badge}
      {children}
    </div>
  </div>
);

// 顶部圆形图标徽标。
const Badge = ({ tone, children }) => (
  <div
    className='mx-auto flex h-14 w-14 items-center justify-center rounded-full text-white sm:h-16 sm:w-16'
    style={{
      background:
        tone === 'success'
          ? 'linear-gradient(135deg, #3ddc84 0%, #07c160 100%)'
          : 'linear-gradient(135deg, #2ad06a 0%, #07c160 100%)',
      boxShadow: '0 10px 24px -10px rgba(7, 193, 96, 0.75)',
    }}
  >
    {children}
  </div>
);

const WechatLoginConfirm = () => {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const [state, setState] = useState('');
  const [confirming, setConfirming] = useState(false);
  const [confirmed, setConfirmed] = useState(false);

  useEffect(() => {
    setState(String(searchParams.get('state') || '').trim());
  }, [searchParams]);

  const confirmLogin = async () => {
    if (!state || confirming || confirmed) return;
    setConfirming(true);
    try {
      const { data } = await API.post('/api/oauth/wechat/confirm', { state });
      if (!data.success) {
        throw new Error(
          data.message || t('微信登录二维码已过期，请重新获取。'),
        );
      }
      setConfirmed(true);
      showSuccess(t('已确认，请返回发起登录的页面。'));
    } catch (error) {
      showError(
        error.response?.data?.message ||
          error.message ||
          t('确认失败，请重新扫码。'),
      );
    } finally {
      setConfirming(false);
    }
  };

  if (!state) {
    return (
      <ConfirmShell
        badge={
          <Badge tone='warning'>
            <IconAlertTriangle size={28} />
          </Badge>
        }
      >
        <h1 className='mt-5 text-base font-semibold text-semi-color-text-0 sm:text-lg'>
          {t('微信登录二维码已过期，请重新获取。')}
        </h1>
        <Link
          to='/login'
          className='mt-7 inline-flex h-10 w-full items-center justify-center rounded-[10px] border border-semi-color-border text-sm text-semi-color-text-1 no-underline transition-colors hover:bg-semi-color-fill-0'
        >
          {t('返回登录')}
        </Link>
      </ConfirmShell>
    );
  }

  return (
    <ConfirmShell
      badge={
        <Badge tone={confirmed ? 'success' : 'default'}>
          {confirmed ? <IconTick size={30} /> : <FaWeixin size={30} />}
        </Badge>
      }
    >
      <h1 className='mt-5 text-lg font-semibold text-semi-color-text-0 sm:text-xl'>
        {t('微信扫码登录')}
      </h1>
      <p className='mt-2 text-sm leading-6 text-semi-color-text-2'>
        {t('请确认使用当前微信账号登录此设备。')}
      </p>

      <div className='mt-7'>
        {confirmed ? (
          <div className='flex items-center justify-center gap-2 rounded-xl bg-semi-color-success-light-default px-4 py-3 text-sm text-semi-color-success'>
            <IconTick size={16} />
            <span>{t('已确认，请返回发起登录的页面。')}</span>
          </div>
        ) : (
          <Button
            block
            size='large'
            theme='solid'
            type='primary'
            loading={confirming}
            onClick={confirmLogin}
            style={{ borderRadius: 10 }}
          >
            {t('确认在此设备登录')}
          </Button>
        )}
      </div>
    </ConfirmShell>
  );
};

export default WechatLoginConfirm;
