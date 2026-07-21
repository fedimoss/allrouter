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

import { describe, expect, test } from 'bun:test';
import {
  getRegistrationTimezoneForLanguage,
  REGISTRATION_TIMEZONE_CHINA,
  REGISTRATION_TIMEZONE_OVERSEAS,
} from './timezone';

describe('getRegistrationTimezoneForLanguage', () => {
  // 简体、繁体及下划线写法都属于中文浏览器语言。
  test.each(['zh', 'zh-CN', 'zh-TW', 'zh-Hans', 'zh_Hant'])(
    '%s uses the China profile',
    (language) => {
      expect(getRegistrationTimezoneForLanguage(language)).toBe(
        REGISTRATION_TIMEZONE_CHINA,
      );
    },
  );

  // 其他语言和异常输入必须稳定回退到海外时区档位。
  test.each(['en', 'en-US', 'fr', 'ja', '', 'invalid'])(
    '%s uses the overseas profile',
    (language) => {
      expect(getRegistrationTimezoneForLanguage(language)).toBe(
        REGISTRATION_TIMEZONE_OVERSEAS,
      );
    },
  );
});
