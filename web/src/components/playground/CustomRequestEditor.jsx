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

import React, { useEffect, useMemo, useState } from 'react';
import { Check, Code2, RotateCcw, TriangleAlert, Wand2, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const ToggleSwitch = ({ checked, onChange }) => {
  return (
    <button
      type='button'
      className='playground-v2-switch'
      data-active={checked}
      onClick={() => onChange(!checked)}
      aria-pressed={checked}
    />
  );
};

const buildTemplate = (type, defaultPayload) => {
  if (type === 'default') {
    return defaultPayload ? JSON.stringify(defaultPayload, null, 2) : '';
  }

  if (type === 'openai') {
    return JSON.stringify(
      {
        model: 'gpt-4o',
        messages: [],
        stream: true,
        temperature: 0.7,
      },
      null,
      2,
    );
  }

  if (type === 'anthropic') {
    return JSON.stringify(
      {
        model: 'claude-3-5-sonnet',
        messages: [],
        stream: true,
        temperature: 0.7,
      },
      null,
      2,
    );
  }

  return '';
};

const CustomRequestEditor = ({
  customRequestMode,
  customRequestBody,
  onCustomRequestModeChange,
  onCustomRequestBodyChange,
  defaultPayload,
}) => {
  const { t } = useTranslation();
  const [localValue, setLocalValue] = useState(customRequestBody || '');
  const [isValid, setIsValid] = useState(true);
  const [errorMessage, setErrorMessage] = useState('');

  const defaultTemplate = useMemo(
    () => buildTemplate('default', defaultPayload),
    [defaultPayload],
  );

  useEffect(() => {
    if (customRequestMode && (!customRequestBody || customRequestBody.trim() === '')) {
      setLocalValue(defaultTemplate);
      onCustomRequestBodyChange(defaultTemplate);
    }
  }, [
    customRequestMode,
    customRequestBody,
    defaultTemplate,
    onCustomRequestBodyChange,
  ]);

  useEffect(() => {
    if ((customRequestBody || '') !== localValue) {
      setLocalValue(customRequestBody || '');
      validateJson(customRequestBody || '');
    }
  }, [customRequestBody]);

  const validateJson = (value) => {
    if (!value.trim()) {
      setIsValid(true);
      setErrorMessage('');
      return true;
    }

    try {
      JSON.parse(value);
      setIsValid(true);
      setErrorMessage('');
      return true;
    } catch (error) {
      setIsValid(false);
      setErrorMessage(`${t('JSON格式错误')}: ${error.message}`);
      return false;
    }
  };

  const handleValueChange = (value) => {
    setLocalValue(value);
    validateJson(value);
    onCustomRequestBodyChange(value);
  };

  const handleFormatJson = () => {
    try {
      const parsed = JSON.parse(localValue);
      const formatted = JSON.stringify(parsed, null, 2);
      setLocalValue(formatted);
      setIsValid(true);
      setErrorMessage('');
      onCustomRequestBodyChange(formatted);
    } catch (error) {
      validateJson(localValue);
    }
  };

  const handleReset = () => {
    setLocalValue(defaultTemplate);
    validateJson(defaultTemplate);
    onCustomRequestBodyChange(defaultTemplate);
  };

  const applyTemplate = (type) => {
    const nextValue = buildTemplate(type, defaultPayload);
    setLocalValue(nextValue);
    validateJson(nextValue);
    onCustomRequestBodyChange(nextValue);
  };

  return (
    <div className='playground-v2-settings-section'>
      <div className='playground-v2-toggle-row'>
        <div>
          <div className='playground-v2-field-label'>
            <Code2 size={16} />
            {t('自定义请求体（JSON）')}
          </div>
          <div className='playground-v2-field-hint'>
            {t(
              '启用后，界面参数仅作为参考，最终会以这里编辑的 JSON 直接发送请求。',
            )}
          </div>
        </div>

        <ToggleSwitch
          checked={customRequestMode}
          onChange={onCustomRequestModeChange}
        />
      </div>

      {customRequestMode && (
        <div className='playground-v2-settings-stack mt-4'>
          <div className='playground-v2-warning'>
            <div className='flex items-start gap-3'>
              <TriangleAlert size={16} className='mt-0.5 flex-shrink-0' />
              <span>
                {t('请确保 JSON 有效，且与目标模型的接口字段兼容。')}
              </span>
            </div>
          </div>

          <div className='playground-v2-json-actions'>
            <button
              type='button'
              className='playground-v2-button-secondary'
              onClick={() => applyTemplate('default')}
            >
              <Wand2 size={14} />
              {t('默认模板')}
            </button>

            <button
              type='button'
              className='playground-v2-button-secondary'
              onClick={() => applyTemplate('openai')}
            >
              {t('OpenAI 兼容')}
            </button>

            <button
              type='button'
              className='playground-v2-button-secondary'
              onClick={() => applyTemplate('anthropic')}
            >
              {t('Anthropic')}
            </button>
          </div>

          <div className='playground-v2-inline-row'>
            <span
              className={
                isValid ? 'playground-v2-pill' : 'playground-v2-outline-pill'
              }
            >
              {isValid ? <Check size={14} /> : <X size={14} />}
              {isValid ? t('JSON 可发送') : t('JSON 有错误')}
            </span>

            <div className='playground-v2-json-actions'>
              <button
                type='button'
                className='playground-v2-button-secondary'
                onClick={handleFormatJson}
                disabled={!isValid}
              >
                {t('格式化')}
              </button>

              <button
                type='button'
                className='playground-v2-button-secondary'
                onClick={handleReset}
              >
                <RotateCcw size={14} />
                {t('重置')}
              </button>
            </div>
          </div>

          <textarea
            className='playground-v2-json-editor'
            value={localValue}
            onChange={(event) => handleValueChange(event.target.value)}
            placeholder='{"model":"gpt-4o","messages":[],"stream":true}'
          />

          <div className='playground-v2-inline-row'>
            <span className='playground-v2-field-hint'>
              {errorMessage || t('长度：{{length}} 字符', { length: localValue.length })}
            </span>
            <span className='playground-v2-field-hint'>
              {t('默认预览可作为快速起点')}
            </span>
          </div>
        </div>
      )}
    </div>
  );
};

export default CustomRequestEditor;
