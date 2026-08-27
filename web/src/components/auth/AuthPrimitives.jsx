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

import React, { useEffect, useState } from 'react';
import { Eye, EyeOff, LoaderCircle, X } from 'lucide-react';

export const Button = ({
  children,
  className = '',
  disabled = false,
  htmlType = 'button',
  icon,
  loading = false,
  onClick,
  size: _size,
  theme: _theme,
  type: _visualType,
  ...props
}) => (
  <button
    {...props}
    type={htmlType}
    className={`auth-native-button auth-native-button-${_theme || 'default'}-${_visualType || 'default'} ${className}`}
    disabled={disabled || loading}
    onClick={onClick}
  >
    {loading ? <LoaderCircle className='auth-native-spinner' size={16} /> : icon}
    {children}
  </button>
);

export const Card = ({ children, className = '' }) => (
  <div className={`auth-native-card ${className}`}>{children}</div>
);

export const Checkbox = ({ checked, children, onChange }) => (
  <label className='auth-native-checkbox'>
    <input type='checkbox' checked={checked} onChange={onChange} />
    <span className='auth-native-checkbox-mark' aria-hidden='true' />
    <span>{children}</span>
  </label>
);

export const Divider = ({ children }) => (
  <div className='auth-native-divider'><span>{children}</span></div>
);

export const Icon = ({ svg, children, style }) => (
  <span className='auth-native-icon' style={style}>{svg || children}</span>
);

export const Title = ({ children, heading = 3, className = '' }) => {
  const Tag = `h${Math.min(Math.max(heading, 1), 6)}`;
  return <Tag className={className}>{children}</Tag>;
};

export const Text = ({ children, className = '' }) => (
  <span className={className}>{children}</span>
);

const AuthInput = ({
  autoComplete,
  field: _field,
  label,
  mode,
  name,
  onChange,
  placeholder,
  prefix,
  suffix,
  type,
  value,
  ...inputProps
}) => {
  const [passwordVisible, setPasswordVisible] = useState(false);
  const password = mode === 'password';
  return (
    <label className='auth-native-field'>
      {label && <span className='auth-native-label'>{label}</span>}
      <span className='auth-native-input-row'>
        {prefix && <span className='auth-native-input-icon'>{prefix}</span>}
        <input
          name={name}
          type={password ? (passwordVisible ? 'text' : 'password') : type || 'text'}
          placeholder={placeholder}
          value={value}
          onChange={(event) => onChange?.(event.target.value)}
          autoComplete={autoComplete || (password ? 'current-password' : undefined)}
          {...inputProps}
        />
        {password && (
          <button
            type='button'
            className='auth-native-eye'
            onClick={() => setPasswordVisible((visible) => !visible)}
            aria-label={passwordVisible ? 'Hide password' : 'Show password'}
          >
            {passwordVisible ? <EyeOff size={18} /> : <Eye size={18} />}
          </button>
        )}
        {suffix && <span className='auth-native-input-suffix'>{suffix}</span>}
      </span>
    </label>
  );
};

export const Form = ({ children, className = '' }) => (
  <div className={`auth-native-form ${className}`}>{children}</div>
);
Form.Input = AuthInput;

export const Modal = ({
  cancelText = '取消',
  children,
  footer,
  onCancel,
  onOk,
  okButtonProps = {},
  okText,
  title,
  visible,
  width = 450,
}) => {
  useEffect(() => {
    if (!visible) return undefined;
    const onKeyDown = (event) => {
      if (event.key === 'Escape') onCancel?.();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [onCancel, visible]);

  if (!visible) return null;
  return (
    <div className='auth-native-modal-layer'>
      <div className='auth-native-modal' role='dialog' aria-modal='true' style={{ maxWidth: width }}>
        <div className='auth-native-modal-header'>
          <div>{title}</div>
          <button type='button' onClick={onCancel} aria-label='Close'><X size={18} /></button>
        </div>
        <div className='auth-native-modal-body'>{children}</div>
        {footer !== null && (
          <div className='auth-native-modal-footer'>
            <Button onClick={onCancel}>{cancelText}</Button>
            <Button onClick={onOk} loading={okButtonProps.loading}>{okText}</Button>
          </div>
        )}
      </div>
    </div>
  );
};

Modal.error = ({ content, title }) => {
  window.alert([title, content].filter(Boolean).join('\n'));
};
