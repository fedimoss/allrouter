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
import { API, showError } from '../../helpers';
import { StatusContext } from '../../context/Status';
import { UserContext } from '../../context/User';
import { useActualTheme, useSetTheme, useTheme } from '../../context/Theme';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { marked } from 'marked';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate } from 'react-router-dom';
import NoticeModal from '../../components/layout/NoticeModal';
import ThemeToggle from '../../components/layout/headerbar/ThemeToggle';
import LanguageSelector from '../../components/layout/headerbar/LanguageSelector';
import NotificationButton from '../../components/layout/headerbar/NotificationButton';
import UserArea from '../../components/layout/headerbar/UserArea';
import MarqueeLogos from '../../components/common/MarqueeLogos';

import { getLogo, getSystemName } from '../../helpers';

import openaiLogo from '../../../public/logos/openai.svg';
import anthropicLogo from '../../../public/logos/anthropic.svg';
import googleLogo from '../../../public/logos/google.svg';
import zhipuLogo from '../../../public/logos/zhipu.svg';
import metaLogo from '../../../public/logos/meta.svg';
import deepseekLogo from '../../../public/logos/deepseek.svg';
import huggingFaceLogo from '../../../public/logos/huggingface.svg';
import kimiLogo from '../../../public/logos/kimi.svg';
import minimaxLogo from '../../../public/logos/minimax.svg';
import chatglmLogo from '../../../public/logos/chatglm.svg';
import doubaoLogo from '../../../public/logos/doubao.svg';

const logo = getLogo();
const systemName = getSystemName();

const partnerLogos = [
  { src: openaiLogo, alt: 'OpenAI' },
  { src: anthropicLogo, alt: 'Anthropic' },
  { src: googleLogo, alt: 'Gemini' },
  { src: zhipuLogo, alt: 'Zhipu AI' },
  { src: metaLogo, alt: 'Meta' },
  { src: deepseekLogo, alt: 'DeepSeek' },
  { src: huggingFaceLogo, alt: 'Hugging Face' },
  { src: kimiLogo, alt: 'Kimi' },
  { src: minimaxLogo, alt: 'Minimax' },
  { src: chatglmLogo, alt: 'ChatGLM' },
  { src: doubaoLogo, alt: 'Doubao' },
];

