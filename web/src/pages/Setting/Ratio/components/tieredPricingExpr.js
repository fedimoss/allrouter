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

import {
  BILLING_CACHE_VAR_MAP,
  BILLING_CONDITION_VARS,
} from '../../../../constants/billing.constants.js';

export const CACHE_MODE_TIMED = 'timed';
export const CACHE_MODE_GENERIC = 'generic';
export const WEEKDAY_KEYS = [
  '周日',
  '周一',
  '周二',
  '周三',
  '周四',
  '周五',
  '周六',
];
export const TIME_SCHEDULE_ERROR =
  '请选择有效时区、至少一个星期和有效时段，开始与结束时间不能相同。';

export function createTimeSchedule() {
  return {
    timezone: 'UTC',
    weekdays: [1, 2, 3, 4, 5],
    ranges: [{ start: 9, end: 18 }],
  };
}

export function getTimeScheduleError(schedule) {
  if (!schedule) return '';
  try {
    if (!schedule.timezone?.trim()) return TIME_SCHEDULE_ERROR;
    new Intl.DateTimeFormat('en', { timeZone: schedule.timezone });
  } catch {
    return TIME_SCHEDULE_ERROR;
  }
  if (
    !schedule.weekdays?.length ||
    schedule.weekdays.some(
      (day) => !Number.isInteger(day) || day < 0 || day > 6,
    ) ||
    !schedule.ranges?.length ||
    schedule.ranges.some(
      ({ start, end }) =>
        !Number.isInteger(start) ||
        !Number.isInteger(end) ||
        start < 0 ||
        start > 23 ||
        end < 0 ||
        end > 24 ||
        start === end,
    )
  ) {
    return TIME_SCHEDULE_ERROR;
  }
  return '';
}

export function buildTimeScheduleExpr(schedule) {
  if (!schedule || getTimeScheduleError(schedule)) return '';
  const tz = JSON.stringify(schedule.timezone);
  const hour = `hour(${tz})`;
  const ranges = schedule.ranges.map(
    ({ start, end }) =>
      `${hour} >= ${start} ${start < end ? '&&' : '||'} ${hour} < ${end}`,
  );
  const days = [...new Set(schedule.weekdays)].sort((a, b) => a - b);
  let weekday = '';
  if (days.length < 7) {
    const fn = `weekday(${tz})`;
    if (days.length === 1) {
      weekday = `${fn} == ${days[0]}`;
    } else if (days.every((day, i) => day === days[0] + i)) {
      weekday = `${fn} >= ${days[0]} && ${fn} <= ${days.at(-1)}`;
    } else {
      weekday = `(${days.map((day) => `${fn} == ${day}`).join(' || ')})`;
    }
  }
  const time = `(${ranges.join(' || ')})`;
  return weekday ? `${time} && ${weekday}` : time;
}

export function getTierCacheMode(tier) {
  if ([CACHE_MODE_TIMED, CACHE_MODE_GENERIC].includes(tier?.cache_mode)) {
    return tier.cache_mode;
  }
  return Number(tier?.cache_create_1h_unit_cost) > 0
    ? CACHE_MODE_TIMED
    : CACHE_MODE_GENERIC;
}

export function normalizeVisualTier(tier = {}) {
  return {
    ...tier,
    conditions: Array.isArray(tier.conditions) ? tier.conditions : [],
    cache_mode: getTierCacheMode(tier),
    time_schedule: tier.time_schedule || null,
  };
}

export function createDefaultVisualConfig() {
  return {
    tiers: [
      normalizeVisualTier({
        input_unit_cost: 0,
        output_unit_cost: 0,
        label: 'base',
      }),
    ],
  };
}

export function normalizeVisualConfig(config) {
  if (!config?.tiers?.length) return createDefaultVisualConfig();
  return { ...config, tiers: config.tiers.map(normalizeVisualTier) };
}

