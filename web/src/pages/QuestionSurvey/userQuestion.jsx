import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Form, Radio, Select, Toast } from '@douyinfe/semi-ui';
import { IconSend } from '@douyinfe/semi-icons';
import { getSystemName } from '../../helpers';
import { API } from '../../helpers';
import './userQuestion.css';

const systemName = getSystemName();

const OptionalLabel = ({ children }) => {
  const { t } = useTranslation();
  return (
    <span>
      {children}
      <span className='survey-label-optional'>{t('可选')}</span>
    </span>
  );
};

const UserQuestion = () => {
  const { t } = useTranslation();
  const [formKey, setFormKey] = useState(0);

  const industryOptions = [
    t('互联网 / 软件'),
    t('金融'),
    t('教育'),
    t('医疗'),
    t('制造业'),
    t('其他'),
  ];

  const issueOptions = [
    t('功能问题'),
    t('使用体验'),
    t('性能问题'),
    t('账单与付费'),
    t('其他建议'),
  ];

  const handleSubmit = async (values) => {
    try {
      const res = await API.post('/api/user/questionnaire', {
        survey_data: values,
        // 未登录时后端按此域名匹配服务商；已登录时以用户 provider_id 为准
        domain: window.location.hostname,
      });
      if (res.data.success) {
        Toast.success(t('感谢您的反馈'));
        setFormKey((key) => key + 1); // 提交成功后重置表单
      } else {
        Toast.error(res.data.message || t('提交失败，请稍后重试'));
      }
    } catch (err) {
      Toast.error(t('提交失败，请稍后重试'));
    }
  };

  const handleReset = () => {
    setFormKey((key) => key + 1);
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
            <div className='survey-section__title'>
              <span className='survey-badge survey-badge--required'>
                {t('必填')}
              </span>
              <strong>{t('请告诉我们您的基本信息')}</strong>
            </div>

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
                field='industry'
                label={t('行业')}
                placeholder={t('请选择所属行业')}
                rules={[{ required: true, message: t('请选择行业') }]}
              >
                {industryOptions.map((item, index) => (
                  <Select.Option key={item} value={String(index + 1)}>
                    {item}
                  </Select.Option>
                ))}
              </Form.Select>
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
          </section>

          <section className='survey-section survey-section--optional'>
            <div className='survey-section__title'>
              <span className='survey-badge survey-badge--optional'>
                {t('选填')}
              </span>
              <strong>{t('更多信息，有助于我们更快定位')}</strong>
            </div>

            <div className='survey-grid'>
              <Form.Input
                field='company'
                label={<OptionalLabel>{t('公司/团队')}</OptionalLabel>}
                placeholder={t('例如：某某科技有限公司')}
              />
              <Form.DatePicker
                field='occurredAt'
                label={<OptionalLabel>{t('发生时间')}</OptionalLabel>}
                placeholder='yyyy/mm/dd'
                format='yyyy/MM/dd'
              />
            </div>

            <Form.RadioGroup
              field='urgency'
              label={<OptionalLabel>{t('紧急程度')}</OptionalLabel>}
              initValue='normal'
              direction='horizontal'
            >
              <Radio value='normal'>{t('一般')}</Radio>
              <Radio value='urgent'>{t('紧急')}</Radio>
              <Radio value='critical'>{t('非常紧急')}</Radio>
            </Form.RadioGroup>

            <Form.TextArea
              field='suggestion'
              label={<OptionalLabel>{t('期望与建议')}</OptionalLabel>}
              placeholder={t('您希望我们如何改进？任何想法都欢迎')}
              autosize={{ minRows: 4, maxRows: 8 }}
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
