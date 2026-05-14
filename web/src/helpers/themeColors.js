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

export const DEFAULT_THEME_PRIMARY_COLOR = '#09FEF7';
export const DEFAULT_THEME_SECONDARY_COLOR = '#BAFF29';

const WEB_PRIMARY_COLOR_KEY = 'web_primary_color';
const WEB_SECONDARY_COLOR_KEY = 'web_secondary_color';

const normalizeHexColor = (value) => {
  if (typeof value !== 'string') {
    return '';
  }
  const color = value.trim();
  if (/^#[0-9a-fA-F]{6}$/.test(color)) {
    return color;
  }
  if (/^#[0-9a-fA-F]{3}$/.test(color)) {
    return `#${color
      .slice(1)
      .split('')
      .map((char) => char + char)
      .join('')}`;
  }
  return '';
};

const hexToRgb = (hex) => {
  const normalized = normalizeHexColor(hex);
  if (!normalized) {
    return null;
  }
  const value = normalized.slice(1);
  return {
    r: parseInt(value.slice(0, 2), 16),
    g: parseInt(value.slice(2, 4), 16),
    b: parseInt(value.slice(4, 6), 16),
  };
};

const getColorWithFallback = (value, fallback) =>
  normalizeHexColor(value) || fallback;

export const applyThemeColors = (primaryColor, secondaryColor) => {
  const primary = getColorWithFallback(
    primaryColor,
    DEFAULT_THEME_PRIMARY_COLOR,
  );
  const secondary = getColorWithFallback(
    secondaryColor,
    DEFAULT_THEME_SECONDARY_COLOR,
  );
  const primaryRgb = hexToRgb(primary);
  const secondaryRgb = hexToRgb(secondary);
  const rootStyle = document.documentElement.style;

  rootStyle.setProperty('--theme-primary', primary);
  rootStyle.setProperty('--theme-secondary', secondary);
  rootStyle.setProperty(
    '--theme-gradient',
    `linear-gradient(97.63deg, ${primary} 0%, ${secondary} 100%)`,
  );
  rootStyle.setProperty(
    '--theme-gradient-135',
    `linear-gradient(135deg, ${primary} 0%, ${secondary} 100%)`,
  );
  rootStyle.setProperty('--brand-cyan', primary);
  rootStyle.setProperty('--brand-yellow', secondary);
  rootStyle.setProperty(
    '--brand-gradient',
    `linear-gradient(135deg, ${primary} 0%, ${secondary} 100%)`,
  );
  rootStyle.setProperty(
    '--brand-gradient-text',
    `linear-gradient(135deg, ${primary}, ${secondary})`,
  );

  if (primaryRgb) {
    rootStyle.setProperty(
      '--theme-primary-rgb',
      `${primaryRgb.r}, ${primaryRgb.g}, ${primaryRgb.b}`,
    );
    rootStyle.setProperty(
      '--theme-primary-10',
      `rgba(${primaryRgb.r}, ${primaryRgb.g}, ${primaryRgb.b}, 0.1)`,
    );
    rootStyle.setProperty(
      '--theme-primary-12',
      `rgba(${primaryRgb.r}, ${primaryRgb.g}, ${primaryRgb.b}, 0.12)`,
    );
    rootStyle.setProperty(
      '--theme-primary-20',
      `rgba(${primaryRgb.r}, ${primaryRgb.g}, ${primaryRgb.b}, 0.2)`,
    );
    rootStyle.setProperty(
      '--theme-primary-30',
      `rgba(${primaryRgb.r}, ${primaryRgb.g}, ${primaryRgb.b}, 0.3)`,
    );
    rootStyle.setProperty(
      '--theme-primary-40',
      `rgba(${primaryRgb.r}, ${primaryRgb.g}, ${primaryRgb.b}, 0.4)`,
    );
  }
  if (secondaryRgb) {
    rootStyle.setProperty(
      '--theme-secondary-rgb',
      `${secondaryRgb.r}, ${secondaryRgb.g}, ${secondaryRgb.b}`,
    );
    rootStyle.setProperty(
      '--theme-secondary-50',
      `rgba(${secondaryRgb.r}, ${secondaryRgb.g}, ${secondaryRgb.b}, 0.5)`,
    );
  }

  rootStyle.setProperty('--semi-color-primary', primary);
  rootStyle.setProperty('--semi-color-primary-hover', primary);
  rootStyle.setProperty('--semi-color-primary-active', primary);
  rootStyle.setProperty(
    '--semi-color-primary-light-default',
    'var(--theme-primary-10)',
  );
  rootStyle.setProperty(
    '--semi-color-primary-light-hover',
    'var(--theme-primary-20)',
  );
  rootStyle.setProperty(
    '--semi-color-primary-light-active',
    'var(--theme-primary-30)',
  );

  localStorage.setItem(WEB_PRIMARY_COLOR_KEY, primary);
  localStorage.setItem(WEB_SECONDARY_COLOR_KEY, secondary);
};

export const applyStoredThemeColors = () => {
  applyThemeColors(
    localStorage.getItem(WEB_PRIMARY_COLOR_KEY),
    localStorage.getItem(WEB_SECONDARY_COLOR_KEY),
  );
};

export const getStoredThemeColors = () => ({
  primaryColor:
    localStorage.getItem(WEB_PRIMARY_COLOR_KEY) || DEFAULT_THEME_PRIMARY_COLOR,
  secondaryColor:
    localStorage.getItem(WEB_SECONDARY_COLOR_KEY) ||
    DEFAULT_THEME_SECONDARY_COLOR,
});

export const extractThemeColors = (payload) => {
  const data = payload?.data?.data ?? payload?.data ?? payload;
  if (!data) {
    return {};
  }
  if (Array.isArray(data)) {
    const colorMap = Object.fromEntries(
      data.map((item) => [item?.key, item?.value]),
    );
    return {
      primaryColor:
        colorMap.primary_color ||
        colorMap.WebPrimaryColor ||
        colorMap.web_primary_color ||
        colorMap.theme_color,
      secondaryColor:
        colorMap.secondary_color ||
        colorMap.WebSecondaryColor ||
        colorMap.web_secondary_color,
    };
  }
  return {
    primaryColor:
      data.primary_color ||
      data.WebPrimaryColor ||
      data.web_primary_color ||
      data.theme_color,
    secondaryColor:
      data.secondary_color ||
      data.WebSecondaryColor ||
      data.web_secondary_color,
  };
};
