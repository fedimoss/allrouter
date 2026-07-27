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
import { ImagePreview } from '@douyinfe/semi-ui';
import { Link } from 'react-router-dom';
import PricingCardView from '../view/card/PricingCardView';
import ModelDetailSideSheet from '../modal/ModelDetailSideSheet';
import { useModelPricingData } from '../../../../hooks/model-pricing/useModelPricingData';
import {
  getLobeHubIcon,
  getLogo,
  getSystemName,
  withBrowserBaseUrl,
} from '../../../../helpers';
import './PricingShowcase.css';

const getVendorName = (model) => model?.vendor_name || 'unknown';

const ProviderIcon = ({ icon, label, logo, isAll }) => {
  if (isAll) {
    return (
      <img
        src={logo || '/logo.png'}
        alt=''
        onError={(event) => {
          event.currentTarget.onerror = null;
          event.currentTarget.src = '/logo.png';
        }}
      />
    );
  }

  if (icon) {
    return getLobeHubIcon(icon, 20);
  }

  return (
    <span className='pricing-showcase-provider-fallback' aria-hidden='true'>
      {String(label || '?')
        .slice(0, 1)
        .toUpperCase()}
    </span>
  );
};

const PricingFooter = ({ systemName, logo, docsHref, consoleTarget, t }) => {
  const currentYear = new Date().getFullYear();

  return (
    <footer className='pricing-showcase-footer' aria-label={t('页脚')}>
      <div className='pricing-showcase-footer-cta'>
        <h2>{t('准备好优化您的 AI 工作流了吗？')}</h2>
        <p>{t('加入 2,000+ 开发者，开始享受更稳定、更廉价的大模型服务。')}</p>
        <Link className='pricing-showcase-footer-cta-button' to={consoleTarget}>
          {t('免费开始构建')}
        </Link>
      </div>

      <div className='pricing-showcase-footer-main'>
        <div className='pricing-showcase-footer-brand'>
          <Link className='pricing-showcase-brand' to='/'>
            <img src={logo} alt='' />
            <span>{systemName}</span>
          </Link>
          <p>
            {t('统一 AI 接入网关，为团队提供模型接入、路由、计费与治理能力。')}
          </p>
        </div>

        <nav className='pricing-showcase-footer-column' aria-label={t('产品')}>
          <b>{t('产品')}</b>
          <Link to='/about'>{t('功能特性')}</Link>
          <Link to='/pricing'>{t('模型生态')}</Link>
          <Link to='/pricing'>{t('定价')}</Link>
          <Link to='/about'>{t('更新日志')}</Link>
        </nav>

        <nav className='pricing-showcase-footer-column' aria-label={t('资源')}>
          <b>{t('资源')}</b>
          <a href={docsHref} target='_blank' rel='noreferrer'>
            {t('文档')}
          </a>
          <a href={docsHref} target='_blank' rel='noreferrer'>
            {t('API 参考')}
          </a>
          <Link to='/about'>{t('社区')}</Link>
          <Link to='/about'>{t('系统状态')}</Link>
        </nav>

        <nav
          className='pricing-showcase-footer-column'
          aria-label={t('帮助中心')}
        >
          <b>{t('帮助中心')}</b>
          <Link to='/about'>{t('关于平台')}</Link>
          <a
            href='https://github.com/QuantumNous/new-api'
            target='_blank'
            rel='noreferrer'
          >
            {t('项目仓库')}
          </a>
          <Link to='/about'>{t('问题反馈')}</Link>
          <Link to='/about'>{t('联系我们')}</Link>
        </nav>
      </div>

      <div className='pricing-showcase-footer-bottom'>
        &copy; {currentYear} {systemName}. {t('版权所有')}
      </div>
    </footer>
  );
};

