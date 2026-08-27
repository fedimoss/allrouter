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
  useRef,
  useState,
} from 'react';
import {
  ArrowRight,
  Braces,
  Play,
  RotateCcw,
  ServerCog,
  ShieldCheck,
  Zap,
} from 'lucide-react';
import {
  API,
  applyThemeColors,
  extractThemeColors,
  fetchNotice,
  getLogo,
  getSystemName,
  setStatusData,
  showError,
  withBrowserBaseUrl,
} from '../../../../helpers';
import { StatusContext } from '../../../../context/Status';
import { UserContext } from '../../../../context/User';
import { useActualTheme } from '../../../../context/Theme';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import { Modal } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import NoticeModal from '../../../../components/layout/NoticeModal';
import Hls from 'hls.js';

import './index.css';

import openaiIcon from '../../../../../public/theme/theme3/openai.svg';
import claudeIcon from '../../../../../public/theme/theme3/claude.svg';
import geminiIcon from '../../../../../public/theme/theme3/gemini.svg';
import metaIcon from '../../../../../public/theme/theme3/meta.svg';
import deepseekIcon from '../../../../../public/theme/theme3/deepseek.svg';
import huggingfaceIcon from '../../../../../public/theme/theme3/huggingface.svg';
import kimiIcon from '../../../../../public/theme/theme3/kimi.svg';
import minimaxIcon from '../../../../../public/theme/theme3/minimax.svg';
import chatglmIcon from '../../../../../public/theme/theme3/chatglm.svg';
import doubaoIcon from '../../../../../public/theme/theme3/doubao.svg';
import grokIcon from '../../../../../public/theme/theme3/grok.svg';
import qwenIcon from '../../../../../public/theme/theme3/qwen.svg';
import brandLogo from '../../../../../public/theme/theme3/allrouter-logo.svg';

// ---------------------------------------------------------------------------
// Static config
// ---------------------------------------------------------------------------

const VIDEO_SRC =
  'https://stream.mux.com/tLkHO1qZoaaQOUeVWo8hEBeGQfySP02EPS02BmnNFyXys.m3u8';
const INTRO_SEEN_KEY = 'theme3_intro_seen';

const INTRO_STEPS = [
  { index: '01', title: '一套 API', caption: '接入 OpenAI、Claude、Llama 及 50+ 模型' },
  { index: '02', title: '智能路由', caption: '自动选择更稳、更快、更划算的调用路径' },
  { index: '03', title: '自建算力集群', caption: 'H200 / B300 / ARM CPU' },
];

