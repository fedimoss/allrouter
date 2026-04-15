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
import {
  Card,
  Tag,
  Tooltip,
  Checkbox,
  Empty,
  Pagination,
  Button,
  Avatar,
} from '@douyinfe/semi-ui';
import { IconHelpCircle } from '@douyinfe/semi-icons';
import { Copy, Heart, Info,Eye } from 'lucide-react';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  stringToColor,
  calculateModelPrice,
  formatPriceInfo,
  getLobeHubIcon,
} from '../../../../../helpers';
import PricingCardSkeleton from './PricingCardSkeleton';
import { useMinimumLoadingTime } from '../../../../../hooks/common/useMinimumLoadingTime';
import { renderLimitedItems } from '../../../../common/ui/RenderUtils';
import { useIsMobile } from '../../../../../hooks/common/useIsMobile';

const getModelKey = (model) => model.key ?? model.model_name ?? model.id;

const estimateContext = (modelName = '') => {
  const name = String(modelName).toLowerCase();
  if (name.includes('claude')) return '200k';
  if (name.includes('gpt-4') || name.includes('gpt-5')) return '128k';
  if (name.includes('gemini')) return '1M';
  if (name.includes('llama')) return '8k';
  return '64k';
};

const estimateChannelCount = (model) => {
  const count = Array.isArray(model?.enable_groups) ? model.enable_groups.length : 0;
  if (count >= 4) return 4;
  if (count >= 2) return 3;
  return 2;
};

const buildPrimaryPriceItems = (priceData, t, quotaDisplayType) => {
  if (priceData?.isPerToken) {
    if (quotaDisplayType === 'TOKENS' || priceData.isTokensDisplay) {
      return [
        { key: 'input-ratio', label: t('输入倍率'), value: priceData.inputRatio, suffix: 'x' },
        { key: 'completion-ratio', label: t('补全倍率'), value: priceData.completionRatio, suffix: 'x' },
        { key: 'cache-ratio', label: t('缓存读取倍率'), value: priceData.cacheRatio, suffix: 'x' },
      ].filter((item) => item.value !== null && item.value !== undefined && item.value !== '');
    }

    const unitSuffix = ` / 1${priceData.unitLabel} Tokens`;
    return [
      { key: 'input', label: t('输入价格'), value: priceData.inputPrice, suffix: unitSuffix },
      { key: 'completion', label: t('补全价格'), value: priceData.completionPrice, suffix: unitSuffix },
      { key: 'cache', label: t('缓存读取价格'), value: priceData.cachePrice, suffix: unitSuffix },
    ].filter((item) => item.value !== null && item.value !== undefined && item.value !== '');
  }

  return [{ key: 'fixed', label: t('模型价格'), value: priceData?.price ?? '-', suffix: ` / ${t('次')}` }];
};

