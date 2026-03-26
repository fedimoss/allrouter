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
import { useTranslation } from 'react-i18next';
import { Button, Input, Popconfirm, Select } from '@douyinfe/semi-ui';
import {
  AlertTriangle,
  CheckCircle2,
  Copy,
  Download,
  FileCheck2,
  FileText,
  RefreshCcw,
  Search,
  Trash2,
} from 'lucide-react';
import { API, getRelativeTime, showError, showSuccess } from '../../../helpers';

const PAGE_SIZE = 10;

const formatLastActiveAt = (value) => {
  if (value === null || value === undefined || value === '') return '-';
  if (typeof value === 'number') {
    const ms = value > 1e12 ? value : value * 1000;
    return getRelativeTime(new Date(ms).toISOString());
  }
  if (value instanceof Date) {
    return getRelativeTime(value.toISOString());
  }
  if (typeof value === 'string') {
    const parsed = Date.parse(value);
    if (!Number.isNaN(parsed)) {
      return getRelativeTime(new Date(parsed).toISOString());
    }
    return value;
  }
  return String(value);
};

const getFileNameFromDisposition = (contentDisposition, fallbackFileName) => {
  if (!contentDisposition || typeof contentDisposition !== 'string') {
    return fallbackFileName;
  }

  const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1].trim().replace(/["']/g, ''));
    } catch (error) {
      return utf8Match[1].trim().replace(/["']/g, '');
    }
  }

  const fileNameMatch = contentDisposition.match(/filename="?([^";]+)"?/i);
  if (fileNameMatch?.[1]) {
    return fileNameMatch[1].trim();
  }

  return fallbackFileName;
};

const providerDotStyleMap = {
  G: {
    backgroundColor: 'var(--semi-color-primary-light-default)',
    color: 'var(--semi-color-primary)',
  },
  K: {
    backgroundColor: 'var(--semi-color-fill-0)',
    color: 'var(--semi-color-text-1)',
  },
  A: {
    backgroundColor: 'var(--semi-color-warning-light-default)',
    color: 'var(--semi-color-warning)',
  },
  Q: {
    backgroundColor: 'rgba(var(--semi-purple-0), 1)',
    color: 'rgba(var(--semi-purple-5), 1)',
  },
  C: {
    backgroundColor: 'var(--semi-color-success-light-default)',
    color: 'var(--semi-color-success)',
  },
};

const MODE_PROVIDER_MAP = {
  1: 'Codex',
  2: 'Anthropic',
  3: 'Qwen',
};