const MODEL_LIST = [
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

// 打字机短语：通过 t() 国际化（键见 i18n locales）。
// segment 形状：{ t: 文本, mint?: 是否高亮薄荷色, sub?: 是否副标题样式 }
// 注意：换行 '\n' 与标点 '。' / 'H200 / B300' 不参与翻译。
const buildTypewriterPhrases = (t, language) => {
  const isChinese = String(language || '')
    .toLowerCase()
    .startsWith('zh');

  return [
    // phrase 1
    [
      { t: t('一套 API，') },
      { t: '\n' },
      { t: t('畅连所有 AI') },
      { t: '', mint: true },
    ],
    // phrase 2
    [
      { t: t('自建') },
      ...(!isChinese ? [{ t: '\n' }] : []),
      { t: t('算力'), mint: true },
      { t: t('集群') },
      { t: '\n' },
      { t: 'H200 / B300', sub: true },
    ],
  ];
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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

// Typewriter hook driving the hero <h1>
function useTypewriter(titleRef, active, hasIntro, phrases) {
  const timeoutsRef = useRef([]);
  useEffect(() => {
    const el = titleRef.current;
    if (!el || !active) return undefined;
    if (!phrases || phrases.length === 0) return undefined;
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

    const render = (node, chars, n) => {
      node.innerHTML = '';
      for (let i = 0; i < n; i++) {
        const c = chars[i];
        if (c.ch === '\n') {
          node.appendChild(document.createElement('br'));
          continue;
        }
        const s = document.createElement('span');
        if (c.mint) {
          node.appendChild(s);
          s.textContent = c.ch;
        } else if (c.sub) {
          s.className = 'tw-sub';
          s.textContent = c.ch;
          node.appendChild(s);
        } else {
          node.appendChild(document.createTextNode(c.ch));
        }
      }
      const caret = document.createElement('span');
      caret.className = 'tw-caret';
      caret.setAttribute('aria-hidden', 'true');
      node.appendChild(caret);
    };

    const flat = phrases.map(flattenPhrase);
    let pi = 0;

    if (reduced) {
      render(el, flat[0], flat[0].length);
      return undefined;
    }

    const later = (fn, delay) => {
      const id = window.setTimeout(fn, delay);
      timeoutsRef.current.push(id);
    };
    const typeStep = (chars, i) => {
      render(el, chars, i);
      if (i < chars.length) later(() => typeStep(chars, i + 1), 95);
      else later(() => eraseStep(chars, chars.length), 2400);
    };
    const eraseStep = (chars, i) => {
      render(el, chars, i);
      if (i > 0) later(() => eraseStep(chars, i - 1), 45);
      else {
        pi = (pi + 1) % flat.length;
        later(() => typeStep(flat[pi], 0), 500);
      }
    };

    later(() => typeStep(flat[0], 0), hasIntro ? 2600 : 500);

    return () => {
      timeoutsRef.current.forEach((id) => window.clearTimeout(id));
      timeoutsRef.current = [];
    };
  }, [titleRef, active, hasIntro, phrases]);
}

// ---------------------------------------------------------------------------
// Sub components
// ---------------------------------------------------------------------------

const BrandLockup = ({ logo, name }) => (
  <Link className='brand-lockup' to='/' aria-label={`${name} 首页`}>
    <img src={logo} alt='' width='32' height='32' />
    <span>{name}</span>
  </Link>
);

const VideoLayer = () => {
  const videoRef = useRef(null);
  const [fallback, setFallback] = useState(false);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return undefined;
    let destroyed = false;
    let hls;
    const tryPlay = () => video.play().catch(() => undefined);

    // Prefer hls.js: desktop Chrome/Edge report canPlayType() === 'maybe' for
    // HLS but fail to play it natively (MEDIA_ERR_SRC_NOT_SUPPORTED). Only
    // fall back to the native path when MSE is unavailable (e.g. iOS Safari).
    if (Hls.isSupported()) {
      hls = new Hls({ enableWorker: false, lowLatencyMode: false });
      hls.loadSource(VIDEO_SRC);
      hls.attachMedia(video);
      hls.on(Hls.Events.MANIFEST_PARSED, tryPlay);
      hls.on(Hls.Events.ERROR, (_e, data) => {
        if (data?.fatal) setFallback(true);
      });
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      video.src = VIDEO_SRC;
      video.addEventListener('loadedmetadata', tryPlay, { once: true });
      video.addEventListener(
        'error',
        () => {
          if (!destroyed) setFallback(true);
        },
        { once: true },
      );
    } else {
      setFallback(true);
    }

    return () => {
      destroyed = true;
      video.removeEventListener('loadedmetadata', tryPlay);
      if (hls) hls.destroy();
    };
  }, []);

  return (
    <div className={`video-layer ${fallback ? 'video-layer--fallback' : ''}`} aria-hidden='true'>
      <video ref={videoRef} muted loop playsInline preload='auto' />
      <div className='video-noise' />
    </div>
  );
};

const IntroSequence = ({ logo, name, runId, onComplete,t }) => {
  useEffect(() => {
    const id = window.setTimeout(onComplete, 3100);
    return () => window.clearTimeout(id);
  }, [onComplete, runId]);

  return (
    <section className='intro-sequence' aria-label='平台能力简介'>
      <div className='intro-grid' aria-hidden='true' />
      <div className='intro-brand'>
        <BrandLockup logo={logo} name={name} />
      </div>
      {INTRO_STEPS.map((step, i) => (
        <div
          key={`${runId}-${step.index}`}
          className='intro-step'
          style={{ '--step-delay': `${i * 0.9}s` }}
        >
          <span>
            {step.index}
            {' / 03'}
          </span>
          <h2>{t(step.title)}</h2>
          <p>{t(step.caption)}</p>
        </div>
      ))}
      <div className='intro-progress' aria-hidden='true'>
        <i />
      </div>
      <button
        type='button'
        className='skip-intro'
        onClick={onComplete}
      >
        跳过开场
        <ArrowRight size={14} strokeWidth={1.8} aria-hidden='true' />
      </button>
    </section>
  );
};

