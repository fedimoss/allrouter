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

import React, { useContext, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Avatar, Button, Dropdown } from '@douyinfe/semi-ui';
import { IconExit, IconUserSetting } from '@douyinfe/semi-icons';
import { API, renderQuota, showSuccess, stringToColor, formatDisplayMoney } from '../../../helpers';
import { UserContext } from '../../../context/User';

const SidebarUserPanel = ({ collapsed }) => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const navigate = useNavigate();
  const userMenuAnchorRef = useRef(null);

  if (!userState?.user) {
    return null;
  }

  const handleLogout = async () => {
    await API.get('/api/user/logout');
    showSuccess(t('注销成功!'));
    userDispatch({ type: 'logout' });
    localStorage.removeItem('user');
    navigate('/login');
  };

  return (
    <div className='sidebar-user-section'>
      <div className='sidebar-user-dropdown-anchor' ref={userMenuAnchorRef}>
        <Dropdown
          trigger='click'
          position='topLeft'
          getPopupContainer={() => userMenuAnchorRef.current || document.body}
          render={
            <Dropdown.Menu className='sidebar-user-dropdown-menu'>
              <Dropdown.Item
                onClick={() => navigate('/console/personal')}
                className='sidebar-user-dropdown-item'
              >
                <div className='sidebar-user-dropdown-row'>
                  <IconUserSetting size='small' />
                  <span>{t('个人设置')}</span>
                </div>
              </Dropdown.Item>
              <Dropdown.Item
                onClick={handleLogout}
                className='sidebar-user-dropdown-item sidebar-user-dropdown-item-danger'
              >
                <div className='sidebar-user-dropdown-row'>
                  <IconExit size='small' />
                  <span>{t('退出')}</span>
                </div>
              </Dropdown.Item>
            </Dropdown.Menu>
          }
        >
          <Button
            theme='borderless'
            className={`sidebar-user-trigger ${collapsed ? 'sidebar-user-trigger-collapsed' : ''}`}
            aria-label={t('个人设置')}
          >
            <Avatar
              size='small'
              className='sidebar-user-avatar'
            >
              {userState.user.username[0]?.toUpperCase()}
            </Avatar>
            {!collapsed && (
              <div className='sidebar-user-meta'>
                <div className='sidebar-user-name'>{userState.user.username}</div>
                <div className='sidebar-user-balance'>
                  {t('余额')}: {formatDisplayMoney(userState.user.quota ?? 0, userState.user.display_symbol)}
                </div>
              </div>
            )}
          </Button>
        </Dropdown>
      </div>
    </div>
  );
};

export default SidebarUserPanel;

