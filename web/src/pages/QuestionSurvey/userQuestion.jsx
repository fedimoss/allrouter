import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Form,
  Radio,
  Select,
  Toast,
  Upload,
} from '@douyinfe/semi-ui';
import { IconPlus, IconSend } from '@douyinfe/semi-icons';
import { getSystemName } from '../../helpers';
import { API } from '../../helpers';
import './userQuestion.css';

const systemName = getSystemName();

// 问题截图最多张数，与后端 5MB/张限制配合
const MAX_SCREENSHOTS = 3;

const UserQuestion = () => {
  const { t } = useTranslation();
  const [formKey, setFormKey] = useState(0);
  // 问题截图：固定 3 个槽位块，空块点击选图自动上传，完成后回显
  const [screenshots, setScreenshots] = useState([]);
  const [uploadingSlots, setUploadingSlots] = useState([]);

  const issueOptions = [
    t('功能问题'),
    t('使用体验'),
    t('性能问题'),
    t('账单与付费'),
    t('其他建议'),
  ];

  const resetForm = () => {
    setFormKey((key) => key + 1);
    setScreenshots([]);
  };

  const handleSubmit = async (values) => {
    if (uploadingSlots.length > 0) {
      Toast.warning(t('截图正在上传，请稍候'));
      return;
    }
    try {
      const res = await API.post('/api/user/questionnaire', {
        // 截图 URL 数组随表单一并存入 survey_data JSON，无需后端加列
        survey_data: {
          ...values,
          screenshots: screenshots.filter(Boolean),
        },
        // 未登录时后端按此域名匹配服务商；已登录时以用户 provider_id 为准
        domain: window.location.hostname,
      });
      if (res.data.success) {
        Toast.success(t('感谢您的反馈'));
        resetForm(); // 提交成功后重置表单
      } else {
        Toast.error(res.data.message || t('提交失败，请稍后重试'));
      }
    } catch (err) {
      Toast.error(t('提交失败，请稍后重试'));
    }
  };

  const handleReset = () => {
    resetForm();
  };

  // 截图上传：成功后填入第一个空槽位并回显
  const handleScreenshotUpload = async (
    slotIndex,
    { file, fileInstance, onSuccess, onError },
  ) => {
    const uploadFile = fileInstance || file?.fileInstance;
    if (!uploadFile) {
      Toast.error(t('请选择图片'));
      onError?.({ status: 400 }, new Error(t('请选择图片')));
      return;
    }
    setUploadingSlots((prev) => [...prev, slotIndex]);
    try {
      const formData = new FormData();
      formData.append('image', uploadFile);
      formData.append('domain', window.location.hostname);
      const res = await API.post(
        '/api/user/questionnaire/upload',
        formData,
        { headers: { 'Content-Type': 'multipart/form-data' } },
      );
      const { success, message, data } = res.data || {};
      if (!success || !data?.url) {
        throw new Error(message || t('截图上传失败'));
      }
      setScreenshots((prev) => {
        const next = prev.slice();
        const emptyIdx = next.findIndex((v) => !v);
        if (emptyIdx >= 0) {
          next[emptyIdx] = data.url; // 从左到右填第一个空位
        } else if (next.length < MAX_SCREENSHOTS) {
          next.push(data.url);
        }
        return next;
      });
      onSuccess?.(data);
    } catch (error) {
      Toast.error(error?.message || t('截图上传失败'));
      onError?.({ status: 500 }, error);
    } finally {
      setUploadingSlots((prev) => prev.filter((i) => i !== slotIndex));
    }
  };

  const handleScreenshotRemove = (slotIndex) => {
    setScreenshots((prev) => prev.filter((_, i) => i !== slotIndex));
  };

  return (
    <main className='question-survey'>
      <header className='question-survey__header'>
        <div className='question-survey__brand'>{systemName}</div>
        <h1>{t('问题反馈')}</h1>
        <p>{t('告诉我们您遇到的问题或建议，帮助我们做得更好。')}</p>
      </header>

      <Card className='question-survey__card' bordered={false}>
        <Form
          key={formKey}
          layout='vertical'
          onSubmit={handleSubmit}
          className='question-survey__form'
        >
          <section className='survey-section'>
            <div className='survey-grid'>
              <Form.Input
                field='name'
                label={t('姓名')}
                placeholder={t('请输入您的姓名')}
                rules={[{ required: true, message: t('请输入姓名') }]}
              />
              <Form.Input
                field='contact'
                label={t('联系方式')}
                placeholder={t('手机号或邮箱，方便我们回复您')}
                rules={[{ required: true, message: t('请输入联系方式') }]}
              />
              <Form.Select
                field='issueType'
                label={t('问题类型')}
                placeholder={t('请选择问题类型')}
                rules={[{ required: true, message: t('请选择问题类型') }]}
              >
                {issueOptions.map((item, index) => (
                  <Select.Option key={item} value={String(index + 1)}>
                    {item}
                  </Select.Option>
                ))}
              </Form.Select>
              <Form.RadioGroup
                field='urgency'
                label={t('紧急程度')}
                rules={[{ required: true, message: t('请选择紧急程度') }]}
                direction='horizontal'
              >
                <Radio value='normal'>{t('一般')}</Radio>
                <Radio value='urgent'>{t('紧急')}</Radio>
                <Radio value='critical'>{t('非常紧急')}</Radio>
              </Form.RadioGroup>
            </div>

            <Form.TextArea
              field='description'
              label={t('问题描述')}
              placeholder={t(
                '请尽量详细描述您遇到的问题或建议，包括操作步骤、期望结果与实际结果等',
              )}
              autosize={{ minRows: 4, maxRows: 8 }}
              rules={[{ required: true, message: t('请填写问题描述') }]}
            />

            {/* 问题截图：非必填，3 个槽位块；空块点击选图自动上传，完成后回显，可删除 */}
            <Form.Slot label={t('问题截图')}>
              <div className='survey-screenshots'>
                {Array.from({ length: MAX_SCREENSHOTS }, (_, i) => {
                  const url = screenshots[i];
                  if (url) {
                    return (
                      <div key={i} className='survey-screenshots__item'>
                        <img src={url} alt={t('问题截图')} />
                        <button
                          type='button'
                          className='survey-screenshots__remove'
                          onClick={() => handleScreenshotRemove(i)}
                          aria-label={t('删除')}
                        >
                          &times;
                        </button>
                      </div>
                    );
                  }
                  const uploading = uploadingSlots.includes(i);
                  return (
                    <Upload
                      key={i}
                      accept='image/jpeg,image/png,image/gif,image/webp'
                      showUploadList={false}
                      uploadTrigger='auto'
                      customRequest={(opts) => handleScreenshotUpload(i, opts)}
                    >
                      <div className='survey-screenshots__item survey-screenshots__item--empty'>
                        {uploading ? (
                          <span className='survey-screenshots__hint'>
                            {t('上传中...')}
                          </span>
                        ) : (
                          <>
                            <IconPlus size='large' />
                            <span className='survey-screenshots__hint'>
                              {t('上传截图')}
                            </span>
                          </>
                        )}
                      </div>
                    </Upload>
                  );
                })}
              </div>
              <p className='survey-screenshots__tip'>
                {t('最多 3 张，单张不超过 5MB')}
              </p>
            </Form.Slot>

            <Form.TextArea
              field='other'
              label={t('其他')}
              placeholder={t('还有什么想告诉我们的')}
              autosize={{ minRows: 3, maxRows: 6 }}
            />

            <Form.Checkbox field='consent' noLabel value='consent'>
              {t(
                '同意我们通过上述联系方式与您联系，跟进本次反馈的处理结果',
              )}
            </Form.Checkbox>
          </section>

          <div className='question-survey__actions'>
            <Button
              htmlType='submit'
              theme='solid'
              type='primary'
              icon={<IconSend />}
              block
            >
              {t('提交反馈')}
            </Button>
            <Button type='tertiary' onClick={handleReset}>
              {t('重置')}
            </Button>
          </div>
        </Form>
      </Card>
    </main>
  );
};

export default UserQuestion;
