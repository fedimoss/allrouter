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

const PricingGroups = ({
  filterGroup,
  setFilterGroup,
  usableGroup = {},
  groupRatio = {},
  models = [],
  loading = false,
  t,
}) => {
  const groups = ['all', ...Object.keys(usableGroup).filter((key) => key !== '')];

  const items = groups.map((group) => {
    const modelCount = group === 'all'
      ? models.length
      : models.filter((model) => model.enable_groups && model.enable_groups.includes(group)).length;

    return {
      value: group,
      label: group === 'all' ? t('全部分组') : group,
      tagCount: group === 'all' ? modelCount : `${groupRatio[group] ?? 1}x`,
    };
  });

  return (
    <SelectableButtonGroup
      title={t('可用令牌分组')}
      items={items}
      activeValue={filterGroup}
      onChange={setFilterGroup}
      loading={loading}
      variant='teal'
      t={t}
    />
  );
};

export default PricingGroups;
