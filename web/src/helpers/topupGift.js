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

// 充值赠送规则工具（主站 SettingsRechargeGift 与服务商 ProviderRechargeGift 共用）
// 规则结构：{ id, threshold, bonus }——id 是稳定标识（作后端"每用户每档一次"幂等键），
// threshold 是充值门槛，bonus 是赠送金额，二者均按"用户币种原值"填写。

// 生成稳定规则 ID
export const newRuleId = () =>
  'r_' + Math.random().toString(36).slice(2, 10) + Date.now().toString(36);

// 解析 JSON 为规则数组，确保每条都有稳定 id；丢弃无效（threshold/bonus<=0）规则
export function parseRules(jsonStr) {
  if (!jsonStr || !String(jsonStr).trim()) return [];
  try {
    const arr = JSON.parse(jsonStr);
    if (!Array.isArray(arr)) return [];
    return arr
      .map((r) => ({
        id:
          r && typeof r.id === 'string' && r.id ? r.id : newRuleId(),
        threshold: Number(r?.threshold) || 0,
        bonus: Number(r?.bonus) || 0,
      }))
      .filter((r) => r.threshold > 0 && r.bonus > 0);
  } catch {
    return [];
  }
}

// 序列化为后端存储的 JSON（只保留有效规则，threshold/bonus 必须 > 0）
export function serializeRules(rules) {
  const cleaned = rules
    .filter((r) => Number(r.threshold) > 0 && Number(r.bonus) > 0)
    .map((r) => ({
      id: r.id,
      threshold: Number(r.threshold),
      bonus: Number(r.bonus),
    }));
  return JSON.stringify(cleaned);
}

export function parseTimedConfig(raw) {
  if (!raw) return { enabled: false, day: 0, endTime: 0 };
  try {
    const value = typeof raw === 'string' ? JSON.parse(raw) : raw;
    return {
      enabled: value?.enabled === true,
      day: Number(value?.day) || 0,
      endTime: Number(value?.end_time) || 0,
    };
  } catch {
    return { enabled: false, day: 0, endTime: 0 };
  }
}

export function serializeTimedConfig(enabled, day) {
  return JSON.stringify({ enabled: enabled === true, day: Number(day) || 0 });
}
