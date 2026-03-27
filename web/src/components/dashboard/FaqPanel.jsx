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

import React from 'react';
import { Empty, Collapse } from '@douyinfe/semi-ui';
import { IconHelpCircle } from '@douyinfe/semi-icons';
import {
  IllustrationConstruction,
  IllustrationConstructionDark,
} from '@douyinfe/semi-illustrations';
import { marked } from 'marked';

import './index.scss';

const FAQ_TAGS = ['接口', '计费', '安全'];

const FaqPanel = ({ faqData = [], ILLUSTRATION_SIZE, t }) => {
  return (
    <section className='custom-card'>
      <div className='custom-card__header'>
        <div className='header-left'>
          <IconHelpCircle style={{ color: 'var(--semi-color-primary)', fontSize: '24px' }} />
          <span>{t('常见问答')}</span>
        </div>
        <div className='header-extra'>{t('分类')}</div>
      </div>

      <div className='custom-card__tags'>
        <div className='tag tag--active'>{t('全部')}</div>
        {FAQ_TAGS.map((item) => (
          <div key={item} className='tag'>
            {t(item)}
          </div>
        ))}
      </div>
      <div className='flex1-content'>
        {faqData.length > 0 ? (
          <Collapse>
            {faqData.map((item, index) => (
              <Collapse.Panel
                key={index}
                header={item.question}
                itemKey={index.toString()}
              >
                <div
                  dangerouslySetInnerHTML={{
                    __html: marked.parse(item.answer || ''),
                  }}
                />
              </Collapse.Panel>
            ))}
          </Collapse>
        ) : (
          <Empty
            image={<IllustrationConstruction style={ILLUSTRATION_SIZE} />}
            darkModeImage={
              <IllustrationConstructionDark style={ILLUSTRATION_SIZE} />
            }
            title={t('暂无常见问答')}
            description={t('请联系管理员在系统设置中配置常见问答')}
          />
        )}
      </div>
    </section>
  );
};

export default FaqPanel;
