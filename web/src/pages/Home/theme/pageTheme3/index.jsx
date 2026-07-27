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

/**
 * 首页主题 · 风格三（pageTheme3）
 *
 * 由独立静态页 `07-新首页` 迁移而来，纯中文、不做 i18n。
 * 原 helper JS（theme-toggle.js / hero-effects.js / footer-section.js）的行为
 * 全部折叠进本文件内的 React 组件与 Hook。
 *
 * 跳转链接已替换为项目内真实路由（保留静态页 4 项导航布局）。
 */

import React, {
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  ArrowRight,
  Braces,
  LogOut,
  Menu,
  Play,
  RotateCcw,
  ServerCog,
  ShieldCheck,
  X,
  Zap,
} from 'lucide-react';
import {
  API,
  fetchNotice,
  setStatusData,
  showError,
  withBrowserBaseUrl,
  buildSupportConfig,
  getLogo,
  getSystemName,
} from '../../../../helpers';
import { StatusContext } from '../../../../context/Status';
import { UserContext } from '../../../../context/User';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import NoticeModal from '../../../../components/layout/NoticeModal';

import './index.css';

// ---- 静态资源 ----
import logoSvg from '../../../../../public/theme/theme3/allrouter-logo.svg';
import openaiIcon from '../../../../../public/theme/theme3/model-icons/openai.svg';
import claudeIcon from '../../../../../public/theme/theme3/model-icons/claude.svg';
import geminiIcon from '../../../../../public/theme/theme3/model-icons/gemini.svg';
import metaIcon from '../../../../../public/theme/theme3/model-icons/meta.svg';
import deepseekIcon from '../../../../../public/theme/theme3/model-icons/deepseek.svg';
import huggingfaceIcon from '../../../../../public/theme/theme3/model-icons/huggingface.svg';
import kimiIcon from '../../../../../public/theme/theme3/model-icons/kimi.svg';
import minimaxIcon from '../../../../../public/theme/theme3/model-icons/minimax.svg';
import chatglmIcon from '../../../../../public/theme/theme3/model-icons/chatglm.svg';
import doubaoIcon from '../../../../../public/theme/theme3/model-icons/doubao.svg';
import grokIcon from '../../../../../public/theme/theme3/model-icons/grok.svg';
import qwenIcon from '../../../../../public/theme/theme3/model-icons/qwen.svg';

// ---- 常量 ----
const HLS_SRC =
  'https://stream.mux.com/tLkHO1qZoaaQOUeVWo8hEBeGQfySP02EPS02BmnNFyXys.m3u8';
const HLS_VENDOR_PATH = '/theme/theme3/hls.light.js';
const INTRO_SEEN_KEY = 'allrouter_intro_seen';
const PAGE_THEME_KEY = 'allrouter-theme';
const LIGHT_BG = '#f2f6f4';
const DARK_BG = '#070b0a';

// 项目标识（与其它主题一致：后台可配置 systemName / logo）
const systemName = getSystemName();
const siteLogo = getLogo();

// 顶部导航：保留静态页 4 项布局，跳转替换为项目内真实路由
const NAV_ITEMS = [
  { label: '首页', kind: 'home' },
  { label: '令牌管理', kind: 'token' },
  { label: '模型广场', kind: 'pricing' },
  { label: '钱包', kind: 'wallet' },
];

// intro 三步
const INTRO_STEPS = [
  { index: '01', title: '一套 API', caption: '接入 OpenAI、Claude、Llama 及 50+ 模型' },
  { index: '02', title: '智能路由', caption: '自动选择更稳、更快、更划算的调用路径' },
  { index: '03', title: '自建算力集群', caption: 'H200 / B300 / ARM CPU' },
];

// 模型 marquee 12 项
const MODELS = [
  { name: 'OpenAI', icon: openaiIcon },
  { name: 'Claude', icon: claudeIcon },
  { name: 'Gemini', icon: geminiIcon },
  { name: 'Meta', icon: metaIcon },
  { name: 'DeepSeek', icon: deepseekIcon },
  { name: 'Hugging Face', icon: huggingfaceIcon },
  { name: 'Kimi', icon: kimiIcon },
  { name: 'MiniMax', icon: minimaxIcon },
  { name: 'ChatGLM', icon: chatglmIcon },
  { name: 'Doubao', icon: doubaoIcon },
  { name: 'Grok', icon: grokIcon },
  { name: 'Qwen', icon: qwenIcon },
];

// 能力条 4 项
const CAPABILITIES = [
  { Icon: ShieldCheck, label: '生产级稳定', value: '为生产环境而生' },
  { Icon: Braces, label: 'OPENAI 兼容', value: '替换地址即可接入' },
  { Icon: Zap, label: '智能路由', value: '最高节省 50% 成本' },
  { Icon: ServerCog, label: '自愈保障', value: '上游波动自动切换' },
];

