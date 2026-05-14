import React, { useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { StatusContext } from '../../context/Status';
import {
  Button,
  Modal,
  SideSheet,
  Spin,
  Tag,
  Toast,
  Typography,
} from '@douyinfe/semi-ui';
import {
  AlertTriangle,
  Check,
  CheckCircle2,
  ChevronDown,
  Loader2,
  ShieldCheck,
  Wallet,
  X,
} from 'lucide-react';
import { SiBinance, SiEthereum, SiOkx } from 'react-icons/si';
import { WALLETS } from '../../helpers/cryptoConfig';
import { API } from '../../helpers';
import {
  confirmCryptoOrder,
  connectWallet,
  createCryptoOrder,
  getUSDTBalance,
  isWalletInstalled,
  sendUSDT,
  shortenAddress,
  switchNetwork,
} from '../../helpers/walletUtils';

const { Text } = Typography;

function getNetworkIcon(name) {
  const n = (name || '').toLowerCase();
  if (n.includes('bsc') || n.includes('binance')) return SiBinance;
  if (n.includes('polygon')) return SiEthereum;
  return SiEthereum;
}

function getNetworkColor(name) {
  const n = (name || '').toLowerCase();
  if (n.includes('bsc') || n.includes('binance')) return '#F0B90B';
  if (n.includes('polygon')) return '#8247E5';
  return '#627EEA';
}

function getExplorerBaseUrl(chainId) {
  const map = {
    1: 'https://etherscan.io',
    5: 'https://goerli.etherscan.io',
    56: 'https://bscscan.com',
    97: 'https://testnet.bscscan.com',
    137: 'https://polygonscan.com',
    80001: 'https://mumbai.polygonscan.com',
    11155111: 'https://sepolia.etherscan.io',
  };
  return map[chainId] || '';
}

function transformChain(chain) {
  const chainIdNum = Number(chain.chain_id);
  return {
    key: (chain.network || '').toLowerCase().replace(/\s+/g, '_'),
    name: chain.network,
    token: chain.token_symbol || 'USDT',
    usdtAddress: chain.token_contract,
    usdtDecimals: chain.token_decimals,
    chainParams: {
      chainId: '0x' + chainIdNum.toString(16),
      chainName: chain.network,
      nativeCurrency: { name: chain.network, symbol: 'ETH', decimals: 18 },
      rpcUrls: [chain.rpc_url],
      blockExplorerUrls: getExplorerBaseUrl(chainIdNum)
        ? [getExplorerBaseUrl(chainIdNum)]
        : [],
    },
  };
}

const formatAmount = (value) => {
  const parsed = Number(value || 0);
  return Number.isFinite(parsed) && parsed > 0 ? parsed.toFixed(2) : '0.00';
};

const WalletMark = ({ walletKey }) => {
  if (walletKey === 'okx') return <SiOkx size={22} color='#111827' />;
  return (
    <span
      className='inline-flex h-6 w-6 items-center justify-center rounded-full'
      style={{ background: '#FFF2E6' }}
    >
      <Wallet size={15} color='#F6851B' />
    </span>
  );
};

const CryptoPaymentDrawer = ({ visible, onClose, amount, t, onSuccess, createOrder, confirmOrder, currency = 'USD' }) => {
  const [selectedNetwork, setSelectedNetwork] = useState('');
  const [selectedWallet, setSelectedWallet] = useState(WALLETS[0].key);
  const [walletModalVisible, setWalletModalVisible] = useState(false);

  const [account, setAccount] = useState(null);
  const [chainId, setChainId] = useState(null);
  const [connecting, setConnecting] = useState(false);
  const [usdtBalance, setUsdtBalance] = useState(null);
  const [balanceLoading, setBalanceLoading] = useState(false);
  const [payState, setPayState] = useState('idle');
  const [orderData, setOrderData] = useState(null);
  const [txHash, setTxHash] = useState(null);
  const [errorMsg, setErrorMsg] = useState('');
  const [switching, setSwitching] = useState(false);

  const [chains, setChains] = useState([]);
  const [chainsLoading, setChainsLoading] = useState(false);
  const [statusState] = useContext(StatusContext);
  const [usdRate, setUsdRate] = useState(1);
  const [cnyRate, setCnyRate] = useState(0);

  const providerRef = useRef(null);

  const network = useMemo(
    () => chains.find((n) => n.key === selectedNetwork) || chains[0] || null,
    [selectedNetwork, chains],
  );
  const walletCfg = useMemo(
    () => WALLETS.find((w) => w.key === selectedWallet) || WALLETS[0],
    [selectedWallet],
  );
  const installed = useMemo(() => isWalletInstalled(selectedWallet), [selectedWallet]);
  const connected = !!account;
  const rawAmount = Number(amount || 0);
  const usdExchangeRate = parseFloat(statusState?.status?.usd_exchange_rate) || 7.25;
  // 顶部"待支付金额"，单位 USD
  const payableUsd = formatAmount(
    currency === 'CNY' ? rawAmount / usdExchangeRate : rawAmount,
  );
  // 底部"支付资产"，单位 USDT
  const payableUsdt = (() => {
    const val = currency === 'CNY' ? rawAmount * (cnyRate || usdRate) : rawAmount * usdRate;
    const parsed = Number(val || 0);
    return Number.isFinite(parsed) && parsed > 0 ? parsed.toFixed(3) : '0.000';
  })();
  const onCorrectChain = network ? chainId === network.chainParams.chainId : false;
  const balanceNum = usdtBalance !== null ? parseFloat(usdtBalance) : 0;
  const sufficient = balanceNum >= parseFloat(payableUsdt);
  const canPay = connected && onCorrectChain && sufficient && payState === 'idle';

  const fetchChains = useCallback(async () => {
    setChainsLoading(true);
    try {
      const [chainRes, rateRes] = await Promise.all([
        API.get('/api/option/get_crypto_chain_config'),
        API.get('/api/option/get_crypto_rate'),
      ]);
      const { success, data } = chainRes.data;
      if (success && Array.isArray(data)) {
        const transformed = data.map(transformChain);
        setChains(transformed);
        setSelectedNetwork((prev) => {
          if (transformed.length > 0 && !transformed.find((c) => c.key === prev)) {
            return transformed[0].key;
          }
          return prev;
        });
      }
      const rateData = rateRes.data;
      if (rateData?.success && rateData.data) {
        const u = Number(rateData.data.usd_to_token_rate || 0);
        const c = Number(rateData.data.cny_to_token_rate || 0);
        if (u > 0) setUsdRate(u);
        if (c > 0) setCnyRate(c);
      }
    } catch {
      // ignore
    } finally {
      setChainsLoading(false);
    }
  }, []);

  const fetchBalance = useCallback(
    async (addr, net) => {
      setBalanceLoading(true);
      try {
        const bal = await getUSDTBalance(
          net.chainParams.rpcUrls[0],
          addr,
          net.usdtAddress,
          net.usdtDecimals,
        );
        setUsdtBalance(bal);
      } catch {
        setUsdtBalance(null);
      } finally {
        setBalanceLoading(false);
      }
    },
    [],
  );

  const handleConnect = useCallback(async () => {
    setConnecting(true);
    setErrorMsg('');
    try {
      const result = await connectWallet(selectedWallet);
      setAccount(result.account);
      setChainId(result.chainId);
      providerRef.current = selectedWallet === 'okx' ? window.okxwallet : window.ethereum;
    } catch (err) {
      if (err.code === 4001) {
        Toast.warning(t('您拒绝了钱包连接请求'));
      } else {
        Toast.error(t('连接钱包失败'));
      }
    } finally {
      setConnecting(false);
    }
  }, [selectedWallet, t]);

  const handleSwitchNetwork = useCallback(async () => {
    if (!providerRef.current || !network) return;
    setSwitching(true);
    try {
      await switchNetwork(providerRef.current, network.chainParams);
    } catch (err) {
      if (err.code === 4001) {
        Toast.warning(t('您拒绝了网络切换请求'));
      } else {
        Toast.error(t('切换网络失败'));
      }
    } finally {
      setSwitching(false);
    }
  }, [network, t]);

  const doCreateOrder = createOrder || (async (networkName) => {
    return await createCryptoOrder(amount, networkName);
  });
  const doConfirmOrder = confirmOrder || confirmCryptoOrder;

  const handlePay = useCallback(async () => {
    if (!network) return;
    setErrorMsg('');
    let order = null;

    setPayState('creating');
    try {
      order = await doCreateOrder(network.name);
      setOrderData(order);
    } catch (err) {
      setErrorMsg(t('创建订单失败') + '：' + err.message);
      setPayState('failed');
      return;
    }

    setPayState('paying');
    let hash;
    try {
      hash = await sendUSDT(selectedWallet, order, network.chainParams);
      setTxHash(hash);
    } catch (err) {
      if (err.code === 4001 || err.code === 'ACTION_REJECTED') {
        setErrorMsg(t('您取消了支付'));
      } else {
        setErrorMsg(err.message || t('支付失败，请重试'));
      }
      setPayState('failed');
      return;
    }

    setPayState('confirming');
    const maxAttempts = 60;
    const interval = 5000;
    for (let i = 0; i < maxAttempts; i++) {
      try {
        await doConfirmOrder(order.trade_no, hash);
        setPayState('success');
        Toast.success(t('充值成功'));
        onSuccess?.();
        return;
      } catch {
        await new Promise((r) => setTimeout(r, interval));
      }
    }
    setErrorMsg(t('确认超时，请稍后在充值记录中查看'));
    setPayState('failed');
  }, [amount, selectedNetwork, selectedWallet, t, network, onSuccess, doCreateOrder, doConfirmOrder]);

  const handleClose = useCallback(() => {
    if (payState === 'paying' || payState === 'confirming') {
      Toast.warning(t('正在确认订单，请稍等'));
      return;
    }
    onClose?.();
  }, [payState, onClose, t]);

  const handleResetPay = useCallback(() => {
    setPayState('idle');
    setOrderData(null);
    setTxHash(null);
    setErrorMsg('');
  }, []);

  useEffect(() => {
    const provider = providerRef.current;
    if (!provider || !connected) return;

    const onAccountsChanged = (accounts) => {
      if (accounts.length === 0) {
        setAccount(null);
        setChainId(null);
        setUsdtBalance(null);
        setPayState('idle');
        setOrderData(null);
        setTxHash(null);
      } else {
        setAccount(accounts[0]);
      }
    };
    const onChainChanged = (newChainId) => {
      setChainId(newChainId);
      setUsdtBalance(null);
      setPayState('idle');
      setOrderData(null);
      setTxHash(null);
    };

    provider.on?.('accountsChanged', onAccountsChanged);
    provider.on?.('chainChanged', onChainChanged);
    return () => {
      provider.removeListener?.('accountsChanged', onAccountsChanged);
      provider.removeListener?.('chainChanged', onChainChanged);
    };
  }, [connected]);

  useEffect(() => {
    if (connected && onCorrectChain && network) {
      fetchBalance(account, network);
    } else {
      setUsdtBalance(null);
    }
  }, [connected, onCorrectChain, account, network, fetchBalance]);

  useEffect(() => {
    if (visible) {
      fetchChains();
    }
  }, [visible, fetchChains]);

  useEffect(() => {
    if (visible && connected && !onCorrectChain && network) {
      handleSwitchNetwork();
    }
  }, [visible, connected, onCorrectChain, handleSwitchNetwork, network]);

  useEffect(() => {
    if (!visible) {
      setPayState('idle');
      setOrderData(null);
      setTxHash(null);
      setErrorMsg('');
    }
  }, [visible]);

  const handleSelectWallet = useCallback(
    (key) => {
      setSelectedWallet(key);
      setWalletModalVisible(false);
      setAccount(null);
      setChainId(null);
      setUsdtBalance(null);
      setPayState('idle');
      setOrderData(null);
      setTxHash(null);
      providerRef.current = null;
    },
    [],
  );

  const getExplorerTxUrl = (hash) => {
    const chainIdNum = orderData?.chain_id || (network ? parseInt(network.chainParams.chainId, 16) : 0);
    const base = getExplorerBaseUrl(chainIdNum);
    return base ? `${base}/tx/${hash}` : '';
  };

  const payButtonText = () => {
    switch (payState) {
      case 'creating':
        return t('创建订单中...');
      case 'paying':
        return t('请在钱包中确认...');
      case 'confirming':
        return t('链上确认中...');
      default:
        if (chains.length === 0) return t('暂无可用网络');
        if (!installed) return t('请先安装钱包');
        if (!connected) return t('请先连接钱包');
        if (!onCorrectChain) return t('请先切换网络');
        if (!sufficient) return t('余额不足');
        return t('现在支付');
    }
  };

  return (
    <SideSheet
      visible={visible}
      onCancel={handleClose}
      placement='right'
      width='min(100vw, 1040px)'
      closeOnEsc={false}
      maskClosable={false}
      headerStyle={{ display: 'none' }}
      bodyStyle={{ padding: 0 }}
    >
      <div className='relative min-h-screen bg-[#F8FAFC] px-4 py-6 text-slate-700 dark:bg-slate-900 sm:px-6 lg:px-10'>
        <button
          type='button'
          onClick={handleClose}
          className='absolute right-4 top-4 z-10 inline-flex h-8 w-8 items-center justify-center rounded-lg bg-white text-slate-500 shadow-sm transition hover:text-slate-800 dark:bg-slate-800 dark:text-slate-300 dark:hover:text-white'
          aria-label={t('关闭')}
        >
          <X size={18} />
        </button>

        {payState === 'success' ? (
          <div className='mx-auto flex max-w-[480px] flex-col items-center py-20'>
            <span className='inline-flex h-20 w-20 items-center justify-center rounded-full bg-emerald-50'>
              <CheckCircle2 size={48} className='text-emerald-500' />
            </span>
            <h2 className='mt-6 text-2xl font-black text-slate-800 dark:text-white'>
              {t('充值成功')}
            </h2>
            {orderData && (
              <div className='mt-4 text-center text-sm text-slate-400'>
                <div>{t('订单号')}：{orderData.trade_no}</div>
                <div className='mt-1'>
                  {t('金额')}：{orderData.pay_amount} {orderData.token}
                </div>
              </div>
            )}
            {txHash && (
              <a
                href={getExplorerTxUrl(txHash)}
                target='_blank'
                rel='noopener noreferrer'
                className='mt-3 text-sm font-semibold text-[#10D9CD] hover:underline'
              >
                {t('查看交易')} →
              </a>
            )}
            <Button
              theme='solid'
              className='!mt-10 !h-12 !w-48 !rounded-lg !bg-[#10D9CD] !text-[#163333]'
              onClick={onClose}
            >
              {t('完成')}
            </Button>
          </div>
        ) : (
          <div className='mx-auto grid max-w-[980px] grid-cols-1 gap-8 lg:grid-cols-[1.55fr_1fr]'>
            <section className='rounded-[14px] bg-white px-5 py-7 shadow-2xl dark:bg-slate-800 sm:px-9 lg:px-10'>
              <div className='text-center'>
                <Text className='!text-[14px] !font-medium !text-[#94A3B8]'>
                  {t('待支付金额')}
                </Text>
                <div className='mt-2 flex items-end justify-center gap-1'>
                  <span className='text-[48px] font-black leading-none tracking-normal text-[#475569] dark:text-slate-100'>
                    {payableUsd}
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

              {/* Wallet section */}
              <div className='mt-14 flex flex-col items-start gap-4 sm:flex-row sm:items-center sm:justify-between'>
                <div className='flex items-center gap-3'>
                  <WalletMark walletKey={selectedWallet} />
                  <div>
                    <div className='text-[13px] font-extrabold text-[#475569] dark:text-slate-100'>
                      {walletCfg.name}
                    </div>
                    <div className='mt-0.5 text-[11px] font-medium text-[#94A3B8]'>
                      {connected ? shortenAddress(account) : t('未连接')}
                    </div>
                  </div>
                </div>
                <div className='flex gap-2'>
                  <Button
                    size='small'
                    theme='light'
                    onClick={() => setWalletModalVisible(true)}
                  >
                    {t('切换钱包')}
                  </Button>
                  {!installed ? (
                    <a
                      href={walletCfg.installUrl}
                      target='_blank'
                      rel='noopener noreferrer'
                    >
                      <Button
                        size='small'
                        theme='solid'
                        style={{ background: '#10D9CD', color: '#163333' }}
                      >
                        {t('安装')} {walletCfg.name}
                      </Button>
                    </a>
                  ) : !connected ? (
                    <Button
                      size='small'
                      loading={connecting}
                      onClick={handleConnect}
                      style={{
                        color: '#10D9CD',
                        borderColor: '#BFF6F3',
                        background: '#F2FFFE',
                      }}
                    >
                      {t('连接钱包')}
                    </Button>
                  ) : (
                    <Tag color='cyan' size='small'>
                      {t('已连接')}
                    </Tag>
                  )}
                </div>
              </div>

              {/* Network selection */}
              <div className='mt-12'>
                <div className='mb-3 text-[13px] font-semibold text-[#64748B]'>
                  {t('选择支付网络')}
                </div>
                {chainsLoading ? (
                  <div className='flex items-center justify-center py-6'>
                    <Spin size='small' />
                  </div>
                ) : chains.length === 0 ? (
                  <div className='py-6 text-center text-[13px] text-[#94A3B8]'>
                    {t('暂无可用网络，请联系管理员配置')}
                  </div>
                ) : (
                  <div className={`grid gap-3 sm:grid-cols-${Math.min(chains.length, 3)}`}>
                    {chains.map((item) => {
                      const Icon = getNetworkIcon(item.name);
                      const color = getNetworkColor(item.name);
                      const selected = item.key === selectedNetwork;
                      return (
                        <button
                          key={item.key}
                          type='button'
                          onClick={() => {
                            setSelectedNetwork(item.key);
                            setUsdtBalance(null);
                            setPayState('idle');
                            setOrderData(null);
                            setTxHash(null);
                          }}
                          disabled={switching || payState === 'creating' || payState === 'paying' || payState === 'confirming'}
                          className={`flex h-[52px] items-center justify-center gap-2 rounded-[11px] border px-4 text-[15px] font-extrabold transition ${
                            selected
                              ? 'border-[#11D8CE] bg-[#EFFFFD] text-[#10D9CD]'
                              : 'border-transparent bg-[#F8FAFC] text-[#94A3B8]'
                          }`}
                        >
                          <span className='inline-flex h-5 w-5 items-center justify-center rounded-full bg-slate-900'>
                            <Icon size={13} color={color} />
                          </span>
                          {item.name}
                          {connected && !switching && chainId === item.chainParams.chainId && (
                            <Check size={14} className='text-[#10D9CD]' />
                          )}
                        </button>
                      );
                    })}
                  </div>
                )}
                {connected && !onCorrectChain && !switching && payState === 'idle' && network && (
                  <button
                    type='button'
                    onClick={handleSwitchNetwork}
                    className='mx-auto mt-4 flex items-center gap-1 text-[13px] font-semibold text-[#10D9CD]'
                  >
                    {t('切换到 {{network}} 网络', { network: network.name })}
                    <ChevronDown size={14} />
                  </button>
                )}
                {switching && (
                  <div className='mt-4 flex items-center justify-center gap-2 text-[13px] text-[#94A3B8]'>
                    <Loader2 size={14} className='animate-spin' />
                    {t('正在切换网络...')}
                  </div>
                )}
              </div>

              {/* Asset & Balance */}
              {network && (
                <div className='mt-10'>
                  <div className='mb-4 text-[13px] font-semibold text-[#64748B]'>
                    {t('支付资产')}
                  </div>
                  <div className='flex flex-col gap-4 rounded-[11px] bg-[#F8FAFC] px-5 py-5 dark:bg-slate-900 sm:flex-row sm:items-center sm:justify-between sm:px-6'>
                    <div className='flex items-center gap-4'>
                      <span
                        className='inline-flex h-9 w-9 items-center justify-center rounded-full'
                        style={{ background: getNetworkColor(network.name) }}
                      >
                        {(() => {
                          const Icon = getNetworkIcon(network.name);
                          return <Icon size={20} color='#fff' />;
                        })()}
                      </span>
                      <div>
                        <div className='text-[18px] font-black text-[#475569] dark:text-slate-100'>
                          {network.token}
                        </div>
                        <div className='mt-0.5 text-[12px] font-semibold text-[#94A3B8]'>
                          {t('在 {{network}} 网络上可用', { network: network.name })}
                        </div>
                      </div>
                    </div>
                    <div className='text-left sm:text-right'>
                      <div className='text-[18px] font-black text-[#475569] dark:text-slate-100'>
                        {payableUsdt} USDT
                      </div>
                      <div className='mt-0.5 text-[12px] font-semibold text-[#94A3B8]'>
                        {t('余额')}：
                        {balanceLoading ? (
                          <Spin size='small' />
                        ) : connected && onCorrectChain && usdtBalance !== null ? (
                          `${parseFloat(usdtBalance).toFixed(4)} ${network.token}`
                        ) : connected && !onCorrectChain ? (
                          t('请先切换网络')
                        ) : (
                          '--'
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              )}

              {/* Insufficient balance warning */}
              {connected && onCorrectChain && usdtBalance !== null && !sufficient && (
                <div className='mt-6 flex items-center gap-3 rounded-[11px] border border-[#FFB7B7] bg-[#FFF0F0] px-5 py-4'>
                  <AlertTriangle size={21} color='#FF4D4F' />
                  <div className='text-[13px] font-semibold text-[#475569]'>
                    {t('余额不足。需额外添加')}{' '}
                    <span className='font-black text-[#FF4D4F]'>
                      {(parseFloat(payableUsdt) - balanceNum).toFixed(4)}{' '}
                      USDT
                    </span>{' '}
                    {t('以完成购买。')}
                  </div>
                </div>
              )}

              {/* Network fees */}
              <div className='mt-6 space-y-3 text-[13px] font-semibold text-[#64748B]'>
                <div className='flex items-center justify-between'>
                  <span>{t('服务费用')}</span>
                  <span className='text-[#10D9CD]'>{t('免费')}</span>
                </div>
              </div>

              {/* Pay button */}
              <Button
                block
                disabled={!canPay || chains.length === 0}
                loading={payState === 'creating' || payState === 'paying' || payState === 'confirming'}
                onClick={handlePay}
                className='!mt-7 !h-14 !rounded-[8px] !border-0 theme-btn-color theme-bg !text-[16px]'
              >
                {payButtonText()}
              </Button>

              {/* Error message */}
              {payState === 'failed' && errorMsg && (
                <div className='mt-4 flex items-center justify-between rounded-[11px] border border-[#FFB7B7] bg-[#FFF0F0] px-5 py-3'>
                  <span className='text-[13px] font-semibold text-[#FF4D4F]'>{errorMsg}</span>
                  <Button
                    size='small'
                    onClick={handleResetPay}
                    style={{ color: '#FF4D4F', borderColor: '#FFB7B7' }}
                  >
                    {t('重新支付')}
                  </Button>
                </div>
              )}

              <p className='mx-auto mt-6 max-w-[430px] text-center text-[11px] font-semibold leading-5 text-[#7D8DA5]'>
                {t(
                  '点击"现在支付"即表示您同意我们的《服务条款》。所有链上交易均不可逆转，请在确认支付前仔细核对收款地址与网络环境。',
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
                      { network: network?.name || '--' },
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
                  {network && (
                    <>
                      <div className='flex items-center justify-between'>
                        <span>{network.chainParams.rpcUrls[0]}</span>
                        <span className='text-[#10D9CD]'>ACTIVE</span>
                      </div>
                      <div className='flex items-center justify-between'>
                        <span>CHAIN_ID</span>
                        <span className='text-[#D5DEE9]'>{parseInt(network.chainParams.chainId, 16)}</span>
                      </div>
                    </>
                  )}
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
                    {!isWalletInstalled(item.key) && (
                      <span className='ml-1 text-[10px] text-[#FF4D4F]'>
                        ({t('未安装')})
                      </span>
                    )}
                  </Tag>
                ))}
              </div>
            </aside>
          </div>
        )}

        {/* Wallet selection modal */}
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
              const isInstalled = isWalletInstalled(item.key);
              return (
                <button
                  key={item.key}
                  type='button'
                  onClick={() => handleSelectWallet(item.key)}
                  className={`flex items-center justify-between rounded-xl border px-4 py-4 text-left transition ${
                    selected
                      ? 'border-[#10D9CD] bg-[#EFFFFD] dark:bg-cyan-900/20'
                      : 'border-slate-200 bg-white hover:border-[#10D9CD] dark:border-slate-700 dark:bg-slate-900'
                  }`}
                >
                  <span className='flex items-center gap-3'>
                    <WalletMark walletKey={item.key} />
                    <span>
                      <span className='block text-[14px] font-bold text-[#475569] dark:text-slate-100'>
                        {item.name}
                        {!isInstalled && (
                          <span className='ml-2 text-[12px] font-normal text-[#FF4D4F]'>
                            ({t('未安装')})
                          </span>
                        )}
                      </span>
                      <span className='mt-0.5 block text-[12px] font-medium text-[#94A3B8]'>
                        {isInstalled ? t('已安装') : t('点击前往安装')}
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
