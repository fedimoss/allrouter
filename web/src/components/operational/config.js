export const pagePanelClassName =
  'rounded-[30px] border border-slate-200/80 bg-white shadow-[0_20px_60px_rgba(148,163,184,0.16)] dark:border-slate-800 dark:bg-slate-900 dark:shadow-[0_20px_60px_rgba(2,6,23,0.42)]';

export const blockPanelClassName =
  'rounded-[24px] border border-slate-200/80 bg-white shadow-[0_14px_40px_rgba(148,163,184,0.12)] dark:border-slate-800 dark:bg-slate-900 dark:shadow-[0_14px_40px_rgba(2,6,23,0.32)]';

export const inputClassName =
  'w-full rounded-2xl border border-slate-200 bg-white px-4 py-3 text-sm text-slate-700 outline-none transition placeholder:text-slate-400 focus:border-cyan-300 focus:ring-4 focus:ring-cyan-100 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200 dark:placeholder:text-slate-500 dark:focus:border-cyan-500 dark:focus:ring-cyan-900/30';

export const lightButtonClassName =
  'inline-flex items-center justify-center gap-2 rounded-2xl border border-slate-200 bg-white px-4 py-3 text-sm font-medium text-slate-600 transition hover:border-cyan-300 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-50 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200 dark:hover:border-cyan-500';

export const gradientButtonStyle = {
  background: 'linear-gradient(97.63deg, #09FEF7 0%, #BAFF29 100%)',
};

export const DATE_RANGE_OPTIONS = [
  { label: '今日', value: 'day' },
  { label: '本周', value: 'week' },
  { label: '本月', value: 'month' },
  { label: '本年', value: 'year' },
];

export const USER_ADVANCED_FILTER_CONFIG = [
  {
    key: 'registerTimeRange',
    label: '注册时间范围',
    startKey: 'startTimestamp',
    endKey: 'endTimestamp',
    startPlaceholder: '开始时间',
    endPlaceholder: '结束时间',
    inputType: 'datetime-local',
  },
  {
    key: 'usedQuotaRange',
    label: '消耗金额范围',
    startKey: 'usedQuotaMin',
    endKey: 'usedQuotaMax',
    startPlaceholder: '最小消耗金额',
    endPlaceholder: '最大消耗金额',
    inputType: 'number',
  },
  {
    key: 'quotaRange',
    label: '余额范围',
    startKey: 'quotaMin',
    endKey: 'quotaMax',
    startPlaceholder: '最小余额',
    endPlaceholder: '最大余额',
    inputType: 'number',
  },
  {
    key: 'requestCountRange',
    label: '使用次数范围',
    startKey: 'requestCountMin',
    endKey: 'requestCountMax',
    startPlaceholder: '最小使用次数',
    endPlaceholder: '最大使用次数',
    inputType: 'number',
  },
];

export const USER_DASHBOARD_CARDS = [
  {
    key: 'userCount',
    title: '总用户数',
    icon: 'trend',
    valueType: 'count',
    aliases: ['total_users', 'userCount', 'user_count', 'totalUsers', 'total_user_count', 'total'],
    defaultValue: '4,592',
    footer: { trend: '+28', text: '今日新增', tone: 'positive' },
    footerTrendAliases: ['today_new_users'],
    footerText: '今日新增',
  },
  {
    key: 'newUserCount',
    title: '新增注册用户',
    icon: 'calendar',
    valueType: 'count',
    aliases: ['new_users', 'newUserCount', 'new_user_count', 'registerCount', 'register_count'],
    defaultValue: '0',
    footer: { trend: '+0%', text: '', tone: 'positive' },
    footerTrendAliases: ['new_users_trend'],
  },
  {
    key: 'activeUserCount',
    title: '活跃用户',
    icon: 'calendar',
    valueType: 'count',
    aliases: ['active_users', 'activeUserCount', 'active_user_count', 'activeUsers'],
    defaultValue: '0',
    footer: { trend: '+0%', text: '', tone: 'positive' },
    footerTrendAliases: ['active_trend'],
  },
  {
    key: 'lostUserCount',
    title: '流失用户',
    icon: 'trend',
    valueType: 'count',
    aliases: ['churned_users', 'lostUserCount', 'lost_user_count', 'churnUserCount', 'churn_user_count'],
    defaultValue: '0',
    footer: { trend: '+0%', text: '', tone: 'negative' },
    footerTrendAliases: ['churned_trend'],
  },
];

export const AGENT_DASHBOARD_CARDS = [
  {
    key: 'agentTotal',
    title: '代理商累计总数',
    icon: 'trend',
    valueType: 'count',
    defaultValue: '9,054,362',
    footer: { trend: '+28', text: '今日新增', tone: 'positive' },
  },
  {
    key: 'agentProfit',
    title: '平均分润收益',
    icon: 'calendar',
    valueType: 'text',
    defaultValue: '$ 1,214.50',
    footer: { trend: '', text: '按预计收益计算（含未结算）', tone: 'neutral' },
  },
  {
    key: 'newAgent',
    title: '新增新代理',
    icon: 'trend',
    valueType: 'count',
    defaultValue: '-4',
    footer: { trend: '较上月 -4', text: '', tone: 'negative' },
  },
  {
    key: 'activeAgent',
    title: '新增活跃代理',
    icon: 'calendar',
    valueType: 'count',
    defaultValue: '6',
    footer: { trend: '较上月 +6%', text: '', tone: 'positive' },
  },
];

