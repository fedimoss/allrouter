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

import React, {
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';
import { ArrowRight } from 'lucide-react';
import {
  API,
  setStatusData,
  shouldShowProviderAgentPartner,
  showError,
  withBrowserBaseUrl,
} from '../../../../helpers';
import { StatusContext } from '../../../../context/Status';
import { UserContext } from '../../../../context/User';
import { useActualTheme, useSetTheme, useTheme } from '../../../../context/Theme';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import { marked } from 'marked';
import { Modal } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate } from 'react-router-dom';
import NoticeModal from '../../../../components/layout/NoticeModal';
import ThemeToggle from '../../../../components/layout/headerbar/ThemeToggle';
import LanguageSelector from '../../../../components/layout/headerbar/LanguageSelector';
import NotificationButton from '../../../../components/layout/headerbar/NotificationButton';
import UserArea from '../../../../components/layout/headerbar/UserArea';
import FloatingSupport from '../../../../components/common/FloatingSupport';
import MarqueeLogos from '../../../../components/common/MarqueeLogos';

import {
  getLogo,
  getQQSupport,
  getSystemName,
  getWechatSupport,
} from '../../../../helpers';

import './index.css';

import openaiLogo from '../../../../../public/logos/openai.svg';
import anthropicLogo from '../../../../../public/logos/anthropic.svg';
import geminiLogo from '../../../../../public/logos/gemini.svg';
import metaLogo from '../../../../../public/logos/meta.svg';
import deepseekLogo from '../../../../../public/logos/deepseek.svg';
import huggingFaceLogo from '../../../../../public/logos/huggingface.svg';
import kimiLogo from '../../../../../public/logos/kimi.svg';
import minimaxLogo from '../../../../../public/logos/minimax.svg';
import chatglmLogo from '../../../../../public/logos/chatglm.svg';
import doubaoLogo from '../../../../../public/logos/doubao.svg';
import grokLogo from '../../../../../public/logos/grok.svg';
import qwenLogo from '../../../../../public/logos/qwen.svg';
import refinedImg from '../../../../../public/theme/theme1/refined.png';
import newLogo from '../../../../../public/theme/theme1/new-logo.png';
import imgOne from '../../../../../public/theme/theme1/img-01.png';
import iconOne from '../../../../../public/theme/theme1/icon01.png';
import iconTwo from '../../../../../public/theme/theme1/icon02.png';
import iconThree from '../../../../../public/theme/theme1/icon03.png';
import iconFour from '../../../../../public/theme/theme1/icon04.png';
import fedimoossLogo from '../../../../../public/theme/theme2/fedimoss-logo.svg';


const logo = getLogo();
const systemName = getSystemName();

const partnerLogos = [
  { src: openaiLogo, alt: 'ChatGPT' },
  { src: anthropicLogo, alt: 'Claude' },
  { src: geminiLogo, alt: 'Gemini' },
  { src: metaLogo, alt: 'Meta' },
  { src: deepseekLogo, alt: 'DeepSeek' },
  { src: huggingFaceLogo, alt: 'Hugging Face' },
  { src: kimiLogo, alt: 'Kimi' },
  { src: minimaxLogo, alt: 'Minimax' },
  { src: chatglmLogo, alt: 'ChatGLM' },
  { src: doubaoLogo, alt: 'Doubao' },
  { src: grokLogo, alt: 'Grok' },
  { src: qwenLogo, alt: 'Qwen' },
];

const featureCards = [
  {
    imgUrl: iconOne,
    title: '多渠道比价',
    description:
      '智能分析各家模型渠道实时价格，为您锁定当前最优成本路径。',
  },
  {
    imgUrl: iconTwo,
    title: 'OpenAI 兼容接入',
    description:
      '完全兼容 OpenAI API 格式，只需更改 Base URL，零代码成本迁移。',
  },
  {
    imgUrl: iconThree,
    title: '自营品质保障',
    description:
      '官方直连渠道，高 SLA 可用性保障，无感应对上游波动。',
  },
  {
    imgUrl: iconFour,
    title: '商家生态入驻',
    description:
      '开放的市场生态，支持优质算力商家入驻，共同打造最低价集群。',
  },
];

const workflowSteps = [
  {
    number: '01',
    title: '接入请求',
    description: '统一 API 入口接收调用',
  },
  {
    number: '02',
    title: '智能评估',
    description: '比较价格、延迟与可用性',
  },
  {
    number: '03',
    title: '自动路由',
    description: '切换到最合适的模型渠道',
  },
  {
    number: '04',
    title: '持续观测',
    description: '输出成本、状态与调用质量',
  },
];

const heroStats = [
  { value: '50%+', label: '基于典型场景估算',tag:'节省' },
  { value: '48+', label: '可用模型渠道' },
  { value: '99.99%', label: '高可用路由' },
  { value: '1 API', label: '统一接入入口' },
];

const explorePills = [
  { name: '1M', desc: '长文本推理' },
  { name: 'S+H', desc: '软硬协同' },
  { name: 'Token', desc: '产品交付' },
];

const escapeHtml = (content = '') =>
  content
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');

