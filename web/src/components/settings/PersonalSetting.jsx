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

import React, { useContext, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  API,
  copy,
  showError,
  showInfo,
  showSuccess,
  setStatusData,
  prepareCredentialCreationOptions,
  buildRegistrationResult,
  isPasskeySupported,
  setUserData,
  renderQuota,
  formatDisplayMoney,
  stringToColor,
  isAdmin,
  isRoot,
  selectFilter,
} from '../../helpers';
import { UserContext } from '../../context/User';
import { useActualTheme, useTheme } from '../../context/Theme';
import {
  Avatar,
  Button,
  Card,
  Input,
  InputNumber,
  Modal,
  Select,
  Switch,
  Tag,
  Upload,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import {
  BarChart3,
  Bell,
  Globe2,
  KeyRound,
  Link2,
  Laptop,
  ShieldCheck,
  UserRoundCog,
  Wallet,
  Camera,
} from 'lucide-react';

import AccountManagement from './personal/cards/AccountManagement';
import NotificationSettings from './personal/cards/NotificationSettings';
import PreferencesSettings, {
  languageOptions,
} from './personal/cards/PreferencesSettings';
import CheckinCalendar from './personal/cards/CheckinCalendar';
import EmailBindModal from './personal/modals/EmailBindModal';
import WeChatBindModal from './personal/modals/WeChatBindModal';
import AccountDeleteModal from './personal/modals/AccountDeleteModal';
import ChangePasswordModal from './personal/modals/ChangePasswordModal';
import { normalizeLanguage } from '../../i18n/language';
import defaultAvatar from '../../../public/logo-white.svg';
import './personal/personal-settings.css';

const style = {
  backgroundColor: 'var(--semi-color-overlay-bg)',
  height: '100%',
  width: '100%',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  color: 'var(--semi-color-white)',
};

const hoverMask = (<div style={style}>
  <Camera />
</div>);

const notificationTypeOptions = [
  { value: 'email', label: '邮件通知' },
  { value: 'webhook', label: 'Webhook' },
  { value: 'bark', label: 'Bark' },
  { value: 'gotify', label: 'Gotify' },
];


const fallbackTimezones = [
  'Asia/Shanghai',
  'Asia/Tokyo',
  'Asia/Seoul',
  'Asia/Singapore',
  'Asia/Bangkok',
  'Asia/Kolkata',
  'Europe/London',
  'Europe/Paris',
  'Europe/Berlin',
  'America/New_York',
  'America/Los_Angeles',
  'America/Toronto',
  'Australia/Sydney',
];

const safeParseSetting = (value) => {
  if (!value) {
    return {};
  }

  try {
    return JSON.parse(value) || {};
  } catch {
    return {};
  }
};

const detectRuntimeDevice = () => {
  if (typeof navigator === 'undefined') {
    return {
      browser: '-',
      os: '-',
    };
  }

  const ua = navigator.userAgent || '';
  const browserMatchers = [
    { key: 'Edg/', label: 'Microsoft Edge' },
    { key: 'Chrome/', label: 'Chrome' },
    { key: 'Firefox/', label: 'Firefox' },
    { key: 'Safari/', label: 'Safari' },
  ];
  const osMatchers = [
    { key: 'Windows', label: 'Windows' },
    { key: 'Mac OS X', label: 'macOS' },
    { key: 'Android', label: 'Android' },
    { key: 'iPhone', label: 'iPhone' },
    { key: 'iPad', label: 'iPadOS' },
    { key: 'Linux', label: 'Linux' },
  ];

  const browser =
    browserMatchers.find((item) => ua.includes(item.key))?.label || 'Browser';
  const os = osMatchers.find((item) => ua.includes(item.key))?.label || 'OS';

  return {
    browser,
    os,
  };
};

const PersonalSetting = () => {
  const [userState, userDispatch] = useContext(UserContext);
  const navigate = useNavigate();
  const theme = useTheme();
  const actualTheme = useActualTheme();
  const { t, i18n } = useTranslation();
  const accountAdvancedRef = useRef(null);
  const notificationAdvancedRef = useRef(null);

  const [inputs, setInputs] = useState({
    wechat_verification_code: '',
    email_verification_code: '',
    email: '',
    self_account_deletion_confirmation: '',
    original_password: '',
    set_new_password: '',
    set_new_password_confirmation: '',
  });
  const [status, setStatus] = useState({});
  const [profileInputs, setProfileInputs] = useState({
    username: '',
    avatar: defaultAvatar,
    phone_country_code: '+86',
    phone_number: '',
    timezone: '',
  });
  const [showChangePasswordModal, setShowChangePasswordModal] = useState(false);
  const [showWeChatBindModal, setShowWeChatBindModal] = useState(false);
  const [showEmailBindModal, setShowEmailBindModal] = useState(false);
  const [showAccountDeleteModal, setShowAccountDeleteModal] = useState(false);
  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');
  const [loading, setLoading] = useState(false);
  const [disableButton, setDisableButton] = useState(false);
  const [countdown, setCountdown] = useState(30);
  const [systemToken, setSystemToken] = useState('');
  const [passkeyStatus, setPasskeyStatus] = useState({ enabled: false });
  const [passkeyRegisterLoading, setPasskeyRegisterLoading] = useState(false);
  const [passkeyDeleteLoading, setPasskeyDeleteLoading] = useState(false);
  const [passkeySupported, setPasskeySupported] = useState(false);
  const [profileSaving, setProfileSaving] = useState(false);
  const [notificationSaving, setNotificationSaving] = useState(false);
  const [twoFAStatus, setTwoFAStatus] = useState({
    enabled: false,
    locked: false,
    backup_codes_remaining: 0,
  });
  const [notificationSettings, setNotificationSettings] = useState({
    warningType: 'email',
    warningThreshold: 100000,
    webhookUrl: '',
    webhookSecret: '',
    notificationEmail: '',
    barkUrl: '',
    gotifyUrl: '',
    gotifyToken: '',
    gotifyPriority: 5,
    upstreamModelUpdateNotifyEnabled: false,
    acceptUnsetModelRatioModel: false,
    recordIpLog: false,
  });

  const currentUser = userState?.user || {};
  const runtimeDevice = useMemo(() => detectRuntimeDevice(), []);

  const timezoneOptions = useMemo(() => {
    const raw =
      typeof Intl !== 'undefined' && typeof Intl.supportedValuesOf === 'function'
        ? Intl.supportedValuesOf('timeZone')
        : fallbackTimezones;
    const unique = Array.from(new Set(raw));
    return unique.map((tz) => ({ value: tz, label: tz }));
  }, []);

  const phoneCountryCodeOptions = useMemo(() => [
    { value: '+86', label: t('中国大陆 (+86)') },
    { value: '+852', label: t('中国香港 (+852)') },
    { value: '+853', label: t('中国澳门 (+853)') },
    { value: '+886', label: t('中国台湾 (+886)') },
    { value: '+1', label: t('美国/加拿大 (+1)') },
    { value: '+81', label: t('日本 (+81)') },
    { value: '+82', label: t('韩国 (+82)') },
    { value: '+65', label: t('新加坡 (+65)') },
    { value: '+66', label: t('泰国 (+66)') },
    { value: '+84', label: t('越南 (+84)') },
    { value: '+91', label: t('印度 (+91)') },
    { value: '+44', label: t('英国 (+44)') },
    { value: '+49', label: t('德国 (+49)') },
    { value: '+33', label: t('法国 (+33)') },
    { value: '+61', label: t('澳大利亚 (+61)') },
  ], [t]);

  const currentLanguageLabel = useMemo(() => {
    const activeLanguage = normalizeLanguage(
      safeParseSetting(currentUser?.setting).language || i18n.language,
    );
    return (
      languageOptions.find((item) => item.value === activeLanguage)?.label ||
      activeLanguage
    );
  }, [currentUser?.setting, i18n.language]);

  const currentThemeLabel = useMemo(() => {
    if (theme === 'auto') {
      return `${t('跟随系统')} · ${actualTheme === 'dark' ? t('深色') : t('浅色')}`;
    }
    return theme === 'dark' ? t('深色') : t('浅色');
  }, [actualTheme, t, theme]);

  const roleLabel = useMemo(() => {
    if (isRoot()) {
      return t('超级管理员');
    }
    if (isAdmin()) {
      return t('管理员');
    }
    return t('普通用户');
  }, [t]);

  const boundAccountCount = useMemo(() => {
    const bindings = [
      currentUser?.email,
      currentUser?.github_id,
      currentUser?.discord_id,
      currentUser?.oidc_id,
      currentUser?.wechat_id,
      currentUser?.telegram_id,
      currentUser?.linux_do_id,
    ].filter(Boolean).length;

    return bindings + (passkeyStatus?.enabled ? 1 : 0);
  }, [
    currentUser?.discord_id,
    currentUser?.email,
    currentUser?.github_id,
    currentUser?.linux_do_id,
    currentUser?.oidc_id,
    currentUser?.telegram_id,
    currentUser?.wechat_id,
    passkeyStatus?.enabled,
  ]);

  const metricItems = useMemo(
    () => [
      {
        key: 'quota',
        label: t('当前余额'),
        value: formatDisplayMoney(currentUser?.quota || 0, currentUser?.display_symbol),
        icon: Wallet,
      },
      {
        key: 'used',
        label: t('历史消耗'),
        value: formatDisplayMoney(currentUser?.used_quota || 0, currentUser?.display_symbol),
        icon: BarChart3,
      },
      {
        key: 'request',
        label: t('请求次数'),
        value: currentUser?.request_count || 0,
        icon: Bell,
      },
      {
        key: 'binding',
        label: t('已绑定方式'),
        value: boundAccountCount,
        icon: Link2,
      },
    ],
    [
      boundAccountCount,
      currentUser?.quota,
      currentUser?.request_count,
      currentUser?.used_quota,
      t,
    ],
  );

  const notificationTargetSummary = useMemo(() => {
    switch (notificationSettings.warningType) {
      case 'webhook':
        return notificationSettings.webhookUrl || t('未配置 Webhook 地址');
      case 'bark':
        return notificationSettings.barkUrl || t('未配置 Bark 推送地址');
      case 'gotify':
        return notificationSettings.gotifyUrl || t('未配置 Gotify 服务器');
      case 'email':
      default:
        return (
          notificationSettings.notificationEmail ||
          currentUser?.email ||
          t('将使用账号绑定邮箱接收通知')
        );
    }
  }, [
    currentUser?.email,
    notificationSettings.barkUrl,
    notificationSettings.gotifyUrl,
    notificationSettings.notificationEmail,
    notificationSettings.warningType,
    notificationSettings.webhookUrl,
    t,
  ]);

  const isAdminUser = (currentUser?.role || 0) >= 10;

  useEffect(() => {
    let saved = localStorage.getItem('status');
    if (saved) {
      const parsed = JSON.parse(saved);
      setStatus(parsed);
      if (parsed.turnstile_check) {
        setTurnstileEnabled(true);
        setTurnstileSiteKey(parsed.turnstile_site_key);
      } else {
        setTurnstileEnabled(false);
        setTurnstileSiteKey('');
      }
    }

    (async () => {
      try {
        const res = await API.get('/api/status');
        const { success, data } = res.data;
        if (success && data) {
          setStatus(data);
          setStatusData(data);
          if (data.turnstile_check) {
            setTurnstileEnabled(true);
            setTurnstileSiteKey(data.turnstile_site_key);
          } else {
            setTurnstileEnabled(false);
            setTurnstileSiteKey('');
          }
        }
      } catch {
        // ignore and keep local status
      }
    })();

    getUserData();
    loadTwoFAStatus();

    isPasskeySupported()
      .then(setPasskeySupported)
      .catch(() => setPasskeySupported(false));
  }, []);

  useEffect(() => {
    let countdownInterval = null;
    if (disableButton && countdown > 0) {
      countdownInterval = setInterval(() => {
        setCountdown(countdown - 1);
      }, 1000);
    } else if (countdown === 0) {
      setDisableButton(false);
      setCountdown(30);
    }
    return () => clearInterval(countdownInterval);
  }, [disableButton, countdown]);

  useEffect(() => {
    if (currentUser?.setting) {
      const settings = safeParseSetting(currentUser.setting);
      setNotificationSettings({
        warningType: settings.notify_type || 'email',
        warningThreshold: settings.quota_warning_threshold || 500000,
        webhookUrl: settings.webhook_url || '',
        webhookSecret: settings.webhook_secret || '',
        notificationEmail: settings.notification_email || '',
        barkUrl: settings.bark_url || '',
        gotifyUrl: settings.gotify_url || '',
        gotifyToken: settings.gotify_token || '',
        gotifyPriority:
          settings.gotify_priority !== undefined ? settings.gotify_priority : 5,
        upstreamModelUpdateNotifyEnabled:
          settings.upstream_model_update_notify_enabled === true,
        acceptUnsetModelRatioModel:
          settings.accept_unset_model_ratio_model || false,
        recordIpLog: settings.record_ip_log || false,
      });
    }
  }, [currentUser?.setting]);

  useEffect(() => {
    const settings = safeParseSetting(currentUser?.setting);
    let detectedTimezone = '';
    if (typeof Intl !== 'undefined') {
      try {
        detectedTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || '';
      } catch {
        detectedTimezone = '';
      }
    }
    setProfileInputs((prev) => ({
      username: currentUser?.username || prev.username || '',
      avatar: currentUser?.avatar || prev.avatar || settings.avatar || defaultAvatar,
      phone_country_code:
        currentUser?.phone_country_code ||
        prev.phone_country_code ||
        settings.phone_country_code ||
        '+86',
      phone_number:
        currentUser?.phone_number || prev.phone_number || settings.phone_number || '',
      timezone:
        currentUser?.timezone ||
        prev.timezone ||
        settings.timezone ||
        detectedTimezone ||
        'Asia/Shanghai',
    }));
    setInputs((prev) => ({
      ...prev,
      email: currentUser?.email || prev.email,
    }));
  }, [
    currentUser?.avatar,
    currentUser?.email,
    currentUser?.phone_country_code,
    currentUser?.phone_number,
    currentUser?.setting,
    currentUser?.timezone,
    currentUser?.username,
  ]);

  const scrollToRef = (ref) => {
    ref.current?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  };

  const handleInputChange = (name, value) => {
    setInputs((currentInputs) => ({ ...currentInputs, [name]: value }));
  };

  const handleProfileChange = (name, value) => {
    setProfileInputs((currentInputs) => ({
      ...currentInputs,
      [name]: value,
    }));
  };

  const handleAvatarUpload = async ({
    file,
    fileInstance,
    onSuccess,
    onError,
  }) => {
    try {
      const uploadFile = fileInstance || file?.fileInstance;
      if (!uploadFile) {
        throw new Error('invalid file');
      }
      const formData = new FormData();
      formData.append('avatar', uploadFile);
      const res = await API.post('/api/user/avatar', formData, {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        throw new Error(message || t('上传头像失败，请重试'));
      }
      const avatarPath = window.location.origin + (data.url || '');
      if (!avatarPath) {
        throw new Error(t('头像返回地址无效'));
      }
      setProfileInputs((prev) => ({
        ...prev,
        avatar: avatarPath,
      }));
      showSuccess(t('头像上传成功'));
      onSuccess?.(data || {});
    } catch (error) {
      showError(error?.message || t('上传头像失败，请重试'));
      onError?.({ status: 500 }, error);
    }
  };

  const generateAccessToken = async () => {
    const res = await API.get('/api/user/token');
    const { success, message, data } = res.data;
    if (success) {
      setSystemToken(data);
      await copy(data);
      showSuccess(t('令牌已重置并已复制到剪贴板'));
    } else {
      showError(message);
    }
  };

  const loadTwoFAStatus = async () => {
    try {
      const res = await API.get('/api/user/2fa/status');
      if (res.data.success) {
        setTwoFAStatus(res.data.data || {});
      }
    } catch {
      // ignore quick summary errors
    }
  };

  const loadPasskeyStatus = async () => {
    try {
      const res = await API.get('/api/user/passkey');
      const { success, data, message } = res.data;
      if (success) {
        setPasskeyStatus({
          enabled: data?.enabled || false,
          last_used_at: data?.last_used_at || null,
          backup_eligible: data?.backup_eligible || false,
          backup_state: data?.backup_state || false,
        });
      } else {
        showError(message);
      }
    } catch {
      // ignore and keep default state
    }
  };

  const handleRegisterPasskey = async () => {
    if (!passkeySupported || !window.PublicKeyCredential) {
      showInfo(t('当前设备不支持 Passkey'));
      return;
    }
    setPasskeyRegisterLoading(true);
    try {
      const beginRes = await API.post('/api/user/passkey/register/begin');
      const { success, message, data } = beginRes.data;
      if (!success) {
        showError(message || t('无法发起 Passkey 注册'));
        return;
      }

      const publicKey = prepareCredentialCreationOptions(
        data?.options || data?.publicKey || data,
      );
      const credential = await navigator.credentials.create({ publicKey });
      const payload = buildRegistrationResult(credential);
      if (!payload) {
        showError(t('Passkey 注册失败，请重试'));
        return;
      }

      const finishRes = await API.post(
        '/api/user/passkey/register/finish',
        payload,
      );
      if (finishRes.data.success) {
        showSuccess(t('Passkey 注册成功'));
        await loadPasskeyStatus();
      } else {
        showError(finishRes.data.message || t('Passkey 注册失败，请重试'));
      }
    } catch (error) {
      if (error?.name === 'AbortError') {
        showInfo(t('已取消 Passkey 注册'));
      } else {
        showError(t('Passkey 注册失败，请重试'));
      }
    } finally {
      setPasskeyRegisterLoading(false);
    }
  };

  const handleRemovePasskey = async () => {
    setPasskeyDeleteLoading(true);
    try {
      const res = await API.delete('/api/user/passkey');
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('Passkey 已解绑'));
        await loadPasskeyStatus();
      } else {
        showError(message || t('操作失败，请重试'));
      }
    } catch {
      showError(t('操作失败，请重试'));
    } finally {
      setPasskeyDeleteLoading(false);
    }
  };

  const getUserData = async () => {
    let res = await API.get(`/api/user/self`);
    const { success, message, data } = res.data;
    if (success) {
      userDispatch({ type: 'login', payload: data });
      setUserData(data);
      await loadPasskeyStatus();
      await loadTwoFAStatus();
    } else {
      showError(message);
    }
  };

  const saveProfile = async () => {
    const username = profileInputs.username.trim();
    const phoneNumber = profileInputs.phone_number.trim();

    if (!username) {
      showError(t('用户名不能为空'));
      return;
    }

    setProfileSaving(true);
    try {
      const res = await API.put('/api/user/self', {
        username,
        avatar: profileInputs.avatar || '',
        phone_country_code: profileInputs.phone_country_code || '+86',
        phone_number: phoneNumber,
        timezone: profileInputs.timezone || 'Asia/Shanghai',
        email: inputs.email || currentUser?.email || '',
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('账户信息已更新'));
        await getUserData();
      } else {
        showError(message);
      }
    } catch {
      showError(t('保存失败，请重试'));
    } finally {
      setProfileSaving(false);
    }
  };

  const handleSystemTokenClick = async (e) => {
    e.target.select();
    await copy(e.target.value);
    showSuccess(t('系统令牌已复制到剪切板'));
  };

  const deleteAccount = async () => {
    if (inputs.self_account_deletion_confirmation !== currentUser.username) {
      showError(t('请输入你的账户名以确认删除！'));
      return;
    }

    const res = await API.delete('/api/user/self');
    const { success, message } = res.data;

    if (success) {
      showSuccess(t('账户已删除！'));
      await API.get('/api/user/logout');
      userDispatch({ type: 'logout' });
      localStorage.removeItem('user');
      navigate('/login');
    } else {
      showError(message);
    }
  };

  const bindWeChat = async () => {
    if (inputs.wechat_verification_code === '') return;
    const res = await API.post('/api/oauth/wechat/bind', {
      code: inputs.wechat_verification_code,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('微信账户绑定成功！'));
      setShowWeChatBindModal(false);
    } else {
      showError(message);
    }
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
      setShowWeChatBindModal(false);
    } else {
      showError(message);
    }
    setShowChangePasswordModal(false);
  };

  const sendVerificationCode = async () => {
    if (inputs.email === '') {
      showError(t('请输入邮箱！'));
      return;
    }
    setDisableButton(true);
    if (turnstileEnabled && turnstileToken === '') {
      showInfo(t('请稍后几秒重试，Turnstile 正在检查用户环境！'));
      return;
    }
    setLoading(true);
    const res = await API.get(
      `/api/verification?email=${inputs.email}&turnstile=${turnstileToken}`,
    );
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('验证码发送成功，请检查邮箱！'));
    } else {
      showError(message);
    }
    setLoading(false);
  };

  const bindEmail = async () => {
    if (inputs.email_verification_code === '') {
      showError(t('请输入邮箱验证码！'));
      return;
    }
    setLoading(true);
    const res = await API.post('/api/oauth/email/bind', {
      email: inputs.email,
      code: inputs.email_verification_code,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('邮箱账户绑定成功！'));
      setShowEmailBindModal(false);
      await getUserData();
    } else {
      showError(message);
    }
    setLoading(false);
  };

  const handleNotificationSettingChange = (type, value) => {
    setNotificationSettings((prev) => ({
      ...prev,
      [type]: value?.target
        ? value.target.value !== undefined
          ? value.target.value
          : value.target.checked
        : value,
    }));
  };

  const saveNotificationSettings = async () => {
    setNotificationSaving(true);
    try {
      const res = await API.put('/api/user/setting', {
        notify_type: notificationSettings.warningType,
        quota_warning_threshold: parseFloat(
          notificationSettings.warningThreshold,
        ),
        webhook_url: notificationSettings.webhookUrl,
        webhook_secret: notificationSettings.webhookSecret,
        notification_email: notificationSettings.notificationEmail,
        bark_url: notificationSettings.barkUrl,
        gotify_url: notificationSettings.gotifyUrl,
        gotify_token: notificationSettings.gotifyToken,
        gotify_priority: (() => {
          const parsed = parseInt(notificationSettings.gotifyPriority);
          return isNaN(parsed) ? 5 : parsed;
        })(),
        upstream_model_update_notify_enabled:
          notificationSettings.upstreamModelUpdateNotifyEnabled === true,
        accept_unset_model_ratio_model:
          notificationSettings.acceptUnsetModelRatioModel,
        record_ip_log: notificationSettings.recordIpLog,
      });

      if (res.data.success) {
        showSuccess(t('设置保存成功'));
        await getUserData();
      } else {
        showError(res.data.message);
      }
    } catch {
      showError(t('设置保存失败'));
    } finally {
      setNotificationSaving(false);
    }
  };

  const handlePasskeySwitch = (checked) => {
    if (checked) {
      handleRegisterPasskey();
      return;
    }

    Modal.confirm({
      title: t('确认解绑 Passkey'),
      content: t('解绑后将无法使用 Passkey 登录，确定要继续吗？'),
      okText: t('确认解绑'),
      cancelText: t('取消'),
      okType: 'danger',
      onOk: handleRemovePasskey,
    });
  };

  const displayName = currentUser?.username || profileInputs.username || '-';
  const localTimeLabel = useMemo(() => {
    if (typeof Intl === 'undefined') {
      return '-';
    }
    try {
      return new Intl.DateTimeFormat(undefined, {
        hour: '2-digit',
        minute: '2-digit',
        month: 'short',
        day: 'numeric',
        timeZone: profileInputs.timezone || 'Asia/Shanghai',
      }).format(new Date());
    } catch {
      return new Intl.DateTimeFormat(undefined, {
        hour: '2-digit',
        minute: '2-digit',
        month: 'short',
        day: 'numeric',
      }).format(new Date());
    }
  }, [profileInputs.timezone]);

  return (
    <div className='personal-setting-v2'>
      <div className='personal-setting-v2-backdrop' aria-hidden='true' />
      <div className='flex justify-center'>
        <div className='personal-setting-v2-container w-full px-2 sm:px-4 lg:px-6'>
          <section className='personal-setting-v2-head personal-v3-hero'>
            <h1 className='person-h1-title'>{t('个人中心')}</h1>
            <p className='person-title-info'>{t('管理您的账户信息、安全设置、通知偏好及界面显示。')}</p>
          </section>

          <section className='personal-v3-main-grid'>
            <Card
              className='personal-v3-card !rounded-[24px]'
              bodyStyle={{ padding: 0 }}
            >
              
              <div className='personal-v3-card-body'>
                <div className='card-heard'>
                  <div><svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-user-icon lucide-user"><path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg></div>
                  <div className='card-h-title'>{t('账户信息')}</div>
                </div>
                <div className='personal-v3-card-header'>
                  <div className='personal-v3-card-title'>
                    <Upload
                      action='/'
                      accept='image/*'
                      showUploadList={false}
                      uploadTrigger='auto'
                      customRequest={handleAvatarUpload}
                    >
                      <Avatar
                        size='large'
                        shape='square'
                        hoverMask={hoverMask}
                        src={profileInputs.avatar || undefined}
                      />
                    </Upload>
                    <div className='min-w-0'>
                      <div className='personal-v3-profile-name'>{displayName}</div>
                      <div className='personal-v3-profile-subtitle'>
                        {t('管理您的基础资料、账户状态与常用信息。')}
                      </div>
                      {/* <div className='personal-v3-chip-row'>
                        <Tag shape='circle' className='personal-v3-soft-tag'>
                          {roleLabel}
                        </Tag>
                        <Tag shape='circle' className='personal-v3-soft-tag'>
                          ID #{currentUser?.id || '-'}
                        </Tag>
                        <Tag shape='circle' className='personal-v3-soft-tag'>
                          {currentUser?.group || t('默认分组')}
                        </Tag>
                      </div> */}
                    </div>
                  </div>
                  {/* <Button
                    theme='outline'
                    onClick={() => scrollToRef(accountAdvancedRef)}
                    className='personal-v3-secondary-btn'
                  >
                    {t('账户绑定与安全设置')}
                  </Button> */}
                </div>

                <div className='personal-v3-account-form-grid'>
                  <div className='personal-v3-field'>
                    <label htmlFor='profile-username'>{t('用户名')}</label>
                    <Input
                      size='large'
                      id='profile-username'
                      value={profileInputs.username}
                      onChange={(value) => handleProfileChange('username', value)}
                      placeholder={t('请输入用户名')}
                      showClear
                    />
                  </div>

                  <div className='personal-v3-field'>
                    <label htmlFor='profile-email'>
                      <span>{t('邮箱地址')}</span>
                      <span className='personal-v3-field-tag'>
                        {currentUser?.email ? t('已绑定') : t('未绑定')}
                      </span>
                    </label>
                    <Input
                      size='large'
                      id='profile-email'
                      value={currentUser?.email || t('暂未绑定邮箱')}
                      readonly
                    />
                    <div className='personal-v3-field-note'>
                      {currentUser?.email
                        ? t('如需修改邮箱，请在下方高级设置中重新绑定。')
                        : t('可在下方高级设置中绑定邮箱，用于通知和登录验证。')}
                    </div>
                  </div>

                  <div className='personal-v3-field'>
                    <label>{t('手机号')}</label>
                    <Input
                      size='large'
                      value={profileInputs.phone_number}
                      addonBefore={
                        <Select
                          size='large'
                          value={profileInputs.phone_country_code}
                          optionList={phoneCountryCodeOptions}
                          onChange={(value) =>
                            handleProfileChange('phone_country_code', value)
                          }
                          filter={selectFilter}
                          searchPosition='dropdown'
                          style={{ width: 138 }}
                        />
                      }
                      onChange={(value) => handleProfileChange('phone_number', value)}
                      placeholder={t('请输入手机号')}
                      showClear
                    />
                    <div className='personal-v3-field-note'>
                      {t('显示名称将展示为区号 + 手机号格式')}
                    </div>
                  </div>

                  <div className='personal-v3-field'>
                    <label htmlFor='profile-timezone'>{t('时区')}</label>
                    <Select
                      size='large'
                      id='profile-timezone'
                      value={profileInputs.timezone}
                      optionList={timezoneOptions}
                      filter={selectFilter}
                      searchPosition='dropdown'
                      onChange={(value) => handleProfileChange('timezone', value)}
                      style={{ width: '100%' }}
                    />
                    <div className='personal-v3-field-note'>
                      {`${profileInputs.timezone || '-'} · ${localTimeLabel}`}
                    </div>
                  </div>
                </div>

                <div className='personal-v3-note-banner'>
                  <strong>{t('提示')}：</strong>
                  {t(
                    '账户绑定、密码修改、两步验证与更多安全操作已保留在下方高级设置区域。',
                  )}
                </div>

                <div className='personal-v3-stat-grid'>
                  {metricItems.map((item) => {
                    const Icon = item.icon;
                    return (
                      <div key={item.key} className='personal-v3-stat-card'>
                        <div className='personal-v3-stat-card-head'>
                          <div>
                            <div className='personal-v3-stat-label'>
                              {item.label}
                            </div>
                            <div className='personal-v3-stat-value'>
                              {item.value}
                            </div>
                          </div>
                          <span className='personal-v3-stat-icon'>
                            <Icon size={18} />
                          </span>
                        </div>
                      </div>
                    );
                  })}
                </div>

                <div className='personal-v3-card-actions'>
                  <Button
                    theme='outline'
                    onClick={() => scrollToRef(accountAdvancedRef)}
                  >
                    {t('更多账户设置')}
                  </Button>
                  <Button
                    type='primary'
                    onClick={saveProfile}
                    loading={profileSaving}
                  >
                    {t('保存修改')}
                  </Button>
                </div>
              </div>
            </Card>

            <Card
              className='personal-v3-card !rounded-[24px]'
              bodyStyle={{ padding: 0 }}
            >
              <div className='personal-v3-card-body'>
                <div className='personal-v3-card-header'>
                  <div className='personal-v3-card-title'>
                    <span className='personal-v3-icon-badge'>
                      <Bell size={18} />
                    </span>
                    <div>
                      <h3>{t('通知偏好')}</h3>
                      <p>{t('快速调整常用通知、风险与隐私开关。')}</p>
                    </div>
                  </div>
                </div>

                <div className='personal-v3-quick-stack'>
                  <div className='personal-v3-inline-field'>
                    <div>
                      <label>{t('通知方式')}</label>
                      <span>{t('额度预警通知使用的推送渠道')}</span>
                    </div>
                    <Select
                      value={notificationSettings.warningType}
                      optionList={notificationTypeOptions.map((item) => ({
                        value: item.value,
                        label: t(item.label),
                      }))}
                      onChange={(value) =>
                        handleNotificationSettingChange('warningType', value)
                      }
                    />
                  </div>

                  <div className='personal-v3-inline-field'>
                    <div>
                      <label>{t('预警阈值')}</label>
                      <span>{t('额度低于该值时发送通知')}</span>
                    </div>
                    <InputNumber
                      value={Number(notificationSettings.warningThreshold) || 0}
                      min={1}
                      step={100000}
                      onChange={(value) =>
                        handleNotificationSettingChange('warningThreshold', value)
                      }
                    />
                  </div>

                  <div className='personal-v3-note-banner personal-v3-note-banner-compact'>
                    <strong>{t('当前通知目标')}：</strong>
                    {notificationTargetSummary}
                  </div>

                  {isAdminUser && (
                    <div className='personal-v3-setting-row'>
                      <div>
                        <h4>{t('上游模型更新通知')}</h4>
                        <p>
                          {t('仅管理员可用，接收模型变更或检测异常汇总')}
                        </p>
                      </div>
                      <Switch
                        checked={
                          notificationSettings.upstreamModelUpdateNotifyEnabled ===
                          true
                        }
                        onChange={(checked) =>
                          handleNotificationSettingChange(
                            'upstreamModelUpdateNotifyEnabled',
                            checked,
                          )
                        }
                      />
                    </div>
                  )}

                  <div className='personal-v3-setting-row'>
                    <div>
                      <h4>{t('接受未设置价格模型')}</h4>
                      <p>
                        {t(
                          '仅在信任站点时开启，避免因价格缺失造成费用风险',
                        )}
                      </p>
                    </div>
                    <Switch
                      checked={notificationSettings.acceptUnsetModelRatioModel}
                      onChange={(checked) =>
                        handleNotificationSettingChange(
                          'acceptUnsetModelRatioModel',
                          checked,
                        )
                      }
                    />
                  </div>

                  <div className='personal-v3-setting-row'>
                    <div>
                      <h4>{t('记录请求与错误日志 IP')}</h4>
                      <p>{t('控制日志中是否保留客户端 IP 信息')}</p>
                    </div>
                    <Switch
                      checked={notificationSettings.recordIpLog}
                      onChange={(checked) =>
                        handleNotificationSettingChange('recordIpLog', checked)
                      }
                    />
                  </div>
                </div>

                <div className='personal-v3-card-actions personal-v3-card-actions-split'>
                  <Button
                    theme='outline'
                    onClick={() => scrollToRef(notificationAdvancedRef)}
                  >
                    {t('高级通知设置')}
                  </Button>
                  <Button
                    type='primary'
                    onClick={saveNotificationSettings}
                    loading={notificationSaving}
                  >
                    {t('保存设置')}
                  </Button>
                </div>
              </div>
            </Card>
          </section>

          <section className='personal-v3-section'>
            <div className='personal-v3-section-head'>
              <div>
                <h2>{t('安全中心')}</h2>
                <p>
                  {t('管理访问令牌、双重验证、Passkey 与当前设备会话信息。')}
                </p>
              </div>
            </div>

            <div className='personal-v3-security-grid'>
              <Card
                className='personal-v3-card !rounded-[24px]'
                bodyStyle={{ padding: 0 }}
              >
                <div className='personal-v3-card-body'>
                  <div className='personal-v3-card-title personal-v3-card-title-compact'>
                    <span className='personal-v3-icon-badge'>
                      <KeyRound size={18} />
                    </span>
                    <div>
                      <h3>{t('系统访问令牌')}</h3>
                      <p>
                        {t('用于 API 调用的身份校验，生成后会自动复制。')}
                      </p>
                    </div>
                  </div>

                  {systemToken ? (
                    <Input
                      readonly
                      value={systemToken}
                      onClick={handleSystemTokenClick}
                    />
                  ) : (
                    <div className='personal-v3-empty-state'>
                      {t('尚未生成系统访问令牌，点击下方按钮立即创建。')}
                    </div>
                  )}

                  <div className='personal-v3-card-actions'>
                    <Button type='primary' onClick={generateAccessToken}>
                      {systemToken ? t('重新生成令牌') : t('生成令牌')}
                    </Button>
                  </div>
                </div>
              </Card>

              <Card
                className='personal-v3-card !rounded-[24px]'
                bodyStyle={{ padding: 0 }}
              >
                <div className='personal-v3-card-body'>
                  <div className='personal-v3-card-title personal-v3-card-title-compact'>
                    <span className='personal-v3-icon-badge'>
                      <ShieldCheck size={18} />
                    </span>
                    <div>
                      <h3>{t('验证与登录保护')}</h3>
                      <p>{t('集中查看 2FA、Passkey 与账号保护状态。')}</p>
                    </div>
                  </div>

                  <div className='personal-v3-quick-stack'>
                    <div className='personal-v3-setting-row'>
                      <div>
                        <h4>{t('双重验证（2FA）')}</h4>
                        <p>
                          {twoFAStatus?.enabled
                            ? t('已启用，登录时需要额外验证码')
                            : t('未启用，建议尽快配置额外验证方式')}
                        </p>
                      </div>
                      <Tag shape='circle' className='personal-v3-soft-tag'>
                        {twoFAStatus?.enabled ? t('已开启') : t('未开启')}
                      </Tag>
                    </div>

                    <div className='personal-v3-setting-row'>
                      <div>
                        <h4>{t('Passkey 登录')}</h4>
                        <p>
                          {passkeyStatus?.enabled
                            ? t('当前已启用 Passkey，可免密登录')
                            : t('使用设备密钥提升登录安全性与便捷性')}
                        </p>
                      </div>
                      <Switch
                        checked={passkeyStatus?.enabled === true}
                        disabled={
                          (!passkeySupported && !passkeyStatus?.enabled) ||
                          passkeyRegisterLoading ||
                          passkeyDeleteLoading
                        }
                        onChange={handlePasskeySwitch}
                      />
                    </div>
                  </div>

                  <div className='personal-v3-status-hint'>
                    {twoFAStatus?.enabled
                      ? t('备用码剩余：{{count}} 个', {
                          count: twoFAStatus?.backup_codes_remaining || 0,
                        })
                      : t(
                          '可在高级设置中完成 2FA 配置、备用码管理与更多安全操作。',
                        )}
                  </div>

                  <div className='personal-v3-card-actions'>
                    <Button
                      theme='outline'
                      onClick={() => scrollToRef(accountAdvancedRef)}
                    >
                      {t('配置验证方式')}
                    </Button>
                    {!passkeySupported && !passkeyStatus?.enabled ? (
                      <Tag shape='circle' className='personal-v3-soft-tag'>
                        {t('当前设备不支持 Passkey')}
                      </Tag>
                    ) : null}
                  </div>
                </div>
              </Card>

              <Card
                className='personal-v3-card !rounded-[24px]'
                bodyStyle={{ padding: 0 }}
              >
                <div className='personal-v3-card-body'>
                  <div className='personal-v3-card-title personal-v3-card-title-compact'>
                    <span className='personal-v3-icon-badge'>
                      <Laptop size={18} />
                    </span>
                    <div>
                      <h3>{t('当前设备与会话')}</h3>
                      <p>
                        {t('基于浏览器环境展示当前设备、语言与主题信息。')}
                      </p>
                    </div>
                  </div>

                  <div className='personal-v3-session-list'>
                    <div className='personal-v3-session-row'>
                      <div>
                        <h4>{t('当前设备')}</h4>
                        <p>{`${runtimeDevice.os} · ${runtimeDevice.browser}`}</p>
                      </div>
                      <Tag shape='circle' className='personal-v3-soft-tag'>
                        {t('当前')}
                      </Tag>
                    </div>

                    <div className='personal-v3-session-row'>
                      <div>
                        <h4>{t('语言设置')}</h4>
                        <p>{currentLanguageLabel}</p>
                      </div>
                      <span className='personal-v3-session-badge'>
                        <Globe2 size={14} />
                      </span>
                    </div>

                    <div className='personal-v3-session-row'>
                      <div>
                        <h4>{t('界面主题')}</h4>
                        <p>{currentThemeLabel}</p>
                      </div>
                      <span className='personal-v3-session-badge'>
                        <UserRoundCog size={14} />
                      </span>
                    </div>

                    <div className='personal-v3-session-row'>
                      <div>
                        <h4>{t('已绑定方式')}</h4>
                        <p>
                          {t('{{count}} 项登录或通知方式已绑定', {
                            count: boundAccountCount,
                          })}
                        </p>
                      </div>
                      <span className='personal-v3-session-badge'>
                        <Link2 size={14} />
                      </span>
                    </div>
                  </div>
                </div>
              </Card>
            </div>
          </section>

          <PreferencesSettings t={t} />

          <section className='personal-v3-section personal-v3-advanced'>
            <div className='personal-v3-section-head'>
              <div>
                <h2>{t('高级设置')}</h2>
                <p>
                  {t(
                    '保留原有完整功能，继续管理账户绑定、细分通知策略、签到奖励与更多安全能力。',
                  )}
                </p>
              </div>
            </div>

            <div className='personal-setting-v2-board mt-4 md:mt-6'>
              <div
                ref={accountAdvancedRef}
                className='personal-setting-v2-col personal-v3-anchor'
              >
                <AccountManagement
                  t={t}
                  userState={userState}
                  status={status}
                  systemToken={systemToken}
                  setShowEmailBindModal={setShowEmailBindModal}
                  setShowWeChatBindModal={setShowWeChatBindModal}
                  generateAccessToken={generateAccessToken}
                  handleSystemTokenClick={handleSystemTokenClick}
                  setShowChangePasswordModal={setShowChangePasswordModal}
                  setShowAccountDeleteModal={setShowAccountDeleteModal}
                  passkeyStatus={passkeyStatus}
                  passkeySupported={passkeySupported}
                  passkeyRegisterLoading={passkeyRegisterLoading}
                  passkeyDeleteLoading={passkeyDeleteLoading}
                  onPasskeyRegister={handleRegisterPasskey}
                  onPasskeyDelete={handleRemovePasskey}
                  onTwoFAStatusChange={setTwoFAStatus}
                />
              </div>

              <div
                ref={notificationAdvancedRef}
                className='personal-setting-v2-col personal-v3-anchor'
              >
                <NotificationSettings
                  t={t}
                  notificationSettings={notificationSettings}
                  handleNotificationSettingChange={handleNotificationSettingChange}
                  saveNotificationSettings={saveNotificationSettings}
                />

                {status?.checkin_enabled && (
                  <CheckinCalendar
                    t={t}
                    status={status}
                    turnstileEnabled={turnstileEnabled}
                    turnstileSiteKey={turnstileSiteKey}
                  />
                )}
              </div>
            </div>
          </section>
        </div>
      </div>

      <EmailBindModal
        t={t}
        showEmailBindModal={showEmailBindModal}
        setShowEmailBindModal={setShowEmailBindModal}
        inputs={inputs}
        handleInputChange={handleInputChange}
        sendVerificationCode={sendVerificationCode}
        bindEmail={bindEmail}
        disableButton={disableButton}
        loading={loading}
        countdown={countdown}
        turnstileEnabled={turnstileEnabled}
        turnstileSiteKey={turnstileSiteKey}
        setTurnstileToken={setTurnstileToken}
      />

      <WeChatBindModal
        t={t}
        showWeChatBindModal={showWeChatBindModal}
        setShowWeChatBindModal={setShowWeChatBindModal}
        inputs={inputs}
        handleInputChange={handleInputChange}
        bindWeChat={bindWeChat}
        status={status}
      />

      <AccountDeleteModal
        t={t}
        showAccountDeleteModal={showAccountDeleteModal}
        setShowAccountDeleteModal={setShowAccountDeleteModal}
        inputs={inputs}
        handleInputChange={handleInputChange}
        deleteAccount={deleteAccount}
        userState={userState}
        turnstileEnabled={turnstileEnabled}
        turnstileSiteKey={turnstileSiteKey}
        setTurnstileToken={setTurnstileToken}
      />

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

export default PersonalSetting;
