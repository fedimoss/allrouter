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
  Space,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconHistogram,
  IconRefresh,
  IconSettingStroked,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, renderQuota, showError, showSuccess } from '../../helpers';
import {
  displayAmountToQuota,
  quotaToDisplayAmount,
} from '../../helpers/quota';
import ProviderRechargeGift from './ProviderRechargeGift';

const { Text } = Typography;

const emptyConfig = {
  id: 0,
  provider_id: 0,
  quota_for_new_user: 0,
  quota_for_inviter: 0,
  quota_for_invitee: 0,
  checkin_enabled: false,
  checkin_min_quota: 0,
  checkin_max_quota: 0,
  register_gift_subscription_plan_id: 0,
  airdrop_subscription_plan_id: 0,
};

const quotaFields = [
  ['quota_for_new_user', 'quota_for_new_user_amount'],
  ['quota_for_inviter', 'quota_for_inviter_amount'],
  ['quota_for_invitee', 'quota_for_invitee_amount'],
  ['checkin_min_quota', 'checkin_min_quota_amount'],
  ['checkin_max_quota', 'checkin_max_quota_amount'],
];

const toFormValues = (config) => {
  const values = { ...emptyConfig, ...(config || {}) };
  quotaFields.forEach(([quotaKey, amountKey]) => {
    values[amountKey] = Number(
      quotaToDisplayAmount(values[quotaKey] || 0).toFixed(6),
    );
  });
  return values;
};

const toPayload = (values, config) => {
  const payload = { ...emptyConfig, ...(config || {}), ...(values || {}) };
  quotaFields.forEach(([quotaKey, amountKey]) => {
    payload[quotaKey] = displayAmountToQuota(payload[amountKey]);
    delete payload[amountKey];
  });
  payload.checkin_enabled = payload.checkin_enabled === true;
  return payload;
};

const Metric = ({ label, value, strong }) => (
  <div
    style={{
      border: '1px solid var(--semi-color-border)',
      borderRadius: 6,
      padding: '14px 16px',
      minHeight: 78,
      background: 'var(--semi-color-bg-1)',
    }}
  >
    <Text type='secondary' size='small'>
      {label}
    </Text>
    <div style={{ marginTop: 8, fontSize: 18, fontWeight: strong ? 700 : 600 }}>
      {value}
    </div>
  </div>
);

