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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Banner,
  Button,
  Card,
  Form,
  Popconfirm,
  Space,
  Spin,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, showWarning } from '../../helpers';

const emptyConfig = {
  app_id: '',
  qrcode_url: '',
  has_app_secret: false,
  enabled: true,
};

const WechatOfficialSetting = ({ provider = false, endpointOverride = '' }) => {
  const { t } = useTranslation();
  const endpoint =
    endpointOverride ||
    (provider
      ? '/api/provider/wechat_official_config'
      : '/api/option/wechat_official_config');
  const formApiRef = useRef(null);
  const [config, setConfig] = useState(emptyConfig);
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [loadError, setLoadError] = useState('');
  const busy = loading || saving || deleting;
  const hasConfig = Boolean(config.app_id);

  const applyConfig = useCallback((data) => {
    const nextConfig = { ...emptyConfig, ...data };
    setConfig(nextConfig);
    formApiRef.current?.setValues({
      app_id: nextConfig.app_id,
      app_secret: '',
      enabled: nextConfig.enabled,
    });
    setLoaded(true);
    setLoadError('');
  }, []);

  const fetchConfig = useCallback(
    async (signal) => {
      setLoading(true);
      setLoadError('');
      try {
        const { data } = await API.get(endpoint, {
          signal,
          disableDuplicate: true,
          skipErrorHandler: true,
        });
        if (signal?.aborted) return;
        if (!data.success || !data.data) {
          throw new Error(data.message || t('加载失败'));
        }
        applyConfig(data.data);
      } catch (error) {
        if (!signal?.aborted) {
          setLoaded(false);
          setLoadError(
            error.response?.data?.message || error.message || t('加载失败'),
          );
        }
      } finally {
        if (!signal?.aborted) setLoading(false);
      }
    },
    [endpoint, applyConfig, t],
  );

  useEffect(() => {
    const controller = new AbortController();
    fetchConfig(controller.signal);
    return () => controller.abort();
  }, [fetchConfig]);

  const handleSave = async (values) => {
    if (busy || !loaded) return;
    const appId = (values.app_id || '').trim();
    const appSecret = (values.app_secret || '').trim();
    if (!appId) {
      showWarning(t('AppId 不能为空'));
      return;
    }
    if (!config.has_app_secret && !appSecret) {
      showWarning(t('首次配置时 AppSecret 不能为空'));
      return;
    }
    if (hasConfig && appId !== config.app_id && !appSecret) {
      showWarning(t('更换 AppId 时需同时填写对应的 AppSecret。'));
      return;
    }

    setSaving(true);
    try {
      const { data } = await API.post(
        endpoint,
        {
          app_id: appId,
          app_secret: appSecret,
          enabled: values.enabled !== false,
        },
        { skipErrorHandler: true },
      );
      if (!data.success || !data.data) {
        throw new Error(data.message || t('保存失败'));
      }
      applyConfig(data.data);
      showSuccess(t('保存成功'));
    } catch (error) {
      showError(
        error.response?.data?.message || error.message || t('保存失败'),
      );
    } finally {
      setSaving(false);
    }
  };

  const handleGenerateQRCode = async () => {
    if (busy || !loaded) return;
    const values = formApiRef.current?.getValues?.() || {};
    const appId = (values.app_id || '').trim();
    const appSecret = (values.app_secret || '').trim();
    if (!appId) {
      showWarning(t('AppId 不能为空'));
      return;
    }
    if (!config.has_app_secret && !appSecret) {
      showWarning(t('首次配置时 AppSecret 不能为空'));
      return;
    }
    if (hasConfig && appId !== config.app_id && !appSecret) {
      showWarning(t('更换 AppId 时需同时填写对应的 AppSecret。'));
      return;
    }

    setSaving(true);
    try {
      // Persist the current form first so the QR code always belongs to the
      // credentials currently shown in the editor.
      const saveResponse = await API.post(
        endpoint,
        {
          app_id: appId,
          app_secret: appSecret,
          enabled: values.enabled !== false,
        },
        { skipErrorHandler: true },
      );
      if (!saveResponse.data.success || !saveResponse.data.data) {
        throw new Error(saveResponse.data.message || t('保存失败'));
      }
      applyConfig(saveResponse.data.data);

      const { data } = await API.post(`${endpoint}/qrcode`, null, {
        skipErrorHandler: true,
      });
      if (!data.success || !data.data) {
        throw new Error(data.message || t('生成失败'));
      }
      applyConfig(data.data);
      showSuccess(t('生成完成'));
    } catch (error) {
      showError(
        error.response?.data?.message || error.message || t('生成失败'),
      );
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (busy || !loaded || !hasConfig) return;
    setDeleting(true);
    try {
      const { data } = await API.delete(endpoint, { skipErrorHandler: true });
      if (!data.success) {
        throw new Error(data.message || t('删除失败'));
      }
      formApiRef.current?.reset();
      applyConfig(emptyConfig);
      showSuccess(t('删除成功'));
    } catch (error) {
      showError(
        error.response?.data?.message || error.message || t('删除失败'),
      );
    } finally {
      setDeleting(false);
    }
  };

  return (
    <Card title={t('微信服务号配置')} bordered={false}>
      <Banner
        type='info'
        description={t(
          '保存服务号凭据。修改 AppSecret 时填写新值，留空保留已保存的密钥。更换 AppId 时需同时填写对应的 AppSecret。',
        )}
        style={{ marginBottom: 16 }}
      />
      {loadError && (
        <Banner
          type='danger'
          description={loadError}
          closeIcon={null}
          style={{ marginBottom: 16 }}
        >
          <Button disabled={busy} onClick={() => fetchConfig()}>
            {t('刷新')}
          </Button>
        </Banner>
      )}
      <Spin spinning={loading}>
        <Form
          getFormApi={(api) => {
            formApiRef.current = api;
          }}
          initValues={{
            app_id: '',
            app_secret: '',
            enabled: true,
          }}
          onSubmit={handleSave}
          disabled={busy || !loaded}
          labelPosition='left'
          labelAlign='right'
          labelWidth={120}
        >
          <Form.Input
            field='app_id'
            label='AppId'
            maxLength={255}
            placeholder={t('请输入微信公众号 AppId')}
            rules={[{ required: true, message: t('AppId 不能为空') }]}
          />
          <Form.Input
            field='app_secret'
            label='AppSecret'
            mode='password'
            maxLength={512}
            autoComplete='new-password'
            placeholder={
              config.has_app_secret
                ? t('留空则保持原值不变')
                : t('请输入微信公众号 AppSecret')
            }
            rules={
              config.has_app_secret
                ? []
                : [
                    {
                      required: true,
                      message: t('首次配置时 AppSecret 不能为空'),
                    },
                  ]
            }
          />
          <Form.Switch field='enabled' label={t('启用')} />
          <Space style={{ marginTop: 16 }}>
            <Button
              htmlType='submit'
              theme='solid'
              type='primary'
              loading={saving}
              disabled={busy || !loaded}
            >
              {t('保存配置')}
            </Button>
            <Button
              htmlType='button'
              theme='solid'
              type='primary'
              loading={saving}
              disabled={busy || !loaded || Boolean(config.qrcode_url)}
              onClick={handleGenerateQRCode}
            >
              {config.qrcode_url ? t('生成完成') : t('生成微信公众号二维码')}
            </Button>
            {hasConfig && (
              <Popconfirm
                title={t('确认删除？')}
                content={t('删除后将清除当前站点保存的 AppId 和 AppSecret。')}
                onConfirm={handleDelete}
                disabled={busy || !loaded}
              >
                <Button
                  htmlType='button'
                  type='danger'
                  loading={deleting}
                  disabled={busy || !loaded}
                >
                  {t('删除配置')}
                </Button>
              </Popconfirm>
            )}
          </Space>
        </Form>
      </Spin>
      {loaded && config.qrcode_url && (
        <div style={{ marginTop: 20, textAlign: 'center' }}>
          <img
            src={config.qrcode_url}
            alt={t('微信二维码')}
            style={{ width: 240, height: 240, objectFit: 'contain' }}
          />
        </div>
      )}
      {loaded && hasConfig && (
        <Banner
          type={config.enabled ? 'success' : 'warning'}
          description={
            config.enabled
              ? t('微信服务号配置已保存并启用。')
              : t('微信服务号配置已保存，当前已禁用。')
          }
          style={{ marginTop: 16 }}
        />
      )}
    </Card>
  );
};

export default WechatOfficialSetting;
