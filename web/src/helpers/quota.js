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
import { getCurrencyConfig } from './render';

export const getQuotaPerUnit = () => {
  // The backend stores quota in its native unit (currently 500,000 units per
  // USD).  The status endpoint normally hydrates this value into localStorage,
  // but subscription/admin screens can render before that request completes
  // (and a first visit has no cached value at all).  Falling back to `1`
  // silently converts a displayed `$0.01` window into one native quota unit,
  // effectively disabling the configured allowance.  Keep the frontend
  // fallback in lockstep with common.QuotaPerUnit so form round-trips remain
  // lossless even before status hydration.
  const raw = parseFloat(
    localStorage.getItem('quota_per_unit') || String(500 * 1000),
  );
  return Number.isFinite(raw) && raw > 0 ? raw : 500 * 1000;
};

export const quotaToDisplayAmount = (quota) => {
  const q = Number(quota || 0);
  if (!Number.isFinite(q) || q === 0) return 0;
  const sign = Math.sign(q);
  const abs = Math.abs(q);
  const { type, rate } = getCurrencyConfig();
  if (type === 'TOKENS') return q;
  const usd = abs / getQuotaPerUnit();
  if (type === 'USD') return sign * usd;
  return sign * usd * (rate || 1);
};

export const displayAmountToQuota = (amount) => {
  const val = Number(amount || 0);
  if (!Number.isFinite(val) || val === 0) return 0;
  const sign = Math.sign(val);
  const abs = Math.abs(val);
  const { type, rate } = getCurrencyConfig();
  if (type === 'TOKENS') return Math.round(val);
  const usd = type === 'USD' ? abs : abs / (rate || 1);
  return sign * Math.round(usd * getQuotaPerUnit());
};