const getStoredUser = () => {
  if (typeof window === 'undefined') {
    return null;
  }
  try {
    const raw = localStorage.getItem('user');
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
};

const Home = () => {
  const { t, i18n } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState, statusDispatch] = useContext(StatusContext);
  const actualTheme = useActualTheme();
  const navigate = useNavigate();
  const theme = useTheme();
  const setTheme = useSetTheme();
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(false);
  const [homePageContent, setHomePageContent] = useState('');
  const [noticeVisible, setNoticeVisible] = useState(false);
  const [typedCodeLines, setTypedCodeLines] = useState([]);
  const [versionLogVisible, setVersionLogVisible] = useState(false);
  const isMobile = useIsMobile();

  const docsLink = statusState?.status?.docs_link || '';
  const serverAddress = `${window.location.origin}`;
  const showDefaultHome = homePageContentLoaded && homePageContent === '';
  const docsLangPrefix = i18n.language.startsWith('zh') ? 'zh' : 'en';

  const docsHref = docsLink || withBrowserBaseUrl(`/${docsLangPrefix}/docs`);
  const apiReferenceHref = withBrowserBaseUrl(`/${docsLangPrefix}/docs/api`);
  const communityHref = withBrowserBaseUrl(
    `/${docsLangPrefix}/docs/support/community-interaction`,
  );
  const supportConfig = useMemo(() => {
    const status = statusState?.status;
    if (!status) {
      return {
        wechatQRCode: getWechatSupport(),
        qqSupport: getQQSupport(),
      };
    }
    const providerConfig = status.provider_config;
    if (providerConfig?.enabled) {
      return {
        wechatQRCode: providerConfig.wechat_support || '',
        qqSupport: providerConfig.qq_support || '',
      };
    }
    return {
      wechatQRCode: status.wechat_support || '',
      qqSupport: status.qq_support || '',
    };
  }, [statusState?.status]);
  const currentUser = userState?.user || getStoredUser();
  const isLoggedIn = Boolean(currentUser?.id);
  const isSelfUseMode = statusState?.status?.self_use_mode_enabled || false;
  const headerNavModules = useMemo(() => {
    const headerNavModulesConfig = statusState?.status?.HeaderNavModules;
    if (!headerNavModulesConfig) {
      return null;
    }
    try {
      const modules = JSON.parse(headerNavModulesConfig);
      if (typeof modules.pricing === 'boolean') {
        modules.pricing = {
          enabled: modules.pricing,
          requireAuth: false,
        };
      }
      return modules;
    } catch {
      return null;
    }
  }, [statusState?.status?.HeaderNavModules]);
  const pricingRequireAuth = useMemo(() => {
    if (headerNavModules?.pricing) {
      return typeof headerNavModules.pricing === 'object'
        ? headerNavModules.pricing.requireAuth
        : false;
    }
    return false;
  }, [headerNavModules]);
  const consoleNavTarget = isLoggedIn ? '/console' : '/login';
  const pricingNavTarget =
    !isLoggedIn && pricingRequireAuth ? '/login' : '/pricing';
  const showAgentPartnerNav = shouldShowProviderAgentPartner(
    statusState?.status,
  );
  const normalizedUserState = { user: currentUser };
  const versionLabel = statusState?.status?.version?.version || 'v2.0';

  const handleThemeToggle = useCallback(
    (newTheme) => {
      if (newTheme === 'light' || newTheme === 'dark' || newTheme === 'auto') {
        setTheme(newTheme);
      }
    },
    [setTheme],
  );

  const handleLanguageChange = useCallback(
    async (lang) => {
      i18n.changeLanguage(lang);
      if (!currentUser?.id) {
        return;
      }
      try {
        const res = await API.put('/api/user/self', { language: lang });
        if (res.data.success && currentUser?.setting) {
          const settings = JSON.parse(currentUser.setting);
          settings.language = lang;
          userDispatch({
            type: 'login',
            payload: {
              ...currentUser,
              setting: JSON.stringify(settings),
            },
          });
        }
      } catch (error) {
        console.error('Failed to save language preference:', error);
      }
    },
    [currentUser, i18n, userDispatch],
  );

  const handleNoticeOpen = useCallback(() => {
    setNoticeVisible(true);
  }, []);

  const logout = useCallback(async () => {
    await API.get('/api/user/logout');
    userDispatch({ type: 'logout' });
    localStorage.removeItem('user');
    navigate('/login');
  }, [navigate, userDispatch]);

  const displayHomePageContent = async () => {
    setHomePageContent(localStorage.getItem('home_page_content') || '');
    const res = await API.get('/api/home_page_content');
    const { success, message, data } = res.data;
    if (success) {
      let content = data;
      if (!data.startsWith('https://')) {
        content = marked.parse(data);
      }
      setHomePageContent(content);
      localStorage.setItem('home_page_content', content);

      // 如果内容是 URL，则发送主题模式
      if (data.startsWith('https://')) {
        const iframe = document.querySelector('iframe');
        if (iframe) {
          iframe.onload = () => {
            iframe.contentWindow.postMessage({ themeMode: actualTheme }, '*');
            iframe.contentWindow.postMessage({ lang: i18n.language }, '*');
          };
        }
      }
    } else {
      showError(message);
      setHomePageContent(t('加载首页内容失败...'));
    }
    setHomePageContentLoaded(true);
  };

  useEffect(() => {
    const checkNoticeAndShow = async () => {
      const lastCloseDate = localStorage.getItem('notice_close_date');
      const today = new Date().toDateString();
      if (lastCloseDate !== today) {
        try {
          const res = await API.get('/api/notice');
          const { success, data } = res.data;
          if (success && data && data.trim() !== '') {
            setNoticeVisible(true);
          }
        } catch (error) {
          console.error('获取公告失败:', error);
        }
      }
    };

    checkNoticeAndShow();
  }, []);

  useEffect(() => {
    displayHomePageContent().then();
  }, []);

  useEffect(() => {
    let cancelled = false;

    const refreshStatus = async () => {
      try {
        const res = await API.get('/api/status');
        const { success, data, message } = res.data || {};
        if (cancelled) {
          return;
        }
        if (success) {
          statusDispatch({ type: 'set', payload: data });
          setStatusData(data);
        } else if (message) {
          showError(message);
        }
      } catch (error) {
        if (!cancelled) {
          console.error('Failed to refresh status:', error);
        }
      }
    };

    refreshStatus();

    return () => {
      cancelled = true;
    };
  }, [statusDispatch]);

  useEffect(() => {
    if (!showDefaultHome) {
      return undefined;
    }

    const codeBaseUrl = `${serverAddress.replace(/\/$/, '')}/v1`;
    const codeLines = [
      {
        html: `<span class='home-c-c'># ${escapeHtml(t('安装官方 SDK'))}</span>`,
        delay: 420,
      },
      {
        html: "<span class='home-c-k'>pip</span> install openai",
        delay: 720,
      },
      { html: '', delay: 160 },
      {
        html: "<span class='home-c-k'>import</span> openai",
        delay: 620,
      },
      { html: '', delay: 140 },
      {
        html: `<span class='home-c-c'># ${escapeHtml(t('只需替换 base_url 与 api_key'))}</span>`,
        delay: 640,
      },
      { html: 'client = openai.OpenAI(', delay: 360 },
      {
        html: `    base_url=<span class='home-c-s'>'${escapeHtml(codeBaseUrl)}'</span>,`,
        delay: 360,
      },
      {
        html: "    api_key=<span class='home-c-s'>'sk-...'</span>",
        delay: 360,
      },
      { html: ')', delay: 420 },
      { html: '', delay: 160 },
      {
        html: `<span class='home-c-c'># ${escapeHtml(
          t('任意模型即开即用：gpt-4、claude-3、llama-3...'),
        )}</span>`,
        delay: 620,
      },
      { html: 'response = client.chat.completions.create(', delay: 360 },
      {
        html: "    model=<span class='home-c-s'>'claude-3-opus'</span>,",
        delay: 360,
      },
      {
        html: "    messages=[{<span class='home-c-s'>'role'</span>: <span class='home-c-s'>'user'</span>, <span class='home-c-s'>'content'</span>: <span class='home-c-s'>'Hello!'</span>}]",
        delay: 360,
      },
      { html: ')', delay: 260 },
      { html: "<span class='home-cursor'></span>", delay: 0 },
    ];

    let timeoutId;
    let lineIndex = 0;
    let cancelled = false;

    setTypedCodeLines([]);

    const printLine = () => {
      if (cancelled || lineIndex >= codeLines.length) {
        return;
      }

      const line = codeLines[lineIndex];
      setTypedCodeLines((prev) => [...prev, line.html]);
      lineIndex += 1;
      timeoutId = window.setTimeout(printLine, line.delay);
    };

    timeoutId = window.setTimeout(printLine, 500);

    return () => {
      cancelled = true;
      window.clearTimeout(timeoutId);
    };
  }, [showDefaultHome, serverAddress, i18n.language, t]);

  return (
    <div className={showDefaultHome ? 'home-app' : 'w-full overflow-x-hidden'}>
      <NoticeModal
        visible={noticeVisible}
        onClose={() => setNoticeVisible(false)}
        isMobile={isMobile}
      />
      <FloatingSupport
        wechatQRCode={supportConfig.wechatQRCode}
        qqSupport={supportConfig.qqSupport}
      />
      <Modal
        title={`${versionLabel} ${t('更新日志')}`}
        visible={versionLogVisible}
        onCancel={() => setVersionLogVisible(false)}
        footer={null}
        width={520}
      >
        <div
          className='pb-20'
          style={{ whiteSpace: 'pre-wrap', lineHeight: 1.8 }}
        >
          {statusState?.status?.version?.log || t('暂无更新日志')}
        </div>
      </Modal>
      {showDefaultHome ? (
        <>
          <div className='home-header-wrapper'>
            <header className='home-header'>
              <Link to='/' className='home-logo-area'>
                <span className='home-logo-icon'>
                  <img src={logo} alt={systemName} />
                </span>
                <span className='home-logo-text'>{systemName}</span>
              </Link>

              <nav className='home-nav-menu'>
                <Link to='/' className='active'>
                  {t('首页')}
                </Link>
                <Link to={consoleNavTarget}>{t('控制台')}</Link>
                <Link to={pricingNavTarget}>{t('模型广场')}</Link>
                <a href={docsHref} target='_blank' rel='noreferrer'>
                  {t('文档')}
                </a>
                <Link to='/about'>{t('关于')}</Link>
              </nav>

              <div className='home-header-actions'>
                <div className='home-header-tools'>
                  <ThemeToggle
                    theme={theme}
                    onThemeToggle={handleThemeToggle}
                    t={t}
                  />
                  <LanguageSelector
                    currentLang={i18n.language}
                    onLanguageChange={handleLanguageChange}
                    t={t}
                  />
                </div>
                {isLoggedIn ? (
                  <UserArea
                    userState={normalizedUserState}
                    isLoading={false}
                    isMobile={isMobile}
                    isSelfUseMode={isSelfUseMode}
                    logout={logout}
                    navigate={navigate}
                    t={t}
                  />
                ) : (
                  <>
                    <Link to='/login' className='home-login-link'>
                      {t('登录')}
                    </Link>
                    <Link to='/console' className='home-btn-primary'>
                      {t('获取 API Key')}
                    </Link>
                  </>
                )}
              </div>
            </header>
          </div>

          <section className='home-hero'>
            <div className='home-container home-hero-content'>
              <div className='home-hero-left'>
                <div
                  className='home-pill-tag'
                  onClick={() => setVersionLogVisible(true)}
                >
                  {versionLabel} {t('现已上线')}
                </div>
                <h1 className='home-hero-title'>
                  {t('一套 API，畅连所有 AI')}
                </h1>
                <p className='home-hero-subtitle'>
                  {t(
                    '统一的大模型网关。可在 OpenAI、Claude、Llama 及 50+ 模型间即时切换，并通过智能路由节省成本。',
                  )}
                </p>
                <div className='home-hero-buttons'>
                  <Link
                    to='/console'
                    target='_blank'
                    className='home-btn-primary home-btn-large'
                  >
                    {t('免费开始构建')}&nbsp;&nbsp;
                    <ArrowRight size={18} />
                  </Link>
                  <a
                    href={docsHref}
                    target='_blank'
                    rel='noreferrer'
                    className='home-btn-ghost home-btn-large'
                  >
                    {t('阅读文档')}
                  </a>
                </div>
                <div className='home-hero-stats'>
                  {heroStats.map((stat) => (
                    <>
                    <div className='home-stat-item' key={stat.value}>
                      <h3>{ stat.tag && <span className='home-stat-tag'>{t(stat.tag)}</span>} {t(stat.value)}</h3>
                      <p>{t(stat.label)}</p>
                    </div>
                    <div className='home-stat-item-line'></div>
                    </>
                  ))}
                </div>
              </div>

              <div className='home-hero-right'>
                <div className='home-mockup-window'>
                  <div className='home-mockup-header'>
                    <div className='home-window-controls'>
                      <span className='home-dot home-dot-red' />
                      <span className='home-dot home-dot-yellow' />
                      <span className='home-dot home-dot-green' />
                    </div>
                    <span className='home-window-title'>
                      {systemName} / {t('智能路由控制台')}
                    </span>
                    <span className='home-window-badge'>
                      {t('实时路由')}
                    </span>
                  </div>

                  <div className='home-mockup-body'>
                    <div className='home-mockup-left-panel'>
                      <div className='home-panel-title'>
                        <h4>{t('模型请求编排')}</h4>
                        <p>
                          {t(
                            '根据延迟、价格、稳定性自动选择最佳模型路径。',
                          )}
                        </p>
                      </div>
                      <div className='home-model-list'>
                        <div className='home-model-item'>
                          <div className='home-model-info'>
                            <span className='home-status-dot orange' />
                            OpenAI
                            <br />
                            <small>{t('主路由')}</small>
                          </div>
                          <div className='home-model-metric'>128ms</div>
                        </div>
                        <div className='home-model-item'>
                          <div className='home-model-info'>
                            <span className='home-status-dot sand' />
                            Claude
                            <br />
                            <small>{t('质量优先')}</small>
                          </div>
                          <div className='home-model-metric'>142ms</div>
                        </div>
                        <div className='home-model-item'>
                          <div className='home-model-info'>
                            <span className='home-status-dot green' />
                            DeepSeek
                            <br />
                            <small>{t('成本优先')}</small>
                          </div>
                          <div className='home-model-metric'>96ms</div>
                        </div>
                        <div className='home-model-item'>
                          <div className='home-model-info'>
                            <span className='home-status-dot orange' />
                            {t('本地降级')}
                            <br />
                            <small>{t('备用通道')}</small>
                          </div>
                          <div className='home-model-metric'>204ms</div>
                        </div>
                      </div>
                      <div className='home-progress-bar-container'>
                        <div className='home-progress-bar'>
                          <div
                            className='home-progress-fill'
                            style={{ width: '65%' }}
                          />
                        </div>
                      </div>
                    </div>

                    <div className='home-mockup-right-panel'>
                      <div className='home-code-card'>
                        <div className='home-code-header'>main.py</div>
                        <div className='home-code-content'>
                          {typedCodeLines.map((line, index) => (
                            <div
                              key={`home-code-line-${index}`}
                              dangerouslySetInnerHTML={{ __html: line }}
                            />
                          ))}
                        </div>
                      </div>
                      <div className='home-savings-card'>
                        <div className='home-savings-title'>
                          {t('智能路由节省')}
                        </div>
                        <div className='home-savings-amount'>{t('快速构建')}</div>
                        <div className='home-savings-desc'>
                          {t('通过智能路由策略，帮助优化模型调用成本。')}
                        </div>
                        <div className='home-savings-bar'>
                          <div
                            className='home-savings-fill'
                            style={{ width: '75%' }}
                          />
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section className='home-logo-bar-section'>
            <div className='home-container home-logo-bar-container'>
              <p className='home-logo-bar-title'>
                {t('支持全球主流开源模型')}
              </p>
              <MarqueeLogos logos={partnerLogos} speed={0.5} />
            </div>
          </section>

          <section className='home-token-section'>
            <div className="w-full mx-auto bg-white rounded-[24px] border border-gray-100 p-6 md:p-12 shadow-sm font-sans flex flex-col lg:flex-row lg:items-center lg:justify-between gap-8 md:gap-12 relative overflow-hidden">

              {/* 右上角淡橙色渐变背景装饰 */}
              <div className="absolute w-[460px] h-[460px] home-token-bg-gradient" />

              {/* 左侧文字区块 */}
              <div className="flex-1 z-10">
                <h1 className="text-[28px] md:text-[38px] font-bold text-gray-900 leading-[1.3] tracking-wide">
                  {t('生产 AGENTIC 时代的高效 Token，')}
                  <br className="hidden md:inline" />
                  {t('致力于实现 AI 平权')} 
                </h1>
                <p className="mt-4 md:mt-6 text-sm md:text-base text-gray-500 leading-relaxed max-w-[540px]">
                  {t('打造新时期的推理底座：面向复杂推理、长上下文与代码专家场景，满足AGENTIC复杂推理（1M长文本）任务的代码专家。')}
                </p>
              </div>

              {/* 右侧交互与卡片区块 */}
              <div className="flex-1 flex flex-col items-start lg:items-end gap-6 z-10 lg:w-auto">

                {/* 四个白色胶囊标签 - 移动端自动换行/平铺 */}
                <div className="flex flex-wrap gap-2 md:gap-3 w-full justify-start lg:justify-end">
                  {['智能', '高效', '性价比', '1M 长文本'].map((tag, index) => (
                    <span
                      key={index}
                      className="px-8 py-2 bg-white border border-[#E2E6ED] rounded-full text-xs md:text-sm font-[700] text-[#111621] whitespace-nowrap"
                    >
                      {t(tag)}
                    </span>
                  ))}
                </div>

                {/* 黑色核心特性卡片 */}
                <div className="w-full lg:w-[610px] bg-[#0D1117] rounded-[20px] p-5 md:p-6 shadow-[0_15px_30px_rgba(0,0,0,0.15)] flex justify-between items-center gap-4 border border-gray-800">
                  {/* 黑色卡片左侧文本 */}
                  <div className="flex-1">
                    <h3 className="text-white text-base md:text-[18px] font-semibold tracking-wide mb-3">
                      {t('面向 1M 长文本与代码专家的复杂推理')}
                    </h3>
                    {/* 点状分隔的英文标签组 */}
                    <div className="text-[11px] md:text-xs text-gray-400 font-mono flex flex-wrap gap-x-2 gap-y-1 leading-normal">
                      <span>agentic reasoning</span>
                      <span className="text-gray-600">•</span>
                      <span>code expert</span>
                      <span className="text-gray-600">•</span>
                      <span>hybrid inference</span>
                      <span>cost-aware routing</span>
                      <span className="text-gray-600">•</span>
                      <span>token production</span>
                    </div>
                  </div>

                  {/* 黑色卡片右侧图标组 */}
                  <div className="flex items-center gap-3 shrink-0">
                    <div className="flex flex-col items-center gap-1.5">
                      <div className="w-11 h-11 md:w-12 md:h-12 bg-[#FF5700] rounded-[12px] flex items-center justify-center font-bold text-white text-sm shadow-lg shadow-orange-700/20">
                        {`</>`}
                      </div>
                      <span className="text-[10px] text-gray-400 font-medium">Code</span>
                    </div>
                    <div className="flex flex-col items-center gap-1.5">
                      <div className="w-11 h-11 md:w-12 md:h-12 bg-[#00C06B] rounded-[12px] flex items-center justify-center font-bold text-white text-sm md:text-base tracking-tighter shadow-lg shadow-green-700/20">
                        1M
                      </div>
                      <span className="text-[10px] text-gray-400 font-medium">Context</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section className='home-features-section'>
            <div className='home-container'>
              <div className='home-section-header'>
                <h2 className='home-section-title'>
                  {t('为什么选择')}
                  {systemName}
                </h2>
                <p className='home-section-subtitle'>
                  {t(
                    '把接入、路由、成本和生态能力收束到一套统一接口，面向开发者和企业团队提供稳定的大模型基础设施。',
                  )}
                </p>
              </div>

              <div className='home-features-grid'>
                {featureCards.map((feature) => (
                  <article
                    key={feature.title}
                    className='home-feature-card'
                  >
                    <div
                      className={`home-icon-wrapper`}
                    >
                      <img src={feature.imgUrl} alt={feature.title} />
                    </div>
                    <h3>{t(feature.title)}</h3>
                    <p>{t(feature.description)}</p>
                  </article>
                ))}
              </div>
            </div>
          </section>

          <section className='home-workflow-section'>
            <div className='home-container home-workflow-inner'>
              <h2 className='home-section-title'>
                {t('从一次请求开始，自动完成模型选择')}
              </h2>
              <p className='home-section-subtitle'>
                {getSystemName()} 
                {t(
                  '将模型接入、价格判断、稳定性降级与请求观测串成一条可管理的链路。',
                )}
              </p>

              <div className='home-workflow-grid'>
                {workflowSteps.map((step) => (
                  <div
                    className='home-workflow-step'
                    key={step.number}
                  >
                    <div className='home-step-number'>{step.number}</div>
                    <h4>{t(step.title)}</h4>
                    <p>{t(step.description)}</p>
                  </div>
                ))}
              </div>
            </div>
          </section>

          <section className='home-agentic-section'>
            <div className='home-container'>
              <div className='home-section-header'>
                <h2 className='home-section-title'>
                  {t('公司的产品定位')}
                </h2>
                <p className='home-section-subtitle'>
                  {t(
                    '在 AGENTIC 驱动 AI 产业变化的当下，通过软硬一体化的深度协同，重点产出能够满足 AGENTIC 时代复杂推理（1M 长文本）任务的、具备高性价比的 TOKEN 产品。',
                  )}
                </p>
              </div>

              <div className='home-agentic-grid'>
                <div className='home-agentic-left-card'>
                  <div className='home-agentic-info-block'>
                    <div className='home-pill-tag'>
                      {t('TOKEN PRODUCT')}
                    </div>
                    <h3>{t('高性价比的TOKEN产品')}</h3>
                    <p>
                      {t(
                        '以模型路由、推理优化与异构算力协同为核心，把 1M 长文本、代码专家等复杂推理任务转化为稳定、可计量、可规模化交付的高效 Token。',
                      )}
                    </p>
                  </div>
                  <div className='home-agentic-visual-block'>
                    <img src={imgOne} alt='imgOne' className='home-refined-img' />
                  </div>
                </div>

                <div className='home-agentic-right-card'>
                  <h3>{t('核心定位')}</h3>
                  <p>
                    {t(
                      '智能 · 高效 · 性价比，围绕复杂推理任务构建新时期 AI Inference 底座。',
                    )}
                  </p>
                  <div className='home-explore-pills'>
                    {explorePills.map((pill) => (
                      <div
                        className='home-explore-pill'
                        key={pill.name}
                      >
                        <span>{t(pill.name)}</span>
                        <small>{t(pill.desc)}</small>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section>
            <div className="home-token-section home-gpu">
              <div className="relative w-full mx-auto bg-[#14161a] rounded-[28px] border border-[#22252a] p-4 sm:p-6 md:p-12 flex flex-col lg:flex-row items-center justify-between gap-10 lg:gap-16 overflow-hidden">
                <div className="absolute -top-40 -right-40 w-[600px] h-[600px] bg-gradient-to-bl from-orange-500/10 to-transparent rounded-full blur-[130px] pointer-events-none" />
                <div className="w-full lg:flex-1 flex flex-col justify-center z-10">
                  <span className="text-[11px] md:text-xs font-mono font-bold tracking-[0.18em] text-[#D4C39E] uppercase block mb-4 md:mb-5">
                    AI Inference Base
                  </span>
                  <h1 className="text-[30px] sm:text-4xl md:text-[48px] font-bold text-white tracking-tight leading-[1.15] mb-5">
                    GPU + ARM AI CPU
                  </h1>
                  <p className="text-[14px] sm:text-base md:text-[22px] text-[#A3ADC2] leading-relaxed max-w-[580px] font-normal mb-6 md:mb-14">
                    {t('GPU + ARM AI CPU 全新混合架构，重新定义 AI Agentic 计算基础设施。')}
                  </p>
                  <div className="w-full max-w-[640px] bg-[#090C11] border border-[#2E3745] rounded-[16px] p-5 sm:p-6 font-mono text-xs mt-8 sm:text-xs md:text-[14px] leading-[1.8] tracking-wide shadow-inner">
                    <div className="flex items-start gap-2.5">
                      <span className="text-[#00c06b] shrink-0 font-bold">&gt;</span>
                      <p className="text-[#86868b]">
                        <span className="text-[#00c06b]">product: efficient token for agentic era</span>
                      </p>
                    </div>
                    <div className="flex items-start gap-2.5 mt-2">
                      <span className="text-[#00c06b] shrink-0 font-bold">&gt;</span>
                      <p className="text-[#86868b]">
                        <span className="text-white">context_window: 1M long-context reasoning</span>
                      </p>
                    </div>
                    <div className="flex items-start gap-2.5 mt-2">
                      <span className="text-[#00c06b] shrink-0 font-bold">&gt;</span>
                      <p className="text-[#86868b]">
                        <span className="text-[#D4C39E]">architecture: GPU + ARM AI CPU co-optimization</span>
                      </p>
                    </div>
                  </div>
                </div>

                <div className="w-full lg:w-[620px] xl:w-[680px] lg:h-[330px] shrink-0 z-10 flex flex-col gap-6 lg:gap-8 items-center lg:items-end">
                  <div className="relative w-full min-h-[280px] lg:aspect-[1.65/1] bg-[#1a1d24] border border-[#2d3139] rounded-[24px] flex flex-col justify-center items-center shadow-[0_16px_40px_rgba(0,0,0,0.5)] px-6 sm:px-12 py-8 lg:py-0 overflow-hidden">
                    <div className="flex items-center justify-between w-full max-w-[440px] relative mb-6 lg:mb-10">
                      <div className="flex flex-col gap-2 opacity-40">
                        {[...Array(6)].map((_, i) => (
                          <div key={i} className="w-12 h-[1.2px] bg-gradient-to-r from-transparent to-white" />
                        ))}
                      </div>
                      {/* 中央 ARM 核心高亮橙色块 */}
                      <div className="relative z-10 w-24 h-24 sm:w-28 sm:h-28 bg-[var(--home-primary)] rounded-[24px] flex items-center justify-center text-white font-bold text-[24px] sm:text-[28px] tracking-wide shadow-[0_10px_30px_rgba(255,87,0,0.4)] transition-transform duration-300 hover:scale-105">
                        ARM
                      </div>

                      {/* 右侧 6 条平行渐变线条 */}
                      <div className="flex flex-col gap-2 opacity-40">
                        {[...Array(6)].map((_, i) => (
                          <div key={i} className="w-12 h-[1.2px] bg-gradient-to-l from-transparent to-white" />
                        ))}
                      </div>
                    </div>
                    <div className="w-full flex flex-wrap justify-center lg:justify-end gap-2.5 sm:gap-3">
                      <span className="px-4 py-2 sm:px-5 sm:py-2.5 bg-[var(--home-primary)] text-white font-bold text-[11px] sm:text-xs rounded-full shadow-[0_4px_12px_rgba(255,87,0,0.25)] whitespace-nowrap">
                        GPU Compute
                      </span>
                      {[
                        'ARM AI CPU',
                        'Hybrid Arch',
                        'Agentic Compute',
                        'AI Infrastructure',
                        'Hybrid Nodes'
                      ].map((text, idx) => (
                        <span
                          key={idx}
                          className="px-4 py-2 sm:px-5 sm:py-2.5 bg-white text-[#0f1115] font-bold text-[11px] sm:text-xs rounded-full shadow-[0_4px_12px_rgba(0,0,0,0.1)] whitespace-nowrap transition-transform duration-200 hover:-translate-y-0.5"
                        >
                          {text}
                        </span>
                      ))}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section className='home-brand-section'>
            <div className='home-container'>
              <div className='home-brand-card'>
                <div className='home-brand-left'>
                  <h2>{t('新易算')}</h2>
                  <h4>
                    {t(
                      '面向企业智能化场景的新一代算力与 AI 基础设施服务商',
                    )}
                  </h4>
                  <p>
                    {t(
                      '新易算专注于将模型能力、推理资源与业务流程连接成稳定、可扩展的智能基础设施。我们希望通过更清晰的资源调度、更灵活的模型接入和更低门槛的工程化能力，帮助企业把 AI 从单点能力推进到持续运行的生产系统。',
                    )}
                  </p>
                  <div className='home-brand-pills'>
                    <span>{t('AI 基础设施')}</span>
                    <span>{t('算力调度')}</span>
                    <span>{t('模型接入')}</span>
                    <span>{t('企业工作流')}</span>
                  </div>
                </div>
                <div className='home-brand-right'>
                  <div className='home-brand-system-header'>
                    <span>Agentic Infra</span>
                    <div className='home-brand-badge'>● AI Infra</div>
                  </div>
                  <div className='home-brand-visual'>
                    <div className='home-brand-logo-mock'>
                      <img src={newLogo} alt={systemName} />
                    </div>
                  </div>
                  <div className='home-brand-footer'>
                    <h4>{t('新易算')} | {systemName}</h4>
                    <p>
                      {t(
                        'GPU + ARM AI CPU 混合架构驱动的新一代算力底座 + 企业级API路由网关',
                      )}
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section className="home-token-section home-intro">
            <div className="w-full bg-white rounded-[32px] border border-gray-100 p-6 sm:p-10 md:p-14 lg:p-16 flex flex-col lg:flex-row items-stretch justify-between gap-10 lg:gap-14 shadow-[0_10px_50px_rgba(0,0,0,0.03)]">
              <div className="w-full lg:flex-1 flex flex-col justify-between py-1 z-10">
                <div>
                  <h1 className="text-[28px] md:text-[56px] font-bold text-[#111621] tracking-tight mb-4 md:mb-5">
                    {t('合作方介绍')}
                  </h1>
                  <h3 className="text-base md:text-[26px] font-semibold text-[#29303D] tracking-normal mb-4 md:mb-5 leading-snug">
                    {t('FEDIMOSS × 之江易算，共建 AGENTIC 时代 AI Inference 生产底座')}
                  </h3>

                  <div className="text-[15px] md:text-[22px] text-[#656E7C] leading-[1.75] tracking-wide mb-6 md:mb-8">
                    {t('一侧负责 AI 基础设施软件栈研究与异构推理优化，一侧负责算力技术集成应用与企业场景交付。能力互补，把复杂推理能力转化为稳定、可规模化的高效 Token 供给。')}
                  </div>
                  <div className="w-full bg-[#F6F7F9] rounded-[16px] p-4 sm:p-5 flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4 border border-[#E2E6ED] mb-8 lg:mb-12">
                    <span className="text-sm md:text-[20px] font-bold text-gray-900 shrink-0">
                      {t('强强联合')}
                    </span>
                    <div className="hidden sm:block w-[1px] h-4 bg-gray-300" />
                    <p className="text-[13px] md:text-[18px] text-[#656E7C] leading-relaxed">
                      {t('打造高性价比 Token 产品 · 1M 长文本复杂推理 · AI Inference 底座')}
                    </p>
                  </div>
                </div>
                <div className="w-full flex flex-wrap gap-2.5 sm:gap-3">
                  <span className="px-5 py-2 bg-white border border-gray-100 rounded-full text-[14px] sm:text-xs font-medium text-[var(--home-primary)] shadow-[0_2px_8px_rgba(0,0,0,0.02)] whitespace-nowrap">
                    {t('AI 软件栈')}
                  </span>
                  {['异构推理', '算力集成', '企业交付'].map((text, idx) => (
                    <span
                      key={idx}
                      className="px-5 py-2 bg-white border border-gray-100 rounded-full text-[11px] sm:text-xs font-medium text-gray-600 shadow-[0_2px_8px_rgba(0,0,0,0.02)] whitespace-nowrap"
                    >
                      {t(text)}
                    </span>
                  ))}
                </div>
              </div>

              {/* =================【右侧：黑底深邃架构卡片区域】================= */}
              <div className="w-full lg:w-[690px] xl:w-[690px] bg-[#0c0e12] rounded-[24px] border border-[#1b1e24] p-6 sm:p-8 flex flex-col gap-5 shadow-[0_20px_50px_rgba(0,0,0,0.3)] shrink-0">

                {/* 卡片头部的小型英文字缀 */}
                <div className="border-b border-[#1c1f26] pb-4 mb-1">
                  <span className="text-[14px] sm:text-[15px] font-mono font-bold tracking-[0.18em] text-[var(--home-primary)] uppercase block">
                    PARTNERSHIP STACK
                  </span>
                </div>

                {/* 1. FEDIMOSS 内层子卡片 */}
                <div className="w-full bg-[#14171d] border border-[#222730] rounded-[16px] p-5 sm:p-6 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 transition-all duration-300 hover:border-[#313845]">
                  <div className="flex items-center gap-4">
                    {/* 蓝色模拟 Logo 块 */}
                    <div className="w-12 h-12 rounded-[12px] flex items-center justify-center text-white font-black text-xl shadow-[0_4px_12px_rgba(29,78,216,0.3)] shrink-0">
                      <img src={fedimoossLogo} alt="FEDIMOSS" />
                    </div>
                    <div>
                      <h4 className="text-white text-[16px] sm:text-[18px] font-bold tracking-wide">
                        FEDIMOSS
                      </h4>
                      <p className="text-[11px] sm:text-xs text-gray-400 mt-1">
                        {t('AI 基础设施软件栈研究')}
                      </p>
                    </div>
                  </div>
                  {/* 右侧解析细字描述 */}
                  <p className="text-[11px] sm:text-xs text-gray-400 leading-relaxed sm:max-w-[240px] border-t sm:border-t-0 border-[#222730] pt-3 sm:pt-0 w-full sm:w-auto">
                    {t('ARM架构CPU 高性能推理优化、CPU+GPU 异构方案，已和北美知名 AI 处理器芯片机构技术合作。')}
                  </p>
                </div>

                {/* 2. 之江易算 内层子卡片 */}
                <div className="w-full bg-[#14171d] border border-[#222730] rounded-[16px] p-5 sm:p-6 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 transition-all duration-300 hover:border-[#313845]">
                  <div className="flex items-center gap-4">
                    {/* 青蓝色模拟 Logo 块 */}
                    <div className="w-12 h-12 bg-white rounded-[12px] flex items-center justify-center border border-gray-200 shadow-sm shrink-0">
                      <img src={newLogo} alt="之江易算" className="w-4" />
                    </div>
                    <div>
                      <h4 className="text-white text-[16px] sm:text-[18px] font-bold tracking-wide">
                        {t('之江易算')}
                      </h4>
                      <p className="text-[11px] sm:text-xs text-gray-400 mt-1">
                        {t('算力技术集成应用')}
                      </p>
                    </div>
                  </div>
                  {/* 右侧解析细字描述 */}
                  <p className="text-[11px] sm:text-xs text-gray-400 leading-relaxed sm:max-w-[240px] border-t sm:border-t-0 border-[#222730] pt-3 sm:pt-0 w-full sm:w-auto">
                    {t('面向企业场景提供算力集成、应用落地与工程化交付能力。为上市公司集智股份的全资子公司。')}
                  </p>
                </div>
              </div>
            </div>
          </section>

          <footer className='home-footer-section w-full'>
            <div className='home-footer-container'>
              <div className='home-footer-brand'>
                <div>
                  <div className='home-footer-logo'>
                    <img src={logo} alt={systemName} />
                    <span>{systemName}</span>
                  </div>
                  <p>
                    {t(
                      '统一模型网关，为全球开发者提供高可用、低成本的 AI 基础设施服务。',
                    )}
                  </p>
                </div>
                <div className='home-footer-bottom'>
                  <span dangerouslySetInnerHTML={{ __html: statusState?.status.footer_html || `© ${new Date().getFullYear()} ${systemName}. All rights reserved.` }} />
                </div>
              </div>

              <div className='home-footer-links-grid'>
                <div className='home-footer-column'>
                  <h5>{t('产品')}</h5>
                  <ul>
                    <li>
                      <Link to={consoleNavTarget}>{t('控制面板')}</Link>
                    </li>
                    <li>
                      <Link to={pricingNavTarget}>{t('模型广场')}</Link>
                    </li>
                    <li>
                      <Link to='/about'>{t('关于平台')}</Link>
                    </li>
                  </ul>
                </div>
                <div className='home-footer-column'>
                  <h5>{t('资源')}</h5>
                  <ul>
                    <li>
                      <a href={docsHref} target='_blank' rel='noreferrer'>
                        {t('文档')}
                      </a>
                    </li>
                    <li>
                      <a
                        href={apiReferenceHref}
                        target='_blank'
                        rel='noreferrer'
                      >
                        {t('API 参考')}
                      </a>
                    </li>
                    <li>
                      <a href={communityHref} target='_blank' rel='noreferrer'>
                        {t('社区')}
                      </a>
                    </li>
                    <li>
                      <a
                        href={`https://status.${systemName.toLowerCase()}/`}
                        target='_blank'
                        rel='noreferrer'
                      >
                        {t('系统状态')}
                      </a>
                    </li>
                  </ul>
                </div>
                <div className='home-footer-column'>
                  <h5>{t('帮助中心')}</h5>
                  <ul>
                    <li>
                      <a
                        href='https://github.com/fedimoss/allrouter'
                        target='_blank'
                        rel='noreferrer'
                      >
                        {t('项目仓库')}
                      </a>
                    </li>
                    <li>
                      <a
                        href='https://github.com/fedimoss/allrouter/issues'
                        target='_blank'
                        rel='noreferrer'
                      >
                        {t('问题反馈')}
                      </a>
                    </li>
                    <li>
                      <a
                        href={`mailto:support@${systemName.toLowerCase()}`}
                      >
                        {t('联系我们')}
                      </a>
                    </li>
                  </ul>
                </div>
              </div>
            </div>
          </footer>
        </>
      ) : (
        <div className='overflow-x-hidden w-full'>
          {homePageContent.startsWith('https://') ? (
            <iframe
              src={homePageContent}
              className='w-full h-screen border-none'
            />
          ) : (
            <div
              className='mt-[60px]'
              dangerouslySetInnerHTML={{ __html: homePageContent }}
            />
          )}
        </div>
      )}
    </div>
  );
};

export default Home;