export const MERCHANT_DASHBOARD_CARDS = [
  {
    key: 'merchantTotal',
    title: '总商家数',
    icon: 'trend',
    valueType: 'count',
    defaultValue: '1,054,592',
    footer: { trend: '+28', text: '今日新增', tone: 'positive' },
  },
  {
    key: 'activeMerchant',
    title: '活跃商家',
    icon: 'calendar',
    valueType: 'count',
    defaultValue: '1249',
    footer: { trend: '较上月 -102', text: '', tone: 'negative' },
  },
  {
    key: 'newMerchant',
    title: '新增商家',
    icon: 'trend',
    valueType: 'count',
    defaultValue: '698',
    footer: { trend: '较上月 +128', text: '', tone: 'positive' },
  },
  {
    key: 'lostMerchant',
    title: '流失商家',
    icon: 'topup',
    valueType: 'count',
    defaultValue: '6',
    footer: { trend: '较上月 +4', text: '', tone: 'negative' },
  },
  {
    key: 'gmv',
    title: '总交易额 (GMV)',
    icon: 'trend',
    valueType: 'text',
    defaultValue: '$ 65,125',
    footer: { trend: '较上月 +128', text: '', tone: 'positive' },
  },
  {
    key: 'products',
    title: '上架商品总数',
    icon: 'topup',
    valueType: 'count',
    defaultValue: '124',
    footer: { trend: '较上月 +46', text: '', tone: 'positive' },
  },
  {
    key: 'income',
    title: '平台累计营收',
    icon: 'trend',
    valueType: 'text',
    defaultValue: '$ 4,698,478',
    footer: { trend: '较上月 +128', text: '', tone: 'positive' },
  },
  {
    key: 'score',
    title: '平均商家评分',
    icon: 'calendar',
    valueType: 'text',
    defaultValue: '4.5',
    footer: { trend: '较上月 -1.2', text: '', tone: 'negative' },
  },
];

export const SELF_HOSTED_DASHBOARD_CARDS = [
  {
    key: 'income',
    title: '自营总营收',
    icon: 'trend',
    valueType: 'text',
    defaultValue: '$ 65,125',
    footer: { trend: '+128', text: '今日新增', tone: 'positive' },
  },
  {
    key: 'paidCustomer',
    title: '付费客户数',
    icon: 'calendar',
    valueType: 'count',
    defaultValue: '1249',
    footer: { trend: '较上月 -102', text: '', tone: 'negative' },
  },
  {
    key: 'utilization',
    title: '资源平均利用率',
    icon: 'trend',
    valueType: 'text',
    defaultValue: '74.6%',
    footer: { trend: '较上月 +6.5%', text: '', tone: 'positive' },
  },
  {
    key: 'errorRate',
    title: '服务异常率',
    icon: 'topup',
    valueType: 'text',
    defaultValue: '5.6%',
    footer: { trend: '较上月 +1.2%', text: '', tone: 'negative' },
  },
];

export const USER_COLUMNS = [
  { key: 'user', title: '用户ID', label: '用户ID', defaultChecked: true },
  { key: 'quota', title: '余额', label: '余额', defaultChecked: true, sortable: true, sortField: 'quota' },
  { key: 'requestCount', title: '使用', label: '使用', defaultChecked: true, sortable: true, sortField: 'request_count' },
  { key: 'usedQuota', title: '消耗', label: '消耗', defaultChecked: true },
  { key: 'retention', title: '留存', label: '留存', defaultChecked: true },
  { key: 'detail', title: '详情', label: '详情', defaultChecked: true },
  { key: 'topupQuota', title: '充值', label: '充值', defaultChecked: false, sortable: true, sortField: 'topup_quota' },
  { key: 'welfareQuota', title: '赠送', label: '赠送', defaultChecked: false, sortable: true, sortField: 'welfare_quota' },
  { key: 'source', title: '注册来源', label: '注册来源', defaultChecked: false },
  { key: 'registerAt', title: '注册时间', label: '注册时间', defaultChecked: false },
  { key: 'lastActiveTime', title: '最后活跃', label: '最后活跃', defaultChecked: false },
];

export const TAB_CONFIG = {
  user: {
    label: '用户',
    tableTitle: '用户数据',
    searchPlaceholder: '搜索用户名称或ID...',
    api: {
      dashboard: '/api/operation/user/dashboard',
      records: '/api/operation/user/records',
    },
    cards: USER_DASHBOARD_CARDS,
    columns: USER_COLUMNS,
    advancedFilters: USER_ADVANCED_FILTER_CONFIG,
  },
  agent: {
    label: '代理商',
    title: '代理商数据',
    subtitle: '查看代理增长、拉新、分润与活跃表现。',
    tableTitle: '代理商数据',
    searchPlaceholder: '搜索代理商 ID 或昵称',
    api: {},
    cards: AGENT_DASHBOARD_CARDS,
    columns: [],
    advancedFilters: [],
  },
  merchant: {
    label: '入驻商家',
    title: '入驻商家',
    subtitle: '追踪商家规模、商品供给和平台营收表现。',
    tableTitle: '入驻商家',
    searchPlaceholder: '搜索商家 ID 或名称',
    api: {},
    cards: MERCHANT_DASHBOARD_CARDS,
    columns: [],
    advancedFilters: [],
  },
  selfHosted: {
    label: '平台自营',
    title: '平台自营',
    subtitle: '按自营服务维度查看营收、客户和资源质量。',
    tableTitle: '平台自营',
    searchPlaceholder: '搜索服务名称或 ID',
    api: {},
    cards: SELF_HOSTED_DASHBOARD_CARDS,
    columns: [],
    advancedFilters: [],
  },
};



