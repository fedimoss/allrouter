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
import { Card, Avatar, Typography, Table, Tag } from '@douyinfe/semi-ui';
import { IconCoinMoneyStroked } from '@douyinfe/semi-icons';
import {
  calculateModelPrice,
  getModelPriceItems,
  parseTierTimeWindows,
} from '../../../../../helpers';

const { Text } = Typography;

const WEEKDAY_NAME_KEYS = [
  '周日',
  '周一',
  '周二',
  '周三',
  '周四',
  '周五',
  '周六',
];
const TIME_FUNC_LABELS = {
  hour: '小时',
  minute: '分钟',
  weekday: '星期',
  month: '月份',
  day: '日期',
};
const OP_SYMBOLS = {
  '==': '=',
  '!=': '≠',
  '>=': '≥',
  '<=': '≤',
  '>': '>',
  '<': '<',
};

const pad2 = (n) => String(Math.trunc(Number(n) || 0)).padStart(2, '0');

function formatHourClock(v) {
  return `${pad2(v)}:00`;
}

// 单个 (时间函数, 时区) 分组 => 可读文本，如「01:00~04:00、06:00~10:00」「周一~周五」
function formatTimeGroupText(group, t) {
  const parts = [];
  const { timeFunc } = group;

  for (const range of group.ranges) {
    if (timeFunc === 'hour') {
      if (range.wraps) {
        parts.push(
          `${formatHourClock(range.from)}~${t('次日')} ${formatHourClock(range.to)}`,
        );
      } else {
        const end = range.toInclusive
          ? `${pad2(range.to)}:59`
          : formatHourClock(range.to);
        parts.push(`${formatHourClock(range.from)}~${end}`);
      }
    } else if (timeFunc === 'weekday') {
      const names = WEEKDAY_NAME_KEYS.map((k) => t(k));
      parts.push(
        `${names[range.from] ?? range.from}~${names[range.to] ?? range.to}`,
      );
    } else {
      const label = t(TIME_FUNC_LABELS[timeFunc] || timeFunc);
      parts.push(`${label} ${range.from}~${range.to}`);
    }
  }

  if (timeFunc === 'hour') {
    for (const v of group.points)
      parts.push(`${formatHourClock(v)}~${pad2(v)}:59`);
  } else if (timeFunc === 'weekday') {
    const names = WEEKDAY_NAME_KEYS.map((k) => t(k));
    for (const v of group.points) parts.push(names[v] ?? String(v));
  } else if (group.points.length > 0) {
    const label = t(TIME_FUNC_LABELS[timeFunc] || timeFunc);
    for (const v of group.points)
      parts.push(`${label} ${OP_SYMBOLS['==']} ${v}`);
  }

  if (group.bounds.length > 0) {
    const useClock = timeFunc === 'hour';
    const label = t(TIME_FUNC_LABELS[timeFunc] || timeFunc);
    for (const b of group.bounds) {
      if (useClock)
        parts.push(`${OP_SYMBOLS[b.op] || b.op}${formatHourClock(b.value)}`);
      else parts.push(`${label} ${OP_SYMBOLS[b.op] || b.op} ${b.value}`);
    }
  }

  return parts.join('、');
}

// 一个档位的全部时间分组 => 一行文本，时区统一时合并为一个后缀
function formatTierTimeText(timeGroups, t) {
  const timezones = [...new Set(timeGroups.map((g) => g.timezone))];
  const multiTz = timezones.length > 1;
  const text = timeGroups
    .map((g) => {
      const body = formatTimeGroupText(g, t);
      return multiTz ? `${body} (${g.timezone})` : body;
    })
    .join(' ');
  return multiTz ? text : `${text} (${timezones[0]})`;
}