export function buildConditionStr(conditions) {
  return (conditions || [])
    .filter((c) => c.var && c.op && c.value != null && c.value !== '')
    .map((c) => `${c.var} ${c.op} ${c.value}`)
    .join(' && ');
}

export function hasTierConditions(tier) {
  return !!(tier?.conditions?.length || tier?.time_schedule);
}

function buildTierBodyExpr(tier) {
  const parts = [
    `p * ${Number(tier.input_unit_cost) || 0}`,
    `c * ${Number(tier.output_unit_cost) || 0}`,
  ];
  const order = [
    ...(tier.priced_vars || []),
    ...BILLING_CACHE_VAR_MAP.map((cv) => cv.exprVar),
  ];
  for (const key of new Set(order)) {
    const cv = BILLING_CACHE_VAR_MAP.find((item) => item.exprVar === key);
    if (!cv) continue;
    const value = Number(tier[cv.field]) || 0;
    if (value !== 0 || tier.priced_vars?.includes(cv.exprVar)) {
      parts.push(`${cv.exprVar} * ${value}`);
    }
  }
  return parts.join(' + ');
}

export function generateExprFromVisualConfig(config) {
  if (!config?.tiers?.length) return 'p * 0 + c * 0';
  const { tiers } = config;
  const parts = [];
  let singleConditional = false;
  for (let i = 0; i < tiers.length; i++) {
    const tier = tiers[i];
    const isConditional = i < tiers.length - 1 || tiers.length === 1;
    if (isConditional && getTimeScheduleError(tier.time_schedule)) return '';
    const tokenCondition = buildConditionStr(tier.conditions);
    const timeCondition = buildTimeScheduleExpr(tier.time_schedule);
    const condition = [tokenCondition, timeCondition]
      .filter(Boolean)
      .join(' && ');
    const label =
      tier.label || (tiers.length === 1 ? 'default' : `第${i + 1}档`);
    const body = `tier(${JSON.stringify(label)}, ${buildTierBodyExpr(tier)})`;
    if (isConditional && condition) {
      singleConditional = tiers.length === 1;
      parts.push(`${condition} ? ${body}`);
    } else {
      // An unconditional tier before the fallback would make later tiers unreachable.
      if (i < tiers.length - 1) return '';
      parts.push(body);
    }
  }
  if (singleConditional) {
    parts.push('p * 0 + c * 0');
  }
  return `${config.version || ''}${parts.join(' : ')}`;
}

// Split only outside parentheses and quoted strings. Unknown syntax stays in raw mode.
function splitTopLevel(source, separator) {
  const parts = [];
  let depth = 0;
  let quoted = false;
  let start = 0;
  for (let i = 0; i < source.length; i++) {
    const ch = source[i];
    if (quoted && ch === '\\') {
      i++;
      continue;
    }
    if (ch === '"') quoted = !quoted;
    if (quoted) continue;
    if (ch === '(') depth++;
    if (ch === ')') depth--;
    if (depth < 0) throw new Error('Unbalanced parentheses');
    if (depth === 0 && source.startsWith(separator, i)) {
      parts.push(source.slice(start, i).trim());
      i += separator.length - 1;
      start = i + 1;
    }
  }
  if (depth !== 0 || quoted) throw new Error('Unbalanced expression');
  parts.push(source.slice(start).trim());
  return parts;
}

function unwrap(source) {
  let current = source.trim();
  while (current.startsWith('(') && current.endsWith(')')) {
    try {
      splitTopLevel(current.slice(1, -1), '?');
      current = current.slice(1, -1).trim();
    } catch {
      break;
    }
  }
  return current;
}

const NUMBER = '-?(?:\\d+\\.?\\d*|\\.\\d+)(?:[eE][+-]?\\d+)?';
const STRING = '"(?:\\\\.|[^"\\\\])*"';
const TOKEN_CONDITION = new RegExp(`^(len|p|c)\\s*(<=|>=|<|>)\\s*(${NUMBER})$`);
const TIME_CONDITION = new RegExp(
  `^(hour|weekday)\\s*\\(\\s*(${STRING})\\s*\\)\\s*(<=|>=|==|!=|<|>)\\s*(${NUMBER})$`,
);

