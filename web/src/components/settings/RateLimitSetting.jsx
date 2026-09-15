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
import { Button, Card, Col, Form, Row, Spin } from '@douyinfe/semi-ui';

import {
  API,
  showError,
  showSuccess,
  showWarning,
  toBoolean,
} from '../../helpers';
import { useTranslation } from 'react-i18next';
import RequestRateLimit from '../../pages/Setting/RateLimit/SettingsRequestRateLimit';

const RateLimitSetting = () => {
  let [inputs, setInputs] = useState({
    ModelRequestRateLimitEnabled: false,
    ModelRequestRateLimitCount: 0,
    ModelRequestRateLimitSuccessCount: 1000,
    ModelRequestRateLimitDurationMinutes: 1,
    ModelRequestRateLimitGroup: '',
    ClientIPBlacklist: '',
  });

  let [loading, setLoading] = useState(false);

  const getOptions = async () => {
    const res = await API.get('/api/option/');
    const { success, message, data } = res.data;
    if (success) {
      let newInputs = {};
      data.forEach((item) => {
        if (item.key === 'ModelRequestRateLimitGroup') {
          item.value = JSON.stringify(JSON.parse(item.value), null, 2);
        }

        if (item.key.endsWith('Enabled')) {
          newInputs[item.key] = toBoolean(item.value);
        } else {
          newInputs[item.key] = item.value;
        }
      });

      setInputs((previous) => ({
        ...previous,
        ...newInputs,
        ClientIPBlacklist: newInputs.ClientIPBlacklist ?? '',
      }));
    } else {
      showError(message);
    }
  };
  async function onRefresh() {
    try {
      setLoading(true);
      await getOptions();
      // showSuccess('刷新成功');
    } catch (error) {
      showError('刷新失败');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    onRefresh();
  }, []);

  return (
    <>
      <Spin spinning={loading} size='large'>
        {/* AI请求速率限制 */}
        <Card style={{ marginTop: '10px' }}>
          <RequestRateLimit options={inputs} refresh={onRefresh} />
        </Card>
        <Card style={{ marginTop: '10px' }}>
          <ClientIPBlacklist options={inputs} refresh={onRefresh} />
        </Card>
      </Spin>
    </>
  );
};

function ClientIPBlacklist({ options, refresh }) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [value, setValue] = useState('');
  const [originalValue, setOriginalValue] = useState('');
  const refForm = useRef();

  useEffect(() => {
    const current = String(options?.ClientIPBlacklist ?? '');
    setValue(current);
    setOriginalValue(current);
    refForm.current?.setValues({ ClientIPBlacklist: current });
  }, [options?.ClientIPBlacklist]);

  const onSubmit = async () => {
    if (value === originalValue) {
      showWarning(t('你似乎并没有修改什么'));
      return;
    }
    setLoading(true);
    try {
      const response = await API.put('/api/option/', {
        key: 'ClientIPBlacklist',
        value,
      });
      if (!response?.data?.success) {
        showError(response?.data?.message || t('保存失败，请重试'));
        return;
      }
      showSuccess(t('保存成功'));
      await refresh();
    } catch (error) {
      showError(t('保存失败，请重试'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Spin spinning={loading}>
      <Form
        values={{ ClientIPBlacklist: value }}
        getFormApi={(formAPI) => (refForm.current = formAPI)}
        style={{ marginBottom: 15 }}
      >
        <Form.Section text={t('客户端 IP 黑名单')}>
          <Row gutter={16}>
            <Col xs={24} sm={24} md={16} lg={16} xl={16}>
              <Form.TextArea
                field='ClientIPBlacklist'
                label={t('禁止访问的客户端 IP')}
                placeholder={t(
                  '每行一个 IP 或 CIDR，例如：203.0.113.8 或 203.0.113.0/24',
                )}
                autosize={{ minRows: 3, maxRows: 10 }}
                extraText={t(
                  '命中后会拒绝访问 API、模型和注册等请求；留空表示不启用。请确认代理已正确传递真实客户端 IP。',
                )}
                onChange={(nextValue) => setValue(nextValue ?? '')}
              />
            </Col>
          </Row>
          <Row>
            <Button size='default' onClick={onSubmit}>
              {t('保存 IP 黑名单')}
            </Button>
          </Row>
        </Form.Section>
      </Form>
    </Spin>
  );
}

export default RateLimitSetting;
