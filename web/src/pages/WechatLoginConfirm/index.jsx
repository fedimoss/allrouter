import React, { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Button, Card, Spin, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';

const { Title, Paragraph } = Typography;

const WechatLoginConfirm = () => {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const [state, setState] = useState('');
  const [confirming, setConfirming] = useState(false);
  const [confirmed, setConfirmed] = useState(false);

  useEffect(() => {
    setState(String(searchParams.get('state') || '').trim());
  }, [searchParams]);

  const confirmLogin = async () => {
    if (!state || confirming || confirmed) return;
    setConfirming(true);
    try {
      const { data } = await API.post('/api/oauth/wechat/confirm', { state });
      if (!data.success) {
        throw new Error(
          data.message || t('微信登录二维码已过期，请重新获取。'),
        );
      }
      setConfirmed(true);
      showSuccess(t('已确认，请返回发起登录的页面。'));
    } catch (error) {
      showError(
        error.response?.data?.message ||
          error.message ||
          t('确认失败，请重新扫码。'),
      );
    } finally {
      setConfirming(false);
    }
  };

  if (!state) {
    return (
      <div className='flex min-h-screen items-center justify-center p-6'>
        <Card style={{ maxWidth: 480, width: '100%' }}>
          <Title heading={5}>{t('微信登录二维码已过期，请重新获取。')}</Title>
        </Card>
      </div>
    );
  }

  return (
    <div className='flex min-h-screen items-center justify-center p-6'>
      <Card style={{ maxWidth: 480, width: '100%', textAlign: 'center' }}>
        <Title heading={4}>{t('微信扫码登录')}</Title>
        <Paragraph>
          {confirmed
            ? t('已确认，请返回发起登录的页面。')
            : t('请确认使用当前微信账号登录此设备。')}
        </Paragraph>
        {!confirmed && (
          <Button
            theme='solid'
            type='primary'
            loading={confirming}
            onClick={confirmLogin}
          >
            {t('确认在此设备登录')}
          </Button>
        )}
        {confirming && <Spin style={{ marginTop: 16 }} />}
      </Card>
    </div>
  );
};

export default WechatLoginConfirm;