function parseCondition(source) {
  const text = unwrap(source);
  for (const op of ['||', '&&']) {
    const parts = splitTopLevel(text, op);
    if (parts.length > 1) {
      const children = parts.map(parseCondition);
      return children.every(Boolean) ? { op, children } : null;
    }
  }
  const token = text.match(TOKEN_CONDITION);
  if (token) return { var: token[1], op: token[2], value: Number(token[3]) };
  const time = text.match(TIME_CONDITION);
  if (time) {
    return {
      timeFunc: time[1],
      timezone: JSON.parse(time[2]),
      op: time[3],
      value: Number(time[4]),
    };
  }
  return null;
}

function flattenAnd(node) {
  return node.op === '&&' ? node.children.flatMap(flattenAnd) : [node];
}

function compare(value, op, target) {
  switch (op) {
    case '<':
      return value < target;
    case '<=':
      return value <= target;
    case '>':
      return value > target;
    case '>=':
      return value >= target;
    case '==':
      return value === target;
    case '!=':
      return value !== target;
    default:
      return false;
  }
}

function evalTimeCondition(node, hour, weekday) {
  if (node.op === '&&')
    return node.children.every((n) => evalTimeCondition(n, hour, weekday));
  if (node.op === '||')
    return node.children.some((n) => evalTimeCondition(n, hour, weekday));
  return compare(
    node.timeFunc === 'hour' ? hour : weekday,
    node.op,
    node.value,
  );
}

function parseTimeSchedule(nodes) {
  const timezones = new Set();
  const visit = (node) => {
    if (node.children) return node.children.every(visit);
    if (!node.timeFunc || !Number.isFinite(node.value)) return false;
    timezones.add(node.timezone);
    return true;
  };
  if (!nodes.every(visit) || timezones.size !== 1) return null;
  const tree = { op: '&&', children: nodes };
  const weekdays = [];
  let activeHours = null;
  // Exhaustively verify the weekly hourly schedule before converting it to form fields.
  for (let day = 0; day < 7; day++) {
    const hours = Array.from({ length: 24 }, (_, hour) =>
      evalTimeCondition(tree, hour, day),
    );
    if (!hours.some(Boolean)) continue;
    if (
      activeHours &&
      hours.some((active, hour) => active !== activeHours[hour])
    )
      return null;
    activeHours = hours;
    weekdays.push(day);
  }
  if (!activeHours) return null;
  const ranges = [];
  for (let hour = 0; hour < 24; hour++) {
    if (!activeHours[hour]) continue;
    const start = hour;
    while (hour < 24 && activeHours[hour]) hour++;
    ranges.push({ start, end: hour });
  }
  const schedule = { timezone: [...timezones][0], weekdays, ranges };
  return getTimeScheduleError(schedule) ? null : schedule;
}

function parseVisualConditions(source) {
  const tree = parseCondition(source);
  if (!tree) return null;
  const conditions = [];
  const timeNodes = [];
  for (const node of flattenAnd(tree)) {
    if (node.var && BILLING_CONDITION_VARS.includes(node.var))
      conditions.push(node);
    else timeNodes.push(node);
  }
  if (conditions.length > 2) return null;
  const time_schedule = timeNodes.length ? parseTimeSchedule(timeNodes) : null;
  if (timeNodes.length && !time_schedule) return null;
  return { conditions, time_schedule };
}

