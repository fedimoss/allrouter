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
import { Layout, ImagePreview } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../../helpers';
import PricingSidebar from './PricingSidebar';
import PricingContent from './content/PricingContent';
import ModelDetailSideSheet from '../modal/ModelDetailSideSheet';
import { useModelPricingData } from '../../../../hooks/model-pricing/useModelPricingData';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import { sortPricingModels } from './pricingSort';
import './PricingPage.css';

const PricingPage = () => {
  const pricingData = useModelPricingData();
  const { Sider, Content } = Layout;
  const isMobile = useIsMobile();
  const [showRatio, setShowRatio] = React.useState(false);
  const [viewMode, setViewMode] = React.useState('card');
  const [sortMode, setSortMode] = React.useState('hot');
  const [sortCriteria, setSortCriteria] = React.useState([
    { field: 'model_name', direction: 'asc' },
  ]);
  const [sortWeights, setSortWeights] = React.useState({});
  const user = pricingData.userState?.user;
  const allowCustomSort =
    Number(user?.role) >= 10 || user?.is_provider_owner === true;
  const pricingSortConfig = pricingData.pricingSortConfig || {
    mode: 'hot',
    criteria: [],
  };

  React.useEffect(() => {
    const mode = pricingSortConfig.mode || 'hot';
    const criteria = Array.isArray(pricingSortConfig.criteria)
      ? pricingSortConfig.criteria
      : [];
    setSortMode(mode);
    setSortCriteria(
      criteria.length > 0
        ? criteria
        : [{ field: 'model_name', direction: 'asc' }],
    );
    setSortWeights(
      pricingSortConfig.weights && typeof pricingSortConfig.weights === 'object'
        ? pricingSortConfig.weights
        : {},
    );
  }, [pricingSortConfig]);

  const [sortConfigSaving, setSortConfigSaving] = React.useState(false);
  const saveSortConfig = React.useCallback(
    async (mode = sortMode, criteria = sortCriteria, weights = sortWeights) => {
      setSortConfigSaving(true);
      try {
        const res = await API.put('/api/pricing/sort', {
          mode,
          criteria,
          weights,
        });
        if (!res.data?.success) {
          showError(res.data?.message || pricingData.t('保存失败'));
          return false;
        }
        const savedConfig = res.data.data || { mode, criteria, weights };
        pricingData.setPricingSortConfig(savedConfig);
        showSuccess(pricingData.t('保存成功'));
        return true;
      } catch (error) {
        showError(error);
        return false;
      } finally {
        setSortConfigSaving(false);
      }
    },
    [pricingData, sortCriteria, sortMode, sortWeights],
  );

  const resetSortConfig = React.useCallback(() => {
    setSortMode('hot');
    setSortCriteria([{ field: 'model_name', direction: 'asc' }]);
    setSortWeights({});
    return saveSortConfig('hot', [], {});
  }, [saveSortConfig]);

  const sortedModels = React.useMemo(() => {
    const publicCustomSort = pricingSortConfig.mode === 'custom';
    const effectiveSortMode =
      !allowCustomSort && publicCustomSort ? 'custom' : sortMode;
    const effectiveCriteria =
      !allowCustomSort && publicCustomSort
        ? pricingSortConfig.criteria || []
        : sortCriteria;
    const effectiveWeights = !allowCustomSort
      ? pricingSortConfig.weights || {}
      : sortWeights;
    return sortPricingModels(
      pricingData.filteredModels,
      effectiveSortMode,
      effectiveCriteria,
      effectiveWeights,
    );
  }, [
    allowCustomSort,
    pricingData.filteredModels,
    pricingSortConfig,
    sortCriteria,
    sortWeights,
    sortMode,
  ]);

  const allProps = {
    ...pricingData,
    filteredModels: sortedModels,
    showRatio,
    setShowRatio,
    viewMode,
    setViewMode,
    sortMode,
    setSortMode,
    allowCustomSort,
    sortCriteria,
    setSortCriteria,
    sortWeights,
    setSortWeights,
    pricingSortConfig,
    sortConfigSaving,
    saveSortConfig,
    resetSortConfig,
  };

  return (
    <div className='pricing-market'>
      <Layout className='pricing-layout'>
        {!isMobile && (
          <Sider className='pricing-scroll-hide pricing-sidebar'>
            <PricingSidebar {...allProps} />
          </Sider>
        )}

        <Content className='pricing-scroll-hide pricing-content'>
          <PricingContent
            {...allProps}
            isMobile={isMobile}
            sidebarProps={allProps}
          />
        </Content>
      </Layout>

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
        showRatio={allProps.showRatio}
        vendorsMap={pricingData.vendorsMap}
        endpointMap={pricingData.endpointMap}
        autoGroups={pricingData.autoGroups}
        t={pricingData.t}
      />
    </div>
  );
};

export default PricingPage;
