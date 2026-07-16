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
  fetchNotice,
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
  buildSupportConfig,
  getLogo,
  getSystemName,
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
  { name: 'Agentic', desc: '智能体执行' },
  { name: 'Infra+', desc: '基础设施探索' },
  { name: 'Workflow', desc: '企业工作流' },
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
  const supportConfig = useMemo(
    () => buildSupportConfig(statusState?.status),
    [statusState?.status],
  );
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
          // 拉取当前界面语言对应的站点公告
          const res = await fetchNotice();
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
        wechatDesc={supportConfig.wechatDesc}
        qqQrcode={supportConfig.qqQrcode}
        qqSupport={supportConfig.qqSupport}
        telegramQRCode={supportConfig.telegramQRCode}
        telegramDesc={supportConfig.telegramDesc}
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
                  {systemName} & Agentic AI
                </h2>
                <p className='home-section-subtitle'>
                  {t(
                    '业务定位于 Agentic AI 的基础设施演进，围绕模型调度、智能体协作、推理成本优化与企业级工作流，探索下一代 AI 应用的系统底座。',
                  )}
                </p>
              </div>

              <div className='home-agentic-grid'>
                <div className='home-agentic-left-card'>
                  <div className='home-agentic-info-block'>
                    <div className='home-pill-tag'>
                      {t('AGENTIC AI 探索')}
                    </div>
                    <h3>{t('让智能体从“会对话”走向“能执行”')}</h3>
                    <p>
                      {t(
                        '把模型能力、工具调用、任务编排和企业数据连接成可持续运行的 Agentic 系统。不只是单次响应，而是智能体在复杂业务中持续感知、决策、执行与反馈的完整链路。',
                      )}
                    </p>
                  </div>
                  <div className='home-agentic-visual-block'>
                    <img src={imgOne} alt='imgOne' className='home-refined-img' />
                  </div>
                </div>

                <div className='home-agentic-right-card'>
                  <h3>{t('探索&研究')}</h3>
                  <p>
                    {t(
                      '在 Agentic AI、模型路由、企业工作流与生态合作上的探索方向。',
                    )}
                  </p>
                  <div className='home-explore-pills'>
                    {explorePills.map((pill) => (
                      <div
                        className='home-explore-pill'
                        key={pill.name}
                      >
                        <span>{pill.name}</span>
                        <small>{t(pill.desc)}</small>
                      </div>
                    ))}
                  </div>
                </div>
              </div>

              {/* <div className='home-gpu-arm-card'>
                <div className='home-gpu-arm-left'>
                  <div className='home-gpu-arm-tag'>GPU + ARM AI CPU</div>
                  <h3>GPU + Arm AI CPU</h3>
                  <p>
                    {t(
                      'GPU + Arm AI CPU 全新混合架构，重新定义 AI Agentic 计算基础设施。',
                    )}
                  </p>
                  <div className='home-terminal-box'>
                    <span>&gt; architecture:</span> GPU + Arm AI CPU
                    <br />
                    <span>&gt; compute_mode:</span> hybrid agentic infra
                    <br />
                    <span>&gt; redefine:</span> AI Agentic infrastructure
                  </div>
                </div>
                <div className='home-gpu-arm-right'>
                  <img src={refinedImg} alt='refined' className='home-refined-img' />
                </div>
              </div> */}
            </div>
          </section>

          <section className='home-brand-section'>
            <div className='home-container'>
              <div className='home-brand-card'>
                <div className='home-brand-left'>
                  <h2>新易算</h2>
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
                    <div className='home-brand-badge'>● AI Infra Brand</div>
                  </div>
                  <div className='home-brand-visual'>
                    <div className='home-brand-logo-mock'>
                      <img src={newLogo} alt={systemName} />
                    </div>
                  </div>
                  <div className='home-brand-footer'>
                    <h4>{systemName}</h4>
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

          <footer className='home-footer-section'>
            <div className='home-footer-container'>
              <div className='home-footer-brand'>
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

            <div className='home-footer-bottom'>
              <span>
                © {new Date().getFullYear()} {systemName}. All rights
                reserved.
              </span>
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
