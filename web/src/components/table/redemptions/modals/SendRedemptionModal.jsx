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
import { Modal, Form } from '@douyinfe/semi-ui';

/**
 * 发放兑换码弹窗：标记发放时弹出，可选输入邮箱将兑换码发送过去。
 * - 点击「取消」关闭弹窗，不标记
 * - 邮箱为空点击「确定」：仅标记发放，不发邮件
 * - 输入邮箱点击「确定」：标记发放并发送兑换码邮件
 */
const SendRedemptionModal = ({
  visible,
  onCancel,
  onOk,
  loading,
  t,
}) => {
  const formApiRef = React.useRef(null);
  const [email, setEmail] = useState('');

  // 每次打开弹窗时清空上次输入
  useEffect(() => {
    if (visible) {
      setEmail('');
      formApiRef.current?.setValues({ email: '' });
    }
  }, [visible]);

  const handleOk = () => {
    onOk(email.trim());
  };

  return (
    <Modal
      title={t('标记为已发放')}
      visible={visible}
      onCancel={onCancel}
      onOk={handleOk}
      okText={t('确定')}
      cancelText={t('取消')}
      confirmLoading={loading}
      closeOnEsc
    >
      <p className='text-sm' style={{ color: 'var(--semi-color-text-1)' }}>
        {t('填写邮箱可将兑换码发送给对方，留空则仅标记发放。')}
      </p>
      <Form getFormApi={(api) => (formApiRef.current = api)} onSubmit={handleOk}>
        <Form.Input
          field='email'
          noLabel
          placeholder={t('请输入邮箱（可选）')}
          type='email'
          showClear
          onChange={(val) => setEmail(val || '')}
          rules={[
            {
              validator: (rule, v) => {
                const val = (v || '').trim();
                // 留空合法（仅标记发放）；填写时校验邮箱格式
                if (val === '') return Promise.resolve();
                return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(val)
                  ? Promise.resolve()
                  : Promise.reject(t('请输入有效的邮箱地址'));
              },
            },
          ]}
        />
      </Form>
    </Modal>
  );
};

export default SendRedemptionModal;
