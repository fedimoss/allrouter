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

export const NOTICE_CLOSE_DATE_KEY = 'notice_close_date';

export function hasNoticeContent(notice) {
  return typeof notice === 'string' && notice.trim() !== '';
}

export function hasAnnouncements(announcements) {
  return Array.isArray(announcements) && announcements.length > 0;
}

export function shouldShowNotice(notice, announcements) {
  return hasNoticeContent(notice) || hasAnnouncements(announcements);
}

export function getNoticeDefaultTab(notice, announcements) {
  if (hasNoticeContent(notice)) {
    return 'inApp';
  }
  return hasAnnouncements(announcements) ? 'system' : 'inApp';
}

export function isNoticeClosedToday() {
  return (
    localStorage.getItem(NOTICE_CLOSE_DATE_KEY) === new Date().toDateString()
  );
}

export function markNoticeClosedToday() {
  localStorage.setItem(NOTICE_CLOSE_DATE_KEY, new Date().toDateString());
}
