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

import { useState, useEffect, useRef } from 'react';
import { API, showError, showSuccess, copy } from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import {
  REDEMPTION_ACTIONS,
  REDEMPTION_STATUS,
} from '../../constants/redemption.constants';
import { Modal } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { useTableCompactMode } from '../common/useTableCompactMode';

export const useRedemptionsData = ({ apiPrefix = '/api/redemption' } = {}) => {
  const { t } = useTranslation();

  // Basic state
  const [redemptions, setRedemptions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [searching, setSearching] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const pageSize = ITEMS_PER_PAGE;
  const [tokenCount, setTokenCount] = useState(0);
  const [selectedKeys, setSelectedKeys] = useState([]);
  const [displaySymbol, setDisplaySymbol] = useState('');
  // 发放状态筛选：'all' 不过滤 | 'sent' 已发放 | 'unsent' 未发放
  // 由表格「发放」列表头筛选驱动，切换后走服务端查询（避免只过滤当前分页）
  const [sentFilter, setSentFilter] = useState('all');

  // Edit state
  const [editingRedemption, setEditingRedemption] = useState({
    id: undefined,
  });
  const [showEdit, setShowEdit] = useState(false);

  // Form API
  const [formApi, setFormApi] = useState(null);

  // UI state
  const [compactMode, setCompactMode] = useTableCompactMode('redemptions');

  // Form state
  const formInitValues = {
    searchKeyword: '',
  };

  // Get form values
  const getFormValues = () => {
    const formValues = formApi ? formApi.getValues() : {};
    return {
      searchKeyword: formValues.searchKeyword || '',
    };
  };

  // Set redemption data format
  const setRedemptionFormat = (redemptions) => {
    setRedemptions(redemptions);
  };

  // Load redemption list
  // sent 为发放状态筛选值，不传时默认取当前 sentFilter
  const loadRedemptions = async (page = 1, pageSize, sent = sentFilter) => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        p: page,
        page_size: pageSize,
      });
      // 非全部时携带 sent 参数，由服务端过滤（sent=已发放 / unsent=未发放）
      if (sent !== 'all') {
        params.set('sent', sent);
      }
      const res = await API.get(`${apiPrefix}?${params.toString()}`);
      const { success, message, data } = res.data;
      if (success) {
        const newPageData = data.items;
        setActivePage(data.page <= 0 ? 1 : data.page);
        setTokenCount(data.total);
        setDisplaySymbol(data.display_symbol || '');
        setRedemptionFormat(newPageData);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
    setLoading(false);
  };

  // Search redemption codes
  const searchRedemptions = async () => {
    const { searchKeyword } = getFormValues();
    if (searchKeyword === '') {
      await loadRedemptions(1, pageSize);
      return;
    }

    setSearching(true);
    try {
      const params = new URLSearchParams({
        keyword: searchKeyword,
        p: 1,
        page_size: pageSize,
      });
      // 发放状态筛选与关键字搜索叠加，由服务端过滤
      if (sentFilter !== 'all') {
        params.set('sent', sentFilter);
      }
      const res = await API.get(`${apiPrefix}/search?${params.toString()}`);
      const { success, message, data } = res.data;
      if (success) {
        const newPageData = data.items;
        setActivePage(data.page || 1);
        setTokenCount(data.total);
        setRedemptionFormat(newPageData);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
    setSearching(false);
  };

  // Manage redemption codes (CRUD operations)
  const manageRedemption = async (id, action, record) => {
    setLoading(true);
    let data = { id };
    let res;

    try {
      switch (action) {
        case REDEMPTION_ACTIONS.DELETE:
          res = await API.delete(`${apiPrefix}/${id}`);
          break;
        case REDEMPTION_ACTIONS.ENABLE:
          data.status = REDEMPTION_STATUS.UNUSED;
          res = await API.put(`${apiPrefix}?status_only=true`, data);
          break;
        case REDEMPTION_ACTIONS.DISABLE:
          data.status = REDEMPTION_STATUS.DISABLED;
          res = await API.put(`${apiPrefix}?status_only=true`, data);
          break;
        default:
          throw new Error('Unknown operation type');
      }

      const { success, message } = res.data;
      if (success) {
        showSuccess(t('操作成功完成！'));
        let redemption = res.data.data;
        let newRedemptions = [...redemptions];
        if (action !== REDEMPTION_ACTIONS.DELETE) {
          record.status = redemption.status;
        }
        setRedemptions(newRedemptions);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
    setLoading(false);
  };

  // Refresh data
  const refresh = async (page = activePage) => {
    const { searchKeyword } = getFormValues();
    if (searchKeyword === '') {
      await loadRedemptions(page, pageSize);
    } else {
      await searchRedemptions();
    }
  };

  // Handle page change
  const handlePageChange = (page) => {
    setActivePage(page);
    const { searchKeyword } = getFormValues();
    if (searchKeyword === '') {
      loadRedemptions(page, pageSize);
    } else {
      searchRedemptions();
    }
  };

  // Row selection configuration
  const rowSelection = {
    onSelect: (record, selected) => {},
    onSelectAll: (selected, selectedRows) => {},
    onChange: (selectedRowKeys, selectedRows) => {
      setSelectedKeys(selectedRows);
    },
  };

  // Row style handling - using isExpired function
  const handleRow = (record, index) => {
    // Local isExpired function
    const isExpired = (rec) => {
      return (
        rec.status === REDEMPTION_STATUS.UNUSED &&
        rec.expired_time !== 0 &&
        rec.expired_time < Math.floor(Date.now() / 1000)
      );
    };

    if (record.status !== REDEMPTION_STATUS.UNUSED || isExpired(record)) {
      return {
        // style: {
        //   background: 'var(--semi-color-disabled-border)',
        // },
      };
    } else {
      return {};
    }
  };

  // Copy text
  const copyText = async (text) => {
    if (await copy(text)) {
      showSuccess(t('已复制到剪贴板！'));
    } else {
      Modal.error({
        title: t('无法复制到剪贴板，请手动复制'),
        content: text,
        size: 'large',
        okText: t('确定'),
        cancelText: t('取消'),
      });
    }
  };

  // Batch copy redemption codes
  const batchCopyRedemptions = async () => {
    if (selectedKeys.length === 0) {
      showError(t('请至少选择一个兑换码！'));
      return;
    }

    let keys = '';
    for (let i = 0; i < selectedKeys.length; i++) {
      keys += selectedKeys[i].name + '    ' + selectedKeys[i].key + '\n';
    }
    await copyText(keys);
  };

  // Batch delete redemption codes (clear invalid)
  const batchDeleteRedemptions = async () => {
    Modal.confirm({
      title: t('确定清除所有失效兑换码？'),
      content: t('将删除已使用、已禁用及过期的兑换码，此操作不可撤销。'),
      okText: t('确定'),
      cancelText: t('取消'),
      onOk: async () => {
        setLoading(true);
        const res = await API.delete(`${apiPrefix}/invalid`);
        const { success, message, data } = res.data;
        if (success) {
          showSuccess(t('已删除 {{count}} 条失效兑换码', { count: data }));
          await refresh();
        } else {
          showError(message);
        }
        setLoading(false);
      },
    });
  };

  // Toggle single redemption sent mark
  // 行内「发放」开关：切换单个兑换码的发放标记，成功后本地就地更新（不整页刷新）
  const toggleSent = async (record) => {
    const sent = !(record.sent_time > 0);
    try {
      const res = await API.put(`${apiPrefix}/sent`, {
        ids: [record.id],
        sent,
      });
      const { success, message } = res.data;
      if (success) {
        // 直接改写 record 的 sent_time 并触发列表重渲染
        record.sent_time = sent ? Math.floor(Date.now() / 1000) : 0;
        setRedemptions([...redemptions]);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
  };

  // Batch mark selected redemptions as sent / unsent
  // 批量标记勾选项：sent=true 标记为已发放，false 取消标记
  // 注意：操作成功后保留勾选状态（表格行勾选为非受控，视觉勾选不会因刷新而清除），
  // 以便对同一批连续执行「标记 → 取消标记」等操作
  const batchMarkSent = async (sent) => {
    if (selectedKeys.length === 0) {
      showError(t('请至少选择一个兑换码！'));
      return;
    }
    const ids = selectedKeys.map((item) => item.id);
    setLoading(true);
    try {
      const res = await API.put(`${apiPrefix}/sent`, { ids, sent });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('操作成功完成！'));
        // 刷新以反映最新发放状态；只用到勾选行的 id，旧对象引用无影响
        await refresh();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
    setLoading(false);
  };

  // Reload list when sent filter changes (skip initial mount to avoid duplicate load)
  // 发放筛选变化时回到第 1 页重新加载；首帧跳过，避免与初始化加载重复请求
  const sentFilterInitialized = useRef(false);
  useEffect(() => {
    if (!sentFilterInitialized.current) {
      sentFilterInitialized.current = true;
      return;
    }
    setActivePage(1);
    loadRedemptions(1, pageSize, sentFilter);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sentFilter]);

  // Close edit modal
  const closeEdit = () => {
    setShowEdit(false);
    setTimeout(() => {
      setEditingRedemption({
        id: undefined,
      });
    }, 500);
  };

  // Remove record (for UI update after deletion)
  const removeRecord = (key) => {
    let newDataSource = [...redemptions];
    if (key != null) {
      let idx = newDataSource.findIndex((data) => data.key === key);
      if (idx > -1) {
        newDataSource.splice(idx, 1);
        setRedemptions(newDataSource);
      }
    }
  };

  // Initialize data loading
  useEffect(() => {
    loadRedemptions(1, pageSize)
      .then()
      .catch((reason) => {
        showError(reason);
      });
  }, [pageSize]);

  return {
    // Data state
    redemptions,
    loading,
    searching,
    activePage,
    pageSize,
    tokenCount,
    selectedKeys,
    displaySymbol,
    apiPrefix,

    // Sent mark state（发放标记状态与操作）
    sentFilter,
    setSentFilter,

    // Edit state
    editingRedemption,
    showEdit,

    // Form state
    formApi,
    formInitValues,

    // UI state
    compactMode,
    setCompactMode,

    // Data operations
    loadRedemptions,
    searchRedemptions,
    manageRedemption,
    refresh,
    copyText,
    removeRecord,

    // State updates
    setActivePage,
    setSelectedKeys,
    setEditingRedemption,
    setShowEdit,
    setFormApi,
    setLoading,

    // Event handlers
    handlePageChange,
    rowSelection,
    handleRow,
    closeEdit,
    getFormValues,

    // Batch operations（批量操作）
    batchCopyRedemptions,
    batchDeleteRedemptions,
    batchMarkSent,

    // Sent mark operations（单行发放开关）
    toggleSent,

    // Translation function
    t,
  };
};
