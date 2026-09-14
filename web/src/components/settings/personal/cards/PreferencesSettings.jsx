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

import React, { useContext, useEffect, useMemo, useState } from 'react';
import {
  CheckCircle2,
  MonitorSmartphone,
  MoonStar,
  SunMedium,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, showSuccess, showError } from '../../../../helpers';
import { UserContext } from '../../../../context/User';
import { useActualTheme, useSetTheme, useTheme } from '../../../../context/Theme';
import { normalizeLanguage } from '../../../../i18n/language';

export const languageOptions = [
  { value: 'zh-CN', label: '简体中文' },
  { value: 'zh-TW', label: '繁體中文' },
  { value: 'en', label: 'English' },
  { value: 'fr', label: 'Français' },
  { value: 'ru', label: 'Русский' },
  { value: 'ja', label: '日本語' },
  { value: 'vi', label: 'Tiếng Việt' },
];

const themeOptionFactory = (t, actualTheme) => [
  {
    value: 'light',
    label: t('浅色'),
    icon: SunMedium,
  },
  {
    value: 'dark',
    label: t('深色'),
    icon: MoonStar,
  },
  {
    value: 'auto',
    label: t('跟随系统'),
    icon: MonitorSmartphone,
  },
];

const PreferencesSettings = ({ t }) => {
  const { i18n } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const theme = useTheme();
  const actualTheme = useActualTheme();
  const setTheme = useSetTheme();
  const [currentLanguage, setCurrentLanguage] = useState(
    normalizeLanguage(i18n.language) || 'zh-CN',
  );
  const [loading, setLoading] = useState(false);

  const themeOptions = useMemo(
    () => themeOptionFactory(t, actualTheme),
    [actualTheme, t],
  );

  useEffect(() => {
    if (userState?.user?.setting) {
      try {
        const settings = JSON.parse(userState.user.setting);
        if (settings.language) {
          const lang = normalizeLanguage(settings.language);
          setCurrentLanguage(lang);
          if (i18n.language !== lang) {
            i18n.changeLanguage(lang);
          }
        }
      } catch {
        // ignore parse errors
      }
    }
  }, [userState?.user?.setting, i18n]);

  const handleLanguagePreferenceChange = async (lang) => {
    if (lang === currentLanguage) return;

    setLoading(true);
    const previousLang = currentLanguage;

    try {
      setCurrentLanguage(lang);
      i18n.changeLanguage(lang);
      localStorage.setItem('i18nextLng', lang);

      const res = await API.put('/api/user/self', {
        language: lang,
      });

      if (res.data.success) {
        showSuccess(t('语言偏好已保存'));
        let settings = {};
        if (userState?.user?.setting) {
          try {
            settings = JSON.parse(userState.user.setting) || {};
          } catch {
            settings = {};
          }
        }
        settings.language = lang;
        const nextUser = {
          ...userState.user,
          setting: JSON.stringify(settings),
        };
        userDispatch({
          type: 'login',
          payload: nextUser,
        });
        localStorage.setItem('user', JSON.stringify(nextUser));
      } else {
        showError(res.data.message || t('保存失败'));
        setCurrentLanguage(previousLang);
        i18n.changeLanguage(previousLang);
        localStorage.setItem('i18nextLng', previousLang);
      }
    } catch {
      showError(t('保存失败，请重试'));
      setCurrentLanguage(previousLang);
      i18n.changeLanguage(previousLang);
      localStorage.setItem('i18nextLng', previousLang);
    } finally {
      setLoading(false);
    }
  };

  const handleThemePreferenceChange = (value) => {
    if (value === theme) {
      return;
    }
    setTheme(value);
  };

  return (
    <section className='ps-settings-card'>
      <div className='ps-settings-card-head'>
        <div>
          <h2>{t('偏好设置')}</h2>
          <p className='ps-settings-card-sub'>
            {t('自定义语言、主题等界面与行为偏好')}
          </p>
        </div>
      </div>

      {/* 主题偏好 */}
      <div className='ps-form-block'>
        <label>{t('主题偏好')}</label>
        <div className='ps-radio-row'>
          {themeOptions.map((option) => {
            const Icon = option.icon;
            return (
              <button
                key={option.value}
                type='button'
                className={`ps-radio-chip ${
                  theme === option.value ? 'is-active' : ''
                }`}
                onClick={() => handleThemePreferenceChange(option.value)}
              >
                <Icon size={15} />
                {option.label}
              </button>
            );
          })}
        </div>
      </div>

      {/* 语言偏好 */}
      <div className='ps-form-block' style={{ marginTop: 20 }}>
        <label>{t('语言偏好')}</label>
        <div className='ps-radio-row'>
          {languageOptions.map((option) => {
            const active = currentLanguage === option.value;
            return (
              <button
                key={option.value}
                type='button'
                disabled={loading}
                className={`ps-radio-chip ${active ? 'is-active' : ''}`}
                onClick={() =>
                  handleLanguagePreferenceChange(option.value)
                }
              >
                {active ? <CheckCircle2 size={14} /> : null}
                {option.label}
              </button>
            );
          })}
        </div>
      </div>

      <div className='ps-inline-summary'>
        <span>
          {t('当前主题')}：
          {themeOptions.find((item) => item.value === theme)?.label}
          {theme === 'auto'
            ? ` · ${actualTheme === 'dark' ? t('深色') : t('浅色')}`
            : ''}
        </span>
        <span>
          {t('当前语言')}：
          {languageOptions.find((item) => item.value === currentLanguage)
            ?.label || currentLanguage}
        </span>
      </div>
    </section>
  );
};

export default PreferencesSettings;