const featureCards = [
  {
    icon: 'fas fa-code-branch',
    iconClass: 'landing-v2-card-icon-cyan',
    title: '多渠道比价',
    description:
      '智能分发到更稳、更快、更划算的上游，让高并发和成本控制同时成立。',
  },
  {
    icon: 'fas fa-layer-group',
    iconClass: 'landing-v2-card-icon-violet',
    title: 'OpenAI 兼容接入',
    description:
      '保持 OpenAI SDK 调用方式，替换 base_url 与 key 即可接入现有系统。',
  },
  {
    icon: 'fas fa-shield-halved',
    iconClass: 'landing-v2-card-icon-amber',
    title: '自愈故障保障',
    description: '当上游波动或限流时自动切换可用节点，减少中断和人工介入成本。',
  },
  {
    icon: 'fas fa-rocket',
    iconClass: 'landing-v2-card-icon-rose',
    title: '高效生态入驻',
    description:
      '围绕鉴权、计费、渠道管理与监控，帮助团队更快把 AI 能力真正上线。',
  },
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
  const [statusState] = useContext(StatusContext);
  const actualTheme = useActualTheme();
  const navigate = useNavigate();
  const theme = useTheme();
  const setTheme = useSetTheme();
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(false);
  const [homePageContent, setHomePageContent] = useState('');
  const [noticeVisible, setNoticeVisible] = useState(false);
  const [typedCodeLines, setTypedCodeLines] = useState([]);
  const isMobile = useIsMobile();

  const docsLink = statusState?.status?.docs_link || '';
  const serverAddress =
    statusState?.status?.server_address || `${window.location.origin}`;
  const showDefaultHome = homePageContentLoaded && homePageContent === '';
  const docsLangPrefix = i18n.language.startsWith('zh') ? 'zh' : 'en';

  const docsHref = docsLink || `https://allrouter.ai/${docsLangPrefix}/docs`;
  const apiReferenceHref = `https://allrouter.ai/${docsLangPrefix}/docs/api`;
  const communityHref = `https://allrouter.ai/${docsLangPrefix}/docs/support/community-interaction`;
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
  const normalizedUserState = { user: currentUser };

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
    if (!showDefaultHome) {
      return undefined;
    }

    const codeBaseUrl = `${serverAddress.replace(/\/$/, '')}/v1`;
    const codeLines = [
      {
        html: `<span class='landing-v2-c-c'># ${escapeHtml(t('安装官方 SDK'))}</span>`,
        delay: 420,
      },
      {
        html: "<span class='landing-v2-c-k'>pip</span> install openai",
        delay: 720,
      },
      {
        html: '',
        delay: 160,
      },
      {
        html: "<span class='landing-v2-c-k'>import</span> openai",
        delay: 620,
      },
      {
        html: '',
        delay: 140,
      },
      {
        html: `<span class='landing-v2-c-c'># ${escapeHtml(t('只需替换 base_url 与 api_key'))}</span>`,
        delay: 640,
      },
      {
        html: 'client = openai.OpenAI(',
        delay: 360,
      },
      {
        html: `    base_url=<span class='landing-v2-c-s'>'${escapeHtml(codeBaseUrl)}'</span>,`,
        delay: 360,
      },
      {
        html: "    api_key=<span class='landing-v2-c-s'>'sk-...'</span>",
        delay: 360,
      },
      {
        html: ')',
        delay: 420,
      },
      {
        html: '',
        delay: 160,
      },
      {
        html: `<span class='landing-v2-c-c'># ${escapeHtml(
          t('任意模型即开即用：gpt-4、claude-3、llama-3...')
        )}</span>`,
        delay: 620,
      },
      {
        html: 'response = client.chat.completions.create(',
        delay: 360,
      },
      {
        html: "    model=<span class='landing-v2-c-s'>'claude-3-opus'</span>,",
        delay: 360,
      },
      {
        html: "    messages=[{<span class='landing-v2-c-s'>'role'</span>: <span class='landing-v2-c-s'>'user'</span>, <span class='landing-v2-c-s'>'content'</span>: <span class='landing-v2-c-s'>'Hello!'</span>}]",
        delay: 360,
      },
      {
        html: ')',
        delay: 260,
      },
      {
        html: "<span class='landing-v2-cursor'></span>",
        delay: 0,
      },
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
    <div
      className={
        showDefaultHome
          ? `landing-home landing-v2 ${!isLoggedIn ? 'landing-v2-guest-home' : ''}`
          : 'w-full overflow-x-hidden'
      }
    >
      <NoticeModal
        visible={noticeVisible}
        onClose={() => setNoticeVisible(false)}
        isMobile={isMobile}
      />
      {showDefaultHome ? (
        <>
          <nav
            className={`landing-v2-nav ${!isLoggedIn ? 'landing-v2-nav-fixed' : ''}`}
          >
            <div className='landing-v2-logo'>
              <div className='landing-v2-logo-bg'>
              <img
                src={logo}
                className='landing-v2-real-logo'
                />
              </div>
              <span>{ systemName }</span>
            </div>

            <div className='landing-v2-nav-links'>
              <Link to='/' className='landing-v2-nav-link-active'>{t('首页')}</Link>
              <Link to={consoleNavTarget}>{t('控制台')}</Link>
              <Link to={pricingNavTarget}>{t('模型广场')}</Link>
              <a href={docsHref} target='_blank' rel='noreferrer'>
                {t('文档')}
              </a>
              <Link to='/about'>{t('关于')}</Link>
            </div>

            <div className='landing-v2-nav-actions'>
              <div className='landing-v2-nav-tools'>
                {/* <NotificationButton
                  unreadCount={0}
                  onNoticeOpen={handleNoticeOpen}
                  t={t}
                /> */}
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
                  <Link to='/login' className='landing-v2-btn-text'>
                    {t('登录')}
                  </Link>
                  <Link to='/console' className='landing-v2-btn-primary'>
                    {t('获取 API Key')}
                  </Link>
                </>
              )}
            </div>
          </nav>

          <main className='landing-v2-main'>
            <section className='landing-v2-hero'>
              <div className='landing-v2-hero-content'>
                <div className='landing-v2-hero-badge'>
                  <span className='landing-v2-hero-badge-dot' />
                  {t('V2.0 现已上线')}
                </div>
                <div className='landing-v2-hero-title'>{t('一套 API，畅连所有 AI')}</div>
                <p>
                  {t(
                    '统一的大模型网关。可在 OpenAI、Claude、Llama 及 50+ 模型间即时切换，并通过智能路由最高节省 50% 成本。',
                  )}
                </p>
                <div className='landing-v2-hero-buttons'>
                  <Link
                    to='/console'
                    target='_blank'
                    className='landing-v2-btn-primary landing-v2-btn-lg'
                  >
                    {t('免费开始构建')}&nbsp;&nbsp;<ArrowRight size={18} />
                  </Link>
                  <a
                    href={docsHref}
                    target='_blank'
                    rel='noreferrer'
                    className='landing-v2-btn-secondary landing-v2-btn-lg'
                  >
                    {t('阅读文档')}
                  </a>
                </div>
              </div>

              <div className='landing-v2-code-shell'>
                <div className='landing-v2-code-window'>
                  <div className='landing-v2-window-header'>
                    <div className='landing-v2-window-dots'>
                      <span className='landing-v2-dot landing-v2-dot-red' />
                      <span className='landing-v2-dot landing-v2-dot-yellow' />
                      <span className='landing-v2-dot landing-v2-dot-green' />
                    </div>
                    <span className='landing-v2-window-title'>
                      import openai
                    </span>
                  </div>
                  <div className='landing-v2-code-content'>
                    {typedCodeLines.map((line, index) => (
                      <div
                        key={`landing-v2-line-${index}`}
                        dangerouslySetInnerHTML={{ __html: line }}
                      />
                    ))}
                  </div>
                </div>
              </div>
            </section>

            <section id='models' className='landing-v2-logo-section'>
              <p>{t('支持全球主流及国产自研模型')}</p>
              <MarqueeLogos logos={partnerLogos} speed={0.5} />
            </section>

            <section id='features' className='landing-v2-features'>
              <div className='landing-v2-section-header'>
                <div className='landing-v2-heading'>
                  {t('为什么选择')}{' '}
                  <span className='landing-v2-heading-accent'>AllRouter</span>
                </div>
              </div>

              <div className='landing-v2-feature-grid'>
                {featureCards.map((feature) => (
                  <article key={feature.title} className='landing-v2-card'>
                    <div
                      className={`landing-v2-card-icon ${feature.iconClass}`}
                    >
                      <i className={feature.icon} />
                    </div>
                    <h3>{t(feature.title)}</h3>
                    <p>{t(feature.description)}</p>
                  </article>
                ))}
              </div>
            </section>

            <section className='landing-v2-cta-section'>
              <div className='landing-v2-cta-box'>
                <div className='landing-v2-cta-box-title'>{t('准备好优化您的 AI 工作流了吗？')}</div>
                <p>
                  {t(
                    '加入 2,000+ 开发者，开始享受更稳定、更廉价的大模型服务。',
                  )}
                </p>
                <Link
                  target='_blank'
                  to='/console'
                  className='landing-v2-btn-primary landing-v2-btn-lg'
                >
                  {t('免费开始构建')}
                </Link>
              </div>
            </section>
          </main>

          <footer className='landing-v2-footer'>
            <div className='landing-v2-footer-top'>
              <div className='landing-v2-footer-brand'>
                <div className='landing-v2-logo landing-v2-logo-small'>
                  <img
                    src={logo}
                    className='landing-v2-real-logo'
                  />
                  <span>{ systemName }</span>
                </div>
                <p>
                  {t(
                    '统一 AI 接入网关，为团队提供模型接入、路由、计费与治理能力。',
                  )}
                </p>
              </div>

              <div className='landing-v2-footer-col'>
                <h4>{t('产品')}</h4>
                <ul>
                  <li>
                    <a href='#features'>{t('功能特性')}</a>
                  </li>
                  <li>
                    <a href='#models'>{t('模型生态')}</a>
                  </li>
                  <li>
                    <Link to='/pricing'>{t('定价')}</Link>
                  </li>
                  <li>
                    <a
                      href='https://github.com/fedimoss/allrouter/releases'
                      target='_blank'
                      rel='noreferrer'
                    >
                      {t('更新日志')}
                    </a>
                  </li>
                </ul>
              </div>

              <div className='landing-v2-footer-col'>
                <h4>{t('资源')}</h4>
                <ul>
                  <li>
                    <a href={docsHref} target='_blank' rel='noreferrer'>
                      {t('文档')}
                    </a>
                  </li>
                  <li>
                    <a href={apiReferenceHref} target='_blank' rel='noreferrer'>
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
                      href='https://status.allrouter.ai/'
                      target='_blank'
                      rel='noreferrer'
                    >
                      {t('系统状态')}
                    </a>
                  </li>
                </ul>
              </div>

              <div className='landing-v2-footer-col'>
                <h4>{t('帮助中心')}</h4>
                <ul>
                  <li>
                    <Link to='/about'>{t('关于平台')}</Link>
                  </li>
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
                    <a href='mailto:support@allrouter.ai'>{t('联系我们')}</a>
                  </li>
                </ul>
              </div>
            </div>

            <div className='landing-v2-footer-bottom'>
              <span>© 2025 {systemName}. All rights reserved.</span>
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
