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

import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { API, isAdmin, showError, timestamp2string } from '../../helpers';
import { getDefaultTime, getInitialTimestamp } from '../../helpers/dashboard';
import {
  TIME_OPTIONS,
  INVITEE_PAGE_SIZE,
} from '../../constants/dashboard.constants';
import { useIsMobile } from '../common/useIsMobile';
import { useMinimumLoadingTime } from '../common/useMinimumLoadingTime';

// 顶部统计卡片固定使用 24h 窗口的统计数据，不随"数据分析"时间筛选（7天/30天）联动
const PERF_WINDOW_TOLERANCE = 3600; // 允许与 24h 相差 1 小时
const DEFAULT_CARD_STATS = { consumeQuota: 0, consumeTokens: 0, times: 0 };

const isAbout24h = (seconds) =>
  Math.abs(seconds - 86400) <= PERF_WINDOW_TOLERANCE;

const summarizeQuotaRecords = (data) =>
  (data || []).reduce(
    (result, item) => ({
      consumeQuota: result.consumeQuota + (Number(item.quota) || 0),
      consumeTokens: result.consumeTokens + (Number(item.token_used) || 0),
      times: result.times + (Number(item.count) || 0),
    }),
    { consumeQuota: 0, consumeTokens: 0, times: 0 },
  );

