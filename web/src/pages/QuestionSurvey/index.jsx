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

import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Divider,
  Space,
  Modal,
  Empty,
  Toast,
  Table,
  Select,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { RefreshCw, Eye, Trash2 } from 'lucide-react';
import { IconComment } from '@douyinfe/semi-icons';
import {
  API,
  timestamp2string,
  showSuccess,
  createCardProPagination,
  isProviderOwner,
  isAdmin,
  getProviderId,
  hasUserPermission,
} from '../../helpers';
import CardPro from '../../components/common/ui/CardPro';

const { Text } = Typography;

// 问题类型选项配置：值与 /userQuestion 页提交的索引值对应（1 开始）
const ISSUE_TYPE_OPTIONS = [
  '功能问题',
  '使用体验',
  '性能问题',
  '账单与付费',
  '其他建议',
];

// 紧急程度：/userQuestion 页提交的值为 normal/urgent/critical
const URGENCY_MAP = {
  normal: '一般',
  urgent: '紧急',
  critical: '非常紧急',
};

// 问卷类型：与后端 model 的 SurveyType* 常量对应
const SURVEY_TYPE_MAP = {
  1: '用户反馈',
  2: '合作申请',
};

// 合作申请：期望时间 /cooperationApplication 页提交的值为 asap/1m/3m/eval
const COOP_TIMELINE_MAP = {
  asap: '尽快',
  '1m': '一个月内',
  '3m': '三个月内',
  eval: '前期评估',
};

// 合作申请：测试包 / 沟通方式提交的值为 standard/gray/consult
const COOP_TEST_PACKAGE_MAP = {
  standard: '标准验证测试',
  gray: '业务灰度测试',
  consult: '先咨询，暂不测试',
};

// 合作申请：期望窗口提交的值为 ''/weekday/weekend
const COOP_WINDOW_MAP = {
  '': '均可',
  weekday: '工作日窗口',
  weekend: '周末窗口',
};

