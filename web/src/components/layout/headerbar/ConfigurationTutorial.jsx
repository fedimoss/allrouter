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

import React, { useState, useCallback } from 'react';
import { SideSheet,Button } from '@douyinfe/semi-ui';
import {IconBookStroked} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { Copy, Check, Download, X } from 'lucide-react';
import { useIsMobile } from '../../../hooks/common/useIsMobile';

const TAB_ITEMS = [
  {
    key: 'openai',
    label: 'OpenAI',
    title: 'OpenAI 兼容接入',
    desc: 'default 分组 API Key + Base URL，适合 SDK、客户端和通用工具链快速接入。',
  },
  {
    key: 'claude',
    label: 'Claude Code',
    title: 'Claude Code 接入',
    desc: 'Anthropic Claude Code CLI 专用配置，使用 ANTHROPIC 协议直接接入。',
  },
  {
    key: 'codex',
    label: 'Codex CLI',
    title: 'Codex CLI 接入',
    desc: 'OpenAI Codex CLI 配置，使用 OPENAI 兼容协议快速接入。',
  },
];

const PANEL_TABS = [
  { key: 'openai', label: 'OpenAI 配置教程' },
  { key: 'claude', label: 'Claude Code 使用教程' },
  { key: 'codex', label: 'Codex CLI 使用教程' },
];

const OS_TABS = [
  { key: 'macos', label: 'MacOS' },
  { key: 'linux', label: 'Linux' },
  { key: 'windows', label: 'Windows' },
];


const MASKED_KEY = 'sk-*************';

// ──────────── Helpers ────────────

function useCopy() {
  const [copied, setCopied] = useState(null);
  const handleCopy = useCallback((text, field) => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(field);
      setTimeout(() => setCopied(null), 2000);
    });
  }, []);
  return [copied, handleCopy];
}

function getBaseUrl() {
  return typeof window !== 'undefined' ? window.location.origin : '';
}

// ──────────── Shared UI Parts ────────────

function SectionTitle({ children }) {
  return <h3 className="text-sm font-bold text-slate-800 dark:text-slate-100 mb-3">{children}</h3>;
}

function StepItem({ number, children, active }) {
  const isFirst = active !== undefined ? active : number === 1;
  return (
    <div className="flex gap-3">
      <span
        className="flex shrink-0 h-6 w-6 items-center justify-center rounded-full text-xs font-bold"
        style={isFirst ? { background: 'var(--theme-gradient)', color: '#fff' } : { background: '#EEF6FB', color: '#64748B' }}
      >
        {number}
      </span>
      <div className="text-sm text-slate-600 dark:text-slate-300 leading-relaxed">{children}</div>
    </div>
  );
}

function NoteItem({ children }) {
  return (
    <li className="flex gap-2 text-sm text-slate-500 dark:text-slate-400">
      <span className="text-slate-300 dark:text-slate-600 shrink-0">•</span>
      <span>{children}</span>
    </li>
  );
}

function InlineCode({ children }) {
  return (
    <code className="px-1.5 py-0.5 rounded bg-slate-100 dark:bg-slate-700 text-xs">{children}</code>
  );
}

function CodeBlockInner({ code, label, field, copied, onCopy }) {
  const isCopied = copied === field;
  return (
    <div>
      {label && <p className="text-xs text-slate-500 dark:text-slate-400 mb-2">{label}</p>}
      <div className="relative group">
        <pre className="bg-[#1e293b] dark:bg-slate-950 text-slate-100 rounded-xl p-3 sm:p-4 text-xs sm:text-sm overflow-x-auto leading-relaxed border border-slate-700/50 whitespace-pre-wrap break-all">
          <code>{code}</code>
        </pre>
        <button
          onClick={() => onCopy(code, field)}
          className="absolute top-2 right-2 p-1.5 rounded-lg bg-white/10 hover:bg-white/20 transition text-slate-400 hover:text-white"
          type="button"
        >
          {isCopied ? <Check className="h-3.5 w-3.5 text-emerald-400" /> : <Copy className="h-3.5 w-3.5" />}
        </button>
      </div>
    </div>
  );
}

