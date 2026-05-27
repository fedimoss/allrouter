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

import React, { useEffect, useMemo, useState } from 'react';
import { Button, Empty, Modal, Pagination, Space, Spin, Table, Tag, Typography } from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { API, showError, timestamp2string } from '../../../../helpers';

const { Text, Title } = Typography;

const DEFAULT_PAGE_SIZE = 5;

const getUserDisplayName = (user) => user?.display_name || user?.username || '-';

const getRoleText = (role, t) => {
  switch (role) {
    case 1:
      return t('普通用户');
    case 10:
      return t('管理员');
    case 100:
      return t('超级管理员');
    default:
      return t('未知身份');
  }
};

const safeFormatTime = (value) => {
  if (value === null || value === undefined || value === '') {
    return '-';
  }
  const timestamp = Number(value);
  if (!Number.isFinite(timestamp) || timestamp <= 0) {
    return '-';
  }
  try {
    const full = timestamp2string(timestamp);
    return full.endsWith(':00') ? full.slice(0, 16) : full;
  } catch {
    return '-';
  }
};

const normalizeInviteesResponse = (data, fallbackInviter, page, pageSize) => {
  if (Array.isArray(data)) {
    return {
      inviter: fallbackInviter,
      items: data,
      page,
      pageSize,
      total: data.length,
    };
  }
  return {
    inviter: data?.inviter || fallbackInviter,
    items: Array.isArray(data?.items) ? data.items : [],
    page: Number(data?.page) > 0 ? Number(data.page) : page,
    pageSize: Number(data?.page_size) > 0 ? Number(data.page_size) : pageSize,
    total: Number(data?.total) >= 0 ? Number(data.total) : 0,
  };
};

const InviteUsersDetailModal = ({
  visible,
  onClose,
  user,
  apiPrefix = '/api/user',
  t,
}) => {
  const normalizedApiPrefix = apiPrefix.replace(/\/$/, '');
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState([]);
  const [inviter, setInviter] = useState(user || null);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(DEFAULT_PAGE_SIZE);
  const [total, setTotal] = useState(0);

  const affCount = Number(user?.aff_count) || 0;

  const loadInvitees = async (nextPage = 1) => {
    if (!visible || !user?.id) {
      return;
    }
    setLoading(true);
    try {
      const res = await API.get(`${normalizedApiPrefix}/${user.id}/invitees`, {
        params: {
          p: nextPage,
          page_size: pageSize,
        },
        disableDuplicate: true,
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message || t('加载失败'));
        return;
      }
      const normalized = normalizeInviteesResponse(data, user, nextPage, pageSize);
      setRows(normalized.items);
      setInviter(normalized.inviter);
      setTotal(normalized.total);
      if (normalized.page !== page) {
        setPage(normalized.page);
      }
    } catch (error) {
      showError(error.message || t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!visible) {
      setRows([]);
      setInviter(user || null);
      setTotal(0);
      setPage(1);
      return;
    }
    setPage(1);
    loadInvitees(1).then();
  }, [visible, user?.id]);

  useEffect(() => {
    if (!visible || page === 1) {
      return;
    }
    loadInvitees(page).then();
  }, [page]);

  const columns = useMemo(
    () => [
      {
        title: t('邀请用户ID'),
        dataIndex: 'id',
        width: '28%',
        render: (value) => <Text>{value || '-'}</Text>,
      },
      {
        title: t('邀请用户'),
        dataIndex: 'username',
        width: '35%',
        render: (value, record) => (
          <Text ellipsis={{ showTooltip: true }}>{getUserDisplayName(record)}</Text>
        ),
      },
      {
        title: t('邀请时间'),
        dataIndex: 'created_at',
        width: '37%',
        render: (value) => <Text type='secondary'>{safeFormatTime(value)}</Text>,
      },
    ],
    [t],
  );

  const start = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const end = Math.min(page * pageSize, total);

  return (
    <Modal
      visible={visible}
      onCancel={onClose}
      footer={null}
      centered
      maskClosable
      closable
      width={604}
      bodyStyle={{ padding: '0 22px 18px' }}
      title={
        <div>
          <Title heading={4} className='m-0'>
            {t('邀请用户详情')}
          </Title>
          <Text type='secondary' size='small'>
            {t('查看该用户通过邀请链接带来的注册用户')}
          </Text>
        </div>
      }
    >
      <div style={{ borderTop: '1px solid var(--semi-color-border)', paddingTop: 20 }}>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 16,
            border: '1px solid var(--semi-color-border)',
            borderRadius: 8,
            padding: '14px 16px',
            // background: 'var(--semi-color-fill-0)',
            backgroundColor:'#F8FAFC'
          }}
        >
          <div>
            <div className='sercondary-text' style={{ fontSize:13,color:"#64748B"}}>  
                {t('邀请人信息')}
            </div>   
            <div className='secondary-info'>
               <Space spacing={8} style={{ marginTop: 8, flexWrap: 'wrap' }}>
                <Text strong style={{ fontSize: 16,marginRight:32 }}>
                  {getUserDisplayName(inviter || user)}
                </Text>
                <Tag color='white' shape='circle' style={{marginRight:8}}>
                  ID {inviter?.id || user?.id || '-'}
                </Tag>
                <Tag color='blue' shape='circle'>
                  {getRoleText(inviter?.role ?? user?.role, t)}
                </Tag>
            </Space>

            </div>
           
          </div>
          <div style={{ textAlign: 'right' }}>
            <div className='sercondary-text' style={{ fontSize:13,color:"#64748B"}}>  
              {t('累计邀请')}
            </div>
            <div style={{ color: '#087568', fontSize: 28, fontWeight: 700, lineHeight: 1.2 }}>
              {affCount} {t('人')}
            </div>
          </div>
        </div>

        <div style={{ marginTop: 20, marginBottom: 10, display: 'flex', gap: 24 }}>
          <Text strong>{t('邀请用户列表')}</Text>
          <Text type='tertiary' size='small'>
            {t('按邀请时间倒序展示')}
          </Text>
        </div>

        <Spin spinning={loading}>
          <Table
            columns={columns}
            dataSource={rows}
            rowKey='id'
            pagination={false}
            size='small'
            empty={
              <Empty
                image={<IllustrationNoResult style={{ width: 120, height: 120 }} />}
                darkModeImage={<IllustrationNoResultDark style={{ width: 120, height: 120 }} />}
                description={t('暂无邀请用户')}
              />
            }
          />
        </Spin>

        <div
          style={{
            marginTop: 18,
            paddingTop: 16,
            borderTop: '1px solid var(--semi-color-border)',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            gap: 12,
          }}
        >
          <Text type='secondary' size='small'>
            {t('显示第')} {start} {t('条')} - {t('第')} {end} {t('条')}，{t('共')} {total}{' '}
            {t('条')}
          </Text>
          <Space>
            <Pagination
              total={total}
              pageSize={pageSize}
              currentPage={page}
              onPageChange={setPage}
              size='small'
              showSizeChanger={false}
            />
            <ButtonClose t={t} onClose={onClose} />
          </Space>
        </div>
      </div>
    </Modal>
  );
};

const ButtonClose = ({ t, onClose }) => (
  <Button size='small' onClick={onClose}>
    {t('关闭')}
  </Button>
);

export default InviteUsersDetailModal;