// 打字机两段
const PHRASES = [
  [{ t: '一套 API，\n畅连所有 AI' }, { t: '。', mint: 1 }],
  [
    { t: '自建' },
    { t: '算力', mint: 1 },
    { t: '集群' },
    { t: '\n' },
    { t: 'H200 / B300', sub: 1 },
  ],
];

// ---- 工具 ----
const getStoredUser = () => {
  if (typeof window === 'undefined') return null;
  try {
    const raw = localStorage.getItem('user');
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
};

const prefersReducedMotion = () =>
  typeof window !== 'undefined' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

const flattenPhrase = (segments) => {
  const chars = [];
  segments.forEach((seg) => {
    if (seg.t === '\n') {
      chars.push({ ch: '\n', mint: false, sub: false });
      return;
    }
    seg.t.split('').forEach((ch) => {
      chars.push({ ch, mint: !!seg.mint, sub: !!seg.sub });
    });
  });
  return chars;
};

const getUserDisplayName = (user) =>
  user?.username || user?.email || user?.display_name || 'User';

const HeaderUserMenu = ({ user, onLogout, mobile = false, tabIndex }) => {
  const displayName = getUserDisplayName(user);
  const initial = String(displayName).trim().slice(0, 1).toUpperCase() || 'U';

  return (
    <div className={`header-user-menu ${mobile ? 'header-user-menu--mobile' : ''}`}>
      <button
        type='button'
        className='header-user-trigger'
        tabIndex={tabIndex}
        aria-haspopup='menu'
      >
        <span className='header-user-avatar' aria-hidden='true'>
          {initial}
        </span>
        <span className='header-user-name'>{displayName}</span>
      </button>
      <div className='header-user-dropdown' role='menu'>
        <button type='button' onClick={onLogout} tabIndex={tabIndex} role='menuitem'>
          <LogOut size={15} aria-hidden='true' />
          <span>退出登录</span>
        </button>
      </div>
    </div>
  );
};

const SunIcon = () => (
  <svg
    className='icon-sun'
    viewBox='0 0 24 24'
    fill='none'
    stroke='currentColor'
    strokeWidth={2}
    strokeLinecap='round'
    aria-hidden='true'
  >
    <circle cx='12' cy='12' r='4' />
    <path d='M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4' />
  </svg>
);

const MoonIcon = () => (
  <svg
    className='icon-moon'
    viewBox='0 0 24 24'
    fill='none'
    stroke='currentColor'
    strokeWidth={2}
    strokeLinecap='round'
    strokeLinejoin='round'
    aria-hidden='true'
  >
    <path d='M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z' />
  </svg>
);

const TelegramIcon = () => (
  <svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'>
    <path d='M21.9 4.4 2.8 11.9c-.95.38-.9 1.66.08 1.98l4.76 1.52 1.83 5.72c.28.86 1.37 1.03 1.93.34l2.53-3.18 4.98 3.67c.74.55 1.8.14 2.01-.75l3.06-15.8c.2-1.03-.84-1.86-1.78-1.44ZM8.6 14.1l8.7-7.3c.22-.18.45.16.27.36l-7.2 6.9-.3 3.06-1.45-3.02Z' />
  </svg>
);

const TopIcon = () => (
  <svg
    viewBox='0 0 24 24'
    fill='none'
    stroke='currentColor'
    strokeWidth={2}
    strokeLinecap='round'
    strokeLinejoin='round'
    aria-hidden='true'
  >
    <path d='M12 19V5M5 12l7-7 7 7' />
  </svg>
);

// ---- 品牌锁 ----
const BrandLockup = ({ size = 32 }) => (
  <Link className='brand-lockup' to='/' aria-label='AllRouter.AI 首页'>
    <img src={logoSvg} alt='' width={size} height={size} />
    <span>AllRouter.AI</span>
  </Link>
);

// ---- 主题切换（本页独立体系：allrouter-theme + html[data-theme]） ----
const ThemeToggle = ({ onToggle, mobile = false }) => (
  <button
    type='button'
    className={`theme-toggle ${mobile ? 'theme-toggle--mobile' : ''}`}
    aria-label='切换深色 / 浅色模式'
    title='切换深色 / 浅色模式'
    onClick={onToggle}
  >
    <span className='theme-toggle__thumb'>
      <SunIcon />
      <MoonIcon />
    </span>
  </button>
);

// ---- 视频 / HLS 层 ----
const VideoLayer = () => {
  const videoRef = useRef(null);
  const [fallback, setFallback] = useState(false);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    let hls = null;
    let destroyed = false;
    const play = () => {
      video.play().catch(() => {});
    };

    if (video.canPlayType('application/vnd.apple.mpegurl')) {
      // Safari 原生 HLS
      video.src = HLS_SRC;
      video.addEventListener('loadedmetadata', play, { once: true });
    } else {
      // 其它浏览器：动态 import vendor 的 hls.light.js（ESM）
      const url = HLS_VENDOR_PATH;
      import(/* @vite-ignore */ url)
        .then((mod) => {
          if (destroyed) return;
          const Hls = mod?.default || mod?.Hls;
          if (!Hls || !Hls.isSupported()) {
            setFallback(true);
            return;
          }
          hls = new Hls({ enableWorker: false, lowLatencyMode: false });
          hls.loadSource(HLS_SRC);
          hls.attachMedia(video);
          hls.on(Hls.Events.MANIFEST_PARSED, play);
          hls.on(Hls.Events.ERROR, (_evt, data) => {
            if (data?.fatal) setFallback(true);
          });
        })
        .catch(() => {
          if (!destroyed) setFallback(true);
        });
    }

    return () => {
      destroyed = true;
      video.removeEventListener('loadedmetadata', play);
      if (hls) hls.destroy();
    };
  }, []);

  return (
    <div
      className={`video-layer ${fallback ? 'video-layer--fallback' : ''}`}
      aria-hidden='true'
    >
      <video ref={videoRef} muted loop playsInline preload='auto' />
      <div className='video-noise' />
    </div>
  );
};

