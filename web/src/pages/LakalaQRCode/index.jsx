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

import React, { useEffect, useMemo } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Button, Toast } from '@douyinfe/semi-ui';
import { ArrowLeft, Copy } from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';
import { useTranslation } from 'react-i18next';
import { API } from '../../helpers';
import { getLakalaQRCodePayment } from '../../helpers/lakalaPayment';

const POLL_FAST_INTERVAL_MS = 3000;
const POLL_SLOW_INTERVAL_MS = 10000;
const POLL_SLOW_AFTER_MS = 2 * 60 * 1000;
const POLL_TIMEOUT_MS = 10 * 60 * 1000;

const LakalaQRCode = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const tradeNo = searchParams.get('trade_no') || '';
  const payment = useMemo(() => getLakalaQRCodePayment(tradeNo), [tradeNo]);

  // Prefer the source page saved when the QR code payment was created.
  const backPath = payment?.return_to || '/console/topup';
  const successPath = payment?.success_path || '/console/topup?pay=success';

  const copyTradeNo = async () => {
    await navigator.clipboard.writeText(payment?.trade_no || tradeNo);
    Toast.success({ content: t('复制成功') });
  };

  useEffect(() => {
    if (!tradeNo) return undefined;

    let stopped = false;
    let timerId;
    const startedAt = Number(payment?.created_at) || Date.now();

    const stopWithTimeout = () => {
      Toast.warning({ content: t('订单已超时，请重新发起充值') });
    };

    const pollOrderStatus = async () => {
      if (stopped) return;

      const elapsed = Date.now() - startedAt;
      if (elapsed >= POLL_TIMEOUT_MS) {
        stopWithTimeout();
        return;
      }

      try {
        // 根据订单号前缀区分充值和订阅：SUB 开头为订阅订单，其余为充值订单
        const statusApi = tradeNo.startsWith('SUB')
          ? '/api/subscription/lakala/status'
          : '/api/user/lakala/status';
        const res = await API.get(
          `${statusApi}?trade_no=${encodeURIComponent(tradeNo)}`,
          { skipErrorHandler: true },
        );
        const payload = res?.data;
        const status = payload?.data?.status;

        if (payload?.message === 'success') {
          if (payload?.data?.paid || status === 'success') {
            Toast.success({ content: t('支付成功') });
            navigate(successPath, { replace: true });
            return;
          }
          if (status === 'failed' || status === 'expired') {
            Toast.warning({ content: t('订单已失效，请重新发起充值') });
            return;
          }
        } else if (payload?.message === 'error') {
          Toast.warning({ content: payload?.data || t('订单状态查询失败') });
          return;
        }
      } catch (error) {
        console.warn('Failed to poll Lakala top-up status', error);
      }

      if (stopped) return;
      timerId = window.setTimeout(
        pollOrderStatus,
        elapsed >= POLL_SLOW_AFTER_MS
          ? POLL_SLOW_INTERVAL_MS
          : POLL_FAST_INTERVAL_MS,
      );
    };

    timerId = window.setTimeout(pollOrderStatus, POLL_FAST_INTERVAL_MS);

    return () => {
      stopped = true;
      window.clearTimeout(timerId);
    };
  }, [navigate, payment?.created_at, successPath, t, tradeNo]);

  if (!payment?.code) {
    return (
      <main className='min-h-screen bg-slate-50 px-4 py-10 text-slate-900 dark:bg-slate-950 dark:text-slate-100'>
        <section className='mx-auto flex min-h-[calc(100vh-80px)] max-w-md flex-col items-center justify-center text-center'>
          <div className='w-full rounded-2xl border border-slate-200 bg-white p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900'>
            <h1 className='mb-3 text-xl font-bold'>{t('微信支付')}</h1>
            <p className='mb-6 text-sm leading-6 text-slate-500 dark:text-slate-400'>
              {t('页面未找到，请检查您的浏览器地址是否正确')}
            </p>
            <Button
              icon={<ArrowLeft size={16} />}
              onClick={() => navigate(backPath)}
              className='!rounded-lg'
            >
              {t('充值')}
            </Button>
          </div>
        </section>
      </main>
    );
  }

  return (
    <main className='min-h-screen bg-[#f4f7f5] px-4 py-8 text-slate-900 dark:bg-slate-950 dark:text-slate-100'>
      <section className='mx-auto flex min-h-[calc(100vh-64px)] max-w-[420px] flex-col items-center justify-center'>
        <div className='w-full overflow-hidden rounded-[22px] border border-slate-200 bg-white shadow-[0_18px_50px_rgba(15,23,42,0.10)] dark:border-slate-800 dark:bg-slate-900'>
          <div className='bg-[#07c160] px-6 py-5 text-center text-white'>
            <h1 className='text-2xl font-bold tracking-normal'>
              {t('微信支付')}
            </h1>
          </div>

          <div className='px-7 pb-7 pt-6 text-center'>
            <div className='mx-auto mb-5 flex h-[272px] w-[272px] items-center justify-center rounded-2xl border border-slate-100 bg-white p-4 shadow-inner dark:border-slate-800'>
              <QRCodeSVG
                value={payment.code}
                size={240}
                level='M'
                includeMargin={false}
              />
            </div>

            <div className='mt-6 space-y-3 rounded-xl bg-slate-50 p-4 text-left text-sm dark:bg-slate-800/70'>
              <div className='flex items-center justify-between gap-4'>
                <span className='shrink-0 text-slate-500 dark:text-slate-400'>
                  {t('充值金额')}
                </span>
                <span className='font-semibold text-slate-900 dark:text-slate-100'>
                  ¥{payment.amount || '-'}
                </span>
              </div>
              <div className='flex items-start justify-between gap-4'>
                <span className='shrink-0 text-slate-500 dark:text-slate-400'>
                  {t('订单号')}
                </span>
                <button
                  type='button'
                  onClick={copyTradeNo}
                  className='inline-flex min-w-0 items-center gap-1 text-right font-medium text-slate-700 hover:text-[#07c160] dark:text-slate-200'
                >
                  <span className='break-all'>{payment.trade_no}</span>
                  <Copy size={14} className='shrink-0' />
                </button>
              </div>
            </div>

            <Button
              theme='borderless'
              icon={<ArrowLeft size={16} />}
              onClick={() => navigate(backPath)}
              className='!mt-5 !rounded-lg'
            >
              {t('充值')}
            </Button>
          </div>
        </div>
      </section>
    </main>
  );
};

export default LakalaQRCode;
