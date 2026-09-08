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

import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button, Input, Modal, Pagination, Spin } from '@douyinfe/semi-ui';
import { RefreshCw, Search, Plus, X } from 'lucide-react';
import { timestamp2string } from '../../helpers';
import { INVITEE_PAGE_SIZE } from '../../constants/dashboard.constants';

const DashboardHeader = ({
  getGreeting,
  greetingVisible,
  showSearchModal,
  refresh,
  loading,
  dataExportDefaultTime,
  invitees,
  inviteesLoading,
  inviteesTotal,
  selectedInvitee,
  loadInvitees,
  selectInvitee,
  clearInvitee,
  t,
}) => {
  const navigate = useNavigate();
  const [inviteeModalVisible, setInviteeModalVisible] = useState(false);
  const [inviteeKeyword, setInviteeKeyword] = useState('');
  const [inviteePage, setInviteePage] = useState(1);

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
    navigate('/console/token');
  };

  const openInviteeModal = async () => {
    setInviteeModalVisible(true);
    setInviteeKeyword('');
    setInviteePage(1);
    await loadInvitees({ page: 1 });
  };

  const handleInviteeSelect = async (invitee) => {
    if (await selectInvitee(invitee)) {
      setInviteeModalVisible(false);
    }
  };

  const handleInviteeSearch = async () => {
    setInviteePage(1);
    await loadInvitees({ page: 1, keyword: inviteeKeyword });
  };

  const handleInviteeSearchClear = async () => {
    setInviteeKeyword('');
    setInviteePage(1);
    await loadInvitees({ page: 1, keyword: '' });
  };

  const handleInviteePageChange = async (page) => {
    setInviteePage(page);
    await loadInvitees({ page, keyword: inviteeKeyword });
  };

  const handleClearInvitee = () => {
    clearInvitee();
    setInviteeModalVisible(false);
  };

  const inviteeStart =
    inviteesTotal === 0 ? 0 : (inviteePage - 1) * INVITEE_PAGE_SIZE + 1;
  const inviteeEnd = Math.min(inviteePage * INVITEE_PAGE_SIZE, inviteesTotal);

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
          {/* <Input
              placeholder={t('搜索 API Key...')}
              size='large'
              prefix={<Search size={16} style={{marginRight:'10px'}} />}
              onFocus={showSearchModal}
              className='dashboard-header-v2__search-input'
            /> */}
          <Button
            type='tertiary'
            theme='light'
            size='large'
            icon={<Search size={16} />}
            onClick={openInviteeModal}
            className='dashboard-header-v2__query-btn'
          >
            {selectedInvitee ? selectedInvitee.username : t('查询')}
          </Button>
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
      <Modal
        title={t('查询邀请用户')}
        visible={inviteeModalVisible}
        onCancel={() => setInviteeModalVisible(false)}
        width={640}
        footer={
          <div className='flex justify-end gap-2'>
            {selectedInvitee ? (
              <Button
                type='tertiary'
                icon={<X size={16} />}
                onClick={handleClearInvitee}
              >
                {t('清除选择')}
              </Button>
            ) : null}
            <Button
              type='primary'
              onClick={() => setInviteeModalVisible(false)}
            >
              {t('关闭')}
            </Button>
          </div>
        }
        closeOnEsc
        centered
      >
        <div className='flex gap-2 mb-4'>
          <Input
            value={inviteeKeyword}
            prefix={<Search size={16} />}
            placeholder={t('搜索被邀请人用户名')}
            showClear
            onChange={setInviteeKeyword}
            onClear={handleInviteeSearchClear}
            onEnterPress={handleInviteeSearch}
          />
          <Button
            type='primary'
            icon={<Search size={16} />}
            loading={inviteesLoading}
            onClick={handleInviteeSearch}
          >
            {t('搜索')}
          </Button>
        </div>

        <Spin spinning={inviteesLoading}>
          {/* 限高内滚，避免弹窗整体高度超出视口导致分页按钮被挤出屏幕 */}
          <div className='min-h-[220px] max-h-[55vh] overflow-y-auto pr-1'>
            {invitees.length ? (
              <div className='flex flex-col gap-2'>
                {invitees.map((invitee) => {
                  const isSelected = selectedInvitee?.id === invitee.id;
                  return (
                    <button
                      type='button'
                      key={invitee.id}
                      className={`w-full rounded-lg border px-4 py-3 text-left transition-colors flex items-center justify-between gap-3 ${
                        isSelected
                          ? 'border-[color:var(--theme-primary)] bg-[color:var(--theme-primary-20)]'
                          : 'border-[color:var(--semi-color-border)] hover:bg-[color:var(--semi-color-fill-0)]'
                      }`}
                      onClick={() => handleInviteeSelect(invitee)}
                    >
                      <div className='font-medium text-[color:var(--semi-color-text-0)] truncate'>
                        {invitee.username}
                      </div>
                      <div className='text-xs text-[color:var(--semi-color-text-2)] whitespace-nowrap'>
                        {t('注册时间')}：
                        {invitee.registerTime
                          ? timestamp2string(invitee.registerTime)
                          : '-'}
                      </div>
                    </button>
                  );
                })}
              </div>
            ) : (
              <div className='py-16 text-center text-[color:var(--semi-color-text-2)]'>
                {inviteeKeyword ? t('未找到匹配的邀请用户') : t('暂无邀请用户')}
              </div>
            )}
          </div>
        </Spin>

        <div className='mt-4 flex items-center justify-between'>
          <div className='text-xs text-[color:var(--semi-color-text-2)]'>
            {t('显示')} {inviteeStart} - {inviteeEnd} {t('共')} {inviteesTotal}{' '}
            {t('条记录')}
          </div>
          <Pagination
            total={inviteesTotal}
            pageSize={INVITEE_PAGE_SIZE}
            currentPage={inviteePage}
            onPageChange={handleInviteePageChange}
            showSizeChanger={false}
            size='small'
          />
        </div>
      </Modal>
    </section>
  );
};

export default DashboardHeader;
