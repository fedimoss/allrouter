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
  Button,
  InputNumber,
  Popconfirm,
  Spin,
  Switch,
  Typography,
} from '@douyinfe/semi-ui';
import { IconPlus, IconDelete } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, showWarning } from '../../helpers';
import {
  newRuleId,
  parseTimedConfig,
  parseRules,
  serializeRules,
  serializeTimedConfig,
} from '../../helpers/topupGift';

const { Text } = Typography;

const RULES_KEY = 'topup_gift.rules';
const ENABLED_KEY = 'topup_gift.enabled';
const TIMED_KEY = 'topup_gift.timed';

// 服务商维度的"充值赠送"配置模块。规则、开关和倒计时均读写 provider_options，
// 供 /console/provider/reward 页面使用。逻辑与主站 SettingsRechargeGift 对称，仅数据源/API 不同。
export default function ProviderRechargeGift({ provider }) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [timedSaving, setTimedSaving] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const [timeEnabled, setTimeEnabled] = useState(false);
  const [timeDay, setTimeDay] = useState(0);
  const [rules, setRules] = useState([]);
  const [original, setOriginal] = useState({ enabled: false, rulesJson: '' });
  const [timeOriginal, setTimeOriginal] = useState({
    timeEnabled: false,
    timeDay: 0,
    anchored: false,
  });

  const providerId = provider?.id;

  const loadConfig = async () => {
    if (!providerId) return;
    setLoading(true);
    try {
      const res = await API.get(`/api/provider/options/${providerId}`);
      if (res.data?.success) {
        const list = res.data.data || [];
        const rulesOpt = list.find((o) => o.key === RULES_KEY);
        const enabledOpt = list.find((o) => o.key === ENABLED_KEY);
        const timedOpt = list.find((o) => o.key === TIMED_KEY);
        const raw = rulesOpt?.value ?? '';
        const en = enabledOpt?.value === 'true';
        const parsed = parseRules(raw);
        const timed = parseTimedConfig(timedOpt?.value);
        setEnabled(en);
        setRules(parsed);
        setOriginal({ enabled: en, rulesJson: serializeRules(parsed) });
        setTimeEnabled(timed.enabled);
        setTimeDay(timed.day);
        setTimeOriginal({
          timeEnabled: timed.enabled,
          timeDay: timed.day,
          anchored: timed.endTime > 0,
        });
      } else {
        showError(res.data?.message || t('加载失败'));
      }
    } catch (e) {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadConfig();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [providerId]);

  const updateRule = (id, field, value) =>
    setRules((rs) => rs.map((r) => (r.id === id ? { ...r, [field]: value } : r)));
  const removeRule = (id) => setRules((rs) => rs.filter((r) => r.id !== id));
  const addRule = () =>
    setRules((rs) => [...rs, { id: newRuleId(), threshold: null, bonus: null }]);

  const onSubmit = async () => {
    const rulesJson = serializeRules(rules);
    // 分别检测规则与开关是否变化，各自变化的才提交
    const queue = [];
    if (rulesJson !== original.rulesJson) {
      queue.push(
        API.put(`/api/provider/options/${providerId}`, {
          key: RULES_KEY,
          value: rulesJson,
        }),
      );
    }
    if (enabled !== original.enabled) {
      queue.push(
        API.put(`/api/provider/options/${providerId}`, {
          key: ENABLED_KEY,
          value: String(enabled),
        }),
      );
    }
    if (queue.length === 0) {
      return showWarning(t('你似乎并没有修改什么'));
    }
    setSaving(true);
    try {
      const results = await Promise.all(queue);
      const failed = results.find((r) => !r.data?.success);
      if (failed) {
        showError(failed.data?.message || t('保存失败'));
      } else {
        showSuccess(t('保存成功'));
        setOriginal({ enabled, rulesJson });
      }
    } catch (e) {
      showError(t('保存失败，请重试'));
    } finally {
      setSaving(false);
    }
  };

  const onTimedSubmit = async () => {
    if (timeEnabled && (!timeDay || timeDay < 1)) {
      return showWarning(t('最少设置1天'));
    }
    const timedJson = serializeTimedConfig(timeEnabled, timeDay);
    const originalJson = serializeTimedConfig(
      timeOriginal.timeEnabled,
      timeOriginal.timeDay,
    );
    if (timedJson === originalJson && (!timeEnabled || timeOriginal.anchored)) {
      return showWarning(t('你似乎并没有修改什么'));
    }

    setTimedSaving(true);
    try {
      const res = await API.put(`/api/provider/options/${providerId}`, {
        key: TIMED_KEY,
        value: timedJson,
      });
      if (!res.data?.success) {
        showError(res.data?.message || t('保存失败'));
      } else {
        showSuccess(t('保存成功'));
        setTimeOriginal({ timeEnabled, timeDay, anchored: timeEnabled });
      }
    } catch (e) {
      showError(t('保存失败，请重试'));
    } finally {
      setTimedSaving(false);
    }
  };

  return (
    <Spin spinning={loading}>
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
          {t('充值赠送')}
        </div>
        <Typography.Text
          type='tertiary'
          size='small'
          style={{ display: 'block', marginBottom: 12 }}
        >
          {t(
            '币种跟随用户充值币种（充$按$送、充¥按¥送）；每次充值按最高命中档赠送；每档每用户仅享受一次，已享受的档位不再重复赠送。',
          )}
        </Typography.Text>

        {/* 启用开关：总闸，未启用时规则保存但不生效 */}
        <div className='flex items-center gap-3' style={{ marginBottom: 12 }}>
          <Text strong>{t('启用充值赠送')}</Text>
          <Switch
            checked={enabled}
            onChange={setEnabled}
            size='default'
            checkedText='｜'
            uncheckedText='〇'
          />
          {!enabled && (
            <Text type='tertiary' size='small'>
              {t('（当前未启用，规则保存后不会生效）')}
            </Text>
          )}
        </div>

        <div className='space-y-2'>
          {rules.length === 0 && (
            <Text type='tertiary' className='block py-2'>
              {t('暂无规则，点击下方按钮添加')}
            </Text>
          )}
          {rules.map((r) => (
            <div key={r.id} className='flex items-center gap-2'>
              <Text
                type='tertiary'
                size='small'
                style={{ whiteSpace: 'nowrap' }}
              >
                {t('充值满')}
              </Text>
              <InputNumber
                min={0}
                precision={2}
                placeholder={t('金额')}
                value={r.threshold}
                onChange={(v) => updateRule(r.id, 'threshold', v)}
                style={{ width: 150 }}
              />
              <Text
                type='tertiary'
                size='small'
                style={{ whiteSpace: 'nowrap' }}
              >
                {t('赠送')}
              </Text>
              <InputNumber
                min={0}
                precision={2}
                placeholder={t('金额')}
                value={r.bonus}
                onChange={(v) => updateRule(r.id, 'bonus', v)}
                style={{ width: 150 }}
              />
              <Popconfirm
                title={t('确认删除该规则？')}
                onConfirm={() => removeRule(r.id)}
              >
                <Button icon={<IconDelete />} type='danger' theme='borderless' />
              </Popconfirm>
            </div>
          ))}
        </div>

        <div
          style={{ marginTop: 16, display: 'flex', alignItems: 'center', gap: 4 }}
        >
          <Button icon={<IconPlus />} theme='outline' onClick={addRule}>
            {t('添加规则')}
          </Button>
          <Button size='default' loading={saving} onClick={onSubmit}>
            {t('保存充值赠送设置')}
          </Button>
        </div>

        <div
          style={{
            borderTop: '1px solid var(--semi-color-border)',
            marginTop: 20,
            paddingTop: 16,
          }}
        >
          <div className='flex items-center gap-3' style={{ marginBottom: 12 }}>
            <Text strong>{t('启用充值赠送倒计时')}</Text>
            <Switch
              checked={timeEnabled}
              onChange={setTimeEnabled}
              size='default'
              checkedText='｜'
              uncheckedText='〇'
            />
            {!timeEnabled && (
              <Text type='tertiary' size='small'>
                {t('（当前未启用，倒计时不会显示）')}
              </Text>
            )}
          </div>
          <div className='flex items-center gap-2'>
            <Text type='tertiary' size='small'>
              {t('倒计时')}
            </Text>
            <InputNumber
              min={1}
              precision={0}
              disabled={!timeEnabled}
              placeholder={t('天数')}
              value={timeDay}
              onChange={(value) => setTimeDay(Number(value) || 0)}
              style={{ width: 200 }}
            />
            <Text type='tertiary' size='small'>
              {t('天后活动结束')}
            </Text>
            <Button loading={timedSaving} onClick={onTimedSubmit}>
              {t('保存设置')}
            </Button>
          </div>
        </div>
      </div>
    </Spin>
  );
}