const PricingPage = () => {
  const pricingData = useModelPricingData();
  const { t } = pricingData;

  const systemName =
    pricingData.statusState?.status?.system_name || getSystemName() || '';
  const logo =
    pricingData.statusState?.status?.logo || getLogo() || '/logo.png';
  const currentLanguage = localStorage.getItem('i18nextLng') || 'zh-CN';
  const docsLanguage = currentLanguage.startsWith('zh') ? 'zh' : 'en';
  const docsHref =
    pricingData.statusState?.status?.docs_link ||
    withBrowserBaseUrl(`/${docsLanguage}/docs`);
  const consoleTarget = pricingData.userState?.user
    ? '/console/token'
    : '/login';

  const sortedModels = React.useMemo(() => {
    return [...(pricingData.filteredModels || [])].sort((a, b) => {
      const aHot = a.tags?.toLowerCase().includes('hot') ? 1 : 0;
      const bHot = b.tags?.toLowerCase().includes('hot') ? 1 : 0;
      if (aHot !== bHot) return bHot - aHot;
      return String(a.model_name || '').localeCompare(
        String(b.model_name || ''),
      );
    });
  }, [pricingData.filteredModels]);

  const providerItems = React.useMemo(() => {
    const providers = new Map();

    (pricingData.models || []).forEach((model) => {
      const value = getVendorName(model);
      const existing = providers.get(value);
      providers.set(value, {
        value,
        label: value === 'unknown' ? t('未知供应商') : value,
        icon: existing?.icon || model.vendor_icon || model.icon,
        count: (existing?.count || 0) + 1,
      });
    });

    const items = Array.from(providers.values()).sort((a, b) => {
      if (a.value === 'unknown') return 1;
      if (b.value === 'unknown') return -1;
      return a.label.localeCompare(b.label);
    });

    return [
      {
        value: 'all',
        label: t('全部'),
        icon: null,
        count: pricingData.models?.length || 0,
      },
      ...items,
    ];
  }, [pricingData.models, t]);

  const selectProvider = (value) => {
    pricingData.setFilterVendor(value);
    pricingData.setCurrentPage(1);
  };

  return (
    <div className='pricing-showcase'>
      <main className='pricing-showcase-main'>
        <section className='pricing-showcase-hero' aria-label={t('模型广场')}>
          <h1>
            {t('现已全面接入')} <em>{systemName}</em>
          </h1>
          <p>
            {t('更快的响应速度，更低的网络延迟，通过')} {systemName}{' '}
            {t('智能路由引擎，自动为您选择最优渠道。')}
          </p>
          <div className='pricing-showcase-hero-actions'>
            <Link
              className='pricing-showcase-button-primary'
              to={consoleTarget}
            >
              {t('立即调用')}
            </Link>
            <a
              className='pricing-showcase-button-secondary'
              href={docsHref}
              target='_blank'
              rel='noreferrer'
            >
              {t('查看文档')}
            </a>
          </div>
        </section>

        <div
          className='pricing-showcase-provider-bar'
          role='tablist'
          aria-label={t('模型供应商')}
        >
          {providerItems.map((provider) => {
            const isActive = pricingData.filterVendor === provider.value;
            return (
              <button
                key={provider.value}
                type='button'
                role='tab'
                aria-selected={isActive}
                className={`pricing-showcase-provider-tab${isActive ? ' is-active' : ''}`}
                onClick={() => selectProvider(provider.value)}
              >
                <span className='pricing-showcase-provider-icon'>
                  <ProviderIcon
                    icon={provider.icon}
                    label={provider.label}
                    logo={logo}
                    isAll={provider.value === 'all'}
                  />
                </span>
                <span>{provider.label}</span>
                <span className='pricing-showcase-provider-count'>
                  {provider.count}
                </span>
              </button>
            );
          })}
        </div>

        <div className='pricing-showcase-list-meta'>
          <b>{t('共 {{count}} 个模型', { count: sortedModels.length })}</b>
          <span>{t('热门优先')}</span>
        </div>

        <PricingCardView {...pricingData} filteredModels={sortedModels} />
      </main>

      <PricingFooter
        systemName={systemName}
        logo={logo}
        docsHref={docsHref}
        consoleTarget={consoleTarget}
        t={t}
      />

      <ImagePreview
        src={pricingData.modalImageUrl}
        visible={pricingData.isModalOpenurl}
        onVisibleChange={(visible) => pricingData.setIsModalOpenurl(visible)}
      />

      <ModelDetailSideSheet
        visible={pricingData.showModelDetail}
        onClose={pricingData.closeModelDetail}
        modelData={pricingData.selectedModel}
        groupRatio={pricingData.groupRatio}
        usableGroup={pricingData.usableGroup}
        currency={pricingData.currency}
        siteDisplayType={pricingData.siteDisplayType}
        tokenUnit={pricingData.tokenUnit}
        displayPrice={pricingData.displayPrice}
        showRatio={false}
        vendorsMap={pricingData.vendorsMap}
        endpointMap={pricingData.endpointMap}
        autoGroups={pricingData.autoGroups}
        t={t}
      />
    </div>
  );
};

export default PricingPage;
