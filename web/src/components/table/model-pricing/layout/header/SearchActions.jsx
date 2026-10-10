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

import React, { memo, useCallback, useMemo, useState } from 'react';
import {
  Input,
  InputNumber,
  Button,
  Modal,
  Select,
  Popover,
  Tooltip,
} from '@douyinfe/semi-ui';
import {
  IconSearch,
  IconCopy,
  IconFilter,
  IconPlus,
  IconDelete,
} from '@douyinfe/semi-icons';
import {
  ArrowDown,
  ArrowUp,
  LayoutGrid,
  ListFilter,
  ListOrdered,
  Rows3,
} from 'lucide-react';
import { PRICING_SORT_FIELDS, sortPricingModels } from '../pricingSort';

const getSortOptions = (t, allowCustomSort, currentMode) => [
  { value: 'hot', label: t('热门优先') },
  { value: 'latest', label: t('最新上架') },
  { value: 'value', label: t('价格优先') },
  ...(allowCustomSort || currentMode === 'custom'
    ? [{ value: 'custom', label: t('自定义排序') }]
    : []),
];

const SearchActions = memo(
  ({
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
    filteredCount = 0,
    sortMode = 'hot',
    setSortMode,
    allowCustomSort = false,
    sortCriteria = [],
    setSortCriteria,
    previewModels = [],
    weightModels = [],
    sortWeights = {},
    setSortWeights,
    onSaveSortConfig,
    onResetSortConfig,
    sortConfigSaving = false,
    t,
  }) => {
    const [weightModalVisible, setWeightModalVisible] = useState(false);
    const [weightDraft, setWeightDraft] = useState({});
    const [weightSearch, setWeightSearch] = useState('');

    const handleCopyClick = useCallback(() => {
      if (copyText && selectedRowKeys.length > 0) {
        copyText(selectedRowKeys.join(', '));
      }
    }, [copyText, selectedRowKeys]);

    const sortFieldOptions = PRICING_SORT_FIELDS.map(({ value, label }) => ({
      value,
      label: t(label),
    }));
    const usedSortFields = new Set(sortCriteria.map(({ field }) => field));
    const availableSortFields = sortFieldOptions.filter(
      ({ value }) => !usedSortFields.has(value),
    );

    const updateSortCriterion = (index, updates) => {
      setSortCriteria?.((current) =>
        current.map((criterion, criterionIndex) =>
          criterionIndex === index ? { ...criterion, ...updates } : criterion,
        ),
      );
      setSortMode?.('custom');
    };

    const moveSortCriterion = (index, offset) => {
      setSortCriteria?.((current) => {
        const targetIndex = index + offset;
        if (targetIndex < 0 || targetIndex >= current.length) return current;
        const next = [...current];
        [next[index], next[targetIndex]] = [next[targetIndex], next[index]];
        return next;
      });
      setSortMode?.('custom');
    };

    const addSortCriterion = () => {
      const field = availableSortFields[0]?.value;
      if (!field) return;
      setSortCriteria?.((current) => [...current, { field, direction: 'asc' }]);
      setSortMode?.('custom');
    };

    const removeSortCriterion = (index) => {
      setSortCriteria?.((current) =>
        current.filter((_, criterionIndex) => criterionIndex !== index),
      );
      setSortMode?.('custom');
    };

    const numericSortFields = new Set([
      'model_ratio',
      'completion_ratio',
      'model_price',
      'quota_type',
      'sort_weight',
    ]);

    const getPreviewValue = (model, field) => {
      const value =
        field === 'sort_weight'
          ? sortWeights?.[model?.model_name]
          : model?.[field];
      if (value === null || value === undefined || value === '') return '-';
      if (Array.isArray(value)) return value.join(', ');
      if (typeof value === 'number') return value;
      return String(value);
    };

    const previewField = sortCriteria[0]?.field;
    const previewFieldLabel = t(
      PRICING_SORT_FIELDS.find(({ value }) => value === previewField)?.label ||
        '排序字段',
    );
    const previewModelsToShow = previewModels.slice(0, 8);

    const weightModelEntries = useMemo(() => {
      const byName = new Map();
      weightModels.forEach((model) => {
        const modelName = String(model?.model_name || '').trim();
        if (!modelName) return;
        const existing = byName.get(modelName);
        const vendorName = String(model?.vendor_name || '').trim();
        if (existing) {
          if (vendorName) existing.vendors.add(vendorName);
          return;
        }
        byName.set(modelName, {
          model,
          modelName,
          vendors: new Set(vendorName ? [vendorName] : []),
        });
      });
      return [...byName.values()].sort((a, b) =>
        a.modelName.localeCompare(b.modelName, undefined, {
          numeric: true,
          sensitivity: 'base',
        }),
      );
    }, [weightModels]);

    const weightedModelEntries = useMemo(() => {
      const models = weightModelEntries.map(({ model }) => model);
      const sorted = sortPricingModels(
        models,
        'custom',
        [{ field: 'sort_weight', direction: 'desc' }],
        weightDraft,
      );
      const rankByName = new Map(
        sorted.map((model, index) => [model.model_name, index + 1]),
      );
      return weightModelEntries
        .map((entry) => ({ ...entry, rank: rankByName.get(entry.modelName) }))
        .sort((a, b) => a.rank - b.rank);
    }, [weightModelEntries, weightDraft]);

    const filteredWeightEntries = useMemo(() => {
      const query = weightSearch.trim().toLowerCase();
      if (!query) return weightedModelEntries;
      return weightedModelEntries.filter(({ modelName, vendors }) =>
        `${modelName} ${[...vendors].join(' ')}`.toLowerCase().includes(query),
      );
    }, [weightedModelEntries, weightSearch]);

    const openWeightModal = () => {
      setWeightDraft({ ...sortWeights });
      setWeightSearch('');
      setWeightModalVisible(true);
    };

    const updateWeight = (modelName, value) => {
      setWeightDraft((current) => {
        const next = { ...current };
        const numericValue = Number(value);
        if (
          value === null ||
          value === undefined ||
          value === '' ||
          !Number.isFinite(numericValue)
        ) {
          delete next[modelName];
        } else {
          next[modelName] = numericValue;
        }
        return next;
      });
    };

    const applyWeights = () => {
      setSortWeights?.(weightDraft);
      if (
        Object.keys(weightDraft).length > 0 &&
        !sortCriteria.some(({ field }) => field === 'sort_weight')
      ) {
        setSortCriteria?.((current) => [
          { field: 'sort_weight', direction: 'desc' },
          ...current,
        ]);
      }
      setSortMode?.('custom');
      setWeightModalVisible(false);
    };

    const customSortPopover = allowCustomSort ? (
      <div className='pricing-market-sort-popover'>
        <div className='pricing-market-sort-popover-title'>
          <span>{t('排序条件')}</span>
          <div className='pricing-market-sort-popover-title-actions'>
            <Button
              size='small'
              theme='outline'
              type='tertiary'
              icon={<ListOrdered size={15} />}
              onClick={openWeightModal}
              aria-label={t('设置排序大小')}
              title={t('设置排序大小')}
            >
              {t('排序大小')}
            </Button>
            <Button
              size='small'
              theme='borderless'
              type='tertiary'
              icon={<IconPlus />}
              onClick={addSortCriterion}
              disabled={availableSortFields.length === 0}
              aria-label={t('添加排序条件')}
              title={t('添加排序条件')}
            />
          </div>
        </div>
        {sortCriteria.map((criterion, index) => {
          const optionList = sortFieldOptions.filter(
            ({ value }) =>
              value === criterion.field || !usedSortFields.has(value),
          );
          return (
            <div className='pricing-market-sort-row' key={criterion.field}>
              <Select
                value={criterion.field}
                optionList={optionList}
                onChange={(field) => updateSortCriterion(index, { field })}
                aria-label={t('排序字段')}
              />
              <Select
                value={criterion.direction}
                optionList={[
                  {
                    value: 'asc',
                    label: numericSortFields.has(criterion.field)
                      ? t('升序（小到大）')
                      : t('升序'),
                  },
                  {
                    value: 'desc',
                    label: numericSortFields.has(criterion.field)
                      ? t('降序（大到小）')
                      : t('降序'),
                  },
                ]}
                onChange={(direction) =>
                  updateSortCriterion(index, { direction })
                }
                aria-label={t('排序方向')}
              />
              <div className='pricing-market-sort-row-actions'>
                <Tooltip content={t('上移')}>
                  <Button
                    size='small'
                    theme='borderless'
                    type='tertiary'
                    icon={<ArrowUp size={14} />}
                    disabled={index === 0}
                    onClick={() => moveSortCriterion(index, -1)}
                    aria-label={t('上移')}
                  />
                </Tooltip>
                <Tooltip content={t('下移')}>
                  <Button
                    size='small'
                    theme='borderless'
                    type='tertiary'
                    icon={<ArrowDown size={14} />}
                    disabled={index === sortCriteria.length - 1}
                    onClick={() => moveSortCriterion(index, 1)}
                    aria-label={t('下移')}
                  />
                </Tooltip>
                <Tooltip content={t('移除排序条件')}>
                  <Button
                    size='small'
                    theme='borderless'
                    type='tertiary'
                    icon={<IconDelete />}
                    disabled={sortCriteria.length <= 1}
                    onClick={() => removeSortCriterion(index)}
                    aria-label={t('移除排序条件')}
                  />
                </Tooltip>
              </div>
            </div>
          );
        })}
        <div className='pricing-market-sort-preview'>
          <div className='pricing-market-sort-preview-header'>
            <span>{t('排序预览')}</span>
            <span className='pricing-market-sort-preview-count'>
              {t('共 {{count}} 个模型', { count: previewModels.length })}
            </span>
          </div>
          <div className='pricing-market-sort-preview-columns'>
            <span>{t('模型名称')}</span>
            <span>{previewFieldLabel}</span>
          </div>
          {previewModelsToShow.length > 0 ? (
            <div className='pricing-market-sort-preview-list'>
              {previewModelsToShow.map((model, index) => (
                <div
                  className='pricing-market-sort-preview-item'
                  key={model.key || model.model_name || model.id || index}
                >
                  <span className='pricing-market-sort-preview-rank'>
                    {index + 1}
                  </span>
                  <span
                    className='pricing-market-sort-preview-name'
                    title={model.model_name}
                  >
                    {model.model_name || '-'}
                  </span>
                  <span className='pricing-market-sort-preview-value'>
                    {getPreviewValue(model, previewField)}
                  </span>
                </div>
              ))}
            </div>
          ) : (
            <div className='pricing-market-sort-preview-empty'>
              {t('搜索无结果')}
            </div>
          )}
        </div>
        <div className='pricing-market-sort-popover-actions'>
          <Button
            size='small'
            theme='outline'
            type='tertiary'
            onClick={onResetSortConfig}
            disabled={
              sortConfigSaving || typeof onResetSortConfig !== 'function'
            }
          >
            {t('恢复默认')}
          </Button>
          <Button
            size='small'
            theme='solid'
            type='primary'
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              onSaveSortConfig?.(sortMode, sortCriteria, sortWeights);
            }}
            loading={sortConfigSaving}
            disabled={
              sortConfigSaving || typeof onSaveSortConfig !== 'function'
            }
          >
            {t('保存')}
          </Button>
          <Button
            size='small'
            theme='borderless'
            type='tertiary'
            icon={<Rows3 size={14} />}
            onClick={() => setViewMode?.('table')}
          >
            {t('表格视图')}
          </Button>
        </div>
      </div>
    ) : null;

    const weightModal = (
      <Modal
        title={t('设置排序大小')}
        visible={weightModalVisible}
        onCancel={() => setWeightModalVisible(false)}
        onOk={applyWeights}
        okText={t('确定')}
        cancelText={t('取消')}
        width={
          typeof window === 'undefined'
            ? 760
            : Math.max(320, Math.min(760, window.innerWidth - 32))
        }
        centered
      >
        <div className='pricing-market-weight-modal-description'>
          {t('数字越大越靠前，未设置的模型排在已设置模型之后。')}
        </div>
        <Input
          prefix={<IconSearch />}
          placeholder={t('搜索模型名称')}
          value={weightSearch}
          onChange={setWeightSearch}
          showClear
          className='pricing-market-weight-search'
        />
        <div className='pricing-market-weight-list'>
          <div className='pricing-market-weight-row pricing-market-weight-head'>
            <span>{t('排名')}</span>
            <span>{t('模型名称')}</span>
            <span>{t('供应商')}</span>
            <span>{t('排序大小')}</span>
          </div>
          {filteredWeightEntries.length > 0 ? (
            filteredWeightEntries.map(({ modelName, vendors, rank }) => (
              <div className='pricing-market-weight-row' key={modelName}>
                <span className='pricing-market-weight-rank'>{rank}</span>
                <span className='pricing-market-weight-name' title={modelName}>
                  {modelName}
                </span>
                <span
                  className='pricing-market-weight-vendor'
                  title={[...vendors].join(', ')}
                >
                  {[...vendors].join(', ') || '-'}
                </span>
                <InputNumber
                  size='small'
                  min={0}
                  max={2147483647}
                  precision={0}
                  value={weightDraft[modelName]}
                  placeholder={t('未设置')}
                  onChange={(value) => updateWeight(modelName, value)}
                  style={{ width: '100%' }}
                />
              </div>
            ))
          ) : (
            <div className='pricing-market-weight-empty'>{t('暂无模型')}</div>
          )}
        </div>
      </Modal>
    );

    const sortControls = (
      <>
        <Select
          value={sortMode}
          onChange={setSortMode}
          optionList={getSortOptions(t, allowCustomSort, sortMode)}
          disabled={!allowCustomSort && sortMode === 'custom'}
          className='pricing-market-toolbar-sort'
        />
        {allowCustomSort && (
          <Tooltip content={t('配置自定义排序')}>
            <span className='pricing-market-sort-trigger'>
              <Popover
                trigger='click'
                position='bottomRight'
                showArrow={false}
                content={customSortPopover}
              >
                <Button
                  size='default'
                  theme={sortMode === 'custom' ? 'light' : 'outline'}
                  type={sortMode === 'custom' ? 'primary' : 'tertiary'}
                  icon={<ListFilter size={16} />}
                  aria-label={t('配置自定义排序')}
                  onClick={() => setSortMode?.('custom')}
                />
              </Popover>
            </span>
          </Tooltip>
        )}
      </>
    );

    if (isMobile) {
      return (
        <>
          <div className='pricing-market-mobile-toolbar'>
            <div className='pricing-market-mobile-search'>
              <Input
                prefix={<IconSearch />}
                placeholder={t('模糊搜索模型名称')}
                value={searchValue}
                onCompositionStart={handleCompositionStart}
                onCompositionEnd={handleCompositionEnd}
                onChange={handleChange}
                showClear
              />
            </div>
            <div className='pricing-market-mobile-actions'>
              {allowCustomSort && (
                <div className='pricing-market-mobile-sort-controls'>
                  {sortControls}
                </div>
              )}
              <Button
                type='primary'
                theme='solid'
                icon={<IconCopy />}
                onClick={handleCopyClick}
                disabled={selectedRowKeys.length === 0}
              >
                {t('复制')}
              </Button>
              <Button
                theme='outline'
                type='tertiary'
                icon={<IconFilter />}
                onClick={() => setShowFilterModal?.(true)}
              >
                {t('筛选')}
              </Button>
            </div>
          </div>
          {weightModal}
        </>
      );
    }

    return (
      <>
        <div className='pricing-market-toolbar'>
          <div className='pricing-market-toolbar-search'>
            <Input
              prefix={<IconSearch />}
              placeholder={t('模糊搜索模型名称') + '  GPT-4 / Claude / Gemini'}
              value={searchValue}
              onCompositionStart={handleCompositionStart}
              onCompositionEnd={handleCompositionEnd}
              onChange={handleChange}
              showClear
            />
          </div>

          <div className='pricing-market-toolbar-count'>
            {t('共 {{count}} 个模型', { count: filteredCount })}
          </div>

          <div className='pricing-market-toolbar-actions'>
            {/* {supportsCurrencyDisplay && (
            <label className='pricing-market-toolbar-toggle'>
              <span>{t('充值价格')}</span>
              <Switch color='red' checked={showWithRecharge} onChange={setShowWithRecharge} />
            </label>
          )}

          {supportsCurrencyDisplay && showWithRecharge && (
            <Select
              value={currency}
              onChange={setCurrency}
              optionList={[
                { value: 'USD', label: 'USD' },
                { value: 'CNY', label: 'CNY' },
                { value: 'CUSTOM', label: t('自定义货币') },
              ]}
              className='pricing-market-toolbar-currency'
            />
          )} */}

            {/* <label className='pricing-market-toolbar-toggle'>
            <span>{t('倍率')}</span>
            <Switch checked={showRatio} onChange={setShowRatio} />
          </label> */}

            <button
              type='button'
              className={
                tokenUnit === 'K'
                  ? 'pricing-market-toolbar-chip is-active'
                  : 'pricing-market-toolbar-chip'
              }
              onClick={() => setTokenUnit?.(tokenUnit === 'K' ? 'M' : 'K')}
            >
              {tokenUnit}
            </button>

            {sortControls}

            <div className='pricing-market-view-switch'>
              <button
                type='button'
                className={viewMode === 'card' ? 'is-active' : ''}
                onClick={() => setViewMode?.('card')}
                aria-label={t('卡片视图')}
              >
                <LayoutGrid size={16} />
              </button>
              <button
                type='button'
                className={viewMode === 'table' ? 'is-active' : ''}
                onClick={() => setViewMode?.('table')}
                aria-label={t('表格视图')}
              >
                <Rows3 size={16} />
              </button>
            </div>
          </div>
        </div>
        {weightModal}
      </>
    );
  },
);

SearchActions.displayName = 'SearchActions';

export default SearchActions;
