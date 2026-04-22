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
import { Button, Form, Spin } from '@douyinfe/semi-ui';
import {
  API,
  removeTrailingSlash,
  showError,
  showSuccess,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

/**
 * SettingsGeneralPayment 通用支付设置组件
 *
 * 功能：
 * 1. 服务器地址配置 - 影响支付回调地址和默认首页显示
 * 2. 邀请充值返利比例配置 - 设置邀请返利的百分比
 *
 * 返利功能说明：
 * - 当被邀请人充值成功后，邀请人会获得返利额度
 * - 返利额度 = 被邀请人充值额度 × 返利比例 ÷ 100
 * - 例如：返利比例设置为 10%，被邀请人充值 100 美元，邀请人获得 10 美元价值的额度
 * - 设置为 0 表示关闭返利功能
 *
 * @param {Object} props - 组件属性
 * @param {Object} props.options - 当前配置选项
 * @param {Function} props.refresh - 刷新配置的回调函数
 */
export default function SettingsGeneralPayment(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);

  // 状态管理：服务器地址和返利比例
  const [inputs, setInputs] = useState({
    ServerAddress: '',          // 服务器地址（影响支付回调）
    InviteTopupRebateRatio: 0, // 邀请充值返利比例（0-100之间的数字）
  });
  const formApiRef = useRef(null);

  // 组件挂载时初始化表单数据
  useEffect(() => {
    if (props.options && formApiRef.current) {
      const currentInputs = {
        ServerAddress: props.options.ServerAddress || '',
        // 解析返利比例，确保是有效数字，默认为 0
        InviteTopupRebateRatio:
          props.options.InviteTopupRebateRatio !== undefined
            ? parseFloat(props.options.InviteTopupRebateRatio) || 0
            : 0,
      };
      setInputs(currentInputs);
      formApiRef.current.setValues(currentInputs);
    }
  }, [props.options]);

  // 表单值变化处理函数
  const handleFormChange = (values) => {
    setInputs(values);
  };

  // 提交通用支付设置
  const submitGeneralPayment = async () => {
    setLoading(true);
    try {
      // 准备要更新的配置项
      const updates = [
        {
          key: 'ServerAddress',
          value: removeTrailingSlash(inputs.ServerAddress), // 移除末尾斜杠
        },
        {
          key: 'InviteTopupRebateRatio',
          value: inputs.InviteTopupRebateRatio || 0, // 确保数值有效
        },
      ];

      // 逐个更新配置
      for (const update of updates) {
        const res = await API.put('/api/option/', update);
        if (!res.data.success) {
          showError(res.data.message);
          setLoading(false);
          return;
        }
      }

      // 更新成功提示
      showSuccess(t('更新成功'));

      // 触发父组件的刷新回调
      props.refresh && props.refresh();
    } catch (error) {
      // 更新失败提示
      showError(t('更新失败'));
    }
    setLoading(false);
  };

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={handleFormChange}
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={t('通用设置')}>
          {/* 服务器地址配置 */}
          <Form.Input
            field='ServerAddress'
            label={t('服务器地址')}
            placeholder={'https://yourdomain.com'}
            style={{ width: '100%' }}
            extraText={t(
              '该服务器地址将影响支付回调地址以及默认首页展示的地址，请确保正确配置',
            )}
          />

          {/* 邀请充值返利比例配置 */}
          <Form.InputNumber
            field='InviteTopupRebateRatio'
            label={t('邀请充值返利比例')}
            min={0}
            max={100}
            step={0.1}
            suffix='%'
            style={{ width: '100%' }}
            extraText={t(
              '被邀请人充值成功后，按充值到账额度的百分比返利给邀请人，0 表示关闭',
            )}
          />

          {/* 保存按钮 */}
          <Button onClick={submitGeneralPayment}>
            {t('保存通用支付设置')}
          </Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
