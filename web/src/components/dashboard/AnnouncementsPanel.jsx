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
import { Empty } from '@douyinfe/semi-ui';
import { BellRing } from 'lucide-react';
import { IllustrationConstruction } from '@douyinfe/semi-illustrations';

import './index.scss';

const normalizeType = (item = {}) => {
  const raw = String(item?.type || item?.category || '').toLowerCase();
  if (raw.includes('important')) {
    return {
      code: 'IMPORTANT',
      className: 'status-label--important',
      textKey: '\u91cd\u8981',
    };
  }
  if (raw.includes('alert') || raw.includes('warning')) {
    return {
      code: 'ALERT',
      className: 'status-label--alert',
      textKey: '\u544a\u8b66',
    };
  }
  if (raw.includes('bulletin')) {
    return {
      code: 'BULLETIN',
      className: 'status-label--bulletin',
      textKey: '\u516c\u544a',
    };
  }
  return {
    code: 'NOTICE',
    className: 'status-label--notice',
    textKey: '\u901a\u77e5',
  };
};

const plainText = (value) =>
  String(value || '')
    .replace(/<[^>]+>/g, '')
    .replace(/\s+/g, ' ')
    .trim();

const AnnouncementsPanel = ({
  announcementData = [],
  announcementLegendData = [],
  t,
}) => {
  const displayList = useMemo(() => {
    return announcementData.slice(0, 20).map((item, index) => {
      const typeInfo = normalizeType(item);
      const content = plainText(item.content || item.title || '');
      const extra = plainText(item.extra || '');
      const relative = plainText(item.relative || '');
      const time = plainText(item.time || item.publishDate || '');
      const metaParts = [];

      if (relative || time) {
        metaParts.push(
          `${t('\u53d1\u5e03\u4e8e\uff1a')}${relative ? `${relative} ` : ''}${time}`.trim(),
        );
      }
      metaParts.push(`${t('\u5206\u7c7b\uff1a')}${t(typeInfo.textKey)}`);

      return {
        id: item.id || `announcement-${index}`,
        title: content || t('\u6682\u65e0\u7cfb\u7edf\u516c\u544a'),
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
          <BellRing style={{ color: 'var(--semi-color-primary)' }} />
          <span>{t('\u7cfb\u7edf\u516c\u544a')}</span>
        </div>
        <div className='header-extra'>{t('\u6700\u65b020\u6761')}</div>
      </div>
      <div className='custom-card__tags'>
        <div className='tag tag--active'>{t('\u5168\u90e8')}</div>
        {announcementLegendData.map((legend) => (
          <div key={legend.label} className='tag'>
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
              {item.extra ? (
                <div className='list-item__meta'>{item.extra}</div>
              ) : null}
              <span className={`status-label ${item.className}`}>
                {item.code}
              </span>
            </div>
          ))
        ) : (
          <div className='flex h-full w-full items-center justify-center'>
            <Empty
              image={
                <IllustrationConstruction
                  style={{ width: '90px', height: '90px' }}
                />
              }
              title={t('\u6682\u65e0\u7cfb\u7edf\u516c\u544a')}
              description={t('\u8bf7\u8054\u7cfb\u7ba1\u7406\u5458\u5728\u7cfb\u7edf\u8bbe\u7f6e\u4e2d\u914d\u7f6e\u516c\u544a\u4fe1\u606f')}
            />
          </div>
        )}
      </div>
    </section>
  );
};

export default AnnouncementsPanel;
