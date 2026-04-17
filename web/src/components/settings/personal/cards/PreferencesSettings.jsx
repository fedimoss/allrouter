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
import { Card } from '@douyinfe/semi-ui';
import {
  CheckCircle2,
  Languages,
  MonitorSmartphone,
  MoonStar,
  SunMedium,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, showSuccess, showError } from '../../../../helpers';
import { UserContext } from '../../../../context/User';
import {
  useActualTheme,
  useSetTheme,
  useTheme,
} from '../../../../context/Theme';
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
    description: t('明亮、清爽，适合白天和高亮环境使用'),
    icon: SunMedium,
  },
  {
    value: 'dark',
    label: t('深色'),
    description: t('更聚焦内容，减少夜间浏览时的视觉刺激'),
    icon: MoonStar,
  },
  {
    value: 'auto',
    label: t('跟随系统'),
    description: `${t('自动匹配系统外观')} · ${
      actualTheme === 'dark' ? t('当前为深色') : t('当前为浅色')
    }`,
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
    <section className='personal-v3-section'>
      <Card
        className='personal-v3-card personal-v3-preference-card !rounded-[24px]'
        bodyStyle={{ padding: 0 }}
      >
        <div className='personal-v3-card-body'>
          <div className='personal-v3-card-title personal-v3-card-title-compact'>
            <span className='personal-v3-icon-badge'>
              <Languages size={18} />
            </span>
            <div>
              <h3>{t('界面偏好')}</h3>
              <p>{t('自定义主题模式和界面语言，兼容浅色、深色与跟随系统。')}</p>
            </div>
          </div>

          <div className='personal-v3-preference-grid'>
            <div className='personal-v3-preference-block'>
              <div className='personal-v3-preference-head'>
                <h4>{t('主题模式')}</h4>
                <p>{t('根据使用场景自由切换外观，立即生效')}</p>
              </div>

              <div className='personal-v3-theme-grid'>
                {themeOptions.map((option) => {
                  const Icon = option.icon;
                  return (
                    <button
                      key={option.value}
                      type='button'
                      className={`personal-v3-theme-item ${
                        theme === option.value ? 'is-active' : ''
                      }`}
                      onClick={() => handleThemePreferenceChange(option.value)}
                    >
                      <span className='personal-v3-theme-icon'>
                        <Icon size={18} />
                      </span>
                      <span className='personal-v3-theme-title'>
                        {option.label}
                      </span>
                      <span className='personal-v3-theme-desc'>
                        {option.description}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>

            <div className='personal-v3-preference-block'>
              <div className='personal-v3-preference-head'>
                <h4>{t('语言设置')}</h4>
                <p>{t('选择界面语言，设置会同步保存到当前账户')}</p>
              </div>

              <div className='personal-v3-language-list'>
                {languageOptions.map((option) => {
                  const active = currentLanguage === option.value;
                  return (
                    <button
                      key={option.value}
                      type='button'
                      disabled={loading}
                      className={`personal-v3-language-item ${
                        active ? 'is-active' : ''
                      }`}
                      onClick={() =>
                        handleLanguagePreferenceChange(option.value)
                      }
                    >
                      <span>{option.label}</span>
                      <span className='personal-v3-language-dot'>
                        {active ? <CheckCircle2 size={14} /> : null}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>
          </div>

          <div className='personal-v3-inline-summary'>
            <span>{t('当前主题')}：{themeOptions.find((item) => item.value === theme)?.label}</span>
            <span>{t('当前语言')}：{languageOptions.find((item) => item.value === currentLanguage)?.label}</span>
          </div>
        </div>
      </Card>
    </section>
  );
};

export default PreferencesSettings;
