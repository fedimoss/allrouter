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

import React, { useEffect, useRef, useState } from 'react';
import {
  Notification,
  Button,
  Space,
  Toast,
  Select,
  Dropdown,
  Modal,
  Empty,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  API,
  renderQuota,
  showError,
  getModelCategories,
  selectFilter,
  timestamp2string,
} from '../../../helpers';
import EditTokenModal from './modals/EditTokenModal';
import CCSwitchModal from './modals/CCSwitchModal';
import CopyTokensModal from './modals/CopyTokensModal';
import DeleteTokensModal from './modals/DeleteTokensModal';
import { useTokensData } from '../../../hooks/tokens/useTokensData';
import {
  AlertCircle,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Copy,
  Eye,
  EyeOff,
  Info,
  KeyRound,
  MessageSquare,
  PenLine,
  Plus,
  RefreshCw,
  Search,
  Trash2,
  X,
  EllipsisVertical
} from 'lucide-react';

const DEFAULT_GROUP_FILTER = '__default__';
const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];

const buildSearchCriteria = (rawQuery) => {
  const query = rawQuery.trim();
  if (!query) {
    return {
      searchKeyword: '',
      searchToken: '',
    };
  }

  const hasWildcard = query.includes('%');
  const allowFuzzy = !hasWildcard && query.length >= 2;

  if (query.startsWith('sk-')) {
    return {
      searchKeyword: '',
      searchToken: allowFuzzy ? `${query}%` : query,
    };
  }

  return {
    searchKeyword: allowFuzzy ? `%${query}%` : query,
    searchToken: '',
  };
};

const normalizeList = (value, separator = ',') => {
  if (!value) {
    return [];
  }

  return String(value)
    .split(separator)
    .map((item) => item.trim())
    .filter(Boolean);
};

const getTokenStatusKey = (record) => {
  const now = Math.floor(Date.now() / 1000);

  if (record.status === 2) {
    return 'disabled';
  }

  if (record.status === 4) {
    return 'exhausted';
  }

  if (record.status === 3) {
    return 'expired';
  }

  if (record.expired_time !== -1 && record.expired_time <= now) {
    return 'expired';
  }

  return 'active';
};

const getTokenStatusMeta = (record, t) => {
  const statusKey = getTokenStatusKey(record);

  switch (statusKey) {
    case 'disabled':
      return {
        key: statusKey,
        label: t('已禁用'),
        className: 'token-v2-status-badge token-v2-status-disabled',
      };
    case 'expired':
      return {
        key: statusKey,
        label: t('已过期'),
        className: 'token-v2-status-badge token-v2-status-expired',
      };
    case 'exhausted':
      return {
        key: statusKey,
        label: t('已耗尽'),
        className: 'token-v2-status-badge token-v2-status-exhausted',
      };
    default:
      return {
        key: 'active',
        label: t('活跃'),
        className: 'token-v2-status-badge token-v2-status-active',
      };
  }
};

const getQuotaMeta = (record) => {
  const remain = Number(record.remain_quota) || 0;
  const used = Number(record.used_quota) || 0;
  const total = remain + used;
  const percent =
    total > 0 ? Math.max(0, Math.min(100, (remain / total) * 100)) : 0;

  let toneClassName = 'token-v2-progress-safe';
  if (percent <= 10) {
    toneClassName = 'token-v2-progress-danger';
  } else if (percent <= 30) {
    toneClassName = 'token-v2-progress-warning';
  }

  return {
    remain,
    total,
    percent,
    toneClassName,
  };
};

const getExpireMeta = (record, t) => {
  if (record.expired_time === -1) {
    return {
      text: t('永不过期'),
      warning: false,
    };
  }

  const now = Math.floor(Date.now() / 1000);
  const delta = record.expired_time - now;

  if (delta <= 0) {
    return {
      text: timestamp2string(record.expired_time),
      warning: false,
    };
  }

  if (delta <= 7 * 24 * 60 * 60) {
    if (delta < 24 * 60 * 60) {
      const hours = Math.max(1, Math.ceil(delta / (60 * 60)));
      return {
        text: t('{{count}}小时后过期', { count: hours }),
        warning: true,
      };
    }

    const days = Math.max(1, Math.ceil(delta / (24 * 60 * 60)));
    return {
      text: t('{{count}}天后过期', { count: days }),
      warning: true,
    };
  }

  return {
    text: timestamp2string(record.expired_time),
    warning: false,
  };
};

const getGroupLabel = (record, groupLabelMap, t) => {
  if (!record.group) {
    return t('默认分组');
  }

  if (record.group === 'auto') {
    return record.cross_group_retry ? t('智能熔断（跨组）') : t('智能熔断');
  }

  return groupLabelMap[record.group] || record.group;
};

const getPaginationItems = (currentPage, totalPages) => {
  if (totalPages <= 1) {
    return [1];
  }

  const items = [];
  const start = Math.max(1, currentPage - 1);
  const end = Math.min(totalPages, currentPage + 1);

  if (start > 1) {
    items.push(1);
  }

  if (start > 2) {
    items.push('left-ellipsis');
  }

  for (let page = start; page <= end; page += 1) {
    items.push(page);
  }

  if (end < totalPages - 1) {
    items.push('right-ellipsis');
  }

  if (end < totalPages) {
    items.push(totalPages);
  }

  return items;
};

