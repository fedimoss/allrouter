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

const FAQ_TAGS = ['\u63a5\u53e3', '\u8ba1\u8d39', '\u5b89\u5168'];

const FaqPanel = ({ faqData = [], ILLUSTRATION_SIZE, t }) => {
  return (
    <section className='custom-card'>
      <div className='custom-card__header'>
        <div className='header-left'>
          <IconHelpCircle style={{ color: 'var(--semi-color-primary)', fontSize: '24px' }} />
          <span>{t('\u5e38\u89c1\u95ee\u7b54')}</span>
        </div>
        <div className='header-extra'>{t('\u5206\u7c7b')}</div>
      </div>

      <div className='custom-card__tags'>
        <div className='tag tag--active'>{t('\u5168\u90e8')}</div>
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
            title={t('\u6682\u65e0\u5e38\u89c1\u95ee\u7b54')}
            description={t('\u8bf7\u8054\u7cfb\u7ba1\u7406\u5458\u5728\u7cfb\u7edf\u8bbe\u7f6e\u4e2d\u914d\u7f6e\u5e38\u89c1\u95ee\u7b54')}
          />
        )}
      </div>
    </section>
  );
};

export default FaqPanel;
