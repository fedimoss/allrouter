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

import React, { useEffect, useState, useCallback } from 'react';
import { Layout, TabPane, Tabs, Card, Spin, TextArea, Button } from '@douyinfe/semi-ui';
import { useNavigate, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { LayoutDashboard } from 'lucide-react';
import { API, showError, showSuccess } from '../../helpers';
import SettingsAnnouncements from '../Setting/Dashboard/SettingsAnnouncements';

const ProviderSetting = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [tabActiveKey, setTabActiveKey] = useState('dashboard');
  const [providerId, setProviderId] = useState(null);
  const [options, setOptions] = useState({});
  const [loading, setLoading] = useState(false);
  // 服务商自有公告（markdown 文本，与主站“公告（中文）/公告（英文）”同构）
  const [noticeZh, setNoticeZh] = useState('');
  const [noticeEn, setNoticeEn] = useState('');
  const [noticeSaving, setNoticeSaving] = useState(false);

  const fetchProviderId = async () => {
    const res = await API.get('/api/provider/self');
    if (res.data.success && res.data.data) {
      setProviderId(res.data.data.id);
      return res.data.data.id;
    }
    showError(t('获取服务商信息失败'));
    return null;
  };

  const fetchOptions = async (pId) => {
    try {
      const res = await API.get(`/api/provider/options/${pId}`);
      if (res.data.success) {
        const optionMap = {};
        (res.data.data || []).forEach((item) => {
          optionMap[item.key] = item.value;
        });
        setOptions(optionMap);
        setNoticeZh(optionMap['Notice'] || '');
        setNoticeEn(optionMap['NoticeEn'] || '');
      }
    } catch (error) {
      console.error('获取服务商配置失败:', error);
    }
  };

  const refresh = async () => {
    setLoading(true);
    try {
      const pId = providerId || (await fetchProviderId());
      if (pId) {
        await fetchOptions(pId);
      }
    } finally {
      setLoading(false);
    }
  };

  const handleSave = useCallback(
    async (key, value) => {
      const pId = providerId;
      if (!pId) return { data: { success: false, message: '服务商ID缺失' } };
      const res = await API.put(`/api/provider/options/${pId}`, {
        key,
        value,
      });
      if (res.data.success) {
        setOptions((prev) => ({ ...prev, [key]: value }));
      }
      return res;
    },
    [providerId],
  );

  useEffect(() => {
    const init = async () => {
      setLoading(true);
      try {
        const pId = await fetchProviderId();
        if (pId) {
          await fetchOptions(pId);
        }
      } finally {
        setLoading(false);
      }
    };
    init();
  }, []);

  useEffect(() => {
    const searchParams = new URLSearchParams(window.location.search);
    const tab = searchParams.get('tab');
    if (tab) {
      setTabActiveKey(tab);
    }
  }, [location.search]);

  const onChangeTab = (key) => {
    setTabActiveKey(key);
    navigate(`?tab=${key}`);
  };

  // 保存服务商自有公告（中/英文）
  const handleSaveNotice = async () => {
    const pId = providerId;
    if (!pId) {
      showError(t('获取服务商信息失败'));
      return;
    }
    setNoticeSaving(true);
    try {
      await Promise.all([
        API.put(`/api/provider/options/${pId}`, { key: 'Notice', value: noticeZh }),
        API.put(`/api/provider/options/${pId}`, { key: 'NoticeEn', value: noticeEn }),
      ]);
      showSuccess(t('设置公告'));
    } catch (error) {
      console.error('保存公告失败:', error);
      showError(error?.message || t('保存失败'));
    } finally {
      setNoticeSaving(false);
    }
  };

  return (
    <div className='mt-[60px] px-2'>
      <Layout>
        <Layout.Content>
          <Tabs
            type='card'
            collapsible
            activeKey={tabActiveKey}
            onChange={(key) => onChangeTab(key)}
          >
            <TabPane
              itemKey='dashboard'
              tab={
                <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
                  <LayoutDashboard size={18} />
                  {t('仪表盘设置')}
                </span>
              }
              key='dashboard'
            >
              {tabActiveKey === 'dashboard' && (
                <Spin spinning={loading} size='large'>
                  <Card style={{ marginTop: '10px' }}>
                    {/* 服务商模式隐藏主站公告专用的“在服务商站点显示”开关。 */}
                    <SettingsAnnouncements
                      options={options}
                      refresh={refresh}
                      onSave={handleSave}
                      onToggleEnabled={handleSave}
                      providerMode
                    />
                  </Card>
                  <Card style={{ marginTop: '10px' }}>
                    {/* 服务商自有公告（markdown 文本，前台“通知”tab 展示，按用户语言择一） */}
                    <div style={{ marginBottom: 16 }}>
                      <div style={{ fontSize: 16, fontWeight: 600, marginBottom: 16 }}>
                        {t('通用设置')}
                      </div>
                      <div style={{ marginBottom: 16 }}>
                        <div style={{ marginBottom: 8 }}>{t('公告（中文）')}</div>
                        <TextArea
                          placeholder={t('在此输入新的公告内容，支持 Markdown & HTML 代码')}
                          value={noticeZh}
                          onChange={setNoticeZh}
                          style={{ fontFamily: 'JetBrains Mono, Consolas' }}
                          autosize={{ minRows: 6, maxRows: 12 }}
                        />
                      </div>
                      <div style={{ marginBottom: 16 }}>
                        <div style={{ marginBottom: 8 }}>{t('公告（英文）')}</div>
                        <TextArea
                          placeholder={t('在此输入新的公告内容，支持 Markdown & HTML 代码')}
                          value={noticeEn}
                          onChange={setNoticeEn}
                          style={{ fontFamily: 'JetBrains Mono, Consolas' }}
                          autosize={{ minRows: 6, maxRows: 12 }}
                        />
                      </div>
                      <Button onClick={handleSaveNotice} loading={noticeSaving}>
                        {t('设置公告')}
                      </Button>
                    </div>
                  </Card>
                </Spin>
              )}
            </TabPane>
          </Tabs>
        </Layout.Content>
      </Layout>
    </div>
  );
};

export default ProviderSetting;
