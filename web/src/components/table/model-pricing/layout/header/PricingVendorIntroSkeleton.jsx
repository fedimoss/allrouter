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

import React, { memo } from 'react';
import { Skeleton } from '@douyinfe/semi-ui';

const PricingVendorIntroSkeleton = memo(({ isMobile = false }) => {
  if (isMobile) {
    return (
      <div className='pricing-market-mobile-toolbar'>
        <div className='pricing-market-mobile-search'>
          <Skeleton.Title style={{ width: '100%', height: 44, marginBottom: 0, borderRadius: 12 }} active />
        </div>
        <div className='pricing-market-mobile-actions'>
          <Skeleton.Button style={{ width: 84, height: 40 }} active />
          <Skeleton.Button style={{ width: 84, height: 40 }} active />
        </div>
      </div>
    );
  }

  return (
    <div className='pricing-market-top-shell'>
      <div className='pricing-market-recommend-section'>
        <div className='pricing-market-recommend-head'>
          <Skeleton.Title style={{ width: 120, height: 22, marginBottom: 0 }} active />
        </div>
        <div className='pricing-market-recommend-grid'>
          {Array.from({ length: 3 }).map((_, index) => (
            <div key={index} className='pricing-market-recommend-card'>
              <Skeleton.Title style={{ width: 120, height: 20 }} active />
              <Skeleton.Paragraph rows={2} style={{ marginTop: 12 }} active />
              <div className='pricing-market-recommend-tags'>
                <Skeleton.Button style={{ width: 90, height: 28 }} active />
                <Skeleton.Button style={{ width: 90, height: 28 }} active />
              </div>
            </div>
          ))}
        </div>
      </div>
      <div className='pricing-market-toolbar-shell'>
        <div className='pricing-market-toolbar'>
          <div className='pricing-market-toolbar-search'>
            <Skeleton.Title style={{ width: '100%', height: 44, marginBottom: 0, borderRadius: 12 }} active />
          </div>
          <Skeleton.Title style={{ width: 72, height: 18, marginBottom: 0 }} active />
          <div className='pricing-market-toolbar-actions'>
            <Skeleton.Button style={{ width: 96, height: 36 }} active />
            <Skeleton.Button style={{ width: 72, height: 36 }} active />
            <Skeleton.Button style={{ width: 132, height: 36 }} active />
            <Skeleton.Button style={{ width: 82, height: 36 }} active />
          </div>
        </div>
      </div>
    </div>
  );
});

PricingVendorIntroSkeleton.displayName = 'PricingVendorIntroSkeleton';

export default PricingVendorIntroSkeleton;
