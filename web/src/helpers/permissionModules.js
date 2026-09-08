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

// 页面级模块权限目录：key 与 SiderBar 导航项的 itemKey 一致，
// 也与后端 model.MainSitePermissionModules / model.ProviderSitePermissionModules 一一对应。
// 主站"管理员"分组的可授权模块（不含 root 专属的系统设置）。
export const MAIN_PERMISSION_MODULES = [
  {
    key: 'channel',
    label: '渠道管理',
    description: '查看和管理上游渠道、密钥与模型配置。',
  },
  {
    key: 'subscription',
    label: '订阅管理',
    description: '管理订阅套餐与用户订阅。',
  },
  {
    key: 'models',
    label: '模型管理',
    description: '管理模型元信息、厂商与预填充分组。',
  },
  {
    key: 'deployment',
    label: '模型部署',
    description: '管理模型部署实例与硬件资源。',
  },
  {
    key: 'callLog',
    label: '调用日志',
    description: '查看全站用户的 API 调用日志与任务记录。',
  },
  {
    key: 'provider',
    label: '服务商管理',
    description: '管理服务商、域名与模型定价。',
  },
  {
    key: 'providerProfits',
    label: '服务商利润',
    description: '查看各服务商的利润汇总与明细。',
  },
  {
    key: 'providerWithdraw',
    label: '提现审核',
    description: '审核服务商的提现申请。',
  },
  {
    key: 'billing',
    label: '账单中心',
    description: '查看账单概览与充值记录。',
  },
  {
    key: 'operational',
    label: '运营数据',
    description: '查看全站运营看板与数据趋势。',
  },
  {
    key: 'reconciliation',
    label: '支付对账',
    description: '查看与执行支付对账。',
  },
  {
    key: 'redemption',
    label: '兑换码管理',
    description: '管理兑换码的创建、发放与删除。',
  },
  {
    key: 'questionSurvey',
    label: '问卷调查',
    description: '查看与删除问卷提交记录。',
  },
];

// 服务商站点"服务商"分组的可授权模块（不含系统设置）。
export const PROVIDER_PERMISSION_MODULES = [
  {
    key: 'provider',
    label: '服务商管理',
    description: '管理本服务商信息、域名与模型定价。',
  },
  {
    key: 'providerOperational',
    label: '运营数据',
    description: '查看本服务商的运营看板。',
  },
  {
    key: 'providerWithdraw',
    label: '提现管理',
    description: '发起、查看与取消提现申请。',
  },
  {
    key: 'providerReward',
    label: '奖励设置',
    description: '配置邀请与消费奖励规则。',
  },
  {
    key: 'providerRewardReport',
    label: '奖励报表',
    description: '查看奖励发放统计。',
  },
  {
    key: 'providerRedemption',
    label: '兑换码管理',
    description: '管理本服务商的兑换码。',
  },
  {
    key: 'providerSubscription',
    label: '订阅管理',
    description: '管理本服务商的私有订阅套餐。',
  },
  {
    key: 'providerProfits',
    label: '服务商利润',
    description: '查看本服务商的利润明细。',
  },
  {
    key: 'providerLogs',
    label: '服务商使用日志',
    description: '查看本站用户的调用日志。',
  },
  {
    key: 'providerQuestionSurvey',
    label: '问卷调查',
    description: '查看与删除本站问卷记录。',
  },
];

export const MAIN_PERMISSION_KEYS = MAIN_PERMISSION_MODULES.map((m) => m.key);
export const PROVIDER_PERMISSION_KEYS =
  PROVIDER_PERMISSION_MODULES.map((m) => m.key);

export function getPermissionModules(providerMode) {
  return providerMode ? PROVIDER_PERMISSION_MODULES : MAIN_PERMISSION_MODULES;
}

// 判断当前操作者是否可设置模块权限：
// 主站仅超级管理员(role=100)，服务商站点仅属主。
export function canSetUserPermissions(providerMode) {
  if (providerMode) {
    let user = null;
    try {
      user = JSON.parse(localStorage.getItem('user'));
    } catch (e) {
      user = null;
    }
    return user?.is_provider_owner === true;
  }
  let user = null;
  try {
    user = JSON.parse(localStorage.getItem('user'));
  } catch (e) {
    user = null;
  }
  return typeof user?.role === 'number' && user.role >= 100;
}
