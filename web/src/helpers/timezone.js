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

export const REGISTRATION_TIMEZONE_CHINA = 'Asia/Shanghai';
export const REGISTRATION_TIMEZONE_OVERSEAS = 'America/New_York';

export function getRegistrationTimezoneForLanguage(language) {
  const normalized = String(language || '')
    .trim()
    .replace(/_/g, '-')
    .toLowerCase();
  return normalized === 'zh' || normalized.startsWith('zh-')
    ? REGISTRATION_TIMEZONE_CHINA
    : REGISTRATION_TIMEZONE_OVERSEAS;
}

export function getRegistrationTimezone() {
  const browserLanguage =
    typeof navigator === 'undefined'
      ? ''
      : navigator.languages?.[0] || navigator.language;
  return getRegistrationTimezoneForLanguage(browserLanguage);
}
