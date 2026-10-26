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

import React, { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Button, Card, Form, Radio, Select, Toast } from '@douyinfe/semi-ui';
import { IconSend, IconTick } from '@douyinfe/semi-icons';
import { API, getSystemName } from '../../helpers';
import './index.css';

const MAX_FILES = 3;
const MAX_FILE_SIZE = 10 * 1024 * 1024;
const FILE_ACCEPT =
  '.pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.txt,.md,.csv,.json,.zip,.png,.jpg,.jpeg';
const FILE_EXTENSION =
  /\.(pdf|docx?|xlsx?|pptx?|txt|md|csv|json|zip|png|jpe?g)$/i;

// DatePicker 表单值为所选日期本地零点的 Date 对象，直接 JSON 序列化会走 toISOString()
// 转成 UTC（东八区 10-19 00:00 → 2026-10-18T16:00:00.000Z，日期倒退一天），
// 提交前按本地日期格式化为 yyyy-MM-dd 字符串
const formatLocalDate = (value) => {
  if (value instanceof Date && !Number.isNaN(value.getTime())) {
    const y = value.getFullYear();
    const m = String(value.getMonth() + 1).padStart(2, '0');
    const d = String(value.getDate()).padStart(2, '0');
    return `${y}-${m}-${d}`;
  }
  return value;
};

