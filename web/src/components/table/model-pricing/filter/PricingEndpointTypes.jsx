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

const PricingEndpointTypes = ({
  filterEndpointType,
  setFilterEndpointType,
  models = [],
  allModels = [],
  loading = false,
  t,
}) => {
  const allEndpointTypes = React.useMemo(() => {
    const endpointTypes = new Set();
    (allModels.length > 0 ? allModels : models).forEach((model) => {
      if (Array.isArray(model.supported_endpoint_types)) {
        model.supported_endpoint_types.forEach((endpoint) => endpointTypes.add(endpoint));
      }
    });
    return Array.from(endpointTypes).sort((a, b) => String(a).localeCompare(String(b)));
  }, [allModels, models]);

  const getEndpointTypeCount = React.useCallback(
    (endpointType) => {
      if (endpointType === 'all') return models.length;
      return models.filter(
        (model) => Array.isArray(model.supported_endpoint_types) && model.supported_endpoint_types.includes(endpointType),
      ).length;
    },
    [models],
  );

  const items = [
    { value: 'all', label: t('全部端点'), tagCount: getEndpointTypeCount('all') },
    ...allEndpointTypes.map((endpointType) => ({
      value: endpointType,
      label: endpointType,
      tagCount: getEndpointTypeCount(endpointType),
    })),
  ];

  return (
    <SelectableButtonGroup
      title={t('端点类型')}
      items={items}
      activeValue={filterEndpointType}
      onChange={setFilterEndpointType}
      loading={loading}
      variant='green'
      t={t}
    />
  );
};

export default PricingEndpointTypes;
