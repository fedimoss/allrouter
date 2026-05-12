import React, { useEffect, useState } from 'react';
import { Spin, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../../helpers';
import ProviderRewardPanel from './ProviderRewardPanel';

const ProviderRewardPage = () => {
  const { t } = useTranslation();
  const [provider, setProvider] = useState(null);
  const [loading, setLoading] = useState(false);

  const loadProvider = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/provider/self');
      if (res.data.success) {
        setProvider(res.data.data || null);
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    }
    setLoading(false);
  };

  useEffect(() => {
    loadProvider();
  }, []);

  return (
    <div className='px-2'>
      <Typography.Title heading={4}>{t('奖励设置')}</Typography.Title>
      <Spin spinning={loading}>
        <ProviderRewardPanel provider={provider} adminMode={false} mode='config' />
      </Spin>
    </div>
  );
};

export default ProviderRewardPage;
