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
  Modal,
  Popconfirm,
  Space,
  Table,
  Tag,
  Typography,
  Upload,
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

const emptyProvider = {
  owner_user_id: undefined,
  name: '',
  status: 1,
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

const emptyPricing = {
  public_model_name: '',
  base_model_name: '',
  enabled: true,
  pricing_type: 'ratio',
  ratio: 1,
  markup_percent: 0,
  delta_model_ratio: 0,
  delta_model_price: 0,
};

const ratioToMarkupPercent = (ratio) => {
  const value = Number(ratio || 1);
  return Number(((value - 1) * 100).toFixed(6));
};

const markupPercentToRatio = (percent) => {
  const value = Number(percent || 0);
  return Number((1 + value / 100).toFixed(6));
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
  const [loading, setLoading] = useState(false);
  const [currentProvider, setCurrentProvider] = useState(null);
  const [providerModalVisible, setProviderModalVisible] = useState(false);
  const [domainModalVisible, setDomainModalVisible] = useState(false);
  const [configModalVisible, setConfigModalVisible] = useState(false);
  const [pricingModalVisible, setPricingModalVisible] = useState(false);
  const [pricingListVisible, setPricingListVisible] = useState(false);
  const [rewardModalVisible, setRewardModalVisible] = useState(false);
  const [editingProvider, setEditingProvider] = useState(null);
  const [editingDomain, setEditingDomain] = useState(null);
  const [editingPricing, setEditingPricing] = useState(null);
  const [pricingRows, setPricingRows] = useState([]);
  const [pricingLoading, setPricingLoading] = useState(false);
  const [logoUploading, setLogoUploading] = useState(false);
  const [wechatQRCodeUploading, setWechatQRCodeUploading] = useState(false);
  const [baseModels, setBaseModels] = useState([]);
  const [baseModelsLoading, setBaseModelsLoading] = useState(false);
  const [pricingType, setPricingType] = useState(emptyPricing.pricing_type);
  const [configColors, setConfigColors] = useState({
    theme_color: DEFAULT_THEME_PRIMARY_COLOR,
    secondary_color: DEFAULT_THEME_SECONDARY_COLOR,
  });
  const [ownerOptions, setOwnerOptions] = useState([]);

  const providerFormRef = useRef(null);
  const domainFormRef = useRef(null);
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

  const fetchOwnerCandidates = async (keyword = '', provider = null) => {
    if (!adminMode) return;
    try {
      const res = await API.get('/api/provider/admin/owner_candidates', {
        params: {
          keyword,
          current_provider_id: provider?.id || undefined,
        },
      });
      if (res.data.success) {
        setOwnerOptions(
          (res.data.data || []).map((user) => ({
            label: getOwnerLabel(user),
            value: user.id,
          })),
        );
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
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

  const fetchBaseModels = async () => {
    if (baseModelsLoading || baseModels.length > 0) return;
    setBaseModelsLoading(true);
    try {
      const url = adminMode
        ? '/api/provider/admin/base_models'
        : '/api/provider/base_models';
      const res = await API.get(url);
      if (res.data.success) {
        setBaseModels(res.data.data || []);
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
    providerFormRef.current.setValues(editingProvider || emptyProvider);
  }, [providerModalVisible, editingProvider]);

  useEffect(() => {
    if (!domainModalVisible || !domainFormRef.current) return;
    domainFormRef.current.setValues(editingDomain || emptyDomain);
  }, [domainModalVisible, editingDomain]);

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
    setPricingType(editingPricing?.pricing_type || emptyPricing.pricing_type);
    fetchBaseModels();
  }, [pricingModalVisible, editingPricing]);

  const openProviderModal = (provider = null) => {
    if (!adminMode && !provider) return;
    setEditingProvider(provider);
    setProviderModalVisible(true);
    if (adminMode) {
      fetchOwnerCandidates('', provider);
    }
  };

  const openDomainModal = (provider, domain = null) => {
    setCurrentProvider(provider);
    setEditingDomain(domain);
    setDomainModalVisible(true);
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

  const openPricingModal = (pricing = null) => {
    setEditingPricing(pricing);
    setPricingType(pricing?.pricing_type || emptyPricing.pricing_type);
    setPricingModalVisible(true);
    fetchBaseModels();
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

  const handleBaseModelChange = (value) => {
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
    const payload = adminMode
      ? {
          ...values,
          owner_user_id: Number(values.owner_user_id),
          status: Number(values.status),
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

  const submitDomain = async () => {
    const values = domainFormRef.current?.getValues?.() || {};
    if (!values.domain) {
      showError(t('域名不能为空'));
      return;
    }
    const payload = {
      ...values,
      status: Number(values.status),
    };
    const res = editingDomain
      ? adminMode
        ? await API.put(
            `/api/provider/admin/${currentProvider.id}/domains/${editingDomain.id}`,
            payload,
          )
        : await API.put(`/api/provider/domains/${editingDomain.id}`, payload)
      : adminMode
        ? await API.post(
            `/api/provider/admin/${currentProvider.id}/domains`,
            payload,
          )
        : await API.post('/api/provider/domains', payload);
    if (res.data.success) {
      showSuccess(t('保存成功'));
      setDomainModalVisible(false);
      refreshAfterMutation();
    } else {
      showError(res.data.message);
    }
  };

  const deleteDomain = async (provider, domain) => {
    const url = adminMode
      ? `/api/provider/admin/${provider.id}/domains/${domain.id}`
      : `/api/provider/domains/${domain.id}`;
    const res = await API.delete(url);
    if (res.data.success) {
      showSuccess(t('删除成功'));
      refreshAfterMutation();
    } else {
      showError(res.data.message);
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

  const columns = useMemo(
    () => [
      { title: 'ID', dataIndex: 'id', width: 80 },
      {
        title: t('服务商'),
        dataIndex: 'name',
        render: (name, record) => (
          <Space vertical align='start' spacing={2}>
            <Text strong>{name}</Text>
            {adminMode ? (
              <Text type='secondary'>
                Owner User ID: {record.owner_user_id}
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
        title: t('域名'),
        dataIndex: 'domains',
        render: (domains, provider) => {
          const domainRows = Array.isArray(domains) ? domains : [];
          return (
            <Space wrap>
              {domainRows.length === 0 ? (
                <Text type='tertiary'>{t('未配置')}</Text>
              ) : null}
              {domainRows.map((domain) => (
                <Tag
                  key={domain.id}
                  color={domain.status === 1 ? 'green' : 'orange'}
                  closable
                  style={{ cursor: 'pointer' }}
                  onClick={() => openDomainModal(provider, domain)}
                  onClose={(e) => {
                    e?.stopPropagation?.();
                    deleteDomain(provider, domain);
                  }}
                >
                  {domain.domain}
                </Tag>
              ))}
            </Space>
          );
        },
      },
      {
        title: t('页面配置'),
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
        width: adminMode ? 360 : 300,
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
              {t('添加域名')}
            </Button>
            <Button size='small' onClick={() => openConfigModal(record)}>
              {t('页面配置')}
            </Button>
            <Button size='small' onClick={() => openPricingList(record)}>
              {t('模型定价')}
            </Button>
            <Button
              size='small'
              icon={<IconGiftStroked />}
              onClick={() => openRewardModal(record)}
            >
              {t('奖励配置')}
            </Button>
            {adminMode ? (
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
            ) : null}
          </Space>
        ),
      },
    ],
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
        dataSource={providers}
        loading={loading}
        pagination={{ pageSize: 10 }}
      />

      <Modal
        title={editingProvider ? t('编辑服务商') : t('新建服务商')}
        visible={providerModalVisible}
        onCancel={() => setProviderModalVisible(false)}
        onOk={submitProvider}
      >
        <Form
          key={editingProvider?.id || 'new-provider'}
          initValues={editingProvider || emptyProvider}
          getFormApi={(api) => (providerFormRef.current = api)}
        >
          <Form.Input field='name' label={t('服务商名称')} />
          {adminMode ? (
            <>
              <Form.Select
                field='owner_user_id'
                label={t('主账号')}
                optionList={ownerOptions}
                filter
                remote
                onSearch={(keyword) =>
                  fetchOwnerCandidates(keyword, editingProvider)
                }
                placeholder={t('搜索用户名、显示名、邮箱或用户 ID')}
              />
              <Form.Select
                field='status'
                label={t('状态')}
                optionList={STATUS_OPTIONS}
              />
            </>
          ) : null}
        </Form>
      </Modal>

      <Modal
        title={editingDomain ? t('编辑域名') : t('添加域名')}
        visible={domainModalVisible}
        onCancel={() => setDomainModalVisible(false)}
        onOk={submitDomain}
      >
        <Form
          key={editingDomain?.id || `${currentProvider?.id || 0}-new-domain`}
          initValues={editingDomain || emptyDomain}
          getFormApi={(api) => (domainFormRef.current = api)}
        >
          <Form.Input
            field='domain'
            label={t('域名')}
            placeholder='ai.example.com'
          />
          <Form.Select
            field='status'
            label={t('状态')}
            optionList={DOMAIN_STATUS_OPTIONS}
          />
          <Form.Input field='verify_token' label={t('验证标识')} />
        </Form>
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
        width={1100}
      >
        <Table
          rowKey='id'
          columns={pricingColumns}
          dataSource={pricingRows}
          loading={pricingLoading}
          pagination={{ pageSize: 8 }}
        />
      </Modal>

      <Modal
        title={editingPricing ? t('编辑模型定价') : t('新增模型定价')}
        visible={pricingModalVisible}
        onCancel={() => setPricingModalVisible(false)}
        onOk={submitPricing}
        width={720}
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
          <Form.Switch field='enabled' label={t('启用')} />
          <Form.Select
            field='pricing_type'
            label={t('计价方式')}
            optionList={PRICING_TYPE_OPTIONS}
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
              <Form.InputNumber
                field='markup_percent'
                label={t('加价比例')}
                min={0}
                step={1}
                suffix='%'
              />
              <Text type='tertiary' size='small'>
                {t(
                  '填 20 表示在主站成本价基础上加价 20%，系统保存时会自动换算为 1.2 倍；填 0 表示不加价。',
                )}
              </Text>
            </>
          ) : (
            <>
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
