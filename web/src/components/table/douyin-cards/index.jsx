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

import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Typography,
  Upload,
  withField,
} from '@douyinfe/semi-ui';
import {
  IconImage,
  IconPlus,
  IconRefresh,
  IconSearch,
} from '@douyinfe/semi-icons';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import CardPro from '../../common/ui/CardPro';
import CardTable from '../../common/ui/CardTable';
import PhonePreview, { useViewportHeight, WechatPreview } from './PhonePreview';
import { API, showError, showSuccess } from '../../../helpers';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import {
  createCardProPagination,
  timestamp2string,
} from '../../../helpers/utils';
import { ITEMS_PER_PAGE } from '../../../constants';

const { Text } = Typography;

// 抖音私信卡片管理页：数据保存在外部服务（系统设置中的"基础URL"+"增删改查接口"），
// 本页面经 /api/douyin_card/* 代理接口转发，页面本身自包含（不与渠道页共享代码）。
// 外部查询接口的返回结构为 { success, data: { items: [...], total, page, pageSize } }，
// 卡片字段为 id/link/title/desc/cover（私信卡片）+ wxAvatar/wxName/wxDesc/wxQr/wxTip（推广链接落地页）
// + createdAt/updatedAt/createdBy/updatedBy（审计字段：添加时由本服务转发前补齐——
// 当前用户 ID + 秒级时间戳；编辑时随行数据原样带回，见 controller/douyin_card.go），
// 其余字段原样保留在行数据里，编辑时随表单值合并提交、不会被清空。

// 表格行数据的通用字段兜底：外部服务字段名不一致时按候选 key 依次取值
const pickField = (record, keys) => {
  for (const k of keys) {
    if (record[k] !== undefined && record[k] !== null && record[k] !== '') {
      return record[k];
    }
  }
  return '';
};

// 弹窗表单与列表回显共用的字段映射（外部服务字段名不一致时按候选 key 依次兜底）
const pickText = (record, keys) =>
  record ? String(pickField(record, keys) || '') : '';

// 外部服务的真实字段名排在首位，候选 key 仅作旧数据/异构数据的兜底
const TITLE_KEYS = ['title', 'card_name', 'name'];
const LOGO_KEYS = ['cover', 'cover_url', 'logo', 'logo_url', 'image'];
const CARD_DESC_KEYS = ['desc', 'content'];
const AVATAR_KEYS = ['wxAvatar', 'avatar', 'avatar_url'];
const NICKNAME_KEYS = ['wxName', 'nickname', 'wechat_name', 'wx_name'];
const WECHAT_DESC_KEYS = ['wxDesc', 'wechat_desc'];
const QRCODE_KEYS = ['wxQr', 'qrcode', 'qrcode_url', 'qr_code'];
const ACTION_TIP_KEYS = ['wxTip', 'action_tip', 'bottom_tip', 'footer_tip'];
const CREATED_AT_KEYS = ['createdAt', 'created_at'];

