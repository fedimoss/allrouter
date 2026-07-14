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
import { Layout, TabPane, Tabs, Card, Spin } from '@douyinfe/semi-ui';
import { useNavigate, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { LayoutDashboard } from 'lucide-react';
import { API, showError } from '../../helpers';
import SettingsAnnouncements from '../Setting/Dashboard/SettingsAnnouncements';

const ProviderSetting = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [tabActiveKey, setTabActiveKey] = useState('dashboard');
  const [providerId, setProviderId] = useState(null);
  const [options, setOptions] = useState({});
  const [loading, setLoading] = useState(false);

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
                    <SettingsAnnouncements
                      options={options}
                      refresh={refresh}
                      onSave={handleSave}
                      onToggleEnabled={handleSave}
                      providerMode
                    />
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
