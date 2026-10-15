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
 * 微信扫码结果页（普通链接二维码 + 网页授权 snsapi_base）
 *
 * 流程：扫码打开本页 → 无 code 由后端 302 跳微信授权（本页此时不加载）→
 * 微信回调带 code → 本页加载并调后端 /api/wechat/web_auth/token →
 * 后端按站点换 openid、按需建号并保证存在 default 令牌 → 本页渲染展示。
 *
 * 展示内容：
 *   - Base URL：按各 SDK 的规则拼接。OpenAI 系列含 /v1，Anthropic 不含 /v1
 *     （Anthropic 官方 SDK 会自行补 /v1/messages，base_url 填带 /v1 会多出一层）。
 *   - API 密钥：后端返回的真实令牌 key。
 *   - 顶部按协议分段（OpenAI / Responses / Anthropic）说明用法，使用按钮式单选组，
 *     整体灰底、选中项白底（与配置页的 API 分段样式一致）；
 *     实际这些协议共用同一个 Base URL，因此不重复展示多份地址。
 *   - 仅新建账号时额外展示一次账号密码（已有账号只展示协议区块）。
 */

import React, { useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Button, Card, RadioGroup, Spin, Toast, Typography } from '@douyinfe/semi-ui';
import { IconCopy } from '@douyinfe/semi-icons';
import { API } from '../../helpers';

const { Title, Paragraph, Text } = Typography;

// 按协议分段的展示文案。
//
// baseUrlSuffix：拼在站点根地址之后的部分。不同 SDK 的 base_url 规则不同：
//   - OpenAI / Responses 用 openai SDK 规范，base_url 需带 /v1（SDK 自己补 /chat/completions 等）；
//   - Anthropic 官方 SDK 默认 base_url 为 https://api.anthropic.com（不含 /v1），
//     由 SDK 自行补 /v1/messages，因此这里不能再带 /v1。
const API_PROTOCOLS = [
  {
    key: 'openai',
    label: 'OpenAI',
    desc: 'Chat Completions，多数工具使用的 API。任何可设置 OpenAI base URL 的工具都能用。',
    hint: '填入 OPENAI_BASE_URL。',
    baseUrlSuffix: '/v1',
  },
  {
    key: 'responses',
    label: 'Responses',
    desc: 'OpenAI 较新的 API: 推理、内置工具项、加密推理。Codex 使用它。',
    hint: '填入 OPENAI_BASE_URL。',
    baseUrlSuffix: '/v1',
  },
  {
    key: 'anthropic',
    label: 'Anthropic',
    desc: 'Messages API。 Claude Code 读取 ANTHROPIC_AUTH_TOKEN 而非密钥。',
    hint: '填入 ANTHROPIC_BASE_URL。',
    baseUrlSuffix: '',
  },
];

// 复制到剪贴板：优先用 Clipboard API，不可用时回退到临时 textarea。
const copyToClipboard = async (text, label) => {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
    } else {
      const textarea = document.createElement('textarea');
      textarea.value = text;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
    }
    Toast.success(`${label}已复制`);
  } catch (error) {
    Toast.error('复制失败，请手动选择复制');
  }
};

// 带复制按钮的取值展示块。
const CopyableValue = ({ label, value }) => (
  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
    <code
      style={{
        background: '#f3f4f6',
        borderRadius: 6,
        padding: '6px 10px',
        wordBreak: 'break-all',
      }}
    >
      {value}
    </code>
    <Button
      size='small'
      icon={<IconCopy />}
      onClick={() => copyToClipboard(value, label)}
      aria-label={`复制${label}`}
    />
  </div>
);

