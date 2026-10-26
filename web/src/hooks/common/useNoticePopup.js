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

import { useEffect, useRef, useState } from 'react';
import { fetchNotice } from '../../helpers/api';
import {
  getNoticeDefaultTab,
  isNoticeClosedToday,
  shouldShowNotice,
} from '../../helpers/notice';

export const useNoticePopup = (announcements = [], { enabled = true } = {}) => {
  const [noticeContent, setNoticeContent] = useState(null);
  const [noticeVisible, setNoticeVisible] = useState(false);
  const autoShownRef = useRef(false);

  useEffect(() => {
    let cancelled = false;

    const loadNotice = async () => {
      if (isNoticeClosedToday()) {
        setNoticeContent('');
        return;
      }

      try {
        const res = await fetchNotice();
        if (cancelled) {
          return;
        }
        const { success, message, data } = res.data;
        if (!success) {
          console.error('获取公告失败:', message);
          setNoticeContent('');
          return;
        }
        setNoticeContent(data || '');
      } catch (error) {
        if (!cancelled) {
          console.error('获取公告失败:', error);
          setNoticeContent('');
        }
      }
    };

    loadNotice();

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!enabled || noticeContent === null || autoShownRef.current) {
      return;
    }
    if (
      isNoticeClosedToday() ||
      !shouldShowNotice(noticeContent, announcements)
    ) {
      return;
    }
    autoShownRef.current = true;
    setNoticeVisible(true);
  }, [announcements, enabled, noticeContent]);

  return {
    noticeVisible,
    setNoticeVisible,
    noticeDefaultTab: getNoticeDefaultTab(noticeContent || '', announcements),
  };
};
