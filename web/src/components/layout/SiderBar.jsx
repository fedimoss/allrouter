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

import React, { useEffect, useMemo, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ChevronsLeft } from 'lucide-react';
import { Nav, Divider, Button } from '@douyinfe/semi-ui';
import { IconRadio } from '@douyinfe/semi-icons';

import { getLucideIcon } from '../../helpers/render';
import { useSidebarCollapsed } from '../../hooks/common/useSidebarCollapsed';
import { useSidebar } from '../../hooks/common/useSidebar';
import { useMinimumLoadingTime } from '../../hooks/common/useMinimumLoadingTime';
import { getLogo, getSystemName, isAdmin, isRoot, showError } from '../../helpers';
import SkeletonWrapper from './components/SkeletonWrapper';
import SidebarUserPanel from './components/SidebarUserPanel';

const routerMap = {
  home: '/',
  channel: '/console/channel',
  token: '/console/token',
  redemption: '/console/redemption',
  topup: '/console/topup',
  user: '/console/user',
  subscription: '/console/subscription',
  log: '/console/log',
  midjourney: '/console/midjourney',
  setting: '/console/setting',
  about: '/about',
  detail: '/console',
  pricing: '/pricing',
  task: '/console/task',
  models: '/console/models',
  deployment: '/console/deployment',
  playground: '/console/playground',
  personal: '/console/personal',
  oauth: '/console/oauth',
  certification: '/console/certification',
  billing: '/console/billing',
  operational: '/console/operational',
  invitation: '/console/invitation',
  exchange: '/console/exchange'
};

