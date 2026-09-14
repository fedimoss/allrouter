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
import SubscriptionsPage from '../../components/table/subscriptions';

// 服务商控制台 - 订阅管理页面。
// 复用主站的 SubscriptionsPage 组件，仅通过 props 把请求地址切换到服务商接口：
//   - plansApi: 套餐增删改查走 /api/provider/subscription/plans（后端会强制绑定当前服务商 provider_id）
//   - modelsApi: 模型候选列表走 /api/provider/subscription/models（仅返回该服务商模型广场上架模型）
//   - tableKey: 紧凑模式等本地态以 'provider-subscriptions' 独立存储，避免与主站订阅页状态互相覆盖。
const ProviderSubscriptionPage = () => {
  return (
    <div className='px-2'>
      <SubscriptionsPage
        plansApi='/api/provider/subscription/plans'
        modelsApi='/api/provider/subscription/models'
        tableKey='provider-subscriptions'
      />
    </div>
  );
};

export default ProviderSubscriptionPage;