function parseTier(source) {
  const call = source.match(
    new RegExp(`^tier\\s*\\(\\s*(${STRING})\\s*,\\s*([\\s\\S]*)\\)$`),
  );
  if (!call) return null;
  const body = call[2];
  const extras = BILLING_CACHE_VAR_MAP.map((cv) => cv.exprVar).join('|');
  const bodyPattern = new RegExp(
    `^p\\s*\\*\\s*(${NUMBER})\\s*\\+\\s*c\\s*\\*\\s*(${NUMBER})((?:\\s*\\+\\s*(?:${extras})\\s*\\*\\s*${NUMBER})*)$`,
  );
  const match = body.trim().match(bodyPattern);
  if (!match) return null;
  const tier = {
    label: JSON.parse(call[1]),
    input_unit_cost: Number(match[1]),
    output_unit_cost: Number(match[2]),
    conditions: [],
    priced_vars: [],
  };
  const extraPattern = new RegExp(`(${extras})\\s*\\*\\s*(${NUMBER})`, 'g');
  let extra;
  while ((extra = extraPattern.exec(match[3])) !== null) {
    const cv = BILLING_CACHE_VAR_MAP.find((item) => item.exprVar === extra[1]);
    if (tier.priced_vars.includes(cv.exprVar)) return null;
    tier.priced_vars.push(cv.exprVar);
    tier[cv.field] = Number(extra[2]);
  }
  if (
    ![
      tier.input_unit_cost,
      tier.output_unit_cost,
      ...BILLING_CACHE_VAR_MAP.map((cv) => tier[cv.field] ?? 0),
    ].every(Number.isFinite)
  )
    return null;
  return normalizeVisualTier(tier);
}

export function tryParseVisualConfig(source) {
  if (!source?.trim()) return null;
  try {
    let expression = source.trim();
    let version = '';
    const prefix = expression.match(/^v\d+:/);
    if (prefix) {
      if (prefix[0] !== 'v1:') return null;
      version = prefix[0];
      expression = expression.slice(version.length).trim();
    }
    const branches = splitTopLevel(unwrap(expression), ':');
    const tiers = [];
    for (let i = 0; i < branches.length; i++) {
      const parts = splitTopLevel(branches[i], '?');
      if (parts.length > 2) return null;
      if (parts.length === 1 && i < branches.length - 1) return null;
      if (parts.length === 2 && i === branches.length - 1) return null;
      if (
        i === 1 &&
        branches.length === 2 &&
        parts[0].replace(/\s/g, '') === 'p*0+c*0'
      ) {
        return normalizeVisualConfig({ tiers, version });
      }
      const tier = parseTier(parts.at(-1));
      if (!tier) return null;
      if (parts.length === 2) {
        const parsed = parseVisualConditions(parts[0]);
        if (!parsed) return null;
        Object.assign(tier, parsed);
      }
      tiers.push(tier);
    }
    return normalizeVisualConfig({ tiers, version });
  } catch {
    return null;
  }
}

export function createBillingTimeEnv(date = new Date()) {
  const cache = new Map();
  const partsInZone = (timezone) => {
    const zone = timezone?.trim() || 'UTC';
    if (!cache.has(zone)) {
      let formatter;
      const options = {
        timeZone: zone,
        hourCycle: 'h23',
        hour: '2-digit',
        minute: '2-digit',
        weekday: 'short',
        month: 'numeric',
        day: 'numeric',
      };
      try {
        formatter = new Intl.DateTimeFormat('en-US', options);
      } catch {
        formatter = new Intl.DateTimeFormat('en-US', {
          ...options,
          timeZone: 'UTC',
        });
      }
      cache.set(
        zone,
        Object.fromEntries(
          formatter.formatToParts(date).map(({ type, value }) => [type, value]),
        ),
      );
    }
    return cache.get(zone);
  };
  return {
    hour: (tz) => Number(partsInZone(tz).hour),
    minute: (tz) => Number(partsInZone(tz).minute),
    weekday: (tz) =>
      ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'].indexOf(
        partsInZone(tz).weekday,
      ),
    month: (tz) => Number(partsInZone(tz).month),
    day: (tz) => Number(partsInZone(tz).day),
  };
}
