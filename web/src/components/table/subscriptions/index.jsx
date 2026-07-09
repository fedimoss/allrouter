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

import React, { useContext } from 'react';
import { Banner } from '@douyinfe/semi-ui';
import CardPro from '../../common/ui/CardPro';
import SubscriptionsTable from './SubscriptionsTable';
import SubscriptionsActions from './SubscriptionsActions';
import SubscriptionsDescription from './SubscriptionsDescription';
import AddEditSubscriptionModal from './modals/AddEditSubscriptionModal';
import { useSubscriptionsData } from '../../../hooks/subscriptions/useSubscriptionsData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';
import { StatusContext } from '../../../context/Status';

// SubscriptionsPage 订阅套餐管理页（Admin 与 Provider 服务商共用同一套 UI）。
// 通过 props 注入不同的接口地址与表格 key，实现"主站订阅管理"与"服务商私有订阅管理"复用：
//   - plansApi:  套餐列表/增/改/启停的接口前缀，默认主站 /api/subscription/admin/plans；
//   - modelsApi: 模型候选下拉数据接口，默认 /api/user/models；
//   - tableKey:  紧凑模式等本地态的存储 key，默认 'subscriptions'。
// 服务商页面(ProviderSubscriptionPage)传入 /api/provider/subscription/* 复用本组件。
const SubscriptionsPage = ({
  plansApi = '/api/subscription/admin/plans',
  modelsApi = '/api/user/models',
  tableKey = 'subscriptions',
}) => {
  const subscriptionsData = useSubscriptionsData({ plansApi, tableKey });
  const isMobile = useIsMobile();
  const [statusState] = useContext(StatusContext);
  const enableEpay = !!statusState?.status?.enable_online_topup;

  const {
    showEdit,
    editingPlan,
    sheetPlacement,
    closeEdit,
    refresh,
    openCreate,
    compactMode,
    setCompactMode,
    t,
  } = subscriptionsData;

  return (
    <>
      <AddEditSubscriptionModal
        visible={showEdit}
        handleClose={closeEdit}
        editingPlan={editingPlan}
        placement={sheetPlacement}
        refresh={refresh}
        t={t}
        // 将接口地址透传给弹窗，使弹窗内的模型下拉、增改请求都走对应(admin/provider)接口。
        plansApi={plansApi}
        modelsApi={modelsApi}
      />

      <CardPro
        type='type1'
        descriptionArea={
          <SubscriptionsDescription
            compactMode={compactMode}
            setCompactMode={setCompactMode}
            t={t}
          />
        }
        actionsArea={
          <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
            {/* Mobile: actions first; Desktop: actions left */}
            <div className='order-1 md:order-0 w-full md:w-auto'>
              <SubscriptionsActions openCreate={openCreate} t={t} />
            </div>
            <Banner
              type='info'
              description={t('Stripe/Creem 需在第三方平台创建商品并填入 ID')}
              closeIcon={null}
              // Mobile: banner below; Desktop: banner right
              className='!rounded-lg order-2 md:order-1'
              style={{ maxWidth: '100%' }}
            />
          </div>
        }
        paginationArea={createCardProPagination({
          currentPage: subscriptionsData.activePage,
          pageSize: subscriptionsData.pageSize,
          total: subscriptionsData.planCount,
          onPageChange: subscriptionsData.handlePageChange,
          isMobile,
          t: subscriptionsData.t,
        })}
        t={t}
      >
        <SubscriptionsTable {...subscriptionsData} enableEpay={enableEpay} />
      </CardPro>
    </>
  );
};

export default SubscriptionsPage;
