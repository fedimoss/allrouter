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
import { Collapse,Empty } from '@douyinfe/semi-ui';
import {
  IllustrationConstruction,
  IllustrationConstructionDark,
} from '@douyinfe/semi-illustrations';
import {
  ILLUSTRATION_SIZE
} from '../../constants/dashboard.constants';
import { marked } from 'marked';
import cjwtIcon from '../../../public/board-cjwt.svg';

import './index.scss';

const FAQ_TAGS = ['全部', '接口', '计费', '安全'];

const FaqPanel = ({ faqData = [], t }) => {
  const displayFaqData = faqData

  return (
    <section className='custom-card dashboard-v2-bottom-card'>
      <div className='custom-card__header'>
        <div className='header-left'>
          <img src={cjwtIcon} alt="" />
          <span>{t('常见问题')}</span>
        </div>
        {/* <div className='header-extra'>{t('常见')}</div> */}
      </div>

      <div className='custom-card__tags'>
        {FAQ_TAGS.map((item, index) => (
          <div key={item} className={`tag ${index === 0 ? 'tag--active' : ''}`}>
            {t(item)}
          </div>
        ))}
      </div>
      <div className='flex1-content'>
        {
          displayFaqData.length > 0 ? (
            <Collapse accordion>
              {displayFaqData.map((item, index) => (
                <Collapse.Panel
                  key={index}
                  header={t(item.question || '--')}
                  itemKey={index.toString()}
                >
                  <div
                    dangerouslySetInnerHTML={{
                      __html: marked.parse(item.answer || '--'),
                    }}
                  />
                </Collapse.Panel>
              ))}
            </Collapse>
          ) : (
            <div className='flex justify-center items-center py-8'>
              <Empty
                image={<IllustrationConstruction style={ILLUSTRATION_SIZE} />}
                darkModeImage={
                  <IllustrationConstructionDark style={ILLUSTRATION_SIZE} />
                }
                title={t('暂无常见问答')}
                description={t('请联系管理员在系统设置中配置常见问答')}
              />
            </div>
          )}
      </div>
    </section>
  );
};

export default FaqPanel;