function OsTabBar({ active, onChange, t }) {
  return (
    <div className="flex gap-1 mb-4 bg-slate-100 dark:bg-slate-800 rounded-lg p-1 w-fit max-w-full overflow-x-auto">
      {OS_TABS.map((tab) => (
        <button
          key={tab.key}
          onClick={() => onChange(tab.key)}
          className={`shrink-0 px-3 py-1.5 text-xs font-medium rounded-md transition ${active === tab.key
              ? 'bg-white dark:bg-slate-700 text-slate-800 dark:text-white shadow-sm'
              : 'text-slate-500 dark:text-slate-400 hover:text-slate-700'
            }`}
          type="button"
        >
          {t ? t(tab.label) : tab.label}
        </button>
      ))}
    </div>
  );
}

// ──────────── Panel: OpenAI ────────────

function OpenAIPanel() {
  const { t } = useTranslation();
  const [copied, onCopy] = useCopy();
  const baseUrl = getBaseUrl();

  return (
    <div className="space-y-4 sm:space-y-6">
      {/* Warning notice */}
      <div
        className="flex items-start gap-2 rounded-xl px-3 py-3 border sm:gap-3 sm:px-4"
        style={{ background: 'rgba(245, 158, 11, 0.08)', borderColor: 'rgba(245, 158, 11, 0.25)' }}
      >
        <svg className="h-5 w-5 shrink-0 mt-0.5 text-amber-500" viewBox="0 0 20 20" fill="currentColor">
          <path fillRule="evenodd" d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.168 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 6a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 6zm0 9a1 1 0 100-2 1 1 0 000 2z" clipRule="evenodd" />
        </svg>
        <p className="text-sm text-amber-700 dark:text-amber-400 leading-relaxed">
          {t('注意：仅')} <InlineCode>default</InlineCode> {t('分组的 API Key 支持 OpenAI 格式调用')}
        </p>
      </div>

      {/* 基础配置 */}
      <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(148,163,184,0.1)] dark:border-slate-700 dark:bg-slate-800/60 dark:shadow-[0_4px_16px_rgba(0,0,0,0.25)] p-4 sm:p-5">
        <SectionTitle>{t('基础配置')}</SectionTitle>
        <div className="space-y-4">
          <div>
            <p className="text-xs text-slate-500 dark:text-slate-400 !mb-2">{t('站点地址')}</p>
            <div className="flex items-center gap-2 px-3 py-3 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700 sm:px-4 sm:gap-3">
              <code className="text-xs sm:text-sm text-slate-800 dark:text-slate-200 font-mono flex-1 min-w-0 break-all">{baseUrl}</code>
              <button
                onClick={() => onCopy(baseUrl, 'openai-baseurl')}
                className="shrink-0 p-1.5 rounded-lg hover:bg-slate-200 dark:hover:bg-slate-700 transition"
                type="button"
              >
                {copied === 'openai-baseurl' ? (
                  <Check className="h-3.5 w-3.5 text-emerald-500" />
                ) : (
                  <Copy className="h-3.5 w-3.5 text-slate-400" />
                )}
              </button>
            </div>
          </div>

          <div>
            <p className="text-xs text-slate-500 dark:text-slate-400 !mb-2">{t('配置示例')}</p>
            <div className="relative group">
              <pre className="bg-[#1e293b] dark:bg-slate-950 text-slate-100 rounded-xl p-3 sm:p-4 text-xs sm:text-sm overflow-x-auto leading-relaxed border border-slate-700/50 whitespace-pre-wrap break-all">
                <code>{`OPENAI_API_KEY=${MASKED_KEY}\nOPENAI_BASE_URL=${baseUrl}/v1`}</code>
              </pre>
              <button
                onClick={() => onCopy(`OPENAI_API_KEY=${MASKED_KEY}\nOPENAI_BASE_URL=${baseUrl}/v1`, 'openai-config-example')}
                className="absolute top-2 right-2 p-1.5 rounded-lg bg-white/10 hover:bg-white/20 transition text-slate-400 hover:text-white"
                type="button"
              >
                {copied === 'openai-config-example' ? <Check className="h-3.5 w-3.5 text-emerald-400" /> : <Copy className="h-3.5 w-3.5" />}
              </button>
            </div>
          </div>
        </div>
      </section>

      {/* 使用说明 */}
      <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(148,163,184,0.1)] dark:border-slate-700 dark:bg-slate-800/60 dark:shadow-[0_4px_16px_rgba(0,0,0,0.25)] p-4 sm:p-5">
        <SectionTitle>{t('使用说明')}</SectionTitle>
        <div className="space-y-3">
          <StepItem number={1}>{t('登录后进入控制台，创建一个新的 API Key')}</StepItem>
          <StepItem number={2}>{t('确保 API Key 属于')} <InlineCode>default</InlineCode> {t('分组')}</StepItem>
          <StepItem number={3}>{t('将 Base URL 设置为上述站点地址')}</StepItem>
          <StepItem number={4}>{t('使用创建的 API Key 作为')} <InlineCode>OPENAI_API_KEY</InlineCode></StepItem>
        </div>
      </section>
    </div>
  );
}

