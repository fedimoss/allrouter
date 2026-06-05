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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Button,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  Upload,
  Pagination,
} from '@douyinfe/semi-ui';
import {
  IconDelete,
  IconEdit,
  IconGiftStroked,
  IconPlus,
  IconRefresh,
  IconUpload,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import {
  API,
  DEFAULT_THEME_PRIMARY_COLOR,
  DEFAULT_THEME_SECONDARY_COLOR,
  isAdmin,
  isProviderOwner,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import ProviderRewardModal from './ProviderRewardModal';
import { useNavigate } from 'react-router-dom';

const { Text } = Typography;

const STATUS_OPTIONS = [
  { label: '启用', value: 1 },
  { label: '禁用', value: 0 },
];

const DOMAIN_STATUS_OPTIONS = [
  { label: '待验证', value: 0 },
  { label: '已验证', value: 1 },
];

const PRICING_TYPE_OPTIONS = [
  { label: '按百分比加价', value: 'ratio' },
  { label: '按固定差价', value: 'delta' },
];

const OWNER_SEARCH_FIELD_OPTIONS = [
  { label: '全部', value: '' },
  { label: '用户名', value: 'username' },
  { label: '显示名', value: 'display_name' },
  { label: '邮箱', value: 'email' },
  { label: '用户 ID', value: 'id' },
];

const OWNER_PAGE_SIZE = 10;

const emptyProvider = {
  owner_user_id: undefined,
  name: '',
  status: 1,
  import_price_ratio: 10,
};

const emptyConfig = {
  site_name: '',
  logo: '',
  theme_color: DEFAULT_THEME_PRIMARY_COLOR,
  secondary_color: DEFAULT_THEME_SECONDARY_COLOR,
  wechat_support: '',
  qq_support: '',
  footer_text: '',
};

const emptyDomain = {
  domain: '',
  status: 0,
  verify_token: '',
};

const createProviderDomainRow = (domain = {}, index = 0) => ({
  rowKey: domain.id
    ? `domain-${domain.id}`
    : `new-domain-${Date.now()}-${index}-${Math.random().toString(36).slice(2)}`,
  id: Number(domain.id || 0),
  domain: domain.domain || '',
  status: Number(domain.status ?? emptyDomain.status),
  verify_token: domain.verify_token || '',
});

const emptyPricing = {
  public_model_name: '',
  base_model_name: '',
  enabled: true,
  pricing_type: 'ratio',
  ratio: 1,
  markup_percent: 0,
  delta_model_ratio: 0,
  delta_model_price: 0,
  consume_rebate_ratio_level1: 0,
  consume_rebate_ratio_level2: 0,
};

const ratioToMarkupPercent = (ratio) => {
  const value = Number(ratio || 1);
  return Number(((value - 1) * 100).toFixed(6));
};

const markupPercentToRatio = (percent) => {
  const value = Number(percent || 0);
  return Number((1 + value / 100).toFixed(6));
};

const ratioToDiscount = (ratio) => {
  const value = Number(ratio || 1);
  return Number((value * 10).toFixed(6));
};

const discountToRatio = (discount) => {
  const value = Number(discount || 10);
  return Number((value / 10).toFixed(6));
};

const formatProviderDiscount = (ratio, t) => {
  const discount = ratioToDiscount(ratio);
  if (discount >= 10) {
    return t('原价');
  }
  return t('{{discount}}折', { discount });
};

const getOrderedProviderDomains = (domains) => {
  if (!Array.isArray(domains)) {
    return [];
  }
  return [...domains].sort((a, b) => Number(a?.id || 0) - Number(b?.id || 0));
};

const getProviderDomainRoleLabel = (index, t) => {
  if (index === 0) {
    return t('主域名');
  }
  return t('备用域名 {{index}}', { index });
};

const formatPriceNumber = (value) => {
  const numberValue = Number(value);
  if (!Number.isFinite(numberValue)) {
    return '-';
  }
  return Number(numberValue.toFixed(6)).toString();
};

const getColorInputValue = (value, fallback = '#000000') => {
  if (typeof value !== 'string') {
    return fallback;
  }
  const color = value.trim();
  if (/^#[0-9a-fA-F]{6}$/.test(color)) {
    return color;
  }
  if (/^#[0-9a-fA-F]{3}$/.test(color)) {
    return `#${color
      .slice(1)
      .split('')
      .map((char) => char + char)
      .join('')}`;
  }
  return fallback;
};

const getConfigFormValues = (config) => {
  const values = { ...emptyConfig, ...(config || {}) };
  return {
    ...values,
    theme_color: values.theme_color || DEFAULT_THEME_PRIMARY_COLOR,
    secondary_color: values.secondary_color || DEFAULT_THEME_SECONDARY_COLOR,
  };
};

const getProviderFormValues = (provider) => ({
  ...emptyProvider,
  ...(provider || {}),
  import_price_ratio: ratioToDiscount(
    provider?.config?.import_price_ratio || 1,
  ),
});

const getPricingFormValues = (pricing) => {
  const values = pricing || emptyPricing;
  return {
    ...values,
    markup_percent: ratioToMarkupPercent(values.ratio),
  };
};

const getOwnerLabel = (user) => {
  if (!user) return '';
  const name =
    user.display_name || user.username || user.email || `ID ${user.id}`;
  const email = user.email ? ` / ${user.email}` : '';
  return `${name}${email} (#${user.id})`;
};

const ProviderPage = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const adminMode = isAdmin();
  const ownerMode = !adminMode && isProviderOwner();
  const pageTitle = adminMode ? t('服务商管理') : t('服务商设置');

  const [providers, setProviders] = useState([]);
  const [providerPage, setProviderPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [currentProvider, setCurrentProvider] = useState(null);
  const [providerModalVisible, setProviderModalVisible] = useState(false);
  const [domainModalVisible, setDomainModalVisible] = useState(false);
  const [configModalVisible, setConfigModalVisible] = useState(false);
  const [pricingModalVisible, setPricingModalVisible] = useState(false);
  const [pricingListVisible, setPricingListVisible] = useState(false);
  const [rewardModalVisible, setRewardModalVisible] = useState(false);
  const [editingProvider, setEditingProvider] = useState(null);
  const [editingPricing, setEditingPricing] = useState(null);
  const [domainRows, setDomainRows] = useState([]);
  const [domainsSaving, setDomainsSaving] = useState(false);
  const [pricingRows, setPricingRows] = useState([]);
  const [pricingLoading, setPricingLoading] = useState(false);
  const [logoUploading, setLogoUploading] = useState(false);
  const [wechatQRCodeUploading, setWechatQRCodeUploading] = useState(false);
  const [baseModels, setBaseModels] = useState([]);
  const [baseModelPrices, setBaseModelPrices] = useState([]);
  const [baseModelPriceProviderId, setBaseModelPriceProviderId] =
    useState(null);
  const [selectedBaseModel, setSelectedBaseModel] = useState('');
  const [baseModelsLoading, setBaseModelsLoading] = useState(false);
  const [pricingType, setPricingType] = useState(emptyPricing.pricing_type);
  const [configColors, setConfigColors] = useState({
    theme_color: DEFAULT_THEME_PRIMARY_COLOR,
    secondary_color: DEFAULT_THEME_SECONDARY_COLOR,
  });
  const [ownerModalVisible, setOwnerModalVisible] = useState(false);
  const [ownerCandidates, setOwnerCandidates] = useState([]);
  const [ownerCandidatesLoading, setOwnerCandidatesLoading] = useState(false);
  const [ownerKeyword, setOwnerKeyword] = useState('');
  const [ownerSearchField, setOwnerSearchField] = useState('');
  const [ownerPage, setOwnerPage] = useState(1);
  const [ownerTotal, setOwnerTotal] = useState(0);
  const [selectedOwnerId, setSelectedOwnerId] = useState(undefined);
  const [selectedOwner, setSelectedOwner] = useState(null);

  const providerFormRef = useRef(null);
  const configFormRef = useRef(null);
  const pricingFormRef = useRef(null);

  const refreshSelfProvider = async () => {
    const res = await API.get('/api/provider/self');
    if (res.data.success) {
      setProviders(res.data.data ? [res.data.data] : []);
    } else {
      showError(res.data.message);
    }
  };

  const fetchProviders = async () => {
    setLoading(true);
    try {
      if (adminMode) {
        const res = await API.get('/api/provider/admin');
        if (res.data.success) {
          setProviders(res.data.data || []);
        } else {
          showError(res.data.message);
        }
      } else if (ownerMode) {
        await refreshSelfProvider();
      }
    } catch (error) {
      showError(error);
    }
    setLoading(false);
  };

  const fetchOwnerCandidates = async ({
    keyword = ownerKeyword,
    field = ownerSearchField,
    page = ownerPage,
    provider = editingProvider,
  } = {}) => {
    if (!adminMode) return;
    setOwnerCandidatesLoading(true);
    try {
      const res = await API.get('/api/provider/admin/owner_candidates', {
        params: {
          keyword,
          field,
          page,
          size: OWNER_PAGE_SIZE,
          current_provider_id: provider?.id || undefined,
        },
      });
      if (res.data.success) {
        const data = res.data.data || {};
        const items = Array.isArray(data) ? data : data.items || [];
        setOwnerCandidates(items);
        setOwnerTotal(Array.isArray(data) ? items.length : data.total || 0);
        setOwnerPage(Array.isArray(data) ? 1 : data.page || page);
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setOwnerCandidatesLoading(false);
    }
  };

  const fetchPricingRows = async (provider) => {
    if (!provider?.id) return;
    setPricingLoading(true);
    try {
      const url = adminMode
        ? `/api/provider/admin/${provider.id}/model_pricing`
        : '/api/provider/model_pricing';
      const res = await API.get(url);
      if (res.data.success) {
        setPricingRows(res.data.data || []);
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    }
    setPricingLoading(false);
  };

  const fetchBaseModels = async (provider = currentProvider) => {
    const providerId = provider?.id || 0;
    if (
      baseModelsLoading ||
      (baseModels.length > 0 && baseModelPriceProviderId === providerId)
    )
      return;
    setBaseModelsLoading(true);
    try {
      const url = adminMode
        ? '/api/provider/admin/base_models'
        : '/api/provider/base_models';
      const res = await API.get(url, {
        params: {
          with_price: true,
          provider_id: adminMode ? providerId : undefined,
        },
      });
      if (res.data.success) {
        const data = res.data.data || [];
        if (Array.isArray(data)) {
          setBaseModels(data);
          setBaseModelPrices([]);
        } else {
          setBaseModels(data.models || []);
          setBaseModelPrices(data.items || []);
        }
        setBaseModelPriceProviderId(providerId);
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    }
    setBaseModelsLoading(false);
  };

  useEffect(() => {
    fetchProviders();
  }, []);

  useEffect(() => {
    if (providers.length === 0) return;
    if (
      currentProvider &&
      providers.some((provider) => provider.id === currentProvider.id)
    )
      return;
    setCurrentProvider(providers[0]);
  }, [providers, currentProvider]);

  useEffect(() => {
    if (!providerModalVisible || !providerFormRef.current) return;
    providerFormRef.current.setValues(getProviderFormValues(editingProvider));
    setSelectedOwnerId(editingProvider?.owner_user_id);
    setSelectedOwner(editingProvider?.owner || null);
  }, [providerModalVisible, editingProvider]);

  useEffect(() => {
    if (!configModalVisible || !configFormRef.current) return;
    const values = getConfigFormValues(currentProvider?.config);
    configFormRef.current.setValues(values);
    setConfigColors({
      theme_color: values.theme_color,
      secondary_color: values.secondary_color,
    });
  }, [configModalVisible, currentProvider]);

  useEffect(() => {
    if (!pricingModalVisible || !pricingFormRef.current) return;
    pricingFormRef.current.setValues(getPricingFormValues(editingPricing));
    setSelectedBaseModel(editingPricing?.base_model_name || '');
    setPricingType(editingPricing?.pricing_type || emptyPricing.pricing_type);
    fetchBaseModels(currentProvider);
  }, [pricingModalVisible, editingPricing]);

  const openProviderModal = (provider = null) => {
    if (!adminMode && !provider) return;
    setEditingProvider(provider);
    setSelectedOwnerId(provider?.owner_user_id);
    setSelectedOwner(provider?.owner || null);
    setProviderModalVisible(true);
  };

  const openDomainModal = (provider) => {
    setCurrentProvider(provider);
    const rows = getOrderedProviderDomains(provider?.domains).map(
      (domain, index) => createProviderDomainRow(domain, index),
    );
    setDomainRows(rows.length > 0 ? rows : [createProviderDomainRow()]);
    setDomainModalVisible(true);
  };

  const addDomainRow = () => {
    setDomainRows((rows) => [
      ...rows,
      createProviderDomainRow({}, rows.length),
    ]);
  };

  const updateDomainRow = (rowKey, field, value) => {
    setDomainRows((rows) =>
      rows.map((row) =>
        row.rowKey === rowKey
          ? {
              ...row,
              [field]: field === 'status' ? Number(value) : value,
            }
          : row,
      ),
    );
  };

  const removeDomainRow = (rowKey) => {
    setDomainRows((rows) => rows.filter((row) => row.rowKey !== rowKey));
  };

  const openConfigModal = (provider) => {
    setCurrentProvider(provider);
    setConfigModalVisible(true);
  };

  const openPricingList = (provider) => {
    setCurrentProvider(provider);
    setPricingListVisible(true);
    fetchPricingRows(provider);
  };

  const openRewardModal = (provider) => {
    setCurrentProvider(provider);
    if (adminMode) {
      setRewardModalVisible(true);
      return;
    }
    navigate('/console/provider/reward');
  };

  const openProviderProfits = (provider) => {
    navigate(
      `/console/provider/profits?provider_id=${provider.id}&provider_name=${encodeURIComponent(provider.name || '')}`,
    );
  };

  const openPricingModal = (pricing = null) => {
    setEditingPricing(pricing);
    setSelectedBaseModel(pricing?.base_model_name || '');
    setPricingType(pricing?.pricing_type || emptyPricing.pricing_type);
    setPricingModalVisible(true);
    fetchBaseModels(currentProvider);
  };

  const openOwnerModal = () => {
    setOwnerModalVisible(true);
    setOwnerPage(1);
    fetchOwnerCandidates({ page: 1 });
  };

  const searchOwnerCandidates = () => {
    setOwnerPage(1);
    fetchOwnerCandidates({ page: 1 });
  };

  const confirmOwnerSelection = () => {
    if (!selectedOwnerId) {
      showError(t('请选择主账号'));
      return;
    }
    providerFormRef.current?.setValue?.('owner_user_id', selectedOwnerId);
    const owner =
      ownerCandidates.find((candidate) => candidate.id === selectedOwnerId) ||
      selectedOwner;
    setSelectedOwner(owner || null);
    setOwnerModalVisible(false);
  };

  const refreshAfterMutation = async () => {
    await fetchProviders();
  };

  const baseModelOptions = useMemo(() => {
    const names = new Set(baseModels || []);
    if (editingPricing?.base_model_name) {
      names.add(editingPricing.base_model_name);
    }
    return Array.from(names)
      .sort((a, b) => String(a).localeCompare(String(b)))
      .map((name) => ({ label: name, value: name }));
  }, [baseModels, editingPricing]);

  const selectedBaseModelPrices = useMemo(
    () =>
      (baseModelPrices || []).filter(
        (item) => item.model_name === selectedBaseModel,
      ),
    [baseModelPrices, selectedBaseModel],
  );

  const handleBaseModelChange = (value) => {
    setSelectedBaseModel(value || '');
    const values = pricingFormRef.current?.getValues?.() || {};
    if (!values.public_model_name) {
      pricingFormRef.current?.setValue?.('public_model_name', value);
    }
  };

  const useBaseModelAsPublicName = () => {
    const values = pricingFormRef.current?.getValues?.() || {};
    if (!values.base_model_name) {
      showError(t('请先选择实际调用主站模型'));
      return;
    }
    pricingFormRef.current?.setValue?.(
      'public_model_name',
      values.base_model_name,
    );
  };

  const handleConfigColorPickerChange = (field, value) => {
    setConfigColors((colors) => ({ ...colors, [field]: value }));
    configFormRef.current?.setValue?.(field, value);
  };

  const handleConfigColorInputChange = (field, value) => {
    setConfigColors((colors) => ({ ...colors, [field]: value }));
  };

  const getLogoUploadPath = (data) => {
    let path = '';
    if (typeof data === 'string') {
      path = data;
    } else {
      path =
        data?.url ||
        data?.path ||
        data?.logo ||
        data?.file_path ||
        data?.filePath ||
        '';
    }
    if (!path || /^https?:\/\//i.test(path)) {
      return path;
    }
    return `${window.location.origin}${path.startsWith('/') ? path : `/${path}`}`;
  };

  const handleLogoUpload = async ({
    file,
    fileInstance,
    onSuccess,
    onError,
  }) => {
    try {
      setLogoUploading(true);
      const uploadFile = fileInstance || file?.fileInstance;
      if (!uploadFile) {
        throw new Error(t('请选择图片'));
      }
      const formData = new FormData();
      formData.append('logo', uploadFile);
      const url = adminMode ? '/api/provider/admin/logo' : '/api/provider/logo';
      const res = await API.post(url, formData, {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        throw new Error(message || t('Logo 上传失败'));
      }
      const logoPath = getLogoUploadPath(data);
      if (!logoPath) {
        throw new Error(t('Logo 上传返回地址无效'));
      }
      configFormRef.current?.setValue?.('logo', logoPath);
      showSuccess(t('Logo 上传成功'));
      onSuccess?.(data || {});
    } catch (error) {
      showError(error?.message || t('Logo 上传失败'));
      onError?.({ status: 500 }, error);
    } finally {
      setLogoUploading(false);
    }
  };

  const handleWeChatQRCodeUpload = async ({
    file,
    fileInstance,
    onSuccess,
    onError,
  }) => {
    try {
      setWechatQRCodeUploading(true);
      const uploadFile = fileInstance || file?.fileInstance;
      if (!uploadFile) {
        throw new Error(t('请选择图片'));
      }
      const formData = new FormData();
      formData.append('wechat_qrcode', uploadFile);
      const res = await API.post('/api/option/wechat_qrcode', formData, {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        throw new Error(message || t('微信二维码上传失败'));
      }
      const wechatQRCodePath = getLogoUploadPath(data);
      if (!wechatQRCodePath) {
        throw new Error(t('微信二维码上传返回地址无效'));
      }
      configFormRef.current?.setValue?.('wechat_support', wechatQRCodePath);
      showSuccess(t('微信二维码上传成功'));
      onSuccess?.(data || {});
    } catch (error) {
      showError(error?.message || t('微信二维码上传失败'));
      onError?.({ status: 500 }, error);
    } finally {
      setWechatQRCodeUploading(false);
    }
  };

  const submitProvider = async () => {
    const values = providerFormRef.current?.getValues?.() || {};
    if (!values.name) {
      showError(t('服务商名称不能为空'));
      return;
    }
    if (adminMode && !values.owner_user_id) {
      showError(t('请选择主账号'));
      return;
    }
    const discountValue =
      values.import_price_ratio === undefined ||
      values.import_price_ratio === null ||
      values.import_price_ratio === ''
        ? 10
        : Number(values.import_price_ratio);
    if (
      adminMode &&
      (Number.isNaN(discountValue) || discountValue < 1 || discountValue > 10)
    ) {
      showError(t('折扣必须在 1 到 10 之间'));
      return;
    }
    const payload = adminMode
      ? {
          ...values,
          owner_user_id: Number(values.owner_user_id),
          status: Number(values.status),
          import_price_ratio: discountToRatio(discountValue),
        }
      : { name: values.name };
    const res =
      adminMode && !editingProvider
        ? await API.post('/api/provider/admin', payload)
        : adminMode
          ? await API.put(`/api/provider/admin/${editingProvider.id}`, payload)
          : await API.put('/api/provider/self', payload);
    if (res.data.success) {
      showSuccess(t('保存成功'));
      setProviderModalVisible(false);
      refreshAfterMutation();
    } else {
      showError(res.data.message);
    }
  };

  const submitDomains = async () => {
    const domains = domainRows.map((row) => ({
      id: Number(row.id || 0),
      domain: String(row.domain || '').trim(),
      status: Number(row.status),
      verify_token: String(row.verify_token || '').trim(),
    }));
    if (domains.some((domain) => !domain.domain)) {
      showError(t('域名不能为空'));
      return;
    }
    const url = adminMode
      ? `/api/provider/admin/${currentProvider.id}/domains`
      : '/api/provider/domains';
    setDomainsSaving(true);
    try {
      const res = await API.put(url, { domains });
      if (res.data.success) {
        const savedDomains = res.data.data || [];
        setProviders((providers) =>
          providers.map((provider) =>
            provider.id === currentProvider.id
              ? { ...provider, domains: savedDomains }
              : provider,
          ),
        );
        setCurrentProvider((provider) =>
          provider?.id === currentProvider.id
            ? { ...provider, domains: savedDomains }
            : provider,
        );
        showSuccess(t('保存成功'));
        setDomainModalVisible(false);
        refreshAfterMutation();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setDomainsSaving(false);
    }
  };

  const submitConfig = async () => {
    const values = configFormRef.current?.getValues?.() || {};
    const payload = {
      ...values,
      wechat_support: values.wechat_support || '',
      qq_support: values.qq_support || '',
    };
    const url = adminMode
      ? `/api/provider/admin/${currentProvider.id}/config`
      : '/api/provider/config';
    const res = await API.put(url, payload);
    if (res.data.success) {
      showSuccess(t('保存成功'));
      setConfigModalVisible(false);
      refreshAfterMutation();
    } else {
      showError(res.data.message);
    }
  };

  const submitPricing = async () => {
    const values = pricingFormRef.current?.getValues?.() || {};
    if (!values.public_model_name || !values.base_model_name) {
      showError(t('模型名称不能为空'));
      return;
    }
    const nextPricingType =
      values.pricing_type || pricingType || emptyPricing.pricing_type;
    const { markup_percent, ...submitValues } = values;
    const payload = {
      ...submitValues,
      id: editingPricing?.id || 0,
      enabled: values.enabled !== false,
      pricing_type: nextPricingType,
      ratio:
        nextPricingType === 'ratio' ? markupPercentToRatio(markup_percent) : 1,
      delta_model_ratio:
        nextPricingType === 'delta' ? Number(values.delta_model_ratio || 0) : 0,
      delta_model_price:
        nextPricingType === 'delta' ? Number(values.delta_model_price || 0) : 0,
      consume_rebate_ratio_level1: Number(
        values.consume_rebate_ratio_level1 || 0,
      ),
      consume_rebate_ratio_level2: Number(
        values.consume_rebate_ratio_level2 || 0,
      ),
    };
    const url = adminMode
      ? `/api/provider/admin/${currentProvider.id}/model_pricing`
      : '/api/provider/model_pricing';
    const res = payload.id
      ? await API.put(url, payload)
      : await API.post(url, payload);
    if (res.data.success) {
      showSuccess(t('保存成功'));
      setPricingModalVisible(false);
      fetchPricingRows(currentProvider);
      refreshAfterMutation();
    } else {
      showError(res.data.message);
    }
  };

  const deletePricing = async (pricing) => {
    const url = adminMode
      ? `/api/provider/admin/${currentProvider.id}/model_pricing/${pricing.id}`
      : `/api/provider/model_pricing/${pricing.id}`;
    const res = await API.delete(url);
    if (res.data.success) {
      showSuccess(t('删除成功'));
      fetchPricingRows(currentProvider);
      refreshAfterMutation();
    } else {
      showError(res.data.message);
    }
  };

  const disableProvider = async (provider) => {
    const res = await API.delete(`/api/provider/admin/${provider.id}`);
    if (res.data.success) {
      showSuccess(t('已禁用服务商'));
      fetchProviders();
    } else {
      showError(res.data.message);
    }
  };

  const enableProvider = async (provider) => {
    const res = await API.put(`/api/provider/admin/${provider.id}/enable`);
    if (res.data.success) {
      showSuccess(t('已启用服务商'));
      fetchProviders();
    } else {
      showError(res.data.message);
    }
  };

  const deleteProvider = async (provider) => {
    const res = await API.delete(`/api/provider/admin/${provider.id}/permanent`);
    if (res.data.success) {
      showSuccess(t('已删除服务商'));
      fetchProviders();
    } else {
      showError(res.data.message);
    }
  };

  const columns = useMemo(
    () =>
      [
        { title: 'ID', dataIndex: 'id', width: 80 },
        {
          title: t('服务商'),
          dataIndex: 'name',
          render: (name, record) => (
            <Space vertical align='start' spacing={2}>
              <Text strong>{name}</Text>
              {adminMode ? (
                <Text type='secondary'>
                  {t('主账号')}：
                  {record.owner?.username || record.owner?.display_name ? (
                    <>
                      {record.owner?.username || record.owner?.display_name}
                      （ID: {record.owner_user_id}）
                    </>
                  ) : (
                    <>ID: {record.owner_user_id}</>
                  )}
                </Text>
              ) : null}
            </Space>
          ),
        },
        {
          title: t('状态'),
          dataIndex: 'status',
          width: 90,
          render: (status) => (
            <Tag color={status === 1 ? 'green' : 'grey'}>
              {status === 1 ? t('启用') : t('禁用')}
            </Tag>
          ),
        },
        {
          title: t('折扣'),
          dataIndex: 'config',
          width: 120,
          render: (config) =>
            formatProviderDiscount(config?.import_price_ratio, t),
        },
        {
          title: t('域名'),
          dataIndex: 'domains',
          render: (domains, provider) => {
            const domainRows = getOrderedProviderDomains(domains);
            return (
              <Space wrap>
                {domainRows.length === 0 ? (
                  <Text type='tertiary'>{t('未配置')}</Text>
                ) : null}
                {domainRows.map((domain, index) => (
                  <Tag
                    key={domain.id || `${domain.domain}-${index}`}
                    color={
                      !domain.domain
                        ? 'grey'
                        : domain.status === 1
                          ? 'green'
                          : 'orange'
                    }
                  >
                    {getProviderDomainRoleLabel(index, t)}
                    {' · '}
                    {domain.domain || t('未配置')}
                  </Tag>
                ))}
              </Space>
            );
          },
        },
        {
          title: t('页面配置'),
          width: 120,
          dataIndex: 'config',
          render: (config) => (
            <Text>
              {config?.site_name ||
              config?.logo ||
              config?.theme_color ||
              config?.secondary_color ||
              config?.wechat_support ||
              config?.qq_support
                ? t('已配置')
                : t('未配置')}
            </Text>
          ),
        },
        {
          title: t('更新时间'),
          dataIndex: 'updated_at',
          width: 170,
          render: (time) => (time ? timestamp2string(time) : '-'),
        },
        {
          title: t('操作'),
          width: 300,
          render: (_, record) => (
            <Space wrap>
              <Button
                size='small'
                icon={<IconEdit />}
                onClick={() => openProviderModal(record)}
              >
                {t('编辑')}
              </Button>
              <Button size='small' onClick={() => openDomainModal(record)}>
                {t('域名管理')}
              </Button>
              <Button size='small' onClick={() => openConfigModal(record)}>
                {t('页面配置')}
              </Button>
              <Button size='small' onClick={() => openPricingList(record)}>
                {t('模型定价')}
              </Button>
              {adminMode ? (
                <Button
                  size='small'
                  onClick={() => openProviderProfits(record)}
                >
                  {t('利润')}
                </Button>
              ) : null}
              <Button
                size='small'
                icon={<IconGiftStroked />}
                onClick={() => openRewardModal(record)}
              >
                {t('奖励配置')}
              </Button>
              {adminMode ? (
                record.status === 1 ? (
                  <Popconfirm
                    title={t('确认禁用该服务商？')}
                    content={t(
                      '禁用后该域名不会再解析成服务商站点，历史数据会保留。',
                    )}
                    onConfirm={() => disableProvider(record)}
                  >
                    <Button size='small' type='danger' icon={<IconDelete />}>
                      {t('禁用')}
                    </Button>
                  </Popconfirm>
                ) : (
                  <Popconfirm
                    title={t('确认启用该服务商？')}
                    content={t('启用后该服务商域名会恢复访问。')}
                    onConfirm={() => enableProvider(record)}
                  >
                    <Button size='small'>{t('启用')}</Button>
                  </Popconfirm>
                )
              ) : null}
              {adminMode ? (
                <Popconfirm
                  title={t('确认删除该服务商？')}
                  content={t(
                    '删除后会移除服务商、域名、页面配置、模型定价和奖励配置。已有服务商用户时不能删除。',
                  )}
                  onConfirm={() => deleteProvider(record)}
                >
                  <Button size='small' type='danger' icon={<IconDelete />}>
                    {t('删除')}
                  </Button>
                </Popconfirm>
              ) : null}
            </Space>
          ),
        },
      ].filter(Boolean),
    [adminMode, t],
  );

  const pricingColumns = [
    { title: 'ID', dataIndex: 'id', width: 70 },
    { title: t('展示模型'), dataIndex: 'public_model_name' },
    { title: t('实际模型'), dataIndex: 'base_model_name' },
    {
      title: t('计价方式'),
      dataIndex: 'pricing_type',
      render: (type) =>
        type === 'delta' ? t('按固定差价') : t('按百分比加价'),
    },
    {
      title: t('加价比例'),
      dataIndex: 'ratio',
      render: (ratio, record) =>
        record.pricing_type === 'ratio'
          ? `${ratioToMarkupPercent(ratio)}%`
          : '-',
    },
    {
      title: t('Token 模型加价倍率'),
      dataIndex: 'delta_model_ratio',
      render: (value, record) =>
        record.pricing_type === 'delta' ? value : '-',
    },
    {
      title: t('按次模型加价金额'),
      dataIndex: 'delta_model_price',
      render: (value, record) =>
        record.pricing_type === 'delta' ? value : '-',
    },
    {
      title: t('一级消费返佣比例'),
      dataIndex: 'consume_rebate_ratio_level1',
      render: (value) => `${Number(value || 0)}%`,
    },
    {
      title: t('二级消费返佣比例'),
      dataIndex: 'consume_rebate_ratio_level2',
      render: (value) => `${Number(value || 0)}%`,
    },
    {
      title: t('状态'),
      dataIndex: 'enabled',
      render: (enabled) => (
        <Tag color={enabled ? 'green' : 'grey'}>
          {enabled ? t('启用') : t('禁用')}
        </Tag>
      ),
    },
    {
      title: t('操作'),
      width: 160,
      render: (_, record) => (
        <Space>
          <Button
            size='small'
            icon={<IconEdit />}
            onClick={() => openPricingModal(record)}
          >
            {t('编辑')}
          </Button>
          <Popconfirm
            title={t('确认删除？')}
            onConfirm={() => deletePricing(record)}
          >
            <Button size='small' type='danger' icon={<IconDelete />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const ownerCandidateColumns = [
    {
      title: t('用户'),
      dataIndex: 'username',
      render: (_, record) => (
        <Space vertical align='start' spacing={1}>
          <Text strong>{record.display_name || record.username || '-'}</Text>
          <Text type='tertiary' size='small'>
            {record.email || '-'} · ID {record.id}
          </Text>
        </Space>
      ),
    },
    {
      title: t('用户名'),
      dataIndex: 'username',
      width: 180,
      render: (username) => username || '-',
    },
  ];

  const ownerCandidateRowSelection = {
    selectedRowKeys: selectedOwnerId ? [selectedOwnerId] : [],
    onChange: (selectedRowKeys, selectedRows) => {
      const nextKey = (selectedRowKeys || []).find(
        (key) => key !== selectedOwnerId,
      );
      const owner =
        (selectedRows || []).find((row) => row.id === nextKey) ||
        selectedRows?.[selectedRows.length - 1];
      setSelectedOwnerId(owner?.id);
      setSelectedOwner(owner || null);
    },
    onSelect: (record, selected) => {
      if (selected) {
        setSelectedOwnerId(record?.id);
        setSelectedOwner(record || null);
      } else if (record?.id === selectedOwnerId) {
        setSelectedOwnerId(undefined);
        setSelectedOwner(null);
      }
    },
  };

  const baseModelPriceColumns = [
    {
      title: t('渠道'),
      dataIndex: 'channel_name',
      render: (name, record) => name || `#${record.channel_id}`,
    },
    {
      title: t('分组'),
      dataIndex: 'group',
      render: (group) => group || '-',
    },
    {
      title: t('计费类型'),
      dataIndex: 'quota_type',
      render: (quotaType) =>
        quotaType === 1 ? t('按次价格') : t('Token 倍率'),
    },
    {
      title: t('输入价格'),
      dataIndex: 'original_price',
      render: (value, record) => (
        <Space vertical align='start' spacing={1}>
          <Text>{t('原价')}：{formatPriceNumber(value)}</Text>
          <Text>
            {t('服务商成本价')}：{formatPriceNumber(record.cost_price)}
          </Text>
        </Space>
      ),
    },
    {
      title: t('补全价格'),
      dataIndex: 'completion_price',
      render: (value, record) =>
        record.quota_type === 0 ? (
          <Space vertical align='start' spacing={1}>
            <Text>{t('原价')}：{formatPriceNumber(value)}</Text>
            <Text>
              {t('服务商成本价')}：{formatPriceNumber(record.cost_completion_price)}
            </Text>
          </Space>
        ) : (
          '-'
        ),
    },
    {
      title: t('缓存读取价格'),
      dataIndex: 'cache_price',
      render: (value, record) =>
        record.quota_type === 0 && value !== undefined && value !== null ? (
          <Space vertical align='start' spacing={1}>
            <Text>{t('原价')}：{formatPriceNumber(value)}</Text>
            <Text>
              {t('服务商成本价')}：{formatPriceNumber(record.cost_cache_price)}
            </Text>
          </Space>
        ) : (
          '-'
        ),
    },
    {
      title: t('服务商折扣'),
      dataIndex: 'import_price_ratio',
      render: (value) => formatProviderDiscount(value, t),
    },
  ];

  if (!adminMode && !ownerMode) {
    return (
      <div className='px-2'>
        <Typography.Title heading={4}>{t('服务商设置')}</Typography.Title>
        <Text type='danger'>{t('当前账号不是服务商主账号，无法访问。')}</Text>
      </div>
    );
  }

  return (
    <div className='px-2'>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 12,
          marginBottom: 12,
          flexWrap: 'wrap',
        }}
      >
        <div>
          <Typography.Title heading={4} style={{ margin: 0 }}>
            {pageTitle}
          </Typography.Title>
          <Text type='secondary'>
            {adminMode
              ? t('管理服务商、域名、页面配置和模型售价。')
              : t('管理当前服务商的域名、页面配置和模型售价。')}
          </Text>
        </div>
        <Space>
          <Button icon={<IconRefresh />} onClick={fetchProviders}>
            {t('刷新')}
          </Button>
          {adminMode ? (
            <Button
              type='primary'
              icon={<IconPlus />}
              onClick={() => openProviderModal()}
            >
              {t('新建服务商')}
            </Button>
          ) : null}
        </Space>
      </div>

      <Table
        rowKey='id'
        columns={columns}
        dataSource={providers.slice((providerPage - 1) * 10, providerPage * 10)}
        loading={loading}
        pagination={false}
      />
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 12 }}>
        <Pagination
          total={providers.length}
          hideOnSinglePage
          onPageChange={(p) => setProviderPage(p)}
        />
      </div>

      <Modal
        title={editingProvider ? t('编辑服务商') : t('新建服务商')}
        visible={providerModalVisible}
        onCancel={() => setProviderModalVisible(false)}
        onOk={submitProvider}
      >
        <Form
          key={editingProvider?.id || 'new-provider'}
          initValues={getProviderFormValues(editingProvider)}
          getFormApi={(api) => (providerFormRef.current = api)}
        >
          <Form.Input field='name' label={t('服务商名称')} />
          {adminMode ? (
            <>
              <Form.Input field='owner_user_id' style={{ display: 'none' }} />
              <div style={{ marginBottom: 12 }}>
                <Text strong>{t('主账号')}</Text>
                <div
                  style={{
                    marginTop: 6,
                    padding: '10px 12px',
                    border: '1px solid var(--semi-color-border)',
                    borderRadius: 6,
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    gap: 12,
                  }}
                >
                  <div>
                    {selectedOwner ? (
                      <Space vertical align='start' spacing={1}>
                        <Text strong>
                          {selectedOwner.username || `ID ${selectedOwner.id}`}
                        </Text>
                        <Text type='tertiary' size='small'>
                          {selectedOwner.email || '-'} · ID {selectedOwner.id}
                        </Text>
                      </Space>
                    ) : (
                      <Text type='tertiary'>{t('尚未选择主账号')}</Text>
                    )}
                  </div>
                  <Button onClick={openOwnerModal}>{t('选择用户')}</Button>
                </div>
              </div>
              <Form.Select
                field='status'
                label={t('状态')}
                optionList={STATUS_OPTIONS.map((option) => ({
                  ...option,
                  label: t(option.label),
                }))}
              />
              <Form.InputNumber
                field='import_price_ratio'
                label={t('模型成本折扣')}
                min={1}
                max={10}
                step={0.1}
                precision={1}
                suffix={t('折')}
              />
              <Text type='tertiary' size='small'>
                {t(
                  '填 1 表示 1 折优惠，填 10 表示原价不优惠；折扣只能填写 1 到 10。服务商用户售价会在折扣价基础上继续按服务商模型定价加价。',
                )}
              </Text>
            </>
          ) : null}
        </Form>
      </Modal>

      <Modal
        title={t('选择主账号')}
        visible={ownerModalVisible}
        onCancel={() => setOwnerModalVisible(false)}
        onOk={confirmOwnerSelection}
        okText={t('确认选择')}
        width={760}
      >
        <Space vertical align='start' style={{ width: '100%' }}>
          <Space wrap>
            <Select
              value={ownerSearchField}
              optionList={OWNER_SEARCH_FIELD_OPTIONS.map((option) => ({
                ...option,
                label: t(option.label),
              }))}
              onChange={(value) => setOwnerSearchField(value || '')}
              style={{ width: 140 }}
            />
            <Input
              value={ownerKeyword}
              onChange={(value) => setOwnerKeyword(value)}
              placeholder={t('搜索用户名、显示名、邮箱或用户 ID')}
              showClear
              style={{ width: 280 }}
              onEnterPress={searchOwnerCandidates}
            />
            <Button type='primary' onClick={searchOwnerCandidates}>
              {t('搜索')}
            </Button>
          </Space>
          <Text type='tertiary' size='small'>
            {t(
              '可以选择普通主站用户或服务商用户。管理员、已绑定其他服务商主账号的用户不能选择。',
            )}
          </Text>
          <Table
            rowKey='id'
            columns={ownerCandidateColumns}
            dataSource={ownerCandidates}
            loading={ownerCandidatesLoading}
            rowSelection={ownerCandidateRowSelection}
            style={{ width: '100%' }}
            pagination={{
              currentPage: ownerPage,
              pageSize: OWNER_PAGE_SIZE,
              total: ownerTotal,
              onPageChange: (page) =>
                fetchOwnerCandidates({
                  page,
                  keyword: ownerKeyword,
                  field: ownerSearchField,
                  provider: editingProvider,
                }),
              showTotal: true,
              showSizeChanger: false,
            }}
            onRow={(record) => ({
              onClick: () => {
                setSelectedOwnerId(record.id);
                setSelectedOwner(record);
              },
              style: { cursor: 'pointer' },
            })}
          />
        </Space>
      </Modal>

      <Modal
        title={t('域名管理')}
        visible={domainModalVisible}
        onCancel={() => setDomainModalVisible(false)}
        footer={
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              width: '100%',
              gap: 12,
            }}
          >
            <Button icon={<IconPlus />} onClick={addDomainRow}>
              {t('域名管理')}
            </Button>
            <Space>
              <Button onClick={() => setDomainModalVisible(false)}>
                {t('取消')}
              </Button>
              <Button
                type='primary'
                loading={domainsSaving}
                onClick={submitDomains}
              >
                {t('保存')}
              </Button>
            </Space>
          </div>
        }
        width={900}
      >
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            gap: 10,
            overflowX: 'auto',
          }}
        >
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '140px minmax(180px, 1fr) 130px minmax(140px, 0.8fr) 48px',
              gap: 8,
              alignItems: 'center',
              minWidth: 860,
              padding: '0 4px',
            }}
          >
            <Text type='tertiary'>{t('类型')}</Text>
            <Text type='tertiary'>{t('域名')}</Text>
            <Text type='tertiary'>{t('状态')}</Text>
            <Text type='tertiary'>{t('验证标识')}</Text>
            <Text type='tertiary'>{t('操作')}</Text>
          </div>
          {domainRows.length === 0 ? (
            <Text type='tertiary'>{t('暂无域名')}</Text>
          ) : null}
          {domainRows.map((row, index) => (
            <div
              key={row.rowKey}
              style={{
                display: 'grid',
                gridTemplateColumns: '140px minmax(180px, 1fr) 130px minmax(140px, 0.8fr) 48px',
                gap: 8,
                alignItems: 'center',
                minWidth: 860,
              }}
            >
              <Tag color={row.status === 1 ? 'green' : 'orange'}>
                {getProviderDomainRoleLabel(index, t)}
              </Tag>
              <Input
                value={row.domain}
                placeholder='ai.example.com'
                onChange={(value) =>
                  updateDomainRow(row.rowKey, 'domain', value)
                }
              />
              <Select
                value={row.status}
                optionList={DOMAIN_STATUS_OPTIONS.map((option) => ({
                  ...option,
                  label: t(option.label),
                }))}
                onChange={(value) =>
                  updateDomainRow(row.rowKey, 'status', value)
                }
              />
              <Input
                value={row.verify_token}
                placeholder={t('验证标识')}
                onChange={(value) =>
                  updateDomainRow(row.rowKey, 'verify_token', value)
                }
              />
              <Popconfirm
                title={t('确认删除？')}
                onConfirm={() => removeDomainRow(row.rowKey)}
              >
                <Button
                  type='danger'
                  theme='borderless'
                  icon={<IconDelete />}
                />
              </Popconfirm>
            </div>
          ))}
        </div>
      </Modal>

      <Modal
        title={t('页面配置')}
        visible={configModalVisible}
        onCancel={() => setConfigModalVisible(false)}
        onOk={submitConfig}
        width={760}
      >
        <Form
          key={`${currentProvider?.id || 0}-config`}
          initValues={getConfigFormValues(currentProvider?.config)}
          getFormApi={(api) => (configFormRef.current = api)}
        >
          <Form.Input field='site_name' label={t('站点名')} />
          <div style={{ display: 'flex', alignItems: 'flex-end', gap: 8 }}>
            <div style={{ flex: 1 }}>
              <Form.Input
                field='logo'
                label={t('Logo 地址')}
                placeholder={t('可以填写图片链接，也可以上传图片自动生成地址')}
              />
            </div>
            <Upload
              action='/'
              accept='image/*'
              showUploadList={false}
              uploadTrigger='auto'
              customRequest={handleLogoUpload}
            >
              <Button
                icon={<IconUpload />}
                loading={logoUploading}
                style={{ marginBottom: 12 }}
              >
                {t('上传图片')}
              </Button>
            </Upload>
          </div>
          <div style={{ marginBottom: 8, fontWeight: 600 }}>
            {t('主题色设置')}
          </div>
          <div className='flex flex-wrap gap-10'>
            <div className='flex-1 min-w-[220px]'>
              <div className='flex items-end gap-2'>
                <input
                  aria-label={t('选择主色')}
                  type='color'
                  value={getColorInputValue(
                    configColors.theme_color,
                    DEFAULT_THEME_PRIMARY_COLOR,
                  )}
                  onChange={(event) =>
                    handleConfigColorPickerChange(
                      'theme_color',
                      event.target.value,
                    )
                  }
                  style={{
                    width: 30,
                    height: 32,
                    padding: 0,
                    border: 0,
                    background: 'transparent',
                    cursor: 'pointer',
                    marginBottom: 12,
                  }}
                />
                <div className='flex-1'>
                  <Form.Input
                    field='theme_color'
                    label={t('主色')}
                    placeholder={DEFAULT_THEME_PRIMARY_COLOR}
                    onChange={(value) =>
                      handleConfigColorInputChange('theme_color', value)
                    }
                  />
                </div>
              </div>
            </div>
            <div className='flex-1 min-w-[220px]'>
              <div className='flex items-end gap-2'>
                <input
                  aria-label={t('选择辅色')}
                  type='color'
                  value={getColorInputValue(
                    configColors.secondary_color,
                    DEFAULT_THEME_SECONDARY_COLOR,
                  )}
                  onChange={(event) =>
                    handleConfigColorPickerChange(
                      'secondary_color',
                      event.target.value,
                    )
                  }
                  style={{
                    width: 30,
                    height: 32,
                    padding: 0,
                    border: 0,
                    background: 'transparent',
                    cursor: 'pointer',
                    marginBottom: 12,
                  }}
                />
                <div className='flex-1'>
                  <Form.Input
                    field='secondary_color'
                    label={t('辅色')}
                    placeholder={DEFAULT_THEME_SECONDARY_COLOR}
                    onChange={(value) =>
                      handleConfigColorInputChange('secondary_color', value)
                    }
                  />
                </div>
              </div>
            </div>
          </div>
          <Form.TextArea field='footer_text' label={t('页脚文案')} autosize />
          <div style={{ marginBottom: 8, fontWeight: 600 }}>
            {t('客服设置')}
          </div>
          <div style={{ display: 'flex', alignItems: 'flex-end', gap: 8 }}>
            <div style={{ flex: 1 }}>
              <Form.Input
                field='wechat_support'
                label={t('微信二维码')}
                placeholder={t('可以填写图片链接，也可以上传图片自动生成地址')}
              />
            </div>
            <Upload
              action='/'
              accept='image/*'
              showUploadList={false}
              uploadTrigger='auto'
              customRequest={handleWeChatQRCodeUpload}
            >
              <Button
                icon={<IconUpload />}
                loading={wechatQRCodeUploading}
                style={{ marginBottom: 12 }}
              >
                {t('上传微信二维码')}
              </Button>
            </Upload>
          </div>
          <Form.Input
            field='qq_support'
            label={t('QQ号')}
            placeholder={t('请输入QQ号')}
            showClear
          />
        </Form>
      </Modal>

      <Modal
        title={`${currentProvider?.name || ''} - ${t('模型定价')}`}
        visible={pricingListVisible}
        onCancel={() => setPricingListVisible(false)}
        footer={
          <Space>
            <Button onClick={() => setPricingListVisible(false)}>
              {t('关闭')}
            </Button>
            <Button
              type='primary'
              icon={<IconPlus />}
              onClick={() => openPricingModal()}
            >
              {t('新增定价')}
            </Button>
          </Space>
        }
        width={1200}
      >
        <Table
          rowKey='id'
          columns={pricingColumns}
          dataSource={pricingRows}
          loading={pricingLoading}
          pagination={{ pageSize: 10 }}
        />
      </Modal>

      <Modal
        title={editingPricing ? t('编辑模型定价') : t('新增模型定价')}
        visible={pricingModalVisible}
        onCancel={() => setPricingModalVisible(false)}
        onOk={submitPricing}
        width={1200}
      >
        <Form
          key={editingPricing?.id || `${currentProvider?.id || 0}-new-pricing`}
          initValues={getPricingFormValues(editingPricing)}
          getFormApi={(api) => (pricingFormRef.current = api)}
        >
          <Form.Input
            field='public_model_name'
            label={t('展示给服务商用户的模型名')}
          />
          <Button
            size='small'
            type='tertiary'
            onClick={useBaseModelAsPublicName}
            style={{ marginBottom: 12 }}
          >
            {t('使用实际模型名')}
          </Button>
          <Form.Select
            field='base_model_name'
            label={t('实际调用主站模型')}
            placeholder={t('请选择已启用渠道支持的主站模型')}
            optionList={baseModelOptions}
            loading={baseModelsLoading}
            filter
            remote={false}
            onChange={handleBaseModelChange}
          />
          <Text type='tertiary' size='small'>
            {t(
              '这里只能选择主站当前已启用渠道支持的模型，避免手动填写错误导致服务商用户调用失败。',
            )}
          </Text>
          <div style={{ marginTop: 12, marginBottom: 12 }}>
            {selectedBaseModel ? (
              selectedBaseModelPrices.length > 0 ? (
                <>
                  <Text type='secondary' size='small'>
                    {t(
                      '下面是这个模型在主站不同渠道里的原价和你的成本价。同一个模型如果有多个渠道，会分别计算；例如主站原价 100，服务商折扣 3 折，那么服务商成本价就是 30。你设置的加价，会在这个成本价基础上继续计算。',
                    )}
                  </Text>
                  <Table
                    size='small'
                    style={{ marginTop: 8 }}
                    rowKey={(record) =>
                      `${record.model_name}-${record.channel_id}-${record.group}`
                    }
                    columns={baseModelPriceColumns}
                    dataSource={selectedBaseModelPrices}
                    pagination={false}
                  />
                </>
              ) : (
                <Text type='tertiary' size='small'>
                  {t(
                    '暂时没有读到这个模型的渠道价格，请确认主站渠道已启用，并且模型价格配置已保存。',
                  )}
                </Text>
              )
            ) : (
              <Text type='tertiary' size='small'>
                {t('先选择一个实际调用的主站模型，这里会显示服务商成本价。')}
              </Text>
            )}
          </div>
          <Form.Switch field='enabled' label={t('启用')} />
          <Form.Select
            field='pricing_type'
            label={t('计价方式')}
            optionList={PRICING_TYPE_OPTIONS.map((option) => ({
              ...option,
              label: t(option.label),
            }))}
            onChange={(value) => {
              const nextType = value || emptyPricing.pricing_type;
              setPricingType(nextType);
              if (nextType === 'ratio') {
                pricingFormRef.current?.setValue?.('delta_model_ratio', 0);
                pricingFormRef.current?.setValue?.('delta_model_price', 0);
              } else {
                pricingFormRef.current?.setValue?.('ratio', 1);
                pricingFormRef.current?.setValue?.('markup_percent', 0);
              }
            }}
          />
          {pricingType === 'ratio' ? (
            <>
              <Text type='tertiary' size='small'>
                {t(
                  '这里的加价基于服务商折扣价计算，不是基于主站原价。最终售价 = 主站原价 × 折扣 / 10 × (1 + 加价比例)',
                )}
              </Text>
              <Form.InputNumber
                field='markup_percent'
                label={t('加价比例')}
                min={0}
                step={1}
                suffix='%'
              />
            </>
          ) : (
            <>
              <Text type='tertiary' size='small'>
                {t(
                  '固定加价是在服务商折扣价基础上额外增加固定金额。最终售价 = 服务商折扣价 + 固定加价。',
                )}
              </Text>
              <Form.InputNumber
                field='delta_model_ratio'
                label={t('Token 模型加价倍率')}
                step={0.01}
              />
              <Form.InputNumber
                field='delta_model_price'
                label={t('按次模型加价金额')}
                step={0.000001}
              />
              <Text type='tertiary' size='small'>
                {t(
                  '按 token 计费的模型看“Token 模型加价倍率”；按次、按张、按任务计费的模型看“按次模型加价金额”。填 0 表示不额外加价。',
                )}
              </Text>
            </>
          )}
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
              gap: 12,
            }}
          >
            <Form.InputNumber
              field='consume_rebate_ratio_level1'
              label={t('一级消费返佣比例（利润比例）')}
              min={0}
              max={100}
              step={0.01}
              precision={6}
              suffix='%'
            />
            <Form.InputNumber
              field='consume_rebate_ratio_level2'
              label={t('二级消费返佣比例（利润比例）')}
              min={0}
              max={100}
              step={0.01}
              precision={6}
              suffix='%'
            />
          </div>
          <Text type='tertiary' size='small'>
            {t(
              '消费返佣比例绑定在当前展示模型上，未配置或填 0 时不产生对应层级返佣。',
            )}
          </Text>
        </Form>
      </Modal>

      <ProviderRewardModal
        visible={rewardModalVisible}
        provider={currentProvider || providers[0]}
        adminMode={adminMode}
        onClose={() => setRewardModalVisible(false)}
      />
    </div>
  );
};

export default ProviderPage;
