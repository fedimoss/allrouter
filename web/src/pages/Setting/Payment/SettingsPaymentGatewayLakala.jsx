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

import React, { useEffect, useState, useRef } from 'react';
import {
  Button,
  Form,
  Row,
  Col,
  Typography,
  Spin,
} from '@douyinfe/semi-ui';
const { Text } = Typography;
import { API, removeTrailingSlash, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';

export default function SettingsPaymentGatewayLakala(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    LakalaAppID: '',
    LakalaSerialNo: '',
    LakalaPrivateKey: '',
    LakalaPublicCert: '',
    LakalaMerchantNo: '',
    LakalaTermNo: '',
    LakalaCallbackAddress: '',
  });
  const [originInputs, setOriginInputs] = useState({});
  const [formReady, setFormReady] = useState(false);
  const formApiRef = useRef(null);

  useEffect(() => {
    if (props.options && formReady && formApiRef.current) {
      const currentInputs = {
        LakalaAppID: props.options.LakalaAppID || '',
        LakalaSerialNo: props.options.LakalaSerialNo || '',
        LakalaPrivateKey: props.options.LakalaPrivateKey || '',
        LakalaPublicCert: props.options.LakalaPublicCert || '',
        LakalaMerchantNo: props.options.LakalaMerchantNo || '',
        LakalaTermNo: props.options.LakalaTermNo || '',
        LakalaCallbackAddress: props.options.LakalaCallbackAddress || '',
      };
      setInputs(currentInputs);
      setOriginInputs({ ...currentInputs });
      formApiRef.current.setValues(currentInputs);
    }
  }, [props.options, formReady]);

  const handleFormChange = (values) => {
    setInputs(values);
  };

  const submitLakalaSetting = async () => {
    setLoading(true);
    try {
      const keys = [
        'LakalaAppID',
        'LakalaSerialNo',
        'LakalaPrivateKey',
        'LakalaPublicCert',
        'LakalaMerchantNo',
        'LakalaTermNo',
        'LakalaCallbackAddress',
      ];

      const options = keys
        .filter((key) => inputs[key] && inputs[key] !== '')
        .map((key) => ({
          key,
          value: key === 'LakalaCallbackAddress'
            ? removeTrailingSlash(inputs[key])
            : inputs[key],
        }));

      const results = await Promise.all(
        options.map((opt) =>
          API.put('/api/option/', {
            key: opt.key,
            value: opt.value,
          }),
        ),
      );

      const errorResults = results.filter((res) => !res.data.success);
      if (errorResults.length > 0) {
        errorResults.forEach((res) => showError(res.data.message));
      } else {
        showSuccess(t('更新成功'));
        setOriginInputs({ ...inputs });
        props.refresh?.();
      }
    } catch (error) {
      showError(t('更新失败'));
    }
    setLoading(false);
  };

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={handleFormChange}
        getFormApi={(api) => { formApiRef.current = api; setFormReady(true); }}
      >
        <Form.Section text={t('拉卡拉设置')}>
          <Text>
            {t('拉卡拉支付配置，如需帮助请')}
            <a
              href='https://moss.lakala.com/'
              target='_blank'
              rel='noreferrer'
            >
              {t('点击此处')}
            </a>
            {t('进入拉卡拉开放平台。')}
          </Text>

          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}>
            <Col xs={24} sm={12} md={12} lg={12} xl={12}>
              <Form.Input
                field='LakalaAppID'
                label={t('AppID')}
                placeholder={t('机构接入申请的 appid')}
              />
            </Col>
            <Col xs={24} sm={12} md={12} lg={12} xl={12}>
              <Form.Input
                field='LakalaSerialNo'
                label={t('证书序列号')}
                placeholder={t('加签用的证书序列号')}
              />
            </Col>
          </Row>
          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.TextArea
                field='LakalaPrivateKey'
                label={t('RSA 私钥')}
                placeholder={t('-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----')}
                rows={6}
                style={{ fontFamily: 'monospace', fontSize: 12 }}
              />
            </Col>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.TextArea
                field='LakalaPublicCert'
                label={t('拉卡拉公钥证书')}
                placeholder={t('-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----')}
                rows={6}
                style={{ fontFamily: 'monospace', fontSize: 12 }}
              />
            </Col>
          </Row>

          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}>
            <Col xs={24} sm={12} md={12} lg={12} xl={12}>
              <Form.Input
                field='LakalaMerchantNo'
                label={t('商户号')}
                placeholder={t('拉卡拉分配的商户号')}
              />
            </Col>
            <Col xs={24} sm={12} md={12} lg={12} xl={12}>
              <Form.Input
                field='LakalaTermNo'
                label={t('终端号')}
                placeholder={t('拉卡拉分配的业务终端号')}
              />
            </Col>
          </Row>

          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='LakalaCallbackAddress'
                label={t('拉卡拉回调地址')}
                placeholder={t('例如：https://yourdomain.com')}
              />
            </Col>
          </Row>

          <Button
            style={{ marginTop: 20 }}
            onClick={submitLakalaSetting}
          >
            {t('保存拉卡拉设置')}
          </Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
