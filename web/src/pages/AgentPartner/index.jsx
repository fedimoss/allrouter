import React, { useCallback, useContext, useMemo } from 'react';
import { ArrowRight } from 'lucide-react';
import {
  Network,
  BadgeCheck,
  Server,
  LayoutDashboard,
} from 'lucide-react';
import { Link, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  API,
  getLogo,
  getSystemName,
  withBrowserBaseUrl,
} from '../../helpers';
import { StatusContext } from '../../context/Status';
import { UserContext } from '../../context/User';
import { useActualTheme, useSetTheme, useTheme } from '../../context/Theme';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import ThemeToggle from '../../components/layout/headerbar/ThemeToggle';
import LanguageSelector from '../../components/layout/headerbar/LanguageSelector';
import UserArea from '../../components/layout/headerbar/UserArea';
import studentIcon from '../../../public/agency-franchise/student.png';
import mediaIcon from '../../../public/agency-franchise/media.png';
import sideBusinessIcon from '../../../public/agency-franchise/side-business.png';
import freeIcon from '../../../public/agency-franchise/free.png';

import channelImg from '../../../public/agency-franchise/channel.png';
import connectionImg from '../../../public/agency-franchise/connection.png';
import fanContentImg from '../../../public/agency-franchise/fan-content.png';
import idleOperationImg from '../../../public/agency-franchise/idle-operation.png';


const logo = getLogo();
const systemName = getSystemName();

const audienceCards = [
  {
    icon: studentIcon,
    title: '大学生',
    desc: '时间灵活，通过校园社群与同学圈推广，增加生活收入。',
    color: 'text-blue-500 bg-blue-50 dark:bg-blue-900/30',
  },
  {
    icon: mediaIcon,
    title: '自媒体从业者',
    desc: '借助粉丝基础，快速实现流量变现。',
    color: 'text-pink-500 bg-pink-50 dark:bg-pink-900/30',
  },
  {
    icon: sideBusinessIcon,
    title: '兼职或副业',
    desc: '空闲时间灵活推广，额外收入轻松获得。',
    color: 'text-purple-500 bg-purple-50 dark:bg-purple-900/30',
  },
  {
    icon: freeIcon,
    title: '自由职业者',
    desc: '无门槛入驻，自由安排推广节奏。',
    color: 'text-orange-500 bg-orange-50 dark:bg-orange-900/30',
  },
];

const qualificationCards = [
  {
    icon: channelImg,
    title: '渠道多',
    desc: '拥有微信群、社群、网站或课程等分发渠道。',
    color: 'text-emerald-500 bg-emerald-50 dark:bg-emerald-900/30',
  },
  {
    icon: connectionImg,
    title: '人脉广',
    desc: '在技术圈、开发者社群有较强影响力。',
    color: 'text-sky-500 bg-sky-50 dark:bg-sky-900/30',
  },
  {
    icon: fanContentImg,
    title: '粉丝基础',
    desc: '运营自媒体账号或社群，拥有一定粉丝量。',
    color: 'text-amber-500 bg-amber-50 dark:bg-amber-900/30',
  },
  {
    icon: idleOperationImg,
    title: '空闲多',
    desc: '有充足的时间投入推广运营工作。',
    color: 'text-rose-500 bg-rose-50 dark:bg-rose-900/30',
  },
];

const successStories = [
  {
    name: '张同学',
    role: '大学生代理',
    revenue: '¥4,500+',
    desc: '通过校园社群推广，月入 3000+',
  },
  {
    name: '李经理',
    role: '技术博主代理',
    revenue: '¥8,000+',
    desc: '利用教育资源网络，稳定月收入',
  },
  {
    name: '王总',
    role: '社群运营代理',
    revenue: '¥38,000+',
    desc: '通过粉丝流量变现，收入可观',
  },
];

