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
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Input, Tag, TextArea } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';
import { Loader2, ArrowRightToLine, KeyRound, CircleCheckBig } from 'lucide-react';
import { OpenAI, Claude, Qwen, Moonshot, Gemini, Flowith } from '@lobehub/icons';

import antigravityIcon from '../../../../public/logos/antigravity.svg';

const MODEL_TYPE_PROVIDER_KEY_MAP = {
  1: 'codex',
  2: 'anthropic',
  3: 'qwen',
};

const OauthList = () => {
  const { t } = useTranslation();
  const [activeMap, setActiveMap] = useState({});
  const [loadingMap, setLoadingMap] = useState({});
  const [completeLoadingMap, setCompleteLoadingMap] = useState({});
  const [callbackValues, setCallbackValues] = useState({});
  const [formValues, setFormValues] = useState({});
  const [statusMap, setStatusMap] = useState({});
  const [connectedCount, setConnectedCount] = useState(0);
  const pollingRef = useRef({});

  const fetchApi = useMemo(
    () => [
      {key: 'codex',api: '/api/v0/management/codex-auth-url'},
      {key: 'anthropic',api: '/api/v0/management/anthropic-auth-url'},
      {key: 'antigravity',api: '/api/v0/management/gemini-cli-auth-url'},
      {key: 'gemini', api: '/api/v0/management/gemini-auth-url'},
      {key: 'kimi',api: '/api/v0/management/kimi-auth-url'},
      { key: 'qwen', api: '/api/v0/management/qwen-auth-url' },
      {key: 'iflow', api: '/api/v0/management/iflow-auth-url', params: { cookie: '' }},
    ],
    [],
  );

  const providers = useMemo(
    () => [
      {
        key: 'codex',
        title: 'Codex OAuth',
        description: t('通过 OAuth 流程登录 Codex 服务，自动获取并保存认证文件。'),
        icon: OpenAI,
        mode: 'oauth',
      },
      {
        key: 'anthropic',
        title: 'Anthropic OAuth',
        description: t(
          '通过 OAuth 流程登录 Anthropic (Claude) 服务，自动获取并保存认证文件。',
        ),
        icon: Claude?.Color || Claude,
        mode: 'oauth',
      },
      {
        key: 'antigravity',
        title: 'Antigravity OAuth',
        description: t(
          '通过 OAuth 流程登录 Antigravity（Google 账号）服务，自动获取并保存认证文件。',
        ),
        icon: antigravityIcon,
        mode: 'oauth',
      },
      {
        key: 'gemini',
        title: 'Gemini CLI OAuth',
        description: t('通过 OAuth 流程登录 Google Gemini CLI 服务，自动获取并保存认证文件。'),
        icon: Gemini?.Color || Gemini,
        mode: 'oauth',
      },
      {
        key: 'kimi',
        title: 'Kimi OAuth',
        description: t('使用 OAuth 登录 Kimi，授权完成后自动保存认证信息。'),
        icon: Moonshot,
        mode: 'status_only',
      },
      {
        key: 'qwen',
        title: 'Qwen OAuth',
        description: t('使用 OAuth 登录 Qwen，授权完成后自动保存认证信息。'),
        icon: Qwen?.Color || Qwen,
        mode: 'status_only',
      },
      // {
      //   key: 'vertex',
      //   title: t('Vertex JSON 登录'),
      //   description: t('上传 Google 服务账号 JSON，并填写区域信息。'),
      //   icon: VertexAI,
      //   mode: 'form',
      //   fields: [
      //     {
      //       key: 'region',
      //       label: t('目标区域（可选）'),
      //       placeholder: 'us-central1',
      //       helper: t('留空表示使用默认区域。'),
      //     },
      //   ],
      //   file: {
      //     key: 'service_account',
      //     label: t('服务账号密钥 JSON'),
      //     helper: t('仅支持 Google Cloud service account key JSON 文件。'),
      //   },
      //   actionText: t('导入 Vertex 凭证'),
      // },
      {
        key: 'iflow',
        title: t('iFlow Cookie 登录'),
        description: t('提交 Cookie 完成登录，服务端自动保存凭据。'),
        icon: Flowith,
        mode: 'iflow',
        fields: [
          {
            key: 'cookie',
            label: t('Cookie 内容'),
            placeholder: t('填入 BXAuth 值 以 BXAuth= 开头'),
            helper: t('提示：需在平台先创建 Key。'),
            textarea: true,
          },
        ],
        actionText: t('提交 Cookie 登录'),
      },
    ],
    [t],
  );

  // 获取已连接服务数量 请求接口获取已连接服务数量，更新 connectedCount 状态
  const getConnectNum = async () => { 
    try { 
      const res = await API.get('/api/v0/management/get-oauth-success-count');
      const payload = res?.data?.data || {};
      setConnectedCount(Number(payload?.count) || 0);

      const modelTypes = Array.isArray(payload?.model_types)
        ? payload.model_types
        : [];
      const connectedStatusPatch = modelTypes.reduce((acc, modelType) => {
        const providerKey = MODEL_TYPE_PROVIDER_KEY_MAP[Number(modelType)];
        if (providerKey) {
          acc[providerKey] = 'connected';
        }
        return acc;
      }, {});
      if (Object.keys(connectedStatusPatch).length > 0) {
        setStatusMap((prev) => ({ ...prev, ...connectedStatusPatch }));
      }
    } catch (error) {
      console.error('获取连接数量失败:', error);
      setConnectedCount(0);
    }
  }

  const stopPolling = (key) => {
    if (pollingRef.current[key]) {
      clearInterval(pollingRef.current[key]);
      delete pollingRef.current[key];
    }
  };

  const extractState = (url, fallback) => {
    if (!url) return fallback || '';
    try {
      const parsed = new URL(url);
      return parsed.searchParams.get('state') || fallback || '';
    } catch (_) {
      return fallback || '';
    }
  };

  const startPolling = (key, state) => {
    if (!state) return;
    stopPolling(key);
    pollingRef.current[key] = setInterval(async () => {
      try {
        const res = await API.get('/api/v0/management/get-auth-status', {
          params: { state },
          skipErrorHandler: true,
        });
        const status = res?.data?.data?.status || res?.data?.status;
        if (status === 'wait') {
          setStatusMap((prev) => ({ ...prev, [key]: 'pending' }));
          return;
        }
        if (status === 'ok') {
          setStatusMap((prev) => ({ ...prev, [key]: 'connected' }));
          setLoadingMap((prev) => ({ ...prev, [key]: false }));
          setCompleteLoadingMap((prev) => ({ ...prev, [key]: false }));
          showSuccess(t('授权成功'));
          stopPolling(key);
          return;
        }
        if (status === 'error') {
          showError(t('授权失败，请重试'));
          setStatusMap((prev) => ({ ...prev, [key]: 'failed' }));
          setLoadingMap((prev) => ({ ...prev, [key]: false }));
          setCompleteLoadingMap((prev) => ({ ...prev, [key]: false }));
          stopPolling(key);
          return;
        }
      } catch (_) {
        setStatusMap((prev) => ({ ...prev, [key]: 'failed' }));
        setLoadingMap((prev) => ({ ...prev, [key]: false }));
        setCompleteLoadingMap((prev) => ({ ...prev, [key]: false }));
        stopPolling(key);
      }
    }, 3000);
  };

  const startOAuth = async (item) => {
    setLoadingMap((prev) => ({ ...prev, [item.key]: true }));
    try {
      const apiConfig = fetchApi.find((config) => config.key === item.key);
      if (!apiConfig?.api) {
        throw new Error(t('未配置授权接口'));
      }
      const res = await API.get(apiConfig.api, { skipErrorHandler: true });
      if (!res?.data?.success) {
        throw new Error(res?.data?.message || t('启动授权失败'));
      }
      const data = res?.data?.data || {};
      const url = data.url || '';
      const state = data.state || extractState(url, '');
      if (!url) {
        throw new Error(t('响应缺少跳转链接'));
      }
      window.open(url, '_blank', 'noopener,noreferrer');
      setActiveMap((prev) => ({ ...prev, [item.key]: true }));
      setStatusMap((prev) => ({ ...prev, [item.key]: 'pending' }));
      if (!state) {
        throw new Error(t('响应缺少 state'));
      }
      startPolling(item.key, state);
    } catch (error) {
      showError(error?.message || t('启动授权失败'));
      setStatusMap((prev) => ({ ...prev, [item.key]: 'failed' }));
      setLoadingMap((prev) => ({ ...prev, [item.key]: false }));
    }
  };

  const submitCallback = async (item) => {
    const input = (callbackValues[item.key] || '').trim();
    if (!input) {
      showError(t('请先粘贴回调 URL'));
      return;
    }
    setCompleteLoadingMap((prev) => ({ ...prev, [item.key]: true }));
    try {
      const res = await API.post(
        '/api/v0/management/oauth-callback',
        { provider: item.key, redirect_url: input },
        { skipErrorHandler: true },
      );
      if (!res?.data?.success) {
        throw new Error(res?.data?.message || t('提交失败'));
      }
      const data = res?.data?.data || {};
      const state = data.state || extractState(input, '');
      if (!state) {
        throw new Error(t('回调 URL 中缺少 state'));
      }
      showSuccess(t('回调 URL 已提交，正在验证...'));
      setStatusMap((prev) => ({ ...prev, [item.key]: 'pending' }));
      startPolling(item.key, state);
    } catch (error) {
      showError(error?.message || t('提交失败'));
      setStatusMap((prev) => ({ ...prev, [item.key]: 'failed' }));
      setCompleteLoadingMap((prev) => ({ ...prev, [item.key]: false }));
    }
  };

  // iFlow 提交：先校验 Cookie，再请求 iflow-auth-url，根据返回直接显示成功/失败
  const submitIFlow = async (item) => {
    const cookie = (formValues[item.key]?.cookie || '').trim();
    if (!cookie) {
      showError(t('请先输入 Cookie 内容'));
      return;
    }
    setCompleteLoadingMap((prev) => ({ ...prev, [item.key]: true }));
    try {
      const apiConfig = fetchApi.find((config) => config.key === item.key);
      if (!apiConfig?.api) {
        throw new Error(t('未配置授权接口'));
      }
      const res = await API.post(apiConfig.api, {
        cookie: cookie
      });
      if (!res?.data?.success) {
        throw new Error(res?.data?.message || t('提交失败'));
      }
      setStatusMap((prev) => ({ ...prev, [item.key]: 'connected' }));
      showSuccess(res?.data?.message || t('认证成功'));
    } catch (error) {
      showError(error?.message || t('提交失败'));
      setStatusMap((prev) => ({ ...prev, [item.key]: 'failed' }));
    } finally {
      setCompleteLoadingMap((prev) => ({ ...prev, [item.key]: false }));
    }
  };

  useEffect(() => {
    getConnectNum();

    return () => {
      Object.values(pollingRef.current).forEach((timerId) =>
        clearInterval(timerId),
      );
      pollingRef.current = {};
    };
  }, []);

  return (
    <div className='w-full pb-8'>
      <div className='mx-auto w-full max-w-[1360px] px-4 pt-4 md:px-8 lg:px-10'>
        <div className='rounded-3xl'>
          <div className='mb-4 flex flex-col justify-between gap-4 rounded-2xl p-5 md:flex-row md:items-center' style={{ backgroundColor: 'var(--semi-color-fill-0)' }}>
          <div>
            <div className='flex items-center gap-3'>
              <KeyRound size={32} style={{ color: 'var(--semi-color-primary)' }} />
              <h2 className='text-[26px] font-semibold leading-none' style={{ color: 'var(--semi-color-text-0)' }}>
                {t('OAuth 授权中心')}
              </h2>
            </div>
            <p className='mt-3 text-[16px]' style={{ color: 'var(--semi-color-text-1)' }}>
              {t(
                '一键连接主流 AI 服务商，自动获取并管理认证凭证。无需手动复制 API Key，安全又便捷。',
              )}
            </p>
          </div>
          <div className='flex min-w-[140px] items-center gap-3 rounded-2xl px-4 py-3' style={{ border: '1px solid var(--semi-color-border)', backgroundColor: 'var(--semi-color-bg-0)' }}>
            <CircleCheckBig size={24} style={{ color: 'var(--semi-color-success)' }} />
            <div>
              <div className='text-[13px]' style={{ color: 'var(--semi-color-text-1)' }}>{t('已连接服务')}</div>
              <div className='text-[26px] font-semibold leading-none' style={{ color: 'var(--semi-color-text-0)' }}>
                {connectedCount}
                <span className='text-[18px]' style={{ color: 'var(--semi-color-text-2)' }}> / {providers.length}</span>
              </div>
            </div>
          </div>
          </div>

          <div className='grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3'>
            {providers.map((item) => {
          const Icon = item.icon;
          const isActive = !!activeMap[item.key];
          const isLoading = !!loadingMap[item.key];
          const isCompleteLoading = !!completeLoadingMap[item.key];
          const status = statusMap[item.key];

          return (
            <div
              key={item.key}
              className='flex min-h-[220px] flex-col rounded-2xl p-5 backdrop-blur' style={{ border: '1px solid var(--semi-color-border)', backgroundColor: 'color-mix(in srgb, var(--semi-color-bg-0) 88%, transparent)', boxShadow: '0 10px 30px rgba(15,23,42,0.08)' }}
            >
              <div className='flex items-start justify-between gap-4'>
                <div className='flex items-start gap-3'>
                  <div className='flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl' style={{ backgroundImage: 'linear-gradient(135deg, var(--semi-color-primary-light-default) 0%, rgba(var(--semi-cyan-0),1) 60%, var(--semi-color-success-light-default) 100%)', color: 'var(--semi-color-text-0)' }}>
                    {
                      item.key === 'antigravity' ? (
                        <img src={antigravityIcon} alt={item.title} className='h-4 w-4' />
                      ) : (
                        <Icon size={20} strokeWidth={2.2} className='shrink-0' />
                      )
                    }
                  </div>
                  <div>
                    <div className='text-[16px] font-semibold' style={{ color: 'var(--semi-color-text-0)' }}>
                      {item.title}
                    </div>
                    <div className='mt-1 text-[12px]' style={{ color: 'var(--semi-color-text-1)' }}>
                      {item.description}
                    </div>
                  </div>
                </div>
                <Tag
                  color={
                    status === 'connected'
                      ? 'green'
                      : status === 'failed'
                        ? 'red'
                        : status === 'pending'
                          ? 'blue'
                          : 'grey'
                  }
                  style={{ borderRadius: 999, padding: '0 10px', whiteSpace: 'nowrap',flexShrink: 0 }}
                >
                  {status === 'connected'
                    ? t('认证成功')
                    : status === 'failed'
                      ? t('认证失败')
                      : status === 'pending'
                        ? (
                            <span className='flex items-center gap-1'>
                              <Loader2 size={12} className='animate-spin' />
                              {t('认证中')}
                            </span>
                          )
                        : t('未认证')}
                </Tag>
              </div>

              {item.mode === 'oauth' && isActive && (
                <div className='mt-4 rounded-xl border border-dashed p-4' style={{ borderColor: 'var(--semi-color-border)', backgroundColor: 'var(--semi-color-fill-0)' }}>
                  <div className='text-[13px] font-semibold' style={{ color: 'var(--semi-color-text-0)' }}>
                    {t('回调 URL')}
                  </div>
                  <Input
                    className='mt-2'
                    value={callbackValues[item.key] || ''}
                    onChange={(value) =>
                      setCallbackValues((prev) => ({
                        ...prev,
                        [item.key]: value,
                      }))
                    }
                    placeholder={t('请粘贴完整回调 URL（包含 code 与 state）')}
                    showClear
                  />
                  <div className='mt-2 text-[12px]' style={{ color: 'var(--semi-color-text-2)' }}>
                    {t(
                      '授权跳转后如需手动提交，可将完整 URL 粘贴到此处（当前版本自动轮询认证状态）。',
                    )}
                  </div>
                  <div className='mt-3 flex items-center justify-between gap-2'>
                    <Button
                      type='primary'
                      theme='solid'
                      onClick={() => submitCallback(item)}
                    >
                      {isCompleteLoading ? (
                        <span className='flex items-center gap-2'>
                          <Loader2 size={14} className='animate-spin' />
                          {t('提交中')}
                        </span>
                      ) : (
                        t('提交回调 URL')
                      )}
                    </Button>
                    <Tag
                      size='small'
                      color={
                        status === 'connected'
                          ? 'green'
                          : status === 'failed'
                            ? 'red'
                            : status === 'pending'
                              ? 'blue'
                              : 'grey'
                      }
                    >
                      {status === 'connected'
                        ? t('认证成功')
                        : status === 'failed'
                          ? t('认证失败')
                          : status === 'pending'
                            ? (
                                <span className='flex items-center gap-1'>
                                  <Loader2 size={12} className='animate-spin' />
                                  {t('认证中')}
                                </span>
                              )
                            : t('未认证')}
                    </Tag>
                  </div>
                </div>
              )}

              {item.mode !== 'iflow' && (
                <div className='mt-auto pt-4'>
                  <Button
                    type='primary'
                    theme='solid'
                    loading={isLoading}
                    onClick={() => startOAuth(item)}
                    disabled={isLoading || status === 'pending'}
                    icon={!isLoading ? <ArrowRightToLine size={16} /> : null}
                    style={{
                      width: '100%',
                      height: 40,
                      border: 'none',
                      borderRadius: 12,
                      backgroundImage:
                        'linear-gradient(135deg, #09FEF7 0%, #F8FF15 100%)',
                      color: '#fff',
                      fontWeight: 600,
                      boxShadow: '0 12px 24px rgba(16, 185, 129, 0.18)',
                    }}
                  >
                    {isLoading ? (
                      <span className='flex items-center justify-center gap-2'>
                        <Loader2 size={14} className='animate-spin' />
                        {t('授权中')}
                      </span>
                    ) : (
                      t('立即授权')
                    )}
                  </Button>
                </div>
              )}

              {item.mode === 'iflow' && (
                <div className='mt-4 rounded-xl border border-dashed p-4' style={{ borderColor: 'var(--semi-color-border)', backgroundColor: 'var(--semi-color-fill-0)' }}>
                  {item.fields?.map((field) => (
                    <div key={field.key} className='mb-3 last:mb-0'>
                      <div className='text-[13px] font-semibold' style={{ color: 'var(--semi-color-text-0)' }}>
                        {field.label}
                      </div>
                      {field.textarea ? (
                        <TextArea
                          className='mt-2'
                          autosize={{ minRows: 2, maxRows: 4 }}
                          value={formValues[item.key]?.[field.key] || ''}
                          onChange={(value) =>
                            setFormValues((prev) => ({
                              ...prev,
                              [item.key]: {
                                ...(prev[item.key] || {}),
                                [field.key]: value,
                              },
                            }))
                          }
                          placeholder={field.placeholder}
                          showClear
                        />
                      ) : (
                        <Input
                          className='mt-2'
                          value={formValues[item.key]?.[field.key] || ''}
                          onChange={(value) =>
                            setFormValues((prev) => ({
                              ...prev,
                              [item.key]: {
                                ...(prev[item.key] || {}),
                                [field.key]: value,
                              },
                            }))
                          }
                          placeholder={field.placeholder}
                          showClear
                        />
                      )}
                      {field.helper && (
                        <div className='mt-1 text-[12px]' style={{ color: 'var(--semi-color-text-2)' }}>
                          {field.helper}
                        </div>
                      )}
                    </div>
                  ))}

                  <div className='mt-2 flex items-center justify-between gap-2'>
                    <Button
                      type='primary'
                      theme='solid'
                      loading={isCompleteLoading}
                      onClick={() => submitIFlow(item)}
                    >
                      {isCompleteLoading ? (
                        <span className='flex items-center gap-2'>
                          <Loader2 size={14} className='animate-spin' />
                          {t('提交中')}
                        </span>
                      ) : (
                        item.actionText || t('提交')
                      )}
                    </Button>
                    <Tag
                      size='small'
                      color={
                        status === 'connected'
                          ? 'green'
                          : status === 'failed'
                            ? 'red'
                            : status === 'pending'
                              ? 'blue'
                              : 'grey'
                      }
                    >
                      {status === 'connected'
                        ? t('认证成功')
                        : status === 'failed'
                          ? t('认证失败')
                          : status === 'pending'
                            ? (
                                <span className='flex items-center gap-1'>
                                  <Loader2 size={12} className='animate-spin' />
                                  {t('认证中')}
                                </span>
                              )
                            : t('未认证')}
                    </Tag>
                  </div>
                </div>
              )}
            </div>
          );
            })}
          </div>
        </div>
      </div>
    </div>
  );
};

export default OauthList;
