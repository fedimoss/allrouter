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
import {
  Button,
  Input,
  Typography,
  Popover,
  Modal,
} from '@douyinfe/semi-ui';
import {
  IconShield,
  IconGithubLogo,
  IconKey,
} from '@douyinfe/semi-icons';
import { SiTelegram, SiLinux, SiDiscord } from 'react-icons/si';
import TelegramLoginButton from 'react-telegram-login';
import {
  API,
  showError,
  showSuccess,
  onGitHubOAuthClicked,
  onOIDCClicked,
  onLinuxDOOAuthClicked,
  onDiscordOAuthClicked,
  onCustomOAuthClicked,
  getOAuthProviderIcon,
} from '../../../../helpers';
import TwoFASetting from '../components/TwoFASetting';

const AccountManagement = ({
  t,
  userState,
  status,
  systemToken,
  runtimeDevice,
  generateAccessToken,
  handleSystemTokenClick,
  passkeyStatus,
  passkeySupported,
  passkeyRegisterLoading,
  passkeyDeleteLoading,
  onPasskeyRegister,
  onPasskeyDelete,
  onTwoFAStatusChange,
}) => {
  const renderAccountInfo = (accountId, label) => {
    if (!accountId || accountId === '') {
      return <span className='text-gray-500'>{t('未绑定')}</span>;
    }

    const popContent = (
      <div className='text-xs p-2'>
        <Typography.Paragraph copyable={{ content: accountId }}>
          {accountId}
        </Typography.Paragraph>
        {label ? (
          <div className='mt-1 text-[11px] text-gray-500'>{label}</div>
        ) : null}
      </div>
    );

    return (
      <Popover content={popContent} position='top' trigger='hover'>
        <span className='block max-w-full truncate text-gray-600 hover:text-blue-600 cursor-pointer'>
          {accountId}
        </span>
      </Popover>
    );
  };
  const isBound = (accountId) => Boolean(accountId);
  const [showTelegramBindModal, setShowTelegramBindModal] =
    React.useState(false);
  const [customOAuthBindings, setCustomOAuthBindings] = React.useState([]);
  const [customOAuthLoading, setCustomOAuthLoading] = React.useState({});

  // Fetch custom OAuth bindings
  const loadCustomOAuthBindings = async () => {
    try {
      const res = await API.get('/api/user/oauth/bindings');
      if (res.data.success) {
        setCustomOAuthBindings(res.data.data || []);
      } else {
        showError(res.data.message || t('获取绑定信息失败'));
      }
    } catch (error) {
      showError(error.response?.data?.message || error.message || t('获取绑定信息失败'));
    }
  };

  // Unbind custom OAuth provider
  const handleUnbindCustomOAuth = async (providerId, providerName) => {
    Modal.confirm({
      title: t('确认解绑'),
      content: t('确定要解绑 {{name}} 吗？', { name: providerName }),
      okText: t('确认'),
      cancelText: t('取消'),
      onOk: async () => {
        setCustomOAuthLoading((prev) => ({ ...prev, [providerId]: true }));
        try {
          const res = await API.delete(`/api/user/oauth/bindings/${providerId}`);
          if (res.data.success) {
            showSuccess(t('解绑成功'));
            await loadCustomOAuthBindings();
          } else {
            showError(res.data.message);
          }
        } catch (error) {
          showError(error.response?.data?.message || error.message || t('操作失败'));
        } finally {
          setCustomOAuthLoading((prev) => ({ ...prev, [providerId]: false }));
        }
      },
    });
  };

  // Handle bind custom OAuth
  const handleBindCustomOAuth = (provider) => {
    onCustomOAuthClicked(provider);
  };

  // Check if custom OAuth provider is bound
  const isCustomOAuthBound = (providerId) => {
    const normalizedId = Number(providerId);
    return customOAuthBindings.some((b) => Number(b.provider_id) === normalizedId);
  };

  // Get binding info for a provider
  const getCustomOAuthBinding = (providerId) => {
    const normalizedId = Number(providerId);
    return customOAuthBindings.find((b) => Number(b.provider_id) === normalizedId);
  };

  React.useEffect(() => {
    loadCustomOAuthBindings();
  }, []);

  const passkeyEnabled = passkeyStatus?.enabled;
  const lastUsedLabel = passkeyStatus?.last_used_at
    ? new Date(passkeyStatus.last_used_at).toLocaleString()
    : t('尚未使用');

  return (
    <section className='ps-settings-card'>
      {/* 卡片头部 */}
      <div className='ps-settings-card-head'>
        <div>
          <h2>{t('安全设置')}</h2>
          <p className='ps-settings-card-sub'>
            {t('两步验证、Passkey、访问令牌与第三方登录绑定')}
          </p>
        </div>
      </div>

      {/* 系统访问令牌 */}
      <div className='ps-field-row'>
        <div className='ps-field-row-label'>
          <div className='ps-field-title'>{t('系统访问令牌')}</div>
          <div className='ps-field-desc'>
            {t('用于 API 调用的身份验证令牌，请妥善保管')}
          </div>
          {systemToken && (
            <div className='mt-2 w-full' style={{ maxWidth: 360 }}>
              <Input
                readonly
                value={systemToken}
                onClick={handleSystemTokenClick}
                prefix={<IconKey />}
              />
            </div>
          )}
        </div>
        <div className='ps-field-row-action'>
          <Button
            theme='outline'
            size='small'
            onClick={generateAccessToken}
            icon={<IconKey />}
          >
            {systemToken ? t('重新生成') : t('生成令牌')}
          </Button>
        </div>
      </div>

      {/* Passkey 登录 */}
      <div className='ps-field-row'>
        <div className='ps-field-row-label'>
          <div className='ps-field-title'>{t('Passkey 登录')}</div>
          <div className='ps-field-desc'>
            {passkeyEnabled
              ? t('已启用 Passkey，无需密码即可登录')
              : t('使用 Passkey 实现免密且更安全的登录体验')}
          </div>
          <div className='ps-field-desc'>
            {t('最后使用时间')}：{lastUsedLabel}
            {!passkeySupported && (
              <span className='ps-text-warn'>
                {' '}
                · {t('当前设备不支持 Passkey')}
              </span>
            )}
          </div>
        </div>
        <div className='ps-field-row-action'>
          <Button
            type={passkeyEnabled ? 'danger' : 'primary'}
            theme='solid'
            size='small'
            onClick={
              passkeyEnabled
                ? () => {
                    Modal.confirm({
                      title: t('确认解绑 Passkey'),
                      content: t(
                        '解绑后将无法使用 Passkey 登录，确定要继续吗？',
                      ),
                      okText: t('确认解绑'),
                      cancelText: t('取消'),
                      okType: 'danger',
                      onOk: onPasskeyDelete,
                    });
                  }
                : onPasskeyRegister
            }
            icon={<IconKey />}
            disabled={!passkeySupported && !passkeyEnabled}
            loading={
              passkeyEnabled ? passkeyDeleteLoading : passkeyRegisterLoading
            }
          >
            {passkeyEnabled ? t('解绑 Passkey') : t('注册 Passkey')}
          </Button>
        </div>
      </div>

      {/* 两步验证设置 */}
      <TwoFASetting t={t} onStatusChange={onTwoFAStatusChange} />

      {/* 当前设备 */}
      <div className='ps-field-row'>
        <div className='ps-field-row-label'>
          <div className='ps-field-title'>{t('当前设备')}</div>
          <div className='ps-field-desc'>
            {runtimeDevice?.os} · {runtimeDevice?.browser}
          </div>
        </div>
        <div className='ps-field-row-action'>
          <span className='ps-device-tag'>{t('当前设备')}</span>
        </div>
      </div>

      {/* 第三方登录绑定 */}
      <div className='ps-group-title'>{t('第三方登录绑定')}</div>

      {/* GitHub绑定 */}
      <div className='ps-field-row'>
        <div className='ps-field-row-label'>
          <div className='ps-field-title'>
            <span className='ps-binding-icon'>
              <IconGithubLogo />
            </span>
            GitHub
          </div>
          <div className='ps-field-desc'>
            {renderAccountInfo(userState.user?.github_id, t('GitHub ID'))}
          </div>
        </div>
        <div className='ps-field-row-action'>
          <Button
            theme='outline'
            size='small'
            onClick={() => onGitHubOAuthClicked(status.github_client_id)}
            disabled={
              isBound(userState.user?.github_id) || !status.github_oauth
            }
          >
            {status.github_oauth ? t('绑定') : t('未启用')}
          </Button>
        </div>
      </div>

      {/* Discord绑定 */}
      <div className='ps-field-row'>
        <div className='ps-field-row-label'>
          <div className='ps-field-title'>
            <span className='ps-binding-icon'>
              <SiDiscord />
            </span>
            Discord
          </div>
          <div className='ps-field-desc'>
            {renderAccountInfo(userState.user?.discord_id, t('Discord ID'))}
          </div>
        </div>
        <div className='ps-field-row-action'>
          <Button
            theme='outline'
            size='small'
            onClick={() => onDiscordOAuthClicked(status.discord_client_id)}
            disabled={
              isBound(userState.user?.discord_id) || !status.discord_oauth
            }
          >
            {status.discord_oauth ? t('绑定') : t('未启用')}
          </Button>
        </div>
      </div>

      {/* OIDC绑定 */}
      <div className='ps-field-row'>
        <div className='ps-field-row-label'>
          <div className='ps-field-title'>
            <span className='ps-binding-icon'>
              <IconShield />
            </span>
            OIDC
          </div>
          <div className='ps-field-desc'>
            {renderAccountInfo(userState.user?.oidc_id, t('OIDC ID'))}
          </div>
        </div>
        <div className='ps-field-row-action'>
          <Button
            theme='outline'
            size='small'
            onClick={() =>
              onOIDCClicked(
                status.oidc_authorization_endpoint,
                status.oidc_client_id,
              )
            }
            disabled={isBound(userState.user?.oidc_id) || !status.oidc_enabled}
          >
            {status.oidc_enabled ? t('绑定') : t('未启用')}
          </Button>
        </div>
      </div>

      {/* Telegram绑定 */}
      <div className='ps-field-row'>
        <div className='ps-field-row-label'>
          <div className='ps-field-title'>
            <span className='ps-binding-icon'>
              <SiTelegram />
            </span>
            Telegram
          </div>
          <div className='ps-field-desc'>
            {renderAccountInfo(userState.user?.telegram_id, t('Telegram ID'))}
          </div>
        </div>
        <div className='ps-field-row-action'>
          {status.telegram_oauth ? (
            isBound(userState.user?.telegram_id) ? (
              <Button disabled size='small' theme='outline'>
                {t('已绑定')}
              </Button>
            ) : (
              <Button
                theme='outline'
                size='small'
                onClick={() => setShowTelegramBindModal(true)}
              >
                {t('绑定')}
              </Button>
            )
          ) : (
            <Button disabled size='small' theme='outline'>
              {t('未启用')}
            </Button>
          )}
        </div>
      </div>
      <Modal
        title={t('绑定 Telegram')}
        visible={showTelegramBindModal}
        onCancel={() => setShowTelegramBindModal(false)}
        footer={null}
      >
        <div className='my-3 text-sm text-gray-600'>
          {t('点击下方按钮通过 Telegram 完成绑定')}
        </div>
        <div className='flex justify-center'>
          <div className='scale-90'>
            <TelegramLoginButton
              dataAuthUrl='/api/oauth/telegram/bind'
              botName={status.telegram_bot_name}
            />
          </div>
        </div>
      </Modal>

      {/* LinuxDO绑定 */}
      <div className='ps-field-row'>
        <div className='ps-field-row-label'>
          <div className='ps-field-title'>
            <span className='ps-binding-icon'>
              <SiLinux />
            </span>
            LinuxDO
          </div>
          <div className='ps-field-desc'>
            {renderAccountInfo(userState.user?.linux_do_id, t('LinuxDO ID'))}
          </div>
        </div>
        <div className='ps-field-row-action'>
          <Button
            theme='outline'
            size='small'
            onClick={() => onLinuxDOOAuthClicked(status.linuxdo_client_id)}
            disabled={
              isBound(userState.user?.linux_do_id) || !status.linuxdo_oauth
            }
          >
            {status.linuxdo_oauth ? t('绑定') : t('未启用')}
          </Button>
        </div>
      </div>

      {/* 自定义 OAuth 提供商绑定 */}
      {status.custom_oauth_providers &&
        status.custom_oauth_providers.map((provider) => {
          const bound = isCustomOAuthBound(provider.id);
          const binding = getCustomOAuthBinding(provider.id);
          return (
            <div key={provider.slug} className='ps-field-row'>
              <div className='ps-field-row-label'>
                <div className='ps-field-title'>
                  <span className='ps-binding-icon'>
                    {getOAuthProviderIcon(
                      provider.icon || binding?.provider_icon || '',
                      20,
                    )}
                  </span>
                  {provider.name}
                </div>
                <div className='ps-field-desc'>
                  {bound
                    ? renderAccountInfo(
                        binding?.provider_user_id,
                        t('{{name}} ID', { name: provider.name }),
                      )
                    : t('未绑定')}
                </div>
              </div>
              <div className='ps-field-row-action'>
                {bound ? (
                  <Button
                    type='danger'
                    theme='outline'
                    size='small'
                    loading={customOAuthLoading[provider.id]}
                    onClick={() =>
                      handleUnbindCustomOAuth(provider.id, provider.name)
                    }
                  >
                    {t('解绑')}
                  </Button>
                ) : (
                  <Button
                    theme='outline'
                    size='small'
                    onClick={() => handleBindCustomOAuth(provider)}
                  >
                    {t('绑定')}
                  </Button>
                )}
              </div>
            </div>
          );
        })}
    </section>
  );
};

export default AccountManagement;