const SiderBar = ({ onNavigate = () => {} }) => {
  const { t } = useTranslation();
  const [collapsed, toggleCollapsed] = useSidebarCollapsed();
  const {
    isModuleVisible,
    hasSectionVisibleModules,
    loading: sidebarLoading,
  } = useSidebar();

  const showSkeleton = useMinimumLoadingTime(sidebarLoading, 200);
  const location = useLocation();

  const logo = getLogo();
  const systemName = getSystemName();

  const [selectedKeys, setSelectedKeys] = useState(['home']);
  const [chatItems, setChatItems] = useState([]);
  const [openedKeys, setOpenedKeys] = useState([]);
  const [routerMapState, setRouterMapState] = useState(routerMap);

  const dashboardItems = useMemo(() => {
    const items = [
      {
        text: t('数据看板'),
        itemKey: 'detail',
        to: '/detail',
        className:
          localStorage.getItem('enable_data_export') === 'true'
            ? ''
            : 'tableHiddle',
      },
    ];

    return items.filter((item) => isModuleVisible('console', item.itemKey));
  }, [localStorage.getItem('enable_data_export'), t, isModuleVisible]);

  const workspaceItems = useMemo(() => {
    const items = [
      {
        text: t('操练场'),
        itemKey: 'playground',
        to: '/playground',
        section: 'console',
      },
      {
        text: t('令牌管理'),
        itemKey: 'token',
        to: '/token',
        section: 'console',
      },
      {
        text: t('模型广场'),
        itemKey: 'pricing',
        to: '/pricing',
        section: 'console',
      },
    ];

    return items.filter((item) =>
      isModuleVisible(item.section, item.itemKey),
    );
  }, [t, isAdmin(), isModuleVisible]);

  const logItems = useMemo(() => {
    const items = [
      {
        text: t('使用日志'),
        itemKey: 'log',
        to: '/log',
      },
      {
        text: t('绘图日志'),
        itemKey: 'midjourney',
        to: '/midjourney',
        className:
          localStorage.getItem('enable_drawing') === 'true'
            ? ''
            : 'tableHiddle',
      },
      {
        text: t('任务日志'),
        itemKey: 'task',
        to: '/task',
        className:
          localStorage.getItem('enable_task') === 'true' ? '' : 'tableHiddle',
      },
    ];

    return items.filter((item) => isModuleVisible('console', item.itemKey));
  }, [
    localStorage.getItem('enable_drawing'),
    localStorage.getItem('enable_task'),
    t,
    isModuleVisible,
  ]);

  const financialItems = useMemo(() => {
    const items = [
      {
        text: t('钱包'),
        itemKey: 'topup',
        to: '/topup',
        badge: t('充值'),
        section: 'personal',
      },
      ...(!isAdmin()
        ? [
            {
              text: t('账单中心'),
              itemKey: 'billing',
              to: '/billing',
              section: 'personal',
            },
          ]
        : []),
      // {
      //   text: t('个人设置'),
      //   itemKey: 'personal',
      //   to: '/personal',
      //   section: 'personal',
      // },
    ];

    return items.filter((item) =>
      isModuleVisible(item.section, item.itemKey),
    );
  }, [t, isModuleVisible]);

  const revenueMerchantItems = useMemo(() => {
    const items = [
      {
        text: t('OAuth 授权'),
        itemKey: 'oauth',
      },
      {
        text: t('认证文件'),
        itemKey: 'certification',
      },
    ];

    return items.filter((item) => isModuleVisible('merchant', item.itemKey));
  }, [t, isModuleVisible]);

  const revenueMarketingItems = useMemo(() => {
    const items = [
      {
        text: t('邀请奖励'),
        itemKey: 'invitation',
      },
      {
        text: t('兑换码'),
        itemKey: 'exchange',
      },
      // {
      //   text: t('兑换码'),
      //   itemKey: 'redemption',
      //   to: '/redemption',
      //   className: isAdmin() ? '' : 'tableHiddle',
      // },
    ];

    return items.filter((item) => isModuleVisible('marketing', item.itemKey));
  }, [isAdmin(), t, isModuleVisible]);

  const adminItems = useMemo(() => {
    const items = [
      {
        text: t('渠道管理'),
        itemKey: 'channel',
        to: '/channel',
        className: isAdmin() ? '' : 'tableHiddle',
      },
      {
        text: t('订阅管理'),
        itemKey: 'subscription',
        to: '/subscription',
        className: isAdmin() ? '' : 'tableHiddle',
      },
      {
        text: t('模型管理'),
        itemKey: 'models',
        to: '/console/models',
        className: isAdmin() ? '' : 'tableHiddle',
      },
      {
        text: t('模型部署'),
        itemKey: 'deployment',
        to: '/deployment',
        className: isAdmin() ? '' : 'tableHiddle',
      },
      {
        text: t('账单管理'),
        itemKey: 'billing',
        to: '/billing',
        className: isAdmin() ? '' : 'tableHiddle',
      },
      {
        text: t('运营数据'),
        itemKey: 'operational',
        to: '/operational',
        className: isAdmin() ? '' : 'tableHiddle',
      },
      {
        text: t('兑换码管理'),
        itemKey: 'redemption',
        to: '/redemption',
        className: isAdmin() ? '' : 'tableHiddle',
      },
      {
        text: t('用户管理'),
        itemKey: 'user',
        to: '/user',
        className: isAdmin() ? '' : 'tableHiddle',
      },
      {
        text: t('系统设置'),
        itemKey: 'setting',
        to: '/setting',
        className: isRoot() ? '' : 'tableHiddle',
      },
    ];

    return items.filter((item) => isModuleVisible('admin', item.itemKey));
  }, [isAdmin(), isRoot(), t, isModuleVisible]);

  const chatMenuItems = useMemo(() => {
    const items = [
      {
        text: t('聊天'),
        itemKey: 'chat',
        items: chatItems,
      },
    ];

    return items.filter((item) => isModuleVisible('chat', item.itemKey));
  }, [chatItems, t, isModuleVisible]);

  const updateRouterMapWithChats = (chats) => {
    const newRouterMap = { ...routerMap };

    if (Array.isArray(chats) && chats.length > 0) {
      for (let i = 0; i < chats.length; i++) {
        newRouterMap[`chat${i}`] = `/console/chat/${i}`;
      }
    }

    setRouterMapState(newRouterMap);
    return newRouterMap;
  };

  useEffect(() => {
    let chats = localStorage.getItem('chats');
    if (chats) {
      try {
        chats = JSON.parse(chats);
        if (Array.isArray(chats)) {
          const nextItems = [];
          for (let i = 0; i < chats.length; i++) {
            let shouldSkip = false;
            let chat = {};
            for (const key in chats[i]) {
              const link = chats[i][key];
              if (typeof link !== 'string') continue;
              if (link.startsWith('fluent') || link.startsWith('ccswitch')) {
                shouldSkip = true;
                break;
              }
              chat.text = key;
              chat.itemKey = `chat${i}`;
              chat.to = `/console/chat/${i}`;
            }
            if (shouldSkip || !chat.text) continue;
            nextItems.push(chat);
          }
          setChatItems(nextItems);
          updateRouterMapWithChats(chats);
        }
      } catch (e) {
        showError('聊天数据解析失败');
      }
    }
  }, []);

  useEffect(() => {
    const currentPath = location.pathname;
    let matchingKey = Object.keys(routerMapState).find(
      (key) => routerMapState[key] === currentPath,
    );

    if (!matchingKey && currentPath.startsWith('/console/chat/')) {
      const chatIndex = currentPath.split('/').pop();
      matchingKey = isNaN(chatIndex) ? 'chat' : `chat${chatIndex}`;
    }

    if (matchingKey) {
      setSelectedKeys([matchingKey]);
    }
  }, [location.pathname, routerMapState]);

  useEffect(() => {
    if (collapsed) {
      document.body.classList.add('sidebar-collapsed');
    } else {
      document.body.classList.remove('sidebar-collapsed');
    }
  }, [collapsed]);

  const SELECTED_COLOR = 'var(--semi-color-text-0)';

  const renderNavItem = (item) => {
    if (item.className === 'tableHiddle') return null;

    const isSelected = selectedKeys.includes(item.itemKey);
    const textColor = isSelected ? SELECTED_COLOR : 'inherit';

    return (
      <Nav.Item
        key={item.itemKey}
        itemKey={item.itemKey}
        text={
          <span className='sidebar-nav-text'>
            <span
              className='truncate font-medium text-sm'
              style={{ color: textColor }}
            >
              {item.text}
            </span>
            {!collapsed && item.badge ? (
              <span className='sidebar-nav-badge'>{item.badge}</span>
            ) : null}
          </span>
        }
        icon={
          <div className='sidebar-icon-container flex-shrink-0'>
            {getLucideIcon(item.itemKey, isSelected)}
          </div>
        }
        className={item.className}
      />
    );
  };

  const renderSubItem = (item) => {
    if (!item.items || item.items.length === 0) {
      return renderNavItem(item);
    }

    const visibleItems = item.items.filter(
      (subItem) => subItem.className !== 'tableHiddle',
    );
    if (visibleItems.length === 0) return null;

    const isGroupSelected = visibleItems.some((subItem) =>
      selectedKeys.includes(subItem.itemKey),
    );
    const textColor = isGroupSelected ? SELECTED_COLOR : 'inherit';
    const iconKey = item.iconKey || item.itemKey;

    return (
      <Nav.Sub
        key={item.itemKey}
        itemKey={item.itemKey}
        text={
          <span
            className='truncate font-medium text-sm'
            style={{ color: textColor }}
          >
            {item.text}
          </span>
        }
        icon={
          <div className='sidebar-icon-container flex-shrink-0'>
            {getLucideIcon(iconKey, isGroupSelected)}
          </div>
        }
      >
        {visibleItems.map((subItem) => {
          const isSubSelected = selectedKeys.includes(subItem.itemKey);
          const subTextColor = isSubSelected ? SELECTED_COLOR : 'inherit';

          return (
            <Nav.Item
              key={subItem.itemKey}
              itemKey={subItem.itemKey}
              icon={<IconRadio size='14' style={{margin:'0 10px 0 24px',color: 'rgb(203 213 225 / 100%)'}} />}
              text={
                <span
                  className='truncate font-medium text-sm'
                  style={{ color: subTextColor }}
                >
                  {subItem.text}
                </span>
              }
            />
          );
        })}
      </Nav.Sub>
    );
  };

  const hasVisible = (items) =>
    items.some((item) => item.className !== 'tableHiddle');

  return (
    <div
      className='sidebar-container'
      style={{
        width: 'var(--sidebar-current-width)',
      }}
    >
      <div
        className={`sidebar-header ${collapsed ? 'sidebar-header-collapsed' : ''}`}
      >
        <Link
          to='/console'
          className='sidebar-brand'
          onClick={onNavigate}
          title={systemName}
        >
          <div className='sidebar-brand-logo'>
            {logo ? (
              <img src={logo} alt={systemName} />
            ) : (
              <div className='sidebar-brand-fallback'>
                {(systemName || 'A').slice(0, 1)}
              </div>
            )}
          </div>
          {!collapsed && <span className='sidebar-brand-text'>{systemName}</span>}
        </Link>

        <Button
          theme='borderless'
          type='tertiary'
          size='small'
          icon={
            <ChevronsLeft
              size={16}
              strokeWidth={2.5}
              color='var(--semi-color-text-2)'
              style={{
                transform: collapsed ? 'rotate(180deg)' : 'rotate(0deg)',
              }}
            />
          }
          onClick={toggleCollapsed}
          icononly
          className='sidebar-collapse-toggle'
          aria-label={t('收起侧边栏')}
        />
      </div>

      <SkeletonWrapper
        loading={showSkeleton}
        type='sidebar'
        className=''
        collapsed={collapsed}
        showAdmin={isAdmin()}
      >
        <Nav
          className='sidebar-nav'
          defaultIsCollapsed={collapsed}
          isCollapsed={collapsed}
          onCollapseChange={toggleCollapsed}
          selectedKeys={selectedKeys}
          itemStyle='sidebar-nav-item'
          hoverStyle='sidebar-nav-item:hover'
          selectedStyle='sidebar-nav-item-selected'
          renderWrapper={({ itemElement, props }) => {
            const to =
              routerMapState[props.itemKey] || routerMap[props.itemKey];

            if (!to) return itemElement;

            return (
              <Link
                style={{ textDecoration: 'none' }}
                to={to}
                onClick={onNavigate}
              >
                {itemElement}
              </Link>
            );
          }}
          onSelect={(key) => {
            if (openedKeys.includes(key.itemKey)) {
              setOpenedKeys(openedKeys.filter((k) => k !== key.itemKey));
            }

            setSelectedKeys([key.itemKey]);
          }}
          openKeys={openedKeys}
          onOpenChange={(data) => {
            setOpenedKeys(data.openKeys);
          }}
        >
          {hasSectionVisibleModules('console') && hasVisible(dashboardItems) && (
            <>
              <div className='sidebar-section'>
                {!collapsed && (
                  <div className='sidebar-group-label'>{t('Dashboard')}</div>
                )}
                {dashboardItems.map((item) => renderNavItem(item))}
              </div>
            </>
          )}

          {false && hasSectionVisibleModules('chat') && chatMenuItems.length > 0 && (
            <div className='sidebar-section'>
              {!collapsed && (
                <div className='sidebar-group-label'>{t('聊天')}</div>
              )}
              {chatMenuItems.map((item) => renderSubItem(item))}
            </div>
          )}

          {hasVisible(workspaceItems) && (
            <>
              <div className='sidebar-section'>
                {!collapsed && (
                  <div className='sidebar-group-label'>{t('Workspace')}</div>
                )}
                {workspaceItems.map((item) => renderNavItem(item))}
              </div>
            </>
          )}

          {hasVisible(logItems) && (
            <>
              <div className='sidebar-section'>
                {!collapsed && (
                  <div className='sidebar-group-label'>{t('Logs')}</div>
                )}
                {renderSubItem({
                  itemKey: 'logs',
                  text: t('操作日志'),
                  iconKey: 'log',
                  items: logItems,
                })}
              </div>
            </>
          )}

          {hasVisible(financialItems) && (
            <>
              <div className='sidebar-section'>
                {!collapsed && (
                  <div className='sidebar-group-label'>{t('Financial')}</div>
                )}
                {financialItems.map((item) => renderNavItem(item))}
              </div>
            </>
          )}

          {(hasVisible(revenueMerchantItems) ||
            hasVisible(revenueMarketingItems)) && (
            <>
              <Divider className='sidebar-divider' />
              <div className='sidebar-section'>
                {!collapsed && (
                  <div className='sidebar-group-label'>{t('Revenue')}</div>
                )}
                {hasVisible(revenueMerchantItems) &&
                  renderSubItem({
                    itemKey: 'merchant',
                    text: t('商家入驻'),
                    iconKey: 'oauth',
                    items: revenueMerchantItems,
                  })}
                {hasVisible(revenueMarketingItems) &&
                  renderSubItem({
                    itemKey: 'marketing',
                    text: t('营销活动'),
                    iconKey: 'marketing',
                    items: revenueMarketingItems,
                  })}
              </div>
            </>
          )}

          {isAdmin() && hasVisible(adminItems) && (
            <>
              <Divider className='sidebar-divider' />
              <div className='sidebar-section'>
                {!collapsed && (
                  <div className='sidebar-group-label'>
                    {t('Administrator')}
                  </div>
                )}
                {hasVisible(adminItems) &&
                  renderSubItem({
                    itemKey: 'admin',
                    text: t('管理员'),
                    iconKey: 'user',
                    items: adminItems,
                  })}
                {/* {adminItems.map((item) => renderNavItem(item))} */}
              </div>
            </>
          )}
        </Nav>
      </SkeletonWrapper>

      <SidebarUserPanel collapsed={collapsed} />
    </div>
  );
}

export default SiderBar;
