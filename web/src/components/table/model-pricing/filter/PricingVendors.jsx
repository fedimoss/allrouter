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
import SelectableButtonGroup from '../../../common/ui/SelectableButtonGroup';
import { getLobeHubIcon } from '../../../../helpers';

const PricingVendors = ({
  filterVendor,
  setFilterVendor,
  models = [],
  allModels = [],
  loading = false,
  t,
}) => {
  const allVendorInfo = React.useMemo(() => {
    const vendors = new Set();
    const vendorIcons = new Map();
    let hasUnknownVendor = false;

    (allModels.length > 0 ? allModels : models).forEach((model) => {
      if (model.vendor_name) {
        vendors.add(model.vendor_name);
        if (model.vendor_icon && !vendorIcons.has(model.vendor_name)) {
          vendorIcons.set(model.vendor_name, model.vendor_icon);
        }
      } else {
        hasUnknownVendor = true;
      }
    });

    return {
      vendors: Array.from(vendors).sort((a, b) => a.localeCompare(b)),
      vendorIcons,
      hasUnknownVendor,
    };
  }, [allModels, models]);

  const getVendorCount = React.useCallback(
    (vendor) => {
      if (vendor === 'all') return models.length;
      if (vendor === 'unknown') return models.filter((model) => !model.vendor_name).length;
      return models.filter((model) => model.vendor_name === vendor).length;
    },
    [models],
  );

  const items = React.useMemo(() => {
    const result = [
      {
        value: 'all',
        label: t('全部供应商'),
        tagCount: getVendorCount('all'),
      },
    ];

    allVendorInfo.vendors.forEach((vendor) => {
      result.push({
        value: vendor,
        label: vendor,
        icon: allVendorInfo.vendorIcons.get(vendor)
          ? getLobeHubIcon(allVendorInfo.vendorIcons.get(vendor), 16)
          : null,
        tagCount: getVendorCount(vendor),
      });
    });

    if (allVendorInfo.hasUnknownVendor) {
      result.push({
        value: 'unknown',
        label: t('未知供应商'),
        tagCount: getVendorCount('unknown'),
      });
    }

    return result;
  }, [allVendorInfo, getVendorCount, t]);

  return (
    <SelectableButtonGroup
      title={t('供应商')}
      items={items}
      activeValue={filterVendor}
      onChange={setFilterVendor}
      loading={loading}
      variant='violet'
      t={t}
    />
  );
};

export default PricingVendors;
