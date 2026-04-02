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
import { Button, Checkbox, Radio } from '@douyinfe/semi-ui';
import { IconFilter, IconChevronDown,IconRadio } from '@douyinfe/semi-icons';
import { resetPricingFilters } from '../../../../helpers/utils';
import { usePricingFilterCounts } from '../../../../hooks/model-pricing/usePricingFilterCounts';

const Section = ({ title, children }) => (
  <section className='pricing-market-filter-section'>
    <div className='pricing-market-filter-section-title'>
      <span>{title}</span>
      <IconChevronDown />
    </div>
    <div className='pricing-market-filter-section-body'>{children}</div>
  </section>
);

const PricingSidebar = ({
  setShowWithRecharge,
  setCurrency,
  handleChange,
  setShowRatio,
  setViewMode,
  filterGroup,
  setFilterGroup,
  handleGroupClick,
  filterQuotaType,
  setFilterQuotaType,
  filterEndpointType,
  setFilterEndpointType,
  filterVendor,
  setFilterVendor,
  filterTag,
  setFilterTag,
  setCurrentPage,
  setTokenUnit,
  t,
  ...categoryProps
}) => {
  const {
    quotaTypeModels,
    endpointTypeModels,
    vendorModels,
    tagModels,
    groupCountModels,
  } = usePricingFilterCounts({
    models: categoryProps.models,
    filterGroup,
    filterQuotaType,
    filterEndpointType,
    filterVendor,
    filterTag,
    searchValue: categoryProps.searchValue,
  });

  const handleResetFilters = () =>
    resetPricingFilters({
      handleChange,
      setShowWithRecharge,
      setCurrency,
      setShowRatio,
      setViewMode,
      setFilterGroup,
      setFilterQuotaType,
      setFilterEndpointType,
      setFilterVendor,
      setFilterTag,
      setCurrentPage,
      setTokenUnit,
    });

  const providerItems = React.useMemo(() => {
    const vendors = new Map();
    let unknownCount = 0;

    (categoryProps.models || []).forEach((model) => {
      if (model.vendor_name) {
        vendors.set(model.vendor_name, (vendors.get(model.vendor_name) || 0) + 1);
      } else {
        unknownCount += 1;
      }
    });

    const items = [
      { value: 'all', label: t('全部供应商'), count: vendorModels.length },
      ...Array.from(vendors.keys())
        .sort((a, b) => a.localeCompare(b))
        .map((name) => ({
          value: name,
          label: name,
          count: vendorModels.filter((model) => model.vendor_name === name).length,
        })),
    ];

    if (unknownCount > 0) {
      items.push({
        value: 'unknown',
        label: t('未知供应商'),
        count: vendorModels.filter((model) => !model.vendor_name).length,
      });
    }

    return items;
  }, [categoryProps.models, t, vendorModels]);

  const groupItems = React.useMemo(() => {
    const groups = Object.keys(categoryProps.usableGroup || {}).filter(Boolean);
    return [
      { value: 'all', label: t('全部分组'), count: groupCountModels.length },
      ...groups.map((group) => ({
        value: group,
        label: group,
        count: groupCountModels.filter((item) => item.enable_groups?.includes(group)).length,
        ratio: categoryProps.groupRatio?.[group],
      })),
    ];
  }, [categoryProps.groupRatio, categoryProps.usableGroup, groupCountModels, t]);

  const quotaItems = React.useMemo(
    () => [
      { value: 'all', label: t('全部类型'), count: quotaTypeModels.length },
      {
        value: 0,
        label: t('按量计费'),
        count: quotaTypeModels.filter((model) => model.quota_type === 0).length,
      },
      {
        value: 1,
        label: t('按次计费'),
        count: quotaTypeModels.filter((model) => model.quota_type === 1).length,
      },
    ],
    [quotaTypeModels, t],
  );

  const endpointItems = React.useMemo(() => {
    const counts = new Map();
    endpointTypeModels.forEach((model) => {
      (model.supported_endpoint_types || []).forEach((type) => {
        counts.set(type, (counts.get(type) || 0) + 1);
      });
    });

    return [
      { value: 'all', label: t('全部端点'), count: endpointTypeModels.length },
      ...Array.from(counts.entries())
        .sort((a, b) => String(a[0]).localeCompare(String(b[0])))
        .map(([value, count]) => ({ value, label: value, count })),
    ];
  }, [endpointTypeModels, t]);

  const tagItems = React.useMemo(() => {
    const counts = new Map();
    (tagModels || []).forEach((model) => {
      (model.tags || '')
        .split(/[,;|]+/)
        .map((tag) => tag.trim())
        .filter(Boolean)
        .forEach((tag) => {
          const key = tag.toLowerCase();
          counts.set(key, { label: tag, count: (counts.get(key)?.count || 0) + 1 });
        });
    });

    return [
      { value: 'all', label: t('全部标签') },
      ...Array.from(counts.entries())
        .sort((a, b) => a[0].localeCompare(b[0]))
        .map(([value, info]) => ({
          value,
          label: info.label,
          count: info.count,
        })),
    ];
  }, [t, tagModels]);

  return (
    <>
    <div className='pricing-market-sidebar-shell rounded-2xl bg-white dark:bg-[#21262D]'>
      <div className='pricing-market-sidebar-header'>
        <div className='pricing-market-sidebar-heading'>
          <IconRadio style={{color:'#1CDFD5',fontSize:'20px'}} />
          <span className='text-[#1CDFD5] text-[12px] font-400'>{t('多维筛选')}</span>
        </div>
        {/* <Button theme='borderless' type='tertiary' onClick={handleResetFilters}>
          {t('重置')}
        </Button> */}
      </div>

      <Section title={t('模型供应商')}>
        {providerItems.map((item) => (
          <label key={item.value} className='pricing-market-filter-row'>
            <Checkbox checked={filterVendor === item.value} onChange={() => setFilterVendor(item.value)} />
            <span className='flex-1'>{item.label}</span>
            <div>{item.count}</div>
          </label>
        ))}
      </Section>

      <Section title={t('可用与使用分组')}>
        {groupItems.map((item) => (
          <label key={item.value} className='pricing-market-filter-row'>
            <Checkbox checked={filterGroup === item.value} onChange={() => handleGroupClick(item.value)} />
            <span className='flex-1'>{item.label}</span>
            <div>{item.ratio ? String(item.ratio) + 'x' : item.count}</div>
          </label>
        ))}
      </Section>

      <Section title={t('计费类型')}>
        {quotaItems.map((item) => (
          <label key={String(item.value)} className='pricing-market-filter-row'>
            <Radio checked={filterQuotaType === item.value} onChange={() => setFilterQuotaType(item.value)} />
            <span className='flex-1'>{item.label}</span>
            <div>{item.count}</div>
          </label>
        ))}
      </Section>

      <Section title={t('标签')}>
        <div className='pricing-market-filter-tags'>
          {tagItems.map((item) => (
            <button
              key={item.value}
              type='button'
              className={filterTag === item.value ? 'pricing-market-tag is-active' : 'pricing-market-tag'}
              onClick={() => setFilterTag(item.value)}
            >
              {item.label}
            </button>
          ))}
        </div>
      </Section>

      <Section title={t('端点类型')}>
        {endpointItems.map((item) => (
          <label key={item.value} className='pricing-market-filter-row'>
            <Checkbox
              checked={filterEndpointType === item.value}
              onChange={() => setFilterEndpointType(item.value)}
            />
            <span className='flex-1'>{item.label}</span>
            <div>{item.count}</div>
          </label>
        ))}
      </Section>
    </div>
    <div className='mt4 rounded-2xl px-4 py-4 mt-4' style={{backgroundImage:'linear-gradient(to right, #006D35, #21262D)',maxWidth:'256px'}}>
      <span className='text-[12px] text-[#1CDFD5]'>{t('开发者优惠')}</span>
      <p className='text-[18px] text-[#ffffff] font-black'>{t('DeepSeek V3')}</p>
      <p className='text-[18px] text-[#ffffff] font-black'>{t('全线特惠开启')}</p>
        <div className='bg-white inline-block p-2 px-4 rounded-lg text-[12px] cursor-pointer mt-4 font-medium'>{t('立即查看')}</div>
    </div>
    </>
  );
};

export default PricingSidebar;
