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
import { useTranslation } from 'react-i18next';
import { Card, Avatar, Typography, Tag, Space } from '@douyinfe/semi-ui';
import { IconInfoCircle } from '@douyinfe/semi-icons';
import { stringToColor } from '../../../../../helpers';

const { Text } = Typography;

const safeParseI18n = (value, fallback = {}) => {
  if (!value) return fallback;
  if (typeof value === 'object') return value;
  try {
    const parsed = JSON.parse(value);
    return parsed && typeof parsed === 'object' ? parsed : fallback;
  } catch {
    return fallback;
  }
};

const normalizeLangCandidates = (lang) => {
  const normalized = (lang || '').replace('_', '-');
  const base = normalized.split('-')[0];
  const candidates = [normalized, base];
  if (normalized === 'zh-CN' || normalized === 'zh-Hans' || base === 'zh') {
    candidates.push('zh-CN', 'zh');
  }
  if (normalized === 'zh-TW' || normalized === 'zh-Hant') {
    candidates.push('zh-TW', 'zh-CN', 'zh');
  }
  candidates.push('en', 'zh-CN', 'zh');
  return [...new Set(candidates.filter(Boolean))];
};

const pickLocalizedValue = (i18nValue, lang, fallback) => {
  const data = safeParseI18n(i18nValue);
  for (const key of normalizeLangCandidates(lang)) {
    const value = data[key];
    if (Array.isArray(value) && value.length > 0) return value;
    if (typeof value === 'string' && value.trim()) return value;
  }
  return fallback;
};

const ModelBasicInfo = ({ modelData, vendorsMap = {}, t }) => {
  const { i18n } = useTranslation();
  const currentLang = i18n.resolvedLanguage || i18n.language;

  const getModelDescription = () => {
    if (!modelData) return t('暂无模型描述');

    const localizedDescription = pickLocalizedValue(
      modelData.description_i18n,
      currentLang,
      modelData.description,
    );
    if (localizedDescription) return localizedDescription;

    if (modelData.vendor_description) {
      return t('供应商信息：') + modelData.vendor_description;
    }

    return t('暂无模型描述');
  };

  const getModelTags = () => {
    const tags = [];

    const localizedFeatures = pickLocalizedValue(
      modelData?.features_i18n,
      currentLang,
      null,
    );
    if (Array.isArray(localizedFeatures) && localizedFeatures.length > 0) {
      localizedFeatures.forEach((tag) => {
        const tagText = String(tag).trim();
        if (tagText) tags.push({ text: tagText, color: stringToColor(tagText) });
      });
      return tags;
    }

    if (modelData?.tags) {
      const customTags = modelData.tags.split(',').filter((tag) => tag.trim());
      customTags.forEach((tag) => {
        const tagText = tag.trim();
        tags.push({ text: tagText, color: stringToColor(tagText) });
      });
    }

    return tags;
  };

  return (
    <Card className='!rounded-2xl shadow-sm border-0 !mb-4'>
      <div className='flex items-center mb-4'>
        <Avatar size='small' color='blue' className='mr-2 shadow-md'>
          <IconInfoCircle size={16} />
        </Avatar>
        <div>
          <Text className='text-lg font-medium'>{t('基本信息')}</Text>
          <div className='text-xs text-gray-600'>
            {t('模型的详细描述和基本特性')}
          </div>
        </div>
      </div>
      <div className='text-gray-600'>
        <p className='mb-4'>{getModelDescription()}</p>
        {getModelTags().length > 0 && (
          <Space wrap>
            {getModelTags().map((tag, index) => (
              <Tag key={index} color={tag.color} shape='circle' size='small'>
                {tag.text}
              </Tag>
            ))}
          </Space>
        )}
      </div>
    </Card>
  );
};

export default ModelBasicInfo;
