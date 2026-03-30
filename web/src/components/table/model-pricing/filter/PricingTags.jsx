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

const PricingTags = ({
  filterTag,
  setFilterTag,
  models = [],
  allModels = [],
  loading = false,
  t,
}) => {
  const allTags = React.useMemo(() => {
    const tagSet = new Set();

    (allModels.length > 0 ? allModels : models).forEach((model) => {
      if (model.tags) {
        model.tags
          .split(/[,;|]+/)
          .map((tag) => tag.trim())
          .filter(Boolean)
          .forEach((tag) => tagSet.add(tag.toLowerCase()));
      }
    });

    return Array.from(tagSet).sort((a, b) => a.localeCompare(b));
  }, [allModels, models]);

  const getTagCount = React.useCallback(
    (tag) => {
      if (tag === 'all') return models.length;
      const tagLower = tag.toLowerCase();
      return models.filter((model) => {
        if (!model.tags) return false;
        return model.tags.toLowerCase().split(/[,;|]+/).map((value) => value.trim()).includes(tagLower);
      }).length;
    },
    [models],
  );

  const items = React.useMemo(() => [
    { value: 'all', label: t('全部标签'), tagCount: getTagCount('all') },
    ...allTags.map((tag) => ({ value: tag, label: tag, tagCount: getTagCount(tag) })),
  ], [allTags, getTagCount, t]);

  return (
    <SelectableButtonGroup
      title={t('标签')}
      items={items}
      activeValue={filterTag}
      onChange={setFilterTag}
      loading={loading}
      variant='rose'
      t={t}
    />
  );
};

export default PricingTags;
