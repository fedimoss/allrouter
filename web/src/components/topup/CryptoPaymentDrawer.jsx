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

import React, { useMemo, useState } from 'react';
import {
  Button,
  Modal,
  SideSheet,
  Tag,
  Toast,
  Typography,
} from '@douyinfe/semi-ui';
import {
  AlertTriangle,
  Check,
  ChevronDown,
  ShieldCheck,
  Wallet,
  X,
} from 'lucide-react';
import { SiBinance, SiOkx, SiPolygon } from 'react-icons/si';

const { Text } = Typography;

const NETWORKS = [
  {
    key: 'binance',
    name: 'Binance',
    subtitle: 'BNB Smart Chain',
    icon: SiBinance,
    color: '#F0B90B',
    token: 'USDT',
    balance: '0.82 USDT',
    fee: '~0.003 BNB',
    rpc: 'BSC_RPC',
    latency: '18ms',
    gas: '3.1 Gwei',
  },
  {
    key: 'polygon',
    name: 'Polygon',
    subtitle: 'Polygon PoS',
    icon: SiPolygon,
    color: '#8247E5',
    token: 'USDT',
    balance: '1.46 USDT',
    fee: '~0.02 POL',
    rpc: 'POLYGON_RPC',
    latency: '24ms',
    gas: '38 Gwei',
  },
];

const WALLETS = [
  {
    key: 'metamask',
    name: 'MetaMask',
    address: '0x742d...8F44e',
    color: '#F6851B',
  },
  {
    key: 'okx',
    name: 'OKX Wallet',
    address: '0xa912...78F629',
    color: '#111827',
  },
];

const formatAmount = (value) => {
  const parsed = Number(value || 0);
  return Number.isFinite(parsed) && parsed > 0 ? parsed.toFixed(2) : '10.50';
};

const WalletMark = ({ wallet }) => {
  if (wallet.key === 'okx') {
    return <SiOkx size={22} color='#111827' />;
  }
  return (
    <span
      className='inline-flex h-6 w-6 items-center justify-center rounded-full'
      style={{ background: '#FFF2E6' }}
    >
      <Wallet size={15} color={wallet.color} />
    </span>
  );
};

