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

import React, { useContext } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { StatusContext } from '../../context/Status';
import { getLogo, getSystemName, withBrowserBaseUrl } from '../../helpers';
import brandLogo from '../../../public/theme/theme3/allrouter-logo.svg';
import './SiteFooter.css';

/**
 * 全站页脚（从首页主题三抽出的公共组件）。
 * 数据自取：logo/站名来自 helpers，docs_link / footer_html 来自 StatusContext。
 * 所有页面统一在 PageLayout 渲染；CTA 大卡片（footer-cta）仅首页显示，
 * 通过 showCta 控制。
 */
const SiteFooter = ({ showCta = false }) => {
  const { t, i18n } = useTranslation();
  const [statusState] = useContext(StatusContext);

  const logo = getLogo() || brandLogo;
  const name = getSystemName() || 'AllRouter.AI';

  const docsLink = statusState?.status?.docs_link || '';
  const docsLangPrefix = i18n.language.startsWith('zh') ? 'zh' : 'en';
  const docsHref = docsLink || withBrowserBaseUrl(`/${docsLangPrefix}/docs`);
  const apiReferenceHref = withBrowserBaseUrl(`/${docsLangPrefix}/docs/api`);
  const communityHref = withBrowserBaseUrl(
    `/${docsLangPrefix}/docs/support/community-interaction`,
  );
  const footerHtml = statusState?.status?.footer_html;

  return (
    <footer className='app-site-footer' aria-label='页脚'>
      {showCta && (
        <div className='footer-cta'>
          <h2>{t('准备好优化您的 AI 工作流了吗？')}</h2>
          <p>{t('加入 2,000+ 开发者，开始享受更稳定、更廉价的大模型服务。')}</p>
          <Link className='footer-cta__btn' to='/console'>
            {t('免费开始构建')}
          </Link>
        </div>
      )}
      <div className='footer-main'>
        <div className='footer-brand'>
          <Link className='brand-lockup' to='/' aria-label={`${name} 首页`}>
            <img src={logo} alt='' width='31' height='31' />
            <span>{name}</span>
          </Link>
          <p>{t('统一 AI 接入网关，为团队提供模型接入、路由、计费与治理能力。')}</p>
        </div>
        <nav className='footer-col' aria-label={t('产品')}>
          <b>{t('产品')}</b>
          <a href='#features'>{t('功能特性')}</a>
          <a href='#models'>{t('模型生态')}</a>
          <Link to='/pricing'>{t('定价')}</Link>
          <a
            href='https://github.com/fedimoss/allrouter/releases'
            target='_blank'
            rel='noreferrer'
          >
            {t('更新日志')}
          </a>
        </nav>
        <nav className='footer-col' aria-label={t('资源')}>
          <b>{t('资源')}</b>
          <a href={docsHref} target='_blank' rel='noreferrer'>
            {t('文档')}
          </a>
          <a href={apiReferenceHref} target='_blank' rel='noreferrer'>
            {t('API 参考')}
          </a>
          <a href={communityHref} target='_blank' rel='noreferrer'>
            {t('社区')}
          </a>
          <a
            href={`https://status.${name.toLowerCase()}/`}
            target='_blank'
            rel='noreferrer'
          >
            {t('系统状态')}
          </a>
        </nav>
        <nav className='footer-col' aria-label={t('帮助中心')}>
          <b>{t('帮助中心')}</b>
          <Link to='/about'>{t('关于平台')}</Link>
          <a
            href='https://github.com/fedimoss/allrouter'
            target='_blank'
            rel='noreferrer'
          >
            {t('项目仓库')}
          </a>
          <a
            href='https://github.com/fedimoss/allrouter/issues'
            target='_blank'
            rel='noreferrer'
          >
            {t('问题反馈')}
          </a>
          <a href={`mailto:support@${name.toLowerCase()}`}>{t('联系我们')}</a>
        </nav>
      </div>
      <div
        className='footer-bottom'
        dangerouslySetInnerHTML={{
          __html:
            footerHtml ||
            `© ${new Date().getFullYear()} ${name}. All rights reserved.`,
        }}
      />
    </footer>
  );
};

export default SiteFooter;