const CertificationList = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState([]);
  const [searchText, setSearchText] = useState('');
  const [searchKeyword, setSearchKeyword] = useState('');
  const [providerFilter, setProviderFilter] = useState('all');
  const [downloadingIds, setDownloadingIds] = useState({});
  const [deletingIds, setDeletingIds] = useState({});
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);

  useEffect(() => {
    const loadData = async () => {
      setLoading(true);
      try {
        const qs =
          `p=${page}&page_size=${PAGE_SIZE}` +
          (searchKeyword
            ? `&keyWord=${encodeURIComponent(searchKeyword)}`
            : '') +
          (providerFilter !== 'all'
            ? `&modelType=${encodeURIComponent(providerFilter)}`
            : '');
        const endpoint = `/api/v0/management/useroauths?${qs}`;
        const res = await API.get(endpoint);
        const { success, message, data } = res?.data || {};
        if (!success) {
          showError(message || t('获取认证文件列表失败'));
          setRows([]);
          setTotal(0);
          return;
        }
        const payload = data || {};
        const items = Array.isArray(payload.items) ? payload.items : [];
        const totalCount = Number(payload.total) || 0;
        const mappedRows = Array.isArray(items)
          ? items.map((item) => {
              const modeType = Number(item.model_type);
              const provider = MODE_PROVIDER_MAP[modeType] ?? 'Unknown';
              const providerShort = provider.trim().slice(0, 1).toUpperCase();
              return {
                id: item.id,
                fileName: item.id,
                provider,
                providerShort,
                successRate: 100,
                sucCount: 0,
                failCount: 0,
                lastActiveAt: formatLastActiveAt(item.updated_at),
              };
            })
          : [];
        setRows(mappedRows);
        setTotal(totalCount || 0);
      } catch (error) {
        console.error('Load oauths error:', error);
        showError(t('加载认证文件列表失败'));
        setRows([]);
        setTotal(0);
      } finally {
        setLoading(false);
      }
    };
    loadData();
  }, [page, providerFilter, searchKeyword, t]);

  const normalizedRows = useMemo(() => rows, [rows]);

  const providerOptions = useMemo(() => {
    return [{ label: t('所有服务商'), value: 'all' }].concat(
      Object.entries(MODE_PROVIDER_MAP).map(([mode, provider]) => ({
        label: provider,
        value: mode,
      })),
    );
  }, [t]);

  const filteredRows = useMemo(() => {
    const keyword = searchKeyword.toLowerCase();
    return normalizedRows.filter((row) => {
      const hitSearch =
        !keyword ||
        row.fileName.toLowerCase().includes(keyword) ||
        row.provider.toLowerCase().includes(keyword);
      const selectedProvider =
        providerFilter === 'all' ? '' : MODE_PROVIDER_MAP[Number(providerFilter)];
      const hitProvider = !selectedProvider || row.provider === selectedProvider;
      return hitSearch && hitProvider;
    });
  }, [normalizedRows, providerFilter, searchKeyword]);

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const pagedRows = useMemo(() => filteredRows, [filteredRows]);

  useEffect(() => {
    if (page > totalPages) setPage(totalPages);
  }, [page, totalPages]);

  const totalCount = total || normalizedRows.length;
  const healthyCount = totalCount;
  const abnormalCount = 0;

  const handleSingleDownload = async (row) => {
    if (!row?.id) return;

    const rowId = String(row.id);
    if (downloadingIds[rowId]) return;

    const fallbackFileName = `oauth-${rowId}.json`;
    setDownloadingIds((prev) => ({ ...prev, [rowId]: true }));

    try {
      const response = await API.get('/api/v0/management/downloadoauth', {
        params: { oauthId: row.id },
        responseType: 'blob',
        disableDuplicate: true,
        skipErrorHandler: true,
      });

      const blob = response?.data instanceof Blob
        ? response.data
        : new Blob([response?.data]);
      const contentDisposition = response?.headers?.['content-disposition'];
      const fileName = getFileNameFromDisposition(contentDisposition, fallbackFileName);

      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = fileName;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
      showSuccess(t('下载成功'));
    } catch (error) {
      console.error('download oauth file error:', error);
      showError(t('下载失败'));
    } finally {
      setDownloadingIds((prev) => {
        const next = { ...prev };
        delete next[rowId];
        return next;
      });
    }
  };

  const handleSearchSubmit = () => {
    const nextKeyword = searchText.trim();
    setPage(1);
    setSearchKeyword(nextKeyword);
  };

  const handleDelete = async (row) => {
    if (!row?.id) return;

    const rowId = String(row.id);
    if (deletingIds[rowId]) return;

    setDeletingIds((prev) => ({ ...prev, [rowId]: true }));
    try {
      const res = await API.delete(
        `/api/v0/management/oauthDelete/${encodeURIComponent(row.id)}`,
        {
          disableDuplicate: true,
          skipErrorHandler: true,
        },
      );
      const { success, message } = res?.data || {};
      if (!success) {
        showError(message || t('删除失败'));
        return;
      }

      setRows((prev) => prev.filter((item) => String(item.id) !== rowId));
      setTotal((prev) => Math.max(0, prev - 1));
      showSuccess(message || t('删除成功'));
    } catch (error) {
      console.error('delete oauth file error:', error);
      showError(t('删除失败'));
    } finally {
      setDeletingIds((prev) => {
        const next = { ...prev };
        delete next[rowId];
        return next;
      });
    }
  };

  const renderHealth = (row) => {
    const healthy = true;
    return (
      <div style={{ minWidth: 170 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div
            style={{
              width: 96,
              height: 8,
              borderRadius: 999,
              overflow: 'hidden',
              background: 'var(--semi-color-fill-0)',
            }}
          >
            <div
              style={{
                height: '100%',
                width: '100%',
                borderRadius: 999,
                backgroundColor: healthy ? 'var(--semi-color-success)' : 'var(--semi-color-danger)',
              }}
            />
          </div>
          <div style={{ fontSize: 14, fontWeight: 600, color: healthy ? 'var(--semi-color-success)' : 'var(--semi-color-danger)' }}>
            100% {t('成功')}
          </div>
        </div>
        <div style={{ marginTop: 6, fontSize: 12, color: 'var(--semi-color-text-2)' }}>
          {`Suc: ${row.sucCount} | Fail: ${row.failCount}`}
        </div>
      </div>
    );
  };

  return (
    <div className='w-full pb-8' style={{ backgroundColor: 'var(--semi-color-fill-0)' }}>
      <div className='mx-auto w-full h-full max-w-[1360px] px-4 pt-4 md:px-8 lg:px-10'>
        <div className='rounded-2xl' style={{ backgroundColor: 'var(--semi-color-fill-0)' }}>
          <div className='flex flex-col gap-4 md:flex-row md:items-start md:justify-between'>
            <div>
              <div className='flex items-center gap-3'>
                <FileCheck2 size={32} style={{ color: 'var(--semi-color-primary)' }} />
                <h2 className='text-[26px] font-semibold leading-none' style={{ color: 'var(--semi-color-text-0)' }}>
                  {t('认证文件管理')}
                </h2>
              </div>
              <p className='mt-3 text-[16px]' style={{ color: 'var(--semi-color-text-1)' }}>
                {t(
                  '集中管理 OAuth 生成的认证文件，监控健康状态，配置模型别名与过滤规则。',
                )}
              </p>
            </div>

            <div className='flex items-center gap-3'>
              <div
                className='flex shrink-0 items-center gap-3 rounded-2xl px-4 backdrop-blur'
                style={{
                  width: '120px',
                  height: '60px',
                  border: '1px solid var(--semi-color-border)',
                  backgroundColor:
                    'color-mix(in srgb, var(--semi-color-bg-0) 88%, transparent)',
                  boxShadow: '0 10px 30px rgba(15,23,42,0.08)',
                }}
              >
                <span className='inline-flex items-center justify-center rounded-full' style={{ backgroundColor: 'var(--semi-color-primary-light-default)', color: 'var(--semi-color-primary)' }}>
                  <FileText size={18} />
                </span>
                <div>
                  <div className='text-[13px]' style={{ color: 'var(--semi-color-text-1)' }}>{t('总文件数')}</div>
                  <div className='text-[20px] font-semibold leading-none' style={{ color: 'var(--semi-color-text-0)' }}>
                    {totalCount}
                  </div>
                </div>
              </div>
              <div
                className='flex shrink-0 items-center gap-3 rounded-2xl px-4 backdrop-blur'
                style={{
                  width: '120px',
                  height: '60px',
                  border: '1px solid var(--semi-color-border)',
                  backgroundColor:
                    'color-mix(in srgb, var(--semi-color-bg-0) 88%, transparent)',
                  boxShadow: '0 10px 30px rgba(15,23,42,0.08)',
                }}
              >
                <span className='inline-flex items-center justify-center rounded-full' style={{ backgroundColor: 'var(--semi-color-success-light-default)', color: 'var(--semi-color-success)' }}>
                  <CheckCircle2 size={18} />
                </span>
                <div>
                  <div className='text-[13px]' style={{ color: 'var(--semi-color-text-1)' }}>{t('健康')}</div>
                  <div className='text-[20px] font-semibold leading-none' style={{ color: 'var(--semi-color-text-0)' }}>
                    {healthyCount}
                  </div>
                </div>
              </div>
              <div
                className='flex shrink-0 items-center gap-3 rounded-2xl px-4 backdrop-blur'
                style={{
                  width: '120px',
                  height: '60px',
                  border: '1px solid color-mix(in srgb, var(--semi-color-danger) 40%, transparent)',
                  boxShadow: '0 10px 30px rgba(239,68,68,0.08)',
                }}
              >
                <span className='inline-flex items-center justify-center rounded-full' style={{ backgroundColor: 'var(--semi-color-danger-light-default)', color: 'var(--semi-color-danger)' }}>
                  <AlertTriangle size={18} color='var(--semi-color-danger)' />
                </span>
                <div>
                  <div className='text-[13px]' style={{ color: 'var(--semi-color-text-1)' }}>{t('异常')}</div>
                  <div
                    className='text-[20px] font-semibold leading-none'
                    style={{ color: 'var(--semi-color-danger)' }}
                  >
                    {abnormalCount}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div className='mt-4 flex flex-col gap-3'>
            <div className='flex flex-1 flex-col gap-3 md:flex-row'>
              <Input
                prefix={<Search size={20} className='ml-2 mr-2' style={{ color: 'var(--semi-color-text-2)' }} />}
                value={searchText}
                onChange={(value) => setSearchText(value)}
                onEnterPress={handleSearchSubmit}
                placeholder={t('搜索文件名或模型...')}
                showClear
                className='w-full md:max-w-[640px]'
                size='large'
                style={{ background: 'var(--semi-color-bg-0)', border: '1px solid var(--semi-color-border)', color: 'var(--semi-color-text-0)' }}
              />
              <Select
                value={providerFilter}
                onChange={(value) => {
                  setProviderFilter(value);
                  setPage(1);
                  setSearchKeyword(searchText.trim());
                }}
                optionList={providerOptions}
                className='w-full md:w-[240px]'
                size='large'
                style={{ background: 'var(--semi-color-bg-0)', border: '1px solid var(--semi-color-border)', color: 'var(--semi-color-text-0)' }}
              />
            </div>
          </div>

          <div
            style={{
              marginTop: 16,
              overflow: 'hidden',
              borderRadius: 12,
              border: '1px solid var(--semi-color-border)',
              background: 'var(--semi-color-bg-0)',
              boxShadow: '0 1px 2px rgba(15,23,42,0.04)',
            }}
          >
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', minWidth: 980, borderCollapse: 'collapse' }}>
                <thead>
                  <tr style={{ height: 48, borderBottom: '1px solid var(--semi-color-border)', background: 'var(--semi-color-fill-0)', textAlign: 'left', color: 'var(--semi-color-text-1)', fontSize: 12, fontWeight: 600 }}>
                    <th style={{ padding: '0 12px' }}>{t('文件信息')}</th>
                    <th style={{ width: 220, padding: '0 12px' }}>{t('服务商')}</th>
                    <th style={{ width: 220, padding: '0 12px' }}>{t('健康状态')}</th>
                    <th style={{ width: 160, padding: '0 12px' }}>{t('最后活跃')}</th>
                    <th style={{ width: 180, padding: '0 24px 0 12px', textAlign: 'right' }}>{t('操作')}</th>
                  </tr>
                </thead>
                <tbody>
                  {pagedRows.map((row) => {
                    const rowId = String(row.id);
                    const isRowDownloading = Boolean(downloadingIds[rowId]);
                    const isRowDeleting = Boolean(deletingIds[rowId]);

                    return (
                    <tr key={row.id} style={{ height: 70, borderTop: '1px solid var(--semi-color-border)', background: 'var(--semi-color-bg-0)' }}>
                      <td style={{ padding: '12px' }}>
                        <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12 }}>
                          <div style={{ marginTop: 2, height: 32, width: 32, borderRadius: 6, background: row.providerShort === 'G' ? 'var(--semi-color-primary-light-default)' : row.providerShort === 'A' ? 'var(--semi-color-warning-light-default)' : 'var(--semi-color-fill-0)', color: row.providerShort === 'G' ? 'var(--semi-color-primary)' : row.providerShort === 'A' ? 'var(--semi-color-warning)' : 'var(--semi-color-text-1)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                            <FileText size={15} />
                          </div>
                          <div>
                            <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 16, fontWeight: 600, lineHeight: 1.2, color: 'var(--semi-color-text-0)' }}>
                              <span>{row.fileName}</span>
                              {/* <Copy size={14} className='text-[#8ca0b8]' /> */}
                            </div>
                            <div style={{ marginTop: 4, fontSize: 13, color: 'var(--semi-color-text-1)' }}>
                              <span>{t('OAuth 凭据')}</span>
                            </div>
                          </div>
                        </div>
                      </td>
                      <td style={{ padding: '0 12px' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 16, color: 'var(--semi-color-text-0)' }}>
                          <span
                            className='inline-flex h-6 w-6 items-center justify-center rounded-full text-[12px] font-semibold'
                            style={providerDotStyleMap[row.providerShort] || providerDotStyleMap.K}
                          >
                            {row.providerShort}
                          </span>
                          <span>{row.provider}</span>
                        </div>
                      </td>
                      <td style={{ padding: '0 12px' }}>{renderHealth(row)}</td>
                      <td style={{ padding: '0 12px', fontSize: 13, color: 'var(--semi-color-text-1)' }}>{row.lastActiveAt}</td>
                      <td style={{ padding: '0 24px 0 12px' }}>
                        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 8, color: 'var(--semi-color-text-2)' }}>
                          {isRowDownloading ? (
                            <RefreshCcw size={15} className='animate-spin' />
                          ) : (
                            <Button
                              theme='borderless'
                              type='tertiary'
                              icon={<Download size={15} />}
                              onClick={() => handleSingleDownload(row)}
                            />
                          )}
                          <Popconfirm
                            title={t('确定要删除此认证文件吗？')}
                            content={t('此操作不可撤销。')}
                            okType='danger'
                            position='leftBottom'
                            onConfirm={() => handleDelete(row)}
                          >
                              {isRowDeleting ? (
                                <RefreshCcw size={15} className='animate-spin' />
                              ) : <Button
                                theme='borderless'
                                type='danger'
                                icon={<Trash2 size={15} />}
                                loading={isRowDeleting}
                                disabled={isRowDeleting}
                              />}
                          </Popconfirm>
                        </div>
                      </td>
                    </tr>
                    );
                  })}
                  {!loading && pagedRows.length === 0 && (
                    <tr>
                      <td colSpan={5} style={{ padding: '40px 0', textAlign: 'center', color: 'var(--semi-color-text-2)' }}>
                        {t('暂无数据')}
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>

            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderTop: '1px solid var(--semi-color-border)', padding: '12px 16px', fontSize: 12, color: 'var(--semi-color-text-1)' }}>
              <div>{`${t('显示第')} ${total === 0 ? 0 : (page - 1) * PAGE_SIZE + 1} ${t('条 - 第')} ${total === 0 ? 0 : (page - 1) * PAGE_SIZE + pagedRows.length} ${t('条，共')} ${total} ${t('条')}`}</div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <Button
                  size='small'
                  disabled={page <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  style={{ fontSize: 12 }}
                >
                  {t('上一页')}
                </Button>
                <Button
                  size='small'
                  disabled={page >= totalPages}
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  style={{ fontSize: 12 }}
                >
                  {t('下一页')}
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default CertificationList;
