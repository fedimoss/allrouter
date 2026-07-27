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
import { Avatar, Empty, Pagination } from '@douyinfe/semi-ui';
import { Check, Copy, Eye } from 'lucide-react';
import {
  calculateModelPrice,
  formatDynamicPriceCompactSummary,
  getLobeHubIcon,
} from '../../../../../helpers';
import { useMinimumLoadingTime } from '../../../../../hooks/common/useMinimumLoadingTime';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';

const getModelKey = (model) => model.key ?? model.model_name ?? model.id;

const estimateChannelCount = (model, usableGroup) => {
  if (!Array.isArray(model?.enable_groups)) return 0;

  const usableGroupNames = new Set(Object.keys(usableGroup || {}));
  if (usableGroupNames.size === 0) {
    return model.enable_groups.length;
  }

  return model.enable_groups.filter((group) => usableGroupNames.has(group))
    .length;
};

const buildPrimaryPriceItems = (priceData, t, quotaDisplayType) => {
  if (priceData?.isDynamicPricing) {
    return [
      {
        key: 'dynamic',
        label: t('动态计费'),
        value: '',
        suffix: '',
        isDynamic: true,
      },
    ];
  }

  if (priceData?.isPerToken) {
    if (quotaDisplayType === 'TOKENS' || priceData.isTokensDisplay) {
      return [
        {
          key: 'input-ratio',
          label: t('输入倍率'),
          value: priceData.inputRatio ?? '-',
          suffix: priceData.inputRatio == null ? '' : 'x',
        },
        {
          key: 'completion-ratio',
          label: t('补全倍率'),
          value: priceData.completionRatio ?? '-',
          suffix: priceData.completionRatio == null ? '' : 'x',
        },
        {
          key: 'cache-ratio',
          label: t('缓存读取倍率'),
          value: priceData.cacheRatio ?? '-',
          suffix: priceData.cacheRatio == null ? '' : 'x',
        },
      ];
    }

    const unitSuffix = ` / 1${priceData.unitLabel} Tokens`;
    return [
      {
        key: 'input',
        label: t('输入价格'),
        value: priceData.inputPrice ?? '-',
        suffix: priceData.inputPrice == null ? '' : unitSuffix,
      },
      {
        key: 'completion',
        label: t('补全价格'),
        value: priceData.completionPrice ?? '-',
        suffix: priceData.completionPrice == null ? '' : unitSuffix,
      },
      {
        key: 'cache',
        label: t('缓存读取价格'),
        value: priceData.cachePrice ?? '-',
        suffix: priceData.cachePrice == null ? '' : unitSuffix,
      },
    ];
  }

  return [
    {
      key: 'fixed',
      label: t('模型价格'),
      value: priceData?.price ?? '-',
      suffix: ` / ${t('次')}`,
    },
  ];
};

const getModelDescription = (model, language, t) => {
  if (model?.description_i18n) {
    try {
      const descriptions = JSON.parse(model.description_i18n);
      const shortLanguage = language.split('-')[0];
      const localized =
        descriptions[language] ||
        descriptions[shortLanguage] ||
        descriptions['zh-CN'] ||
        descriptions.en;
      if (localized) return localized;
    } catch {
      // Fall through to the plain description for malformed legacy data.
    }
  }

  if (model?.description) return t(model.description);
  return `${model?.vendor_name || t('通用')} ${t('最新模型，适合多轮对话、推理与生产环境调用。')}`;
};

const ModelIcon = ({ model }) => {
  if (model?.icon) {
    return getLobeHubIcon(model.icon, 26);
  }

  if (model?.vendor_icon) {
    return getLobeHubIcon(model.vendor_icon, 26);
  }

  return (
    <Avatar size='small'>
      {String(model?.model_name || '?')
        .slice(0, 1)
        .toUpperCase()}
    </Avatar>
  );
};

const PricingCardsSkeleton = () => (
  <div className='pricing-showcase-card-grid' aria-hidden='true'>
    {Array.from({ length: 6 }).map((_, index) => (
      <div
        className='pricing-showcase-card pricing-showcase-card-skeleton'
        key={index}
      >
        <div className='pricing-showcase-skeleton-head'>
          <span className='pricing-showcase-skeleton-circle' />
          <span className='pricing-showcase-skeleton-line is-title' />
          <span className='pricing-showcase-skeleton-pill' />
        </div>
        <span className='pricing-showcase-skeleton-line' />
        <span className='pricing-showcase-skeleton-line is-short' />
        <div className='pricing-showcase-skeleton-prices'>
          <span />
          <span />
          <span />
        </div>
      </div>
    ))}
  </div>
);

