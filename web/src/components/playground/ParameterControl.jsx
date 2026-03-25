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
import { Check } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const formatNumber = (value, precision) =>
  Number(value ?? 0)
    .toFixed(precision)
    .replace(/\.?0+$/, '');

const sliderConfigs = [
  {
    key: 'temperature',
    label: 'Temperature',
    description: '控制输出的随机性和创造性',
    min: 0,
    max: 2,
    step: 0.1,
    formatValue: (value) => formatNumber(value, 1),
  },
  {
    key: 'top_p',
    label: 'Top P',
    description: '核采样，控制词汇选择的多样性',
    min: 0,
    max: 1,
    step: 0.05,
    formatValue: (value) => formatNumber(value, 2),
  },
  {
    key: 'frequency_penalty',
    label: 'Freq. Penalty',
    description: '频率惩罚，减少重复词汇的出现',
    min: -2,
    max: 2,
    step: 0.1,
    formatValue: (value) => formatNumber(value, 1),
  },
  {
    key: 'presence_penalty',
    label: 'Pres. Penalty',
    description: '存在惩罚，鼓励讨论新话题',
    min: -2,
    max: 2,
    step: 0.1,
    formatValue: (value) => formatNumber(value, 1),
  },
];

const textConfigs = [
  {
    key: 'max_tokens',
    label: 'Max Tokens',
    description: '控制单次回复的最大 token 数量',
    placeholder: '4096',
    inputMode: 'numeric',
  },
  {
    key: 'seed',
    label: 'Seed',
    description: '可选，用于复现结果',
    placeholder: '随机种子',
    inputMode: 'numeric',
  },
];

const ToggleCheckbox = ({ checked, disabled, onClick }) => {
  return (
    <button
      type='button'
      className='playground-v2-checkbox'
      data-active={checked}
      data-disabled={disabled}
      onClick={onClick}
      disabled={disabled}
      aria-pressed={checked}
    >
      {checked ? <Check size={12} /> : null}
    </button>
  );
};

const SliderParameter = ({
  config,
  value,
  enabled,
  disabled,
  onInputChange,
  onParameterToggle,
  t,
}) => {
  const numericValue = Number(value ?? 0);
  const progress =
    ((numericValue - config.min) / (config.max - config.min || 1)) * 100;

  return (
    <div
      className='playground-v2-parameter-card'
      data-disabled={!enabled || disabled}
    >
      <div className='playground-v2-parameter-header'>
        <div className='playground-v2-parameter-label'>{config.label}</div>

        <div className='flex items-center gap-3'>
          <span className='playground-v2-value-badge'>
            {config.formatValue(numericValue)}
          </span>
          <ToggleCheckbox
            checked={enabled}
            disabled={disabled}
            onClick={() => onParameterToggle(config.key)}
          />
        </div>
      </div>

      <input
        type='range'
        className='playground-v2-range'
        min={config.min}
        max={config.max}
        step={config.step}
        value={numericValue}
        disabled={!enabled || disabled}
        style={{ '--range-progress': `${Math.max(0, Math.min(progress, 100))}%` }}
        onChange={(event) =>
          onInputChange(config.key, Number(event.target.value))
        }
      />

      <div className='playground-v2-parameter-desc'>{t(config.description)}</div>
    </div>
  );
};

const TextParameter = ({
  config,
  value,
  enabled,
  disabled,
  onInputChange,
  onParameterToggle,
  t,
}) => {
  return (
    <div
      className='playground-v2-parameter-card'
      data-disabled={!enabled || disabled}
    >
      <div className='playground-v2-parameter-header'>
        <div className='playground-v2-parameter-label'>{config.label}</div>

        <ToggleCheckbox
          checked={enabled}
          disabled={disabled}
          onClick={() => onParameterToggle(config.key)}
        />
      </div>

      <input
        type='text'
        className='playground-v2-text-input'
        inputMode={config.inputMode}
        placeholder={t(config.placeholder)}
        disabled={!enabled || disabled}
        value={value ?? ''}
        onChange={(event) => {
          const nextValue = event.target.value;

          if (config.key === 'seed') {
            onInputChange(config.key, nextValue === '' ? null : nextValue);
            return;
          }

          onInputChange(config.key, nextValue);
        }}
      />

      <div className='playground-v2-parameter-desc'>{t(config.description)}</div>
    </div>
  );
};

const ParameterControl = ({
  inputs,
  parameterEnabled,
  onInputChange,
  onParameterToggle,
  disabled = false,
}) => {
  const { t } = useTranslation();

  return (
    <div className='playground-v2-settings-section'>
      <div className='playground-v2-section-kicker'>Parameters</div>
      <div className='playground-v2-settings-stack'>
        {sliderConfigs.map((config) => (
          <SliderParameter
            key={config.key}
            config={config}
            value={inputs[config.key]}
            enabled={parameterEnabled[config.key]}
            disabled={disabled}
            onInputChange={onInputChange}
            onParameterToggle={onParameterToggle}
            t={t}
          />
        ))}

        {textConfigs.map((config) => (
          <TextParameter
            key={config.key}
            config={config}
            value={inputs[config.key]}
            enabled={parameterEnabled[config.key]}
            disabled={disabled}
            onInputChange={onInputChange}
            onParameterToggle={onParameterToggle}
            t={t}
          />
        ))}
      </div>
    </div>
  );
};

export default ParameterControl;
