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
import {
  Modal,
  Button,
  Input,
  Table,
  Tag,
  Typography,
  Select,
  Switch,
  Banner,
  Tooltip,
} from '@douyinfe/semi-ui';
import { IconSearch, IconInfoCircle } from '@douyinfe/semi-icons';
import { Settings } from 'lucide-react';
import { copy, showError, showInfo, showSuccess } from '../../../../helpers';
import {
  MODEL_TABLE_PAGE_SIZE,
  CHANNEL_TYPE_RESPONSES_CHAT,
  RESPONSES_CHAT_TEST_PROTOCOLS,
  modelTestKey,
  modelTestingKey,
} from '../../../../constants';

// Responses→Chat 渠道逐协议测试按钮/状态展示元数据
const PROTO_META = {
  'openai-response': {
    short: 'Resp',
    path: '/v1/responses',
    color: 'var(--semi-color-purple)',
  },
  'openai': {
    short: 'Chat',
    path: '/v1/chat/completions',
    color: 'var(--semi-color-blue)',
  },
};

const ModelTestModal = ({
  showModelTestModal,
  currentTestChannel,
  handleCloseModal,
  isBatchTesting,
  batchTestModels,
  modelSearchKeyword,
  setModelSearchKeyword,
  selectedModelKeys,
  setSelectedModelKeys,
  modelTestResults,
  testingModels,
  testChannel,
  modelTablePage,
  setModelTablePage,
  selectedEndpointType,
  setSelectedEndpointType,
  isStreamTest,
  setIsStreamTest,
  allSelectingRef,
  isMobile,
  t,
}) => {
  const hasChannel = Boolean(currentTestChannel);
  // Responses→Chat 渠道:同一模型同时支持 /v1/responses 与 /v1/chat/completions,
  // 测试时每模型提供 Response、Chat 两个独立按钮与两行独立状态。
  const isRespChatChannel =
    hasChannel && currentTestChannel.type === CHANNEL_TYPE_RESPONSES_CHAT;
  // Responses→Chat 的两种协议都支持流式,流式开关始终可用;其余渠道按端点类型决定
  const streamToggleDisabled =
    !isRespChatChannel &&
    [
      'embeddings',
      'image-generation',
      'jina-rerank',
      'openai-response-compact',
    ].includes(selectedEndpointType);

  React.useEffect(() => {
    if (streamToggleDisabled && isStreamTest) {
      setIsStreamTest(false);
    }
  }, [streamToggleDisabled, isStreamTest, setIsStreamTest]);

  const filteredModels = hasChannel
    ? currentTestChannel.models
        .split(',')
        .filter((model) =>
          model.toLowerCase().includes(modelSearchKeyword.toLowerCase()),
        )
    : [];

  const endpointTypeOptions = [
    { value: '', label: t('自动检测') },
    { value: 'openai', label: 'OpenAI (/v1/chat/completions)' },
    { value: 'openai-response', label: 'OpenAI Response (/v1/responses)' },
    {
      value: 'openai-response-compact',
      label: 'OpenAI Response Compaction (/v1/responses/compact)',
    },
    { value: 'anthropic', label: 'Anthropic (/v1/messages)' },
    {
      value: 'gemini',
      label: 'Gemini (/v1beta/models/{model}:generateContent)',
    },
    { value: 'jina-rerank', label: 'Jina Rerank (/v1/rerank)' },
    {
      value: 'image-generation',
      label: t('图像生成') + ' (/v1/images/generations)',
    },
    { value: 'embeddings', label: 'Embeddings (/v1/embeddings)' },
  ];

  const handleCopySelected = () => {
    if (selectedModelKeys.length === 0) {
      showError(t('请先选择模型！'));
      return;
    }
    copy(selectedModelKeys.join(',')).then((ok) => {
      if (ok) {
        showSuccess(
          t('已复制 ${count} 个模型').replace(
            '${count}',
            selectedModelKeys.length,
          ),
        );
      } else {
        showError(t('复制失败，请手动复制'));
      }
    });
  };

  const handleSelectSuccess = () => {
    if (!currentTestChannel) return;
    const successKeys = currentTestChannel.models
      .split(',')
      .filter((m) => m.toLowerCase().includes(modelSearchKeyword.toLowerCase()))
      .filter((m) => {
        if (isRespChatChannel) {
          // Responses→Chat 渠道:任一协议成功即视为该模型可用
          return RESPONSES_CHAT_TEST_PROTOCOLS.some(
            (proto) =>
              modelTestResults[
                modelTestKey(
                  currentTestChannel.type,
                  currentTestChannel.id,
                  m,
                  proto,
                )
              ]?.success,
          );
        }
        const result = modelTestResults[`${currentTestChannel.id}-${m}`];
        return result && result.success;
      });
    if (successKeys.length === 0) {
      showInfo(t('暂无成功模型'));
    }
    setSelectedModelKeys(successKeys);
  };

  // 渲染单个协议的测试状态(withPrefix=true 时附色点+短标签前缀,仅 Responses→Chat 渠道用)。
  // 非 Responses→Chat 渠道调用时 proto 仅作占位,键构造退化为单键,行为与原实现一致。
  const renderStatusForProto = (model, proto, withPrefix) => {
    const testResult =
      modelTestResults[
        modelTestKey(currentTestChannel.type, currentTestChannel.id, model, proto)
      ];
    const isTesting = testingModels.has(
      modelTestingKey(currentTestChannel.type, model, proto),
    );
    const prefix = withPrefix ? (
      <>
        <span
          aria-hidden
          style={{
            width: 7,
            height: 7,
            borderRadius: '50%',
            background: PROTO_META[proto].color,
            display: 'inline-block',
            flexShrink: 0,
          }}
        />
        <Typography.Text
          type='tertiary'
          size='small'
          className='shrink-0'
          style={{ minWidth: 32 }}
        >
          {PROTO_META[proto].short}
        </Typography.Text>
      </>
    ) : null;

    if (isTesting) {
      return (
        <div className='flex items-center gap-2'>
          {prefix}
          <Tag color='blue' shape='circle'>
            {t('测试中')}
          </Tag>
        </div>
      );
    }
    if (!testResult) {
      return (
        <div className='flex items-center gap-2'>
          {prefix}
          <Tag color='grey' shape='circle'>
            {t('未开始')}
          </Tag>
        </div>
      );
    }
    return (
      <div className='flex flex-col gap-1'>
        <div className='flex items-center gap-2'>
          {prefix}
          <Tag color={testResult.success ? 'green' : 'red'} shape='circle'>
            {testResult.success ? t('成功') : t('失败')}
          </Tag>
          {testResult.success && (
            <Typography.Text type='tertiary'>
              {t('请求时长: ${time}s').replace(
                '${time}',
                testResult.time.toFixed(2),
              )}
            </Typography.Text>
          )}
        </div>
        {!testResult.success && testResult.message && (
          <div className='flex flex-col gap-1'>
            <Typography.Text
              type='danger'
              size='small'
              className='break-all'
              style={{ maxWidth: '400px', fontSize: '12px' }}
            >
              {testResult.message}
            </Typography.Text>
            {testResult.errorCode === 'model_price_error' && (
              <Button
                size='small'
                theme='light'
                type='warning'
                icon={<Settings size={12} />}
                onClick={() =>
                  window.open('/console/setting?tab=ratio', '_blank')
                }
                style={{ width: 'fit-content' }}
              >
                {t('前往设置')}
              </Button>
            )}
          </div>
        )}
      </div>
    );
  };

  const columns = [
    {
      title: t('模型名称'),
      dataIndex: 'model',
      render: (text) => (
        <div className='flex items-center'>
          <Typography.Text strong>{text}</Typography.Text>
        </div>
      ),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (text, record) => {
        if (isRespChatChannel) {
          return (
            <div className='flex flex-col gap-2'>
              {RESPONSES_CHAT_TEST_PROTOCOLS.map((proto) => (
                <React.Fragment key={proto}>
                  {renderStatusForProto(record.model, proto, true)}
                </React.Fragment>
              ))}
            </div>
          );
        }
        return renderStatusForProto(record.model, selectedEndpointType, false);
      },
    },
    {
      title: '',
      dataIndex: 'operate',
      render: (text, record) => {
        if (isRespChatChannel) {
          return (
            <div className='flex items-center justify-end gap-1'>
              {RESPONSES_CHAT_TEST_PROTOCOLS.map((proto) => {
                const meta = PROTO_META[proto];
                const isTesting = testingModels.has(
                  modelTestingKey(currentTestChannel.type, record.model, proto),
                );
                return (
                  <Tooltip key={proto} content={meta.path}>
                    <Button
                      size='small'
                      loading={isTesting}
                      onClick={() =>
                        testChannel(
                          currentTestChannel,
                          record.model,
                          proto,
                          isStreamTest,
                        )
                      }
                    >
                      <span
                        aria-hidden
                        style={{
                          width: 7,
                          height: 7,
                          borderRadius: '50%',
                          background: meta.color,
                          display: 'inline-block',
                          marginRight: 5,
                        }}
                      />
                      {t(meta.short)}
                    </Button>
                  </Tooltip>
                );
              })}
            </div>
          );
        }
        const isTesting = testingModels.has(
          modelTestingKey(
            currentTestChannel.type,
            record.model,
            selectedEndpointType,
          ),
        );
        return (
          <Button
            type='tertiary'
            onClick={() =>
              testChannel(
                currentTestChannel,
                record.model,
                selectedEndpointType,
                isStreamTest,
              )
            }
            loading={isTesting}
            size='small'
          >
            {t('测试')}
          </Button>
        );
      },
    },
  ];

  const dataSource = (() => {
    if (!hasChannel) return [];
    const start = (modelTablePage - 1) * MODEL_TABLE_PAGE_SIZE;
    const end = start + MODEL_TABLE_PAGE_SIZE;
    return filteredModels.slice(start, end).map((model) => ({
      model,
      key: model,
    }));
  })();

  return (
    <Modal
      title={
        hasChannel ? (
          <div className='flex flex-col gap-2 w-full'>
            <div className='flex items-center gap-2'>
              <Typography.Text
                strong
                className='!text-[var(--semi-color-text-0)] !text-base'
              >
                {currentTestChannel.name} {t('渠道的模型测试')}
              </Typography.Text>
              <Typography.Text type='tertiary' size='small'>
                {t('共')} {currentTestChannel.models.split(',').length}{' '}
                {t('个模型')}
              </Typography.Text>
            </div>
          </div>
        ) : null
      }
      visible={showModelTestModal}
      onCancel={handleCloseModal}
      footer={
        hasChannel ? (
          <div className='flex justify-end'>
            {isBatchTesting ? (
              <Button type='danger' onClick={handleCloseModal}>
                {t('停止测试')}
              </Button>
            ) : (
              <Button type='tertiary' onClick={handleCloseModal}>
                {t('取消')}
              </Button>
            )}
            <Button
              onClick={batchTestModels}
              loading={isBatchTesting}
              disabled={isBatchTesting}
            >
              {isBatchTesting
                ? t('测试中...')
                : t('批量测试${count}个模型').replace(
                    '${count}',
                    filteredModels.length,
                  )}
            </Button>
          </div>
        ) : null
      }
      maskClosable={!isBatchTesting}
      className='!rounded-lg'
      size={isMobile ? 'full-width' : 'large'}
    >
      {hasChannel && (
        <div className='model-test-scroll'>
          {/* Endpoint toolbar */}
          <div className='flex flex-col sm:flex-row sm:items-center gap-2 w-full mb-2'>
            {isRespChatChannel ? (
              <div className='flex items-center gap-2 flex-1 min-w-0'>
                <Typography.Text type='tertiary' size='small'>
                  {t(
                    'Responses→Chat 渠道：每个模型提供 Response 与 Chat 两种测试',
                  )}
                </Typography.Text>
              </div>
            ) : (
              <div className='flex items-center gap-2 flex-1 min-w-0'>
                <Typography.Text strong className='shrink-0'>
                  {t('端点类型')}:
                </Typography.Text>
                <Select
                  value={selectedEndpointType}
                  onChange={setSelectedEndpointType}
                  optionList={endpointTypeOptions}
                  className='!w-full min-w-0'
                  placeholder={t('选择端点类型')}
                />
              </div>
            )}
            <div className='flex items-center justify-between sm:justify-end gap-2 shrink-0'>
              <Typography.Text strong className='shrink-0'>
                {t('流式')}:
              </Typography.Text>
              <Switch
                checked={isStreamTest}
                onChange={setIsStreamTest}
                size='small'
                disabled={streamToggleDisabled}
                aria-label={t('流式')}
              />
            </div>
          </div>

          <Banner
            type='info'
            closeIcon={null}
            icon={<IconInfoCircle />}
            className='!rounded-lg mb-2'
            description={t(
              '说明：本页测试为非流式请求；若渠道仅支持流式返回，可能出现测试失败，请以实际使用为准。',
            )}
          />

          {/* 搜索与操作按钮 */}
          <div className='flex flex-col sm:flex-row sm:items-center gap-2 w-full mb-2'>
            <Input
              placeholder={t('搜索模型...')}
              value={modelSearchKeyword}
              onChange={(v) => {
                setModelSearchKeyword(v);
                setModelTablePage(1);
              }}
              className='!w-full sm:!flex-1'
              prefix={<IconSearch />}
              showClear
            />

            <div className='flex items-center justify-end gap-2'>
              <Button onClick={handleCopySelected}>{t('复制已选')}</Button>
              <Button type='tertiary' onClick={handleSelectSuccess}>
                {t('选择成功')}
              </Button>
            </div>
          </div>

          <Table
            columns={columns}
            dataSource={dataSource}
            rowSelection={{
              selectedRowKeys: selectedModelKeys,
              onChange: (keys) => {
                if (allSelectingRef.current) {
                  allSelectingRef.current = false;
                  return;
                }
                setSelectedModelKeys(keys);
              },
              onSelectAll: (checked) => {
                allSelectingRef.current = true;
                setSelectedModelKeys(checked ? filteredModels : []);
              },
            }}
            pagination={{
              currentPage: modelTablePage,
              pageSize: MODEL_TABLE_PAGE_SIZE,
              total: filteredModels.length,
              showSizeChanger: false,
              onPageChange: (page) => setModelTablePage(page),
            }}
          />
        </div>
      )}
    </Modal>
  );
};

export default ModelTestModal;