const ModelEcosystem = ({t}) => (
  <section className='model-ecosystem' aria-label='支持全球主流开源模型'>
    <div className='model-ecosystem__label'>
      <span>
        <i />
        {t('支持全球主流开源模型')}
      </span>
      <b>50+ MODELS</b>
    </div>
    <div className='model-marquee'>
      <div className='model-marquee__track'>
        {[0, 1].map((group) => (
          <div key={group} className='model-marquee__group' aria-hidden={group === 1}>
            {MODEL_LIST.map((m) => (
              <Link
                key={`${group}-${m.name}`}
                className='model-item'
                to='/pricing'
                tabIndex={group === 1 ? -1 : 0}
                title={`查看 ${m.name} 模型`}
              >
                <img src={m.icon} alt='' width='18' height='18' />
                <span>{m.name}</span>
              </Link>
            ))}
          </div>
        ))}
      </div>
    </div>
  </section>
);

const ComputeCard = ({ t }) => (
  <aside className='compute-card' aria-label={t('自建算力集群')}>
    <svg className='compute-card__circuit' viewBox='0 0 320 184' aria-hidden='true'>
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
        <i />
        {' LIVE'}
      </span>
    </div>
    <div className='compute-card__body'>
      <div className='compute-card__copy'>
        <span>ALLROUTER COMPUTE</span>
        <h2>
          {t('自建')}<span className='serif-italic'>{t('算力')}</span>{t('集群')}
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
      <span>{t('稳定')}</span>
      <span>{t('高精度')}</span>
      <span>{t('不降智')}</span>
    </div>
    <div className='compute-card__contacts' aria-hidden='true'>
      {Array.from({ length: 9 }).map((_, i) => (
        <i key={i} />
      ))}
    </div>
    <span className='compute-card__edge-label' aria-hidden='true'>
      H200 · B300 · ARM
    </span>
  </aside>
);

const CapabilityRail = ({ t }) => {
  const items = [
    { icon: ShieldCheck, label: '生产级稳定', value: '为生产环境而生' },
    { icon: Braces, label: 'OPENAI 兼容', value: '替换地址即可接入' },
    { icon: Zap, label: '智能路由', value: '最高节省 50% 成本' },
    { icon: ServerCog, label: '自愈保障', value: '上游波动自动切换' },
  ];
  return (
    <div className='capability-rail'>
      <div className='capability-rail__items'>
        {items.map(({ icon: Icon, label, value }) => (
          <div key={label} className='capability-item'>
            <Icon size={18} strokeWidth={1.5} aria-hidden='true' />
            <span>
              <b>{t(label)}</b>
              <small>{t(value)}</small>
            </span>
          </div>
        ))}
      </div>
    </div>
  );
};

const SiteFooter = ({
  logo,
  name,
  docsHref,
  apiReferenceHref,
  communityHref,
  footerHtml,
  t,
}) => (
  <footer className='site-footer' aria-label='页脚'>
    <div className='footer-cta'>
      <h2>{t('准备好优化您的 AI 工作流了吗？')}</h2>
      <p>{t('加入 2,000+ 开发者，开始享受更稳定、更廉价的大模型服务。')}</p>
      <Link className='footer-cta__btn' to='/console'>
        {t('免费开始构建')}
      </Link>
    </div>
    <div className='footer-main'>
      <div className='footer-brand'>
        <Link className='brand-lockup' to='/' aria-label={`${name} 首页`}>
          <img src={logo} alt='' width='31' height='31' />
          <span>{name}</span>
        </Link>
        <p>{t('统一 AI 接入网关，为团队提供模型接入、路由、计费与治理能力。')}</p>
      </div>
      <nav className='footer-col' aria-label={t('产品')}>
        <b>{t('产品')}</b>
        <a href='#features'>{t('功能特性')}</a>
        <a href='#models'>{t('模型生态')}</a>
        <Link to='/pricing'>{t('定价')}</Link>
        <a
          href='https://github.com/fedimoss/allrouter/releases'
          target='_blank'
          rel='noreferrer'
        >
          {t('更新日志')}
        </a>
      </nav>
      <nav className='footer-col' aria-label={t('资源')}>
        <b>{t('资源')}</b>
        <a href={docsHref} target='_blank' rel='noreferrer'>
          {t('文档')}
        </a>
        <a href={apiReferenceHref} target='_blank' rel='noreferrer'>
          {t('API 参考')}
        </a>
        <a href={communityHref} target='_blank' rel='noreferrer'>
          {t('社区')}
        </a>
        <a
          href={`https://status.${name.toLowerCase()}/`}
          target='_blank'
          rel='noreferrer'
        >
          {t('系统状态')}
        </a>
      </nav>
      <nav className='footer-col' aria-label={t('帮助中心')}>
        <b>{t('帮助中心')}</b>
        <Link to='/about'>{t('关于平台')}</Link>
        <a
          href='https://github.com/fedimoss/allrouter'
          target='_blank'
          rel='noreferrer'
        >
          {t('项目仓库')}
        </a>
        <a
          href='https://github.com/fedimoss/allrouter/issues'
          target='_blank'
          rel='noreferrer'
        >
          {t('问题反馈')}
        </a>
        <a href={`mailto:support@${name.toLowerCase()}`}>{t('联系我们')}</a>
      </nav>
    </div>
    <div
      className='footer-bottom'
      dangerouslySetInnerHTML={{
        __html:
          footerHtml ||
          `© ${new Date().getFullYear()} ${name}. All rights reserved.`,
      }}
    />
  </footer>
);