const PricingCardView = ({
  filteredModels,
  loading,
  pageSize,
  currentPage,
  setCurrentPage,
  selectedGroup,
  groupRatio,
  usableGroup,
  copyText,
  currency,
  siteDisplayType,
  tokenUnit,
  displayPrice,
  t,
  openModelDetail,
}) => {
  const showSkeleton = useMinimumLoadingTime(loading);
  const [copiedModel, setCopiedModel] = React.useState(null);
  const startIndex = (currentPage - 1) * pageSize;
  const paginatedModels = filteredModels.slice(
    startIndex,
    startIndex + pageSize,
  );
  const language = localStorage.getItem('i18nextLng') || 'zh-CN';

  if (showSkeleton) {
    return <PricingCardsSkeleton />;
  }

  if (!filteredModels || filteredModels.length === 0) {
    return (
      <div className='pricing-showcase-empty'>
        <Empty
          image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
          darkModeImage={
            <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
          }
          description={t('搜索无结果')}
        />
      </div>
    );
  }

  const handleCopy = async (modelName) => {
    await copyText(modelName);
    setCopiedModel(modelName);
    window.setTimeout(() => {
      setCopiedModel((current) => (current === modelName ? null : current));
    }, 1200);
  };

  return (
    <section className='pricing-showcase-model-list' aria-label={t('模型列表')}>
      <div className='pricing-showcase-card-grid'>
        {paginatedModels.map((model, index) => {
          const modelKey = getModelKey(model);
          const priceData = calculateModelPrice({
            record: model,
            selectedGroup,
            groupRatio,
            tokenUnit,
            displayPrice,
            currency,
            quotaDisplayType: siteDisplayType,
          });
          const priceItems = buildPrimaryPriceItems(
            priceData,
            t,
            siteDisplayType,
          );
          const isCopied = copiedModel === model.model_name;

          return (
            <article
              key={modelKey || index}
              className='pricing-showcase-card'
              style={{ animationDelay: `${Math.min(index, 8) * 45}ms` }}
              onClick={() => openModelDetail?.(model)}
            >
              <div className='pricing-showcase-card-head'>
                <span className='pricing-showcase-model-icon'>
                  <ModelIcon model={model} />
                </span>
                <h2>{model.model_name}</h2>
                <button
                  className={`pricing-showcase-copy-button${isCopied ? ' is-copied' : ''}`}
                  type='button'
                  aria-label={`${t('复制')} ${model.model_name}`}
                  title={t('复制')}
                  onClick={(event) => {
                    event.stopPropagation();
                    handleCopy(model.model_name);
                  }}
                >
                  {isCopied ? <Check size={14} /> : <Copy size={14} />}
                </button>
                <span className='pricing-showcase-channel-count'>
                  {estimateChannelCount(model, usableGroup)} {t('个渠道')}
                </span>
              </div>

              <p className='pricing-showcase-card-description'>
                {getModelDescription(model, language, t)}
              </p>

              <div
                className={`pricing-showcase-card-prices${priceData.isDynamicPricing ? ' is-dynamic' : ''}`}
              >
                <div
                  className='pricing-showcase-card-price-list'
                  style={{
                    gridTemplateColumns: `repeat(${priceItems.length}, minmax(0, 1fr))`,
                  }}
                >
                  {priceItems.map((item) => (
                    <div className='pricing-showcase-price-cell' key={item.key}>
                      <small>{item.label}</small>
                      {item.isDynamic ? (
                        <div className='pricing-showcase-dynamic-price'>
                          {formatDynamicPriceCompactSummary(
                            priceData.billingExpr,
                            t,
                          )}
                        </div>
                      ) : (
                        <>
                          <strong>{item.value}</strong>
                          <span>{item.suffix}</span>
                        </>
                      )}
                    </div>
                  ))}
                </div>

                <button
                  className='pricing-showcase-detail-button'
                  type='button'
                  aria-label={`${t('详情')} ${model.model_name}`}
                  title={t('详情')}
                  onClick={(event) => {
                    event.stopPropagation();
                    openModelDetail?.(model);
                  }}
                >
                  <Eye size={16} />
                </button>
              </div>
            </article>
          );
        })}
      </div>

      {filteredModels.length > pageSize && (
        <div className='pricing-showcase-pagination'>
          <Pagination
            currentPage={currentPage}
            pageSize={pageSize}
            total={filteredModels.length}
            onPageChange={(page) => setCurrentPage(page)}
          />
        </div>
      )}
    </section>
  );
};

export default PricingCardView;
