// normalizeCurrencyAmount 将任意输入转为合法数字，无效值返回 0
export const normalizeCurrencyAmount = (value) => {
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : 0;
};

// normalizeDisplayCurrency 将后端返回的展示币种配置标准化为统一结构
// 非人民币统一回退为美元，人民币必须提供合法的正汇率
export const normalizeDisplayCurrency = (config) => {
  const currency = config?.currency === 'CNY' ? 'CNY' : 'USD';
  const fallbackSymbol = currency === 'CNY' ? '¥' : '$';
  const rawUnitPrice = Number(config?.unitPrice ?? config?.unit_price ?? 1);
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

// convertUsdToDisplayCurrency 将美元金额按汇率转换为展示币种金额并格式化
// config 为已标准化的 normalizedDisplayCurrency 对象
export const convertAndFormat = (usdAmount, config) => {
  console.log("usdAmount------>", usdAmount)
  const amount = normalizeCurrencyAmount(usdAmount);
  console.log("amount------>", amount)
  console.log("config------>", config)
  // CNY 按汇率换算，USD 直接使用原值
  const displayAmount =
    config.currency === 'CNY' ? amount * config.unitPrice : amount;
  return formatDisplayMoney(displayAmount, config.symbol);
};

// formatDisplayMoney 格式化金额为带符号的字符串
// 先四舍五入到指定小数位（消除浮点精度损失），金额 <= 0 时不显示符号
export const formatDisplayMoney = (amount, symbol = '$', digits = 2) => {
  const normalizedAmount = normalizeCurrencyAmount(amount);
  const factor = Math.pow(10, digits);
  const rounded = Math.round(normalizedAmount * factor) / factor;
  const prefix = rounded <= 0 ? '' : symbol;
  return `${prefix}${rounded.toFixed(digits)}`;
};
