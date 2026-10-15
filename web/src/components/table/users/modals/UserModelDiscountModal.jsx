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

import React, { useEffect, useState } from 'react';
import {
  Banner,
  Button,
  Empty,
  Modal,
  SideSheet,
  Space,
  Tag,
  Typography,
  Select,
  InputNumber,
} from '@douyinfe/semi-ui';
import { IconPlusCircle, IconDelete } from '@douyinfe/semi-icons';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  API,
  selectFilter,
  showError,
  showSuccess,
} from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import CardTable from '../../../common/ui/CardTable';

const { Text } = Typography;

function formatTs(ts) {
  if (!ts) return '-';
  return new Date(ts * 1000).toLocaleString();
}

// 用户模型专属折扣：仅余额支付生效；只折扣输入输出 token（缓存不参与）；
// 按次计费与阶梯表达式计费的模型不支持；与分组倍率叠加；范围 (0, 1]
const UserModelDiscountModal = ({ visible, onCancel, user, apiPrefix = '/api/user', t }) => {
  const normalizedApiPrefix = apiPrefix.replace(/\/$/, '');
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [records, setRecords] = useState([]);
  const [modelOptions, setModelOptions] = useState([]);
  const [modelName, setModelName] = useState('');
  const [discount, setDiscount] = useState(0.8);

  const loadRecords = async () => {
    if (!user?.id) return;
    setLoading(true);
    try {
      const res = await API.get(`${normalizedApiPrefix}/${user.id}/model_discounts`);
      if (res.data?.success) {
        const data = res.data.data || {};
        const nextRecords = Array.isArray(data) ? data : data.discounts || [];
        const candidates = Array.isArray(data) ? [] : data.models || [];
        setRecords(nextRecords);
        setModelOptions(
          Array.from(new Set(candidates.filter(Boolean)))
            .sort((a, b) => a.localeCompare(b))
            .map((name) => ({ label: name, value: name })),
        );
      } else {
        showError(res.data?.message || t('加载失败'));
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!visible) return;
    setModelName('');
    setDiscount(0.8);
    loadRecords();
  }, [visible]);

  const upsertDiscount = async () => {
    if (!user?.id) {
      showError(t('用户信息缺失'));
      return;
    }
    const name = (modelName || '').trim();
    if (!name) {
      showError(t('请选择模型'));
      return;
    }
    const value = Number(discount);
    if (!(value > 0 && value <= 1)) {
      showError(t('折扣范围必须在 (0, 1] 之间（1 表示不打折）'));
      return;
    }
    setSaving(true);
    try {
      const res = await API.put(`${normalizedApiPrefix}/${user.id}/model_discounts`, {
        model_name: name,
        discount: value,
      });
      if (res.data?.success) {
        showSuccess(t('保存成功'));
        setModelName('');
        await loadRecords();
      } else {
        showError(res.data?.message || t('保存失败'));
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setSaving(false);
    }
  };

  const deleteDiscount = (record) => {
    Modal.confirm({
      title: t('确认删除'),
      content: t('删除后该模型将恢复原价。是否继续？'),
      centered: true,
      okType: 'danger',
      okText: t('确定'),
      cancelText: t('取消'),
      onOk: async () => {
        try {
          const res = await API.delete(
            `${normalizedApiPrefix}/${user.id}/model_discounts?model_name=${encodeURIComponent(
              record?.model_name || '',
            )}`,
          );
          if (res.data?.success) {
            showSuccess(t('已删除'));
            await loadRecords();
          } else {
            showError(res.data?.message || t('删除失败'));
          }
        } catch (e) {
          showError(t('请求失败'));
        }
      },
    });
  };

  const columns = [
    {
      title: t('模型'),
      dataIndex: 'model_name',
      width: 220,
      render: (text) => (
        <div className='font-medium truncate' title={text}>
          {text || '-'}
        </div>
      ),
    },
    {
      title: t('折扣'),
      dataIndex: 'discount',
      width: 100,
      render: (text) => {
        const value = Number(text);
        return (
          <Tag color={value < 1 ? 'green' : 'grey'} shape='circle' size='small'>
            {Number.isFinite(value) ? value : '-'}
          </Tag>
        );
      },
    },
    {
      title: t('更新时间'),
      dataIndex: 'updated_at',
      width: 180,
      render: (text) => (
        <Text type='secondary' size='small'>
          {formatTs(text)}
        </Text>
      ),
    },
    {
      title: '',
      key: 'operate',
      width: 90,
      render: (_, record) => (
        <Button
          size='small'
          type='danger'
          theme='light'
          icon={<IconDelete />}
          onClick={() => deleteDiscount(record)}
        >
          {t('删除')}
        </Button>
      ),
    },
  ];

  return (
    <SideSheet
      visible={visible}
      placement='right'
      width={isMobile ? '100%' : 720}
      bodyStyle={{ padding: 0 }}
      onCancel={onCancel}
      title={
        <Space>
          <Tag color='blue' shape='circle'>
            {t('管理')}
          </Tag>
          <Typography.Title heading={4} className='m-0'>
            {t('模型专属折扣')}
          </Typography.Title>
          <Text type='tertiary' className='ml-2'>
            {user?.username || '-'} (ID: {user?.id || '-'})
          </Text>
        </Space>
      }
    >
      <div className='p-4'>
        <Banner
          type='info'
          closeIcon={null}
          className='mb-4'
          message={t(
            '专属折扣仅在使用账户余额支付时生效；只作用于输入输出 token，缓存部分不打折；按次计费与阶梯表达式计费的模型不支持；折扣与分组倍率叠加，范围 (0, 1]。',
          )}
        />

        {/* 顶部操作栏：新增折扣 */}
        <div className='flex flex-col md:flex-row md:items-center gap-3 mb-4'>
          <Select
            placeholder={t('请选择模型')}
            optionList={modelOptions}
            value={modelName}
            onChange={setModelName}
            style={{ width: isMobile ? '100%' : 280 }}
            loading={loading}
            filter={selectFilter}
            searchable
            showClear
            emptyContent={t('暂无数据')}
          />
          <InputNumber
            min={0.000001}
            max={1}
            step={0.05}
            precision={6}
            value={discount}
            onChange={setDiscount}
            style={{ width: isMobile ? '100%' : 160 }}
          />
          <Button
            type='primary'
            theme='solid'
            icon={<IconPlusCircle />}
            loading={saving}
            onClick={upsertDiscount}
          >
            {t('保存折扣')}
          </Button>
        </div>

        <CardTable
          columns={columns}
          dataSource={records}
          rowKey={(row) => row?.id || row?.model_name}
          loading={loading}
          scroll={{ x: 'max-content' }}
          hidePagination={true}
          empty={
            <Empty
              image={
                <IllustrationNoResult style={{ width: 150, height: 150 }} />
              }
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
              }
              description={t('暂无专属折扣')}
              style={{ padding: 30 }}
            />
          }
          size='middle'
        />
      </div>
    </SideSheet>
  );
};

export default UserModelDiscountModal;
