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
  userRawQuotaToDisplay,
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
  Input,
  Modal,
  Select,
  Upload,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import {
  BarChart3,
  Bell,
  CalendarCheck,
  Camera,
  Fingerprint,
  Link2,
  Mail,
  Settings2,
  ShieldCheck,
  TriangleAlert,
  UserRound,
  Wallet,
} from 'lucide-react';
import { getLogo } from '../../helpers';
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
import { getLanguageByTimezone, normalizeLanguage } from '../../i18n/language';
import defaultAvatar from '../../../public/avatar.svg';
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
const logo = getLogo();

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
    avatar: '',
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
  });

  const currentUser = userState?.user || {};
  const runtimeDevice = useMemo(() => detectRuntimeDevice(), []);

  // 右侧设置导航锚点 + 滚动高亮
  const sectionRefs = {
    account: useRef(null),
    security: useRef(null),
    checkin: useRef(null),
    notification: useRef(null),
    preferences: useRef(null),
    danger: useRef(null),
  };
  const [activeSection, setActiveSection] = useState('account');

  useEffect(() => {
    const handleScroll = () => {
      // 以视口坐标判定当前所在区块，兼容任意滚动容器
      const threshold = 140;
      let current = 'account';
      Object.keys(sectionRefs).forEach((key) => {
        const el = sectionRefs[key]?.current;
        if (el && el.getBoundingClientRect().top <= threshold) {
          current = key;
        }
      });
      setActiveSection(current);
    };
    // scroll 事件不冒泡，用捕获阶段监听所有滚动容器（含控制台内部滚动区）
    document.addEventListener('scroll', handleScroll, true);
    window.addEventListener('resize', handleScroll);
    handleScroll();
    return () => {
      document.removeEventListener('scroll', handleScroll, true);
      window.removeEventListener('resize', handleScroll);
    };
  }, []);

  const settingsNavItems = useMemo(() => {
    const items = [
      { key: 'account', label: t('账户管理'), icon: UserRound },
      { key: 'security', label: t('安全设置'), icon: ShieldCheck },
    ];
    if (status?.checkin_enabled) {
      items.push({
        key: 'checkin',
        label: t('签到日历'),
        icon: CalendarCheck,
      });
    }
    items.push(
      { key: 'notification', label: t('通知设置'), icon: Bell },
      { key: 'preferences', label: t('偏好设置'), icon: Settings2 },
      { key: 'danger', label: t('危险区域'), icon: TriangleAlert },
    );
    return items;
  }, [status?.checkin_enabled, t]);

  const scrollToSection = (key) => {
    sectionRefs[key]?.current?.scrollIntoView({
      behavior: 'smooth',
      block: 'start',
    });
  };

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
        value: userRawQuotaToDisplay(currentUser?.quota, currentUser),
        icon: Wallet,
      },
      {
        key: 'used',
        label: t('历史消耗'),
        value: userRawQuotaToDisplay(currentUser?.used_quota, currentUser),
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
      currentUser?.display_rate,
      currentUser?.display_symbol,
      t,
    ],
  );

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
      });
    }
  }, [currentUser?.setting]);

  // users.timezone 是个人资料的唯一时区来源；历史空值用户保持下拉框为空。
  useEffect(() => {
    const settings = safeParseSetting(currentUser?.setting);
    setProfileInputs((prev) => ({
      username: currentUser?.username || prev.username || '',
      avatar: currentUser?.avatar || defaultAvatar,
      phone_country_code:
        currentUser?.phone_country_code ||
        prev.phone_country_code ||
        settings.phone_country_code ||
        '+86',
      phone_number:
        currentUser?.phone_number || prev.phone_number || settings.phone_number || '',
      timezone: currentUser?.timezone || '',
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

  // 时区映射命中语言后，同时刷新运行时、缓存和用户上下文，保证界面立即一致。
  const syncLanguageLocally = (language) => {
    const normalizedLanguage = normalizeLanguage(language);
    if (!normalizedLanguage) {
      return;
    }

    i18n.changeLanguage(normalizedLanguage);
    localStorage.setItem('i18nextLng', normalizedLanguage);

    const settings = safeParseSetting(currentUser?.setting);
    settings.language = normalizedLanguage;
    const nextUser = {
      ...currentUser,
      setting: JSON.stringify(settings),
    };
    userDispatch({ type: 'login', payload: nextUser });
    setUserData(nextUser);
  };

  const saveProfile = async () => {
    const username = profileInputs.username.trim();
    const phoneNumber = profileInputs.phone_number.trim();
    const timezone = profileInputs.timezone || '';

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
        timezone,
        email: inputs.email || currentUser?.email || '',
      });
      const { success, message } = res.data;
      if (success) {
        const currentLanguage = normalizeLanguage(
          safeParseSetting(currentUser?.setting).language || i18n.language,
        );
        // 使用完整时区映射：除中英文外，还支持法语、俄语、日语和越南语等。
        const matchedLanguage = getLanguageByTimezone(timezone);
        let languageSyncError = '';

        if (matchedLanguage && matchedLanguage !== currentLanguage) {
          try {
            const languageRes = await API.put('/api/user/self', {
              language: matchedLanguage,
            });
            if (languageRes.data.success) {
              syncLanguageLocally(matchedLanguage);
            } else {
              languageSyncError =
                languageRes.data.message || t('保存失败，请重试');
            }
          } catch {
            languageSyncError = t('保存失败，请重试');
          }
        }

        showSuccess(t('账户信息已更新'));
        await getUserData();
        if (languageSyncError) {
          showError(languageSyncError);
        }
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
  const avatarUrl = profileInputs.avatar;
  const isCustomAvatar = avatarUrl && avatarUrl !== defaultAvatar;
  const [avatarImgLoaded, setAvatarImgLoaded] = useState(false);

  useEffect(() => {
    if (isCustomAvatar) {
      setAvatarImgLoaded(false);
      const img = new Image();
      img.onload = () => setAvatarImgLoaded(true);
      img.onerror = () => setAvatarImgLoaded(true);
      img.src = avatarUrl;
    } else {
      setAvatarImgLoaded(false);
    }
  }, [avatarUrl, isCustomAvatar]);

  const avatarSrc = isCustomAvatar && avatarImgLoaded ? avatarUrl : undefined;
  const localTimeLabel = useMemo(() => {
    if (typeof Intl === 'undefined' || !profileInputs.timezone) {
      return '-';
    }
    try {
      return new Intl.DateTimeFormat(undefined, {
        hour: '2-digit',
        minute: '2-digit',
        month: 'short',
        day: 'numeric',
        timeZone: profileInputs.timezone,
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
      <div className='ps-container'>
        {/* 页面头部 */}
        <section className='ps-page-head'>
          <h1>{t('个人设置')}</h1>
          <p className='sub'>{t('管理您的账户信息、安全设置与偏好')}</p>
        </section>

        {/* 用户信息头部卡片 */}
        <div className='ps-user-profile-card'>
          <Upload
            action='/'
            accept='image/*'
            showUploadList={false}
            uploadTrigger='auto'
            customRequest={handleAvatarUpload}
          >
            <Avatar
              size={64}
              shape='circle'
              hoverMask={hoverMask}
              src={avatarSrc}
              className='ps-user-profile-avatar'
            >
              {displayName?.[0]?.toUpperCase()}
            </Avatar>
          </Upload>
          <div className='ps-user-profile-info'>
            <div className='ps-user-profile-name'>{displayName}</div>
            <div className='ps-user-profile-meta'>
              <span>
                <Mail size={14} />
                {currentUser?.email || t('未绑定邮箱')}
              </span>
              <span>
                <Fingerprint size={14} />
                {t('用户 ID')}：{currentUser?.id ?? '-'}
              </span>
              <span>
                <ShieldCheck size={14} />
                {roleLabel}
              </span>
            </div>
          </div>
          <Button
            theme='outline'
            size='small'
            onClick={() => setShowEmailBindModal(true)}
          >
            {t('更换邮箱')}
          </Button>
        </div>

        {/* 账户统计 */}
        <div className='ps-stat-row'>
          {metricItems.map((item) => (
            <div key={item.key} className='ps-stat'>
              <div className='ps-stat-value'>{item.value}</div>
              <div className='ps-stat-label'>{item.label}</div>
            </div>
          ))}
        </div>

        <div className='ps-settings-layout'>
          <div className='ps-settings-content'>
            {/* 账户管理卡片 */}
            <section
              ref={sectionRefs.account}
              className='ps-settings-card ps-anchor'
            >
              <div className='ps-settings-card-head'>
                <div>
                  <h2>{t('账户管理')}</h2>
                  <p className='ps-settings-card-sub'>
                    {t('管理用户名、邮箱、密码与第三方绑定')}
                  </p>
                </div>
              </div>

              <div className='ps-account-form-grid'>
                <div className='ps-form-block'>
                  <label htmlFor='profile-username'>{t('用户名')}</label>
                  <Input
                    id='profile-username'
                    value={profileInputs.username}
                    onChange={(value) => handleProfileChange('username', value)}
                    placeholder={t('请输入用户名')}
                    showClear
                  />
                </div>

                <div className='ps-form-block'>
                  <label>{t('手机号')}</label>
                  <Input
                    value={profileInputs.phone_number}
                    addonBefore={
                      <Select
                        value={profileInputs.phone_country_code}
                        optionList={phoneCountryCodeOptions}
                        onChange={(value) =>
                          handleProfileChange('phone_country_code', value)
                        }
                        filter={selectFilter}
                        searchPosition='dropdown'
                        style={{ width: 132 }}
                      />
                    }
                    onChange={(value) =>
                      handleProfileChange('phone_number', value)
                    }
                    placeholder={t('请输入手机号')}
                    showClear
                  />
                </div>

                <div className='ps-form-block'>
                  <label htmlFor='profile-timezone'>{t('时区')}</label>
                  <Select
                    id='profile-timezone'
                    value={profileInputs.timezone}
                    optionList={timezoneOptions}
                    filter={selectFilter}
                    searchPosition='dropdown'
                    onChange={(value) => handleProfileChange('timezone', value)}
                    style={{ width: '100%' }}
                  />
                  <div className='ps-field-note'>
                    {profileInputs.timezone
                      ? `${profileInputs.timezone} · ${localTimeLabel}`
                      : t('选择时区后可同步本地时间')}
                  </div>
                </div>
              </div>

              <div className='ps-form-save-row'>
                <Button
                  type='primary'
                  onClick={saveProfile}
                  loading={profileSaving}
                >
                  {t('保存修改')}
                </Button>
              </div>

              <div className='ps-field-row'>
                <div className='ps-field-row-label'>
                  <div className='ps-field-title'>{t('用户ID')}</div>
                  <div className='ps-field-row-value ps-mono'>
                    {currentUser?.id ?? '-'}
                  </div>
                </div>
                <div className='ps-field-row-action'>
                  <Button
                    theme='outline'
                    size='small'
                    onClick={async () => {
                      await copy(String(currentUser?.id ?? ''));
                      showSuccess(t('用户ID已复制'));
                    }}
                  >
                    {t('复制')}
                  </Button>
                </div>
              </div>

              <div className='ps-field-row'>
                <div className='ps-field-row-label'>
                  <div className='ps-field-title'>{t('邮箱')}</div>
                  <div className='ps-field-desc'>
                    {currentUser?.email
                      ? `${t('已绑定邮箱')} ${currentUser.email}，${t(
                          '用于登录与通知',
                        )}`
                      : t('暂未绑定邮箱，可在绑定后用于登录与通知')}
                  </div>
                </div>
                <div className='ps-field-row-action'>
                  <Button
                    theme='outline'
                    size='small'
                    onClick={() => setShowEmailBindModal(true)}
                  >
                    {currentUser?.email ? t('更换邮箱') : t('绑定邮箱')}
                  </Button>
                </div>
              </div>

              <div className='ps-field-row'>
                <div className='ps-field-row-label'>
                  <div className='ps-field-title'>{t('密码')}</div>
                  <div className='ps-field-desc'>
                    {t('定期更改密码可以提高账户安全性')}
                  </div>
                </div>
                <div className='ps-field-row-action'>
                  <Button
                    theme='outline'
                    size='small'
                    onClick={() => setShowChangePasswordModal(true)}
                  >
                    {t('修改密码')}
                  </Button>
                </div>
              </div>

              <div className='ps-field-row'>
                <div className='ps-field-row-label'>
                  <div className='ps-field-title'>{t('微信绑定')}</div>
                  <div className='ps-field-desc'>
                    {t('绑定后可使用微信扫码登录')}
                  </div>
                </div>
                <div className='ps-field-row-value'>
                  {!status.wechat_login
                    ? t('未启用')
                    : currentUser?.wechat_id
                      ? t('已绑定')
                      : t('未绑定')}
                </div>
                <div className='ps-field-row-action'>
                  <Button
                    theme='outline'
                    size='small'
                    disabled={!status.wechat_login}
                    onClick={() => setShowWeChatBindModal(true)}
                  >
                    {currentUser?.wechat_id ? t('修改绑定') : t('绑定微信')}
                  </Button>
                </div>
              </div>
            </section>

            {/* 安全设置卡片 */}
            <div ref={sectionRefs.security} className='ps-anchor'>
              <AccountManagement
                t={t}
                userState={userState}
                status={status}
                systemToken={systemToken}
                runtimeDevice={runtimeDevice}
                generateAccessToken={generateAccessToken}
                handleSystemTokenClick={handleSystemTokenClick}
                passkeyStatus={passkeyStatus}
                passkeySupported={passkeySupported}
                passkeyRegisterLoading={passkeyRegisterLoading}
                passkeyDeleteLoading={passkeyDeleteLoading}
                onPasskeyRegister={handleRegisterPasskey}
                onPasskeyDelete={handleRemovePasskey}
                onTwoFAStatusChange={setTwoFAStatus}
              />
            </div>

            {/* 签到日历卡片 */}
            {status?.checkin_enabled && (
              <div ref={sectionRefs.checkin} className='ps-anchor'>
                <CheckinCalendar
                  t={t}
                  status={status}
                  turnstileEnabled={turnstileEnabled}
                  turnstileSiteKey={turnstileSiteKey}
                />
              </div>
            )}

            {/* 通知设置卡片 */}
            <div ref={sectionRefs.notification} className='ps-anchor'>
              <NotificationSettings
                t={t}
                notificationSettings={notificationSettings}
                handleNotificationSettingChange={handleNotificationSettingChange}
                saveNotificationSettings={saveNotificationSettings}
              />
            </div>

            {/* 偏好设置卡片 */}
            <div ref={sectionRefs.preferences} className='ps-anchor'>
              <PreferencesSettings t={t} />
            </div>

            {/* 危险区域卡片 */}
            <section
              ref={sectionRefs.danger}
              className='ps-settings-card ps-settings-card--danger ps-anchor'
            >
              <div className='ps-settings-card-head'>
                <div>
                  <h2>{t('危险区域')}</h2>
                  <p className='ps-settings-card-sub'>
                    {t('以下操作不可逆，请谨慎执行')}
                  </p>
                </div>
              </div>
              <div className='ps-field-row'>
                <div className='ps-field-row-label'>
                  <div className='ps-field-title ps-text-danger'>
                    {t('注销账户')}
                  </div>
                  <div className='ps-field-desc'>
                    {t(
                      '永久删除您的账户及所有关联数据，包括余额、令牌、调用记录等，此操作无法撤销。',
                    )}
                  </div>
                </div>
                <div className='ps-field-row-action'>
                  <Button
                    type='danger'
                    theme='solid'
                    size='small'
                    onClick={() => setShowAccountDeleteModal(true)}
                  >
                    {t('注销账户')}
                  </Button>
                </div>
              </div>
            </section>
          </div>

          {/* 桌面端设置导航 */}
          <nav className='ps-settings-nav'>
            <div className='ps-settings-nav-title'>{t('设置')}</div>
            {settingsNavItems.map((item) => {
              const Icon = item.icon;
              return (
                <a
                  key={item.key}
                  className={activeSection === item.key ? 'active' : ''}
                  onClick={() => scrollToSection(item.key)}
                >
                  <Icon size={15} />
                  {item.label}
                </a>
              );
            })}
          </nav>
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
