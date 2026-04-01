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

import React, { memo, useMemo, useCallback } from 'react';
import {
  ArrowUpRight,
  Flame,
  Gem,
  Link2,
  Rocket,
  Sparkles,
  Zap,
} from 'lucide-react';
import SearchActions from './SearchActions';
import pricingBannerImg from '../../../../../../public/pricing-banner.jpg'


const getRecommendationCards = (t) => [
  {
    key: 'hot',
    title: t('热门优选'),
    description: t('快速查看当前点击量和使用量最高的模型。'),
    tags: ['GPT-4-Turbo', 'Claude 3.5'],
    icon: Flame,
    accentIcon: ArrowUpRight,
    tone: 'warm',
  },
  {
    key: 'value',
    title: t('性价比首选'),
    description: t('按价格和倍率快速筛出更适合成本敏感场景的模型。'),
    tags: ['Haiku', 'Llama 3 8B'],
    icon: Gem,
    accentIcon: Link2,
    tone: 'mint',
  },
  {
    key: 'latest',
    title: t('最新上架'),
    description: t('优先查看最近新增或带有新标签的模型。'),
    tags: ['GPT-5-Codex', 'Gemini 1.5 Pro'],
    icon: Rocket,
    accentIcon: Zap,
    tone: 'blue',
  },
];

const PricingVendorIntro = memo(
  ({
    models = [],
    allModels = [],
    selectedRowKeys = [],
    copyText,
    handleChange,
    handleCompositionStart,
    handleCompositionEnd,
    isMobile = false,
    searchValue = '',
    setShowFilterModal,
    showWithRecharge,
    setShowWithRecharge,
    currency,
    setCurrency,
    siteDisplayType,
    showRatio,
    setShowRatio,
    viewMode,
    setViewMode,
    tokenUnit,
    setTokenUnit,
    sortMode,
    setSortMode,
    sidebarProps,
    t,
  }) => {
    const sourceModels = allModels.length > 0 ? allModels : models;
    const recommendationCards = useMemo(() => getRecommendationCards(t), [t]);

    const featuredModels = useMemo(
      () => [
        sourceModels.find((item) => item.tags?.toLowerCase().includes('hot')) ||
          sourceModels.find((item) => item.model_name?.toLowerCase().includes('gpt')) ||
          sourceModels[0],
        sourceModels.find((item) => item.tags?.toLowerCase().includes('value')) ||
          [...sourceModels]
            .filter((item) => item.quota_type === 0)
            .sort((a, b) => (a.model_ratio || 999) - (b.model_ratio || 999))[0] ||
          sourceModels[1],
        sourceModels.find((item) => item.tags?.toLowerCase().includes('new')) ||
          sourceModels.find((item) => item.model_name?.toLowerCase().includes('gemini')) ||
          sourceModels[2],
      ],
      [sourceModels],
    );

    const resetBaseFilters = useCallback(() => {
      sidebarProps?.setFilterVendor?.('all');
      sidebarProps?.setFilterGroup?.('all');
      sidebarProps?.setSelectedGroup?.('all');
      sidebarProps?.setFilterEndpointType?.('all');
    }, [sidebarProps]);

    const handleRecommendationClick = useCallback(
      (key) => {
        resetBaseFilters();

        if (key === 'hot') {
          sidebarProps?.setFilterQuotaType?.('all');
          sidebarProps?.setFilterTag?.('hot');
          setSortMode?.('hot');
        } else if (key === 'value') {
          sidebarProps?.setFilterTag?.('all');
          sidebarProps?.setFilterQuotaType?.(0);
          setSortMode?.('value');
        } else if (key === 'latest') {
          sidebarProps?.setFilterQuotaType?.('all');
          sidebarProps?.setFilterTag?.('new');
          setSortMode?.('latest');
        }

        sidebarProps?.setCurrentPage?.(1);
      },
      [resetBaseFilters, setSortMode, sidebarProps],
    );

    return (
      <div className='pricing-market-top-shell'>
        {!isMobile && (
          <div className='pricing-market-recommend-section' style={{backgroundImage: `url(${pricingBannerImg})`, backgroundSize: 'cover', backgroundPosition: 'center'}}>
            <div className='pricing-banner-cont'>
              <div className='pricing-banner-cont-title'>GPT-5.4 & Gemini 3.1 Pro</div>
              <div className='pricing-banner-cont-title color'>{t('现已全面接入 AllRouter')}</div>
              <div className='pricing-banner-cont-description'>
                {t('更快的响应速度，更低的网络延迟，通过 AllRouter 智能路由引擎，自动为您选择最优渠道。')}
              </div>
              <div className='pricing-banner-cont-action'>
                <div className='pricing-banner-cont-action-btn'>{t('立即调用')}</div>
                <div className='pricing-banner-cont-action-btn'>{t('查看文档')}</div>
              </div>
            </div>
          </div>
        )}

        <div className='pricing-market-toolbar-shell'>
          <SearchActions
            selectedRowKeys={selectedRowKeys}
            copyText={copyText}
            handleChange={handleChange}
            handleCompositionStart={handleCompositionStart}
            handleCompositionEnd={handleCompositionEnd}
            isMobile={isMobile}
            searchValue={searchValue}
            setShowFilterModal={setShowFilterModal}
            showWithRecharge={showWithRecharge}
            setShowWithRecharge={setShowWithRecharge}
            currency={currency}
            setCurrency={setCurrency}
            siteDisplayType={siteDisplayType}
            showRatio={showRatio}
            setShowRatio={setShowRatio}
            viewMode={viewMode}
            setViewMode={setViewMode}
            tokenUnit={tokenUnit}
            setTokenUnit={setTokenUnit}
            filteredCount={models.length}
            sortMode={sortMode}
            setSortMode={setSortMode}
            t={t}
          />
        </div>
      </div>
    );
  },
);

PricingVendorIntro.displayName = 'PricingVendorIntro';

export default PricingVendorIntro;