const ModelPricingTable = ({
  modelData,
  groupRatio,
  currency,
  siteDisplayType,
  tokenUnit,
  displayPrice,
  showRatio,
  usableGroup,
  autoGroups = [],
  t,
}) => {
  const modelEnableGroups = Array.isArray(modelData?.enable_groups)
    ? modelData.enable_groups
    : [];
  const autoChain = autoGroups.filter((g) => modelEnableGroups.includes(g));
  const renderGroupPriceTable = () => {
    // 仅展示模型可用的分组：模型 enable_groups 与用户可用分组的交集

    const availableGroups = Object.keys(usableGroup || {})
      .filter((g) => g !== '')
      .filter((g) => g !== 'auto')
      .filter((g) => modelEnableGroups.includes(g));

    // 准备表格数据
    const tableData = availableGroups.map((group) => {
      const priceData = modelData
        ? calculateModelPrice({
            record: modelData,
            selectedGroup: group,
            groupRatio,
            tokenUnit,
            displayPrice,
            currency,
            quotaDisplayType: siteDisplayType,
          })
        : { inputPrice: '-', outputPrice: '-', price: '-' };

      // 获取分组倍率
      const groupRatioValue =
        groupRatio && groupRatio[group] ? groupRatio[group] : 1;

      return {
        key: group,
        group: group,
        ratio: groupRatioValue,
        billingType:
          modelData?.billing_mode === 'tiered_expr'
            ? t('动态计费')
            : modelData?.billing_mode === 'per_second'
              ? t('按秒计费')
            : modelData?.quota_type === 0
            ? t('按量计费')
            : modelData?.quota_type === 1
              ? t('按次计费')
              : '-',
        priceItems: getModelPriceItems(priceData, t, siteDisplayType),
      };
    });

    // 定义表格列
    const columns = [
      {
        title: t('分组'),
        dataIndex: 'group',
        render: (text) => (
          <Tag color='white' size='small' shape='circle'>
            {text}
            {t('分组')}
          </Tag>
        ),
      },
    ];

    const isDynamic = modelData?.billing_mode === 'tiered_expr';

    // 动态计费且表达式含时间条件时，展示「时间」列：每个档位生效的时间窗口
    const tierTimeWindows =
      isDynamic && modelData?.billing_expr
        ? parseTierTimeWindows(modelData.billing_expr)
        : [];
    const hasTimeWindows = tierTimeWindows.some(
      (tier) => tier.timeGroups.length > 0,
    );

    // 动态计费时始终显示分组倍率，否则根据设置显示倍率
    if (showRatio || isDynamic) {
      columns.push({
        title: isDynamic ? t('分组倍率') : t('倍率'),
        dataIndex: 'ratio',
        render: (text) => (
          <Tag color={isDynamic ? 'blue' : 'white'} size='small' shape='circle'>
            {text}x
          </Tag>
        ),
      });
    }

    // 添加计费类型列
    columns.push({
      title: t('计费类型'),
      dataIndex: 'billingType',
      render: (text) => {
        let color = 'white';
        if (text === t('按量计费')) color = 'violet';
        else if (text === t('按次计费')) color = 'teal';
        else if (text === t('按秒计费')) color = 'cyan';
        else if (text === t('动态计费')) color = 'amber';
        return (
          <Tag color={color} size='small' shape='circle'>
            {text || '-'}
          </Tag>
        );
      },
    });

    // 时间窗口列：各档位在不同时间段的价格不同
    if (hasTimeWindows) {
      columns.push({
        title: t('时间'),
        render: () => (
          <div className='space-y-1'>
            {tierTimeWindows
              .filter((tier) => tier.timeGroups.length > 0 || tier.isElse)
              .map((tier, idx) => (
                <div
                  key={`${tier.label}-${idx}`}
                  className='flex items-center gap-1 flex-wrap'
                >
                  <Tag
                    color={tier.isElse ? 'grey' : 'blue'}
                    size='small'
                    shape='circle'
                  >
                    {tier.label || t('默认')}
                  </Tag>
                  <span className='text-xs text-gray-600'>
                    {tier.timeGroups.length > 0
                      ? formatTierTimeText(tier.timeGroups, t)
                      : t('其他时间')}
                  </span>
                </div>
              ))}
          </div>
        ),
      });
    }

    columns.push({
      title: siteDisplayType === 'TOKENS' ? t('计费摘要') : t('价格摘要'),
      dataIndex: 'priceItems',
      render: (items) => {
        if (items.length === 1 && items[0].isDynamic) {
          return (
            <Text type='tertiary' size='small'>
              {t('见上方动态计费详情')}
            </Text>
          );
        }
        return (
          <div className='space-y-1'>
            {items.map((item) => (
              <div key={item.key}>
                <div className='font-semibold text-orange-600'>
                  {item.label} {item.value}
                </div>
                <div className='text-xs text-gray-500'>{item.suffix}</div>
              </div>
            ))}
          </div>
        );
      },
    });

    return (
      <Table
        dataSource={tableData}
        columns={columns}
        pagination={false}
        size='small'
        bordered={false}
        className='!rounded-lg'
      />
    );
  };

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      <div className='flex items-center mb-4'>
        <Avatar size='small' color='orange' className='mr-2 shadow-md'>
          <IconCoinMoneyStroked size={16} />
        </Avatar>
        <div>
          <Text className='text-lg font-medium'>{t('分组价格')}</Text>
          <div className='text-xs text-gray-600'>
            {t('不同用户分组的价格信息')}
          </div>
        </div>
      </div>
      {autoChain.length > 0 && (
        <div className='flex flex-wrap items-center gap-1 mb-4'>
          <span className='text-sm text-gray-600'>{t('auto分组调用链路')}</span>
          <span className='text-sm'>→</span>
          {autoChain.map((g, idx) => (
            <React.Fragment key={g}>
              <Tag color='white' size='small' shape='circle'>
                {g}
                {t('分组')}
              </Tag>
              {idx < autoChain.length - 1 && <span className='text-sm'>→</span>}
            </React.Fragment>
          ))}
        </div>
      )}
      {renderGroupPriceTable()}
    </Card>
  );
};

export default ModelPricingTable;