// ---- 顶部 Header + 移动菜单 ----
const Header = ({
  menuOpen,
  setMenuOpen,
  onToggleTheme,
  targets,
  currentUser,
  isLoggedIn,
  onLogout,
}) => (
  <>
    <header className='site-header'>
      <BrandLockup />
      <nav className='desktop-nav' aria-label='主导航'>
        {NAV_ITEMS.map((item) => (
          <Link key={item.kind} to={targets[item.kind]}>
            {item.label}
          </Link>
        ))}
      </nav>
      <div className='header-actions'>
        <ThemeToggle onToggle={onToggleTheme} />
        {isLoggedIn ? (
          <HeaderUserMenu user={currentUser} onLogout={onLogout} />
        ) : (
          <Link className='login-link' to={targets.login}>
            登录
          </Link>
        )}
        <Link className='header-cta' to={targets.console}>
          获取 API Key
          <ArrowRight size={16} aria-hidden='true' />
        </Link>
      </div>
      <button
        className='mobile-menu-button'
        type='button'
        aria-label={menuOpen ? '关闭菜单' : '打开菜单'}
        aria-expanded={menuOpen}
        onClick={() => setMenuOpen((o) => !o)}
        title={menuOpen ? '关闭菜单' : '打开菜单'}
      >
        {menuOpen ? <X aria-hidden='true' /> : <Menu aria-hidden='true' />}
      </button>
    </header>
    <div
      className={`mobile-menu ${menuOpen ? 'mobile-menu--open' : ''}`}
      aria-hidden={!menuOpen}
    >
      <nav aria-label='移动端导航'>
        {NAV_ITEMS.map((item, idx) => (
          <Link
            key={item.kind}
            to={targets[item.kind]}
            onClick={() => setMenuOpen(false)}
            style={{ '--menu-index': idx }}
            tabIndex={menuOpen ? 0 : -1}
          >
            <span>0{idx + 1}</span>
            {item.label}
            <ArrowRight size={22} aria-hidden='true' />
          </Link>
        ))}
      </nav>
      <div className='mobile-menu-footer'>
        <span>ALL MODELS. ONE ROUTE.</span>
        {isLoggedIn ? (
          <HeaderUserMenu
            user={currentUser}
            onLogout={onLogout}
            mobile
            tabIndex={menuOpen ? 0 : -1}
          />
        ) : (
          <Link to={targets.console} tabIndex={menuOpen ? 0 : -1}>
            免费开始构建
          </Link>
        )}
      </div>
    </div>
  </>
);

// ---- 打字机 h1 ----
const Typewriter = ({ introActive }) => {
  const h1Ref = useRef(null);
  const reduced = useRef(prefersReducedMotion());
  // 仅在挂载时探测一次 intro 是否存在（与静态页 hero-effects.js 一致，
  // 避免 intro 完成后 seen 翻转导致打字机重启）
  const introActiveAtMount = useRef(introActive);

  useEffect(() => {
    const h1 = h1Ref.current;
    if (!h1) return undefined;

    const caret = document.createElement('span');
    caret.className = 'tw-caret';
    caret.setAttribute('aria-hidden', 'true');

    const render = (chars, n) => {
      h1.innerHTML = '';
      for (let i = 0; i < n; i++) {
        const c = chars[i];
        if (c.ch === '\n') {
          h1.appendChild(document.createElement('br'));
          continue;
        }
        if (c.sub) {
          const s = document.createElement('span');
          s.className = 'tw-sub';
          s.textContent = c.ch;
          h1.appendChild(s);
        } else if (c.mint) {
          const s = document.createElement('span');
          s.textContent = c.ch;
          h1.appendChild(s);
        } else {
          h1.appendChild(document.createTextNode(c.ch));
        }
      }
      h1.appendChild(caret);
    };

    const phrases = PHRASES.map(flattenPhrase);
    const first = phrases[0];

    if (reduced.current) {
      render(first, first.length);
      return () => {};
    }

    let pi = 0;
    let pendingTimeout = null;
    let cancelled = false;

    const typeStep = (chars, i) => {
      if (cancelled) return;
      render(chars, i);
      if (i < chars.length) {
        pendingTimeout = window.setTimeout(() => typeStep(chars, i + 1), 95);
      } else {
        pendingTimeout = window.setTimeout(
          () => eraseStep(chars, chars.length),
          2400,
        );
      }
    };
    const eraseStep = (chars, i) => {
      if (cancelled) return;
      render(chars, i);
      if (i > 0) {
        pendingTimeout = window.setTimeout(() => eraseStep(chars, i - 1), 45);
      } else {
        pi = (pi + 1) % phrases.length;
        pendingTimeout = window.setTimeout(
          () => typeStep(phrases[pi], 0),
          500,
        );
      }
    };

    const startDelay = introActiveAtMount.current ? 3000 : 500;
    pendingTimeout = window.setTimeout(() => typeStep(phrases[0], 0), startDelay);

    return () => {
      cancelled = true;
      if (pendingTimeout) window.clearTimeout(pendingTimeout);
    };
  }, []);

  return (
    <h1 id='hero-title' ref={h1Ref}>
      一套 API，<br />
      畅连所有 AI<span>。</span>
    </h1>
  );
};

