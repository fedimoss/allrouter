// normalizeCurrencyAmount 将任意输入转为合法数字，无效值返回 0
export const normalizeCurrencyAmount = (value) => {
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : 0;
};

// normalizeDisplayCurrency 将后端返回的展示币种配置标准化为统一结构
// 非人民币统一回退为美元，人民币必须提供合法的正汇率
export const normalizeDisplayCurrency = (config) => {
  // 只支持 CNY 和 USD 两种展示币种，其他一律回退为 USD
  const currency = config?.currency === 'CNY' ? 'CNY' : 'USD';
  // 根据币种选择默认符号
  const fallbackSymbol = currency === 'CNY' ? '¥' : '$';
  // 兼容驼峰和下划线两种字段名
  const rawUnitPrice = Number(config?.unitPrice ?? config?.unit_price ?? 1);
  // 人民币汇率必须为有限正数，否则回退为 1
  const unitPrice =
    currency === 'CNY' && Number.isFinite(rawUnitPrice) && rawUnitPrice > 0
      ? rawUnitPrice
      : 1;

  return {
    currency,
    symbol: config?.symbol || config?.display_symbol || fallbackSymbol,
    unitPrice,
  };
};

// convertUsdToDisplayAmount 将美元金额转换为展示币种金额
// 人民币按汇率换算，美元直接返回原值
export const convertUsdToDisplayAmount = (usdAmount, config) => {
  const amount = normalizeCurrencyAmount(usdAmount);
  const displayCurrency = normalizeDisplayCurrency(config);
  return displayCurrency.currency === 'CNY'
    ? amount * displayCurrency.unitPrice
    : amount;
};

// formatDisplayMoney 格式化金额为带符号的字符串
// amount 金额数值，symbol 币种符号（默认 $），digits 小数位数（默认 2）
// 金额 <= 0 时不显示符号
export const formatDisplayMoney = (amount, symbol = '$', digits = 2) => {
  const normalizedAmount = normalizeCurrencyAmount(amount);
  const prefix = normalizedAmount <= 0 ? '' : symbol;
  return `${prefix}${normalizedAmount.toFixed(digits)}`;
};