const WechatScan = () => {
  const [searchParams] = useSearchParams();
  const [status, setStatus] = useState('loading'); // loading | done | error
  const [loadingText, setLoadingText] = useState('正在加载，请稍候…');
  const [info, setInfo] = useState(null);
  const [errorMsg, setErrorMsg] = useState('');
  // 当前选中的 API 协议（按钮式单选，控制下方说明与地址区块）。
  const [protocolKey, setProtocolKey] = useState(API_PROTOCOLS[0].key);
  // 防止 React 18 StrictMode 下重复请求。
  const started = useRef(false);

  useEffect(() => {
    if (started.current) {
      return;
    }
    started.current = true;

    // 调试模式（?debug=1）：不跳授权、不取数，直接用演示数据渲染，方便在浏览器里调样式。
    if (searchParams.get('debug') != null) {
      setInfo({
        openid: 'debug_openid_demo',
        found: false,
        created: true,
        site_url: window.location.origin,
        api_key: 'sk-debug-demo-key-1234567890',
        username: 'wx_debug_demo',
        password: 'debug_password_demo',
        user_id: 0,
        display_name: 'wx_debug_demo',
        email: '',
      });
      setStatus('done');
      return;
    }

    const code = searchParams.get('code');
    if (code) {
      exchangeCode(code);
    } else {
      // 正常情况下无 code 会由后端 302 拦下，走不到这里；
      // 兜底提示，避免用户看到空白页。
      setStatus('error');
      setErrorMsg('缺少授权参数，请重新扫码进入。');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 拿到 code 后，交给后端换 openid、按需建号并取展示信息。
  const exchangeCode = async (code) => {
    setLoadingText('正在获取你的账号信息，请稍候…');
    try {
      const res = await API.get('/api/wechat/web_auth/token', {
        params: { code },
      });
      const payload = res?.data?.data;
      if (!res?.data?.success) {
        throw new Error(res?.data?.message || '获取账号信息失败');
      }
      setInfo(payload);
      setStatus('done');
    } catch (error) {
      setStatus('error');
      setErrorMsg(error?.message || '获取账号信息失败');
    }
  };

  if (status === 'loading') {
    return (
      <div className='flex flex-col justify-center items-center h-screen p-8 gap-4'>
        <Spin size='large' />
        <Text type='tertiary'>{loadingText}</Text>
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

  const activeProtocol =
    API_PROTOCOLS.find((item) => item.key === protocolKey) || API_PROTOCOLS[0];
  // 各协议实际的 Base URL：站点根地址 + 该协议后缀（OpenAI 系列 /v1，Anthropic 为空）。
  const activeBaseUrl = `${info?.site_url || ''}${activeProtocol.baseUrlSuffix}`;

  return (
    <div className='flex justify-center items-start min-h-screen p-4 sm:p-8'>
      <Card style={{ maxWidth: 720, width: '100%' }}>
        {/* API 分段：按钮式单选组，整体灰底、选中项白底 */}
        <div className='flex gap-3' style={{ alignItems: 'center' }}>
          <span style={{ flexShrink: 0, color: '#6b7280' }}>API</span>
          <RadioGroup
            className='wechat-scan-api-switch'
            type='button'
            buttonSize='middle'
            value={protocolKey}
            onChange={(e) => setProtocolKey(e.target.value)}
            options={API_PROTOCOLS.map((protocol) => ({
              label: protocol.label,
              value: protocol.key,
            }))}
          />
        </div>

        <Paragraph style={{ marginTop: 12 }}>{activeProtocol.desc}</Paragraph>
        <div className='flex flex-col gap-4' style={{ marginTop: 16 }}>
          <div>
            <div className='flex gap-3' style={{ alignItems: 'center' }}>
              <span style={{ width: 76, flexShrink: 0, color: '#6b7280' }}>Base URL</span>
              <CopyableValue label='Base URL' value={activeBaseUrl} />
            </div>
            <div style={{ marginLeft: 88, marginTop: 4 }}>
              <Text type='tertiary'>{activeProtocol.hint}</Text>
            </div>
          </div>
          <div className='flex gap-3' style={{ alignItems: 'center' }}>
            <span style={{ width: 76, flexShrink: 0, color: '#6b7280' }}>API 密钥</span>
            <CopyableValue label='API 密钥' value={info?.api_key} />
          </div>
        </div>

        {info?.created ? (
          <div
            style={{
              marginTop: 20,
              padding: 12,
              borderRadius: 8,
              background: '#fffbeb',
              border: '1px solid #fde68a',
            }}
          >
            <Text strong>已为你创建新账号，请立即保存以下信息：</Text>
            <div className='flex flex-col gap-2' style={{ marginTop: 8 }}>
              <div className='flex gap-3' style={{ alignItems: 'center' }}>
                <span style={{ width: 76, flexShrink: 0, color: '#6b7280' }}>账号</span>
                <CopyableValue label='账号' value={info?.username} />
              </div>
              <div className='flex gap-3' style={{ alignItems: 'center' }}>
                <span style={{ width: 76, flexShrink: 0, color: '#6b7280' }}>密码</span>
                <CopyableValue label='密码' value={info?.password} />
              </div>
            </div>
            <Paragraph type='tertiary' style={{ marginTop: 8 }}>
              密码仅在本次显示一次，关闭后无法再次查看，请立即自行保存。
              <span style={{ color: 'var(--semi-color-warning)', fontWeight: 600 }}>
                后续可使用微信扫码直接登录，该功能开发中，敬请期待。
              </span>
            </Paragraph>
          </div>
        ) : null}
      </Card>
    </div>
  );
};

export default WechatScan;
