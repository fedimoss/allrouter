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
  Form,
  InputNumber,
  Popconfirm,
  Spin,
  Switch,
  Typography,
} from '@douyinfe/semi-ui';
import { IconPlus, IconDelete } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, showWarning } from '../../../helpers';
import {
  newRuleId,
  parseRules,
  serializeRules,
} from '../../../helpers/topupGift';

const { Text } = Typography;

export default function SettingsRechargeGift(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [enabled, setEnabled] = useState(false);
  // 倒计时模块：合并到单个 option 字段 TopUpGiftTimed，value 为 {enabled, day} 的 JSON
  const [timeEnabled, setTimeEnabled] = useState(false);
  const [timeDay, setTimeDay] = useState(0);
  const [rules, setRules] = useState([]);
  const [original, setOriginal] = useState({ enabled: false, rulesJson: '' });
  const [timeOriginal, setTimeOriginal] = useState({
    timeEnabled: false,
    timeDay: 0,
  });

  // 安全解析 option TopUpGiftTimed（{enabled, day} JSON 字符串）
  const parseTopUpGiftTimed = (raw) => {
    if (!raw) return { enabled: false, day: 0 };
    try {
      const obj = JSON.parse(raw);
      return {
        enabled: obj?.enabled === true,
        day: Number(obj?.day) || 0,
      };
    } catch {
      return { enabled: false, day: 0 };
    }
  };

  // 从父组件 option 初始化（启用开关 + 规则列表 + 倒计时）
  useEffect(() => {
    const en = props.options?.TopUpGiftEnabled === true;
    const raw = props.options?.TopUpGiftRules ?? '';
    const parsed = parseRules(raw);
    setEnabled(en);
    setRules(parsed);
    setOriginal({ enabled: en, rulesJson: serializeRules(parsed) });

    // 倒计时模块回显
    const timed = parseTopUpGiftTimed(props.options?.TopUpGiftTimed);
    setTimeEnabled(timed.enabled);
    setTimeDay(timed.day);
    setTimeOriginal({ timeEnabled: timed.enabled, timeDay: timed.day });
  }, [
    props.options?.TopUpGiftEnabled,
    props.options?.TopUpGiftRules,
    props.options?.TopUpGiftTimed,
  ]);

  const updateRule = (id, field, value) =>
    setRules((rs) => rs.map((r) => (r.id === id ? { ...r, [field]: value } : r)));
  const removeRule = (id) => setRules((rs) => rs.filter((r) => r.id !== id));
  const addRule = () =>
    setRules((rs) => [...rs, { id: newRuleId(), threshold: null, bonus: null }]);

  const onSubmit = async () => {
    const rulesJson = serializeRules(rules);
    // 分别检测"启用开关"与"规则列表"是否变化，各自变化的才提交
    const queue = [];
    if (rulesJson !== original.rulesJson) {
      queue.push(
        API.put('/api/option/', { key: 'TopUpGiftRules', value: rulesJson }),
      );
    }
    if (enabled !== original.enabled) {
      queue.push(
        API.put('/api/option/', {
          key: 'TopUpGiftEnabled',
          value: String(enabled),
        }),
      );
    }
    if (queue.length === 0) {
      return showWarning(t('你似乎并没有修改什么'));
    }
    setLoading(true);
    try {
      const results = await Promise.all(queue);
      const failed = results.find((r) => !r.data?.success);
      if (failed) {
        showError(failed.data?.message || t('保存失败'));
      } else {
        showSuccess(t('保存成功'));
        setOriginal({ enabled, rulesJson });
        props.refresh?.();
      }
    } catch (e) {
      showError(t('保存失败，请重试'));
    } finally {
      setLoading(false);
    }
  };

  // 保存「启用充值赠送倒计时」：与保存充值赠送设置一样调用 /api/option/
  // 合并为单个字段 TopUpGiftTimed，value 为 {enabled, day} 的 JSON 字符串
  const onTimedSubmit = async () => {
    // 启用倒计时时，天数必须 >=1，否则提示
    if (timeEnabled && (!timeDay || timeDay < 1)) {
      return showWarning(t('最少设置1天'));
    }
    const timedJson = JSON.stringify({ enabled: timeEnabled, day: timeDay });
    const origJson = JSON.stringify({
      enabled: timeOriginal.timeEnabled,
      day: timeOriginal.timeDay,
    });
    if (timedJson === origJson) {
      return showWarning(t('你似乎并没有修改什么'));
    }
    setLoading(true);
    try {
      const res = await API.put('/api/option/', {
        key: 'TopUpGiftTimed',
        value: timedJson,
      });
      if (!res.data?.success) {
        showError(res.data?.message || t('保存失败'));
      } else {
        showSuccess(t('保存成功'));
        setTimeOriginal({ timeEnabled, timeDay });
        props.refresh?.();
      }
    } catch (e) {
      showError(t('保存失败，请重试'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Spin spinning={loading}>
      <Form style={{ marginBottom: 15 }}>
        <Form.Section text={t('充值赠送')}>
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

          <Typography.Text
            type='tertiary'
            style={{ marginBottom: 12, display: 'block' }}
          >
            {t(
              '币种跟随用户充值币种（充$按$送、充¥按¥送）；每次充值按最高命中档赠送；每档每用户仅享受一次，已享受的档位不再重复赠送。',
            )}
          </Typography.Text>

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

          <div style={{ marginTop: 16, display: 'flex', alignItems: 'center', gap: 4 }}>
            <Button icon={<IconPlus />} theme='outline' onClick={addRule}>
              {t('添加规则')}
            </Button>
            <Button size='default' onClick={onSubmit}>
              {t('保存充值赠送设置')}
            </Button>
          </div>
        </Form.Section>
      </Form>

      <Form style={{ marginBottom: 15 }}>
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
        {timeEnabled && (
          <div className='flex items-center gap-2'>
            <Text type='tertiary' size='small'>
              {t('倒计时')}
            </Text>
            <InputNumber
              min={0}
              precision={0}
              placeholder={t('天数')}
              value={timeDay}
              onChange={(v) => setTimeDay(Number(v) || 0)}
              style={{ width: 200 }}
            />
            <Text type='tertiary' size='small'>
              {t('天后活动结束')}
            </Text>
            <Button size='default' onClick={onTimedSubmit}>
              {t('保存设置')}
            </Button>
          </div>
        )}
      </Form>
    </Spin>
  );
}
