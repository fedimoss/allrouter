import React, { useEffect, useState } from 'react';
import {
  Button,
  Input,
  InputNumber,
  Modal,
  Space,
  Spin,
  Table,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const DEFAULT_FORM = {
  network: '',
  chain_id: '',
  token_symbol: 'USDT',
  token_decimals: 6,
  token_contract: '',
  receiver_address: '',
  rpc_url: '',
  min_confirmations: 3,
};

export default function SettingCryptocurrency() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [chains, setChains] = useState([]);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingIndex, setEditingIndex] = useState(-1);
  const [form, setForm] = useState({ ...DEFAULT_FORM });

  const [usdRate, setUsdRate] = useState('');
  const [cnyRate, setCnyRate] = useState('');

  const fetchChains = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/option/get_crypto_chain_config');
      const { success, data } = res.data;
      if (success && Array.isArray(data)) {
        setChains(data);
      }
    } catch {
      showError(t('获取链配置失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchChains();
    fetchRate();
  }, []);

  const fetchRate = async () => {
    try {
      const res = await API.get('/api/option/get_crypto_rate');
      const { success, data } = res.data;
      if (success && data) {
        setUsdRate(data.usd_to_token_rate || '');
        setCnyRate(data.cny_to_token_rate || '');
      }
    } catch {
      // ignore
    }
  };

  const submitRate = async () => {
    if (!usdRate || !cnyRate) {
      showError(t('汇率不能为空'));
      return;
    }
    setLoading(true);
    try {
      const res = await API.post('/api/option/update_crypto_rate', {
        usd_to_token_rate: usdRate,
        cny_to_token_rate: cnyRate,
      });
      if (res.data.success) {
        showSuccess(t('更新成功'));
      } else {
        showError(res.data.message);
      }
    } catch {
      showError(t('更新失败'));
    } finally {
      setLoading(false);
    }
  };

  const submitChains = async () => {
    setLoading(true);
    try {
      const res = await API.post('/api/option/update_crypto_chain_config', {
        crypto: chains.map((c) => ({
          network: c.network,
          chain_id: Number(c.chain_id),
          token_decimals: Number(c.token_decimals),
          token_contract: c.token_contract,
          receiver_address: c.receiver_address,
          rpc_url: c.rpc_url,
          min_confirmations: Number(c.min_confirmations),
        })),
      });
      if (res.data.success) {
        showSuccess(t('更新成功'));
      } else {
        showError(res.data.message);
      }
    } catch {
      showError(t('更新失败'));
    } finally {
      setLoading(false);
    }
  };

  const openAdd = () => {
    setEditingIndex(-1);
    setForm({ ...DEFAULT_FORM });
    setModalVisible(true);
  };

  const openEdit = (record, index) => {
    setEditingIndex(index);
    setForm({ ...record, chain_id: String(record.chain_id) });
    setModalVisible(true);
  };

  const handleModalOk = () => {
    if (!form.network?.trim()) {
      showError(t('网络名称不能为空'));
      return;
    }
    if (!form.chain_id) {
      showError(t('链 ID 不能为空'));
      return;
    }
    if (!form.token_contract?.trim()) {
      showError(t('代币合约地址不能为空'));
      return;
    }
    if (!form.receiver_address?.trim()) {
      showError(t('收款地址不能为空'));
      return;
    }
    if (!form.rpc_url?.trim()) {
      showError(t('RPC 地址不能为空'));
      return;
    }

    const item = {
      ...form,
      chain_id: Number(form.chain_id),
      token_decimals: Number(form.token_decimals),
      min_confirmations: Number(form.min_confirmations),
    };

    const updated = [...chains];
    if (editingIndex === -1) {
      updated.push(item);
    } else {
      updated[editingIndex] = item;
    }
    setChains(updated);
    setModalVisible(false);
  };

  const handleDelete = (index) => {
    Modal.error({
      title: t('确认删除'),
      content: t('确定要删除该链配置吗？'),
      okText: t('确定'),
      cancelText: t('取消'),
      onOk: () => {
        const updated = chains.filter((_, i) => i !== index);
        setChains(updated);
      },
    });
  };

  const columns = [
    {
      title: t('网络名称'),
      dataIndex: 'network',
    },
    {
      title: 'Chain ID',
      dataIndex: 'chain_id',
    },
    {
      title: t('代币精度'),
      dataIndex: 'token_decimals',
    },
    {
      title: t('代币合约地址'),
      dataIndex: 'token_contract',
      render: (text) => (
        <Tooltip content={text}>
          <Text copyable={{ copyTips: false }}>
            {text ? `${text.slice(0, 6)}****${text.slice(-4)}` : '-'}
          </Text>
        </Tooltip>
      ),
    },
    {
      title: t('收款地址'),
      dataIndex: 'receiver_address',
      render: (text) => (
        <Tooltip content={text}>
          <Text copyable={{ copyTips: false }}>
            {text ? `${text.slice(0, 6)}****${text.slice(-4)}` : '-'}
          </Text>
        </Tooltip>
      ),
    },
    {
      title: 'RPC URL',
      dataIndex: 'rpc_url',
      render: (text) => (
        <Text
          ellipsis={{ showTooltip: true }}
          style={{ maxWidth: 220 }}
          copyable
        >
          {text}
        </Text>
      ),
    },
    {
      title: t('最小确认数'),
      dataIndex: 'min_confirmations',
    },
    {
      title: t('操作'),
      key: 'action',
      render: (_, record, index) => (
        <Space>
          <Button
            size='small'
            onClick={() => openEdit(record, index)}
          >
            {t('编辑')}
          </Button>
          <Button
            size='small'
            type='danger'
            onClick={() => handleDelete(index)}
          >
            {t('删除')}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <Spin spinning={loading}>
      <h5 className='semi-form-section-text'>{t('加密货币支付设置')}</h5>
      <div style={{ marginBottom: 12 }}>
        <Text type='secondary'>
          {t('配置加密货币支付的链网络信息，添加后用户在充值时可选择对应网络进行支付。')}
        </Text>
      </div>
      <div style={{ marginBottom: 12 }}>
        <Button onClick={openAdd}>{t('新增链配置')}</Button>
      </div>
      <Table
        columns={columns}
        dataSource={chains}
        rowKey={(record, index) => record.network + '_' + index}
        pagination={false}
        size='small'
        empty={
          <Text type='tertiary'>{t('暂无链配置，点击上方按钮新增')}</Text>
        }
      />
      <Button onClick={submitChains} disabled={chains.length === 0} style={{ marginTop: 16 }}>
        {t('更新加密货币设置')}
      </Button>

      <h5 className='semi-form-section-text' style={{ marginTop: 32, borderTop: '1px solid var(--semi-color-border)', paddingTop: 16 }}>{t('汇率设置')}</h5>
      <div style={{ marginBottom: 12 }}>
        <Text type='secondary'>
          {t('配置法币到 USDT 的兑换汇率，用于前端展示加密货币支付时的金额换算。')}
        </Text>
      </div>
      <div style={{ display: 'flex', gap: 16, alignItems: 'flex-end', flexWrap: 'wrap' }}>
        <div style={{ flex: 1, minWidth: 200 }}>
          <div style={{ marginBottom: 4 }}>
            <Text strong>{t('美元到 USDT 汇率')}</Text>
          </div>
          <Input
            value={usdRate}
            onChange={setUsdRate}
            placeholder='1'
          />
        </div>
        <div style={{ flex: 1, minWidth: 200 }}>
          <div style={{ marginBottom: 4 }}>
            <Text strong>{t('人民币到 USDT 汇率')}</Text>
          </div>
          <Input
            value={cnyRate}
            onChange={setCnyRate}
            placeholder='0.1471'
          />
        </div>
        <Button onClick={submitRate} theme='solid' style={{ marginBottom: 0 }}>
          {t('确认更新')}
        </Button>
      </div>

      <Modal
        title={editingIndex === -1 ? t('新增链配置') : t('编辑链配置')}
        visible={modalVisible}
        onOk={handleModalOk}
        onCancel={() => setModalVisible(false)}
        okText={t('确定')}
        cancelText={t('取消')}
        width={520}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div>
            <div style={{ marginBottom: 4 }}>
              <Text strong>{t('网络名称')}</Text>
              <span style={{ color: 'var(--semi-color-danger)', marginLeft: 4 }}>*</span>
            </div>
            <Input
              value={form.network}
              onChange={(val) => setForm({ ...form, network: val })}
              placeholder={t('例如 BSC、Polygon、Sepolia')}
            />
          </div>
          <div>
            <div style={{ marginBottom: 4 }}>
              <Text strong>Chain ID</Text>
              <span style={{ color: 'var(--semi-color-danger)', marginLeft: 4 }}>*</span>
            </div>
            <InputNumber
              value={form.chain_id}
              onChange={(val) => setForm({ ...form, chain_id: val })}
              placeholder='97'
              style={{ width: '100%' }}
            />
          </div>
          <div>
            <div style={{ marginBottom: 4 }}>
              <Text strong>{t('代币精度')}</Text>
            </div>
            <InputNumber
              value={form.token_decimals}
              onChange={(val) => setForm({ ...form, token_decimals: val })}
              min={0}
              max={18}
              style={{ width: '100%' }}
            />
          </div>
          <div>
            <div style={{ marginBottom: 4 }}>
              <Text strong>{t('代币合约地址')}</Text>
              <span style={{ color: 'var(--semi-color-danger)', marginLeft: 4 }}>*</span>
            </div>
            <Input
              value={form.token_contract}
              onChange={(val) => setForm({ ...form, token_contract: val })}
              placeholder='0x...'
            />
          </div>
          <div>
            <div style={{ marginBottom: 4 }}>
              <Text strong>{t('收款钱包地址')}</Text>
              <span style={{ color: 'var(--semi-color-danger)', marginLeft: 4 }}>*</span>
            </div>
            <Input
              value={form.receiver_address}
              onChange={(val) => setForm({ ...form, receiver_address: val })}
              placeholder='0x...'
            />
          </div>
          <div>
            <div style={{ marginBottom: 4 }}>
              <Text strong>{t('链节点 RPC 地址')}</Text>
              <span style={{ color: 'var(--semi-color-danger)', marginLeft: 4 }}>*</span>
            </div>
            <Input
              value={form.rpc_url}
              onChange={(val) => setForm({ ...form, rpc_url: val })}
              placeholder='https://...'
            />
          </div>
          <div>
            <div style={{ marginBottom: 4 }}>
              <Text strong>{t('最小链上确认数')}</Text>
            </div>
            <InputNumber
              value={form.min_confirmations}
              onChange={(val) => setForm({ ...form, min_confirmations: val })}
              min={1}
              style={{ width: '100%' }}
            />
          </div>
        </div>
      </Modal>
    </Spin>
  );
}
