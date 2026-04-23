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
  Banner,
  Button,
  Form,
  Row,
  Col,
  Typography,
  Spin,
  Input,
  InputNumber,
} from '@douyinfe/semi-ui';
const { Text } = Typography;
import {
  API,
  removeTrailingSlash,
  showError,
  showSuccess,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

// 固定支持的币种列表：币种代码 → 显示标签
const CURRENCY_LABELS = {
  USD: 'USD ($)',
  CNY: 'CNY (¥)',
};

export default function SettingsPaymentGateway(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    StripeApiSecret: '',
    StripeWebhookSecret: '',
    StripeUnitPrice: 8.0,
    StripeMinTopUp: 1,
    StripePromotionCodesEnabled: false,
  });
  const [originInputs, setOriginInputs] = useState({});
  const formApiRef = useRef(null);

  // 币种 → Stripe Price ID 的映射，用于多币种支付
  const [currencyPriceIds, setCurrencyPriceIds] = useState({
    USD: '', // 美元对应的 Stripe 商品价格 ID
    CNY: '', // 人民币对应的 Stripe 商品价格 ID
  });
  // 人民币对美元的汇率，初始值从后端加载，未配置时默认 7.25
  const [cnyUnitPrice, setCnyUnitPrice] = useState(7.25);

  // 加载币种 Stripe 配置（从后端 currency_stripe_config 表获取）
  useEffect(() => {
    const fetchCurrencyConfigs = async () => {
      try {
        const res = await API.get('/api/currency-stripe-config/');
        if (res.data?.success && Array.isArray(res.data.data)) {
          const mapping = { USD: '', CNY: '' };
          res.data.data.forEach((cfg) => {
            // 只处理白名单中的币种
            if (cfg.currency in mapping) {
              mapping[cfg.currency] = cfg.stripe_price_id || '';
            }
            // 更新人民币汇率
            if (cfg.currency === 'CNY' && Number(cfg.unit_price) > 0) {
              setCnyUnitPrice(Number(cfg.unit_price));
            }
          });
          setCurrencyPriceIds(mapping);
        }
      } catch (e) {
        // 表可能尚未创建（首次运行），静默忽略
      }
    };
    fetchCurrencyConfigs();
  }, []);

  // 加载其他 Stripe 选项
  useEffect(() => {
    if (props.options && formApiRef.current) {
      const currentInputs = {
        StripeApiSecret: props.options.StripeApiSecret || '',
        StripeWebhookSecret: props.options.StripeWebhookSecret || '',
        StripeUnitPrice:
          props.options.StripeUnitPrice !== undefined
            ? parseFloat(props.options.StripeUnitPrice)
            : 8.0,
        StripeMinTopUp:
          props.options.StripeMinTopUp !== undefined
            ? parseFloat(props.options.StripeMinTopUp)
            : 1,
        StripePromotionCodesEnabled:
          props.options.StripePromotionCodesEnabled !== undefined
            ? props.options.StripePromotionCodesEnabled
            : false,
      };
      setInputs(currentInputs);
      setOriginInputs({ ...currentInputs });
      formApiRef.current.setValues(currentInputs);
    }
  }, [props.options]);

  const handleFormChange = (values) => {
    setInputs(values);
  };

  const submitStripeSetting = async () => {
    if (props.options.ServerAddress === '') {
      showError(t('请先填写服务器地址'));
      return;
    }
    // 校验人民币汇率必须为有效正数
    if (!Number.isFinite(Number(cnyUnitPrice)) || Number(cnyUnitPrice) <= 0) {
      showError(t('请输入有效的人民币价格比例'));
      return;
    }

    setLoading(true);
    try {
      const options = [];

      if (inputs.StripeApiSecret && inputs.StripeApiSecret !== '') {
        options.push({ key: 'StripeApiSecret', value: inputs.StripeApiSecret });
      }
      if (inputs.StripeWebhookSecret && inputs.StripeWebhookSecret !== '') {
        options.push({
          key: 'StripeWebhookSecret',
          value: inputs.StripeWebhookSecret,
        });
      }
      if (
        inputs.StripeUnitPrice !== undefined &&
        inputs.StripeUnitPrice !== null
      ) {
        options.push({
          key: 'StripeUnitPrice',
          value: inputs.StripeUnitPrice.toString(),
        });
      }
      if (
        inputs.StripeMinTopUp !== undefined &&
        inputs.StripeMinTopUp !== null
      ) {
        options.push({
          key: 'StripeMinTopUp',
          value: inputs.StripeMinTopUp.toString(),
        });
      }
      if (
        originInputs['StripePromotionCodesEnabled'] !==
          inputs.StripePromotionCodesEnabled &&
        inputs.StripePromotionCodesEnabled !== undefined
      ) {
        options.push({
          key: 'StripePromotionCodesEnabled',
          value: inputs.StripePromotionCodesEnabled ? 'true' : 'false',
        });
      }

      // 发送普通选项请求
      const requestQueue = options.map((opt) =>
        API.put('/api/option/', {
          key: opt.key,
          value: opt.value,
        }),
      );

      // 同时发送币种价格 ID 配置请求（写 currency_stripe_config 表）
      requestQueue.push(
        API.put('/api/currency-stripe-config/', {
          // 遍历所有币种，CNY 时附带汇率
          configs: Object.entries(currencyPriceIds).map(
            ([currency, stripe_price_id]) => ({
              currency,
              stripe_price_id,
              // 人民币额外提交汇率
              ...(currency === 'CNY'
                ? { unit_price: Number(cnyUnitPrice) }
                : {}),
            }),
          ),
        }),
      );

      const results = await Promise.all(requestQueue);

      // 检查所有请求是否成功
      const errorResults = results.filter((res) => !res.data.success);
      if (errorResults.length > 0) {
        errorResults.forEach((res) => {
          showError(res.data.message);
        });
      } else {
        showSuccess(t('更新成功'));
        // 更新本地存储的原始值
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
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={t('Stripe 设置')}>
          <Text>
            Stripe 密钥、Webhook 等设置请
            <a
              href='https://dashboard.stripe.com/developers'
              target='_blank'
              rel='noreferrer'
            >
              点击此处
            </a>
            进行设置，最好先在
            <a
              href='https://dashboard.stripe.com/test/developers'
              target='_blank'
              rel='noreferrer'
            >
              测试环境
            </a>
            进行测试。
            <br />
          </Text>
          <Banner
            type='info'
            description={`Webhook 填：${props.options.ServerAddress ? removeTrailingSlash(props.options.ServerAddress) : t('网站地址')}/api/stripe/webhook`}
          />
          <Banner
            type='warning'
            description={`需要包含事件：checkout.session.completed 和 checkout.session.expired`}
          />
          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='StripeApiSecret'
                label={t('API 密钥')}
                placeholder={t(
                  'sk_xxx 或 rk_xxx 的 Stripe 密钥，敏感信息不显示',
                )}
                type='password'
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='StripeWebhookSecret'
                label={t('Webhook 签名密钥')}
                placeholder={t('whsec_xxx 的 Webhook 签名密钥，敏感信息不显示')}
                type='password'
              />
            </Col>
          </Row>

          {/* 币种 → 商品价格 ID 映射 */}
          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}>
            {Object.entries(CURRENCY_LABELS).map(([currency]) => (
              <Col key={currency} xs={24} sm={12} md={8} lg={8} xl={8}>
                <div style={{ marginTop: 16, marginBottom: 4 }}>
                  <Text strong>
                    {`${t('商品价格 ID')} ${currency === 'USD' ? '($)' : '(¥)'}`}
                  </Text>
                </div>
                <Input
                  value={currencyPriceIds[currency]}
                  onChange={(value) =>
                    setCurrencyPriceIds((prev) => ({
                      ...prev,
                      [currency]: value,
                    }))
                  }
                  placeholder={t('price_xxx 的商品价格 ID')}
                />
              </Col>
            ))}
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <div style={{ marginBottom: 4 }}>
                <Text strong>{t('美元人民币汇率')}</Text>
              </div>
              <InputNumber
                value={cnyUnitPrice}
                min={0.000001}
                precision={6}
                step={0.01}
                onChange={(value) => setCnyUnitPrice(value)}
                placeholder={t('例如：7.25')}
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='StripeUnitPrice'
                precision={2}
                label={t('充值价格（x元/美金）')}
                placeholder={t('例如：7，就是7元/美金')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='StripeMinTopUp'
                label={t('最低充值美元数量')}
                placeholder={t('例如：2，就是最低充值2$')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Switch
                field='StripePromotionCodesEnabled'
                size='default'
                checkedText='｜'
                uncheckedText='〇'
                label={t('允许在 Stripe 支付中输入促销码')}
              />
            </Col>
          </Row>
          <Button onClick={submitStripeSetting}>{t('更新 Stripe 设置')}</Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