export const useDashboardData = (userState, userDispatch, statusState) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const isMobile = useIsMobile();
  const initialized = useRef(false);

  // ========== 基础状态 ==========
  const [loading, setLoading] = useState(false);
  const [greetingVisible, setGreetingVisible] = useState(false);
  const [searchModalVisible, setSearchModalVisible] = useState(false);
  const showLoading = useMinimumLoadingTime(loading);

  // ========== 输入状态 ==========
  const [inputs, setInputs] = useState({
    username: '',
    token_name: '',
    model_name: '',
    start_timestamp: getInitialTimestamp(),
    end_timestamp: timestamp2string(new Date().getTime() / 1000),
    channel: '',
    data_export_default_time: '',
  });

  const [dataExportDefaultTime, setDataExportDefaultTime] =
    useState(getDefaultTime());

  // ========== 数据状态 ==========
  const [quotaData, setQuotaData] = useState([]);
  const [consumeQuota, setConsumeQuota] = useState(0);
  const [consumeTokens, setConsumeTokens] = useState(0);
  const [times, setTimes] = useState(0);
  const [pieData, setPieData] = useState([{ type: 'null', value: '0' }]);
  const [lineData, setLineData] = useState([]);
  const [modelColors, setModelColors] = useState({});
  const [displayCurrency, setDisplayCurrency] = useState({
    currency: 'USD',
    rate: 1,
    symbol: '$',
  });
  const [modelPopularRank, setModelPopularRank] = useState([]);
  const [modelQuotaRadio, setModelQuotaRadio] = useState([]);
  const [invitees, setInvitees] = useState([]);
  const [inviteesLoading, setInviteesLoading] = useState(false);
  const [inviteesTotal, setInviteesTotal] = useState(0);
  const [selectedInvitee, setSelectedInvitee] = useState(null);
  const [selectedCardUser, setSelectedCardUser] = useState(null);
  const [selectedCardStats, setSelectedCardStats] = useState(null);
  const [selectedCardLoading, setSelectedCardLoading] = useState(false);
  // self 视角 24h 卡片快照（邀请人视角使用 selectedCardStats，均为 24h 口径）
  const [selfCardStats, setSelfCardStats] = useState(null);

  // ========== 图表状态 ==========
  const [activeChartTab, setActiveChartTab] = useState('1');

  // ========== 趋势数据 ==========
  const [trendData, setTrendData] = useState({
    balance: [],
    usedQuota: [],
    requestCount: [],
    times: [],
    consumeQuota: [],
    tokens: [],
    rpm: [],
    tpm: [],
  });

  // ========== Uptime 数据 ==========
  const [uptimeData, setUptimeData] = useState([]);
  const [uptimeLoading, setUptimeLoading] = useState(false);
  const [activeUptimeTab, setActiveUptimeTab] = useState('');

  // ========== 常量 ==========
  const now = new Date();
  const isAdminUser = isAdmin();

  // ========== Panel enable flags ==========
  const apiInfoEnabled = statusState?.status?.api_info_enabled ?? true;
  const announcementsEnabled =
    statusState?.status?.announcements_enabled ?? true;
  const faqEnabled = statusState?.status?.faq_enabled ?? true;
  const uptimeEnabled = statusState?.status?.uptime_kuma_enabled ?? true;

  const hasApiInfoPanel = apiInfoEnabled;
  const hasInfoPanels = announcementsEnabled || faqEnabled || uptimeEnabled;

  // ========== Memoized Values ==========
  const timeOptions = useMemo(
    () =>
      TIME_OPTIONS.map((option) => ({
        ...option,
        label: t(option.label),
      })),
    [t],
  );

  const performanceMetrics = useMemo(() => {
    const { start_timestamp, end_timestamp } = inputs;
    const timeDiff =
      (Date.parse(end_timestamp) - Date.parse(start_timestamp)) / 60000;
    const avgRPM = isNaN(times / timeDiff)
      ? '0'
      : (times / timeDiff).toFixed(3);
    const avgTPM = isNaN(consumeTokens / timeDiff)
      ? '0'
      : (consumeTokens / timeDiff).toFixed(3);

    return { avgRPM, avgTPM, timeDiff };
  }, [times, consumeTokens, inputs.start_timestamp, inputs.end_timestamp]);

  const cardPerformanceMetrics = useMemo(() => {
    // 性能指标卡固定展示 24h 窗口数据：卡片快照仅在挂载/点刷新时按 24h 口径更新，不随图表筛选联动
    const snapshot = selectedCardUser
      ? selectedCardStats ?? DEFAULT_CARD_STATS
      : selfCardStats ?? DEFAULT_CARD_STATS;
    const { times: snapTimes, consumeTokens: snapTokens } = snapshot;
    return {
      avgRPM: isNaN(snapTimes / 1440)
        ? '0'
        : (snapTimes / 1440).toFixed(3),
      avgTPM: isNaN(snapTokens / 1440)
        ? '0'
        : (snapTokens / 1440).toFixed(3),
      timeDiff: 1440,
    };
  }, [selectedCardUser, selectedCardStats, selfCardStats]);

  const getGreeting = useMemo(() => {
    const hours = new Date().getHours();
    let greeting = '';

    if (hours >= 5 && hours < 12) {
      greeting = t('早上好');
    } else if (hours >= 12 && hours < 14) {
      greeting = t('中午好');
    } else if (hours >= 14 && hours < 18) {
      greeting = t('下午好');
    } else {
      greeting = t('晚上好');
    }

    const username = userState?.user?.username || '';
    return `👋${greeting}，${username}`;
  }, [t, userState?.user?.username]);

  // ========== 回调函数 ==========
  const handleInputChange = useCallback((value, name) => {
    if (name === 'data_export_default_time') {
      setDataExportDefaultTime(value);
      localStorage.setItem('data_export_default_time', value);
      return;
    }
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  }, []);

  const showSearchModal = useCallback(() => {
    setSearchModalVisible(true);
  }, []);

  const handleCloseModal = useCallback(() => {
    setSearchModalVisible(false);
  }, []);

  const { start_timestamp, end_timestamp, username } = inputs;
  const localStartTimestamp = Date.parse(start_timestamp) / 1000;
  const localEndTimestamp = Date.parse(end_timestamp) / 1000;

  const getDataQuery = useCallback(
    (override) => {
      const params = new URLSearchParams({
        username: (override?.username ?? username) || '',
        start_timestamp: String(
          override?.start_timestamp ?? localStartTimestamp,
        ),
        end_timestamp: String(override?.end_timestamp ?? localEndTimestamp),
        default_time: override?.default_time ?? dataExportDefaultTime,
      });
      return params.toString();
    },
    [username, localStartTimestamp, localEndTimestamp, dataExportDefaultTime],
  );

  // ========== API 调用函数 ==========
  const loadQuotaData = useCallback(
    async ({ updateStats = true, override } = {}) => {
      setLoading(true);
      try {
        let url = '';

        if (isAdminUser) {
          url = `/api/data/?${getDataQuery(override)}`;
        } else {
          url = `/api/data/self/?${getDataQuery(override)}`;
        }

        const res = await API.get(url);
        const { success, message, data } = res.data;
        if (success) {
          // 存储后端返回的展示币种信息（根据用户时区从 currency_stripe_config 表获取）
          if (updateStats) {
            setDisplayCurrency({
              currency: res.data.display_currency || 'USD',
              unit_price: res.data.display_rate || 1,
            });
          }
          setQuotaData(data);
          data.sort((a, b) => a.created_at - b.created_at);
          return data;
        } else {
          showError(message);
          return [];
        }
      } finally {
        setLoading(false);
      }
    },
    [getDataQuery, isAdminUser, now],
  );

  const loadModelPopularRank = useCallback(
    async (override) => {
      const endpoint = isAdminUser
        ? '/api/data/modelPopularRank'
        : '/api/data/self/modelPopularRank';
      const res = await API.get(`${endpoint}?${getDataQuery(override)}`);
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return [];
      }
      const nextData = data || [];
      setModelPopularRank(nextData);
      return nextData;
    },
    [getDataQuery, isAdminUser],
  );

  const loadModelQuotaRadio = useCallback(
    async (override) => {
      const endpoint = isAdminUser
        ? '/api/data/modelQuotaRadio'
        : '/api/data/self/modelQuotaRadio';
      const res = await API.get(`${endpoint}?${getDataQuery(override)}`);
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return [];
      }
      const nextData = data || [];
      setModelQuotaRadio(nextData);
      return nextData;
    },
    [getDataQuery, isAdminUser],
  );

  const loadModelData = useCallback(
    async (override) => {
      const [popularRank, quotaRadio] = await Promise.all([
        loadModelPopularRank(override),
        loadModelQuotaRadio(override),
      ]);
      return { popularRank, quotaRadio };
    },
    [loadModelPopularRank, loadModelQuotaRadio],
  );

  const loadInvitees = useCallback(
    async ({ page = 1, keyword = '' } = {}) => {
      setInviteesLoading(true);
      try {
        const response = await API.get(
          `/api/user/self/aff/records?p=${page}&page_size=${INVITEE_PAGE_SIZE}&keyword=${encodeURIComponent(keyword.trim())}`,
        );
        const result = response.data;
        if (!result.success) {
          showError(result.message);
          return [];
        }

        const records = result.data?.items || [];
        const nextInvitees = records
          .filter((record) => record?.invitee_id)
          .map((record) => ({
            id: record.invitee_id,
            username: record.invitee_name || String(record.invitee_id),
            registerTime: record.register_time,
          }));
        setInvitees(nextInvitees);
        setInviteesTotal(Number(result.data?.total || 0));
        return nextInvitees;
      } catch (err) {
        console.error(err);
        showError(t('加载失败'));
        return [];
      } finally {
        setInviteesLoading(false);
      }
    },
    [t],
  );

  // 邀请人卡片数据：固定按 24h 窗口加载（顶部卡片口径），与图表筛选窗口无关
  const loadInviteeCardData = useCallback(
    async (inviteeId) => {
      setSelectedCardLoading(true);
      try {
        const nowSec = Math.floor(new Date().getTime() / 1000);
        const query = getDataQuery({
          username: '',
          start_timestamp: nowSec - 86400,
          end_timestamp: nowSec,
        });
        const res = await API.get(
          `/api/data/self/invitee?user_id=${encodeURIComponent(inviteeId)}&${query}`,
        );
        const { success, message, user, data } = res.data;
        if (!success) {
          showError(message);
          return false;
        }
        const totals = summarizeQuotaRecords(data);
        setSelectedCardUser(user);
        setSelectedCardStats(totals);
        return true;
      } catch (err) {
        console.error(err);
        showError(t('加载失败'));
        return false;
      } finally {
        setSelectedCardLoading(false);
      }
    },
    [getDataQuery, t],
  );

  // self 视角卡片快照：由已取得的 24h 数据直接生成（挂载时复用初始加载，避免重复请求）
  const applySelfCardData = useCallback((data) => {
    setSelfCardStats(summarizeQuotaRecords(data));
  }, []);

  // self 视角卡片快照：当前图表窗口非 24h 时（如切到 7 天/30 天后点刷新），单独按 24h 拉取
  const loadSelfCard24h = useCallback(async () => {
    const nowSec = Math.floor(new Date().getTime() / 1000);
    try {
      const override = { start_timestamp: nowSec - 86400, end_timestamp: nowSec };
      const url = isAdminUser
        ? `/api/data/?${getDataQuery(override)}`
        : `/api/data/self/?${getDataQuery(override)}`;
      const res = await API.get(url);
      const { success, data } = res.data;
      if (success) {
        setSelfCardStats(summarizeQuotaRecords(data));
      }
    } catch (err) {
      console.error(err);
    }
  }, [getDataQuery, isAdminUser]);

  const selectInvitee = useCallback(
    async (invitee) => {
      // id='all' 为"全部邀请用户"虚拟选项,其余必须为具体被邀请人 ID
      if (invitee?.id !== 'all' && !invitee?.id) return false;
      // loadInviteeCardData 固定按 24h 加载，与图表筛选窗口无关
      const loaded = await loadInviteeCardData(invitee.id);
      if (loaded) {
        setSelectedInvitee(invitee);
      }
      return loaded;
    },
    [loadInviteeCardData],
  );

  const clearInvitee = useCallback(() => {
    setSelectedInvitee(null);
    setSelectedCardUser(null);
    setSelectedCardStats(null);
  }, []);

  const loadUptimeData = useCallback(async () => {
    setUptimeLoading(true);
    try {
      const res = await API.get('/api/uptime/status');
      const { success, message, data } = res.data;
      if (success) {
        setUptimeData(data || []);
        if (data && data.length > 0 && !activeUptimeTab) {
          setActiveUptimeTab(data[0].categoryName);
        }
      } else {
        showError(message);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setUptimeLoading(false);
    }
  }, [activeUptimeTab]);

  const loadUserQuotaData = useCallback(async () => {
    if (!isAdminUser) return [];
    try {
      const { start_timestamp, end_timestamp } = inputs;
      const localStartTimestamp = Date.parse(start_timestamp) / 1000;
      const localEndTimestamp = Date.parse(end_timestamp) / 1000;
      const url = `/api/data/users?start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}`;
      const res = await API.get(url);
      const { success, message, data } = res.data;
      if (success) {
        return data || [];
      } else {
        showError(message);
        return [];
      }
    } catch (err) {
      console.error(err);
      return [];
    }
  }, [inputs, isAdminUser]);

  const getUserData = useCallback(async () => {
    let res = await API.get(
      `/api/user/self?start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}`,
    );
    const { success, message, data } = res.data;
    if (success) {
      userDispatch({ type: 'login', payload: data });
      localStorage.setItem('user', JSON.stringify(data));
    } else {
      showError(message);
    }
  }, [userDispatch]);

  const refresh = useCallback(async () => {
    const data = await loadQuotaData();
    await loadUptimeData();
    // 卡片数据固定按 24h 窗口重新拉取，与图表当前筛选窗口无关
    if (selectedInvitee?.id) {
      await loadInviteeCardData(selectedInvitee.id);
    } else if (!isAbout24h(localEndTimestamp - localStartTimestamp)) {
      await loadSelfCard24h();
    } else if (data) {
      applySelfCardData(data);
    }
    return data;
  }, [
    loadQuotaData,
    loadUptimeData,
    selectedInvitee,
    loadInviteeCardData,
    loadSelfCard24h,
    applySelfCardData,
    localStartTimestamp,
    localEndTimestamp,
  ]);

  const handleSearchConfirm = useCallback(
    async (updateChartDataCallback) => {
      const [data] = await Promise.all([
        loadQuotaData({ updateStats: false }),
        loadModelData(),
      ]);
      if (data && updateChartDataCallback) {
        updateChartDataCallback(data, { updateStats: false });
      }
      // 图表筛选不影响顶部卡片，卡片仅在挂载/刷新时按 24h 口径更新
      setSearchModalVisible(false);
    },
    [loadQuotaData, loadModelData],
  );

  // ========== 快捷时间区间筛选 ==========
  // value: '24h' | '7d' | '30d'；按区间重新计算起止时间戳并刷新三接口
  const handleDateRangeChange = useCallback(
    async (value, updateChartDataCallback) => {
      if (!value) return;
      // 取整秒，避免带小数（如 1784542418.07）
      const nowSec = Math.floor(new Date().getTime() / 1000);
      let startSec = nowSec;
      let defaultTime = 'hour';
      switch (value) {
        case '24h':
          startSec = nowSec - 86400;
          defaultTime = 'hour';
          break;
        case '7d':
          startSec = nowSec - 86400 * 7;
          defaultTime = 'day';
          break;
        case '30d':
          startSec = nowSec - 86400 * 30;
          defaultTime = 'day';
          break;
        default:
          return;
      }
      const startStr = timestamp2string(startSec);
      const endStr = timestamp2string(nowSec);
      // 同步 inputs 与时间粒度，使后续 getDataQuery 生成正确参数
      setInputs((prev) => ({
        ...prev,
        start_timestamp: startStr,
        end_timestamp: endStr,
      }));
      setDataExportDefaultTime(defaultTime);
      localStorage.setItem('data_export_default_time', defaultTime);

      // 使用 override 立即触发请求（避免等待 state 更新）
      const override = {
        start_timestamp: startSec,
        end_timestamp: nowSec,
        default_time: defaultTime,
      };

      const [data] = await Promise.all([
        loadQuotaData({ updateStats: false, override }),
        loadModelData(override),
      ]);
      if (data && updateChartDataCallback) {
        updateChartDataCallback(data, { updateStats: false, defaultTime });
      }
      // 图表筛选不影响顶部卡片，卡片仅在挂载/刷新时按 24h 口径更新
    },
    [loadQuotaData, loadModelData],
  );

  // ========== Effects ==========
  useEffect(() => {
    const timer = setTimeout(() => {
      setGreetingVisible(true);
    }, 100);
    return () => clearTimeout(timer);
  }, []);

  useEffect(() => {
    if (!initialized.current) {
      getUserData();
      initialized.current = true;
    }
  }, [getUserData]);

  return {
    // 基础状态
    loading: showLoading,
    greetingVisible,
    searchModalVisible,

    // 输入状态
    inputs,
    dataExportDefaultTime,

    // 数据状态
    quotaData,
    consumeQuota,
    setConsumeQuota,
    consumeTokens,
    setConsumeTokens,
    times,
    setTimes,
    pieData,
    setPieData,
    lineData,
    setLineData,
    modelColors,
    setModelColors,
    displayCurrency,
    modelPopularRank,
    modelQuotaRadio,
    invitees,
    inviteesLoading,
    inviteesTotal,
    selectedInvitee,
    selectedCardUser,
    selectedCardStats,
    selectedCardLoading,
    selfCardStats,

    // 图表状态
    activeChartTab,
    setActiveChartTab,

    // 趋势数据
    trendData,
    setTrendData,

    // Uptime 数据
    uptimeData,
    uptimeLoading,
    activeUptimeTab,
    setActiveUptimeTab,

    // 计算值
    timeOptions,
    performanceMetrics,
    cardPerformanceMetrics,
    getGreeting,
    isAdminUser,
    hasApiInfoPanel,
    hasInfoPanels,
    apiInfoEnabled,
    announcementsEnabled,
    faqEnabled,
    uptimeEnabled,

    // 函数
    handleInputChange,
    showSearchModal,
    handleCloseModal,
    applySelfCardData,
    loadQuotaData,
    loadModelData,
    loadUserQuotaData,
    loadUptimeData,
    getUserData,
    refresh,
    handleSearchConfirm,
    handleDateRangeChange,
    loadInvitees,
    selectInvitee,
    clearInvitee,

    // 导航和翻译
    navigate,
    t,
    isMobile,
  };
};
