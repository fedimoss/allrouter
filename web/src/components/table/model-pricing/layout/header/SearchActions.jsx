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

import React, { memo, useCallback } from 'react';
import { Input, Button, Switch, Select } from '@douyinfe/semi-ui';
import { IconSearch, IconCopy, IconFilter } from '@douyinfe/semi-icons';
import { LayoutGrid, Rows3 } from 'lucide-react';

const getSortOptions = (t) => [
  { value: 'hot', label: t('热门优先') },
  { value: 'latest', label: t('最新上架') },
  { value: 'value', label: t('价格优先') },
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
    t,
  }) => {
    const supportsCurrencyDisplay = siteDisplayType !== 'TOKENS';

    const handleCopyClick = useCallback(() => {
      if (copyText && selectedRowKeys.length > 0) {
        copyText(selectedRowKeys.join(', '));
      }
    }, [copyText, selectedRowKeys]);

    if (isMobile) {
      return (
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
      );
    }

    return (
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
          {supportsCurrencyDisplay && (
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
          )}

          <label className='pricing-market-toolbar-toggle'>
            <span>{t('倍率')}</span>
            <Switch checked={showRatio} onChange={setShowRatio} />
          </label>

          <button
            type='button'
            className={tokenUnit === 'K' ? 'pricing-market-toolbar-chip is-active' : 'pricing-market-toolbar-chip'}
            onClick={() => setTokenUnit?.(tokenUnit === 'K' ? 'M' : 'K')}
          >
            {tokenUnit}
          </button>

          <Select
            value={sortMode}
            onChange={setSortMode}
            optionList={getSortOptions(t)}
            className='pricing-market-toolbar-sort'
          />

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
    );
  },
);

SearchActions.displayName = 'SearchActions';

export default SearchActions;