function TokenDetailModal({ token, groupLabelMap, visible, onClose, t }) {
  const currentToken = token || {};
  const models = normalizeList(currentToken.model_limits);
  const ips = normalizeList(currentToken.allow_ips || '', '\n');
  const quotaMeta = getQuotaMeta(currentToken);
  const statusMeta = token
    ? getTokenStatusMeta(currentToken, t)
    : {
        className: 'token-v2-status-badge token-v2-status-disabled',
        label: '',
      };

  return (
    <Modal
      centered
      visible={visible}
      onCancel={onClose}
      footer={null}
      width='min(560px, calc(100vw - 24px))'
      closable={false}
      closeIcon={null}
      maskClosable={false}
      className='token-v2-detail-modal'
    >
      <div className='token-v2-detail-card'>
        <div className='token-v2-detail-header'>
          <div>
            <h3 className='token-v2-detail-title'>{t('令牌详情')}</h3>
            <p className='token-v2-detail-subtitle'>
              {t('查看当前令牌的详细配置与限制信息。')}
            </p>
          </div>
          <button
            type='button'
            className='token-v2-icon-button'
            onClick={onClose}
            aria-label={t('关闭')}
          >
            <X size={16} />
          </button>
        </div>

        <div className='token-v2-detail-grid'>
          <div className='token-v2-detail-item'>
            <span className='token-v2-detail-label'>{t('名称')}</span>
            <span className='token-v2-detail-value'>{currentToken.name || '-'}</span>
          </div>
          <div className='token-v2-detail-item'>
            <span className='token-v2-detail-label'>{t('状态')}</span>
            {token ? (
              <span className={statusMeta.className}>{statusMeta.label}</span>
            ) : (
              <span className='token-v2-detail-value'>-</span>
            )}
          </div>
          <div className='token-v2-detail-item'>
            <span className='token-v2-detail-label'>{t('令牌分组')}</span>
            <span className='token-v2-detail-value'>
              {token ? getGroupLabel(currentToken, groupLabelMap, t) : '-'}
            </span>
          </div>
          <div className='token-v2-detail-item'>
            <span className='token-v2-detail-label'>{t('剩余额度/总额度')}</span>
            <span className='token-v2-detail-value'>
              {!token
                ? '-'
                : currentToken.unlimited_quota
                ? t('无限额度')
                : `${renderQuota(quotaMeta.remain)} / ${renderQuota(quotaMeta.total)}`}
            </span>
          </div>
          <div className='token-v2-detail-item'>
            <span className='token-v2-detail-label'>{t('创建时间')}</span>
            <span className='token-v2-detail-value'>
              {token ? timestamp2string(currentToken.created_time) : '-'}
            </span>
          </div>
          <div className='token-v2-detail-item'>
            <span className='token-v2-detail-label'>{t('过期时间')}</span>
            <span className='token-v2-detail-value'>
              {!token
                ? '-'
                : currentToken.expired_time === -1
                ? t('永不过期')
                : timestamp2string(currentToken.expired_time)}
            </span>
          </div>
        </div>

        <div className='token-v2-detail-section'>
          <span className='token-v2-detail-label'>{t('模型限制列表')}</span>
          {models.length > 0 ? (
            <div className='token-v2-detail-tags'>
              {models.map((model) => (
                <span key={model} className='token-v2-inline-tag'>
                  {model}
                </span>
              ))}
            </div>
          ) : (
            <div className='token-v2-detail-muted'>{t('无限制')}</div>
          )}
        </div>

        <div className='token-v2-detail-section'>
          <span className='token-v2-detail-label'>
            {t('IP白名单（支持CIDR表达式）')}
          </span>
          {ips.length > 0 ? (
            <div className='token-v2-detail-tags'>
              {ips.map((ip) => (
                <span key={ip} className='token-v2-inline-tag'>
                  {ip}
                </span>
              ))}
            </div>
          ) : (
            <div className='token-v2-detail-muted'>{t('无限制')}</div>
          )}
        </div>

        <div className='token-v2-detail-footer'>
          <button
            type='button'
            className='token-v2-secondary-button token-v2-detail-close-button'
            onClick={onClose}
          >
            {t('关闭')}
          </button>
        </div>
      </div>
    </Modal>
  );
}

