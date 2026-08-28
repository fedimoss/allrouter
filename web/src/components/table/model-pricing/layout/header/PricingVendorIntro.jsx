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
import { Link } from 'react-router-dom';
import SearchActions from './SearchActions';
import { getLobeHubIcon, getSystemName } from '@/helpers';

const systemName = getSystemName() || '';

const PricingVendorIntro = memo(
  ({
    models = [],
    allModels = [],
    filterVendor = 'all',
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

    const resetBaseFilters = useCallback(() => {
      sidebarProps?.setFilterVendor?.('all');
      sidebarProps?.setFilterGroup?.('all');
      sidebarProps?.setSelectedGroup?.('all');
      sidebarProps?.setFilterEndpointType?.('all');
    }, [sidebarProps]);

    const toDocs = useCallback(() => {
      window.open('/docs', '_blank');
    }, []);

    // 供应商胶囊数据：与静态页 provider-tab 一致（图标 + 名称 + 数量）
    const providerItems = useMemo(() => {
      const vendors = new Map();
      (sourceModels || []).forEach((model) => {
        const name = model.vendor_name;
        if (!name) return;
        if (!vendors.has(name)) {
          vendors.set(name, { count: 0, icon: model.vendor_icon || model.icon || '' });
        }
        vendors.get(name).count += 1;
      });
      return [
        { value: 'all', label: t('全部'), count: sourceModels.length, icon: '' },
        ...Array.from(vendors.entries())
          .sort((a, b) => a[0].localeCompare(b[0]))
          .map(([name, info]) => ({ value: name, label: name, count: info.count, icon: info.icon })),
      ];
    }, [sourceModels, t]);

    const handleProviderClick = useCallback(
      (value) => {
        sidebarProps?.setFilterVendor?.(value);
        sidebarProps?.setCurrentPage?.(1);
      },
      [sidebarProps],
    );

    const renderProviderIcon = (item) => {
      if (!item.icon) return null;
      return <span className='llm-provider-tab__icon'>{getLobeHubIcon(item.icon, 16)}</span>;
    };

    return (
      <div className='pricing-market-top-shell'>
        {!isMobile && (
          <>
            {/* 顶部 Banner：静态页 LLM.html 的流光样式（品牌双色湍流光斑） */}
            <section className='llm-hero-banner'>
              <div className='llm-hero-flow' aria-hidden='true'>
                <div className='llm-flow-blob llm-flow-blob-1' />
                <div className='llm-flow-blob llm-flow-blob-2' />
                <div className='llm-flow-blob llm-flow-blob-3' />
              </div>
              <h1>
                {t('一个接口')}
                <br />
                {t('接入全球')} <em>{t('主流大模型')}</em>
              </h1>
              <p>
                {t('Claude、DeepSeek、Gemini、GPT 等顶级模型统一封装，透明计价，按量付费，一行代码完成调用。')}
              </p>
              <div className='llm-banner-actions'>
                <Link className='llm-btn-primary' to='/console/token'>
                  {t('获取 API 密钥')}
                </Link>
                <button type='button' className='llm-btn-ghost' onClick={toDocs}>
                  {t('查看文档')}
                </button>
              </div>
            </section>

            {/* 供应商切换：横向胶囊 tabs */}
            <section className='llm-provider-bar' aria-label={t('按供应商筛选')}>
              {providerItems.map((item) => (
                <button
                  key={item.value}
                  type='button'
                  className={
                    filterVendor === item.value
                      ? 'llm-provider-tab active'
                      : 'llm-provider-tab'
                  }
                  onClick={() => handleProviderClick(item.value)}
                >
                  {renderProviderIcon(item)}
                  <span>{item.label}</span>
                  <span className='count'>{item.count}</span>
                </button>
              ))}
            </section>

            {/* 列表 meta：数量 + 更新时间 */}
            <div className='llm-list-meta'>
              <b>
                {t('共 {{count}} 个模型', { count: models.length })}
              </b>
              {/* <span>{systemName}</span> */}
            </div>
          </>
        )}

        {/* 搜索/工具栏整行暂时隐藏（需求：先注释掉；含排序、token 单位、视图切换） */}
        {/* <div className='pricing-market-toolbar-shell'>
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
        </div> */}
      </div>
    );
  },
);

PricingVendorIntro.displayName = 'PricingVendorIntro';

export default PricingVendorIntro;
