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

const getNumericValue = (value) => {
  if (value === null || value === undefined || value === '') return null;
  const numericValue = Number(value);
  return Number.isFinite(numericValue) ? numericValue : null;
};

const getSortValue = (model, field, weights) => {
  switch (field) {
    case 'model_ratio':
    case 'completion_ratio':
    case 'model_price':
    case 'quota_type':
      return getNumericValue(model[field]);
    case 'sort_weight':
      return getNumericValue(weights?.[model.model_name]);
    case 'supported_endpoint_types':
      return Array.isArray(model.supported_endpoint_types) &&
        model.supported_endpoint_types.length
        ? model.supported_endpoint_types.join(', ').toLowerCase()
        : null;
    default: {
      const value = model[field];
      return typeof value === 'string' && value.trim()
        ? value.trim().toLowerCase()
        : null;
    }
  }
};

const compareValues = (a, b, direction) => {
  if (a === null && b !== null) return 1;
  if (a !== null && b === null) return -1;
  if (a === null && b === null) return 0;

  const comparison =
    typeof a === 'number' && typeof b === 'number'
      ? a - b
      : String(a).localeCompare(String(b), undefined, {
          numeric: true,
          sensitivity: 'base',
        });
  return direction === 'desc' ? -comparison : comparison;
};

export const PRICING_SORT_FIELDS = [
  { value: 'model_name', label: '模型名称' },
  { value: 'vendor_name', label: '供应商' },
  { value: 'model_ratio', label: '模型倍率' },
  { value: 'completion_ratio', label: '补全倍率' },
  { value: 'model_price', label: '模型价格' },
  { value: 'quota_type', label: '计费类型' },
  { value: 'billing_mode', label: '计费模式' },
  { value: 'tags', label: '标签' },
  { value: 'supported_endpoint_types', label: '可用端点类型' },
  { value: 'sort_weight', label: '排序大小' },
];

export const sortPricingModels = (
  models = [],
  sortMode = 'hot',
  criteria = [],
  weights = {},
) => {
  const source = Array.isArray(models) ? models : [];
  const indexedModels = source.map((model, index) => ({ model, index }));

  if (sortMode === 'custom') {
    const validFields = new Set(PRICING_SORT_FIELDS.map(({ value }) => value));
    const seenFields = new Set();
    const sortCriteria = criteria.filter(({ field }) => {
      if (!validFields.has(field) || seenFields.has(field)) return false;
      seenFields.add(field);
      return true;
    });

    if (sortCriteria.length === 0) return [...source];

    return indexedModels
      .sort((a, b) => {
        for (const { field, direction = 'asc' } of sortCriteria) {
          const comparison = compareValues(
            getSortValue(a.model, field, weights),
            getSortValue(b.model, field, weights),
            direction,
          );
          if (comparison !== 0) return comparison;
        }
        return a.index - b.index;
      })
      .map(({ model }) => model);
  }

  const compareByName = (a, b) =>
    String(a.model.model_name || '').localeCompare(
      String(b.model.model_name || ''),
    );

  if (sortMode === 'value') {
    return indexedModels
      .sort(
        (a, b) =>
          (getNumericValue(a.model.model_ratio) ?? Number.MAX_SAFE_INTEGER) -
            (getNumericValue(b.model.model_ratio) ?? Number.MAX_SAFE_INTEGER) ||
          a.index - b.index,
      )
      .map(({ model }) => model);
  }

  if (sortMode === 'latest') {
    return indexedModels
      .sort((a, b) => {
        const aNew = a.model.tags?.toLowerCase().includes('new') ? 1 : 0;
        const bNew = b.model.tags?.toLowerCase().includes('new') ? 1 : 0;
        return bNew - aNew || compareByName(a, b) || a.index - b.index;
      })
      .map(({ model }) => model);
  }

  return indexedModels
    .sort((a, b) => {
      const aHot = a.model.tags?.toLowerCase().includes('hot') ? 1 : 0;
      const bHot = b.model.tags?.toLowerCase().includes('hot') ? 1 : 0;
      return bHot - aHot || compareByName(a, b) || a.index - b.index;
    })
    .map(({ model }) => model);
};