function TokensPage() {
  const openFluentNotificationRef = useRef(null);
  const openCCSwitchModalRef = useRef(null);
  const selectAllRef = useRef(null);
  const tokensData = useTokensData(
    (key) => openFluentNotificationRef.current?.(key),
    (key) => openCCSwitchModalRef.current?.(key),
  );
  const latestRef = useRef({
    tokens: [],
    selectedKeys: [],
    t: (k) => k,
    selectedModel: '',
    prefillKey: '',
    fetchTokenKey: async () => '',
  });
  const [modelOptions, setModelOptions] = useState([]);
  const [groupFilters, setGroupFilters] = useState([]);
  const [selectedModel, setSelectedModel] = useState('');
  const [fluentNoticeOpen, setFluentNoticeOpen] = useState(false);
  const [prefillKey, setPrefillKey] = useState('');
  const [ccSwitchVisible, setCCSwitchVisible] = useState(false);
  const [ccSwitchKey, setCCSwitchKey] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [groupFilter, setGroupFilter] = useState('');
  const [showCopyModal, setShowCopyModal] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [detailToken, setDetailToken] = useState(null);
  const [detailVisible, setDetailVisible] = useState(false);

  useEffect(() => {
    latestRef.current = {
      tokens: tokensData.tokens,
      selectedKeys: tokensData.selectedKeys,
      t: tokensData.t,
      selectedModel,
      prefillKey,
      fetchTokenKey: tokensData.fetchTokenKey,
    };
  }, [
    tokensData.tokens,
    tokensData.selectedKeys,
    tokensData.t,
    selectedModel,
    prefillKey,
    tokensData.fetchTokenKey,
  ]);

  const loadModels = async () => {
    try {
      const res = await API.get('/api/user/models');
      const { success, message, data } = res.data || {};
      if (success) {
        const categories = getModelCategories(tokensData.t);
        const options = (data || []).map((model) => {
          let icon = null;
          for (const [key, category] of Object.entries(categories)) {
            if (key !== 'all' && category.filter({ model_name: model })) {
              icon = category.icon;
              break;
            }
          }
          return {
            label: (
              <span className='flex items-center gap-1'>
                {icon}
                {model}
              </span>
            ),
            value: model,
          };
        });
        setModelOptions(options);
      } else {
        showError(tokensData.t(message));
      }
    } catch (error) {
      showError(error.message || 'Failed to load models');
    }
  };

  const loadGroupFilters = async () => {
    try {
      const res = await API.get('/api/user/self/groups');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(tokensData.t(message));
        return;
      }

      const options = Object.entries(data || {})
        .filter(([value]) => value)
        .map(([value, info]) => ({
          value,
          label: info?.desc || value,
        }));
      setGroupFilters(options);
    } catch (error) {
      showError(error.message || tokensData.t('加载分组失败'));
    }
  };

  useEffect(() => {
    loadGroupFilters();
  }, []);

  function openFluentNotification(key) {
    const { t } = latestRef.current;
    const suppressKey = 'fluent_notify_suppressed';
    if (modelOptions.length === 0) {
      loadModels();
    }
    if (!key && localStorage.getItem(suppressKey) === '1') {
      return;
    }
    const container = document.getElementById('fluent-new-api-container');
    if (!container) {
      Toast.warning(t('未检测到 FluentRead（流畅阅读），请确认扩展已启用'));
      return;
    }
    setPrefillKey(key || '');
    setFluentNoticeOpen(true);
    Notification.info({
      id: 'fluent-detected',
      title: t('检测到 FluentRead（流畅阅读）'),
      content: (
        <div>
          <div style={{ marginBottom: 8 }}>
            {key
              ? t('请选择模型。')
              : t('选择模型后可一键填充当前选中令牌（或本页第一个令牌）。')}
          </div>
          <div style={{ marginBottom: 8 }}>
            <Select
              placeholder={t('请选择模型')}
              optionList={modelOptions}
              onChange={setSelectedModel}
              filter={selectFilter}
              style={{ width: 320 }}
              showClear
              searchable
              emptyContent={t('暂无数据')}
            />
          </div>
          <Space>
            <Button
              theme='solid'
              type='primary'
              onClick={handlePrefillToFluent}
            >
              {t('一键填充到 FluentRead')}
            </Button>
            {!key && (
              <Button
                type='warning'
                onClick={() => {
                  localStorage.setItem(suppressKey, '1');
                  Notification.close('fluent-detected');
                  Toast.info(t('已关闭后续提醒'));
                }}
              >
                {t('不再提醒')}
              </Button>
            )}
            <Button
              type='tertiary'
              onClick={() => Notification.close('fluent-detected')}
            >
              {t('关闭')}
            </Button>
          </Space>
        </div>
      ),
      duration: 0,
    });
  }

  openFluentNotificationRef.current = openFluentNotification;

  function openCCSwitchModal(key) {
    if (modelOptions.length === 0) {
      loadModels();
    }
    setCCSwitchKey(key || '');
    setCCSwitchVisible(true);
  }

  openCCSwitchModalRef.current = openCCSwitchModal;

  const handlePrefillToFluent = async () => {
    const {
      tokens,
      selectedKeys,
      t,
      selectedModel: chosenModel,
      prefillKey: overrideKey,
      fetchTokenKey,
    } = latestRef.current;
    const container = document.getElementById('fluent-new-api-container');
    if (!container) {
      Toast.error(t('未检测到 Fluent 容器'));
      return;
    }

    if (!chosenModel) {
      Toast.warning(t('请选择模型'));
      return;
    }

    let status = localStorage.getItem('status');
    let serverAddress = '';
    if (status) {
      try {
        status = JSON.parse(status);
        serverAddress = status.server_address || '';
      } catch (_) {}
    }
    if (!serverAddress) {
      serverAddress = window.location.origin;
    }

    let apiKeyToUse = '';
    if (overrideKey) {
      apiKeyToUse = 'sk-' + overrideKey;
    } else {
      const token =
        selectedKeys && selectedKeys.length === 1
          ? selectedKeys[0]
          : tokens && tokens.length > 0
            ? tokens[0]
            : null;
      if (!token) {
        Toast.warning(t('没有可用令牌用于填充'));
        return;
      }
      try {
        apiKeyToUse = 'sk-' + (await fetchTokenKey(token));
      } catch (_) {
        return;
      }
    }

    const payload = {
      id: 'new-api',
      baseUrl: serverAddress,
      apiKey: apiKeyToUse,
      model: chosenModel,
    };

    container.dispatchEvent(
      new CustomEvent('fluent:prefill', { detail: payload }),
    );
    Toast.success(t('已发送到 Fluent'));
    Notification.close('fluent-detected');
  };

  useEffect(() => {
    const onAppeared = () => {
      openFluentNotification();
    };
    const onRemoved = () => {
      setFluentNoticeOpen(false);
      Notification.close('fluent-detected');
    };

    window.addEventListener('fluent-container:appeared', onAppeared);
    window.addEventListener('fluent-container:removed', onRemoved);
    return () => {
      window.removeEventListener('fluent-container:appeared', onAppeared);
      window.removeEventListener('fluent-container:removed', onRemoved);
    };
  }, []);

  useEffect(() => {
    if (fluentNoticeOpen) {
      openFluentNotification();
    }
  }, [modelOptions, selectedModel, tokensData.t, fluentNoticeOpen]);

  useEffect(() => {
    const selector = '#fluent-new-api-container';
    const root = document.body || document.documentElement;

    const existing = document.querySelector(selector);
    if (existing) {
      window.dispatchEvent(
        new CustomEvent('fluent-container:appeared', { detail: existing }),
      );
    }

    const isOrContainsTarget = (node) => {
      if (!(node && node.nodeType === 1)) {
        return false;
      }
      if (node.id === 'fluent-new-api-container') {
        return true;
      }
      return (
        typeof node.querySelector === 'function' &&
        !!node.querySelector(selector)
      );
    };

    const observer = new MutationObserver((mutations) => {
      for (const mutation of mutations) {
        for (const added of mutation.addedNodes) {
          if (isOrContainsTarget(added)) {
            const element = document.querySelector(selector);
            if (element) {
              window.dispatchEvent(
                new CustomEvent('fluent-container:appeared', {
                  detail: element,
                }),
              );
            }
            break;
          }
        }

        for (const removed of mutation.removedNodes) {
          if (isOrContainsTarget(removed)) {
            const currentElement = document.querySelector(selector);
            if (!currentElement) {
              window.dispatchEvent(new CustomEvent('fluent-container:removed'));
            }
            break;
          }
        }
      }
    });

    observer.observe(root, { childList: true, subtree: true });
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    const currentCriteria = tokensData.searchCriteria || {};
    if (currentCriteria.searchKeyword) {
      setSearchQuery(currentCriteria.searchKeyword.replaceAll('%', ''));
      return;
    }
    if (currentCriteria.searchToken) {
      setSearchQuery(currentCriteria.searchToken.replaceAll('%', ''));
      return;
    }
    setSearchQuery('');
  }, [tokensData.searchCriteria]);

  const currentGroupOptions = [...groupFilters];
  const currentGroupValues = new Set(
    currentGroupOptions.map((item) => item.value),
  );
  if (
    tokensData.tokens.some((token) => !token.group) &&
    !currentGroupValues.has(DEFAULT_GROUP_FILTER)
  ) {
    currentGroupOptions.unshift({
      value: DEFAULT_GROUP_FILTER,
      label: tokensData.t('默认分组'),
    });
    currentGroupValues.add(DEFAULT_GROUP_FILTER);
  }
  tokensData.tokens.forEach((token) => {
    if (token.group && !currentGroupValues.has(token.group)) {
      currentGroupOptions.push({
        value: token.group,
        label: getGroupLabel(token, {}, tokensData.t),
      });
      currentGroupValues.add(token.group);
    }
  });

  const groupLabelMap = {};
  currentGroupOptions.forEach((item) => {
    if (item.value && item.value !== DEFAULT_GROUP_FILTER) {
      groupLabelMap[item.value] = item.label;
    }
  });

  const displayTokens = tokensData.tokens.filter((token) => {
    const matchesStatus =
      !statusFilter || getTokenStatusKey(token) === statusFilter;
    const matchesGroup =
      !groupFilter
        ? true
        : groupFilter === DEFAULT_GROUP_FILTER
          ? !token.group
          : token.group === groupFilter;
    return matchesStatus && matchesGroup;
  });

  useEffect(() => {
    const visibleIds = new Set(displayTokens.map((token) => token.id));
    const nextSelected = tokensData.selectedKeys.filter((token) =>
      visibleIds.has(token.id),
    );

    if (nextSelected.length !== tokensData.selectedKeys.length) {
      tokensData.setSelectedKeys(nextSelected);
    }
  }, [tokensData.tokens, statusFilter, groupFilter]);

  const selectedIds = new Set(tokensData.selectedKeys.map((token) => token.id));
  const selectedVisibleCount = displayTokens.filter((token) =>
    selectedIds.has(token.id),
  ).length;
  const allVisibleSelected =
    displayTokens.length > 0 && selectedVisibleCount === displayTokens.length;
  const isPartiallySelected =
    selectedVisibleCount > 0 && selectedVisibleCount < displayTokens.length;

  useEffect(() => {
    if (selectAllRef.current) {
      selectAllRef.current.indeterminate = isPartiallySelected;
    }
  }, [isPartiallySelected]);

  const rangeStart =
    tokensData.tokenCount === 0
      ? 0
      : (tokensData.activePage - 1) * tokensData.pageSize + 1;
  const rangeEnd = Math.min(
    tokensData.activePage * tokensData.pageSize,
    tokensData.tokenCount,
  );
  const totalPages = Math.max(
    1,
    Math.ceil(tokensData.tokenCount / tokensData.pageSize),
  );
  const paginationItems = getPaginationItems(tokensData.activePage, totalPages);

  const refreshCurrentView = async () => {
    const hasSearch =
      !!tokensData.searchCriteria?.searchKeyword ||
      !!tokensData.searchCriteria?.searchToken;

    if (hasSearch) {
      await tokensData.searchTokens(
        tokensData.activePage,
        tokensData.pageSize,
        tokensData.searchCriteria,
      );
      return;
    }

    await tokensData.loadTokens(tokensData.activePage, tokensData.pageSize);
  };

  const handleSearchSubmit = async (event) => {
    event.preventDefault();
    await tokensData.searchTokens(
      1,
      tokensData.pageSize,
      buildSearchCriteria(searchQuery),
    );
  };

  const handleSearchChange = async (event) => {
    const value = event.target.value;
    setSearchQuery(value);
    if (!value.trim()) {
      await tokensData.searchTokens(1, tokensData.pageSize, {
        searchKeyword: '',
        searchToken: '',
      });
    }
  };

  const handleToggleSelection = (record, checked) => {
    if (checked) {
      const nextSelected = [...tokensData.selectedKeys];
      if (!nextSelected.some((token) => token.id === record.id)) {
        nextSelected.push(record);
      }
      tokensData.setSelectedKeys(nextSelected);
      return;
    }

    tokensData.setSelectedKeys(
      tokensData.selectedKeys.filter((token) => token.id !== record.id),
    );
  };

  const handleSelectAll = (checked) => {
    if (checked) {
      tokensData.setSelectedKeys(displayTokens);
      return;
    }
    tokensData.setSelectedKeys([]);
  };

  const openCopyModal = () => {
    if (tokensData.selectedKeys.length === 0) {
      showError(tokensData.t('请至少选择一个令牌！'));
      return;
    }
    setShowCopyModal(true);
  };

  const openDeleteModal = () => {
    if (tokensData.selectedKeys.length === 0) {
      showError(tokensData.t('请至少选择一个令牌！'));
      return;
    }
    setShowDeleteModal(true);
  };

  const handleConfirmDelete = async () => {
    await tokensData.batchDeleteTokens();
    setShowDeleteModal(false);
  };

  const handleOpenCreate = () => {
    tokensData.setEditingToken({
      id: undefined,
    });
    tokensData.setShowEdit(true);
  };

  const getChatMenuItems = (record) => {
    try {
      const raw = localStorage.getItem('chats');
      const parsed = JSON.parse(raw);
      if (!Array.isArray(parsed)) {
        return [];
      }

      return parsed
        .map((item, index) => {
          const name = Object.keys(item || {})[0];
          if (!name) {
            return null;
          }

          return {
            node: 'item',
            key: `${record.id}-${index}`,
            name,
            onClick: () => tokensData.onOpenLink(name, item[name], record),
          };
        })
        .filter(Boolean);
    } catch (_) {
      showError(tokensData.t('聊天链接配置错误，请联系管理员'));
      return [];
    }
  };

  const handlePrimaryChat = (record) => {
    const items = getChatMenuItems(record);
    if (items.length === 0) {
      showError(tokensData.t('请联系管理员配置聊天链接'));
      return;
    }

    items[0].onClick();
  };

  const handleDeleteRecord = (record) => {
    Modal.confirm({
      title: tokensData.t('确定是否要删除此令牌？'),
      content: tokensData.t('此修改将不可逆'),
      onOk: async () => {
        await tokensData.manageToken(record.id, 'delete', record);
        await refreshCurrentView();
      },
    });
  };

  const handleOpenDetail = (record) => {
    setDetailToken(record);
    setDetailVisible(true);
  };

  const handleCloseDetail = () => {
    setDetailVisible(false);
  };

  const hasToolbarFilter = !!statusFilter || !!groupFilter;

  return (
    <div className='token-v2'>
      {tokensData.showEdit && (
        <EditTokenModal
          refresh={refreshCurrentView}
          editingToken={tokensData.editingToken}
          visiable={tokensData.showEdit}
          handleClose={tokensData.closeEdit}
        />
      )}

      {ccSwitchVisible && (
        <CCSwitchModal
          visible={ccSwitchVisible}
          onClose={() => setCCSwitchVisible(false)}
          tokenKey={ccSwitchKey}
          modelOptions={modelOptions}
        />
      )}

      {showCopyModal && (
        <CopyTokensModal
          visible={showCopyModal}
          onCancel={() => setShowCopyModal(false)}
          batchCopyTokens={tokensData.batchCopyTokens}
          t={tokensData.t}
        />
      )}

      {showDeleteModal && (
        <DeleteTokensModal
          visible={showDeleteModal}
          onCancel={() => setShowDeleteModal(false)}
          onConfirm={handleConfirmDelete}
          selectedKeys={tokensData.selectedKeys}
          t={tokensData.t}
        />
      )}

      <TokenDetailModal
        visible={detailVisible}
        token={detailToken}
        groupLabelMap={groupLabelMap}
        onClose={handleCloseDetail}
        t={tokensData.t}
      />

      <div className='token-v2-shell'>
        <div className='token-v2-card'>
          <div className='token-v2-toolbar'>
            <div className='token-v2-toolbar-title'>
              {tokensData.t('令牌管理')}
            </div>
            <div className='token-v2-toolbar-desc'>
              <div className='token-v2-toolbar-desc-txt'>{tokensData.t('管理您的 API 访问令牌，监控配额使用情况及分组权限。')}</div>
              <div className='token-v2-toolbar-actions'>
                <button
                  type='button'
                  className='token-v2-primary-button'
                  onClick={handleOpenCreate}
                >
                  <Plus size={16} />
                  {tokensData.t('添加令牌')}
                </button>

                <button
                  type='button'
                  className='token-v2-icon-button'
                  onClick={refreshCurrentView}
                  title={tokensData.t('刷新列表')}
                  aria-label={tokensData.t('刷新列表')}
                >
                  <RefreshCw
                    size={16}
                    className={tokensData.loading ? 'token-v2-spin' : ''}
                  />{tokensData.t('刷新')}
                </button>
              </div>
            </div>
            <div className='token-v2-toolbar-row'>
              <form
                className='token-v2-toolbar-filters'
                onSubmit={handleSearchSubmit}
              >
                <label className='token-v2-search-field'>
                  <Search size={16} />
                  <input
                    type='text'
                    value={searchQuery}
                    onChange={handleSearchChange}
                    placeholder={tokensData.t('搜索名称或密钥前缀...')}
                  />
                </label>

                <Select
                  className='token-v2-select'
                  value={statusFilter}
                  onChange={val => setStatusFilter(val)}
                >
                  <Select.Option value=''>{tokensData.t('所有状态')}</Select.Option>
                  <Select.Option value='active'>{tokensData.t('活跃')}</Select.Option>
                  <Select.Option value='expired'>{tokensData.t('已过期')}</Select.Option>
                  <Select.Option value='disabled'>{tokensData.t('已禁用')}</Select.Option>
                  <Select.Option value='exhausted'>{tokensData.t('已耗尽')}</Select.Option>
                </Select>

                <Select
                  className='token-v2-select'
                  value={groupFilter}
                  onChange={val => setGroupFilter(val)}
                >
                  <Select.Option value=''>{tokensData.t('全部分组')}</Select.Option>
                  {currentGroupOptions.map((group) => (
                    <Select.Option key={group.value} value={group.value}>
                      {group.label}
                    </Select.Option>
                  ))}
                </Select>
              </form>
            </div>

            {hasToolbarFilter && (
              <div className='token-v2-toolbar-hint'>
                <Info size={14} />
                {tokensData.t('当前筛选仅作用于当前页已加载的数据。')}
              </div>
            )}
          </div>

          <div className='token-v2-table-area'>
            <div className='token-v2-table-scroll'>
              {displayTokens.length > 0 ? (
                <table className='token-v2-table'>
                  <thead>
                    <tr>
                      {/* <th className='token-v2-checkbox-col'>
                        <input
                          ref={selectAllRef}
                          type='checkbox'
                          checked={allVisibleSelected}
                          onChange={(event) =>
                            handleSelectAll(event.target.checked)
                          }
                        />
                      </th> */}
                      <th>{tokensData.t('名称')}</th>
                      <th className='token-v2-col-status'>{tokensData.t('状态')}</th>
                      <th className='token-v2-col-quota'>{tokensData.t('剩余额度 / 总额度')}</th>
                      <th className='token-v2-col-group'>{tokensData.t('分组')}</th>
                      <th className='token-v2-col-key'>{tokensData.t('密钥 (Key)')}</th>
                      <th className='token-v2-col-date'>
                        <div>{tokensData.t('创建时间')}</div>
                        <div>{tokensData.t('过期时间')}</div>
                      </th>
                      <th className='token-v2-actions-col'>
                        {tokensData.t('操作')}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {displayTokens.map((record) => {
                      const chatItems = getChatMenuItems(record);
                      const quotaMeta = getQuotaMeta(record);
                      const statusMeta = getTokenStatusMeta(record, tokensData.t);
                      const expireMeta = getExpireMeta(record, tokensData.t);
                      const isSelected = selectedIds.has(record.id);
                      const isKeyVisible = !!tokensData.showKeys[record.id];
                      const isLoadingKey = !!tokensData.loadingTokenKeys[record.id];
                      const resolvedKey =
                        isKeyVisible && tokensData.resolvedTokenKeys[record.id]
                          ? tokensData.resolvedTokenKeys[record.id]
                          : record.key || '';
                      const displayedKey = resolvedKey ? `sk-${resolvedKey}` : '';

                      return (
                        <tr
                          key={record.id}
                          className={isSelected ? 'token-v2-row-selected' : ''}
                        >
                          {/* <td className='token-v2-checkbox-col'>
                            <input
                              type='checkbox'
                              checked={isSelected}
                              onChange={(event) =>
                                handleToggleSelection(
                                  record,
                                  event.target.checked,
                                )
                              }
                            />
                          </td> */}
                          <td>
                            <div className='token-v2-name-cell'>
                              <div className='token-v2-name-title'>{record.name}</div>
                              <div className='token-v2-name-subtitle'>
                                {record.unlimited_quota
                                  ? tokensData.t('无限额度')
                                  : tokensData.t('剩余 {{remain}}', {
                                      remain: renderQuota(quotaMeta.remain),
                                    })}
                              </div>
                            </div>
                          </td>
                          <td className='token-v2-col-status'>
                            <span className={statusMeta.className}>
                              {statusMeta.label}
                            </span>
                          </td>
                          <td className='token-v2-col-quota'>
                            {record.unlimited_quota ? (
                              <span className='token-v2-inline-tag'>
                                {tokensData.t('无限额度')}
                              </span>
                            ) : (
                              <div className='token-v2-quota-cell'>
                                <div className='token-v2-quota-meter'>
                                  <span className='token-v2-quota-percent'>
                                    {`${Math.round(quotaMeta.percent)}%`}
                                  </span>
                                  <div className='token-v2-progress-track'>
                                    <div
                                      className={`token-v2-progress-fill ${quotaMeta.toneClassName}`}
                                      style={{ width: `${quotaMeta.percent}%` }}
                                    />
                                  </div>
                                </div>
                                <div className='token-v2-quota-text'>
                                  {`${renderQuota(quotaMeta.remain)} / ${renderQuota(quotaMeta.total)}`}
                                </div>
                              </div>
                            )}
                          </td>
                          <td className='token-v2-col-group'>
                            <span className='token-v2-group-chip'>
                              {getGroupLabel(record, groupLabelMap, tokensData.t)}
                            </span>
                          </td>
                          <td className='token-v2-col-key'>
                            <div className='token-v2-key-cell'>
                              <code
                                className={
                                  isKeyVisible
                                    ? 'token-v2-key-box token-v2-key-box-visible'
                                    : 'token-v2-key-box'
                                }
                                title={displayedKey}
                              >
                                {displayedKey}
                              </code>
                              <div className='token-v2-key-actions'>
                                {/* <button
                                  type='button'
                                  className='token-v2-icon-button'
                                  onClick={async () =>
                                    tokensData.toggleTokenVisibility(record)
                                  }
                                  title={tokensData.t('显示/隐藏密钥')}
                                  aria-label={tokensData.t('显示/隐藏密钥')}
                                >
                                  {isLoadingKey ? (
                                    <RefreshCw size={16} className='token-v2-spin' />
                                  ) : isKeyVisible ? (
                                    <EyeOff size={16} />
                                  ) : (
                                    <Eye size={16} />
                                  )}
                                </button> */}
                                <button
                                  type='button'
                                  className='token-v2-icon-button'
                                  onClick={async () => tokensData.copyTokenKey(record)}
                                  title={tokensData.t('复制密钥')}
                                  aria-label={tokensData.t('复制密钥')}
                                >
                                  <Copy size={14} />
                                </button>
                              </div>
                            </div>
                          </td>
                          <td className='token-v2-date-cell token-v2-col-date'>
                            {timestamp2string(record.created_time)}
                            <div>
                              {expireMeta.warning ? (
                              <span className='token-v2-expire-warning'>
                                <AlertCircle size={13} />
                                {expireMeta.text}
                              </span>
                            ) : (
                              expireMeta.text
                            )}
                            </div>
                          </td>
                          <td className='token-v2-actions-col'>
                            <div className='token-v2-row-actions'>
                              <div className='token-v2-split-action'>
                                <button
                                  type='button'
                                  className='token-v2-action-button'
                                  onClick={() => handlePrimaryChat(record)}
                                >
                                  <MessageSquare size={14} />
                                  {tokensData.t('聊天')}
                                </button>
                                <Dropdown
                                  trigger='click'
                                  position='bottomRight'
                                  menu={chatItems}
                                >
                                  <button
                                    type='button'
                                    className='token-v2-action-chevron'
                                    aria-label={tokensData.t('聊天下拉菜单')}
                                  >
                                    <ChevronDown size={14} />
                                  </button>
                                </Dropdown>
                              </div>
                              <button
                                type='button'
                                className='token-v2-action-button'
                                onClick={() => {
                                  tokensData.setEditingToken(record);
                                  tokensData.setShowEdit(true);
                                }}
                              >
                                <PenLine size={14} />
                              </button>
                              <button
                                type='button'
                                className='token-v2-action-button token-v2-action-danger'
                                onClick={() => handleDeleteRecord(record)}
                              >
                                <Trash2 size={14} />
                              </button>

                              <Dropdown
                                  render={
                                  <Dropdown.Menu>
                                    <Dropdown.Item>
                                      <button
                                        type='button'
                                        className='token-v2-action-button'
                                        onClick={() => handleOpenDetail(record)}
                                      >
                                        {tokensData.t('详情')}
                                      </button>
                                    </Dropdown.Item>
                                    <Dropdown.Item>
                                      {record.status === 1 ? (
                                        <button
                                          type='button'
                                          className='token-v2-action-button'
                                          onClick={async () => {
                                            await tokensData.manageToken(
                                              record.id,
                                              'disable',
                                              record,
                                            );
                                            await refreshCurrentView();
                                          }}
                                        >
                                          {tokensData.t('禁用')}
                                        </button>
                                      ) : (
                                        <button
                                          type='button'
                                          className='token-v2-action-button token-v2-action-success'
                                          onClick={async () => {
                                            await tokensData.manageToken(
                                              record.id,
                                              'enable',
                                              record,
                                            );
                                            await refreshCurrentView();
                                          }}
                                        >
                                          {tokensData.t('启用')}
                                        </button>
                                      )}
                                    </Dropdown.Item>
                                  </Dropdown.Menu>
                                }
                              >
                                <Button theme='borderless' type='tertiary' icon={<EllipsisVertical />} />
                              </Dropdown>
                            </div>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              ) : (
                <div className='token-v2-empty-state'>
                  <Empty
                    image={
                      <IllustrationNoResult style={{ width: 140, height: 140 }} />
                    }
                    darkModeImage={
                      <IllustrationNoResultDark
                        style={{ width: 140, height: 140 }}
                      />
                    }
                    title={tokensData.t('暂无令牌')}
                    description={
                      hasToolbarFilter || searchQuery.trim()
                        ? tokensData.t('当前筛选条件下没有匹配的令牌。')
                        : tokensData.t(
                            '您还没有创建任何 API 令牌。创建一个新令牌，开始集成您的 AI 应用吧！',
                          )
                    }
                  />
                  {!hasToolbarFilter && !searchQuery.trim() && (
                    <button
                      type='button'
                      className='token-v2-primary-button token-v2-empty-button'
                      onClick={handleOpenCreate}
                    >
                      <KeyRound size={16} />
                      {tokensData.t('创建第一个令牌')}
                    </button>
                  )}
                </div>
              )}
            </div>
          </div>

          <div className='token-v2-footer'>
            <div className='token-v2-footer-summary'>
              {tokensData.t('显示第 {{start}} 到 {{end}} 条，共 {{total}} 条结果', {
                start: rangeStart,
                end: rangeEnd,
                total: tokensData.tokenCount,
              })}
            </div>

            <div className='token-v2-footer-actions'>
              <label className='token-v2-page-size'>
                <span>{tokensData.t('每页')}</span>
                <Select
                  value={tokensData.pageSize}
                  onChange={val =>
                    tokensData.handlePageSizeChange(Number(val))
                  }
                >
                  {PAGE_SIZE_OPTIONS.map((size) => (
                    <Select.Option key={size} value={size}>
                      {size}
                    </Select.Option>
                  ))}
                </Select>
              </label>

              <nav className='token-v2-pagination' aria-label='Pagination'>
                <button
                  type='button'
                  className='token-v2-page-button'
                  disabled={tokensData.activePage <= 1}
                  onClick={() =>
                    tokensData.handlePageChange(tokensData.activePage - 1)
                  }
                >
                  <ChevronLeft size={14} />
                </button>
                {paginationItems.map((item) =>
                  typeof item === 'number' ? (
                    <button
                      key={item}
                      type='button'
                      className={
                        item === tokensData.activePage
                          ? 'token-v2-page-button token-v2-page-current'
                          : 'token-v2-page-button'
                      }
                      onClick={() => tokensData.handlePageChange(item)}
                    >
                      {item}
                    </button>
                  ) : (
                    <span key={item} className='token-v2-page-ellipsis'>
                      ...
                    </span>
                  ),
                )}
                <button
                  type='button'
                  className='token-v2-page-button'
                  disabled={tokensData.activePage >= totalPages}
                  onClick={() =>
                    tokensData.handlePageChange(tokensData.activePage + 1)
                  }
                >
                  <ChevronRight size={14} />
                </button>
              </nav>
            </div>
          </div>
        </div>
      </div>

      {tokensData.selectedKeys.length > 0 && (
        <div className='token-v2-bottom-bar'>
          <div className='token-v2-bottom-bar-inner'>
            <span className='token-v2-bottom-count'>
              {tokensData.t('{{count}} 项已选择', {
                count: tokensData.selectedKeys.length,
              })}
            </span>
            <span className='token-v2-bottom-divider' />
            <button
              type='button'
              className='token-v2-bottom-action'
              onClick={openCopyModal}
            >
              <Copy size={14} />
              {tokensData.t('复制所选令牌')}
            </button>
            <button
              type='button'
              className='token-v2-bottom-action'
              onClick={openDeleteModal}
            >
              <Trash2 size={14} />
              {tokensData.t('批量删除')}
            </button>
            <button
              type='button'
              className='token-v2-bottom-close'
              onClick={() => tokensData.setSelectedKeys([])}
              aria-label={tokensData.t('关闭批量条')}
            >
              <X size={14} />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

export default TokensPage;
