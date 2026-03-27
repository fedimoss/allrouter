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
import PricingSidebar from './PricingSidebar';
import PricingContent from './content/PricingContent';
import ModelDetailSideSheet from '../modal/ModelDetailSideSheet';
import { useModelPricingData } from '../../../../hooks/model-pricing/useModelPricingData';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import './PricingPage.css';

const PricingPage = () => {
  const pricingData = useModelPricingData();
  const { Sider, Content } = Layout;
  const isMobile = useIsMobile();
  const [showRatio, setShowRatio] = React.useState(false);
  const [viewMode, setViewMode] = React.useState('card');
  const [sortMode, setSortMode] = React.useState('hot');

  const sortedModels = React.useMemo(() => {
    const models = [...(pricingData.filteredModels || [])];

    if (sortMode === 'value') {
      return models.sort((a, b) => {
        const aRatio = typeof a.model_ratio === 'number' ? a.model_ratio : Number.MAX_SAFE_INTEGER;
        const bRatio = typeof b.model_ratio === 'number' ? b.model_ratio : Number.MAX_SAFE_INTEGER;
        return aRatio - bRatio;
      });
    }

    if (sortMode === 'latest') {
      return models.sort((a, b) => {
        const aNew = a.tags?.toLowerCase().includes('new') ? 1 : 0;
        const bNew = b.tags?.toLowerCase().includes('new') ? 1 : 0;
        if (aNew !== bNew) return bNew - aNew;
        return String(a.model_name || '').localeCompare(String(b.model_name || ''));
      });
    }

    return models.sort((a, b) => {
      const aHot = a.tags?.toLowerCase().includes('hot') ? 1 : 0;
      const bHot = b.tags?.toLowerCase().includes('hot') ? 1 : 0;
      if (aHot !== bHot) return bHot - aHot;
      return String(a.model_name || '').localeCompare(String(b.model_name || ''));
    });
  }, [pricingData.filteredModels, sortMode]);

  const allProps = {
    ...pricingData,
    filteredModels: sortedModels,
    showRatio,
    setShowRatio,
    viewMode,
    setViewMode,
    sortMode,
    setSortMode,
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
