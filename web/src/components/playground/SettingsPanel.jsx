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
import { Select } from '@douyinfe/semi-ui';
import { ChevronDown, PanelsTopLeft, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { renderGroupOption, selectFilter } from '../../helpers';
import ParameterControl from './ParameterControl';
import ImageUrlInput from './ImageUrlInput';
import ConfigManager from './ConfigManager';
import CustomRequestEditor from './CustomRequestEditor';

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

const SettingsPanel = ({
  inputs,
  parameterEnabled,
  models,
  groups,
  styleState,
  showDebugPanel,
  customRequestMode,
  customRequestBody,
  onInputChange,
  onParameterToggle,
  onCloseSettings,
  onConfigImport,
  onConfigReset,
  onCustomRequestModeChange,
  onCustomRequestBodyChange,
  previewPayload,
  messages,
}) => {
  const { t } = useTranslation();
  console.log('groups==', groups);

  const currentConfig = {
    inputs,
    parameterEnabled,
    showDebugPanel,
    customRequestMode,
    customRequestBody,
  };

  return (
    <div className='playground-v2-settings h-full'>
      {styleState.isMobile && (
        <div className='playground-v2-settings-header'>
          <div>
            <h2 className='playground-v2-panel-title'>{t('操练场设置')}</h2>
            <div className='playground-v2-panel-subtitle'>
              {t('按模型、分组和采样参数实时调整对话请求。')}
            </div>
          </div>

          <button
            type='button'
            className='playground-v2-icon-button'
            onClick={onCloseSettings || (() => {})}
            aria-label={t('关闭')}
          >
            <X size={16} />
          </button>
        </div>
      )}

      <div className='playground-v2-settings-body'>
        <div className='playground-v2-settings-stack'>
          <div className='playground-v2-settings-section'>
            <div className='playground-v2-section-kicker'>Model Config</div>
            <div className='playground-v2-settings-stack' style={{ gap: '16px' }}>
              <div className='playground-v2-field'>
                <label className='playground-v2-field-label'>
                  {t('模型')}
                </label>
                <Select
                  placeholder={t('请选择模型')}
                  selection
                  filter={selectFilter}
                  searchPosition='dropdown'
                  optionList={models}
                  value={inputs.model}
                  disabled={customRequestMode}
                  className='playground-v2-select'
                  dropdownClassName='playground-v2-select-dropdown'
                  dropdownStyle={{ maxWidth: '100%' }}
                  arrowIcon={<ChevronDown size={16} strokeWidth={2.25} />}
                  onChange={(value) => onInputChange('model', value)}
                />
              </div>

              <div className='playground-v2-field'>
                <label className='playground-v2-field-label'>
                  {t('分组')}
                </label>
                <Select
                  placeholder={t('请选择分组')}
                  selection
                  filter={selectFilter}
                  searchPosition='dropdown'
                  optionList={groups}
                  value={inputs.group}
                  disabled={customRequestMode}
                  className='playground-v2-select'
                  dropdownClassName='playground-v2-select-dropdown'
                  dropdownStyle={{ maxWidth: '100%' }}
                  arrowIcon={<ChevronDown size={16} strokeWidth={2.25} />}
                  renderOptionItem={renderGroupOption}
                  onChange={(value) => onInputChange('group', value)}
                />
              </div>
            </div>
          </div>

          <ImageUrlInput
            imageUrls={inputs.imageUrls}
            imageEnabled={inputs.imageEnabled}
            disabled={customRequestMode}
            onImageUrlsChange={(urls) => onInputChange('imageUrls', urls)}
            onImageEnabledChange={(enabled) =>
              onInputChange('imageEnabled', enabled)
            }
          />

          <ParameterControl
            inputs={inputs}
            parameterEnabled={parameterEnabled}
            onInputChange={onInputChange}
            onParameterToggle={onParameterToggle}
            disabled={customRequestMode}
          />

          <CustomRequestEditor
            customRequestMode={customRequestMode}
            customRequestBody={customRequestBody}
            onCustomRequestModeChange={onCustomRequestModeChange}
            onCustomRequestBodyChange={onCustomRequestBodyChange}
            defaultPayload={previewPayload}
          />

          <div className='playground-v2-settings-section'>
            <div className='playground-v2-section-kicker'>Configuration</div>
            <ConfigManager
              currentConfig={currentConfig}
              onConfigImport={onConfigImport}
              onConfigReset={onConfigReset}
              styleState={styleState}
              messages={messages}
            />
          </div>

          <div className='playground-v2-settings-section'>
            <div className='playground-v2-toggle-row'>
              <div>
                <div className='playground-v2-field-label'>
                  <PanelsTopLeft size={16} />
                  {t('流式输出')}
                </div>
                <div className='playground-v2-field-hint'>
                  {t('开启后将实时接收响应，并在调试面板中查看 SSE 数据流。')}
                </div>
              </div>

              <ToggleSwitch
                checked={inputs.stream}
                disabled={customRequestMode}
                onChange={(checked) => onInputChange('stream', checked)}
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default SettingsPanel;
