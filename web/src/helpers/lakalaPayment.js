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

export const LAKALA_QRCODE_ROUTE = '/payment/lakala/qrcode';

const LAKALA_QRCODE_STORAGE_PREFIX = 'lakala:qrcode:';

const getStorageKey = (tradeNo) =>
  `${LAKALA_QRCODE_STORAGE_PREFIX}${String(tradeNo || '').trim()}`;

export const isLakalaQRCodePayment = (url, data) =>
  String(url || '').trim() === LAKALA_QRCODE_ROUTE &&
  !!String(data?.code || '').trim() &&
  !!String(data?.trade_no || '').trim();

export const saveLakalaQRCodePayment = (data) => {
  const tradeNo = String(data?.trade_no || '').trim();
  if (!tradeNo) return '';

  localStorage.setItem(
    getStorageKey(tradeNo),
    JSON.stringify({
      code: String(data?.code || '').trim(),
      trade_no: tradeNo,
      amount: String(data?.amount || '').trim(),
      created_at: Date.now(),
    }),
  );
  return tradeNo;
};

export const getLakalaQRCodePayment = (tradeNo) => {
  const key = getStorageKey(tradeNo);
  const raw = localStorage.getItem(key);
  if (!raw) return null;

  try {
    return JSON.parse(raw);
  } catch (error) {
    localStorage.removeItem(key);
    return null;
  }
};
