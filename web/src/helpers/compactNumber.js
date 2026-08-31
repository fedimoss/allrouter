import i18next from 'i18next';

// 紧凑显示阈值：低于该值的数字保持原样展示
export const COMPACT_NUMBER_THRESHOLD = 10_000;

// formatCompactValue 将带前缀（如 $、¥）/后缀的数字字符串转为紧凑显示
// 返回 displayValue（紧凑数值）、compactUnit（万/K 等单位）、trailingValue（尾部内容）、fullValue（原始完整值）
export const formatCompactValue = (value) => {
  const fullValue = String(value);
  const normalizedValue = fullValue.trim().replaceAll(',', '');
  const valueParts = normalizedValue.match(
    /^([^0-9+-]*)([+-]?\d+(?:\.\d+)?)(.*)$/,
  );

  if (!valueParts) {
    return {
      displayValue: value,
      compactUnit: '',
      trailingValue: '',
      fullValue,
      isCompact: false,
    };
  }

  const numericValue = Number(valueParts[2]);
  if (
    !Number.isFinite(numericValue) ||
    Math.abs(numericValue) < COMPACT_NUMBER_THRESHOLD
  ) {
    return {
      displayValue: value,
      compactUnit: '',
      trailingValue: '',
      fullValue,
      isCompact: false,
    };
  }

  const locale = i18next.resolvedLanguage || i18next.language || 'zh-CN';
  let compactParts;
  try {
    compactParts = new Intl.NumberFormat(locale, {
      notation: 'compact',
      compactDisplay: 'short',
      maximumFractionDigits: 2,
    }).formatToParts(numericValue);
  } catch {
    compactParts = new Intl.NumberFormat('zh-CN', {
      notation: 'compact',
      compactDisplay: 'short',
      maximumFractionDigits: 2,
    }).formatToParts(numericValue);
  }

  const compactUnit = compactParts
    .filter((part) => part.type === 'compact')
    .map((part) => part.value)
    .join('');
  const compactNumber = compactParts
    .filter((part) => part.type !== 'compact')
    .map((part) => part.value)
    .join('');

  return {
    displayValue: `${valueParts[1]}${compactNumber}`,
    compactUnit,
    trailingValue: valueParts[3],
    fullValue,
    isCompact: true,
  };
};
