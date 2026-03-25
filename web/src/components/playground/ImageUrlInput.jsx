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

import React, { useRef } from 'react';
import { Toast } from '@douyinfe/semi-ui';
import { Image, Plus, Upload, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const readFileAsDataUrl = (file) =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result);
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(file);
  });

const ToggleSwitch = ({ checked, disabled, onChange }) => {
  return (
    <button
      type='button'
      className='playground-v2-switch'
      data-active={checked}
      data-disabled={disabled}
      disabled={disabled}
      onClick={() => !disabled && onChange(!checked)}
      aria-pressed={checked}
    />
  );
};

const ImageUrlInput = ({
  imageUrls,
  imageEnabled,
  onImageUrlsChange,
  onImageEnabledChange,
  disabled = false,
}) => {
  const { t } = useTranslation();
  const fileInputRef = useRef(null);

  const normalizedUrls = Array.isArray(imageUrls) ? imageUrls : [];
  const activeCount = normalizedUrls.filter((url) => url && url.trim()).length;

  const handleAddImageUrl = () => {
    onImageUrlsChange([...(normalizedUrls.length ? normalizedUrls : []), '']);
  };

  const handleUpdateImageUrl = (index, value) => {
    const nextUrls = [...normalizedUrls];
    nextUrls[index] = value;
    onImageUrlsChange(nextUrls);
  };

  const handleRemoveImageUrl = (index) => {
    const nextUrls = normalizedUrls.filter((_, currentIndex) => currentIndex !== index);
    onImageUrlsChange(nextUrls);
  };

  const handleFileUpload = async (event) => {
    const files = Array.from(event.target.files || []);

    if (files.length === 0) {
      return;
    }

    try {
      const uploadedImages = await Promise.all(files.map(readFileAsDataUrl));
      onImageUrlsChange([...(normalizedUrls || []), ...uploadedImages]);
      Toast.success({
        content: t('图片已添加'),
        duration: 2,
      });
    } catch (error) {
      console.error('Failed to load uploaded images:', error);
      Toast.error({
        content: t('图片加载失败'),
        duration: 2,
      });
    } finally {
      event.target.value = '';
    }
  };

  return (
    <div className='playground-v2-settings-section'>
      <div className='playground-v2-toggle-row'>
        <div>
          <div className='playground-v2-field-label'>
            <Image size={16} />
            {t('多模态输入')}
          </div>
          <div className='playground-v2-field-hint'>
            {disabled
              ? t('自定义请求体模式下，图片输入不会参与请求体生成。')
              : t('支持图片 URL、文件上传和在输入框中直接粘贴图片。')}
          </div>
        </div>

        <ToggleSwitch
          checked={imageEnabled}
          disabled={disabled}
          onChange={onImageEnabledChange}
        />
      </div>

      {imageEnabled && (
        <div className='playground-v2-settings-stack mt-4'>
          {normalizedUrls.length > 0 && (
            <div className='playground-v2-image-list'>
            {normalizedUrls.map((url, index) => (
              <div key={`${index}-${url}`} className='playground-v2-image-row'>
                <input
                  type='text'
                  className='playground-v2-text-input'
                  placeholder={`https://example.com/image-${index + 1}.jpg`}
                  disabled={disabled}
                  value={url}
                  onChange={(event) =>
                    handleUpdateImageUrl(index, event.target.value)
                  }
                />

                <button
                  type='button'
                  className='playground-v2-image-remove-button'
                  onClick={() => handleRemoveImageUrl(index)}
                  disabled={disabled}
                  aria-label={t('删除')}
                >
                  <X size={14} />
                </button>
              </div>
            ))}
          </div>
          )}

          <div className='playground-v2-image-actions'>
            <button
              type='button'
              className='playground-v2-image-add-button'
              onClick={handleAddImageUrl}
              disabled={disabled}
            >
              <Plus size={14} />
              {t('添加图片')}
            </button>

            <button
              type='button'
              className='playground-v2-image-upload-button'
              onClick={() => fileInputRef.current?.click()}
              disabled={disabled}
            >
              <Upload size={14} />
              {t('上传图片（占位）')}
            </button>

            {activeCount > 0 && (
              <span className='playground-v2-outline-pill'>
                {t('已附加 {{count}} 张图片', { count: activeCount })}
              </span>
            )}
          </div>

          <input
            ref={fileInputRef}
            type='file'
            accept='image/*'
            multiple
            className='hidden'
            onChange={handleFileUpload}
          />
        </div>
      )}
    </div>
  );
};

export default ImageUrlInput;