const PricingCardView = ({
  filteredModels,
  loading,
  rowSelection,
  pageSize,
  setPageSize,
  currentPage,
  setCurrentPage,
  selectedGroup,
  groupRatio,
  copyText,
  setModalImageUrl,
  setIsModalOpenurl,
  currency,
  siteDisplayType,
  tokenUnit,
  displayPrice,
  showRatio,
  t,
  selectedRowKeys = [],
  setSelectedRowKeys,
  openModelDetail,
}) => {
  const showSkeleton = useMinimumLoadingTime(loading);
  const startIndex = (currentPage - 1) * pageSize;
  const paginatedModels = filteredModels.slice(startIndex, startIndex + pageSize);
  const isMobile = useIsMobile();

  const handleCheckboxChange = (model, checked) => {
    if (!setSelectedRowKeys) return;
    const modelKey = getModelKey(model);
    const newKeys = checked
      ? Array.from(new Set([...selectedRowKeys, modelKey]))
      : selectedRowKeys.filter((key) => key !== modelKey);
    setSelectedRowKeys(newKeys);
    rowSelection?.onChange?.(newKeys, null);
  };

  const handleOpenModelDetail = (model) => {
    openModelDetail?.(model);
  };

  const getModelIcon = (model) => {
    if (!model || !model.model_name) {
      return <div className='pricing-market-model-logo'><Avatar size='large'>?</Avatar></div>;
    }
    if (model.icon) {
      return <div className='pricing-market-model-logo'>{getLobeHubIcon(model.icon, 28)}</div>;
    }
    if (model.vendor_icon) {
      return <div className='pricing-market-model-logo'>{getLobeHubIcon(model.vendor_icon, 28)}</div>;
    }
    return (
      <div className='pricing-market-model-logo'>
        <Avatar size='large' style={{ width: 44, height: 44, borderRadius: 14, fontWeight: 700 }}>
          {model.model_name.slice(0, 1).toUpperCase()}
        </Avatar>
      </div>
    );
  };

  const renderTags = (record) => {
    let billingTag = <Tag key='billing' shape='circle' color='white' size='small'>-</Tag>;
    if (record.quota_type === 1) {
      billingTag = <Tag key='billing' shape='circle' color='teal' size='small'>{t('按次计费')}</Tag>;
    } else if (record.quota_type === 0) {
      billingTag = <Tag key='billing' shape='circle' color='violet' size='small'>{t('按量计费')}</Tag>;
    }

    const customTags = [];
    if (record.tags) {
      record.tags.split(',').filter(Boolean).forEach((tag, idx) => {
        customTags.push(<Tag key={`custom-${idx}`} shape='circle' color={stringToColor(tag)} size='small'>{tag}</Tag>);
      });
    }

    return (
      <div className='flex items-center justify-between'>
        <div className='flex items-center gap-2'>{billingTag}</div>
        <div className='flex items-center gap-1'>
          {customTags.length > 0 && renderLimitedItems({
            items: customTags.map((tag, idx) => ({ key: `custom-${idx}`, element: tag })),
            renderItem: (item) => item.element,
            maxDisplay: 3,
          })}
        </div>
      </div>
    );
  };

  if (showSkeleton) {
    return <PricingCardSkeleton rowSelection={!!rowSelection} showRatio={showRatio} />;
  }

  if (!filteredModels || filteredModels.length === 0) {
    return (
      <div className='flex justify-center items-center py-20'>
        <Empty
          image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
          darkModeImage={<IllustrationNoResultDark style={{ width: 150, height: 150 }} />}
          description={t('搜索无结果')}
        />
      </div>
    );
  }

  return (
    <div className={isMobile ? 'px-2 pt-2' : 'pricing-market-card-view'}>
      <div className={isMobile ? 'grid grid-cols-1 xl:grid-cols-2 2xl:grid-cols-3 gap-4' : 'pricing-market-card-grid'}>
        {paginatedModels.map((model, index) => {
          const modelKey = getModelKey(model);
          const isSelected = selectedRowKeys.includes(modelKey);
          const priceData = calculateModelPrice({
            record: model,
            selectedGroup,
            groupRatio,
            tokenUnit,
            displayPrice,
            currency,
            quotaDisplayType: siteDisplayType,
          });
          const priceItems = buildPrimaryPriceItems(priceData, t, siteDisplayType);
          const priceBoardItems = [...priceItems, { key: 'detail', label: t('详情'), isAction: true }];
          const priceGridStyle = { gridTemplateColumns: `repeat(${Math.max(priceBoardItems.length, 1)}, minmax(0, 1fr))` };

          if (isMobile) {
            return (
              <Card
                key={modelKey || index}
                className={`pricing-market-mobile-card${isSelected ? ' is-selected' : ''}`}
                bodyStyle={{ height: '100%' }}
              >
                <div className='flex flex-col h-full' onClick={() => handleOpenModelDetail(model)}>
                  <div className='flex items-start justify-between mb-3'>
                    <div className='flex items-start space-x-3 flex-1 min-w-0'>
                      {getModelIcon(model)}
                      <div className='flex-1 min-w-0'>
                        <h3 className='pricing-market-mobile-card-title'>{model.model_name}</h3>
                        <div className='pricing-market-mobile-card-price flex flex-col gap-1 text-xs mt-1'>
                          {formatPriceInfo(priceData, t, siteDisplayType)}
                        </div>
                      </div>
                    </div>
                    <div className='flex items-center space-x-2 ml-3'>
                      <Eye onClick={(e) => {
                          e.stopPropagation();
                          handleOpenModelDetail(model);
                        }} />
                      {/* <Button
                        size='small'
                        theme='outline'
                        type='tertiary'
                        icon={<Info size={12} />}
                        onClick={(e) => {
                          e.stopPropagation();
                          handleOpenModelDetail(model);
                        }}
                      >
                        {t('详情')}
                      </Button> */}
                      <Button size='small' theme='outline' type='tertiary' icon={<Copy size={12} />} onClick={(e) => { e.stopPropagation(); copyText(model.model_name); }} />
                      {rowSelection && (
                        <Checkbox checked={isSelected} onChange={(e) => { e.stopPropagation(); handleCheckboxChange(model, e.target.checked); }} />
                      )}
                    </div>
                  </div>

                  <div className='flex-1 mb-4'>
                    <p className='pricing-market-mobile-card-description text-xs line-clamp-2 leading-relaxed'>
                      {model.description || ''}
                    </p>
                  </div>

                  <div className='mt-auto'>
                    {renderTags(model)}
                    {showRatio && (
                      <div className='pricing-market-mobile-card-ratio pt-3'>
                        <div className='flex items-center space-x-1 mb-2'>
                          <span className='text-xs font-medium'>{t('倍率信息')}</span>
                          <Tooltip content={t('倍率是为了方便换算不同价格的模型')}>
                            <IconHelpCircle className='text-blue-500 cursor-pointer' size='small' onClick={(e) => { e.stopPropagation(); setModalImageUrl('/ratio.png'); setIsModalOpenurl(true); }} />
                          </Tooltip>
                        </div>
                        <div className='grid grid-cols-3 gap-2 text-xs'>
                          <div>{t('模型')}: {model.quota_type === 0 ? model.model_ratio : t('无')}</div>
                          <div>{t('补全')}: {model.quota_type === 0 ? parseFloat(model.completion_ratio.toFixed(3)) : t('无')}</div>
                          <div>{t('分组')}: {priceData?.usedGroupRatio ?? '-'}</div>
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              </Card>
            );
          }


          return (
            <Card key={modelKey || index} className={`pricing-market-desktop-card${isSelected ? ' is-selected' : ''}`} bodyStyle={{ padding: 0, height: '100%' }}>
              <div
                className='pricing-market-desktop-card-inner'
                onClick={() => handleOpenModelDetail(model)}
                style={{ cursor: openModelDetail ? 'pointer' : 'default' }}
              >
                <div className='pricing-market-desktop-card-header'>
                  <div className='pricing-market-desktop-card-brand'>
                    {getModelIcon(model)}
                    <div className='pricing-market-desktop-card-title-wrap'>
                      <h3>{model.model_name}</h3>
                      <div className='pricing-market-desktop-card-meta'>
                        <span>Context: {estimateContext(model.model_name)}</span>
                        <span>{estimateChannelCount(model)} {t('个渠道')}</span>
                      </div>
                    </div>
                  </div>

                  <div className='pricing-market-desktop-card-actions'>
                    <Button theme='borderless' type='tertiary' icon={<Heart size={16} />} onClick={(e) => { e.stopPropagation(); }} />
                    <Button theme='borderless' type='tertiary' icon={<Copy size={14} />} onClick={(e) => { e.stopPropagation(); copyText(model.model_name); }} />
                    {/* {rowSelection && <Checkbox checked={isSelected} onChange={(e) => handleCheckboxChange(model, e.target.checked)} />} */}
                  </div>
                </div>

                <p className='pricing-market-desktop-card-description'>
                  {model.description || `${model.vendor_name || t('通用')} ${t('最新模型，适合多轮对话、推理与生产环境调用。')}`}
                </p>

                {/* <div className='pricing-market-desktop-card-tags'>
                  {(rawTags.length ? rawTags : [t('对话')]).map((tag) => (
                    <span key={tag} className='pricing-market-desktop-card-tag' style={{ backgroundColor: `${stringToColor(tag)}22`, color: stringToColor(tag) }}>
                      {tag}
                    </span>
                  ))}
                </div> */}

                <div className='pricing-market-price-board'>
                  <div className='pricing-market-price-head' style={priceGridStyle}>
                    {priceBoardItems.map((item) => <span key={item.key}>{item.label}</span>)}
                  </div>
                  <div className='pricing-market-price-row' style={priceGridStyle}>
                    {priceBoardItems.map((item) => (
                      item.isAction ? (
                      <Eye
                        key={item.key}
                        size={16}
                        className='pricing-market-detail-trigger'
                        onClick={(e) => {
                          e.stopPropagation();
                          handleOpenModelDetail(model);
                        }}
                      />
                      ) : (
                        <span key={item.key}>{item.value}<small>{item.suffix}</small></span>
                      )
                    ))}
                  </div>
                </div>

                {showRatio && (
                  <div className='pricing-market-desktop-card-ratio'>
                    <span>{t('模型倍率')} {model.quota_type === 0 ? model.model_ratio : '-'}</span>
                    <span>{t('补全倍率')} {model.quota_type === 0 ? parseFloat(model.completion_ratio.toFixed(3)) : '-'}</span>
                    <span>{t('分组倍率')} {priceData?.usedGroupRatio ?? '-'}</span>
                  </div>
                )}

                {/* <button type='button' className='pricing-market-desktop-card-link' onClick={(e) => { e.stopPropagation(); handleOpenModelDetail(model); }}>
                  {t('查看全部渠道对比')} <ChevronDown size={14} />
                </button> */}
              </div>
            </Card>
          );
        })}
      </div>

      {filteredModels.length > 0 && (
        <div className='flex justify-center mt-6 py-4 border-t pricing-pagination-divider'>
          <Pagination
            currentPage={currentPage}
            pageSize={pageSize}
            total={filteredModels.length}
            showSizeChanger={true}
            pageSizeOptions={[10, 20, 50, 100]}
            size={isMobile ? 'small' : 'default'}
            showQuickJumper={isMobile}
            onPageChange={(page) => setCurrentPage(page)}
            onPageSizeChange={(size) => {
              setPageSize(size);
              setCurrentPage(1);
            }}
          />
        </div>
      )}
    </div>
  );
};

export default PricingCardView;
