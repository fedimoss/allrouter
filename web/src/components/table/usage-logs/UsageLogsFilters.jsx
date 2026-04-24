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
import { Button, Form } from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';
import { Columns3, RotateCcw, Search, SlidersHorizontal } from 'lucide-react';
import CompactModeToggle from '../../common/ui/CompactModeToggle';

import { DATE_RANGE_PRESETS } from '../../../constants/console.constants';

const LogsFilters = ({
  formInitValues,
  setFormApi,
  refresh,
  setShowColumnSelector,
  formApi,
  setLogType,
  loading,
  isAdminUser,
  compactMode,
  setCompactMode,
  t,
}) => {
  const [showAdvanced, setShowAdvanced] = useState(false);

  const handleReset = () => {
    if (!formApi) {
      return;
    }

    formApi.reset();
    setLogType(0);
    setTimeout(() => {
      refresh();
    }, 100);
  };

  return (
    <Form
      initValues={formInitValues}
      getFormApi={(api) => setFormApi(api)}
      onSubmit={refresh}
      allowEmpty={true}
      autoComplete='off'
      layout='vertical'
      trigger='change'
      stopValidateWithError={false}
      className='usage-logs-v2-filter-form'
    >
      <div className='usage-logs-v2-filter-header'>
        <div>
          <div className='usage-logs-v2-filter-title'>{t('筛选条件')}</div>
        </div>
        <div className='usage-logs-v2-filter-actions'>
          <Button
            htmlType='submit'
            loading={loading}
            icon={<Search size={15} />}
            className='usage-logs-v2-button usage-logs-v2-button-primary'
          >
            {t('查询')}
          </Button>
          <Button
            type='tertiary'
            onClick={() => setShowAdvanced((value) => !value)}
            icon={<SlidersHorizontal size={15} />}
            className='usage-logs-v2-button usage-logs-v2-button-secondary usage-logs-v2-button-secondary-active'
          >
            {t('高级筛选')}
          </Button>
          <CompactModeToggle
            compactMode={compactMode}
            setCompactMode={setCompactMode}
            t={t}
            className='usage-logs-v2-compact-toggle'
          />
        </div>
      </div>

      <div className='usage-logs-v2-filter-grid'>
        <div className='usage-logs-v2-filter-item usage-logs-v2-filter-item-wide'>
          <div className='usage-logs-v2-filter-label'>{t('时间范围')}</div>
          <Form.DatePicker
            field='dateRange'
            className='usage-logs-v2-control usage-logs-v2-control-range'
            type='dateTimeRange'
            placeholder={[t('开始时间'), t('结束时间')]}
            showClear
            pure
            size='large'
            presets={DATE_RANGE_PRESETS.map((preset) => ({
              text: t(preset.text),
              start: preset.start(),
              end: preset.end(),
            }))}
          />
        </div>

        <div className='usage-logs-v2-filter-item'>
          <div className='usage-logs-v2-filter-label'>{t('令牌名称')}</div>
          <Form.Input
            field='token_name'
            prefix={<IconSearch />}
            placeholder={t('令牌名称')}
            showClear
            pure
            size='large'
            className='usage-logs-v2-control'
          />
        </div>

        <div className='usage-logs-v2-filter-item'>
          <div className='usage-logs-v2-filter-label'>{t('模型名称')}</div>
          <Form.Input
            field='model_name'
            prefix={<IconSearch />}
            placeholder={t('模型名称')}
            showClear
            pure
            size='large'
            className='usage-logs-v2-control'
          />
        </div>
      </div>

      {showAdvanced && (
        <div className='usage-logs-v2-advanced-panel'>
          <div className='usage-logs-v2-advanced-grid'>
            <div className='usage-logs-v2-filter-item'>
              <div className='usage-logs-v2-filter-label'>{t('分组')}</div>
              <Form.Input
                field='group'
                prefix={<IconSearch />}
                placeholder={t('分组')}
                showClear
                pure
                size='large'
                className='usage-logs-v2-control'
              />
            </div>

            <div className='usage-logs-v2-filter-item'>
              <div className='usage-logs-v2-filter-label'>Request ID</div>
              <Form.Input
                field='request_id'
                prefix={<IconSearch />}
                placeholder='Request ID'
                showClear
                pure
                size='large'
                className='usage-logs-v2-control'
              />
            </div>

            <div className='usage-logs-v2-filter-item'>
              <div className='usage-logs-v2-filter-label'>{t('日志类型')}</div>
              <Form.Select
                field='logType'
                placeholder={t('日志类型')}
                size='large'
                showClear
                pure
                className='usage-logs-v2-control'
                onChange={() => {
                  setTimeout(() => {
                    refresh();
                  }, 0);
                }}
              >
                <Form.Select.Option value='0'>{t('全部')}</Form.Select.Option>
                <Form.Select.Option value='1'>{t('充值')}</Form.Select.Option>
                <Form.Select.Option value='2'>{t('消费')}</Form.Select.Option>
                <Form.Select.Option value='3'>{t('管理')}</Form.Select.Option>
                <Form.Select.Option value='4'>{t('系统')}</Form.Select.Option>
                <Form.Select.Option value='5'>{t('错误')}</Form.Select.Option>
                <Form.Select.Option value='6'>{t('退款')}</Form.Select.Option>
              </Form.Select>
            </div>

            {isAdminUser && (
              <>
                <div className='usage-logs-v2-filter-item'>
                  <div className='usage-logs-v2-filter-label'>{t('渠道 ID')}</div>
                  <Form.Input
                    field='channel'
                    prefix={<IconSearch />}
                    placeholder={t('渠道 ID')}
                    showClear
                    pure
                    size='large'
                    className='usage-logs-v2-control'
                  />
                </div>

                <div className='usage-logs-v2-filter-item'>
                  <div className='usage-logs-v2-filter-label'>{t('用户名称')}</div>
                  <Form.Input
                    field='username'
                    prefix={<IconSearch />}
                    placeholder={t('用户名称')}
                    showClear
                    pure
                    size='large'
                    className='usage-logs-v2-control'
                  />
                </div>
              </>
            )}
          </div>
        </div>
      )}

      <div className='usage-logs-v2-filter-footer'>
        <div className='usage-logs-v2-filter-actions'>
          <Button
            type='tertiary'
            onClick={handleReset}
            icon={<RotateCcw size={15} />}
            className='usage-logs-v2-button usage-logs-v2-button-secondary'
          >
            {t('重置')}
          </Button>
          <Button
            type='tertiary'
            onClick={() => setShowColumnSelector(true)}
            icon={<Columns3 size={15} />}
            className='usage-logs-v2-button usage-logs-v2-button-secondary'
          >
            {t('列设置')}
          </Button>
        </div>
      </div>
    </Form>
  );
};

export default LogsFilters;