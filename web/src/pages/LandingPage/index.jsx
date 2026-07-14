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
import './index.css';

import { getLogo } from '@/helpers';

const logo = getLogo();

const COUPON_LINK =
  'allrouter.ai/register?aff=CinA&utm_source=jinse&utm_medium=event&utm_campaign=ai_money_competition_2026&utm_content=coupon_card';

const LandingPage = () => {
  const [claimModalOpen, setClaimModalOpen] = useState(false);
  const [toastVisible, setToastVisible] = useState(false);
  const toastTimerRef = useRef(null);

  useEffect(() => {
    const handleKeyDown = (event) => {
      if (event.key === 'Escape') {
        setClaimModalOpen(false);
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, []);

  useEffect(
    () => () => {
      if (toastTimerRef.current) {
        window.clearTimeout(toastTimerRef.current);
      }
    },
    [],
  );

  const showCopiedToast = () => {
    if (toastTimerRef.current) {
      window.clearTimeout(toastTimerRef.current);
    }
    setToastVisible(true);
    toastTimerRef.current = window.setTimeout(() => {
      setToastVisible(false);
    }, 1600);
  };

  const copyCouponLink = async () => {
    try {
      await navigator.clipboard.writeText(COUPON_LINK);
    } catch {
      const textarea = document.createElement('textarea');
      textarea.value = COUPON_LINK;
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      textarea.remove();
    }
    showCopiedToast();
  };

  return (
    <div className='activity-landing-page'>
      <nav className='nav'>
        <div className='shell nav-inner'>
          <a className='brand' href='#top' aria-label='AllRouter 首页'>
            <span className='logo'>
              <img src={logo} />
            </span>
            <span>
              AllRouter<span className='brand-accent'>.AI</span>
              <small>官方算力伙伴 · AI搞钱争霸赛</small>
            </span>
          </a>
          <div className='nav-links'>
            <a href='#coupon' className='keep'>
              领券
            </a>
            <a href='#tutorial'>3分钟接入</a>
            <a href='#utm'>接入优势</a>
            <a href='#faq'>FAQ</a>
            <a
              className='btn btn-primary keep'
              href='https://allrouter.ai/register?utm_source=jinse&utm_medium=event&utm_campaign=ai_money_competition_2026&utm_content=nav_coupon'
              target='_blank'
              rel='noopener noreferrer'
            >
              立即领取
            </a>
          </div>
        </div>
      </nav>

      <main id='top'>
        <section className='hero'>
          <div className='shell hero-grid'>
            <div className='hero-copy'>
              <div className='eyebrow'>
                <span className='pulse' /> 金色财经 × Twinkle × 大树财经 ·
                参赛企业专属
              </div>
              <h1>
                <span className='grad'>GLM-5.2 自建算力</span>
                <br />
                高精度，不降智
              </h1>
              <p className='lead'>
                AllRouter.AI 为「AI 搞钱争霸赛」提供 7000U API
                算力额度。参赛团队可领取专项代金券，使用自营 H200/B300 集群上的
                GLM-5.2，在高精度输出与稳定推理体验下快速接入业务。
              </p>
              <div className='hero-actions'>
                <button
                  className='btn btn-primary'
                  type='button'
                  onClick={() => setClaimModalOpen(true)}
                >
                  领取活动代金券
                </button>
                <a className='btn btn-ghost' href='#tutorial'>
                  查看接入教程
                </a>
              </div>
              <div className='micro'>
                专项额度用于
                GLM-5.2；自建算力直供，适合高质量问答、代码生成与智能体工作流试调。
              </div>
              <div className='trust' aria-label='核心优势'>
                <div className='trust-item'>
                  <b>自建算力</b>
                  <span>H200/B300 GPU 集群直供 GLM-5.2</span>
                </div>
                <div className='trust-item'>
                  <b>高精度输出</b>
                  <span>面向问答、代码、智能体任务稳定试调</span>
                </div>
                <div className='trust-item'>
                  <b>不降智体验</b>
                  <span>减少模型能力折损，保持复杂任务推理质量</span>
                </div>
              </div>
            </div>

            <aside
              className='coupon-card'
              id='coupon'
              aria-label='活动代金券卡片'
            >
              <div className='coupon-inner'>
                <div className='coupon-top'>
                  <div>
                    <span className='badge'>社群传播代金券</span>
                  </div>
                  <div className='coupon-meta'>
                    发完即止
                    <br />
                    独立批次可核销
                  </div>
                </div>
                <div className='amount'>
                  <span className='num'>$10</span>
                  <span className='unit'>起</span>
                  <span className='desc'>GLM-5.2 API 体验额度</span>
                </div>
                <div className='coupon-list'>
                  <div className='coupon-row'>
                    <span>适用对象</span>
                    <b>活动社群 / 报名企业</b>
                  </div>
                  <div className='coupon-row'>
                    <span>使用范围</span>
                    <b>GLM-5.2</b>
                  </div>
                  <div className='coupon-row'>
                    <span>推荐有效期</span>
                    <b>30 天</b>
                  </div>
                  <div className='coupon-row'>
                    <span>追踪方式</span>
                    <b>UTM + 批次代金券</b>
                  </div>
                </div>
                <button
                  className='btn btn-primary coupon-claim-button'
                  type='button'
                  onClick={() => setClaimModalOpen(true)}
                >
                  扫码/点击领取专属额度
                </button>
                <div className='code-box'>
                  <code id='utmCode'>{COUPON_LINK}</code>
                  <button
                    className='btn btn-ghost copy'
                    type='button'
                    onClick={copyCouponLink}
                  >
                    复制
                  </button>
                </div>
              </div>
            </aside>
          </div>
        </section>

        <section className='section' id='tutorial'>
          <div className='shell'>
            <div className='section-head'>
              <div>
                <h2>3 分钟接入教程</h2>
                <p className='sub'>
                  复制下方参数即可开始试调：注册领取额度、替换 Base URL、选择
                  GLM-5.2 发起一次最小请求。
                </p>
              </div>
              <a
                className='btn btn-ghost'
                href='https://allrouter.ai/register?utm_source=jinse&utm_medium=event&utm_campaign=ai_money_competition_2026&utm_content=tutorial_cta'
                target='_blank'
                rel='noopener noreferrer'
              >
                注册并获取 API Key
              </a>
            </div>
            <div className='steps'>
              <article className='step'>
                <div className='step-num'>1</div>
                <h3>注册并兑换</h3>
                <p>
                  通过活动链接注册 AllRouter.AI，在「兑换码 /
                  礼品卡」入口输入活动码，GLM-5.2 额度即时到账。
                </p>
                <div className='snippet'>
                  <span className='green'>活动入口</span>
                  <br />
                  /register?utm_source=jinse...
                </div>
              </article>
              <article className='step'>
                <div className='step-num'>2</div>
                <h3>替换 Base URL</h3>
                <p>
                  现有 OpenAI SDK 项目只需要替换 base_url，并填入 AllRouter API
                  Key。
                </p>
                <div className='snippet'>
                  base_url=
                  <span className='yellow'>
                    &quot;https://allrouter.ai/v1&quot;
                  </span>
                  <br />
                  api_key=<span className='yellow'>&quot;sk-...&quot;</span>
                </div>
              </article>
              <article className='step'>
                <div className='step-num'>3</div>
                <h3>调用 GLM-5.2</h3>
                <p>
                  选择 GLM-5.2 或
                  GLM-5.2-Codex，先跑一个最小请求，再接入业务工作流。
                </p>
                <div className='snippet'>
                  model=<span className='yellow'>&quot;glm-5.2&quot;</span>
                  <br />
                  {'messages=[{"role":"user",...}]'}
                </div>
              </article>
            </div>

            <div className='channel-block' aria-label='常用渠道接入教程'>
              <div className='channel-head'>
                <div>
                  <h3>常用渠道接入方式</h3>
                  <p>
                    OpenClaw、Codex、Hermes 可按“自定义模型供应商”接入；Claude
                    Code 使用网关环境变量接入 AllRouter，再选择 GLM-5.2。
                  </p>
                </div>
                <span className='badge'>渠道配置速查</span>
              </div>
              <div className='channel-grid'>
                <article className='channel-card'>
                  <div className='channel-title'>
                    <span className='channel-logo openclaw' aria-hidden='true'>
                      <img
                        src='https://unpkg.com/@lobehub/icons-static-svg@latest/icons/openclaw-color.svg'
                        alt=''
                        loading='lazy'
                      />
                    </span>
                    <span>
                      <b>OpenClaw</b>
                      <span>OpenAI-compatible</span>
                    </span>
                  </div>
                  <ol>
                    <li>
                      进入 Settings / Providers，新建 OpenAI Compatible
                      Provider。
                    </li>
                    <li>
                      Base URL 填写 <code>https://allrouter.ai/v1</code>，API
                      Key 填写 AllRouter Key。
                    </li>
                    <li>
                      模型名称填写 <code>glm-5.2</code> 或{' '}
                      <code>glm-5.2-codex</code>，保存后开始对话。
                    </li>
                  </ol>
                  <div className='channel-mini'>
                    Provider: AllRouter · Model: glm-5.2-codex
                  </div>
                </article>

                <article className='channel-card'>
                  <div className='channel-title'>
                    <span className='channel-logo codex' aria-hidden='true'>
                      <img
                        src='https://unpkg.com/@lobehub/icons-static-svg@latest/icons/openai.svg'
                        alt=''
                        loading='lazy'
                      />
                    </span>
                    <span>
                      <b>ChatGPT（Codex）</b>
                      <span>代码任务优先</span>
                    </span>
                  </div>
                  <ol>
                    <li>
                      在支持自定义 API Endpoint 的 Codex 客户端或插件中，选择
                      Custom / OpenAI Compatible。
                    </li>
                    <li>
                      Endpoint 填写 AllRouter Base URL，并粘贴活动账户的 API
                      Key。
                    </li>
                    <li>
                      代码生成、重构、调试任务优先选择{' '}
                      <code>glm-5.2-codex</code>。
                    </li>
                  </ol>
                  <div className='channel-mini'>
                    base_url=https://allrouter.ai/v1
                  </div>
                </article>

                <article className='channel-card'>
                  <div className='channel-title'>
                    <span className='channel-logo hermes' aria-hidden='true'>
                      <img
                        src='https://unpkg.com/@lobehub/icons-static-svg@latest/icons/hermesagent.svg'
                        alt=''
                        loading='lazy'
                      />
                    </span>
                    <span>
                      <b>Hermes</b>
                      <span>Hermes Agent</span>
                    </span>
                  </div>
                  <ol>
                    <li>
                      打开 Hermes 的 Model Provider 设置，新增自定义 OpenAI
                      协议供应商。
                    </li>
                    <li>
                      供应商名称填写 AllRouter，Base URL 与 API Key
                      使用活动账户配置。
                    </li>
                    <li>
                      将默认模型设为 <code>glm-5.2</code>
                      ，需要代码能力时切换到 <code>glm-5.2</code>。
                    </li>
                  </ol>
                  <div className='channel-mini'>Default model: glm-5.2</div>
                </article>

                <article className='channel-card'>
                  <div className='channel-title'>
                    <span className='channel-logo claude' aria-hidden='true'>
                      <img
                        src='https://unpkg.com/@lobehub/icons-static-svg@latest/icons/claudecode-color.svg'
                        alt=''
                        loading='lazy'
                      />
                    </span>
                    <span>
                      <b>Claude Code</b>
                      <span>LLM Gateway</span>
                    </span>
                  </div>
                  <ol>
                    <li>Claude Code 启动前通过环境变量接入网关。</li>
                    <li>
                      设置 <code>ANTHROPIC_BASE_URL</code> 为 AllRouter
                      网关地址，<code>ANTHROPIC_AUTH_TOKEN</code> 使用 AllRouter
                      API Key。
                    </li>
                    <li>
                      设置 <code>ANTHROPIC_MODEL=glm-5.2</code> 后启动 Claude
                      Code。
                    </li>
                  </ol>
                  <div className='channel-mini'>ANTHROPIC_MODEL=glm-5.2</div>
                </article>
              </div>
            </div>
          </div>
        </section>

        <section className='section' id='utm'>
          <div className='shell'>
            <div className='panel'>
              <h3>AllRouter 接入优势</h3>
              <ul>
                <li>
                  <span className='check'>✓</span>
                  <span>
                    统一 API 网关连接 20+ 主流模型，已有项目少量配置即可切换。
                  </span>
                </li>
                <li>
                  <span className='check'>✓</span>
                  <span>
                    GLM-5.2 专项额度面向活动团队发放，适合快速试跑产品 Demo。
                  </span>
                </li>
                <li>
                  <span className='check'>✓</span>
                  <span>
                    自营 GPU 集群提供稳定算力，支持 API 场景持续调用。
                  </span>
                </li>
                <li>
                  <span className='check'>✓</span>
                  <span>UTM 与批次代金券独立记录，便于活动方核销与复盘。</span>
                </li>
              </ul>
            </div>
          </div>
        </section>

        <section className='section' id='faq'>
          <div className='shell'>
            <div className='section-head'>
              <div>
                <h2>常见问题</h2>
                <p className='sub'>
                  常见问题提前说明，用户领券后如何用等。
                </p>
              </div>
            </div>
            <div className='faq'>
              <details open>
                <summary>代金券能调用 Claude / GPT 吗？</summary>
                <p>
                  活动额度专项用于 GLM-5.2；Claude / GPT / Gemini / DeepSeek 等
                  20+ 模型可按官网 8 折正常开通使用。
                </p>
              </details>
              <details>
                <summary>不会写代码也能用吗？</summary>
                <p>
                  如果已有 OpenAI 协议项目，只需替换 base_url 和 API
                  Key。企业参赛团队可进入活动接入群获取协助。
                </p>
              </details>
              <details>
                <summary>活动方如何看投放效果？</summary>
                <p>
                  每个渠道使用独立 UTM
                  链接，每批代金券码单独生成。后台可导出领取、兑换、首次调用数据，用于核销与复盘。
                </p>
              </details>
            </div>
          </div>
        </section>
      </main>

      <footer className='footer'>
        <div className='shell'>
          © AllRouter.AI · AI
          搞钱争霸赛官方算力伙伴。活动额度、有效期与适用模型以 AllRouter.AI
          账户展示及活动规则为准。
        </div>
      </footer>

      <div
        className={`modal${claimModalOpen ? ' show' : ''}`}
        role='dialog'
        aria-modal='true'
        aria-labelledby='modalTitle'
        onMouseDown={(event) => {
          if (event.target === event.currentTarget) {
            setClaimModalOpen(false);
          }
        }}
      >
        <div className='modal-card'>
          <button
            className='close'
            type='button'
            aria-label='关闭'
            onClick={() => setClaimModalOpen(false)}
          >
            ×
          </button>
          <span className='badge'>活动专属入口</span>
          <h3 id='modalTitle'>领取 GLM-5.2 活动代金券</h3>
          <p>
            点击下方按钮进入 AllRouter.AI 注册页，完成注册后在账户内领取或兑换
            GLM-5.2 专项额度。注册链接已带本次活动追踪参数，便于完成权益核销。
          </p>
          <div className='modal-actions'>
            <a
              className='btn btn-primary'
              href='https://allrouter.ai/register?utm_source=jinse&utm_medium=event&utm_campaign=ai_money_competition_2026&utm_content=claim_modal'
              target='_blank'
              rel='noopener noreferrer'
            >
              前往 AllRouter.AI 注册
            </a>
            <button
              className='btn btn-ghost'
              type='button'
              onClick={copyCouponLink}
            >
              复制链接
            </button>
          </div>
        </div>
      </div>
      <div className={`toast${toastVisible ? ' show' : ''}`}>已复制链接</div>
    </div>
  );
};

export default LandingPage;
