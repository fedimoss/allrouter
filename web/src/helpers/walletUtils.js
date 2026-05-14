import { ethers } from 'ethers';
import { CRYPTO_API, ERC20_ABI } from './cryptoConfig';

export function isWalletInstalled(type) {
  if (type === 'metamask') {
    return !!(window.ethereum && window.ethereum.isMetaMask);
  }
  if (type === 'okx') {
    return !!window.okxwallet;
  }
  return false;
}

export function getWalletProvider(type) {
  if (type === 'metamask') {
    return window.ethereum;
  }
  if (type === 'okx') {
    return window.okxwallet;
  }
  return null;
}

export async function connectWallet(type) {
  const provider = getWalletProvider(type);
  if (!provider) throw new Error('Wallet not installed');
  const accounts = await provider.request({ method: 'eth_requestAccounts' });
  const chainId = await provider.request({ method: 'eth_chainId' });
  return { account: accounts[0], chainId };
}

export async function switchNetwork(provider, chainParams) {
  try {
    await provider.request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId: chainParams.chainId }],
    });
  } catch (err) {
    if (err.code === 4902) {
      await provider.request({
        method: 'wallet_addEthereumChain',
        params: [chainParams],
      });
    } else {
      throw err;
    }
  }
}

export async function getUSDTBalance(rpcUrl, userAddress, usdtAddress, decimals) {
  const provider = new ethers.JsonRpcProvider(rpcUrl);
  const contract = new ethers.Contract(usdtAddress, ERC20_ABI, provider);
  const raw = await contract.balanceOf(userAddress);
  return ethers.formatUnits(raw, decimals);
}

function parseTokenAmount(amountStr, decimals) {
  const parts = String(amountStr).split('.');
  const whole = parts[0] || '0';
  const frac = (parts[1] || '').padEnd(decimals, '0').slice(0, decimals);
  return BigInt(whole + frac);
}

export async function sendUSDT(walletType, orderData, chainParams) {
  const rawProvider = getWalletProvider(walletType);
  if (!rawProvider) throw new Error('Wallet not installed');

  const chainIdHex = '0x' + orderData.chain_id.toString(16);

  try {
    await rawProvider.request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId: chainIdHex }],
    });
  } catch (err) {
    if (err.code === 4902) {
      const params = chainParams || {
        chainId: chainIdHex,
        chainName: orderData.network + ' Mainnet',
        rpcUrls: ['https://bsc-dataseed1.binance.org'],
        nativeCurrency: { name: 'BNB', symbol: 'BNB', decimals: 18 },
        blockExplorerUrls: ['https://bscscan.com'],
      };
      await rawProvider.request({
        method: 'wallet_addEthereumChain',
        params: [params],
      });
    } else {
      throw err;
    }
  }

  const accounts = await rawProvider.request({ method: 'eth_requestAccounts' });
  const from = accounts[0];

  const transferMethodSig = '0xa9059cbb';
  const toAddress = orderData.to_address.toLowerCase().replace('0x', '').padStart(64, '0');
  const baseUnits = parseTokenAmount(orderData.pay_amount, orderData.decimals);
  const amountHex = baseUnits.toString(16).padStart(64, '0');
  const data = transferMethodSig + toAddress + amountHex;

  const txHash = await rawProvider.request({
    method: 'eth_sendTransaction',
    params: [{
      from: from,
      to: orderData.token_contract,
      data: data,
      value: '0x0',
      gas: '0x' + (120000).toString(16),
    }],
  });

  return txHash;
}

function getAuthHeaders() {
  const headers = { 'Content-Type': 'application/json' };
  try {
    const raw = localStorage.getItem('user');
    if (raw) {
      const user = JSON.parse(raw);
      if (user?.id) headers['New-Api-User'] = String(user.id);
    }
  } catch { /* ignore */ }
  return headers;
}

export async function createCryptoOrder(amount, networkName) {
  const resp = await fetch(CRYPTO_API + '/pay', {
    method: 'POST',
    headers: getAuthHeaders(),
    credentials: 'include',
    body: JSON.stringify({ amount, payment_method: 'crypto', network: networkName }),
  });
  const data = await resp.json();
  if (!data.success) throw new Error(data.message);
  return data.data;
}

export async function confirmCryptoOrder(tradeNo, txHash) {
  const resp = await fetch(CRYPTO_API + '/confirm', {
    method: 'POST',
    headers: getAuthHeaders(),
    credentials: 'include',
    body: JSON.stringify({ trade_no: tradeNo, tx_hash: txHash }),
  });
  const data = await resp.json();
  if (!data.success) throw new Error(data.message);
  return data.data;
}

export function shortenAddress(addr) {
  if (!addr || addr.length < 10) return addr;
  return `${addr.slice(0, 6)}...${addr.slice(-4)}`;
}