// ──────────── Panel: Claude Code ────────────

function ClaudeCodePanel() {
  const { t } = useTranslation();
  const [copied, onCopy] = useCopy();
  const [osTab, setOsTab] = useState('macos');
  const baseUrl = getBaseUrl();

  const tempCode = {
    macos: `export ANTHROPIC_BASE_URL=${baseUrl}\nexport ANTHROPIC_AUTH_TOKEN=${MASKED_KEY}`,
    linux: `export ANTHROPIC_BASE_URL=${baseUrl}\nexport ANTHROPIC_AUTH_TOKEN=${MASKED_KEY}`,
    windows: `set ANTHROPIC_BASE_URL=${baseUrl}\nset ANTHROPIC_AUTH_TOKEN=${MASKED_KEY}`,
  };

  const permCode = {
    macos: `# ${t('添加到')} ~/.zshrc ${t('或')} ~/.bash_profile\necho 'export ANTHROPIC_BASE_URL=${baseUrl}' >> ~/.zshrc\necho 'export ANTHROPIC_AUTH_TOKEN=${MASKED_KEY}' >> ~/.zshrc\nsource ~/.zshrc`,
    linux: `# ${t('添加到')} ~/.bashrc\necho 'export ANTHROPIC_BASE_URL=${baseUrl}' >> ~/.bashrc\necho 'export ANTHROPIC_AUTH_TOKEN=${MASKED_KEY}' >> ~/.bashrc\nsource ~/.bashrc`,
    windows: `setx ANTHROPIC_BASE_URL "${baseUrl}"\nsetx ANTHROPIC_AUTH_TOKEN "${MASKED_KEY}"`,
  };

  return (
    <div className="space-y-4 sm:space-y-6">
      <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(148,163,184,0.1)] dark:border-slate-700 dark:bg-slate-800/60 dark:shadow-[0_4px_16px_rgba(0,0,0,0.25)] p-4 sm:p-5">
        <SectionTitle>{t('使用前准备')}</SectionTitle>
        <div className="space-y-3">
          <StepItem number={1}>
            {t('请确保在')} <InlineCode>claude code</InlineCode> {t('专用分组创建 API Key（该分组仅用于 Claude Code CLI）')}
          </StepItem>
          <div className="flex gap-3">
            <span
              className="flex shrink-0 h-6 w-6 items-center justify-center rounded-full text-xs font-bold"
              style={{ background: '#EEF6FB', color: '#64748B' }}
            >
              2
            </span>
            <div>
              <p className="text-sm text-slate-600 dark:text-slate-300 leading-relaxed">
                {t('推荐使用 cc-switch 工具快速切换环境')}
              </p>
              <button
                className="mt-2 inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-100 dark:bg-slate-700 text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-600 transition"
                type="button"
                onClick={() => window.open('https://github.com/farion1231/cc-switch/releases', '_blank')}
              >
                <Download className="h-3.5 w-3.5" />
                {t('下载 cc-switch')}
              </button>
              <p className="mt-2 text-xs text-slate-400 dark:text-slate-500 leading-relaxed">
                {t('cc-switch 是一个图形化工具，可以方便地管理多个 Claude Code 配置')}
              </p>
            </div>
          </div>
        </div>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(148,163,184,0.1)] dark:border-slate-700 dark:bg-slate-800/60 dark:shadow-[0_4px_16px_rgba(0,0,0,0.25)] p-4 sm:p-5">
        <SectionTitle>{t('终端配置指南')}</SectionTitle>
        <OsTabBar active={osTab} onChange={setOsTab} t={t} />
        <div className="space-y-4">
          <CodeBlockInner
            label={t('临时设置（当前终端会话有效）')}
            code={tempCode[osTab]}
            field={`claude-temp-${osTab}`}
            copied={copied}
            onCopy={onCopy}
          />
          <CodeBlockInner
            label={t('永久设置（需要重启终端生效）')}
            code={permCode[osTab]}
            field={`claude-perm-${osTab}`}
            copied={copied}
            onCopy={onCopy}
          />
        </div>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(148,163,184,0.1)] dark:border-slate-700 dark:bg-slate-800/60 dark:shadow-[0_4px_16px_rgba(0,0,0,0.25)] p-4 sm:p-5">
        <SectionTitle>{t('注意事项')}</SectionTitle>
        <ul className="space-y-2">
          <NoteItem>
            {t('请将')} <InlineCode>{MASKED_KEY}</InlineCode> {t('替换为您在 claude code 分组创建的实际 API Key')}
          </NoteItem>
          <NoteItem>{t('永久设置后需要重新打开终端或执行 source 命令才能生效')}</NoteItem>
          <NoteItem>{t('Windows 用户使用 setx 命令后需要重新打开命令提示符')}</NoteItem>
        </ul>
      </section>
    </div>
  );
}

