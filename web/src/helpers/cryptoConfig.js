export const CRYPTO_API = '/api/user/crypto';

export const ERC20_ABI = [
  'function balanceOf(address owner) view returns (uint256)',
  'function transfer(address to, uint256 amount) returns (bool)',
  'function decimals() view returns (uint8)',
];

// ===== 测试网络配置（上线前使用） =====
export const NETWORKS = [
  {
    key: 'bsc',
    name: 'BSC Testnet',
    token: 'USDT',
    fee: '~0.003 tBNB',
    usdtAddress: '0xcE5DD515c545bEe30EF9a0E42a5da3211A79D983',
    usdtDecimals: 6,
    chainParams: {
      chainId: '0x61',
      chainName: 'BSC Testnet',
      nativeCurrency: { name: 'tBNB', symbol: 'tBNB', decimals: 18 },
      rpcUrls: ['https://data-seed-prebsc-1-s1.binance.org:8545'],
      blockExplorerUrls: ['https://testnet.bscscan.com'],
    },
  },
  {
    key: 'sepolia',
    name: 'Sepolia',
    token: 'USDT',
    fee: '~0.001 ETH',
    usdtAddress: '0xcE5DD515c545bEe30EF9a0E42a5da3211A79D983',
    usdtDecimals: 6,
    chainParams: {
      chainId: '0xaa36a7',
      chainName: 'Sepolia Testnet',
      nativeCurrency: { name: 'SepoliaETH', symbol: 'ETH', decimals: 18 },
      rpcUrls: ['https://ethereum-sepolia.publicnode.com'],
      blockExplorerUrls: ['https://sepolia.etherscan.io'],
    },
  },
];

// ===== 正式网络配置（上线时替换上面的 NETWORKS） =====
// export const NETWORKS = [
//   {
//     key: 'bsc',
//     name: 'BSC',
//     token: 'USDT',
//     fee: '~0.003 BNB',
//     usdtAddress: '0x55d398326f99059fF775485246999027B3197955',
//     usdtDecimals: 18,
//     chainParams: {
//       chainId: '0x38',
//       chainName: 'BNB Smart Chain',
//       nativeCurrency: { name: 'BNB', symbol: 'BNB', decimals: 18 },
//       rpcUrls: ['https://bsc-dataseed1.binance.org/'],
//       blockExplorerUrls: ['https://bscscan.com'],
//     },
//   },
//   {
//     key: 'polygon',
//     name: 'Polygon',
//     token: 'USDT',
//     fee: '~0.02 POL',
//     usdtAddress: '0xc2132D05D31c914a87C6611C10748AEb04B58e8F',
//     usdtDecimals: 6,
//     chainParams: {
//       chainId: '0x89',
//       chainName: 'Polygon PoS',
//       nativeCurrency: { name: 'MATIC', symbol: 'MATIC', decimals: 18 },
//       rpcUrls: ['https://polygon-rpc.com/'],
//       blockExplorerUrls: ['https://polygonscan.com'],
//     },
//   },
// ];

export const WALLETS = [
  {
    key: 'metamask',
    name: 'MetaMask',
    installUrl: 'https://metamask.io/download/',
    color: '#F6851B',
  },
  {
    key: 'okx',
    name: 'OKX Wallet',
    installUrl: 'https://www.okx.com/web3',
    color: '#111827',
  },
];