const QuestionSurvey = () => {
  const { t } = useTranslation();
  const [records, setRecords] = useState([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(10);
  const [total, setTotal] = useState(0);

  // 服务商 owner 模式：使用 /api/provider/ 接口，只操作本站问卷；主站管理员使用 /api/user/questionnaire/admin/
  // 服务商域名下，被授予 providerQuestionSurvey 模块权限的普通用户也按服务商模式查看本站问卷
  // 服务商域名下属主（即使身兼主站管理员）按服务商模式查看本站问卷
  const providerMode =
    (isProviderOwner() || hasUserPermission('providerQuestionSurvey')) &&
    !(getProviderId() === 0 && isAdmin());

  // 主站管理员筛选：站点选项 + 当前选中（0=主站，>0=分站，-1=全部）
  const [providerFilter, setProviderFilter] = useState(0);
  const [providerOptions, setProviderOptions] = useState([]);

  // 主站管理员加载分站列表（用于右上角筛选下拉）
  useEffect(() => {
    if (providerMode) return;
    let cancelled = false;
    API.get('/api/provider/admin')
      .then((res) => {
        if (cancelled || !res.data.success) return;
        const providers = Array.isArray(res.data.data) ? res.data.data : [];
        setProviderOptions(
          providers.map((p) => ({ value: p.id, label: p.name || `#${p.id}` })),
        );
      })
      .catch(() => {
        // 分站列表加载失败不阻塞页面，仅筛选下拉无分站选项
      });
    return () => {
      cancelled = true;
    };
  }, [providerMode]);

  // 请求序号：切换筛选/快速操作时丢弃过期响应，避免旧请求后返回覆盖新数据
  const requestIdRef = useRef(0);

  // 加载问卷提交记录
  const loadRecords = useCallback(
    async (currentPage) => {
      const requestId = ++requestIdRef.current;
      setLoading(true);
      try {
        const url = providerMode
          ? '/api/provider/questionnaires'
          : '/api/questionnaire';
        const res = await API.get(url, {
          params: {
            p: currentPage,
            page_size: pageSize,
            ...(providerMode ? {} : { provider_id: providerFilter }),
          },
        });
        if (requestId !== requestIdRef.current) return; // 过期响应，丢弃
        if (res.data.success) {
          setRecords(res.data.data.items || []);
          setTotal(res.data.data.total || 0);
          setPage(currentPage);
        } else {
          Toast.error(res.data.message || t('加载失败'));
        }
      } catch (err) {
        if (requestId !== requestIdRef.current) return; // 过期响应，丢弃
        Toast.error(t('加载失败'));
      } finally {
        if (requestId === requestIdRef.current) {
          setLoading(false);
        }
      }
    },
    [pageSize, t, providerMode, providerFilter],
  );

  // 问卷详情弹窗
  const [detailRecord, setDetailRecord] = useState(null);

  useEffect(() => {
    loadRecords(1);
  }, [loadRecords]);

  // 解析问卷数据（survey_data 为 JSON 文本）
  const parseSurveyData = (raw) => {
    if (!raw) return {};
    try {
      return typeof raw === 'string' ? JSON.parse(raw) : raw;
    } catch (e) {
      return {};
    }
  };

  // 根据用户名称列显示
  const formatUserName = (record) => {
    if (record.user_id === 0) return t('匿名');
    return record.display_name || record.username || `${record.user_id}`;
  };

  // 选项值 -> 文案
  const optionText = (options, value, t) => {
    const idx = parseInt(value, 10);
    if (!Number.isInteger(idx) || idx < 1 || idx > options.length) {
      return t(value) || value || '-';
    }
    return t(options[idx - 1]);
  };

  // 详情字段配置：label + 取值方式，回显 /userQuestion 页表单结构
  const detailFields = (data, t) => [
    { label: t('姓名'), value: data.name || '-' },
    { label: t('联系方式'), value: data.contact || '-' },
    {
      label: t('问题类型'),
      value: optionText(ISSUE_TYPE_OPTIONS, data.issueType, t),
    },
    {
      label: t('紧急程度'),
      value: URGENCY_MAP[data.urgency] ? t(URGENCY_MAP[data.urgency]) : '-',
    },
    { label: t('问题描述'), value: data.description || '-' },
    {
      label: t('问题截图'),
      // 数组走弹窗里的图片分支渲染；老记录无此字段显示 '-'
      value: Array.isArray(data.screenshots) && data.screenshots.length
        ? data.screenshots
        : '-',
    },
    { label: t('其他'), value: data.other || '-' },
    {
      label: t('同意联系'),
      value: data.consent ? t('是') : t('否'),
    },
  ];

  // 合作申请详情字段配置：回显 /cooperationApplication 页表单结构
  const cooperationDetailFields = (data, t) => [
    { label: t('公司 / 机构'), value: data.company || '-' },
    { label: t('联系人'), value: data.contact || '-' },
    { label: t('职务'), value: data.title || '-' },
    { label: t('邮箱'), value: data.email || '-' },
    { label: t('电话'), value: data.phone || '-' },
    { label: t('微信'), value: data.wechat || '-' },
    {
      label: t('合作方向'),
      value: data.cooperationType ? t(data.cooperationType) : '-',
    },
    { label: t('并发目标'), value: data.concurrency || '-' },
    { label: t('上下文长度'), value: data.contextLength || '-' },
    { label: t('目标模型'), value: data.models || '-' },
    {
      label: t('期望时间'),
      value: COOP_TIMELINE_MAP[data.timeline]
        ? t(COOP_TIMELINE_MAP[data.timeline])
        : '-',
    },
    {
      label: t('测试包 / 沟通方式'),
      value: COOP_TEST_PACKAGE_MAP[data.testPackage]
        ? t(COOP_TEST_PACKAGE_MAP[data.testPackage])
        : '-',
    },
    {
      label: t('期望窗口'),
      value: COOP_WINDOW_MAP[data.windowPreference] !== undefined
        ? t(COOP_WINDOW_MAP[data.windowPreference])
        : '-',
    },
    { label: t('期望日期'), value: data.windowDate || '-' },
    { label: t('业务场景与需求'), value: data.requirement || '-' },
    { label: t('其他补充'), value: data.remark || '-' },
    {
      label: t('上传资料'),
      // 对象数组（{url,name,size}）走弹窗里的文件链接分支渲染；无附件显示 '-'
      value: Array.isArray(data.files) && data.files.length ? data.files : '-',
    },
    {
      label: t('同意联系'),
      value: data.consent ? t('是') : t('否'),
    },
  ];

  // 详情字段按问卷类型分支：合作申请(2)之外的记录一律按用户反馈(1)回显
  const surveyDetailFields = (record, data, t) =>
    record.survey_type === 2
      ? cooperationDetailFields(data, t)
      : detailFields(data, t);

  const handleOpenDetail = (record) => {
    setDetailRecord(record);
  };

  const handleCloseDetail = () => {
    setDetailRecord(null);
  };

  const handleDelete = (record) => {
    Modal.error({
      title: t('确定是否要删除此问卷提交记录？'),
      content: (
        <div>
          <div className='mb-1 font-medium'>
            {t('用户')}：{formatUserName(record)}
          </div>
          <div>{t('此修改将不可逆')}</div>
        </div>
      ),
      okText: t('确定'),
      cancelText: t('取消'),
      onOk: async () => {
        try {
          const url = providerMode
            ? `/api/provider/questionnaires/${record.id}`
            : `/api/questionnaire/${record.id}`;
          const res = await API.delete(url);
          if (res.data.success) {
            showSuccess(t('删除成功！'));
            // 当前页删除后为空且不是第一页时，回退一页
            if (records.length === 1 && page > 1) {
              loadRecords(page - 1);
            } else {
              loadRecords(page);
            }
          } else {
            Toast.error(res.data.message || t('删除失败'));
          }
        } catch (err) {
          Toast.error(t('删除失败'));
        }
      },
    });
  };

  const handleRefresh = () => {
    loadRecords(page);
  };

  const paginationArea = createCardProPagination({
    currentPage: page,
    pageSize,
    total,
    onPageChange: (p) => loadRecords(p),
    t,
  });

  const columns = [
    ...(providerMode
      ? []
      : [
          {
            title: t('来源站点'),
            dataIndex: 'provider_id',
            width: 140,
            render: (_, record) => {
              if (record.provider_id === 0) return <div>{t('主站')}</div>;
              return (
                <div>{record.provider_name || `#${record.provider_id}`}</div>
              );
            },
          },
        ]),
    {
      title: t('用户ID'),
      dataIndex: 'user_id',
      width: 90,
      render: (value) => {
        return <div>{value != null ? value : '-'}</div>;
      },
    },
    {
      title: t('用户名称'),
      dataIndex: 'username',
      render: (_, record) => {
        return <div className='font-medium'>{formatUserName(record)}</div>;
      },
    },
    {
      title: t('类型'),
      dataIndex: 'survey_type',
      width: 110,
      render: (value) => {
        // 老数据 survey_type 为空时按用户反馈(1)展示
        const label = SURVEY_TYPE_MAP[value || 1];
        return <div>{label ? t(label) : '-'}</div>;
      },
    },
    {
      title: t('问卷提交时间'),
      dataIndex: 'created_at',
      render: (text) => {
        return <div>{text ? timestamp2string(text) : '-'}</div>;
      },
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      fixed: 'right',
      width: 150,
      render: (_, record) => {
        return (
          <Space>
            <Button
              type='tertiary'
              size='small'
              icon={<Eye size={14} />}
              onClick={() => handleOpenDetail(record)}
            >
              {t('查看')}
            </Button>
            <Button
              type='danger'
              size='small'
              icon={<Trash2 size={14} />}
              onClick={() => handleDelete(record)}
            >
              {t('删除')}
            </Button>
          </Space>
        );
      },
    },
  ];

  const detailData = detailRecord
    ? parseSurveyData(detailRecord.survey_data)
    : {};

  return (
    <div className='px-2'>
      <CardPro
        type='type1'
        descriptionArea={
          <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
            <div className='flex items-center text-blue-500'>
              <IconComment className='mr-2' />
              <Text>{t('问卷调查')}</Text>
            </div>
            <Text type='tertiary' size='small'>
              {t('查看用户提交的问卷记录，可查看详情与删除。')}
            </Text>
          </div>
        }
        actionsArea={
          <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
            <div className='flex gap-2 w-full md:w-auto'>
              <Button
                type='tertiary'
                icon={<RefreshCw size={14} />}
                onClick={handleRefresh}
                size='small'
              >
                {t('刷新')}
              </Button>
            </div>
            {!providerMode && (
              <div className='flex gap-2 w-full md:w-auto'>
                <Select
                  value={providerFilter}
                  onChange={(value) => {
                    // 只更新筛选值；加载由 useEffect([loadRecords]) 在筛选变化后自动触发，
                    // 避免手动调用使用旧闭包值发出过期请求造成数据竞态
                    setProviderFilter(value);
                  }}
                  optionList={[
                    { value: -1, label: t('全部站点') },
                    { value: 0, label: t('主站') },
                    ...providerOptions,
                  ]}
                  style={{ width: 180 }}
                  size='small'
                  placeholder={t('选择站点')}
                />
              </div>
            )}
          </div>
        }
        paginationArea={paginationArea}
        t={t}
      >
        <Table
          columns={columns}
          dataSource={records}
          rowKey='id'
          loading={loading}
          scroll={{ x: 'max-content' }}
          pagination={false}
          empty={
            <Empty
              image={
                <IllustrationNoResult style={{ width: 150, height: 150 }} />
              }
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
              }
              description={t('暂无问卷提交记录')}
              style={{ padding: 30 }}
            />
          }
          className='rounded-xl overflow-hidden'
          size='middle'
        />
      </CardPro>

      {/* 问卷详情弹窗 */}
      <Modal
        visible={!!detailRecord}
        onCancel={handleCloseDetail}
        title={t('问卷详情')}
        footer={
          <Button theme='solid' onClick={handleCloseDetail}>
            {t('确定')}
          </Button>
        }
      >
        {detailRecord && (
          <>
            {/* 提交者信息：与问卷内容之间用分割线区分 */}
            <div className='flex flex-wrap gap-x-8 gap-y-1 text-sm'>
              <div>
                <span className='text-gray-500'>{t('用户')}：</span>
                <span className='font-medium'>
                  {formatUserName(detailRecord)}
                </span>
              </div>
              <div>
                <span className='text-gray-500'>{t('问卷提交时间')}：</span>
                <span className='font-medium'>
                  {detailRecord.created_at
                    ? timestamp2string(detailRecord.created_at)
                    : '-'}
                </span>
              </div>
            </div>

            <Divider margin='12px' />

            {/* 问卷内容：标签列固定宽度对齐，值列自适应 */}
            <div className='space-y-2 text-sm'>
              {surveyDetailFields(detailRecord, detailData, t).map(
                (field, idx) => (
                  <div key={idx} className='flex items-start gap-3'>
                    <span className='w-20 shrink-0 text-gray-500'>
                      {field.label}：
                    </span>
                    {Array.isArray(field.value) ? (
                      // 附件：字符串为问题截图（缩略图），对象为合作申请资料（文件链接）
                      <div className='flex flex-wrap gap-2'>
                        {field.value.map((item, i) =>
                          typeof item === 'string' ? (
                            <a
                              key={item}
                              href={item}
                              target='_blank'
                              rel='noreferrer'
                            >
                              <img
                                src={item}
                                alt={t('问题截图')}
                                className='w-16 h-16 object-cover rounded-md border border-gray-200'
                              />
                            </a>
                          ) : (
                            <a
                              key={item.url || i}
                              href={item.url}
                              target='_blank'
                              rel='noreferrer'
                              className='break-all text-blue-500 hover:underline'
                              title={item.name}
                            >
                              {item.name || item.url}
                            </a>
                          ),
                        )}
                      </div>
                    ) : (
                      <span className='break-all font-medium'>
                        {field.value}
                      </span>
                    )}
                  </div>
                ),
              )}
            </div>
          </>
        )}
      </Modal>
    </div>
  );
};

export default QuestionSurvey;