const whyUsCards = [
  {
    icon: BadgeCheck,
    title: '官方统一结算',
    desc: '分成规则清晰，充值与收益链路可追踪。',
  },
  {
    icon: Server,
    title: '稳定 API 接入',
    desc: '覆盖主流模型服务，降低上游波动带来的沟通成本。',
  },
  {
    icon: Network,
    title: '支持 OEM',
    desc: '使用自有独立域名，自定义品牌LOGO与色彩系统。',
  },
  {
    icon: LayoutDashboard,
    title: '可视化收益看板',
    desc: '实时查看用户增长、充值金额与本月分成。',
  }
];

function WhyUsCard({ t, title, desc }) {
  return (
    <div className='bg-white dark:bg-[var(--landing-v2-bg-code)] rounded-xl border border-gray-100 dark:border-gray-700/50 p-5'>
      <div className='flex items-center gap-4'>
        <div className='w-3 h-3 rounded-full bg-[var(--theme-primary)] flex-shrink-0' />
        <span className='text-base font-bold text-[var(--landing-v2-text-main)]'>
          {t(title)}
        </span>
        <span className='flex-1 text-sm text-[var(--landing-v2-text-sub)] truncate'>
          {t(desc)}
        </span>
        <span className='text-[var(--landing-v2-text-sub)] flex-shrink-0 w-6 h-6 rounded-full border border-gray-200 dark:border-gray-600 flex items-center justify-center text-sm'>
          +
        </span>
      </div>
    </div>
  );
}

