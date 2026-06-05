import React from 'react';
import { useTranslation } from 'react-i18next';
import UsageLogsTable from '../../components/table/usage-logs';
import '../Log/log-v2.css';

const CallLog = () => {
  const { t } = useTranslation();

  return (
    <div className='log-v2-page mt-[10px] px-2'>
      <UsageLogsTable
        scope='admin-call'
        title={t('调用日志')}
        description={t('查看所有用户的 API 调用、服务商归属、计费和请求状态。')}
      />
    </div>
  );
};

export default CallLog;
