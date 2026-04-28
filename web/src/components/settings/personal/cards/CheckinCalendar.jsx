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

// React 核心钩子：useState 状态管理，useEffect 副作用处理，useMemo 计算缓存
import React, { useState, useEffect, useMemo } from 'react';
// Semi Design UI 组件库
import {
  Card,           // 卡片容器，用于包裹签到面板
  Calendar,       // 日历组件，用于展示签到日历
  Button,         // 按钮组件，用于签到按钮
  Typography,     // 排版组件，用于文字展示
  Avatar,         // 头像/图标容器组件
  Spin,           // 加载旋转指示器
  Tooltip,        // 文字提示气泡
  Collapsible,    // 折叠面板组件，用于展开/收起签到详情
  Modal,          // 弹窗组件，用于 Turnstile 人机验证弹窗
} from '@douyinfe/semi-ui';
// Lucide 图标库
import {
  CalendarCheck,  // 日历勾选图标，签到面板标题图标
  Gift,           // 礼物图标，签到按钮图标
  Check,          // 勾选图标，已签到日期标记
  ChevronDown,    // 向下箭头，展开指示
  ChevronUp,      // 向上箭头，收起指示
} from 'lucide-react';
// Cloudflare Turnstile 人机验证组件
import Turnstile from 'react-turnstile';
// 工具函数：API 请求封装、消息提示、额度单位转换
import { API, showError, showSuccess, getQuotaPerUnit } from '../../../../helpers';

/**
 * CheckinCalendar — 每日签到日历组件
 *
 * 功能说明：
 * 1. 展示用户签到状态（今日是否已签到、累计签到天数）
 * 2. 用户点击按钮执行签到，获得随机额度奖励
 * 3. 以日历形式展示当月签到记录和每日获得的额度
 * 4. 支持 Cloudflare Turnstile 人机验证（可配置）
 * 5. 支持折叠/展开，已签到时默认折叠
 * 6. 支持多币种展示（USD/CNY）
 *
 * @param {Object} props
 * @param {Function} props.t - 国际化翻译函数
 * @param {Object} props.status - 用户状态对象，包含 checkin_enabled 字段
 * @param {boolean} props.turnstileEnabled - 是否启用了 Turnstile 人机验证
 * @param {string} props.turnstileSiteKey - Turnstile 站点密钥
 */