// 相对路径（本站 /static）转浏览器地址，仅用于页面内展示（列表缩略图/弹窗预览）。
// 提交给外部接口的图片地址由服务端转发前统一用系统设置
// 「通用设置 → 服务器地址」拼接（见 controller/douyin_card.go），
// 避免把管理端本地访问地址（如 http://127.0.0.1:5173）带给外部服务
const toAbsoluteUrl = (url) => {
  if (!url || /^https?:\/\//i.test(url)) return url;
  return `${window.location.origin}${url.startsWith('/') ? '' : '/'}${url}`;
};

// 列表图片缩略列共用渲染（卡片Logo/微信头像/微信二维码）：无图显示 -。
// 手机上卡片是「标签 + 值」的竖排行，56px 的图会把这一行撑得过高，
// 收成 40px 更接近其他页面移动端缩略图的观感（不影响桌面表格）
const renderThumb = (record, keys, alt, isMobile) => {
  const url = pickField(record, keys);
  if (!url) return <Text type='secondary'>-</Text>;
  const size = isMobile ? 40 : 56;
  return (
    <img
      src={toAbsoluteUrl(url)}
      alt={alt}
      style={{
        width: size,
        height: size,
        objectFit: 'cover',
        borderRadius: 6,
        display: 'block',
      }}
    />
  );
};

// 图片窄列的表头：禁止折行（「卡片Logo」「微信二维码」在窄列里会把后半截挤到第二行）。
// 列宽按中英文最长译文预留（如 WeChat QR Code ≈ 104px），nowrap 兜底字体渲染差异
const nowrapTitle = (text) => <span style={{ whiteSpace: 'nowrap' }}>{text}</span>;

// 图片上传框：点击选择图片上传，已有图时显示缩略图，上传中显示提示。
// kind 是上传用途，决定服务端的落盘目录（static/card/{kind}/{用户ID}/），
// 私信卡片 Logo 用 logo，推广链接的微信头像/二维码用 weixin
const ImageUploadBox = ({
  value,
  uploading,
  kind,
  size = 118,
  invalid = false,
  onUploaded,
  onUploadingChange,
}) => {
  const { t } = useTranslation();

  // 上传到本站 /api/douyin_card/upload，拿回相对 URL（提交时再拼成绝对 URL）
  const customRequest = async ({ file, fileInstance, onSuccess, onError }) => {
    const uploadFile = fileInstance || file?.fileInstance;
    if (!uploadFile) {
      showError(t('请选择图片'));
      onError?.({ status: 400 }, new Error(t('请选择图片')));
      return;
    }
    onUploadingChange?.(true);
    try {
      const formData = new FormData();
      formData.append('image', uploadFile);
      formData.append('type', kind);
      const res = await API.post('/api/douyin_card/upload', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      const { success, message, data } = res.data || {};
      if (!success || !data?.url) {
        throw new Error(message || t('上传失败'));
      }
      onUploaded?.(data.url);
      showSuccess(t('上传成功'));
      onSuccess?.(data);
    } catch (error) {
      showError(error?.message || t('上传失败'));
      onError?.({ status: 500 }, error);
    } finally {
      onUploadingChange?.(false);
    }
  };

  // 图片为必填项且点击已有图即可重新上传替换，故不提供单独的删除入口
  return (
    <Upload
      action='/'
      // 与个人页头像上传一致：文件选择框按 image/* 过滤，具体格式由服务端校验
      accept='image/*'
      showUploadList={false}
      uploadTrigger='auto'
      customRequest={customRequest}
    >
      <div
        style={{
          width: size,
          height: size,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 6,
          border: `1px dashed ${
            invalid
              ? 'var(--semi-color-danger)'
              : uploading
                ? 'var(--semi-color-primary)'
                : 'var(--semi-color-border)'
          }`,
          borderRadius: 8,
          cursor: 'pointer',
          overflow: 'hidden',
          color: 'var(--semi-color-text-2)',
          transition: 'border-color 0.18s ease',
        }}
      >
        {uploading ? (
          <span style={{ fontSize: 12 }}>{t('上传中...')}</span>
        ) : value ? (
          <img
            src={toAbsoluteUrl(value)}
            alt=''
            style={{ width: '100%', height: '100%', objectFit: 'cover' }}
          />
        ) : (
          <>
            <IconImage size={22} />
            <span style={{ fontSize: 12 }}>{t('上传图片')}</span>
          </>
        )}
      </div>
    </Upload>
  );
};

// 图片上传框的「表单字段」版本：withField 会注入 value / onChange / validateStatus，
// 并像 Form.Input 一样渲染标签与错误行，这样上传框也能用 rules 做必填校验。
// 上传框有内部状态（Semi Upload），关掉字段级 memo 避免挡住它自身的重渲染。
const ImageUploadField = withField(
  ({
    value,
    onChange,
    validateStatus,
    uploading,
    kind,
    size,
    onUploadingChange,
  }) => (
    <ImageUploadBox
      value={value}
      size={size}
      kind={kind}
      uploading={uploading}
      // 校验不通过时把上传框描红，与 Semi 输入框的错误态保持一致
      invalid={validateStatus === 'error'}
      onUploadingChange={onUploadingChange}
      onUploaded={onChange}
    />
  ),
  { shouldMemo: false },
);

const DouyinCardsTable = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const viewportHeight = useViewportHeight();

  // 列表状态
  const [cards, setCards] = useState([]);
  const [loading, setLoading] = useState(true);
  const [searching, setSearching] = useState(false);
  const [activePage, setActivePage] = useState(1);
  // 与渠道页一致：固定每页条数，分页只提供页码切换
  const pageSize = ITEMS_PER_PAGE;
  const [totalCount, setTotalCount] = useState(0);
  const [keyword, setKeyword] = useState('');

  // 编辑弹窗状态
  const [showEdit, setShowEdit] = useState(false);
  const [editingCard, setEditingCard] = useState(null);
  const [saving, setSaving] = useState(false);
  // 弹窗顶部胶囊切换：私信卡片（卡片外观）/ 推广链接（微信加粉落地页）
  const [activeTab, setActiveTab] = useState('card');
  // 表单当前值：右侧手机预览随左侧输入实时变化。
  // 三张图片（cover/avatar/qrcode）也是表单字段，值由表单托管，这里只是镜像给预览用
  const [draft, setDraft] = useState({
    title: '',
    desc: '',
    cover: '',
    avatar: '',
    nickname: '',
    description: '',
    qrcode: '',
    actionTip: '',
  });
  const formApiRef = useRef(null);
  // 正在上传的字段名（'' 表示空闲），用于显示对应上传框的进行中状态与禁用提交
  const [uploadingField, setUploadingField] = useState('');

  const isEdit = !!editingCard?.id;

  // 弹窗打开时初始化预览数据：优先取外部记录中的同名字段。
  // 未上传 Logo 时 cover 为空，预览里显示占位方块（.card-douyin-img-empty）。
  // 表单字段本身由 Form 的 initValues 回填（onValueChange 在挂载时不会触发，所以预览要单独初始化一次）
  useEffect(() => {
    if (showEdit) {
      setDraft({
        title: pickText(editingCard, TITLE_KEYS),
        desc: pickText(editingCard, CARD_DESC_KEYS),
        cover: pickText(editingCard, LOGO_KEYS),
        avatar: pickText(editingCard, AVATAR_KEYS),
        nickname: pickText(editingCard, NICKNAME_KEYS),
        description: pickText(editingCard, WECHAT_DESC_KEYS),
        qrcode: pickText(editingCard, QRCODE_KEYS),
        actionTip: pickText(editingCard, ACTION_TIP_KEYS),
      });
    }
  }, [showEdit, editingCard]);

  // ---------- 数据加载 ----------

  // 从外部接口响应中解析列表与总数，兼容数组与分页对象两种结构
  const parseListResponse = (data) => {
    if (Array.isArray(data)) {
      return { list: data, total: data.length };
    }
    if (data && typeof data === 'object') {
      const list = Array.isArray(data.list)
        ? data.list
        : Array.isArray(data.items)
          ? data.items
          : Array.isArray(data.records)
            ? data.records
            : [];
      const total =
        typeof data.total === 'number'
          ? data.total
          : typeof data.count === 'number'
            ? data.count
            : list.length;
      return { list, total };
    }
    return { list: [], total: 0 };
  };

  // kw 允许显式指定关键词：清除输入框时 setKeyword('') 尚未生效，
  // 直接调 loadCards 会用到闭包里的旧关键词，需传空串覆盖
  const loadCards = useCallback(
    async (page = activePage, size = pageSize, search = false, kw = keyword) => {
      if (search) setSearching(true);
      else setLoading(true);
      try {
        const params = { page, page_size: size };
        if (kw.trim()) params.keyword = kw.trim();
        const res = await API.get('/api/douyin_card/', { params });
        const { success, message, data } = res.data || {};
        if (success) {
          const { list, total } = parseListResponse(data);
          setCards(list);
          setTotalCount(total);
          // 页码越界时回到最后一页
          const totalPages = Math.max(1, Math.ceil(total / size));
          if (page > totalPages) {
            setActivePage(totalPages);
          }
        } else {
          showError(message || t('加载失败'));
        }
      } catch (e) {
        // axios 错误已由全局拦截器提示
      } finally {
        if (search) setSearching(false);
        else setLoading(false);
      }
    },
    [keyword, activePage, pageSize, t],
  );

  useEffect(() => {
    loadCards(activePage, pageSize);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activePage, pageSize]);

  const handlePageChange = (page) => setActivePage(page);

  const handleSearch = () => {
    setActivePage(1);
    loadCards(1, pageSize, true);
  };

  // 点击输入框的 x 清除按钮：清空关键词的同时按空关键词自动查一次（回到全量列表）
  const handleClear = () => {
    setKeyword('');
    setActivePage(1);
    loadCards(1, pageSize, true, '');
  };

  const refresh = () => {
    loadCards(activePage, pageSize, true);
  };

  // ---------- 增 / 改 / 删 ----------

  const openCreate = () => {
    setEditingCard(null);
    setShowEdit(true);
  };

  const openEdit = (record) => {
    setEditingCard(record);
    setShowEdit(true);
  };

  const closeEdit = () => {
    setShowEdit(false);
    setEditingCard(null);
    setActiveTab('card');
  };

  // 编辑表单初始值：新建给默认值，编辑把行数据原样回填。
  // 三张图片也是表单字段（由 ImageUploadField 托管），一并回填。
  // 卡片描述、跳转链接输入框已移除，desc/link 不再是表单字段
  const getFormInitValues = () => ({
    title: pickText(editingCard, TITLE_KEYS),
    cover: pickText(editingCard, LOGO_KEYS),
    avatar: pickText(editingCard, AVATAR_KEYS),
    nickname: pickText(editingCard, NICKNAME_KEYS),
    description: pickText(editingCard, WECHAT_DESC_KEYS),
    qrcode: pickText(editingCard, QRCODE_KEYS),
    action_tip: pickText(editingCard, ACTION_TIP_KEYS),
  });

  // 上传进行中的字段名包装：让各上传框与提交按钮各自显示对应的加载状态。
  // kind 同时作为上传用途传给服务端：私信卡片 Logo 归 logo，微信头像/二维码归 weixin
  const imageBoxProps = (field, size, kind) => ({
    size,
    kind,
    uploading: uploadingField === field,
    onUploadingChange: (uploading) => setUploadingField(uploading ? field : ''),
  });

  // 提交新建/编辑：表单内部字段名与外部服务不同（如 avatar ↔ wxAvatar），
  // 这里统一映射成外部服务的真实字段名再提交。
  // 编辑时把原始行数据与表单值合并后整体提交，
  // 避免外部服务存在页面未展示的字段在更新时被清空；
  // 选填字段被清空时显式置 null（外部记录里空值即 null），否则合并会把旧值带回去
  const submitCard = async (values) => {
    setSaving(true);
    try {
      const formValues = {
        title: values.title?.trim(),
        cover: values.cover,
      };
      // 表单字段名 → 外部字段名。图片保持上传接口返回的原样（相对路径 /static/card/...），
      // 由服务端转发前统一用系统设置「通用设置 → 服务器地址」拼接为绝对 URL——
      // 不能在浏览器端用 window.location.origin 拼，本地/开发访问时会带上
      // 127.0.0.1 这类外部服务访问不到的地址。
      // 卡片描述、跳转链接输入框已移除，两者都不在此处提交——新建时字段缺省，
      // 编辑时原始行数据里的旧值随下方展开合并带回；link 会被服务端在转发前
      // 统一替换为系统设置「抖音私信卡片 → 微信小程序页面地址」的值
      const optional = {
        wxAvatar: values.avatar,
        wxName: values.nickname,
        wxDesc: values.description,
        wxQr: values.qrcode,
        wxTip: values.action_tip,
      };
      Object.entries(optional).forEach(([key, value]) => {
        const v = typeof value === 'string' ? value.trim() : value;
        if (v) {
          formValues[key] = v;
        } else if (isEdit) {
          formValues[key] = null;
        }
        // 新建时留空的选填字段不提交，避免给外部服务带上无意义的空值
      });
      const payload = editingCard
        ? // 旧版本页面曾以 avatar/nickname/description/qrcode/action_tip 提交，
          // 这些并非外部服务的字段名，编辑时剔除掉，避免继续残留在记录里
          {
            ...editingCard,
            avatar: undefined,
            nickname: undefined,
            description: undefined,
            qrcode: undefined,
            action_tip: undefined,
            ...formValues,
          }
        : { ...formValues };
      const res = isEdit
        ? await API.put('/api/douyin_card/', payload)
        : await API.post('/api/douyin_card/', payload);
      const { success, message } = res.data || {};
      if (success) {
        showSuccess(isEdit ? t('修改成功') : t('创建成功'));
        closeEdit();
        refresh();
      } else {
        showError(message || t('操作失败'));
      }
    } catch (e) {
      // 全局拦截器已提示
    } finally {
      setSaving(false);
    }
  };

  const deleteCard = async (record) => {
    const id = pickField(record, ['id', 'card_id']);
    if (id === '' || id === undefined) {
      showError(t('该记录缺少主键，无法删除'));
      return;
    }
    try {
      const res = await API.delete(`/api/douyin_card/${id}`);
      const { success, message } = res.data || {};
      if (success) {
        showSuccess(t('删除成功'));
        // 当前页删空后回退一页，避免停留在空页
        if (cards.length === 1 && activePage > 1) {
          setActivePage(activePage - 1);
        } else {
          refresh();
        }
      } else {
        showError(message || t('操作失败'));
      }
    } catch (e) {
      // 全局拦截器已提示
    }
  };

  // ---------- 表格列 ----------

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
      render: (v, record) => (
        <Text type='secondary'>{pickField(record, ['id', 'card_id'])}</Text>
      ),
    },
    {
      title: nowrapTitle(t('卡片Logo')),
      dataIndex: 'cover',
      key: 'cover',
      width: 100,
      render: (v, record) =>
        renderThumb(record, LOGO_KEYS, t('卡片Logo'), isMobile),
    },
    {
      title: t('卡片标题'),
      dataIndex: 'title',
      key: 'title',
      render: (v, record) => (
        <span className='font-medium'>
          {pickField(record, TITLE_KEYS) || '-'}
        </span>
      ),
    },
    {
      // 推广链接（微信加粉落地页）四列：头像/二维码为缩略图，昵称/描述为文本
      title: nowrapTitle(t('微信头像')),
      dataIndex: 'wxAvatar',
      key: 'wxAvatar',
      width: 126,
      render: (v, record) =>
        renderThumb(record, AVATAR_KEYS, t('微信头像'), isMobile),
    },
    {
      title: t('微信昵称'),
      dataIndex: 'wxName',
      key: 'wxName',
      render: (v, record) => (
        <Text type='secondary'>{pickField(record, NICKNAME_KEYS) || '-'}</Text>
      ),
    },
    {
      title: t('微信描述'),
      dataIndex: 'wxDesc',
      key: 'wxDesc',
      render: (v, record) => (
        <Text type='secondary'>
          {pickField(record, WECHAT_DESC_KEYS) || '-'}
        </Text>
      ),
    },
    {
      title: nowrapTitle(t('微信二维码')),
      dataIndex: 'wxQr',
      key: 'wxQr',
      width: 140,
      render: (v, record) =>
        renderThumb(record, QRCODE_KEYS, t('微信二维码'), isMobile),
    },
    {
      // 外部服务返回秒级 Unix 时间戳（createdAt），为空说明旧数据未记录
      title: t('创建时间'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 170,
      render: (v, record) => {
        const createdAt = pickField(record, CREATED_AT_KEYS);
        if (!createdAt) return <Text type='secondary'>-</Text>;
        return (
          <Text type='secondary'>{timestamp2string(Number(createdAt))}</Text>
        );
      },
    },
    {
      // 手机端不显示「操作」标签：CardTable 对无标题列按「值右对齐」渲染，
      // 与渠道/令牌等页的移动端一致（桌面保留表头，方便对照）
      title: isMobile ? '' : t('操作'),
      key: 'operate',
      width: 140,
      render: (v, record) => (
        <div className='flex gap-1'>
          <Button
            size='small'
            theme='borderless'
            onClick={() => openEdit(record)}
          >
            {t('编辑')}
          </Button>
          <Popconfirm
            title={t('确定要删除该卡片吗？')}
            content={t('此操作不可逆')}
            onConfirm={() => deleteCard(record)}
            okText={t('确定')}
            cancelText={t('取消')}
          >
            <Button size='small' theme='borderless' type='danger'>
              {t('删除')}
            </Button>
          </Popconfirm>
        </div>
      ),
    },
  ];

  // ---------- 渲染 ----------

  // 手机预览的目标高度：桌面端用来源页面的 550；窄屏时预览会单独占一行，
  // 若仍用 550 会把表单挤成一小条，所以按视口高度的 45% 收一档
  const previewHeight = isMobile ? Math.round(viewportHeight * 0.45) : 550;

  // 弹窗标题位：胶囊形页签切换器（一个大胶囊内嵌「私信卡片」「推广链接」两个小胶囊）。
  // ml-6 抵消右侧关闭按钮的占位，使胶囊在弹窗内水平居中（标题内容区比弹窗窄一个关闭按钮宽度）
  const tabSwitcher = (
    <div className='flex w-full justify-center'>
      <div className='ml-6 inline-flex items-center gap-1 rounded-full bg-[var(--semi-color-fill-0)] p-1'>
        {[
          { key: 'card', label: t('私信卡片') },
          { key: 'link', label: t('推广链接') },
        ].map(({ key, label }) => (
          <button
            key={key}
            type='button'
            onClick={() => setActiveTab(key)}
            className={`cursor-pointer rounded-full border-none px-5 py-1 text-sm leading-5 transition-colors ${
              activeTab === key
                ? 'bg-[var(--semi-color-bg-2)] font-semibold text-[var(--semi-color-primary)] shadow-[0_1px_3px_rgba(0,0,0,0.1)]'
                : 'bg-transparent text-[var(--semi-color-text-2)] hover:text-[var(--semi-color-text-0)]'
            }`}
          >
            {label}
          </button>
        ))}
      </div>
    </div>
  );

  return (
    <>
      <CardPro
        className='douyin-cards-card'
        type='type1'
        searchArea={
          // 与渠道页筛选区同款布局：左侧操作按钮、右侧搜索框 + 查询按钮，同一行
          <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
            <div className='flex gap-2 w-full md:w-auto order-2 md:order-1'>
              <Button
                size='small'
                theme='light'
                type='primary'
                className='w-full md:w-auto'
                icon={<IconPlus />}
                onClick={openCreate}
              >
                {t('添加卡片')}
              </Button>
              <Button
                size='small'
                type='tertiary'
                className='w-full md:w-auto'
                icon={<IconRefresh />}
                onClick={refresh}
                loading={searching}
              >
                {t('刷新')}
              </Button>
            </div>
            <div className='flex flex-col md:flex-row items-stretch md:items-center gap-2 w-full md:w-auto order-1 md:order-2'>
              <div className='relative w-full md:w-64'>
                <Input
                  size='small'
                  prefix={<IconSearch />}
                  placeholder={t('卡片ID/标题')}
                  value={keyword}
                  onChange={(v) => setKeyword(v)}
                  onEnterPress={handleSearch}
                  onClear={handleClear}
                  showClear
                />
              </div>
              <Button
                size='small'
                type='tertiary'
                className='w-full md:w-auto'
                onClick={handleSearch}
                loading={searching}
              >
                {t('查询')}
              </Button>
            </div>
          </div>
        }
        paginationArea={createCardProPagination({
          currentPage: activePage,
          pageSize,
          total: totalCount,
          onPageChange: handlePageChange,
          isMobile,
          t,
        })}
        t={t}
      >
        <CardTable
          columns={columns}
          dataSource={cards}
          loading={loading || searching}
          rowKey={(record) =>
            pickField(record, ['id', 'card_id']) ?? cards.indexOf(record)
          }
          // scroll 不传 x: 'max-content'，让表格撑满卡片宽度（与渠道页紧凑模式一致），
          // 否则列少/无数据时表格只占内容宽度，右侧留白
          scroll={undefined}
          empty={
            // 手机端卡片是定高的（.table-scroll-card），空态若只贴顶会留下大片空白，
            // 用 douyin-cards-empty 在窄屏下撑满卡片并垂直居中（见 index.css）
            <div className='douyin-cards-empty flex justify-center p-4'>
              <Empty
                image={
                  <IllustrationNoResult style={{ width: 150, height: 150 }} />
                }
                darkModeImage={
                  <IllustrationNoResultDark
                    style={{ width: 150, height: 150 }}
                  />
                }
                description={t('暂无卡片数据')}
              />
            </div>
          }
          pagination={false}
        />
      </CardPro>

      {/* 新建/编辑弹窗：标题位是胶囊形页签切换器（私信卡片 / 推广链接），
          左侧表单改动时右侧手机预览实时联动 */}
      <Modal
        title={tabSwitcher}
        visible={showEdit}
        onCancel={closeEdit}
        closeOnEsc
        // 垂直居中：去掉 Semi 默认的 80px 上下外边距，机身才能取到更高的可用高度
        centered
        width={isMobile ? '94%' : 860}
        // 「取消 / 提交」由 Modal 底部统一渲染：两个页签共用同一组按钮，
        // 切换页签时位置不跳动，也不会随表单区滚动而移出视口
        footer={
          <div className='flex justify-end gap-2'>
            <Button type='tertiary' onClick={closeEdit}>
              {t('取消')}
            </Button>
            <Button
              theme='solid'
              type='primary'
              loading={saving || !!uploadingField}
              // 通过表单实例提交，等价于原生 submit：一样会先跑校验再进 onSubmit
              onClick={() => formApiRef.current?.submitForm()}
            >
              {t('提交')}
            </Button>
          </div>
        }
        // 表头与底部按钮合计约 164px，这里给表单区留出剩余高度；
        // 视口过矮时（手机横屏/小窗）表单区内部滚动，底部按钮始终可见可点
        bodyStyle={{
          maxHeight: 'calc(100vh - 200px)',
          overflowY: 'auto',
        }}
      >
        {showEdit && (
          <Form
            key={editingCard ? `edit-${editingCard.id}` : 'create'}
            initValues={getFormInitValues()}
            getFormApi={(api) => (formApiRef.current = api)}
            onSubmit={submitCard}
            // 校验失败时切到出错的页签，否则错误信息在隐藏的那个面板里看不见。
            // 两个页签都有必填项（私信卡片：标题/Logo；推广链接：头像/昵称/描述/二维码），
            // 按页签顺序优先展示「私信卡片」页的报错，修完再暴露推广链接页的
            onSubmitFail={(errors) => {
              const fields = Object.keys(errors || {});
              const cardFields = ['title', 'cover'];
              setActiveTab(
                fields.some((f) => cardFields.includes(f)) ? 'card' : 'link',
              );
            }}
            onValueChange={(values) =>
              setDraft({
                title: values.title || '',
                // 卡片描述输入框已移除：预览沿用外部记录里的原有描述（编辑提交时原样保留）
                desc: pickText(editingCard, CARD_DESC_KEYS),
                cover: values.cover || '',
                avatar: values.avatar || '',
                nickname: values.nickname || '',
                description: values.description || '',
                qrcode: values.qrcode || '',
                actionTip: values.action_tip || '',
              })
            }
          >
            {/* 左右两栏：左侧表单、右侧手机预览。
                两个页签的表单面板都常驻挂载、只用 display 控制显隐，切换胶囊不丢表单值 */}
            <div className='flex flex-col md:flex-row gap-6 md:gap-8'>
              <div className='flex-1 min-w-0'>
                <div className={activeTab === 'card' ? 'block' : 'hidden'}>
                  <Form.Input
                    field='title'
                    label={t('卡片标题')}
                    placeholder={t('请输入卡片标题')}
                    rules={[{ required: true, message: t('请输入卡片标题') }]}
                  />
                  {/* Logo 图片传到本站 static/douyin_card，URL 随卡片数据提交外部接口。
                      Logo 为必填，用 field='cover' 交给表单规则校验；
                      格式说明用 extraText 交给表单渲染，会排在错误信息下方 */}
                  <ImageUploadField
                    field='cover'
                    label={t('卡片Logo')}
                    extraText={t('支持 JPG / PNG / GIF / WebP，不超过 5MB')}
                    rules={[{ required: true, message: t('请上传卡片Logo') }]}
                    {...imageBoxProps('logo', 118, 'logo')}
                  />
                  {/* 跳转链接输入框已移除：新建/编辑提交时由服务端统一注入
                      系统设置「抖音私信卡片 → 微信小程序页面地址」的值 */}
                </div>
                <div className={activeTab === 'link' ? 'block' : 'hidden'}>
                  {/* 字段顺序与外部落地页表单一致：微信头像 → 微信昵称 → 微信描述 → 上传二维码 → 底部操作提示。
                      头像/昵称/描述/二维码为必填（推广落地页四要素）；仅底部操作提示选填。
                      编辑旧记录时若这些字段为空，需补全后才能提交 */}
                  <ImageUploadField
                    field='avatar'
                    label={t('微信头像')}
                    rules={[{ required: true, message: t('请上传微信头像') }]}
                    {...imageBoxProps('avatar', 88, 'weixin')}
                  />
                  <Form.Input
                    field='nickname'
                    label={t('微信昵称')}
                    placeholder={t('请输入微信昵称')}
                    rules={[{ required: true, message: t('请输入微信昵称') }]}
                  />
                  <Form.Input
                    field='description'
                    label={t('微信描述')}
                    placeholder={t('请输入微信描述')}
                    rules={[{ required: true, message: t('请输入微信描述') }]}
                  />
                  <ImageUploadField
                    field='qrcode'
                    label={t('上传二维码')}
                    rules={[{ required: true, message: t('请上传二维码') }]}
                    {...imageBoxProps('qrcode', 96, 'weixin')}
                  />
                  <Form.Input
                    field='action_tip'
                    label={t('底部操作提示')}
                    placeholder={t('请输入底部操作提示')}
                  />
                </div>
              </div>
              {/* 预览列：固定宽度的浅色底「预览台」，只渲染当前页签的预览
                  （切换胶囊时机身位置不跳动；表单字段少的页签也不会显得空落） */}
              <div className='flex flex-none justify-center rounded-xl bg-[var(--semi-color-fill-0)] px-4 py-2 md:w-[332px]'>
                {activeTab === 'card' ? (
                  <PhonePreview
                    height={previewHeight}
                    maxWidth={isMobile ? 260 : 300}
                    title={draft.title}
                    desc={draft.desc}
                    cover={draft.cover}
                  />
                ) : (
                  <WechatPreview
                    height={previewHeight}
                    maxWidth={isMobile ? 260 : 300}
                    avatar={draft.avatar}
                    nickname={draft.nickname}
                    description={draft.description}
                    qrcode={draft.qrcode}
                    actionTip={draft.actionTip}
                  />
                )}
              </div>
            </div>
          </Form>
        )}
      </Modal>
    </>
  );
};

export default DouyinCardsTable;