// ---------------------------------------------------------------------------
// Main Home page (theme 3)
// ---------------------------------------------------------------------------

const Theme3Home = () => {
  const { t, i18n } = useTranslation();
  const [userState] = useContext(UserContext);
  const [statusState, statusDispatch] = useContext(StatusContext);
  const actualTheme = useActualTheme();
  const isMobile = useIsMobile();

  const [noticeVisible, setNoticeVisible] = useState(false);
  const [versionLogVisible, setVersionLogVisible] = useState(false);
  const [introDone, setIntroDone] = useState(() => {
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (reduced) return true;
    try {
      return window.localStorage.getItem(INTRO_SEEN_KEY) === '1';
    } catch {
      return false;
    }
  });
  const [introRunId, setIntroRunId] = useState(0);
  const [typewriterActive, setTypewriterActive] = useState(false);

  const heroTitleRef = useRef(null);
  const heroRef = useRef(null);

  const logo = getLogo() || brandLogo;
  const systemName = getSystemName() || 'AllRouter.AI';

  const docsLink = statusState?.status?.docs_link || '';
  const docsLangPrefix = i18n.language.startsWith('zh') ? 'zh' : 'en';
  const docsHref = docsLink || withBrowserBaseUrl(`/${docsLangPrefix}/docs`);
  const apiReferenceHref = withBrowserBaseUrl(`/${docsLangPrefix}/docs/api`);
  const communityHref = withBrowserBaseUrl(
    `/${docsLangPrefix}/docs/support/community-interaction`,
  );
  const footerHtml = statusState?.status?.footer_html;
  const versionLabel = statusState?.status?.version?.version || 'v2.0';

  const currentUser = userState?.user || null;
  const isLoggedIn = Boolean(currentUser?.id);

  // Fetch theme colors + status
  useEffect(() => {
    let cancelled = false;
    const init = async () => {
      try {
        const [statusRes, colorsRes] = await Promise.allSettled([
          API.get('/api/status'),
          API.get('/api/web_colors'),
        ]);
        if (!cancelled && colorsRes.status === 'fulfilled') {
          const { primaryColor, secondaryColor, buttonTextColor } =
            extractThemeColors(colorsRes.value);
          if (primaryColor || secondaryColor || buttonTextColor) {
            applyThemeColors(primaryColor, secondaryColor, buttonTextColor);
          }
        }
        if (!cancelled && statusRes.status === 'fulfilled') {
          const { success, data } = statusRes.value.data || {};
          if (success) {
            statusDispatch({ type: 'set', payload: data });
            setStatusData(data);
          }
        }
      } catch (e) {
        // ignore
      }
    };
    init();
    return () => {
      cancelled = true;
    };
  }, [statusDispatch]);

  // Notice modal
  useEffect(() => {
    const checkNoticeAndShow = async () => {
      const lastCloseDate = localStorage.getItem('notice_close_date');
      const today = new Date().toDateString();
      if (lastCloseDate !== today) {
        try {
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

  // Kick-off the typewriter once intro completes
  useEffect(() => {
    setTypewriterActive(introDone);
  }, [introDone]);

  // 打字机短语：随语言切换重新生成，i18n.language 变化时动效自动重播
  const typewriterPhrases = useMemo(
    () => buildTypewriterPhrases(t, i18n.language),
    [t, i18n.language],
  );

  useTypewriter(heroTitleRef, typewriterActive, !introDone, typewriterPhrases);

  // Hero pointer parallax
  useEffect(() => {
    const el = heroRef.current;
    if (!el || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return undefined;
    const onMove = (e) => {
      const x = (e.clientX / window.innerWidth - 0.5) * 14;
      const y = (e.clientY / window.innerHeight - 0.5) * 10;
      el.style.setProperty('--pointer-x', `${x}px`);
      el.style.setProperty('--pointer-y', `${y}px`);
    };
    window.addEventListener('pointermove', onMove, { passive: true });
    return () => window.removeEventListener('pointermove', onMove);
  }, []);

  const replayIntro = useCallback(() => {
    setIntroRunId((v) => v + 1);
    setIntroDone(false);
  }, []);

  const onIntroComplete = useCallback(() => {
    try {
      window.localStorage.setItem(INTRO_SEEN_KEY, '1');
    } catch {
      // noop
    }
    setIntroDone(true);
  }, []);

  return (
    <div className={`t3-app ${actualTheme === 'light' ? 't3-app--light' : ''}`}>
      <div className='app-shell'>
      <NoticeModal
        visible={noticeVisible}
        onClose={() => setNoticeVisible(false)}
        isMobile={isMobile}
      />
      <Modal
        title={`${versionLabel} ${t('更新日志')}`}
        visible={versionLogVisible}
        onCancel={() => setVersionLogVisible(false)}
        footer={null}
        width={520}
      >
        <div className='pb-20' style={{ whiteSpace: 'pre-wrap', lineHeight: 1.8 }}>
          {statusState?.status?.version?.log || t('暂无更新日志')}
        </div>
      </Modal>

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
              <stop offset='0' stopColor='var(--theme-primary, #17372d)' stopOpacity='0' />
              <stop offset='0.48' stopColor='var(--theme-primary, #68e4de)' stopOpacity='0.48' />
              <stop offset='1' stopColor='var(--theme-primary, #17372d)' stopOpacity='0' />
            </linearGradient>
          </defs>
          <ellipse cx='450' cy='110' rx='390' ry='22' fill='url(#glow-color)' filter='url(#glow-blur)' />
        </svg>
        <section className='hero-content' aria-labelledby='hero-title'>
          <div className='hero-copy'>
            <div className='hero-eyebrow'>
              <i />
              <span>{t('统一大模型网关 · 稳定运行中')}</span>
            </div>
            <h1 id='hero-title' ref={heroTitleRef}>
              {t('一套 API，')}
              <br />
              {t('畅连所有 AI')}
            </h1>
            <p className='hero-description'>
              {t('在 OpenAI、Claude、Llama 及 50+ 模型间即时切换。通过智能路由与自建算力，为每次调用选择更稳、更快、更划算的路径。')}
            </p>
            <div className='hero-buttons'>
              <Link className='primary-cta' to={isLoggedIn ? '/console' : '/login'}>
                {t('免费开始构建')}
                <ArrowRight size={18} strokeWidth={2} aria-hidden='true' />
              </Link>
              <a className='secondary-cta' href={docsHref} target='_blank' rel='noreferrer'>
                <Play size={15} fill='currentColor' aria-hidden='true' />
                {t('阅读文档')}
              </a>
            </div>
            <ModelEcosystem t={t} />
          </div>
          <div className='compute-card-wrap'>
            <ComputeCard t={t} />
          </div>
        </section>

        <CapabilityRail t={t} />

        <button
          type='button'
          className='replay-button'
          onClick={replayIntro}
          title='重播开场动画'
        >
          <RotateCcw size={15} aria-hidden='true' />
          <span>重播开场</span>
        </button>
      </main>

      <SiteFooter
        logo={logo}
        name={systemName}
        docsHref={docsHref}
        apiReferenceHref={apiReferenceHref}
        communityHref={communityHref}
        footerHtml={footerHtml}
        t={t}
      />

        {!introDone && (
          <IntroSequence
            key={introRunId}
            logo={logo}
            name={systemName}
            runId={introRunId}
            onComplete={onIntroComplete}
            t={t}
          />
        )}
      </div>
    </div>
  );
};

export default Theme3Home;
