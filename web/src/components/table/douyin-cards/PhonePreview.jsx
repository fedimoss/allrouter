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

import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { IconImage, IconQrCode, IconUser } from '@douyinfe/semi-icons';
import './phone-preview.css';

// 响应式视口高度：机身框按来源页面的尺寸等比摆放，视口矮时再按可用高度收一次，
// 避免机身把弹窗撑得过高。父组件（窄屏下要给预览单独限高）也用它取实时高度。
export const useViewportHeight = () => {
  const [height, setHeight] = useState(() =>
    typeof window === 'undefined' ? 900 : window.innerHeight,
  );
  useEffect(() => {
    const onResize = () => setHeight(window.innerHeight);
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);
  return height;
};

// 机身框：两个预览共用同一张机身描边图，参数集中在这里定义，
// 免得两处各自写一份而慢慢走样（改机身只需改这里）。
// frame 指向 public 下的图，图内屏幕区域完全透明，内容从镂空处透出；
// aspect 是机身框宽高比，来源页面就是按这个比例铺这张图的。
const PHONE_FRAME = {
  src: '/iphone-link.png',
  aspect: 461 / 845,
  // 屏幕镂空占机身框的比例（该图 461x845 的镂空范围为 x 30..429、y 21..819），
  // 用于把内容缩放到屏幕宽度
  holeWidth: 400 / 461,
};

// 手机预览外壳：框内屏幕里放来源页面原样搬过来的内容，
// 由父组件传入当前表单值实现实时联动。
//   height    机身框高度（px）：来源页面的机身框高度就是 550，这里按它等比摆放，
//             视口矮或所在列窄时再按可用空间收一次（等比缩小，比例不变）
//   maxWidth  机身框宽度上限（px），横向空间不够时按宽度收
//   base      来源页面里屏幕镂空处的内容宽度（容器扣掉机身边框那圈内边距后的宽度）
// CSS 无法由长度算出 scale() 需要的无单位倍数，所以 scale 在组件里按屏幕宽度算好传给样式。
const PhoneShell = ({ height = 550, maxWidth, base, variant, children }) => {
  const { src: frame, aspect, holeWidth } = PHONE_FRAME;
  const viewportHeight = useViewportHeight();
  // 扣除弹窗的表头/底部按钮（见 Modal 的 bodyStyle）与预览台的上下内边距，
  // 机身最多占满其余高度，这样表单区不会被撑出滚动条
  let phoneHeight = Math.min(height, Math.max(320, viewportHeight - 224));
  let phoneWidth = phoneHeight * aspect;
  // 列宽不够时按宽度收，高度同步收，保持机身比例
  if (maxWidth && phoneWidth > maxWidth) {
    phoneWidth = maxWidth;
    phoneHeight = phoneWidth / aspect;
  }
  const scale = (phoneWidth * holeWidth) / base;
  return (
    <div
      className={`douyin-phone${variant ? ` is-${variant}` : ''}`}
      style={{
        width: `${phoneWidth}px`,
        height: `${phoneHeight}px`,
        '--phone-scale': scale,
      }}
    >
      <div className='douyin-phone-screen'>
        <div className='douyin-phone-page'>{children}</div>
      </div>
      {/* 机身图盖在屏幕之上：图内屏幕区域透明，内容从镂空处透出 */}
      <img className='douyin-phone-frame' src={frame} alt='' />
    </div>
  );
};

// 私信卡片：结构取自「图文分享卡片制作」页面的 .phone-cotain
// （屏幕 .phone-container = 顶部「卡片样式展示」地址栏 + .card-douyin 卡片）
// base 取内容区宽度：310 - 16 - 16 = 278
// desc 为卡片描述（外部记录的 desc 字段），未填写时沿用来源页面的固定副标题文案
export const PhonePreview = ({ height, title, desc, cover }) => {
  const { t } = useTranslation();
  return (
    <PhoneShell height={height} base={278}>
      <div className='phone-container'>
        <div className='phone-container-browser'>{t('卡片样式展示')}</div>
        <div className='card-douyin'>
          {cover ? (
            <img src={cover} alt='' />
          ) : (
            <span className='card-douyin-img-empty'>
              <IconImage />
            </span>
          )}
          <div className='info'>
            <h3>{title?.trim() ? title : t('点击咨询')}</h3>
            <h5>{desc?.trim() || t('已通过官方安全认证')}</h5>
          </div>
        </div>
      </div>
    </PhoneShell>
  );
};

// 推广链接：结构取自「添加推广链接」页面的 .phone-cotainer-main
// （灰底页面 .phone-default-cotain + 白卡 .phone-default：头像 + 昵称/描述、二维码、底部操作提示）
// base 取内容区宽度：300 - 19 - 19 = 262
export const WechatPreview = ({
  height,
  avatar,
  nickname,
  description,
  qrcode,
  actionTip,
}) => {
  const { t } = useTranslation();
  // 未填写时用字段名占位，并置灰区别于已填内容
  const placeholder = (value) => (value?.trim() ? '' : ' is-placeholder');
  return (
    <PhoneShell height={height} base={262} variant='wechat-link'>
      <div className='phone-cotainer-main'>
        <div className='phone-body-full' style={{ display: 'none' }} />
        <div className='phone-default-cotain' style={{ display: 'block' }}>
          <div className='phone-default'>
            <div className='phone-default-header'>
              {avatar ? (
                <img src={avatar} alt='' />
              ) : (
                <span className='phone-default-header-img-empty'>
                  <IconUser />
                </span>
              )}
              <div className='phone-default-header-right'>
                <h3 className={placeholder(nickname).trim()}>
                  {nickname?.trim() || t('微信昵称')}
                </h3>
                <h5 className={placeholder(description).trim()}>
                  {description?.trim() || t('微信描述')}
                </h5>
              </div>
            </div>
            {qrcode ? (
              <img className='phone-default-qrcode' src={qrcode} alt='' />
            ) : (
              <div className='phone-default-qrcode is-empty'>
                <IconQrCode />
              </div>
            )}
            <div className={`phone-default-footer${placeholder(actionTip)}`}>
              {actionTip?.trim() || t('底部操作提示')}
            </div>
          </div>
        </div>
      </div>
    </PhoneShell>
  );
};

export default PhonePreview;
