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

import { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import { useTableCompactMode } from '../common/useTableCompactMode';

// useSubscriptionsData 订阅套餐管理的数据 Hook（Admin 与 Provider 共用）。
// options.plansApi: 套餐增删改查接口前缀，默认主站 /api/subscription/admin/plans；
// options.tableKey: 紧凑模式等本地态存储 key，默认 'subscriptions'，服务商页面传 'provider-subscriptions' 以隔离状态。
export const useSubscriptionsData = (options = {}) => {
  const { t } = useTranslation();
  const {
    plansApi = '/api/subscription/admin/plans',
    tableKey = 'subscriptions',
  } = options;
  const [compactMode, setCompactMode] = useTableCompactMode(tableKey);

  // State management
  const [allPlans, setAllPlans] = useState([]);
  const [loading, setLoading] = useState(true);

  // Pagination (client-side for now)
  const [activePage, setActivePage] = useState(1);
  const pageSize = 10;

  // Drawer states
  const [showEdit, setShowEdit] = useState(false);
  const [editingPlan, setEditingPlan] = useState(null);
  const [sheetPlacement, setSheetPlacement] = useState('left'); // 'left' | 'right'

  // Load subscription plans
  const loadPlans = async () => {
    setLoading(true);
    try {
      // 通过 plansApi 拉取套餐列表，主站走 admin 接口，服务商走 provider 接口。
      const res = await API.get(plansApi);
      if (res.data?.success) {
        const next = res.data.data || [];
        setAllPlans(next);

        // Keep page in range after data changes
        const totalPages = Math.max(1, Math.ceil(next.length / pageSize));
        setActivePage((p) => Math.min(p || 1, totalPages));
      } else {
        showError(res.data?.message || t('加载失败'));
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setLoading(false);
    }
  };

  // Refresh data
  const refresh = async () => {
    await loadPlans();
  };

  const handlePageChange = (page) => {
    setActivePage(page);
  };

  // Update plan enabled status (single endpoint)
  const setPlanEnabled = async (planRecordOrId, enabled) => {
    const planId =
      typeof planRecordOrId === 'number'
        ? planRecordOrId
        : planRecordOrId?.plan?.id;
    if (!planId) return;
    setLoading(true);
    try {
      // 切换启停走 PATCH {plansApi}/{id}，后端按当前用户身份(admin/provider)做归属鉴权。
      const res = await API.patch(`${plansApi}/${planId}`, {
        enabled: !!enabled,
      });
      if (res.data?.success) {
        showSuccess(enabled ? t('已启用') : t('已禁用'));
        await loadPlans();
      } else {
        showError(res.data?.message || t('操作失败'));
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setLoading(false);
    }
  };

  // Modal control functions
  const closeEdit = () => {
    setShowEdit(false);
    setEditingPlan(null);
  };

  const openCreate = () => {
    setSheetPlacement('left');
    setEditingPlan(null);
    setShowEdit(true);
  };

  const openEdit = (planRecord) => {
    setSheetPlacement('right');
    setEditingPlan(planRecord);
    setShowEdit(true);
  };

  // Initialize data on component mount
  useEffect(() => {
    loadPlans();
  }, []);

  const planCount = allPlans.length;
  const plans = allPlans.slice(
    Math.max(0, (activePage - 1) * pageSize),
    Math.max(0, (activePage - 1) * pageSize) + pageSize,
  );

  return {
    // Data state
    plans,
    planCount,
    loading,

    // Modal state
    showEdit,
    editingPlan,
    sheetPlacement,
    setShowEdit,
    setEditingPlan,

    // UI state
    compactMode,
    setCompactMode,

    // Pagination
    activePage,
    pageSize,
    handlePageChange,

    // Actions
    loadPlans,
    setPlanEnabled,
    refresh,
    closeEdit,
    openCreate,
    openEdit,

    // Translation
    t,
  };
};
