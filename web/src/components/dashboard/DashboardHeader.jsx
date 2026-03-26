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
import { Button,Input } from '@douyinfe/semi-ui';
import { RefreshCw, Search,FileText,Plus } from 'lucide-react';

const DashboardHeader = ({
  getGreeting,
  greetingVisible,
  showSearchModal,
  refresh,
  loading,
  t,
}) => {
  const navigate = useNavigate();
  const ICON_BUTTON_CLASS = 'text-white hover:bg-opacity-80 !rounded-full';

  const getDateNow = () => {
    const now = new Date();
    const year = now.getFullYear();
    const month = String(now.getMonth() + 1).padStart(2, '0');
    const day = String(now.getDate()).padStart(2, '0');
    return `${year} ${t('年')} ${month} ${t('月')} ${day} ${t('日')}`;
  }
  const toPage = () => {
    navigate('/console/playground');
  }

  return (
    <div className='flex items-center justify-between mb-4'>
      <div>
        <h2
          className='text-2xl font-semibold text-gray-800 transition-opacity duration-1000 ease-in-out'
          style={{ opacity: greetingVisible ? 1 : 0,color: 'var(--semi-color-text-0)' }}
        >
          {getGreeting}
        </h2>
        <p className='text-sm m-2' style={{color:'rgb(100 116 139 / 100%)'}}>{t('今天是')} {getDateNow()}，{t('系统运行正常。')}</p>
      </div>
      
      <div className='flex gap-3'>
        <Input
          placeholder={t('请输入关键词搜索')}
          size='large'
          prefix={<Search size={16} style={{color:'rgb(148 163 184 / 100%)'}} />}
          onFocus={showSearchModal}
          className="common-input"
        />
        <Button
          theme='outline'
          type='tertiary'
          size='large'
          icon={<FileText size={16} />}
        >
          {t('查看报表')}
        </Button>
        <Button
          type='primary'
          theme='solid'
          className='theme-btn'
          size='large'
          icon={<Plus size={16} />}
          onClick={toPage}
        >
          {t('新建任务')}
        </Button>
        {/* <Button
          type='tertiary'
          icon={<Search size={16} />}
          onClick={showSearchModal}
          className={`bg-green-500 hover:bg-green-600 ${ICON_BUTTON_CLASS}`}
        /> */}
        <Button
          type='tertiary'
          icon={<RefreshCw size={16} />}
          onClick={refresh}
          loading={loading}
          className={`bg-blue-500 hover:bg-blue-600 ${ICON_BUTTON_CLASS}`}
        />
      </div>
    </div>
  );
};

export default DashboardHeader;
