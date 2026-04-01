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

import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Button, Input } from '@douyinfe/semi-ui';
import { RefreshCw, Search, FileText, Plus } from 'lucide-react';

const DashboardHeader = ({
  getGreeting,
  greetingVisible,
  showSearchModal,
  refresh,
  loading,
  dataExportDefaultTime,
  t,
}) => {
  const navigate = useNavigate();

  const getDateNow = () => {
    const now = new Date();
    const year = now.getFullYear();
    const month = String(now.getMonth() + 1).padStart(2, '0');
    const day = String(now.getDate()).padStart(2, '0');
    return `${year} ${t('年')} ${month} ${t('月')} ${day} ${t('日')}`;
  };

  const getDefaultRangeText = () => {
    switch (dataExportDefaultTime) {
      case 'week':
        return t('最近 30 天');
      case 'day':
        return t('最近 7 天');
      default:
        return t('最近 24 小时');
    }
  };

  const toPage = () => {
    navigate('/console/playground');
  };

  return (
    <section className='dashboard-header-v2'>
      <div className='dashboard-header-v2__content'>
        <div className='dashboard-header-v2__copy'>
          <div
            className='dashboard-header-v2__title'
            style={{ opacity: greetingVisible ? 1 : 0 }}
          >
            {getGreeting}
          </div>
          {/* <p className='dashboard-header-v2__subtitle'>
            {t('当前系统运行正常，今日已产生 3 次请求。')}
          </p> */}
          <div className='dashboard-header-v2__hint'>
            <span className='dashboard-header-v2__hint-dot' />
            <span>
              {t('今天是')} {getDateNow()}，
              {t('默认展示 {{range}} 内的数据。', {
                range: getDefaultRangeText(),
              })}
            </span>
          </div>
        </div>

        <div className='dashboard-header-v2__actions'>
            <Input
              placeholder={t('搜索 API Key...')}
              size='large'
              prefix={<Search size={16} style={{marginRight:'10px'}} />}
              onFocus={showSearchModal}
              className='dashboard-header-v2__search-input'
            />
            <Button
              type='primary'
              theme='solid'
              size='large'
              icon={<Plus size={16} />}
              className='dashboard-header-v2__primary-btn'
              onClick={toPage}
            >
              {t('添加令牌')}
            </Button>
            <Button
              type='tertiary'
              icon={<RefreshCw size={16} />}
              onClick={refresh}
              loading={loading}
              className='dashboard-header-v2__icon-btn'
              aria-label={t('刷新')}
            />
        </div>
      </div>
    </section>
  );
};

export default DashboardHeader;
