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

/*
 * 微信扫码体验页（普通链接二维码 + 网页授权 snsapi_base）
 *
 * 流程：
 *  1. 用户用微信扫“普通链接二维码”，微信内置浏览器直接打开本页；
 *  2. 本页发现 URL 没有 code，就先向后端取授权链接，再跳转到微信授权页；
 *  3. snsapi_base 为静默授权，用户无感知，微信回调本页并带上 code；
 *  4. 本页拿 code 调后端 /api/wechat/web_auth/token 换 openid 并取展示信息；
 *  5. 渲染展示内容。
 *
 * 说明：目前是体验流程，展示内容固定；后端接入真实业务后本页无需改动。
 */

import React, { useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Card, Spin, Typography } from '@douyinfe/semi-ui';
import { API } from '../../helpers';

const { Title, Paragraph, Text } = Typography;

const WechatScan = () => {
  const [searchParams] = useSearchParams();
  const [status, setStatus] = useState('loading'); // loading | done | error
  const [info, setInfo] = useState(null);
  const [errorMsg, setErrorMsg] = useState('');
  // 防止 React 18 StrictMode 下重复执行授权跳转/换取。
  const started = useRef(false);

  useEffect(() => {
    if (started.current) {
      return;
    }
    started.current = true;

    const code = searchParams.get('code');
    if (code) {
      exchangeCode(code);
    } else {
      redirectToAuthorize();
    }
    // 仅在首次挂载时执行，依赖项留空。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 没有 code 时，先向后端要授权链接，再跳转到微信授权页。
  const redirectToAuthorize = async () => {
    try {
      // 回调地址用当前页面的完整地址（去掉已有 query），必须是已配置的网页授权域名下。
      const redirectUri = window.location.origin + window.location.pathname;
      const res = await API.get('/api/wechat/web_auth/config', {
        params: { redirect_uri: redirectUri },
      });
      const data = res?.data?.data;
      if (!res?.data?.success || !data?.authorize_url) {
        throw new Error(res?.data?.message || '获取授权链接失败');
      }
      window.location.replace(data.authorize_url);
    } catch (error) {
      setStatus('error');
      setErrorMsg(error?.message || '获取授权链接失败');
    }
  };

  // 拿到 code 后，交给后端换 openid 并取要展示的信息。
  const exchangeCode = async (code) => {
    try {
      const res = await API.get('/api/wechat/web_auth/token', {
        params: { code },
      });
      const payload = res?.data?.data;
      if (!res?.data?.success) {
        throw new Error(res?.data?.message || '换取用户信息失败');
      }
      setInfo(payload);
      setStatus('done');
    } catch (error) {
      setStatus('error');
      setErrorMsg(error?.message || '换取用户信息失败');
    }
  };

  if (status === 'loading') {
    return (
      <div className='flex justify-center items-center h-screen p-8'>
        <Spin size='large' />
      </div>
    );
  }

  if (status === 'error') {
    return (
      <div className='flex justify-center items-center h-screen p-8'>
        <Card style={{ maxWidth: 520, width: '100%' }}>
          <Title heading={5}>扫码服务暂时不可用</Title>
          <Paragraph type='danger'>{errorMsg}</Paragraph>
        </Card>
      </div>
    );
  }

  return (
    <div className='flex justify-center items-center h-screen p-8'>
      <Card style={{ maxWidth: 560, width: '100%' }}>
        <Title heading={4} style={{ marginBottom: 16 }}>
          欢迎
        </Title>
        <Paragraph>
          <Text strong>Base URL: </Text>
          {info?.base_url}
        </Paragraph>
        <Paragraph>
          <Text strong>API 密钥: </Text>
          {info?.api_key}
        </Paragraph>
        <Paragraph type='tertiary' style={{ marginTop: 16 }}>
          openid: {info?.openid}
        </Paragraph>
        {info?.found ? (
          <Paragraph type='tertiary'>
            已绑定用户：{info?.display_name || info?.username}（ID {info?.user_id}）
          </Paragraph>
        ) : (
          <Paragraph type='tertiary'>该微信尚未绑定站内账号</Paragraph>
        )}
      </Card>
    </div>
  );
};

export default WechatScan;