// ---- 模型 marquee ----
const ModelMarquee = ({ pricingTarget }) => (
  <section className='model-ecosystem' aria-label='支持全球主流开源模型'>
    <div className='model-ecosystem__label'>
      <span>
        <i />
        支持全球主流开源模型
      </span>
      <b>12+ MODELS</b>
    </div>
    <div className='model-marquee'>
      <div className='model-marquee__track'>
        {[0, 1].map((group) => (
          <div
            key={group}
            className='model-marquee__group'
            aria-hidden={group === 1}
          >
            {MODELS.map((model) => (
              <Link
                key={`${group}-${model.name}`}
                className='model-item'
                to={pricingTarget}
                tabIndex={group === 1 ? -1 : 0}
                title={`查看 ${model.name} 模型`}
              >
                <img src={model.icon} alt='' width={18} height={18} />
                <span>{model.name}</span>
              </Link>
            ))}
          </div>
        ))}
      </div>
    </div>
  </section>
);

// ---- 算力卡片（CSS 已隐藏，仍渲染保留细节） ----
const ComputeCard = () => (
  <aside className='compute-card' aria-label='自建算力集群'>
    <svg
      className='compute-card__circuit'
      viewBox='0 0 320 184'
      aria-hidden='true'
    >
      <path d='M18 48h44l14 14h38' />
      <path d='M18 134h58l14-14h34' />
      <path d='M204 28v24l-16 16v18' />
      <path d='M232 28v34l-12 12v20' />
      <path d='M258 154v-25l-13-13v-18' />
      <circle cx='76' cy='62' r='2' />
      <circle cx='90' cy='120' r='2' />
      <circle cx='204' cy='52' r='2' />
      <circle cx='258' cy='129' r='2' />
    </svg>
    <div className='compute-card__bracket' aria-hidden='true'>
      <i />
      <i />
      <i />
    </div>
    <div className='compute-card__topline'>
      <span>[ AI ACCELERATOR ]</span>
      <span className='compute-card__status'>
        <i /> LIVE
      </span>
    </div>
    <div className='compute-card__body'>
      <div className='compute-card__copy'>
        <span>ALLROUTER COMPUTE</span>
        <h2>
          自建<span className='font-serif italic'>算力</span>集群
        </h2>
        <p>H200 / B300 / ARM CPU</p>
      </div>
      <div className='compute-card__core' aria-hidden='true'>
        <i className='compute-card__core-ring' />
        <i className='compute-card__core-chip' />
        <span>H200</span>
        <small>HBM3e</small>
      </div>
    </div>
    <div className='compute-card__footer'>
      <span>稳定</span>
      <span>高精度</span>
      <span>不降智</span>
    </div>
    <div className='compute-card__contacts' aria-hidden='true'>
      {Array.from({ length: 9 }, (_, i) => (
        <i key={i} />
      ))}
    </div>
    <span className='compute-card__edge-label' aria-hidden='true'>
      H200 · B300 · ARM
    </span>
  </aside>
);

// ---- 能力条 ----
const CapabilityRail = () => (
  <div className='capability-rail'>
    <div className='capability-rail__items'>
      {CAPABILITIES.map(({ Icon, label, value }) => (
        <div key={label} className='capability-item'>
          <Icon size={18} strokeWidth={1.5} aria-hidden='true' />
          <span>
            <b>{label}</b>
            <small>{value}</small>
          </span>
        </div>
      ))}
    </div>
  </div>
);