const AgentPartner = () => {
  const { t, i18n } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);
  const theme = useTheme();
  const setTheme = useSetTheme();
  const navigate = useNavigate();
  const isMobile = useIsMobile();

  const docsLink = statusState?.status?.docs_link || '';
  const docsLangPrefix = i18n.language.startsWith('zh') ? 'zh' : 'en';
  const docsHref = docsLink || withBrowserBaseUrl(`/${docsLangPrefix}/docs`);
  const apiReferenceHref = withBrowserBaseUrl(`/${docsLangPrefix}/docs/api`);
  const communityHref = withBrowserBaseUrl(
    `/${docsLangPrefix}/docs/support/community-interaction`,
  );

  const getStoredUser = () => {
    try {
      const raw = localStorage.getItem('user');
      return raw ? JSON.parse(raw) : null;
    } catch {
      return null;
    }
  };

  const currentUser = userState?.user || getStoredUser();
  const isLoggedIn = Boolean(currentUser?.id);
  const isSelfUseMode = statusState?.status?.self_use_mode_enabled || false;
  const consoleNavTarget = isLoggedIn ? '/console' : '/login';
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
      if (!currentUser?.id) return;
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

  const logout = useCallback(async () => {
    await API.get('/api/user/logout');
    userDispatch({ type: 'logout' });
    localStorage.removeItem('user');
    navigate('/login');
  }, [navigate, userDispatch]);

  return (
    <div className='landing-home landing-v2 landing-v2-guest-home'>
      {/* ===== Same nav as Home page ===== */}
      <nav className='landing-v2-nav landing-v2-nav-fixed'>
        <div className='landing-v2-logo'>
          <div className='landing-v2-logo-bg'>
            <img src={logo} className='landing-v2-real-logo' />
          </div>
          <span>{systemName}</span>
        </div>

        <div className='landing-v2-nav-links'>
          <Link to='/'>{t('首页')}</Link>
          <Link to={consoleNavTarget}>{t('控制台')}</Link>
          <Link to='/pricing'>{t('模型广场')}</Link>
          <Link to='/agent-partner' className='landing-v2-nav-link-active'>
            {t('代理加盟')}
          </Link>
          <a href={docsHref} target='_blank' rel='noreferrer'>
            {t('文档')}
          </a>
          <Link to='/about'>{t('关于')}</Link>
        </div>

        <div className='landing-v2-nav-actions'>
          <div className='landing-v2-nav-tools'>
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
        {/* ===== Hero Section — left/right layout ===== */}
        <section className='w-full bg-[var(--landing-v2-bg-body)] pt-28 pb-16 px-6'>
          <div className='max-w-6xl mx-auto flex flex-col md:flex-row items-center gap-12 md:gap-16'>
            {/* Left — copy */}
            <div className='flex-1 min-w-0'>
              <div className='inline-flex border border-[var(--theme-primary)]/10 items-center gap-2 px-3 py-1.5 rounded-full bg-[var(--theme-primary)]/10 text-[var(--theme-primary)] text-xs font-medium mb-6'>
                <span className='w-1.5 h-1.5 rounded-full bg-[var(--theme-primary)]' />
                {t('代理计划已开放')}
              </div>
              <h1 className='text-3xl sm:text-4xl lg:text-5xl font-extrabold text-[var(--landing-v2-text-main)] leading-tight mb-4'>
                {t('加入我们！')}
              </h1>
              <h1 className='text-3xl sm:text-4xl lg:text-5xl font-extrabold text-[var(--landing-v2-text-main)] leading-tight mb-4'>
                {t('成为合作代理商')}
              </h1>
              <p className='text-[#A6ACB0] line-height-1.5 font-[20px]'>
                {t('为开发者与企业引入稳定、低成本的大模型 API 服务，获得长期分润收益。')}
              </p>
              <div className='flex flex-wrap gap-3 mb-8 mt-8'>
                <a
                  href='#cta'
                  className='inline-flex items-center gap-1.5 px-6 py-3 rounded-lg text-white bg-gradient-to-r from-[var(--theme-primary)] to-[var(--theme-secondary)] hover:opacity-90 transition'
                >
                  {t('立即申请')}
                  <ArrowRight size={16} />
                </a>
                <a
                  href='#why-us'
                  className='inline-flex items-center gap-1.5 px-6 py-3 rounded-lg text-[var(--theme-primary)] border border-[var(--theme-primary)]/30 hover:bg-[var(--theme-primary)]/5 transition'
                >
                  {t('了解方案')}
                </a>
              </div>
              <ul className='space-y-2.5 text-sm text-[var(--landing-v2-text-sub)]'>
                {[
                  '域名与价格可定制',
                  '充值分成透明结算',
                  '教程与运营物料支持',
                ].map((item) => (
                  <li key={item} className='flex items-center gap-2'>
                    <div className='w-[14px] h-[14px] rounded-full display-inline-block items-center justify-center flex bg-[var(--theme-primary)] flex-shrink-0'>
                      <span className='w-1 h-1 rounded-full bg-white'></span>
                    </div>
                    {t(item)}
                  </li>
                ))}
              </ul>
            </div>

            {/* Right — dashboard preview */}
            <div className='flex-1 min-w-0 w-full max-w-lg'>
              <div className='rounded-2xl border border-gray-700 overflow-hidden shadow-2xl bg-[#0f172a]'>
                <div className='flex items-center gap-2 px-4 py-3 border-b border-gray-700/50'>
                  <span className='w-3 h-3 rounded-full bg-red-400' />
                  <span className='w-3 h-3 rounded-full bg-yellow-400' />
                  <span className='w-3 h-3 rounded-full bg-green-400' />
                </div>
                <p className='px-5 text-[26px] text-white font-[700] mt-4'>
                  {t('代理收益看板')}
                </p>
                <p className='px-5 pt-2 pb-2 text-xs text-gray-500'>
                  {t('实时查看推广链接、用户充值与分润结算。')}
                </p>
                <div className='px-5 pb-5 mt-6'>
                  <div className='grid grid-cols-3 gap-3 mb-4'>
                    <div className='bg-white/5 rounded-xl p-3'>
                      <p className='text-[10px] text-gray-400 mt-1'>
                        {t('本月分成')}
                      </p>
                      <p className='text-xl font-extrabold text-emerald-400'>
                        ¥8,000+
                      </p>
                    </div>
                    <div className='bg-white/5 rounded-xl p-3'>
                      <p className='text-[10px] text-gray-400 mt-1'>
                        {t('新增用户')}
                      </p>
                      <p className='text-xl font-extrabold text-sky-400'>126</p>
                    </div>
                    <div className='bg-white/5 rounded-xl p-3'>
                      <p className='text-[10px] text-gray-400 mt-1'>
                        {t('转化率')}
                      </p>
                      <p className='text-xl font-extrabold text-white'>
                        18.6%
                      </p>
                    </div>
                  </div>
                  <div className='bg-white/5 rounded-lg p-4'>
                    <div className='flex justify-between items-end h-20'>
                      {[
                        40, 65, 45, 80, 55, 90, 70, 95, 60, 85, 75, 100,
                      ].map((h, i) => (
                        <div
                          key={i}
                          className='flex-1 mx-0.5 rounded-t'
                          style={{
                            height: `${h}%`,
                            background:
                              'linear-gradient(to top, var(--theme-primary), var(--theme-secondary))',
                          }}
                        />
                      ))}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* ===== Target Audience ===== */}
        <section className='w-full py-20 px-6 bg-white dark:bg-[var(--landing-v2-bg-body)]'>
          <div className='max-w-6xl mx-auto'>
            <div className='text-center mb-12'>
              <div className='text-2xl sm:text-3xl font-extrabold text-[var(--landing-v2-text-main)] mb-3'>
                {t('适合人群')}
              </div>
              <div className='w-12 h-1 mx-auto rounded-full bg-gradient-to-r from-[var(--theme-primary)] to-[var(--theme-secondary)] mb-4' />
              <p className='text-[var(--landing-v2-text-sub)] text-sm'>
                {t('为不同群体提供灵活、可持续的推广收入机会。')}
              </p>
            </div>
            <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6'>
              {audienceCards.map((card) => (
                <article
                  key={card.title}
                  className='bg-white dark:bg-[var(--landing-v2-bg-code)] rounded-xl border border-gray-100 dark:border-gray-700/50 p-5 transition-shadow shadow-lg'
                >
                  <img alt src={card.icon} className='w-[52px]' />
                  <h3 className='text-base font-bold text-[var(--landing-v2-text-main)] mb-2'>
                    {t(card.title)}
                  </h3>
                  <p className='text-sm text-[var(--landing-v2-text-sub)] leading-relaxed'>
                    {t(card.desc)}
                  </p>
                </article>
              ))}
            </div>
          </div>
        </section>

        {/* ===== Qualification ===== */}
        <section className='w-full py-20 px-6 bg-gray-50 dark:bg-gray-900/30'>
          <div className='max-w-6xl mx-auto'>
            <div className='text-center mb-12'>
              <div className='text-2xl sm:text-3xl font-extrabold text-[var(--landing-v2-text-main)] mb-3'>
                {t('如果你有以下特点')}
              </div>
              <div className='w-12 h-1 mx-auto rounded-full bg-gradient-to-r from-[var(--theme-primary)] to-[var(--theme-secondary)] mb-4' />
              <p className='text-[var(--landing-v2-text-sub)] text-sm'>
                {t('具备其中任意一项，就可以把 {{systemName}} 代理计划做成稳定副业。', { systemName })}
              </p>
            </div>
            <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6 overflow-hidden'>
              {qualificationCards.map((card) => (
                <div
                  key={card.title}
                  style={{
                    height: '300px',
                    background: 'linear-gradient(120deg, #FFFFFF 4.39%, #F2FFFD 46.49%, #ECFFE1 92.11%)',
                  }}
                  className=' flex flex-col items-center relative rounded-xl text-center'
                >
                  <img alt src={card.icon} className='w-[100%] h-[158px]' />
                  <div className='bg-white dark:bg-[var(--landing-v2-bg-code)] rounded-xl flex-1 w-full p-5 flex flex-col items-center'>
                    <h3 className='text-base font-bold text-[var(--landing-v2-text-main)] mb-2'>
                      {t(card.title)}
                    </h3>
                    <p className='text-sm text-[var(--landing-v2-text-sub)] leading-relaxed'>
                      {t(card.desc)}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* ===== Success Stories ===== */}
        <section className='w-full py-20 px-6 bg-white dark:bg-[var(--landing-v2-bg-body)]'>
          <div className='max-w-5xl mx-auto'>
            <div className='text-center mb-12'>
              <div className='text-2xl sm:text-3xl font-extrabold text-[var(--landing-v2-text-main)] mb-3'>
                {t('成功案例')}
              </div>
              <div className='w-12 h-1 mx-auto rounded-full bg-gradient-to-r from-[var(--theme-primary)] to-[var(--theme-secondary)] mb-4' />
              <p className='text-[var(--landing-v2-text-sub)] text-sm'>
                {t('真实推广路径可复制，适合从轻量尝试开始。')}
              </p>
            </div>
            <div className='grid grid-cols-1 sm:grid-cols-3 gap-8'>
              {successStories.map((story) => (
                <div
                  key={story.name}
                  className='text-center px-4'
                >
                  <p className='text-[42px] font-[700] text-[var(--landing-v2-text-main)]'>
                    {t(story.revenue)}
                  </p>
                  <div className='w-12 h-1 mx-auto rounded-full bg-gradient-to-r from-[var(--theme-primary)] to-[var(--theme-secondary)] mb-4' />
                  <div className='mt-5'>
                    <span className='text-[17px] font-[500] text-[var(--landing-v2-text-main)]'>
                      {t(story.name)} {t(story.role)}
                    </span>
                  </div>
                  <p className='text-[12px] text-[#A6ACB0] mt-3 leading-relaxed'>
                    {t(story.desc)}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* ===== Why Choose Us ===== */}
        <section id='why-us' className='w-full py-20 px-6 bg-gray-50 dark:bg-gray-900/30'>
          <div className='max-w-2xl mx-auto'>
            <div className='text-center mb-12'>
              <div className='text-[36px] sm:text-3xl font-extrabold text-[var(--landing-v2-text-main)] mb-3'>
                {t('为什么选择我们')}
              </div>
              <div className='w-12 h-1 mx-auto rounded-full bg-gradient-to-r from-[var(--theme-primary)] to-[var(--theme-secondary)]' />
              <p className='text-[15px] text-[#A6ACB0] mt-3 leading-relaxed'>
                {t('用统一模型网关、透明结算和运营支持，降低代理推广门槛。')}
              </p>
            </div>
            <div className='space-y-3'>
              {whyUsCards.map((card) => (
                <WhyUsCard key={card.title} t={t} {...card} />
              ))}
            </div>
          </div>
        </section>

        {/* ===== CTA ===== */}
        <section id='cta' className='landing-v2-cta-section'>
          <div className='landing-v2-cta-box'>
            <div className='landing-v2-cta-box-title'>
              {t('准备好开始推广赚钱了吗？')}
            </div>
            <p>
              {t('加入代理计划，把统一 API 服务推荐给更多开发者和企业客户。')}
            </p>
            <Link
              to='/login'
              className='landing-v2-btn-primary landing-v2-btn-lg'
            >
              {t('提交合作意向表')}
            </Link>
          </div>
        </section>
      </main>

      {/* ===== Same footer as Home page ===== */}
      <footer className='landing-v2-footer'>
        <div className='landing-v2-footer-top'>
          <div className='landing-v2-footer-brand'>
            <div className='landing-v2-logo landing-v2-logo-small'>
              <img src={logo} className='landing-v2-real-logo' />
              <span>{systemName}</span>
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
                <a href='/#features'>{t('功能特性')}</a>
              </li>
              <li>
                <a href='/#models'>{t('模型生态')}</a>
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
    </div>
  );
};

export default AgentPartner;
