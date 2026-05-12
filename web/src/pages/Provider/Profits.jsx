import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Empty, Form, Space, Tag, Typography } from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';
import { Coins, RotateCcw, Search } from 'lucide-react';
import CardTable from '../../components/common/ui/CardTable';
import { DATE_RANGE_PRESETS } from '../../constants/console.constants';
import {
  API,
  getTodayStartTimestamp,
  renderQuota,
  showError,
  timestamp2string,
} from '../../helpers';
import { createCardProPagination } from '../../helpers/utils';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import '../Log/log-v2.css';

const ProviderProfitsPage = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [formApi, setFormApi] = useState(null);
  const [loading, setLoading] = useState(false);
  const [records, setRecords] = useState([]);
  const [summary, setSummary] = useState({});
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(parseInt(localStorage.getItem('page-size')) || 10);
  const [total, setTotal] = useState(0);

  const now = new Date();
  const formInitValues = {
    provider_user_id: '',
    model_name: '',
    request_id: '',
    dateRange: [
      timestamp2string(getTodayStartTimestamp()),
      timestamp2string(now.getTime() / 1000 + 3600),
    ],
  };

  const getQueryValues = () => {
    const values = formApi ? formApi.getValues() : formInitValues;
    const dateRange = Array.isArray(values.dateRange) ? values.dateRange : formInitValues.dateRange;
    return {
      providerUserId: values.provider_user_id || '',
      modelName: values.model_name || '',
      requestId: values.request_id || '',
      startTimestamp: Date.parse(dateRange[0]) / 1000,
      endTimestamp: Date.parse(dateRange[1]) / 1000,
    };
  };

  const loadProfits = async (page = activePage, size = pageSize) => {
    setLoading(true);
    const query = getQueryValues();
    const url = encodeURI(
      `/api/provider/profits?p=${page}&page_size=${size}&provider_user_id=${query.providerUserId}&model_name=${query.modelName}&request_id=${query.requestId}&start_timestamp=${query.startTimestamp}&end_timestamp=${query.endTimestamp}`,
    );
    try {
      const res = await API.get(url);
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      const pageData = data?.page || {};
      setRecords(pageData.items || []);
      setSummary(data?.summary || {});
      setActivePage(pageData.page || page);
      setPageSize(pageData.page_size || size);
      setTotal(pageData.total || 0);
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadProfits(1, pageSize);
  }, [formApi]);

  const resetFilters = () => {
    formApi?.reset();
    setTimeout(() => loadProfits(1, pageSize), 100);
  };

  const summaryCards = [
    { key: 'provider_user_quota', label: t('用户收费') },
    { key: 'base_cost_quota', label: t('基础成本') },
    { key: 'covered_cost_quota', label: t('用户覆盖成本') },
    { key: 'owner_cost_quota', label: t('服务商承担') },
    { key: 'profit_quota', label: t('利润') },
  ];

  const columns = useMemo(
    () => [
      {
        title: t('时间'),
        dataIndex: 'created_at',
        render: (value) => timestamp2string(value),
      },
      {
        title: t('服务商用户'),
        dataIndex: 'provider_user_id',
        render: (value) => <Tag shape='circle'>#{value}</Tag>,
      },
      {
        title: t('公开模型'),
        dataIndex: 'public_model_name',
      },
      {
        title: t('基础模型'),
        dataIndex: 'base_model_name',
      },
      {
        title: t('用户收费'),
        dataIndex: 'provider_user_quota',
        render: (value) => renderQuota(value, 6),
      },
      {
        title: t('基础成本'),
        dataIndex: 'base_cost_quota',
        render: (value) => renderQuota(value, 6),
      },
      {
        title: t('已支付'),
        dataIndex: 'paid_quota',
        render: (value) => renderQuota(value, 6),
      },
      {
        title: t('服务商承担'),
        dataIndex: 'owner_cost_quota',
        render: (value) => renderQuota(value, 6),
      },
      {
        title: t('利润'),
        dataIndex: 'profit_quota',
        render: (value) => <Typography.Text strong>{renderQuota(value, 6)}</Typography.Text>,
      },
      {
        title: 'Request ID',
        dataIndex: 'request_id',
        render: (value) => (
          <Typography.Text copyable ellipsis={{ showTooltip: true }} style={{ maxWidth: 220 }}>
            {value}
          </Typography.Text>
        ),
      },
    ],
    [t],
  );

  const paginationArea = createCardProPagination({
    currentPage: activePage,
    pageSize,
    total,
    onPageChange: (page) => loadProfits(page, pageSize),
    onPageSizeChange: (size) => {
      localStorage.setItem('page-size', `${size}`);
      loadProfits(1, size);
    },
    isMobile,
    t,
  });

  return (
    <div className='log-v2-page mt-[10px] px-2'>
      <div className='log-v2 usage-logs-v2'>
        <div className='log-v2-shell'>
          <div className='log-v2-stack'>
            <section className='usage-logs-v2-header'>
              <div className='usage-logs-v2-title'>{t('服务商利润')}</div>
              <p className='usage-logs-v2-description'>
                {t('查看当前服务商下用户调用产生的收费、成本和利润流水。')}
              </p>
            </section>

            <section className='usage-logs-v2-stats'>
              <div className='usage-logs-v2-stat-grid'>
                {summaryCards.map((item) => (
                  <div key={item.key} className='usage-logs-v2-stat-card usage-logs-v2-stat-accent-quota'>
                    <div className='usage-logs-v2-stat-head'>
                      <div className='usage-logs-v2-stat-label'>{item.label}</div>
                      <div className='usage-logs-v2-stat-icon usage-logs-v2-stat-icon-quota'>
                        <Coins size={16} />
                      </div>
                    </div>
                    <div className='usage-logs-v2-stat-value'>{renderQuota(summary[item.key] || 0, 6)}</div>
                  </div>
                ))}
              </div>
            </section>

            <section className='log-v2-filter-card usage-logs-v2-filter-card'>
              <Form
                initValues={formInitValues}
                getFormApi={setFormApi}
                onSubmit={() => loadProfits(1, pageSize)}
                allowEmpty
                layout='vertical'
                className='usage-logs-v2-filter-form'
              >
                <div className='usage-logs-v2-filter-header'>
                  <div className='usage-logs-v2-filter-title'>{t('筛选条件')}</div>
                  <div className='usage-logs-v2-filter-actions'>
                    <Button htmlType='submit' loading={loading} icon={<Search size={15} />} className='usage-logs-v2-button usage-logs-v2-button-primary'>
                      {t('查询')}
                    </Button>
                    <Button type='tertiary' onClick={resetFilters} icon={<RotateCcw size={15} />} className='usage-logs-v2-button usage-logs-v2-button-secondary'>
                      {t('重置')}
                    </Button>
                  </div>
                </div>
                <div className='usage-logs-v2-filter-grid'>
                  <div className='usage-logs-v2-filter-item usage-logs-v2-filter-item-wide'>
                    <div className='usage-logs-v2-filter-label'>{t('时间范围')}</div>
                    <Form.DatePicker
                      field='dateRange'
                      type='dateTimeRange'
                      showClear
                      pure
                      size='large'
                      className='usage-logs-v2-control usage-logs-v2-control-range'
                      presets={DATE_RANGE_PRESETS.map((preset) => ({
                        text: t(preset.text),
                        start: preset.start(),
                        end: preset.end(),
                      }))}
                    />
                  </div>
                  <div className='usage-logs-v2-filter-item'>
                    <div className='usage-logs-v2-filter-label'>{t('服务商用户 ID')}</div>
                    <Form.Input field='provider_user_id' prefix={<IconSearch />} showClear pure size='large' className='usage-logs-v2-control' />
                  </div>
                  <div className='usage-logs-v2-filter-item'>
                    <div className='usage-logs-v2-filter-label'>{t('模型名称')}</div>
                    <Form.Input field='model_name' prefix={<IconSearch />} showClear pure size='large' className='usage-logs-v2-control' />
                  </div>
                  <div className='usage-logs-v2-filter-item'>
                    <div className='usage-logs-v2-filter-label'>Request ID</div>
                    <Form.Input field='request_id' prefix={<IconSearch />} showClear pure size='large' className='usage-logs-v2-control' />
                  </div>
                </div>
              </Form>
            </section>

            <section className='log-v2-table-card usage-logs-v2-table-card'>
              <div className='usage-logs-v2-table-wrap'>
                <CardTable
                  columns={columns}
                  dataSource={records}
                  rowKey='id'
                  loading={loading}
                  scroll={{ x: 'max-content' }}
                  hidePagination
                  empty={<Empty description={t('暂无服务商利润数据')} style={{ padding: 40 }} />}
                  className='usage-logs-v2-table rounded-[24px] overflow-hidden'
                  size='small'
                />
              </div>
              {paginationArea && <div className='usage-logs-v2-pagination'>{paginationArea}</div>}
            </section>
          </div>
        </div>
      </div>
    </div>
  );
};

export default ProviderProfitsPage;