const CooperationApplication = () => {
  const { t } = useTranslation();
  const systemName = getSystemName();
  const formApiRef = useRef(null);
  const resultRef = useRef(null);
  const fileInputRef = useRef(null);
  const [formKey, setFormKey] = useState(0);
  const [testPackage, setTestPackage] = useState('');
  // 附件：选中即上传，提交时仅将 URL 随表单存入 survey_data.files
  const [files, setFiles] = useState([]);
  const [fileError, setFileError] = useState('');
  const [dragging, setDragging] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const requiresAcknowledgement =
    testPackage === 'standard' || testPackage === 'gray';

  useEffect(() => {
    if (submitted) {
      resultRef.current?.focus();
      resultRef.current?.scrollIntoView({
        behavior: 'smooth',
        block: 'nearest',
      });
    }
  }, [submitted]);

  const optionalLabel = (label) => (
    <span className='cooperation-application__label'>
      {label}
      <small>{t('选填')}</small>
    </span>
  );

  const requiredRule = (message) => [
    { required: true, whitespace: true, message },
  ];

  const handleReset = () => {
    setFormKey((key) => key + 1);
    setTestPackage('');
    setFiles([]);
    setFileError('');
    setDragging(false);
    setSubmitted(false);
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const handlePackageChange = (event) => {
    const value = event.target.value;
    setTestPackage(value);
    if (value !== 'standard' && value !== 'gray') {
      formApiRef.current?.setValue('testAck', false);
    }
  };

  // 附件上传：与 /userQuestion 截图共用 /api/user/questionnaire/upload，
  // 未登录时后端按 domain 匹配服务商站点，与问卷提交的归属逻辑一致
  const uploadFile = async (file) => {
    const id = `${file.name}-${file.size}-${Date.now()}`;
    setFiles((prev) => [
      ...prev,
      { id, name: file.name, size: file.size, uploading: true },
    ]);
    try {
      const formData = new FormData();
      formData.append('file', file);
      formData.append('domain', window.location.hostname);
      const res = await API.post(
        '/api/user/questionnaire/upload',
        formData,
        { headers: { 'Content-Type': 'multipart/form-data' } },
      );
      const { success, message, data } = res.data || {};
      if (!success || !data?.url) {
        throw new Error(message || t('资料上传失败'));
      }
      setFiles((prev) =>
        prev.map((f) =>
          f.id === id ? { ...f, url: data.url, uploading: false } : f,
        ),
      );
    } catch (error) {
      // 上传失败的条目直接移除，仅保留提示
      setFiles((prev) => prev.filter((f) => f.id !== id));
      Toast.error(error?.message || t('资料上传失败'));
    }
  };

  // 校验通过的文件立即上传；放弃表单或移除的文件由后台孤儿清理任务回收
  const addFiles = (incoming) => {
    let error = '';
    let count = files.length;
    Array.from(incoming).forEach((file) => {
      if (!FILE_EXTENSION.test(file.name) || file.size > MAX_FILE_SIZE) {
        error = t('仅支持指定格式且单个不超过 10 MB 的文件。');
        return;
      }
      if (files.some((picked) => picked.name === file.name && picked.size === file.size)) {
        return;
      }
      if (count >= MAX_FILES) {
        error = t('最多选择 3 个文件。');
        return;
      }
      count += 1;
      uploadFile(file);
    });
    setFileError(error);
  };

  const handleSubmit = async (values) => {
    if (files.some((f) => f.uploading)) {
      Toast.warning(t('资料正在上传，请稍候'));
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.post('/api/user/questionnaire', {
        survey_type: 2, // 合作申请问卷（后端 SurveyTypeCooperation）
        survey_data: {
          ...values,
          // 期望日期为 Date 对象，先格式化成本地日期串再随表单存入
          windowDate: formatLocalDate(values.windowDate),
          // 附件信息随表单一并存入 survey_data JSON，无需后端加列
          files: files
            .filter((f) => f.url)
            .map((f) => ({ url: f.url, name: f.name, size: f.size })),
        },
        // 未登录时后端按此域名匹配服务商；已登录时以用户 provider_id 为准
        domain: window.location.hostname,
      });
      if (res.data.success) {
        setSubmitted(true);
      } else {
        Toast.error(res.data.message || t('提交失败，请稍后重试'));
      }
    } catch (err) {
      Toast.error(t('提交失败，请稍后重试'));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <main className='cooperation-application'>
      <header className='cooperation-application__header'>
        <Link className='cooperation-application__brand' to='/'>
          {systemName}
        </Link>
        <h1>{t('合作申请')}</h1>
        <p>{t('告诉我们您的合作方向和需求，我们将尽快与您联系。')}</p>
      </header>

      <Card className='cooperation-application__card' bordered={false}>
        <div hidden={submitted}>
          <Form
            key={formKey}
            className='cooperation-application__form'
            layout='vertical'
            initValues={{ timeline: 'asap', windowPreference: '' }}
            getFormApi={(api) => {
              formApiRef.current = api;
            }}
            onSubmit={handleSubmit}
          >
            <div className='cooperation-application__grid'>
              <Form.Input
                field='company'
                label={t('公司 / 机构')}
                maxLength={120}
                autoComplete='organization'
                placeholder={t('请输入公司或机构名称')}
                rules={requiredRule(t('请填写公司或机构名称'))}
              />
              <Form.Input
                field='contact'
                label={t('联系人')}
                maxLength={60}
                autoComplete='name'
                placeholder={t('请输入联系人姓名')}
                rules={requiredRule(t('请填写联系人姓名'))}
              />
              <Form.Input
                field='title'
                label={optionalLabel(t('职务'))}
                maxLength={60}
                autoComplete='organization-title'
                placeholder={t('例如：技术负责人')}
              />
              <Form.Input
                field='email'
                type='email'
                label={t('邮箱')}
                maxLength={120}
                autoComplete='email'
                placeholder={t('请输入工作邮箱')}
                transform={(value) => value?.trim()}
                rules={[
                  { required: true, message: t('请填写有效的邮箱') },
                  {
                    pattern: /^[^\s@]+@[^\s@]+\.[^\s@]+$/,
                    message: t('请填写有效的邮箱'),
                  },
                ]}
              />
              <Form.Input
                field='phone'
                type='tel'
                label={optionalLabel(t('电话'))}
                maxLength={40}
                autoComplete='tel'
                placeholder={t('请输入联系电话')}
              />
              <Form.Input
                field='wechat'
                label={optionalLabel(t('微信'))}
                maxLength={60}
                placeholder={t('请输入微信号')}
              />
              <Form.Select
                field='cooperationType'
                label={t('合作方向')}
                placeholder={t('请选择合作方向')}
                rules={requiredRule(t('请选择合作方向'))}
              >
                <Select.Option value='算力机'>{t('算力机')}</Select.Option>
                <Select.Option value='企业 API'>{t('企业 API')}</Select.Option>
              </Form.Select>
              <Form.Input
                field='concurrency'
                label={optionalLabel(t('并发目标'))}
                maxLength={40}
                placeholder={t('如：50 路并发')}
              />
              <Form.Input
                field='contextLength'
                label={optionalLabel(t('上下文长度'))}
                maxLength={40}
                placeholder={t('如：128K')}
              />
              <Form.Input
                field='models'
                label={optionalLabel(t('目标模型'))}
                maxLength={200}
                placeholder={t('如：DeepSeek、Kimi 等')}
              />
              <Form.Select
                field='timeline'
                label={optionalLabel(t('期望时间'))}
              >
                <Select.Option value='asap'>{t('尽快')}</Select.Option>
                <Select.Option value='1m'>{t('一个月内')}</Select.Option>
                <Select.Option value='3m'>{t('三个月内')}</Select.Option>
                <Select.Option value='eval'>{t('前期评估')}</Select.Option>
              </Form.Select>

              <div className='cooperation-application__full'>
                <Form.RadioGroup
                  field='testPackage'
                  label={optionalLabel(t('测试包 / 沟通方式'))}
                  className='cooperation-application__choices'
                  type='pureCard'
                  onChange={handlePackageChange}
                >
                  <Radio
                    value='standard'
                    extra={t('在约定窗口完成基础验证，并提供测试结果说明。')}
                  >
                    {t('标准验证测试')}
                  </Radio>
                  <Radio
                    value='gray'
                    extra={t('结合实际业务场景，进一步确认接入和运行表现。')}
                  >
                    {t('业务灰度测试')}
                  </Radio>
                  <Radio
                    value='consult'
                    extra={t('先沟通现有方案和需求，再确定后续安排。')}
                  >
                    {t('先咨询，暂不测试')}
                  </Radio>
                </Form.RadioGroup>
                {requiresAcknowledgement && (
                  <div className='cooperation-application__acknowledgement'>
                    <Form.Checkbox
                      field='testAck'
                      noLabel
                      trigger='change'
                      validate={(value) =>
                        value ? undefined : t('请确认测试安排后继续')
                      }
                    >
                      {t(
                        '我了解：测试的具体安排、费用及验收标准以双方后续确认的合作方案为准。',
                      )}
                    </Form.Checkbox>
                  </div>
                )}
              </div>

              <Form.Select
                field='windowPreference'
                label={optionalLabel(t('期望窗口'))}
              >
                <Select.Option value=''>{t('均可')}</Select.Option>
                <Select.Option value='weekday'>{t('工作日窗口')}</Select.Option>
                <Select.Option value='weekend'>{t('周末窗口')}</Select.Option>
              </Form.Select>
              <Form.DatePicker
                field='windowDate'
                label={optionalLabel(t('期望日期'))}
                format='yyyy-MM-dd'
                placeholder={t('请选择日期')}
              />
              <Form.TextArea
                field='requirement'
                fieldClassName='cooperation-application__full'
                label={t('业务场景与需求')}
                maxLength={2000}
                placeholder={t('请描述应用场景、当前架构和希望解决的问题。')}
                autosize={{ minRows: 4, maxRows: 10 }}
                rules={requiredRule(t('请简要说明您的业务场景与需求'))}
              />
              <Form.TextArea
                field='remark'
                fieldClassName='cooperation-application__full'
                label={optionalLabel(t('其他补充'))}
                maxLength={1000}
                placeholder={t('还有什么想告诉我们的')}
                autosize={{ minRows: 4, maxRows: 8 }}
              />
              <Form.Slot
                className='cooperation-application__full'
                label={optionalLabel(t('上传资料'))}
              >
                <label
                  className={`cooperation-application__file-drop${dragging ? ' is-dragging' : ''}`}
                  onDragEnter={(event) => {
                    event.preventDefault();
                    setDragging(true);
                  }}
                  onDragOver={(event) => {
                    event.preventDefault();
                    event.dataTransfer.dropEffect = 'copy';
                    setDragging(true);
                  }}
                  onDragLeave={(event) => {
                    if (!event.currentTarget.contains(event.relatedTarget)) {
                      setDragging(false);
                    }
                  }}
                  onDrop={(event) => {
                    event.preventDefault();
                    setDragging(false);
                    addFiles(event.dataTransfer.files);
                  }}
                >
                  <input
                    ref={fileInputRef}
                    className='cooperation-application__file-input'
                    type='file'
                    multiple
                    accept={FILE_ACCEPT}
                    aria-label={t('上传资料')}
                    aria-describedby='cooperation-file-tip'
                    onChange={(event) => {
                      addFiles(event.target.files);
                      event.target.value = '';
                    }}
                  />
                  <strong>{t('点击选择或拖入资料')}</strong>
                  <span>{t('需求文档、架构图、测试样本等')}</span>
                  <em id='cooperation-file-tip'>
                    {t(
                      'PDF / Office / 文本 / 压缩包 / 图片 · 最多 3 个 · 单个 10 MB 以内',
                    )}
                  </em>
                </label>
                {fileError && (
                  <p
                    className='cooperation-application__file-error'
                    role='alert'
                  >
                    {fileError}
                  </p>
                )}
                <ul
                  className='cooperation-application__file-list'
                  aria-live='polite'
                >
                  {files.map((file, index) => (
                    <li key={file.id}>
                      <b title={file.name}>{file.name}</b>
                      <span>
                        {(file.size / 1024 / 1024).toFixed(2)} MB
                        {file.uploading ? ` · ${t('上传中...')}` : ''}
                      </span>
                      <button
                        type='button'
                        aria-label={t('移除 {{name}}', { name: file.name })}
                        onClick={() => {
                          setFiles((prev) => prev.filter((_, i) => i !== index));
                          setFileError('');
                        }}
                      >
                        &times;
                      </button>
                    </li>
                  ))}
                </ul>
              </Form.Slot>
            </div>

            <Form.Checkbox
              field='consent'
              fieldClassName='cooperation-application__consent'
              noLabel
              trigger='change'
              validate={(value) => (value ? undefined : t('请确认同意后继续'))}
            >
              {t('同意我们通过上述联系方式与您联系，跟进本次合作需求。')}
            </Form.Checkbox>
            <div className='cooperation-application__actions'>
              <Button
                className='cooperation-application__submit'
                htmlType='submit'
                theme='solid'
                type='primary'
                icon={<IconSend />}
                loading={submitting}
                block
              >
                {t('提交合作意向')}
              </Button>
              <Button
                className='cooperation-application__reset'
                type='tertiary'
                htmlType='button'
                onClick={handleReset}
              >
                {t('重置')}
              </Button>
            </div>
          </Form>
        </div>

        <section
          ref={resultRef}
          className='cooperation-application__result'
          hidden={!submitted}
          tabIndex={-1}
          aria-live='polite'
          aria-labelledby='cooperation-result-title'
        >
          <span
            className='cooperation-application__result-icon'
            aria-hidden='true'
          >
            <IconTick size='large' />
          </span>
          <h2 id='cooperation-result-title'>{t('合作申请已提交')}</h2>
          <p>
            {t(
              '我们已收到您的合作意向，会通过您留下的联系方式尽快与您联系。',
            )}
          </p>
          <Button
            className='cooperation-application__edit'
            type='tertiary'
            onClick={handleReset}
          >
            {t('再填一份')}
          </Button>
        </section>
      </Card>
      <p className='cooperation-application__footer'>
        {systemName} · {t('商务合作')}
      </p>
    </main>
  );
};

export default CooperationApplication;