// ──────────── Panel: Codex CLI ────────────

function CodexCLIPanel() {
  const { t } = useTranslation();
  const [copied, onCopy] = useCopy();
  const [osTab, setOsTab] = useState('macos');
  const baseUrl = getBaseUrl();

  const configToml = `# ~/.codex/config.toml
model_provider = "packycode"
model = "gpt-5.2"
model_reasoning_effort = "high"
disable_response_storage = true
preferred_auth_method = "apikey"

[model_providers.packycode]
name = "packycode"
base_url = "${baseUrl}/v1"
wire_api = "responses"
requires_openai_auth = true
env_key = "PACKYCODE_API_KEY"`;

  const envTempCode = {
    macos: `export PACKYCODE_API_KEY=${MASKED_KEY}`,
    linux: `export PACKYCODE_API_KEY=${MASKED_KEY}`,
    windows: `set PACKYCODE_API_KEY=${MASKED_KEY}`,
  };

  const envPermCode = {
    macos: `# ${t('添加到')} ~/.zshrc ${t('或')} ~/.bash_profile\necho 'export PACKYCODE_API_KEY=${MASKED_KEY}' >> ~/.zshrc\nsource ~/.zshrc`,
    linux: `# ${t('添加到')} ~/.bashrc\necho 'export PACKYCODE_API_KEY=${MASKED_KEY}' >> ~/.bashrc\nsource ~/.bashrc`,
    windows: `setx PACKYCODE_API_KEY "${MASKED_KEY}"`,
  };

  return (
    <div className="space-y-4 sm:space-y-6">
      {/* Info notice */}
      <div
        className="flex items-start gap-2 rounded-xl px-3 py-3 border sm:gap-3 sm:px-4"
        style={{ background: 'rgba(14, 165, 233, 0.06)', borderColor: 'rgba(14, 165, 233, 0.2)' }}
      >
        <svg className="h-5 w-5 shrink-0 mt-0.5 text-sky-500" viewBox="0 0 20 20" fill="currentColor">
          <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a.75.75 0 000 1.5h.253a.25.25 0 01.244.304l-.459 2.066A1.75 1.75 0 0010.747 15H11a.75.75 0 000-1.5h-.253a.25.25 0 01-.244-.304l.459-2.066A1.75 1.75 0 009.253 9H9z" clipRule="evenodd" />
        </svg>
        <p className="text-sm text-sky-700 dark:text-sky-400 leading-relaxed">
          {t('OpenAI Codex CLI 是 OpenAI 官方推出的本地编辑代理工具，支持通过自定义代理使用，推荐使用 cc-switch 进行管理')}
        </p>
      </div>

      {/* 安装 Codex CLI */}
      <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(148,163,184,0.1)] dark:border-slate-700 dark:bg-slate-800/60 dark:shadow-[0_4px_16px_rgba(0,0,0,0.25)] p-4 sm:p-5">
        <SectionTitle>{t('安装 Codex CLI')}</SectionTitle>
        <div className="space-y-4">
          <div>
            <p className="text-xs text-slate-500 dark:text-slate-400 !mb-2">{t('使用 npm 安装')}</p>
            <CodeBlockInner
              code="npm install -g @openai/codex"
              field="codex-install-npm"
              copied={copied}
              onCopy={onCopy}
            />
          </div>
          <div>
            <p className="text-xs text-slate-500 dark:text-slate-400 !mb-2">{t('使用 Homebrew 安装 (macOS)')}</p>
            <CodeBlockInner
              code="brew install --cask codex"
              field="codex-install-brew"
              copied={copied}
              onCopy={onCopy}
            />
          </div>
          <div>
            <p className="text-xs text-slate-500 dark:text-slate-400 !mb-2">{t('或从 GitHub 下载二进制文件')}</p>
            <button
              className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl bg-slate-100 dark:bg-slate-700 text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-600 transition"
              type="button"
              onClick={() => window.open('https://github.com/openai/codex/releases/latest')}
              target="_blank"
            >
              <Download className="h-3.5 w-3.5" />
              GitHub Releases
            </button>
          </div>
        </div>
      </section>

      {/* 配置代理 */}
      <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(148,163,184,0.1)] dark:border-slate-700 dark:bg-slate-800/60 dark:shadow-[0_4px_16px_rgba(0,0,0,0.25)] p-4 sm:p-5">
        <SectionTitle>{t('配置代理')}</SectionTitle>
        <div className="space-y-3 mb-4">
          <StepItem number={1}>
            {t('请确保在')} <InlineCode>codex</InlineCode> {t('专用分组创建 API Key（该分组仅支持 Codex CLI）')}
          </StepItem>
          <StepItem number={2}>
            {t('创建或编辑配置文件')} <InlineCode>~/.codex/config.toml</InlineCode>
          </StepItem>
        </div>
        <div>
          <p className="text-xs text-slate-500 dark:text-slate-400 !mb-2 font-medium">{t('config.toml 配置示例')}</p>
          <CodeBlockInner
            code={configToml}
            field="codex-config-toml"
            copied={copied}
            onCopy={onCopy}
          />
        </div>
      </section>

      {/* 设置环境变量 */}
      <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(148,163,184,0.1)] dark:border-slate-700 dark:bg-slate-800/60 dark:shadow-[0_4px_16px_rgba(0,0,0,0.25)] p-4 sm:p-5">
        <SectionTitle>{t('设置环境变量')}</SectionTitle>
        <OsTabBar active={osTab} onChange={setOsTab} t={t} />
        <div className="space-y-4">
          <CodeBlockInner
            label={t('临时设置（当前终端会话有效）')}
            code={envTempCode[osTab]}
            field={`codex-env-temp-${osTab}`}
            copied={copied}
            onCopy={onCopy}
          />
          <CodeBlockInner
            label={t('永久设置（需要重启终端生效）')}
            code={envPermCode[osTab]}
            field={`codex-env-perm-${osTab}`}
            copied={copied}
            onCopy={onCopy}
          />
        </div>
      </section>

      {/* 启动使用 */}
      <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(148,163,184,0.1)] dark:border-slate-700 dark:bg-slate-800/60 dark:shadow-[0_4px_16px_rgba(0,0,0,0.25)] p-4 sm:p-5">
        <SectionTitle>{t('启动使用')}</SectionTitle>
        <div className="space-y-3 mb-4">
          <p className="text-sm text-slate-600 dark:text-slate-300 leading-relaxed">
            {t('配置完成后，在终端运行 codex 即可启动')}
          </p>
          <p className="text-sm text-slate-600 dark:text-slate-300 leading-relaxed">
            {t('首次运行会提示登录，选择')} <InlineCode>Sign in with API key</InlineCode> {t('使用 API Key 登录')}
          </p>
        </div>
        <CodeBlockInner
          code="codex"
          field="codex-launch"
          copied={copied}
          onCopy={onCopy}
        />
      </section>

      {/* 注意事项 */}
      <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(148,163,184,0.1)] dark:border-slate-700 dark:bg-slate-800/60 dark:shadow-[0_4px_16px_rgba(0,0,0,0.25)] p-4 sm:p-5">
        <SectionTitle>{t('注意事项')}</SectionTitle>
        <ul className="space-y-2 mb-4">
          <NoteItem>
            {t('请将')} <InlineCode>your-api-key</InlineCode> {t('替换为您在 codex 分组创建的实际 API Key')}
          </NoteItem>
          <NoteItem>{t('Codex CLI 支持多种模型，可在 config.toml 中修改 model 参数')}</NoteItem>
          <NoteItem>{t('更多配置选项请参考官方文档')}</NoteItem>
        </ul>
        <button
          className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl bg-slate-100 dark:bg-slate-700 text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-600 transition"
          type="button"
          onClick={() => window.open('https://developers.openai.com/codex')}
          rel="_blank"
          target="_blank"
        >
          {t('查看文档')}
        </button>
      </section>
    </div>
  );
}

