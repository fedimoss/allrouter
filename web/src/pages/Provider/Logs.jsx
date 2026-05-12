import React from 'react';
import { useTranslation } from 'react-i18next';
import UsageLogsTable from '../../components/table/usage-logs';
import '../Log/log-v2.css';

const ProviderLogsPage = () => {
  const { t } = useTranslation();

  return (
    <div className='log-v2-page mt-[10px] px-2'>
      <UsageLogsTable
        scope='provider'
        title={t('服务商使用日志')}
        description={t('查看当前服务商下用户的 API 调用、计费和请求状态。')}
      />
    </div>
  );
};

export default ProviderLogsPage;
