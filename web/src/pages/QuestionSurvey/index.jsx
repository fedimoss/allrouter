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

import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Space, Modal, Empty, Toast, Table } from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { RefreshCw, Eye, Trash2 } from 'lucide-react';
import {
  API,
  timestamp2string,
  showSuccess,
  createCardProPagination,
  isProviderOwner,
  isAdmin,
} from '../../helpers';
import './questionSurvey.css';

// 行业选项配置：值与 /userQuestion 页提交的索引值对应（1 开始），展示文案可调整
const INDUSTRY_OPTIONS = [
  '互联网 / 软件',
  '金融',
  '教育',
  '医疗',
  '制造业',
  '其他',
];
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

const QuestionSurvey = () => {
  const { t } = useTranslation();
  const [records, setRecords] = useState([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(10);
  const [total, setTotal] = useState(0);

  // 服务商 owner 模式：使用 /api/provider/ 接口，只操作本站问卷；主站管理员使用 /api/user/questionnaire/admin/
  const providerMode = isProviderOwner() && !isAdmin();

  // 加载问卷提交记录
  const loadRecords = useCallback(
    async (currentPage) => {
      setLoading(true);
      try {
        const url = providerMode
          ? '/api/provider/questionnaires'
          : '/api/questionnaire';
        const res = await API.get(url, {
          params: { p: currentPage, page_size: pageSize },
        });
        if (res.data.success) {
          setRecords(res.data.data.items || []);
          setTotal(res.data.data.total || 0);
          setPage(currentPage);
        } else {
          Toast.error(res.data.message || t('加载失败'));
        }
      } catch (err) {
        Toast.error(t('加载失败'));
      } finally {
        setLoading(false);
      }
    },
    [pageSize, t, providerMode],
  );

  // 问卷详情弹窗
  const [detailRecord, setDetailRecord] = useState(null);

  useEffect(() => {
    loadRecords(1);
  }, [loadRecords]);

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
      label: t('行业'),
      value: optionText(INDUSTRY_OPTIONS, data.industry, t),
    },
    {
      label: t('问题类型'),
      value: optionText(ISSUE_TYPE_OPTIONS, data.issueType, t),
    },
    { label: t('问题描述'), value: data.description || '-' },
    { label: t('公司/团队'), value: data.company || '-' },
    { label: t('发生时间'), value: data.occurredAt || '-' },
    {
      label: t('紧急程度'),
      value: URGENCY_MAP[data.urgency]
        ? t(URGENCY_MAP[data.urgency])
        : '-',
    },
    { label: t('期望与建议'), value: data.suggestion || '-' },
    {
      label: t('同意联系'),
      value: data.consent ? t('是') : t('否'),
    },
  ];

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
    {
      title: t('用户名称'),
      dataIndex: 'username',
      render: (_, record) => {
        return <div className='font-medium'>{formatUserName(record)}</div>;
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

  const detailData = detailRecord ? parseSurveyData(detailRecord.survey_data) : {};

  return (
    <div className='survey-v2 mt-[10px] px-2'>
      <div className='survey-v2-shell'>
        <div className='survey-v2-card'>
          <div className='survey-v2-toolbar'>
            <div className='survey-v2-toolbar-title'>{t('问卷调查')}</div>
            <div className='survey-v2-toolbar-desc'>
              <div className='survey-v2-toolbar-desc-txt'>
                {t('查看用户提交的问卷记录，可查看详情与删除。')}
              </div>
            </div>
            <div className='survey-v2-toolbar-right'>
              <Button
                type='tertiary'
                icon={<RefreshCw size={14} />}
                onClick={handleRefresh}
                style={{ marginRight: 8 }}
              >
                {t('刷新')}
              </Button>
            </div>
          </div>

          <div className='survey-v2-table-area'>
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
                    <IllustrationNoResultDark
                      style={{ width: 150, height: 150 }}
                    />
                  }
                  description={t('暂无问卷提交记录')}
                  style={{ padding: 30 }}
                />
              }
              className='rounded-xl overflow-hidden'
              size='middle'
            />
            {paginationArea && (
              <div className='survey-v2-pagination'>{paginationArea}</div>
            )}
          </div>
        </div>
      </div>

      {/* 问卷详情弹窗 */}
      <Modal
        visible={!!detailRecord}
        onCancel={handleCloseDetail}
        footer={
          <Button theme='solid' onClick={handleCloseDetail}>
            {t('确定')}
          </Button>
        }
      >
        <div className='py-2'>
          <h3 className='text-lg font-semibold mb-3'>{t('问卷详情')}</h3>
          {detailRecord && (
            <>
              <div className='mb-3 space-y-1 text-sm'>
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
              <div className='space-y-2 text-sm'>
                {detailFields(detailData, t).map((field, idx) => (
                  <div key={idx}>
                    <span className='text-gray-500'>{field.label}：</span>
                    <span className='font-medium'>{field.value}</span>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </Modal>
    </div>
  );
};

export default QuestionSurvey;
