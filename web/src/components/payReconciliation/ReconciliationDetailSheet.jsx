import React, { useEffect, useMemo, useState } from 'react';
import { SideSheet, Table, Tag } from '@douyinfe/semi-ui';
import { IconClose } from '@douyinfe/semi-icons';
import { CircleAlert, CircleCheckBig } from 'lucide-react';
import { API, showError } from '../../helpers';
import { useTranslation } from 'react-i18next';
import wechatPayImg from '../../../public/wechat_pay.png';

const StatusChip = ({ text, danger = false }) => (
  <span className={`inline-flex items-center gap-2 font-semibold ${danger ? 'text-[#FF4D4F]' : 'text-[#09CC73]'}`}>
    <span className={`inline-flex h-5 w-5 items-center justify-center rounded-full ${danger ? 'bg-[#FDEBEC]' : 'bg-[#ECFBF3]'}`}>
      {danger ? <CircleAlert size={12} className='text-[#FF4D4F]' /> : <CircleCheckBig size={12} className='text-[#09CC73]' />}
    </span>
    <span className='text-[14px] leading-[20px]'>{text}</span>
  </span>
);

const PairValue = ({ label, value, status }) => (
  <div className='px-2 py-1'>
    <span className='mr-4 text-[13px] text-slate-400'>{label}：</span>
    {status === 'success' ? <StatusChip text={value || '-'} /> : <span className='text-[15px] font-semibold text-slate-600'>{value || '-'}</span>}
  </div>
);

const ReconciliationDetailSheet = ({ visible, onClose, row }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [detail, setDetail] = useState(null);
  const billId = row?.id;

  useEffect(() => {
    const loadDetail = async () => {
      if (!visible || !billId) return;
      setLoading(true);
      try {
        const res = await API.get(`/api/wechat_trade_bill/${billId}`);
        const { success, message, data } = res.data;
        if (success) {
          setDetail(data || null);
        } else {
          showError(message || '加载详情失败');
        }
      } catch {
        showError('加载详情失败');
      } finally {
        setLoading(false);
      }
    };
    loadDetail().then();
  }, [visible, billId]);

  const localRecord = detail?.local_record || {};
  const channelRecord = detail?.channel_record || {};
  const header = detail?.header || {};
  const markTxt = detail?.markTxt || detail?.remark || '-';
  const isMatched = detail?.reconcile_status === 'matched';
  const reconcileText = detail?.reconcile_status_text || (isMatched ? '一致' : '异常');

  const detailRows = useMemo(
    () => [
      {
        id: 'r1',
        leftLabel: t('系统单号'),
        leftValue: localRecord.local_id ?? '-',
        rightLabel: t('渠道单号'),
        rightValue: channelRecord.wechat_trade_no,
      },
      {
        id: 'r2',
        leftLabel: t('订单状态'),
        leftValue: t(localRecord.status_text),
        leftStatus: localRecord.status === 'success' ? 'success' : '',
        rightLabel: t('交易状态'),
        rightValue: t(channelRecord.trade_status_text),
        rightStatus: channelRecord.trade_status === 'SUCCESS' ? 'success' : '',
      },
      {
        id: 'r3',
        leftLabel: t('用户ID'),
        leftValue: localRecord.user_id ? `${localRecord.user_id}` : '-',
        rightLabel: t('付款用户'),
        rightValue: channelRecord.user_identifier,
      },
      {
        id: 'r4',
        leftLabel: t('请求金额'),
        leftValue: localRecord.requested_amount_text ? `¥${localRecord.requested_amount_text}` : '-',
        rightLabel: t('实际支付'),
        rightValue: channelRecord.actual_amount_text ? `¥${channelRecord.actual_amount_text}` : '-',
      },
      {
        id: 'r5',
        leftLabel: t('创建时间'),
        leftValue: localRecord.create_time_text,
        rightLabel: t('支付完成'),
        rightValue: channelRecord.trade_complete_time || channelRecord.trade_time,
      },
      {
        id: 'remark',
        isMerged: true,
        mergedLabel: t('备注'),
        mergedValue: t(markTxt),
      },
    ],
    [localRecord, channelRecord, markTxt, t],
  );

  const columns = useMemo(
    () => [
      {
        title: <span className='text-[16px] text-[#475569]'>{t('Allrouter 系统记录')}</span>,
        key: 'left',
        width: '50%',
        render: (_, record) => {
          if (record?.isMerged) {
            return {
              children: (
                <div className='px-2 py-1'>
                  <span className='mr-4 text-[13px] text-slate-400'>{record.mergedLabel}：</span>
                  <span className='text-[15px] font-semibold text-slate-600'>{record.mergedValue || '-'}</span>
                </div>
              ),
              props: { colSpan: 2 },
            };
          }
          return <PairValue label={record.leftLabel} value={record.leftValue} status={record.leftStatus} />;
        },
      },
      {
        title: <span className='text-[16px] text-[#475569]'>{t('支付渠道')}（{t(channelRecord.payment_method_text) || t('微信支付')}）</span>,
        key: 'right',
        width: '50%',
        render: (_, record) => {
          if (record?.isMerged) {
            return { children: null, props: { colSpan: 0 } };
          }
          return <PairValue label={record.rightLabel} value={record.rightValue} status={record.rightStatus} />;
        },
      },
    ],
    [channelRecord.payment_method_text, t],
  );

  return (
    <SideSheet visible={visible} onCancel={onClose} width={800} placement='right' closeOnEsc maskClosable bodyStyle={{ padding: 0 }} headerStyle={{ display: 'none' }}>
      <div className='p-5'>
        <div className='overflow-hidden rounded-2xl bg-white'>
          <div className='border-b border-slate-200 pb-3'>
            <div className='flex items-start justify-between gap-4'>
              <div className='flex items-center gap-2.5'>
                <img src={wechatPayImg} alt='微信支付' className='h-12 w-12 rounded-md' />
                <div>
                  <div className='flex items-center gap-2'>
                    <span className='text-[20px] leading-[22px] font-semibold text-slate-800'>{t(header.title) || t('微信支付')}</span>
                    <Tag size='small' color='blue'>
                      {t(header.tag) || '-'}
                    </Tag>
                  </div>
                  <div className='mt-1 text-[13px] leading-[18px] text-slate-500'>{t('Allrouter系统对账单明细')}</div>
                </div>
              </div>
              <button onClick={onClose} className='mt-0.5 inline-flex h-6 w-6 items-center justify-center rounded-md text-slate-500 hover:bg-slate-100'>
                <IconClose />
              </button>
            </div>
          </div>

          <div className='py-4'>
            <div className='mb-4 text-[16px] text-[#475569]'>{t('对账详情')}</div>
            <div className='overflow-hidden rounded-xl border border-slate-200'>
              <Table columns={columns} dataSource={detailRows} rowKey='id' pagination={false} bordered loading={loading} />
              <div className='border-t border-slate-200 px-5 py-4'>
                <span className='mr-4 text-[13px] text-slate-400'>{t('核对结果')}：</span>
                {isMatched ? <StatusChip text={t(reconcileText)} /> : <StatusChip text={t(reconcileText)} danger />}
              </div>
            </div>
          </div>
        </div>
      </div>
    </SideSheet>
  );
};

export default ReconciliationDetailSheet;
