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

import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Button, Space, Modal, Empty, Toast, Switch } from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { Plus, RefreshCw, Eye, PenLine, Trash2, ListChecks } from 'lucide-react';
import CardTable from '../../components/common/ui/CardTable';
import { timestamp2string, showSuccess } from '../../helpers';
import './questionSurvey.css';

const STATIC_SURVEYS = [
  {
    id: 1,
    name: '2025 春季用户满意度调研',
    created_time: 1711296000,
    enabled: true,
    questions: ['您对整体服务的满意度如何？', '您最常使用的功能是？'],
  },
  {
    id: 2,
    name: 'API 接口使用体验问卷',
    created_time: 1711968000,
    enabled: true,
    questions: ['接口响应速度是否满意？', '文档是否清晰易懂？'],
  },
  {
    id: 3,
    name: '模型定价合理性调查',
    created_time: 1712572800,
    enabled: false,
    questions: ['当前定价是否合理？'],
  },
];

const QuestionSurvey = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [surveys, setSurveys] = useState(STATIC_SURVEYS);
  const [loading, setLoading] = useState(false);

  // 问卷详情弹窗
  const [detailVisible, setDetailVisible] = useState(false);
  const [detailRecord, setDetailRecord] = useState(null);

  // 新增/编辑问卷弹窗
  const [editVisible, setEditVisible] = useState(false);
  const [editingSurvey, setEditingSurvey] = useState(null);

  // 打开新增问卷弹窗
  const handleOpenCreate = () => {
    setEditingSurvey(null);
    setEditVisible(true);
  };

  // 关闭新增/编辑弹窗
  const handleCloseEdit = () => {
    setEditVisible(false);
    setEditingSurvey(null);
  };

  // 提交（新增/编辑）——后端待接入，先本地占位处理
  const handleSubmitSurvey = (payload) => {
    if (payload.id !== undefined) {
      // 编辑
      setSurveys((prev) =>
        prev.map((item) =>
          item.id === payload.id
            ? {
                ...item,
                name: payload.name,
                questions: payload.questions.map((q) => q.desc),
              }
            : item,
        ),
      );
      showSuccess(t('问卷更新成功！'));
    } else {
      // 新增
      const newId =
        surveys.length > 0 ? Math.max(...surveys.map((s) => s.id)) + 1 : 1;
      setSurveys((prev) => [
        {
          id: newId,
          name: payload.name,
          enabled: true,
          questions: payload.questions.map((q) => q.desc),
          created_time: Math.floor(Date.now() / 1000),
        },
        ...prev,
      ]);
      showSuccess(t('问卷创建成功！'));
    }
    handleCloseEdit();
  };

  const handleOpenDetail = (record) => {
    setDetailRecord(record);
    setDetailVisible(true);
  };

  const handleCloseDetail = () => {
    setDetailVisible(false);
  };

  // 开启/关闭问卷
  const handleToggleEnabled = (record, checked) => {
    setSurveys((prev) =>
      prev.map((item) =>
        item.id === record.id ? { ...item, enabled: checked } : item,
      ),
    );
    showSuccess(checked ? t('问卷已开启') : t('问卷已关闭'));
  };

  const handleDelete = (record) => {
    Modal.error({
      title: t('确定是否要删除此问卷？'),
      content: (
        <div>
          <div className='mb-1 font-medium'>{record?.name}</div>
          <div>{t('此修改将不可逆')}</div>
        </div>
      ),
      okText: t('确定'),
      cancelText: t('取消'),
      onOk: () => {
        // 删除逻辑待补充，先本地占位处理
        setSurveys((prev) => prev.filter((item) => item.id !== record.id));
        showSuccess(t('删除成功！'));
      },
    });
  };

  const handleRefresh = () => {
    setLoading(true);
    setTimeout(() => {
      setLoading(false);
      Toast.success(t('刷新成功'));
    }, 300);
  };

  const columns = [
    {
      title: t('问卷调查名称'),
      dataIndex: 'name',
      render: (text) => {
        return <div className='font-medium'>{text || '-'}</div>;
      },
    },
    {
      title: t('创建时间'),
      dataIndex: 'created_time',
      render: (text) => {
        return <div>{timestamp2string(text)}</div>;
      },
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      fixed: 'right',
      window: 200,
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

  return (
    <div className='survey-v2 mt-[10px] px-2'>
      <div className='survey-v2-shell'>
        <div className='survey-v2-card'>
          <div className='survey-v2-toolbar'>
            <div className='survey-v2-toolbar-title'>{t('问卷调查')}</div>
            <div className='survey-v2-toolbar-desc'>
              <div className='survey-v2-toolbar-desc-txt'>
                {t('管理问卷调查，可查看、编辑与删除问卷。')}
              </div>
            </div>
          </div>

          <div className='survey-v2-table-area'>
            <CardTable
              columns={columns}
              dataSource={surveys}
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
                  description={t('暂无问卷')}
                  style={{ padding: 30 }}
                />
              }
              className='rounded-xl overflow-hidden'
              size='middle'
            />
          </div>
        </div>
      </div>

      {/* 问卷详情弹窗 */}
      <Modal
        visible={detailVisible}
        onCancel={handleCloseDetail}
        footer={
          <Button theme='solid' onClick={handleCloseDetail}>
            {t('确定')}
          </Button>
        }
      >
        <div className='py-2'>
          <h3 className='text-lg font-semibold mb-3'>{t('问卷详情')}</h3>
          <div className='space-y-2 text-sm'>
            <div>
              <span className='text-gray-500'>{t('问卷调查名称')}：</span>
              <span className='font-medium'>{detailRecord?.name || '-'}</span>
            </div>
            <div>
              <span className='text-gray-500'>{t('创建时间')}：</span>
              <span className='font-medium'>
                {detailRecord
                  ? timestamp2string(detailRecord.created_time)
                  : '-'}
              </span>
            </div>
            <div>
              <span className='text-gray-500'>{t('问题列表')}：</span>
            </div>
            <div className='pl-4'>
              {detailRecord && detailRecord.questions?.length > 0 ? (
                <ol className='list-decimal space-y-1'>
                  {detailRecord.questions.map((q, idx) => (
                    <li key={idx}>{q}</li>
                  ))}
                </ol>
              ) : (
                <span className='text-gray-400'>{t('暂无问题')}</span>
              )}
            </div>
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default QuestionSurvey;
