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

import React, { useMemo } from 'react';
import {
  BellRing,
} from 'lucide-react';
import { marked } from 'marked';

import './index.scss';

const normalizeType = (item = {}) => {
  const raw = String(item?.type || item?.category || '').toLowerCase();
  if (raw.includes('important')) return { code: 'IMPORTANT', className: 'status-label--important', text: '重要' };
  if (raw.includes('alert') || raw.includes('warning')) return { code: 'ALERT', className: 'status-label--alert', text: '告警' };
  if (raw.includes('bulletin')) return { code: 'BULLETIN', className: 'status-label--bulletin', text: '公告' };
  if (raw.includes('notice')) return { code: 'NOTICE', className: 'status-label--notice', text: '通知' };
  return { code: 'NOTICE', className: 'status-label--notice', text: '通知' };
};

const plainText = (value) =>
  String(value || '')
    .replace(/<[^>]+>/g, '')
    .replace(/\s+/g, ' ')
    .trim();

const AnnouncementsPanel = ({ announcementData = [],announcementLegendData, t }) => {
  const displayList = useMemo(() => {
    return announcementData.slice(0, 20).map((item, index) => {
      const typeInfo = normalizeType(item);
      const content = plainText(item.content || item.title || '');
      const extra = plainText(item.extra || '');
      const relative = plainText(item.relative || '');
      const time = plainText(item.time || item.publishDate || '');
      const metaParts = [];
      if (relative || time) {
        metaParts.push(`发布于：${relative ? `${relative} ` : ''}${time}`.trim());
      }
      if (typeInfo.text) {
        metaParts.push(`分类：${typeInfo.text}`);
      }

      return {
        id: item.id || `announcement-${index}`,
        title: content || t('暂无系统公告'),
        meta: metaParts.join(' · '),
        code: typeInfo.code,
        className: typeInfo.className,
        extra,
      };
    });
  }, [announcementData, t]);

  return (
    <section className='custom-card lg:col-span-2'>
      <div className='custom-card__header'>
        <div className='header-left'>
          <BellRing style={{ color: '#3370ff' }} />
          <span>{t('系统公告')}</span>
        </div>
        <div className='header-extra'>{t('最新20条')}</div>
      </div>
      <div className="custom-card__tags">
          <div className="tag tag--active">全部</div>
          {announcementLegendData.map((legend) => (
            <div key={legend.label} className="tag ">
              {legend.label}
            </div>
          ))}
        </div>
      <div className='flex1-content'>
        {displayList.length > 0 ? (
          displayList.slice(0, 2).map((item) => (
            <div key={item.id} className='list-item'>
              <span className='list-item__title'>{item.title}</span>
              <div className='list-item__meta'>{item.meta}</div>
              {item.extra ? <div className='list-item__meta'>{item.extra}</div> : null}
              <span className={`status-label ${item.className}`}>{item.code}</span>
            </div>
          ))
        ) : (
          <div className="flex justify-center items-center w-full h-full">
            <Empty
              image={<IllustrationConstruction style={{ width: '90px', height: '90px' }} />}
              title={t('暂无系统公告')}
              description={t('请联系管理员在系统设置中配置公告信息')}
            />
          </div>
        )}
      </div>
      
    </section>
  );
};

export default AnnouncementsPanel;