const CryptoPaymentDrawer = ({ visible, onClose, amount, t }) => {
  const [selectedNetwork, setSelectedNetwork] = useState(NETWORKS[0].key);
  const [selectedWallet, setSelectedWallet] = useState(WALLETS[0].key);
  const [walletModalVisible, setWalletModalVisible] = useState(false);

  const network = useMemo(
    () => NETWORKS.find((item) => item.key === selectedNetwork) || NETWORKS[0],
    [selectedNetwork],
  );
  const wallet = useMemo(
    () => WALLETS.find((item) => item.key === selectedWallet) || WALLETS[0],
    [selectedWallet],
  );
  const payableAmount = formatAmount(amount);
  const AssetIcon = network.icon;

  const handlePay = () => {
    Toast.info({ content: t('当前为加密货币支付静态演示') });
  };

  return (
    <SideSheet
      visible={visible}
      onCancel={onClose}
      placement='right'
      width='min(100vw, 1040px)'
      closeOnEsc
      maskClosable
      headerStyle={{ display: 'none' }}
      bodyStyle={{ padding: 0 }}
    >
      <div className='relative min-h-screen bg-[#F8FAFC] px-4 py-6 text-slate-700 dark:bg-slate-900 sm:px-6 lg:px-10'>
        <button
          type='button'
          onClick={onClose}
          className='absolute right-4 top-4 z-10 inline-flex h-8 w-8 items-center justify-center rounded-lg bg-white text-slate-500 shadow-sm transition hover:text-slate-800 dark:bg-slate-800 dark:text-slate-300 dark:hover:text-white'
          aria-label={t('关闭')}
        >
          <X size={18} />
        </button>
        <div className='mx-auto grid max-w-[980px] grid-cols-1 gap-8 lg:grid-cols-[1.55fr_1fr]'>
          <section className='rounded-[14px] bg-white px-5 py-7 shadow-2xl dark:bg-slate-800 sm:px-9 lg:px-10'>
            <div className='text-center'>
              <Text className='!text-[14px] !font-medium !text-[#94A3B8]'>
                {t('待支付金额')}
              </Text>
              <div className='mt-2 flex items-end justify-center gap-1'>
                <span className='text-[48px] font-black leading-none tracking-normal text-[#475569] dark:text-slate-100'>
                  {payableAmount}
                </span>
                <span className='pb-1 text-[22px] font-extrabold leading-none text-[#475569] dark:text-slate-100'>
                  USD
                </span>
              </div>
              <div className='mt-6 text-[14px] font-medium text-[#94A3B8]'>
                {t('收款方')}：
                <span className='font-bold text-[#10D9CD]'>AllRouter.AI</span>
              </div>
            </div>

            <div className='mt-14 flex flex-col items-start gap-4 sm:flex-row sm:items-center sm:justify-between'>
              <div className='flex items-center gap-3'>
                <WalletMark wallet={wallet} />
                <div>
                  <div className='text-[13px] font-extrabold text-[#475569] dark:text-slate-100'>
                    {wallet.name}
                  </div>
                  <div className='mt-0.5 text-[11px] font-medium text-[#94A3B8]'>
                    {wallet.address}
                  </div>
                </div>
              </div>
              <Button
                size='small'
                theme='light'
                style={{
                  color: '#10D9CD',
                  borderColor: '#BFF6F3',
                  background: '#F2FFFE',
                }}
                onClick={() => setWalletModalVisible(true)}
              >
                {t('切换钱包')}
              </Button>
            </div>

            <div className='mt-12'>
              <div className='mb-3 text-[13px] font-semibold text-[#64748B]'>
                {t('选择支付网络')}
              </div>
              <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
                {NETWORKS.map((item) => {
                  const Icon = item.icon;
                  const selected = item.key === selectedNetwork;
                  return (
                    <button
                      key={item.key}
                      type='button'
                      onClick={() => setSelectedNetwork(item.key)}
                      className={`flex h-[52px] items-center justify-center gap-2 rounded-[11px] border px-4 text-[15px] font-extrabold transition ${
                        selected
                          ? 'border-[#11D8CE] bg-[#EFFFFD] text-[#10D9CD]'
                          : 'border-transparent bg-[#F8FAFC] text-[#94A3B8]'
                      }`}
                    >
                      <span className='inline-flex h-5 w-5 items-center justify-center rounded-full bg-slate-900'>
                        <Icon size={13} color={item.color} />
                      </span>
                      {item.name}
                    </button>
                  );
                })}
              </div>
              <button
                type='button'
                className='mx-auto mt-7 flex items-center gap-1 text-[13px] font-semibold text-[#10D9CD]'
              >
                {t('查看全部支持网络')} <ChevronDown size={14} />
              </button>
            </div>

            <div className='mt-10'>
              <div className='mb-4 text-[13px] font-semibold text-[#64748B]'>
                {t('支付资产')}
              </div>
              <div className='flex flex-col gap-4 rounded-[11px] bg-[#F8FAFC] px-5 py-5 dark:bg-slate-900 sm:flex-row sm:items-center sm:justify-between sm:px-6'>
                <div className='flex items-center gap-4'>
                  <span
                    className='inline-flex h-9 w-9 items-center justify-center rounded-full'
                    style={{ background: network.color }}
                  >
                    <AssetIcon size={20} color='#fff' />
                  </span>
                  <div>
                    <div className='text-[18px] font-black text-[#475569] dark:text-slate-100'>
                      {network.token}
                    </div>
                    <div className='mt-0.5 text-[12px] font-semibold text-[#94A3B8]'>
                      {t('在 {{network}} 网络上可用', {
                        network: network.name,
                      })}
                    </div>
                  </div>
                </div>
                <div className='text-left sm:text-right'>
                  <div className='text-[18px] font-black text-[#475569] dark:text-slate-100'>
                    {payableAmount} {network.token}
                  </div>
                  <div className='mt-0.5 text-[12px] font-semibold text-[#94A3B8]'>
                    {t('余额')}：{network.balance}
                  </div>
                </div>
              </div>
            </div>

            <div className='mt-6 flex items-center gap-3 rounded-[11px] border border-[#FFB7B7] bg-[#FFF0F0] px-5 py-4'>
              <AlertTriangle size={21} color='#FF4D4F' />
              <div className='text-[13px] font-semibold text-[#475569]'>
                {t('余额不足。需额外添加')}{' '}
                <span className='font-black text-[#FF4D4F]'>
                  {(Number(payableAmount) - 1.46).toFixed(4)} {network.token}
                </span>{' '}
                {t('以完成购买。')}
              </div>
            </div>

            <div className='mt-8 space-y-3 text-[13px] font-semibold text-[#64748B]'>
              <div className='flex items-center justify-between'>
                <span>{t('网络费用（预计）')}</span>
                <span className='text-[#94A3B8]'>{network.fee}</span>
              </div>
              <div className='flex items-center justify-between'>
                <span>{t('服务费用')}</span>
                <span className='text-[#10D9CD]'>{t('免费')}</span>
              </div>
            </div>

            <Button
              block
              onClick={handlePay}
              className='!mt-7 !h-14 !rounded-[8px] !border-0 !bg-gradient-to-r !from-[#13DFD4] !to-[#9BFF1D] !text-[16px] !font-black !text-[#163333]'
            >
              {t('现在支付')}
            </Button>

            <p className='mx-auto mt-6 max-w-[430px] text-center text-[11px] font-semibold leading-5 text-[#7D8DA5]'>
              {t(
                '点击“现在支付”即表示您同意我们的《服务条款》。所有链上交易均不可逆转，请在确认支付前仔细核对收款地址与网络环境。',
              )}
            </p>
          </section>

          <aside className='pt-1 text-slate-700 dark:text-white'>
            <h3 className='text-[18px] font-black text-[#64748B]'>
              {t('交易确认提示')}
            </h3>
            <div className='mt-6 space-y-7'>
              <div className='flex gap-4'>
                <span className='mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full bg-[#10D9CD]'>
                  <Check size={13} color='#031313' />
                </span>
                <p className='text-[13px] font-bold leading-6 text-[#94A3B8]'>
                  {t(
                    '系统将自动检测 {{network}} 网络确认状态，通常需要 10-50 秒。',
                    { network: network.name },
                  )}
                </p>
              </div>
              <div className='flex gap-4'>
                <span className='mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full bg-[#10D9CD]'>
                  <Check size={13} color='#031313' />
                </span>
                <p className='text-[13px] font-bold leading-6 text-[#94A3B8]'>
                  {t('请确保您的钱包中有足够的链上资产用于支付 Gas 费用。')}
                </p>
              </div>
            </div>

            <div className='mt-10 rounded-[14px] bg-white px-6 py-6 text-[#475569] shadow-sm dark:bg-slate-900 dark:text-slate-200'>
              <div className='mb-5 text-[13px] font-black tracking-[0.08em] text-[#10D9CD]'>
                {t('实时节点状态')}
              </div>
              <div className='space-y-4 text-[11px] font-bold uppercase tracking-normal'>
                <div className='flex items-center justify-between'>
                  <span>{network.rpc}</span>
                  <span className='text-[#10D9CD]'>
                    ACTIVE ({network.latency})
                  </span>
                </div>
                <div className='flex items-center justify-between'>
                  <span>GAS_PRICE</span>
                  <span className='text-[#D5DEE9]'>{network.gas}</span>
                </div>
              </div>
            </div>

            <div className='mt-8 flex h-[30px] items-center gap-3 bg-white px-2 text-[#475569] shadow-sm dark:bg-slate-900 dark:text-slate-200'>
              <ShieldCheck size={19} color='#10D9CD' />
              <div>
                <div className='text-[10px] font-black tracking-[0.12em]'>
                  SECURE PROTOCOL
                </div>
                <div className='text-[8px] font-bold text-[#94A3B8]'>
                  256-bit Encrypted Transaction
                </div>
              </div>
            </div>

            <div className='mt-8 flex flex-wrap gap-2'>
              {WALLETS.map((item) => (
                <Tag
                  key={item.key}
                  color={item.key === selectedWallet ? 'cyan' : 'white'}
                  size='large'
                >
                  {item.name}
                </Tag>
              ))}
            </div>
          </aside>
        </div>

        <Modal
          title={t('选择钱包')}
          visible={walletModalVisible}
          onCancel={() => setWalletModalVisible(false)}
          footer={null}
          width={420}
          centered
        >
          <div className='grid grid-cols-1 gap-3'>
            {WALLETS.map((item) => {
              const selected = item.key === selectedWallet;
              return (
                <button
                  key={item.key}
                  type='button'
                  onClick={() => {
                    setSelectedWallet(item.key);
                    setWalletModalVisible(false);
                  }}
                  className={`flex items-center justify-between rounded-xl border px-4 py-4 text-left transition ${
                    selected
                      ? 'border-[#10D9CD] bg-[#EFFFFD] dark:bg-cyan-900/20'
                      : 'border-slate-200 bg-white hover:border-[#10D9CD] dark:border-slate-700 dark:bg-slate-900'
                  }`}
                >
                  <span className='flex items-center gap-3'>
                    <WalletMark wallet={item} />
                    <span>
                      <span className='block text-[14px] font-bold text-[#475569] dark:text-slate-100'>
                        {item.name}
                      </span>
                      <span className='mt-0.5 block text-[12px] font-medium text-[#94A3B8]'>
                        {item.address}
                      </span>
                    </span>
                  </span>
                  {selected ? <Check size={18} color='#10D9CD' /> : null}
                </button>
              );
            })}
          </div>
        </Modal>
      </div>
    </SideSheet>
  );
};

export default CryptoPaymentDrawer;
