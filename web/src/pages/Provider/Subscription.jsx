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
