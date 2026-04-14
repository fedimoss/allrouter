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

import React from 'react';
import { Link } from 'react-router-dom';
import { Breadcrumb } from '@douyinfe/semi-ui';
import { useHeaderBar } from '../../../hooks/common/useHeaderBar';
import { useNotifications } from '../../../hooks/common/useNotifications';
import NoticeModal from '../NoticeModal';
import MobileMenuButton from './MobileMenuButton';
import ActionButtons from './ActionButtons';
import ThemeToggle from './ThemeToggle';
import LanguageSelector from './LanguageSelector';
import NotificationButton from './NotificationButton';
import UserArea from './UserArea';

const HeaderBar = ({ onMobileMenuToggle, drawerOpen }) => {
    const {
        userState,
        statusState,
        isMobile,
        collapsed,
        currentLang,
        isLoading,
        systemName,
        logo,
        isNewYear,
        isSelfUseMode,
        docsLink,
        isConsoleRoute,
        theme,
        pricingRequireAuth,
        logout,
        handleLanguageChange,
        handleThemeToggle,
        handleMobileMenuToggle,
        navigate,
        t,
    } = useHeaderBar({ onMobileMenuToggle, drawerOpen });

    const {
        noticeVisible,
        unreadCount,
        handleNoticeOpen,
        handleNoticeClose,
        getUnreadKeys,
    } = useNotifications(statusState);

    const isPublicRoute = !isConsoleRoute;
    const docsLangPrefix = currentLang.startsWith('zh') ? 'zh' : 'en';
    const docsHref = docsLink || `https://allrouter.ai/${docsLangPrefix}/docs`;
    const isLoggedIn = Boolean(userState?.user);
    const consoleNavTarget = isLoggedIn ? '/console' : '/login';
    const pricingNavTarget =
        !isLoggedIn && pricingRequireAuth ? '/login' : '/pricing';
    const headerClassName = isPublicRoute
        ? 'landing-v2-nav landing-v2-nav-shell'
        : 'landing-v2-nav landing-v2-nav-shell landing-v2-nav-console';

    const breadcrumbLabelMap = {
        '/console': t('数据看板'),
        '/console/playground': t('操练场'),
        '/console/token': t('令牌管理'),
        '/pricing': t('模型广场'),
        '/console/log': t('使用日志'),
        '/console/midjourney': t('绘图日志'),
        '/console/task': t('任务日志'),
        '/console/topup': t('钱包'),
        '/console/personal': t('个人设置'),
        '/console/channel': t('渠道管理'),
        '/console/subscription': t('订阅管理'),
        '/console/models': t('模型管理'),
        '/console/deployment': t('模型部署'),
        '/console/billing': t('账单管理'),
        '/console/operational': t('运营数据'),
        '/console/user': t('用户管理'),
        '/console/setting': t('系统设置'),
        '/console/redemption': t('兑换码管理'),
        '/console/oauth': t('OAuth 授权'),
        '/console/invitation': t('邀请奖励'),
        '/console/exchange': t('兑换码'),
        '/console/certification': t('认证文件'),
    };

    const pathname = location.pathname;
    const currentLabel =
        pathname.startsWith('/console/chat')
            ? t('聊天')
            : breadcrumbLabelMap[pathname] || t('控制台');
    const breadcrumbItems = [
        { label: t('首页'), to: '/' },
        { label: t('控制台'), to: '/console' },
        { label: currentLabel },
    ];

    return (
        <header className={headerClassName}>
            <NoticeModal
                visible={noticeVisible}
                onClose={handleNoticeClose}
                isMobile={isMobile}
                defaultTab={unreadCount > 0 ? 'system' : 'inApp'}
                unreadKeys={getUnreadKeys()}
            />
            {isPublicRoute ? (
                <>
            <Link to='/' className='landing-v2-logo'>
              <div className='landing-v2-logo-bg'>
                <img
                  src={logo || '/logo.png'}
                  alt={`${systemName} Logo`}
                  className='landing-v2-real-logo'
                />
              </div>
              <span>{systemName}</span>
            </Link>

                    <div className='landing-v2-nav-links'>
                        <Link to={consoleNavTarget}>{t('控制台')}</Link>
                        <Link to={pricingNavTarget}>{t('模型广场')}</Link>
                        <a href={docsHref} target='_blank' rel='noreferrer'>
                            {t('文档')}
                        </a>
                        <Link to='/about'>{t('关于')}</Link>
                    </div>

                    <div className='landing-v2-nav-actions'>
                        <div className='landing-v2-nav-tools'>
                            <NotificationButton
                                unreadCount={unreadCount}
                                onNoticeOpen={handleNoticeOpen}
                                t={t}
                            />
                            <ThemeToggle
                                theme={theme}
                                onThemeToggle={handleThemeToggle}
                                t={t}
                            />
                            <LanguageSelector
                                currentLang={currentLang}
                                onLanguageChange={handleLanguageChange}
                                t={t}
                            />
                        </div>
                        {isLoggedIn ? (
                            <UserArea
                                userState={userState}
                                isLoading={isLoading}
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
                                <Link to={consoleNavTarget} className='landing-v2-btn-primary'>
                                    {t('获取 API Key')}
                                </Link>
                            </>
                        )}
                    </div>
                </>
            ) : (
                <div className='w-full px-2'>
                    <div className='flex items-center justify-between h-16'>
                        <div className='flex items-center min-w-0 flex-1 gap-2'>
                            <MobileMenuButton
                                isConsoleRoute={isConsoleRoute}
                                isMobile={isMobile}
                                drawerOpen={drawerOpen}
                                collapsed={collapsed}
                                onToggle={handleMobileMenuToggle}
                                t={t}
                            />
                            <div className='header-console-breadcrumb-wrap hidden md:flex'>
                                <Breadcrumb
                                    separator='/'
                                    className='header-console-breadcrumb'
                                >
                                    {breadcrumbItems.map((item, index) => {
                                        const isLast = index === breadcrumbItems.length - 1;
                                        return (
                                            <Breadcrumb.Item key={`${item.label}-${index}`}>
                                                {item.to && !isLast ? (
                                                    <Link to={item.to}>{item.label}</Link>
                                                ) : (
                                                    <span className='header-console-breadcrumb-current'>
                                                        {item.label}
                                                    </span>
                                                )}
                                            </Breadcrumb.Item>
                                        );
                                    })}
                                </Breadcrumb>
                            </div>
                        </div>
                            <ActionButtons
                                isNewYear={isNewYear}
                                unreadCount={unreadCount}
                                onNoticeOpen={handleNoticeOpen}
                                theme={theme}
                                onThemeToggle={handleThemeToggle}
                                currentLang={currentLang}
                                onLanguageChange={handleLanguageChange}
                                t={t}
                            />
                        </div>
                    </div>
            )}
        </header>
    );
};

export default HeaderBar;