// ---- Hero 主体 ----
const Hero = ({
  onToggleTheme,
  onReplay,
  targets,
  introActive,
  currentUser,
  isLoggedIn,
  onLogout,
}) => {
  const [menuOpen, setMenuOpen] = useState(false);
  const heroRef = useRef(null);

  // 移动菜单：锁滚动 + Esc 关闭
  useEffect(() => {
    document.body.classList.toggle('menu-open', menuOpen);
    const onKey = (e) => {
      if (e.key === 'Escape') setMenuOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => {
      document.body.classList.remove('menu-open');
      window.removeEventListener('keydown', onKey);
    };
  }, [menuOpen]);

  // pointer 跟随光晕（reduced-motion 跳过）
  useEffect(() => {
    const el = heroRef.current;
    if (!el || prefersReducedMotion()) return undefined;
    const onMove = (e) => {
      const x = (e.clientX / window.innerWidth - 0.5) * 14;
      const y = (e.clientY / window.innerHeight - 0.5) * 10;
      el.style.setProperty('--pointer-x', `${x}px`);
      el.style.setProperty('--pointer-y', `${y}px`);
    };
    window.addEventListener('pointermove', onMove, { passive: true });
    return () => window.removeEventListener('pointermove', onMove);
  }, []);

  return (
    <main className='hero' ref={heroRef}>
      <VideoLayer />

      <div className='hero-overlay hero-overlay--left' aria-hidden='true' />
      <div className='hero-overlay hero-overlay--bottom' aria-hidden='true' />
      <div className='hero-grid' aria-hidden='true' />

      <svg className='central-glow' viewBox='0 0 900 220' aria-hidden='true'>
        <defs>
          <filter id='glow-blur' x='-20%' y='-100%' width='140%' height='300%'>
            <feGaussianBlur stdDeviation='25' />
          </filter>
          <linearGradient id='glow-color' x1='0' y1='0' x2='1' y2='0'>
            <stop offset='0' stopColor='#17372d' stopOpacity='0' />
            <stop offset='0.48' stopColor='#68e4de' stopOpacity='0.48' />
            <stop offset='1' stopColor='#17372d' stopOpacity='0' />
          </linearGradient>
        </defs>
        <ellipse
          cx='450'
          cy='110'
          rx='390'
          ry='22'
          fill='url(#glow-color)'
          filter='url(#glow-blur)'
        />
      </svg>

      <Header
        menuOpen={menuOpen}
        setMenuOpen={setMenuOpen}
        onToggleTheme={onToggleTheme}
        targets={targets}
        currentUser={currentUser}
        isLoggedIn={isLoggedIn}
        onLogout={onLogout}
      />

      <section className='hero-content' aria-labelledby='hero-title'>
        <div className='hero-copy'>
          <div className='hero-eyebrow'>
            <i />
            <span>统一大模型网关 · 稳定运行中</span>
          </div>

          <Typewriter introActive={introActive} />

          <p className='hero-description'>
            在 OpenAI、Claude、Llama 及 50+
            模型间即时切换。通过智能路由与自建算力，为每次调用选择更稳、更快、更划算的路径。
          </p>

          <div className='hero-buttons'>
            <Link className='primary-cta' to={targets.console}>
              免费开始构建
              <ArrowRight size={18} strokeWidth={2} aria-hidden='true' />
            </Link>
            <a className='secondary-cta' href={targets.docs} rel='noreferrer'>
              <Play size={15} fill='currentColor' aria-hidden='true' />
              阅读文档
            </a>
          </div>

          <ModelMarquee pricingTarget={targets.pricing} />
        </div>

        <div className='compute-card-wrap'>
          <ComputeCard />
        </div>
      </section>

      <CapabilityRail />

      <button
        className='replay-button'
        type='button'
        onClick={onReplay}
        title='重播开场动画'
      >
        <RotateCcw size={15} aria-hidden='true' />
        <span>重播开场</span>
      </button>
    </main>
  );
};

// ---- Intro 开场序列 ----
const Intro = ({ runId, onComplete }) => {
  const [skipped, setSkipped] = useState(false);

  useEffect(() => {
    setSkipped(false);
    const t = window.setTimeout(onComplete, 3100);
    return () => window.clearTimeout(t);
  }, [onComplete, runId]);

  return (
    <section
      className={`intro-sequence ${skipped ? 'intro-sequence--skipped' : ''}`}
      aria-label='AllRouter 能力简介'
    >
      <div className='intro-grid' aria-hidden='true' />
      <div className='intro-brand'>
        <BrandLockup size={25} />
      </div>
      {INTRO_STEPS.map((step, idx) => (
        <div
          key={`${runId}-${step.index}`}
          className='intro-step'
          style={{ '--step-delay': `${idx * 0.9}s` }}
        >
          <span>{step.index} / 03</span>
          <h2>{step.title}</h2>
          <p>{step.caption}</p>
        </div>
      ))}
      <div className='intro-progress' aria-hidden='true'>
        <i />
      </div>
      <button
        className='skip-intro'
        type='button'
        onClick={() => {
          setSkipped(true);
          window.setTimeout(onComplete, 360);
        }}
      >
        跳过开场
        <ArrowRight size={14} strokeWidth={1.8} aria-hidden='true' />
      </button>
    </section>
  );
};

// ---- 页脚 ----
// 链接体系与默认首页（pageTheme1/2）保持一致：产品/资源/帮助中心三列。
const SiteFooter = ({
  targets,
  docsHref,
  apiReferenceHref,
  communityHref,
  footerHtml,
}) => (
  <footer className='site-footer' aria-label='页脚'>
    <div className='footer-cta'>
      <h2>准备好优化您的 AI 工作流了吗？</h2>
      <p>加入 2,000+ 开发者，开始享受更稳定、更廉价的大模型服务。</p>
      <Link className='footer-cta__btn' to={targets.console}>
        免费开始构建
      </Link>
    </div>
    <div className='footer-main'>
      <div className='footer-brand'>
        <BrandLockup />
        <p>统一 AI 接入网关，为团队提供模型接入、路由、计费与治理能力。</p>
      </div>
      <nav className='footer-col' aria-label='产品'>
        <b>产品</b>
        <Link to={targets.console}>控制面板</Link>
        <Link to={targets.pricing}>模型广场</Link>
        <Link to='/about'>关于平台</Link>
      </nav>
      <nav className='footer-col' aria-label='资源'>
        <b>资源</b>
        <a href={docsHref} target='_blank' rel='noreferrer'>
          文档
        </a>
        <a href={apiReferenceHref} target='_blank' rel='noreferrer'>
          API 参考
        </a>
        <a href={communityHref} target='_blank' rel='noreferrer'>
          社区
        </a>
        <a
          href={`https://status.${String(systemName).toLowerCase()}/`}
          target='_blank'
          rel='noreferrer'
        >
          系统状态
        </a>
      </nav>
      <nav className='footer-col' aria-label='帮助中心'>
        <b>帮助中心</b>
        <a
          href='https://github.com/fedimoss/allrouter'
          target='_blank'
          rel='noreferrer'
        >
          项目仓库
        </a>
        <a
          href='https://github.com/fedimoss/allrouter/issues'
          target='_blank'
          rel='noreferrer'
        >
          问题反馈
        </a>
        <a href={`mailto:support@${String(systemName).toLowerCase()}`}>
          联系我们
        </a>
      </nav>
    </div>
    <div
      className='footer-bottom'
      dangerouslySetInnerHTML={{ __html: footerHtml }}
    />
  </footer>
);

// ---- 右下悬浮 FAB（客服：微信 / Telegram / QQ 动态来自 buildSupportConfig + 回到顶部） ----
// 联系方式二维码与描述走后台配置（provider_config 优先，否则 status，最后 localStorage 回退），
// 与项目默认首页的 FloatingSupport 数据源一致；为空则隐藏对应按钮。
const WechatIcon = () => (
  <svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'>
    <path d='M8.7 3C4.6 3 1.4 5.8 1.4 9.2c0 1.96 1.06 3.7 2.7 4.86l-.67 2.04 2.38-1.2c.85.17 1.7.26 2.6.26.2 0 .4 0 .6-.02-.13-.45-.2-.92-.2-1.4 0-3.04 2.94-5.5 6.56-5.5.18 0 .36.01.53.02C16.18 5.42 13.46 3 8.7 3Zm-2.55 4.04c.43 0 .78.34.78.78s-.35.78-.78.78a.78.78 0 0 1-.78-.78c0-.44.35-.78.78-.78Zm4.97 0c.43 0 .78.34.78.78s-.35.78-.78.78a.78.78 0 0 1-.78-.78c0-.44.35-.78.78-.78Zm4.6 2.58c-3.16 0-5.73 2.18-5.73 4.86 0 2.69 2.57 4.86 5.73 4.86.74 0 1.45-.12 2.12-.32l1.95.98-.53-1.66c1.32-.9 2.18-2.27 2.18-3.84 0-2.69-2.56-4.86-5.72-4.86Zm-1.84 2.06c.36 0 .65.28.65.64 0 .36-.29.65-.65.65a.65.65 0 0 1-.65-.65c0-.36.29-.64.65-.64Zm3.78 0c.36 0 .65.28.65.64 0 .36-.29.65-.65.65a.65.65 0 0 1-.65-.65c0-.36.29-.64.65-.64Z' />
  </svg>
);

const Fab = ({ supportConfig }) => {
  const [openContact, setOpenContact] = useState(null);
  const [showTop, setShowTop] = useState(false);
  const reduced = useRef(prefersReducedMotion());

  // 三个渠道：每项 image + 文案；任一非空即显示对应按钮
  const channels = useMemo(() => {
    const norm = (v) => String(v || '').trim();
    return [
      {
        key: 'wechat',
        label: '微信客服',
        image: norm(supportConfig?.wechatQRCode),
        text: norm(supportConfig?.wechatDesc),
        Icon: WechatIcon,
        btnClass: 'fab-wechat',
        qrAlt: '微信客服二维码',
        qrCaption: '微信扫码联系',
      },
      {
        key: 'telegram',
        label: 'Telegram 联系方式',
        image: norm(supportConfig?.telegramQRCode),
        text: norm(supportConfig?.telegramDesc),
        Icon: TelegramIcon,
        btnClass: 'fab-tg',
        qrAlt: 'Telegram 二维码',
        qrCaption: 'Telegram 扫码联系',
      },
      {
        key: 'qq',
        label: 'QQ 联系方式',
        image: norm(supportConfig?.qqQrcode),
        text: norm(supportConfig?.qqSupport),
        Icon: null,
        btnClass: 'fab-qq',
        qrAlt: 'QQ 二维码',
        qrCaption: 'QQ 扫码联系',
        btnText: 'QQ',
      },
    ].filter((c) => c.image || c.text);
  }, [supportConfig]);

  useEffect(() => {
    const onScroll = () => {
      setShowTop(window.scrollY > 360);
    };
    window.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
    return () => window.removeEventListener('scroll', onScroll);
  }, []);

  useEffect(() => {
    if (openContact === null) return undefined;
    const onDocClick = () => setOpenContact(null);
    document.addEventListener('click', onDocClick);
    return () => document.removeEventListener('click', onDocClick);
  }, [openContact]);

  const scrollTop = () => {
    window.scrollTo({ top: 0, behavior: reduced.current ? 'auto' : 'smooth' });
  };

  if (channels.length === 0 && !showTop) return null;

  return (
    <div className='fab'>
      {channels.map((c) => (
        <div className='fab-contact' key={c.key}>
          <button
            className={`fab-btn ${c.btnClass}`}
            type='button'
            aria-label={c.label}
            onClick={(e) => {
              e.stopPropagation();
              setOpenContact((cur) => (cur === c.key ? null : c.key));
            }}
          >
            {c.Icon ? <c.Icon /> : c.btnText}
          </button>
          <div
            className='fab-qr'
            style={{ visibility: openContact === c.key ? 'visible' : 'hidden' }}
          >
            {c.image ? (
              <img src={c.image} alt={c.qrAlt} loading='lazy' />
            ) : null}
            {c.text ? <span>{c.text}</span> : <span>{c.qrCaption}</span>}
          </div>
        </div>
      ))}
      <button
        className={`fab-btn fab-top ${showTop ? 'fab-top--show' : ''}`}
        type='button'
        aria-label='回到顶部'
        onClick={scrollTop}
      >
        <TopIcon />
      </button>
    </div>
  );
};

const usePageTheme3NavState = () => {
  const [statusState] = useContext(StatusContext);
  const [userState, userDispatch] = useContext(UserContext);
  const navigate = useNavigate();

  const currentUser = userState?.user || getStoredUser();
  const isLoggedIn = Boolean(currentUser?.id);

  const headerNavModules = useMemo(() => {
    const cfg = statusState?.status?.HeaderNavModules;
    if (!cfg) return null;
    try {
      const modules = JSON.parse(cfg);
      if (typeof modules.pricing === 'boolean') {
        modules.pricing = { enabled: modules.pricing, requireAuth: false };
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

  const docsLink = statusState?.status?.docs_link || '';
  const docsHref = docsLink || withBrowserBaseUrl('/zh/docs');

  const targets = useMemo(
    () => ({
      home: '/',
      token: isLoggedIn ? '/token' : '/login',
      pricing: !isLoggedIn && pricingRequireAuth ? '/login' : '/pricing',
      wallet: isLoggedIn ? '/topup' : '/login',
      console: isLoggedIn ? '/token' : '/login',
      login: '/login',
      docs: docsHref,
    }),
    [isLoggedIn, pricingRequireAuth, docsHref],
  );

  const logout = useCallback(async () => {
    try {
      await API.get('/api/user/logout');
    } catch (error) {
      console.error('Failed to logout:', error);
    } finally {
      localStorage.removeItem('user');
      userDispatch({ type: 'logout' });
      navigate('/login');
    }
  }, [navigate, userDispatch]);

  return {
    statusState,
    currentUser,
    isLoggedIn,
    targets,
    docsHref,
    logout,
  };
};

const usePageTheme3Theme = () => {
  const [pageTheme, setPageTheme] = useState(() => {
    try {
      return localStorage.getItem(PAGE_THEME_KEY) === 'light'
        ? 'light'
        : 'dark';
    } catch {
      return 'dark';
    }
  });

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', pageTheme);
    const meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.setAttribute('content', pageTheme === 'light' ? LIGHT_BG : DARK_BG);
    try {
      localStorage.setItem(PAGE_THEME_KEY, pageTheme);
    } catch {
      /* ignore */
    }
  }, [pageTheme]);

  useEffect(() => {
    return () => {
      document.documentElement.removeAttribute('data-theme');
    };
  }, []);

  return useCallback(() => {
    setPageTheme((p) => (p === 'light' ? 'dark' : 'light'));
  }, []);
};

const PageTheme3NavOnly = ({
  targets,
  currentUser,
  isLoggedIn,
  onLogout,
  onToggleTheme,
}) => {
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    document.body.classList.toggle('menu-open', menuOpen);
    const onKey = (e) => {
      if (e.key === 'Escape') setMenuOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => {
      document.body.classList.remove('menu-open');
      window.removeEventListener('keydown', onKey);
    };
  }, [menuOpen]);

  return (
    <Header
      menuOpen={menuOpen}
      setMenuOpen={setMenuOpen}
      onToggleTheme={onToggleTheme}
      targets={targets}
      currentUser={currentUser}
      isLoggedIn={isLoggedIn}
      onLogout={onLogout}
    />
  );
};

export const PageTheme3HeaderShell = ({ children }) => {
  const {
    statusState,
    targets,
    currentUser,
    isLoggedIn,
    docsHref,
    logout,
  } = usePageTheme3NavState();
  const handleToggleTheme = usePageTheme3Theme();
  const apiReferenceHref = withBrowserBaseUrl('/zh/docs/api');
  const communityHref = withBrowserBaseUrl(
    '/zh/docs/support/community-interaction',
  );
  const footerHtml =
    statusState?.status?.footer_html ||
    `© ${new Date().getFullYear()} ${systemName}. All rights reserved.`;

  return (
    <div className='page-theme3-route-shell'>
      <PageTheme3NavOnly
        targets={targets}
        currentUser={currentUser}
        isLoggedIn={isLoggedIn}
        onLogout={logout}
        onToggleTheme={handleToggleTheme}
      />
      <div className='page-theme3-route-content'>{children}</div>
      <SiteFooter
        targets={targets}
        docsHref={docsHref}
        apiReferenceHref={apiReferenceHref}
        communityHref={communityHref}
        footerHtml={footerHtml}
      />
    </div>
  );
};

// ---- 主组件 ----
const PageTheme3Home = () => {
  const [, statusDispatch] = useContext(StatusContext);
  const {
    statusState,
    currentUser,
    isLoggedIn,
    targets,
    docsHref,
    logout,
  } = usePageTheme3NavState();
  const isMobile = useIsMobile();

  const [noticeVisible, setNoticeVisible] = useState(false);
  const [seen, setSeen] = useState(
    () =>
      prefersReducedMotion() ||
      (() => {
        try {
          return localStorage.getItem(INTRO_SEEN_KEY) === '1';
        } catch {
          return false;
        }
      })(),
  );
  const [runId, setRunId] = useState(0);
  const handleToggleTheme = usePageTheme3Theme();
  const apiReferenceHref = withBrowserBaseUrl('/zh/docs/api');
  const communityHref = withBrowserBaseUrl(
    '/zh/docs/support/community-interaction',
  );
  const supportConfig = useMemo(
    () => buildSupportConfig(statusState?.status),
    [statusState?.status],
  );
  const footerHtml =
    statusState?.status?.footer_html ||
    `© ${new Date().getFullYear()} ${systemName}. All rights reserved.`;

  // 拉取站点状态
  useEffect(() => {
    let cancelled = false;
    const refresh = async () => {
      try {
        const res = await API.get('/api/status');
        if (cancelled) return;
        const { success, data, message } = res.data || {};
        if (success) {
          statusDispatch({ type: 'set', payload: data });
          setStatusData(data);
        } else if (message) {
          showError(message);
        }
      } catch (error) {
        if (!cancelled) console.error('Failed to refresh status:', error);
      }
    };
    refresh();
    return () => {
      cancelled = true;
    };
  }, [statusDispatch]);

  // 公告
  useEffect(() => {
    const check = async () => {
      try {
        const lastCloseDate = localStorage.getItem('notice_close_date');
        const today = new Date().toDateString();
        if (lastCloseDate === today) return;
        const res = await fetchNotice();
        const { success, data } = res.data;
        if (success && data && String(data).trim() !== '') {
          setNoticeVisible(true);
        }
      } catch (error) {
        console.error('获取公告失败:', error);
      }
    };
    check();
  }, []);

  const handleReplay = useCallback(() => {
    setRunId((r) => r + 1);
    setSeen(false);
  }, []);

  const handleIntroComplete = useCallback(() => {
    try {
      localStorage.setItem(INTRO_SEEN_KEY, '1');
    } catch {
      /* ignore */
    }
    setSeen(true);
  }, []);

  const introActive = !seen;

  return (
    <div className='app-shell'>
      <NoticeModal
        visible={noticeVisible}
        onClose={() => setNoticeVisible(false)}
        isMobile={isMobile}
      />
      <Hero
        onToggleTheme={handleToggleTheme}
        onReplay={handleReplay}
        targets={targets}
        introActive={introActive}
        currentUser={currentUser}
        isLoggedIn={isLoggedIn}
        onLogout={logout}
      />
      {!seen && (
        <Intro key={runId} runId={runId} onComplete={handleIntroComplete} />
      )}
      <SiteFooter
        targets={targets}
        docsHref={docsHref}
        apiReferenceHref={apiReferenceHref}
        communityHref={communityHref}
        footerHtml={footerHtml}
      />
      <Fab supportConfig={supportConfig} />
    </div>
  );
};

export default PageTheme3Home;