const CheckinCalendar = ({ t, status, turnstileEnabled, turnstileSiteKey }) => {
  // loading — 数据加载状态，控制日历区域的 Spin 加载动画
  const [loading, setLoading] = useState(false);
  // checkinLoading — 签到按钮加载状态，防止重复点击签到
  const [checkinLoading, setCheckinLoading] = useState(false);
  // turnstileModalVisible — Turnstile 人机验证弹窗是否可见
  const [turnstileModalVisible, setTurnstileModalVisible] = useState(false);
  // turnstileWidgetKey — Turnstile 组件的 key，用于强制重新渲染（验证过期时刷新）
  const [turnstileWidgetKey, setTurnstileWidgetKey] = useState(0);
  // checkinData — 签到数据对象，包含功能开关、统计数据、展示币种信息
  const [checkinData, setCheckinData] = useState({
    enabled: false,       // 签到功能是否启用
    stats: {
      checked_in_today: false,  // 今日是否已签到
      total_checkins: 0,        // 累计签到总天数
      total_quota: 0,           // 累计获得的总额度
      checkin_count: 0,         // 当月签到次数
      records: [],              // 当月签到记录数组，每项包含 checkin_date 和 quota_awarded
    },
  });
  // currentMonth — 当前日历显示的月份，格式 YYYY-MM，用于请求对应月份的签到记录
  const [currentMonth, setCurrentMonth] = useState(
    new Date().toISOString().slice(0, 7),
  );
  // initialLoaded — 初始加载状态标志，用于避免折叠状态在首次加载前闪烁
  const [initialLoaded, setInitialLoaded] = useState(false);
  // isCollapsed — 折叠状态：null 表示未确定（等待首次加载完成后设置），true=折叠，false=展开
  const [isCollapsed, setIsCollapsed] = useState(null);

  /**
   * formatCheckinQuota — 格式化签到额度为展示字符串
   *
   * 将内部额度数值转换为用户可读的币种金额字符串
   * 处理流程：额度数值 → 除以单位 → 乘以汇率（CNY时）→ 添加货币符号
   *
   * @param {number|string} quota - 额度值（可能是数字或字符串）
   * @param {number} digits - 小数位数，默认 2 位
   * @param {Object} source - 包含 display_currency/rate/symbol 的数据源，默认使用 checkinData
   * @returns {string} 格式化后的金额字符串，如 "$1.23" 或 "¥8.50"
   */
  const formatCheckinQuota = (quota, digits = 2, source = checkinData) => {
    // 将 quota 转为数字类型
    const numericQuota = Number(quota);
    // 如果不是有效数字（如 NaN、Infinity），直接返回原始值
    if (!Number.isFinite(numericQuota)) {
      return quota;
    }

    // 从数据源获取展示币种信息，默认 USD
    const displayCurrency = source?.display_currency || 'USD';
    // 展示币种的汇率，默认 1（即美元汇率）
    const displayRate = Number(source?.display_rate) || 1;
    // 展示币种的货币符号，如 "$"、"¥"
    const displaySymbol = source?.display_symbol || '';

    // 将额度除以单位换算为美元金额
    let amount = numericQuota / getQuotaPerUnit();
    // 如果展示币种是人民币，则乘以汇率
    if (displayCurrency === 'CNY') {
      amount = amount * displayRate;
    }

    // 拼接货币符号和格式化后的金额
    return `${displaySymbol}${amount.toFixed(digits)}`;
  };

  /**
   * checkinRecordsMap — 日期到额度的映射表
   *
   * 将签到记录数组转换为 { "YYYY-MM-DD": quota_awarded } 的对象
   * 用于在日历渲染时快速查找某日是否有签到记录及获得的额度
   */
  const checkinRecordsMap = useMemo(() => {
    const map = {};
    const records = checkinData.stats?.records || [];
    // 遍历签到记录，建立日期到额度的映射
    records.forEach((record) => {
      map[record.checkin_date] = record.quota_awarded;
    });
    return map;
  }, [checkinData.stats?.records]); // 当签到记录变化时重新计算

  /**
   * monthlyQuota — 本月累计获得的额度
   *
   * 对当月所有签到记录的 quota_awarded 求和
   */
  const monthlyQuota = useMemo(() => {
    const records = checkinData.stats?.records || [];
    return records.reduce(
      (sum, record) => sum + (record.quota_awarded || 0),
      0,
    );
  }, [checkinData.stats?.records]); // 当签到记录变化时重新计算

  /**
   * fetchCheckinStatus — 获取用户指定月份的签到状态和历史记录
   *
   * 调用 GET /api/user/checkin?month=YYYY-MM 接口
   * 首次加载时根据"今日是否已签到"决定面板折叠状态：
   *   - 已签到 → 折叠（简洁展示）
   *   - 未签到 → 展开（引导用户签到）
   *
   * @param {string} month - 月份字符串，格式 YYYY-MM
   */
  const fetchCheckinStatus = async (month) => {
    // 记录是否为首次加载（首次加载需要设置折叠状态）
    const isFirstLoad = !initialLoaded;
    setLoading(true);
    try {
      // 请求后端签到状态接口
      const res = await API.get(`/api/user/checkin?month=${month}`);
      const { success, data, message } = res.data;
      if (success) {
        // 请求成功，更新签到数据
        setCheckinData(data);
        // 首次加载时，根据今日是否已签到设置折叠状态
        if (isFirstLoad) {
          // 已签到则折叠，未签到则展开
          setIsCollapsed(data.stats?.checked_in_today ?? false);
          setInitialLoaded(true);
        }
      } else {
        // 请求失败，显示错误信息
        showError(message || t('获取签到状态失败'));
        // 首次加载失败时，默认展开面板
        if (isFirstLoad) {
          setIsCollapsed(false);
          setInitialLoaded(true);
        }
      }
    } catch (error) {
      // 网络异常等错误
      showError(t('获取签到状态失败'));
      if (isFirstLoad) {
        setIsCollapsed(false);
        setInitialLoaded(true);
      }
    } finally {
      // 无论成功失败都关闭加载状态
      setLoading(false);
    }
  };

  /**
   * postCheckin — 发送签到请求到后端
   *
   * 如果有 Turnstile token（通过人机验证获取），则附带在请求中
   *
   * @param {string|null} token - Turnstile 人机验证 token，无则传 null
   * @returns {Promise} API 请求 Promise
   */
  const postCheckin = async (token) => {
    // 根据 token 是否存在拼接不同的 URL
    const url = token
      ? `/api/user/checkin?turnstile=${encodeURIComponent(token)}` // 带 Turnstile token
      : '/api/user/checkin'; // 不带 token
    return API.post(url);
  };

  /**
   * shouldTriggerTurnstile — 判断是否需要触发 Turnstile 人机验证
   *
   * 条件：
   * 1. Turnstile 功能已启用
   * 2. 后端返回的错误消息中包含 "Turnstile" 关键字（说明后端要求验证）
   *
   * @param {string} message - 后端返回的错误消息
   * @returns {boolean} 是否需要触发验证
   */
  const shouldTriggerTurnstile = (message) => {
    // 未启用 Turnstile 则不触发
    if (!turnstileEnabled) return false;
    // 消息不是字符串则默认触发（安全起见）
    if (typeof message !== 'string') return true;
    // 消息中包含 "Turnstile" 说明后端要求验证
    return message.includes('Turnstile');
  };

  /**
   * doCheckin — 执行签到操作
   *
   * 签到流程：
   * 1. 调用后端签到接口
   * 2. 成功 → 显示奖励额度，刷新签到状态，关闭验证弹窗
   * 3. 失败且需要 Turnstile → 弹出人机验证弹窗
   * 4. 验证通过后携带 token 重新签到
   *
   * @param {string|null} token - Turnstile 验证 token，首次签到时为 null
   */
  const doCheckin = async (token) => {
    setCheckinLoading(true);
    try {
      // 发送签到请求
      const res = await postCheckin(token);
      const { success, data, message } = res.data;
      if (success) {
        // 签到成功，显示获得的额度奖励
        showSuccess(
          t('签到成功！获得') + ' ' + formatCheckinQuota(data.quota_awarded, 2, data),
        );
        // 刷新签到状态以更新日历和统计数据
        fetchCheckinStatus(currentMonth);
        // 关闭 Turnstile 验证弹窗（如果打开了的话）
        setTurnstileModalVisible(false);
      } else {
        // 签到失败的情况
        if (!token && shouldTriggerTurnstile(message)) {
          // 没有 token 且后端要求 Turnstile 验证 → 弹出验证窗口
          if (!turnstileSiteKey) {
            // site key 未配置，直接报错
            showError('Turnstile is enabled but site key is empty.');
            return;
          }
          // 显示 Turnstile 人机验证弹窗
          setTurnstileModalVisible(true);
          return;
        }
        if (token && shouldTriggerTurnstile(message)) {
          // 有 token 但仍然需要验证 → 可能 token 过期，刷新 widget 重新验证
          setTurnstileWidgetKey((v) => v + 1);
        }
        // 显示后端返回的错误消息
        showError(message || t('签到失败'));
      }
    } catch (error) {
      // 网络异常等错误
      showError(t('签到失败'));
    } finally {
      // 无论成功失败都关闭签到按钮加载状态
      setCheckinLoading(false);
    }
  };

  /**
   * useEffect — 初始化副作用
   *
   * 当签到功能启用状态或当前月份变化时，自动获取签到数据
   * 依赖项：status?.checkin_enabled（签到功能开关）、currentMonth（当前查看的月份）
   */
  useEffect(() => {
    if (status?.checkin_enabled) {
      // 签到功能已启用时才请求数据
      fetchCheckinStatus(currentMonth);
    }
  }, [status?.checkin_enabled, currentMonth]);

  // 如果签到功能未启用，不渲染任何内容
  if (!status?.checkin_enabled) {
    return null;
  }

  /**
   * dateRender — 日历单元格渲染函数
   *
   * 为每个日期单元格添加签到状态展示：
   * - 已签到的日期：显示绿色勾选图标 + 获得的额度金额
   * - 未签到的日期：不额外渲染内容
   *
   * @param {string} dateString - Semi Calendar 传入的日期字符串（Date.toString() 格式）
   * @returns {JSX.Element|null} 签到标记元素或 null
   */
  const dateRender = (dateString) => {
    // Semi Calendar 传入的 dateString 是 Date.toString() 格式，如 "Mon Apr 28 2026 00:00:00 GMT+0800"
    // 需要转换为 YYYY-MM-DD 格式才能与后端签到记录匹配
    const date = new Date(dateString);
    // 无效日期直接返回
    if (isNaN(date.getTime())) {
      return null;
    }
    // 使用本地时间格式化，避免时区偏移导致的日期不匹配
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0'); // 月份从 0 开始，需 +1
    const day = String(date.getDate()).padStart(2, '0');
    const formattedDate = `${year}-${month}-${day}`; // 拼接为 YYYY-MM-DD 格式
    // 从签到记录映射表中查找该日期的额度
    const quotaAwarded = checkinRecordsMap[formattedDate];
    // 额度存在则表示该日已签到
    const isCheckedIn = quotaAwarded !== undefined;

    if (isCheckedIn) {
      // 已签到：渲染带 Tooltip 的绿色勾选标记和额度
      return (
        <Tooltip
          content={`${t('获得')} ${formatCheckinQuota(quotaAwarded)}`}
          position='top'
        >
          {/* 绝对定位覆盖整个日期单元格 */}
          <div className='absolute inset-0 flex flex-col items-center justify-center cursor-pointer'>
            {/* 绿色圆形勾选图标 */}
            <div className='w-6 h-6 rounded-full bg-green-500 flex items-center justify-center mb-0.5 shadow-sm'>
              <Check size={14} className='text-white' strokeWidth={3} />
            </div>
            {/* 获得的额度金额文字 */}
            <div className='text-[10px] font-medium text-green-600 dark:text-green-400 leading-none'>
              {formatCheckinQuota(quotaAwarded)}
            </div>
          </div>
        </Tooltip>
      );
    }
    // 未签到日期不渲染额外内容
    return null;
  };

  /**
   * handleMonthChange — 日历月份切换处理
   *
   * 用户在日历上切换月份时，更新 currentMonth 状态
   * currentMonth 变化会触发 useEffect 重新获取该月签到数据
   *
   * @param {Date} date - 日历组件返回的新月份日期对象
   */
  const handleMonthChange = (date) => {
    // 从日期对象提取 YYYY-MM 格式的月份字符串
    const month = date.toISOString().slice(0, 7);
    setCurrentMonth(month);
  };

  // ==================== 渲染签到卡片 ====================
  return (
    <Card className='personal-v2-panel personal-v2-checkin !rounded-2xl'>
      {/* Turnstile 人机验证弹窗 */}
      <Modal
        title='Security Check'
        visible={turnstileModalVisible}
        footer={null}
        centered
        onCancel={() => {
          // 用户取消关闭弹窗，同时刷新 widget key 以重置验证组件
          setTurnstileModalVisible(false);
          setTurnstileWidgetKey((v) => v + 1);
        }}
      >
        {/* Turnstile 验证组件容器 */}
        <div className='flex justify-center py-2'>
          <Turnstile
            key={turnstileWidgetKey}
            sitekey={turnstileSiteKey}
            onVerify={(token) => {
              // 用户完成人机验证后，携带 token 重新执行签到
              doCheckin(token);
            }}
            onExpire={() => {
              // 验证 token 过期，刷新 widget 让用户重新验证
              setTurnstileWidgetKey((v) => v + 1);
            }}
          />
        </div>
      </Modal>

      {/* ===== 卡片头部区域：标题 + 签到按钮 ===== */}
      <div className='flex items-center justify-between'>
        {/* 左侧：点击可折叠/展开的区域 */}
        <div
          className='flex items-center flex-1 cursor-pointer'
          onClick={() => setIsCollapsed(!isCollapsed)}
        >
          {/* 绿色头像图标 */}
          <Avatar size='small' color='green' className='mr-3 shadow-md'>
            <CalendarCheck size={16} />
          </Avatar>
          <div className='flex-1'>
            {/* 标题行：文字 + 折叠箭头 */}
            <div className='flex items-center gap-2'>
              <Typography.Text className='text-lg font-medium'>
                {t('每日签到')}
              </Typography.Text>
              {/* 根据折叠状态显示不同方向的箭头 */}
              {isCollapsed ? (
                <ChevronDown size={16} className='text-gray-400' />
              ) : (
                <ChevronUp size={16} className='text-gray-400' />
              )}
            </div>
            {/* 副标题：根据状态显示不同的提示文字 */}
            <div className='text-xs text-gray-500 dark:text-gray-400'>
              {!initialLoaded
                ? t('正在加载签到状态...')
                : checkinData.stats?.checked_in_today
                  ? t('今日已签到，累计签到') +
                    ` ${checkinData.stats?.total_checkins || 0} ` +
                    t('天')
                  : t('每日签到可获得随机额度奖励')}
            </div>
          </div>
        </div>
        {/* 右侧：签到按钮 */}
        <Button
          type='primary'
          theme='solid'
          icon={<Gift size={16} />}
          onClick={() => doCheckin()}
          loading={checkinLoading || !initialLoaded}
          disabled={!initialLoaded || checkinData.stats?.checked_in_today}
          className='!bg-green-600 hover:!bg-green-700'
        >
          {/* 按钮文字根据状态变化 */}
          {!initialLoaded
            ? t('加载中...')
            : checkinData.stats?.checked_in_today
              ? t('今日已签到')
              : t('立即签到')}
        </Button>
      </div>

      {/* ===== 可折叠内容区域 ===== */}
      {/* isOpen=false 时折叠（已签到的默认状态） */}
      <Collapsible isOpen={isCollapsed === false} keepDOM>
        {/* 签到统计面板：三列网格展示累计签到、本月获得、累计获得 */}
        <div className='grid grid-cols-3 gap-3 mb-4 mt-4'>
          {/* 累计签到天数 */}
          <div className='text-center p-2.5 bg-slate-50 dark:bg-slate-800 rounded-lg'>
            <div className='text-xl font-bold text-green-600'>
              {checkinData.stats?.total_checkins || 0}
            </div>
            <div className='text-xs text-gray-500'>{t('累计签到')}</div>
          </div>
          {/* 本月获得的额度（6位小数精度） */}
          <div className='text-center p-2.5 bg-slate-50 dark:bg-slate-800 rounded-lg'>
            <div className='text-xl font-bold text-orange-600'>
              {formatCheckinQuota(monthlyQuota, 6)}
            </div>
            <div className='text-xs text-gray-500'>{t('本月获得')}</div>
          </div>
          {/* 累计获得的总额度（6位小数精度） */}
          <div className='text-center p-2.5 bg-slate-50 dark:bg-slate-800 rounded-lg'>
            <div className='text-xl font-bold text-blue-600'>
              {formatCheckinQuota(checkinData.stats?.total_quota || 0, 6)}
            </div>
            <div className='text-xs text-gray-500'>{t('累计获得')}</div>
          </div>
        </div>

        {/* ===== 签到日历区域 ===== */}
        <Spin spinning={loading}>
          <div className='border rounded-lg overflow-hidden checkin-calendar'>
            {/* 日历自定义样式：调整 Semi Calendar 的默认尺寸使其更紧凑 */}
            <style>{`
            .checkin-calendar .semi-calendar {
              font-size: 13px;
            }
            .checkin-calendar .semi-calendar-month-header {
              padding: 8px 12px;
            }
            .checkin-calendar .semi-calendar-month-week-row {
              height: 28px;
            }
            .checkin-calendar .semi-calendar-month-week-row th {
              font-size: 12px;
              padding: 4px 0;
            }
            .checkin-calendar .semi-calendar-month-grid-row {
              height: auto;
            }
            .checkin-calendar .semi-calendar-month-grid-row td {
              height: 56px;
              padding: 2px;
            }
            .checkin-calendar .semi-calendar-month-grid-row-cell {
              position: relative;
              height: 100%;
            }
            .checkin-calendar .semi-calendar-month-grid-row-cell-day {
              position: absolute;
              top: 4px;
              left: 50%;
              transform: translateX(-50%);
              font-size: 12px;
              z-index: 1;
            }
            .checkin-calendar .semi-calendar-month-same {
              background: transparent;
            }
            .checkin-calendar .semi-calendar-month-today .semi-calendar-month-grid-row-cell-day {
              background: var(--semi-color-primary);
              color: white;border-radius: 50%;
              width: 20px;
              height: 20px;
              display: flex;
              align-items: center;
              justify-content: center;}
          `}</style>
            {/* Semi Calendar 组件 */}
            <Calendar
              mode='month'
              onChange={handleMonthChange}
              dateGridRender={(dateString, date) => dateRender(dateString)}
            />
          </div>
        </Spin>

        {/* ===== 签到说明文字 ===== */}
        <div className='mt-3 p-2.5 bg-slate-50 dark:bg-slate-800 rounded-lg'>
          <Typography.Text type='tertiary' className='text-xs'>
            <ul className='list-disc list-inside space-y-0.5'>
              <li>{t('每日签到可获得随机额度奖励')}</li>
              <li>{t('签到奖励将直接添加到您的账户余额')}</li>
              <li>{t('每日仅可签到一次，请勿重复签到')}</li>
            </ul>
          </Typography.Text>
        </div>
      </Collapsible>
    </Card>
  );
};

export default CheckinCalendar;