const ProviderRewardPanel = ({ provider, adminMode, mode = 'all' }) => {
  const { t } = useTranslation();
  const formRef = useRef(null);
  const [config, setConfig] = useState(emptyConfig);
  const [summary, setSummary] = useState({});
  const [subscriptionPlans, setSubscriptionPlans] = useState([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const baseUrl = useMemo(() => {
    if (!provider?.id) return '';
    return adminMode
      ? `/api/provider/admin/${provider.id}/reward`
      : '/api/provider/reward';
  }, [adminMode, provider?.id]);

  const formValues = useMemo(() => toFormValues(config), [config]);
  const subscriptionPlanOptions = useMemo(
    () =>
      subscriptionPlans.map((item) => {
        const plan = item?.plan || item;
        return {
          label: plan?.title || `#${plan?.id}`,
          value: Number(plan?.id || 0),
          disabled: plan?.enabled === false,
        };
      }),
    [subscriptionPlans],
  );

  const loadRewardData = async () => {
    if (!baseUrl) return;
    setLoading(true);
    try {
      const plansUrl = adminMode
        ? '/api/subscription/admin/plans'
        : '/api/provider/subscription/plans';
      const [configRes, summaryRes, plansRes] = await Promise.all([
        API.get(`${baseUrl}/config`, { disableDuplicate: true }),
        API.get(`${baseUrl}/summary`, { disableDuplicate: true }),
        API.get(plansUrl, { disableDuplicate: true }),
      ]);
      if (configRes.data.success) {
        setConfig({ ...emptyConfig, ...(configRes.data.data || {}) });
      } else {
        showError(configRes.data.message);
      }
      if (summaryRes.data.success) {
        setSummary(summaryRes.data.data || {});
      } else {
        showError(summaryRes.data.message);
      }
      if (plansRes.data.success) {
        setSubscriptionPlans(
          (plansRes.data.data || []).filter((item) => {
            const plan = item?.plan || item;
            return Number(plan?.provider_id || 0) === Number(provider?.id || 0);
          }),
        );
      } else {
        showError(plansRes.data.message);
      }
    } catch (error) {
      showError(error);
    }
    setLoading(false);
  };

  useEffect(() => {
    if (!provider?.id) return;
    loadRewardData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [provider?.id, baseUrl, mode]);

  useEffect(() => {
    if (!formRef.current) return;
    formRef.current.setValues(formValues);
  }, [formValues]);

  const submitConfig = async () => {
    if (!baseUrl) return;
    const values = formRef.current?.getValues?.() || {};
    const payload = toPayload(values, config);
    setSaving(true);
    try {
      const res = await API.put(`${baseUrl}/config`, payload);
      if (res.data.success) {
        showSuccess(t('保存成功'));
        setConfig({ ...emptyConfig, ...(res.data.data || payload) });
        await loadRewardData();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    }
    setSaving(false);
  };

  const summaryRows = useMemo(
    () => [
      {
        key: 'new_user_quota',
        label: t('新用户注册奖励'),
        quota: summary.new_user_quota || 0,
      },
      {
        key: 'invitee_quota',
        label: t('被邀请人奖励'),
        quota: summary.invitee_quota || 0,
      },
      {
        key: 'inviter_quota',
        label: t('邀请人奖励'),
        quota: summary.inviter_quota || 0,
      },
      {
        key: 'checkin_quota',
        label: t('签到奖励'),
        quota: summary.checkin_quota || 0,
      },
      {
        key: 'redemption_quota',
        label: t('兑换码奖励'),
        quota: summary.redemption_quota || 0,
      },
      {
        key: 'consume_rebate_quota',
        label: t('消费返利'),
        quota: summary.consume_rebate_quota || 0,
      },
      {
        key: 'topup_rebate_quota',
        label: t('充值返利'),
        quota: summary.topup_rebate_quota || 0,
      },
    ],
    [summary, t],
  );

  const summaryColumns = [
    { title: t('奖励类型'), dataIndex: 'label' },
    {
      title: t('累计额度'),
      dataIndex: 'quota',
      render: (quota) => renderQuota(quota),
    },
  ];

  if (!provider?.id) {
    return (
      <Text type='tertiary'>
        {t('当前账户不是服务商主账号，无法访问奖励设置。')}
      </Text>
    );
  }

  const showConfig = mode !== 'summary';
  const showSummary = mode !== 'config';

  return (
    <Spin spinning={loading}>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          gap: 12,
          marginBottom: 12,
        }}
      >
        <div>
          <Text strong>{provider.name}</Text>
          <div>
            <Tag color='blue'>
              {t('服务商 ID')} {provider.id}
            </Tag>
          </div>
        </div>
        <Space>
          <Button icon={<IconRefresh />} onClick={loadRewardData}>
            {t('刷新')}
          </Button>
          {showConfig ? (
            <Button type='primary' loading={saving} onClick={submitConfig}>
              {t('保存配置')}
            </Button>
          ) : null}
        </Space>
      </div>

      {showConfig && (
        <>
          <Form
            key={`${provider?.id || 0}-${config?.id || 'default'}-reward-config`}
            initValues={formValues}
            getFormApi={(api) => (formRef.current = api)}
            labelPosition='top'
          >
            <div
              style={{
                border: '1px solid var(--semi-color-border)',
                borderRadius: 8,
                padding: '16px 20px',
                marginBottom: 16,
                background: '#fff',
              }}
            >
              <div style={{ marginBottom: 12, fontWeight: 600 }}>
                {t('注册与邀请奖励')}
              </div>
              <Text type='tertiary' size='small' style={{ display: 'block', marginBottom: 12 }}>
                {t('新用户注册、邀请人及被邀请人获得的奖励额度。')}
              </Text>
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
                  columnGap: 24,
                }}
              >
                <Form.InputNumber
                  field='quota_for_new_user_amount'
                  label={t('注册赠送')}
                  min={0}
                  step={0.01}
                  precision={6}
                />
                <Form.InputNumber
                  field='quota_for_invitee_amount'
                  label={t('被邀请人奖励')}
                  min={0}
                  step={0.01}
                  precision={6}
                />
                <Form.InputNumber
                  field='quota_for_inviter_amount'
                  label={t('邀请人奖励')}
                  min={0}
                  step={0.01}
                  precision={6}
                />
                <Form.Select
                  field='register_gift_subscription_plan_id'
                  label={t('注册赠送订阅')}
                  optionList={[
                    { label: t('不赠送订阅'), value: 0 },
                    ...subscriptionPlanOptions,
                  ]}
                />
                <Form.Select
                  field='airdrop_subscription_plan_id'
                  label={t('空投订阅方案')}
                  optionList={[
                    { label: t('不空投订阅'), value: 0 },
                    ...subscriptionPlanOptions,
                  ]}
                />
              </div>
            </div>

            <div
              style={{
                border: '1px solid var(--semi-color-border)',
                borderRadius: 8,
                padding: '16px 20px',
                marginBottom: 16,
                background: '#fff',
              }}
            >
              <div style={{ marginBottom: 12, fontWeight: 600 }}>
                {t('签到奖励')}
              </div>
              <Text type='tertiary' size='small' style={{ display: 'block', marginBottom: 12 }}>
                {t('用户每日签到可随机获得的奖励额度范围。')}
              </Text>
              <Form.Switch
                field='checkin_enabled'
                label={t('启用签到奖励')}
              />
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
                  columnGap: 24,
                }}
              >
                <Form.InputNumber
                  field='checkin_min_quota_amount'
                  label={t('签到最小奖励')}
                  min={0}
                  step={0.01}
                  precision={6}
                />
                <Form.InputNumber
                  field='checkin_max_quota_amount'
                  label={t('签到最大奖励')}
                  min={0}
                  step={0.01}
                  precision={6}
                />
              </div>
            </div>

            <ProviderRechargeGift provider={provider} />

            <Text type='secondary' size='small'>
              {t('金额输入会按当前额度显示设置换算为系统原始 quota。')}
            </Text>
          </Form>
        </>
      )}

      {showSummary && (
        <>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
              gap: 12,
              marginBottom: 16,
            }}
          >
            <Metric
              label={t('服务商 ID')}
              value={
                <Tag color='blue'>
                  {summary.provider_id || provider?.id || '-'}
                </Tag>
              }
            />
            <Metric
              label={t('累计奖励支出')}
              value={renderQuota(summary.welfare_quota || 0)}
              strong
            />
            <Metric
              label={t('邀请体系奖励')}
              value={renderQuota(
                (summary.inviter_quota || 0) +
                  (summary.invitee_quota || 0) +
                  (summary.consume_rebate_quota || 0) +
                  (summary.topup_rebate_quota || 0),
              )}
            />
            <Metric
              label={t('运营活动奖励')}
              value={renderQuota(
                (summary.new_user_quota || 0) +
                  (summary.checkin_quota || 0) +
                  (summary.redemption_quota || 0),
              )}
            />
          </div>
          <Table
            rowKey='key'
            columns={summaryColumns}
            dataSource={summaryRows}
            pagination={false}
          />
        </>
      )}
    </Spin>
  );
};

export default ProviderRewardPanel;
