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

export const supportedLanguages = [
  'zh-CN',
  'zh-TW',
  'en',
  'fr',
  'ru',
  'ja',
  'vi',
];

// 根据常见时区推断默认语言，供个人设置里自动同步语言时使用。
const timezoneLanguageMap = Object.freeze({
  'Asia/Chongqing': 'zh-CN',
  'Asia/Harbin': 'zh-CN',
  'Asia/Shanghai': 'zh-CN',
  'Asia/Urumqi': 'zh-CN',
  'Asia/Hong_Kong': 'zh-TW',
  'Asia/Macau': 'zh-TW',
  'Asia/Taipei': 'zh-TW',
  'America/Anchorage': 'en',
  'America/Chicago': 'en',
  'America/Denver': 'en',
  'America/Los_Angeles': 'en',
  'America/New_York': 'en',
  'America/Phoenix': 'en',
  'Australia/Melbourne': 'en',
  'Australia/Perth': 'en',
  'Australia/Sydney': 'en',
  'Europe/London': 'en',
  'Pacific/Auckland': 'en',
  'Pacific/Honolulu': 'en',
  'Europe/Paris': 'fr',
  'Europe/Moscow': 'ru',
  'Asia/Tokyo': 'ja',
  'Asia/Ho_Chi_Minh': 'vi',
});

export const normalizeLanguage = (language) => {
  if (!language) {
    return language;
  }

  const normalized = language.trim().replace(/_/g, '-');
  const lower = normalized.toLowerCase();

  if (
    lower === 'zh' ||
    lower === 'zh-cn' ||
    lower === 'zh-sg' ||
    lower.startsWith('zh-hans')
  ) {
    return 'zh-CN';
  }

  if (
    lower === 'zh-tw' ||
    lower === 'zh-hk' ||
    lower === 'zh-mo' ||
    lower.startsWith('zh-hant')
  ) {
    return 'zh-TW';
  }

  const matchedLanguage = supportedLanguages.find(
    (supportedLanguage) => supportedLanguage.toLowerCase() === lower,
  );

  return matchedLanguage || normalized;
};

// 将时区映射结果统一走语言标准化，避免出现格式不一致的问题。
export const getLanguageByTimezone = (timezone) => {
  if (!timezone) {
    return '';
  }

  const matchedLanguage = timezoneLanguageMap[timezone.trim()];
  if (!matchedLanguage) {
    return '';
  }

  return normalizeLanguage(matchedLanguage);
};