// ──────────── Main Component ────────────

export default function ConfigurationTutorial({ className }) {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [activeTab, setActiveTab] = useState('openai');
  const [panelOpen, setPanelOpen] = useState(false);
  const [panelTab, setPanelTab] = useState('openai');

  const openPanel = () => {
    setPanelTab(activeTab);
    setPanelOpen(true);
  };

  const renderPanelContent = () => {
    switch (panelTab) {
      case 'openai':
        return <OpenAIPanel />;
      case 'claude':
        return <ClaudeCodePanel />;
      case 'codex':
        return <CodexCLIPanel />;
      default:
        return null;
    }
  };

  return (
    <>
      <Button theme='outline' type='tertiary' onClick={openPanel}>
        <IconBookStroked /><span className="hidden sm:inline">&nbsp;{t('配置教程')}</span>
      </Button>

      {/* ── Right Slide-in Panel ── */}
      <SideSheet
        visible={panelOpen}
        onCancel={() => setPanelOpen(false)}
        placement="right"
        width={isMobile ? '100%' : 600}
        title={null}
        headerStyle={{ display: 'none' }}
        bodyStyle={{ padding: 0, height: '100%' }}
        maskClosable
        closeOnEsc
      >
        <div className="h-full flex flex-col">
          {/* Panel Nav */}
          <div className="shrink-0 border-b border-slate-200 dark:border-slate-700 px-4 py-3 sm:px-6 sm:py-4">
            <div className="flex items-center justify-between gap-2">
              <div className="flex gap-4 sm:gap-6 overflow-x-auto min-w-0 flex-1">
                {PANEL_TABS.map((tab) => (
                  <button
                    key={tab.key}
                    onClick={() => setPanelTab(tab.key)}
                    className={`relative shrink-0 pb-2 text-sm font-medium transition ${panelTab === tab.key
                        ? ''
                        : 'text-slate-400 dark:text-slate-500 hover:text-slate-600 dark:hover:text-slate-300'
                      }`}
                    style={panelTab === tab.key ? { color: 'var(--theme-primary)' } : undefined}
                    type="button"
                  >
                    {t(tab.label)}
                    {panelTab === tab.key && (
                      <span
                        className="absolute bottom-0 left-0 right-0 h-0.5 rounded-full"
                        style={{ background: 'var(--theme-gradient)' }}
                      />
                    )}
                  </button>
                ))}
              </div>
              <button
                onClick={() => setPanelOpen(false)}
                className="shrink-0 ml-2 sm:ml-4 flex h-8 w-8 items-center justify-center rounded-full text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-600 transition"
                type="button"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          </div>

          {/* Panel Body */}
          <div className="flex-1 overflow-y-auto px-4 py-4 sm:px-6 sm:py-6">
            {renderPanelContent()}
          </div>
        </div>
      </SideSheet>
    </>
  );
}
