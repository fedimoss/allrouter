import { renderQuotaWithAmount } from '../../helpers';

export function joinClasses(...values) {
  return values.filter(Boolean).join(' ');
}

export function hasValue(value) {
  if (Array.isArray(value)) {
    return value.length > 0;
  }
  return value !== undefined && value !== null && String(value).trim() !== '';
}

export function firstDefined(source, keys, fallback = undefined) {
  if (!source || typeof source !== 'object') {
    return fallback;
  }

  for (const key of keys) {
    if (Object.prototype.hasOwnProperty.call(source, key) && hasValue(source[key])) {
      return source[key];
    }
  }

  return fallback;
}

export function getDefaultVisibleColumns(columns) {
  return columns.filter((column) => column.defaultChecked).map((column) => column.key);
}

export function createInitialFilters(fields) {
  return fields.reduce((result, field) => {
    result[field.startKey] = '';
    result[field.endKey] = '';
    return result;
  }, {});
}

export function normalizeNumber(value) {
  if (!hasValue(value)) {
    return null;
  }

  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : null;
  }

  if (typeof value === 'string') {
    const parsed = Number(value.replace(/,/g, '').trim());
    return Number.isFinite(parsed) ? parsed : null;
  }

  return null;
}

export function formatInteger(value) {
  const numberValue = normalizeNumber(value);
  if (numberValue === null) {
    return hasValue(value) ? String(value) : '--';
  }

  return new Intl.NumberFormat('zh-CN', {
    maximumFractionDigits: 0,
  }).format(numberValue);
}

export function formatQuotaValue(value) {
  if (!hasValue(value)) {
    return '--';
  }

  if (typeof value === 'string' && Number.isNaN(Number(value.replace(/,/g, '').trim()))) {
    return value;
  }

  const numberValue = normalizeNumber(value);
  if (numberValue === null) {
    return String(value);
  }

  return renderQuotaWithAmount(numberValue);
}

export function normalizeTimestampToMs(value) {
  if (!hasValue(value)) {
    return null;
  }

  if (typeof value === 'number') {
    if (!Number.isFinite(value) || value <= 0) {
      return null;
    }
    return value > 1e12 ? value : value * 1000;
  }

  const stringValue = String(value).trim();
  if (/^\d+$/.test(stringValue)) {
    const numericValue = Number(stringValue);
    if (!Number.isFinite(numericValue) || numericValue <= 0) {
      return null;
    }
    return numericValue > 1e12 ? numericValue : numericValue * 1000;
  }

  const parsed = Date.parse(stringValue);
  return Number.isNaN(parsed) ? null : parsed;
}

export function formatDateTime(value) {
  if (!hasValue(value)) {
    return '--';
  }

  const timestamp = normalizeTimestampToMs(value);
  if (timestamp === null) {
    if (typeof value === 'number') {
      return '--';
    }

    const stringValue = String(value).trim();
    if (!stringValue || /^\d+$/.test(stringValue)) {
      return '--';
    }
    return stringValue;
  }

  const date = new Date(timestamp);
  const yyyy = date.getFullYear();
  const mm = `${date.getMonth() + 1}`.padStart(2, '0');
  const dd = `${date.getDate()}`.padStart(2, '0');
  const hh = `${date.getHours()}`.padStart(2, '0');
  const mi = `${date.getMinutes()}`.padStart(2, '0');
  return `${yyyy}-${mm}-${dd} ${hh}:${mi}`;
}

export function toTimestampParam(value) {
  if (!hasValue(value)) {
    return '';
  }

  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? '' : Math.floor(parsed / 1000);
}

export function countAppliedFilters(filters) {
  return Object.values(filters).filter((value) => hasValue(value)).length;
}

export function getSortLabel(sortState, sortField) {
  if (sortState.key !== sortField || !sortState.order) {
    return '';
  }
  return sortState.order === 'asc' ? '升序' : '降序';
}

export function extractResponsePayload(responseBody) {
  const hasSuccessField =
    responseBody && typeof responseBody === 'object' && 'success' in responseBody;
  const success = hasSuccessField ? !!responseBody.success : true;
  const message =
    responseBody && typeof responseBody === 'object' ? responseBody.message : '';
  const data =
    responseBody &&
    typeof responseBody === 'object' &&
    Object.prototype.hasOwnProperty.call(responseBody, 'data')
      ? responseBody.data
      : responseBody;

  return { success, message, data };
}

export function extractListPayload(payload) {
  const list = Array.isArray(payload)
    ? payload
    : Array.isArray(payload?.list)
      ? payload.list
      : Array.isArray(payload?.items)
        ? payload.items
        : Array.isArray(payload?.records)
          ? payload.records
          : Array.isArray(payload?.rows)
            ? payload.rows
            : [];

  return {
    list,
    total:
      normalizeNumber(
        firstDefined(payload, ['total', 'count', 'totalCount', 'recordCount'], list.length),
      ) || 0,
    pageSize:
      normalizeNumber(firstDefined(payload, ['page_size', 'pageSize', 'size'])) || null,
  };
}

export function buildDashboardCards(cardsConfig, payload) {
  return (cardsConfig || []).map((card) => ({
    ...card,
    value: firstDefined(payload, card.aliases || [], card.defaultValue),
    footer: {
      ...(card.footer || {}),
      trend: firstDefined(
        payload,
        card.footerTrendAliases || [],
        card.footer?.trend || '',
      ),
      text: firstDefined(
        payload,
        card.footerTextAliases || [],
        card.footerText ?? card.footer?.text ?? '',
      ),
    },
  }));
}

export function buildRecordsParams(page, pageSize, keyword, sortState, filters, fields) {
  const params = {
    p: page,
    page_size: pageSize,
  };

  if (hasValue(keyword)) {
    params.keyword = keyword.trim();
  }

  if (sortState.key && sortState.order) {
    params[sortState.key] = sortState.order;
  }

  fields.forEach((field) => {
    const startValue = filters[field.startKey];
    const endValue = filters[field.endKey];

    if (field.inputType === 'datetime-local') {
      const startTimestamp = toTimestampParam(startValue);
      const endTimestamp = toTimestampParam(endValue);
      if (startTimestamp) {
        params[field.startKey] = startTimestamp;
      }
      if (endTimestamp) {
        params[field.endKey] = endTimestamp;
      }
      return;
    }

    if (hasValue(startValue)) {
      params[field.startKey] = startValue;
    }
    if (hasValue(endValue)) {
      params[field.endKey] = endValue;
    }
  });

  return params;
}

export function getRetentionMeta(value) {
  const numericValue = normalizeNumber(value);

  switch (numericValue) {
    case 1:
      return {
        label: '已留存',
        className:
          'bg-emerald-50 text-emerald-600 ring-1 ring-inset ring-emerald-200 dark:bg-emerald-500/10 dark:text-emerald-300 dark:ring-emerald-500/30',
      };
    case 0:
      return {
        label: '流失预警',
        className:
          'bg-amber-50 text-amber-600 ring-1 ring-inset ring-amber-200 dark:bg-amber-500/10 dark:text-amber-300 dark:ring-amber-500/30',
      };
    case -1:
      return {
        label: '已流失',
        className:
          'bg-slate-100 text-slate-500 ring-1 ring-inset ring-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:ring-slate-700',
      };
    default:
      return {
        label: '--',
        className:
          'bg-slate-100 text-slate-500 ring-1 ring-inset ring-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:ring-slate-700',
      };
  }
}

export function getRegistrationSourceLabel(invited) {
  return invited ? '邀请注册' : '自主注册';
}
