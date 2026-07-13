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

import React, { useContext, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Avatar, Button, Dropdown } from '@douyinfe/semi-ui';
import {
  IconExit,
  IconUserSetting,
  IconUnlockStroked,
} from '@douyinfe/semi-icons';
import {
  API,
  showError,
  showSuccess,
  formatDisplayMoney,
} from '../../../helpers';
import { UserContext } from '../../../context/User';
import { StatusContext } from '../../../context/Status';
import ChangePasswordModal from '../../settings/personal/modals/ChangePasswordModal';
import defaultAvatar from '../../../../public/avatar.svg';

const SidebarUserPanel = ({ collapsed }) => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);
  const navigate = useNavigate();
  const userMenuAnchorRef = useRef(null);
  const avatarUrl = userState?.user?.avatar;
  const [imgLoaded, setImgLoaded] = useState(false);
  const [showChangePasswordModal, setShowChangePasswordModal] = useState(false);
  const [, setTurnstileToken] = useState('');
  const [inputs, setInputs] = useState({
    original_password: '',
    set_new_password: '',
    set_new_password_confirmation: '',
  });

  useEffect(() => {
    if (avatarUrl) {
      setImgLoaded(false);
      const img = new Image();
      img.onload = () => setImgLoaded(true);
      img.onerror = () => setImgLoaded(true);
      img.src = avatarUrl;
    }
  }, [avatarUrl]);

  const avatarSrc = avatarUrl && imgLoaded ? avatarUrl : undefined;
  const turnstileEnabled = Boolean(statusState?.turnstile_check);
  const turnstileSiteKey = statusState?.turnstile_site_key || '';

  if (!userState?.user) {
    return null;
  }

  const handleInputChange = (name, value) => {
    setInputs((currentInputs) => ({ ...currentInputs, [name]: value }));
  };

  const changePassword = async () => {
    if (inputs.set_new_password === '') {
      showError(t('请输入新密码！'));
      return;
    }
    if (inputs.original_password === inputs.set_new_password) {
      showError(t('新密码需要和原密码不一致！'));
      return;
    }
    if (inputs.set_new_password !== inputs.set_new_password_confirmation) {
      showError(t('两次输入的密码不一致！'));
      return;
    }
    const res = await API.put(`/api/user/self`, {
      original_password: inputs.original_password,
      password: inputs.set_new_password,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('密码修改成功！'));
      setInputs({
        original_password: '',
        set_new_password: '',
        set_new_password_confirmation: '',
      });
      setTurnstileToken('');
    } else {
      showError(message);
    }
    setShowChangePasswordModal(false);
  };

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
                onClick={() => setShowChangePasswordModal(true)}
                className='sidebar-user-dropdown-item'
              >
                <div className='sidebar-user-dropdown-row'>
                  <IconUnlockStroked size='small' />
                  <span>{t('修改密码')}</span>
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
              src={avatarSrc}
            >
              {userState.user.username?.[0]?.toUpperCase()}
            </Avatar>
            {!collapsed && (
              <div className='sidebar-user-meta'>
                <div className='sidebar-user-name'>
                  {userState.user.username}
                </div>
                <div className='sidebar-user-balance'>
                  {t('余额')}:{' '}
                  {formatDisplayMoney(
                    userState.user.quota ?? 0,
                    userState.user.display_symbol,
                  )}
                </div>
              </div>
            )}
          </Button>
        </Dropdown>
      </div>
      <ChangePasswordModal
        t={t}
        showChangePasswordModal={showChangePasswordModal}
        setShowChangePasswordModal={setShowChangePasswordModal}
        inputs={inputs}
        handleInputChange={handleInputChange}
        changePassword={changePassword}
        turnstileEnabled={turnstileEnabled}
        turnstileSiteKey={turnstileSiteKey}
        setTurnstileToken={setTurnstileToken}
      />
    </div>
  );
};

export default SidebarUserPanel;
