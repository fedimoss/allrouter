import React, { useEffect, useMemo } from 'react';
import { Anchor } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router-dom';
import { normalizeLanguage } from '../../i18n/language';
import docsZhCNMd from './docs_content.md?raw';
// import docsZhTWMd from './docs_content.zh-TW.md?raw';
import docsEnMd from './docs_content.en.md?raw';
// import docsFrMd from './docs_content.fr.md?raw';
// import docsJaMd from './docs_content.ja.md?raw';
// import docsRuMd from './docs_content.ru.md?raw';
// import docsViMd from './docs_content.vi.md?raw';
import docsOpenClawMd from './docs_content.openclaw.md?raw';
import { marked } from 'marked';

const DOCS_UPDATED_AT = '2026-04-17';

const USAGE_CONTENT_MAP = {
  'zh-CN': docsZhCNMd,
  // 'zh-TW': docsZhTWMd,
  en: docsEnMd,
  // fr: docsFrMd,
  // ru: docsRuMd,
  // ja: docsJaMd,
  // vi: docsViMd,
  default: docsEnMd,
};

const DOCS_SECTION_MAP = {
  usage: {
    contentMap: USAGE_CONTENT_MAP,
    titles: {
      'zh-CN': 'AllRouter.AI使用文档',
      // 'zh-TW': 'AllRouter.AI使用文件',
      en: 'Usage Guide',
      default: 'Usage Guide',
    },
  },
  openclaw: {
    contentMap: {
      default: docsOpenClawMd,
    },
    titles: {
      'zh-CN': '使用AllRouter.AI接入飞书妙搭openclaw手册',
      // 'zh-TW': '使用AllRouter.AI接入飛書妙搭openclaw手冊',
      en: 'Feishu Miaoda OpenClaw Manual',
      default: 'Feishu Miaoda OpenClaw Manual',
    },
  },
};

const DOCS_UI_TEXT_MAP = {
  'zh-CN': {
    navTitle: '文档导航',
    usageLabel: 'AllRouter.AI使用文档',
    openclawLabel: '飞书妙搭openclaw手册',
    tocTitle: '本文目录',
    updatedAtPrefix: '更新时间：',
  },
  'zh-TW': {
    navTitle: '文件導航',
    usageLabel: '使用文件',
    openclawLabel: '飛書妙搭openclaw手冊',
    tocTitle: '本文目錄',
    updatedAtPrefix: '更新時間：',
  },
  en: {
    navTitle: 'Documents',
    usageLabel: 'Usage Guide',
    openclawLabel: 'Feishu Miaoda OpenClaw Manual',
    tocTitle: 'On this page',
    updatedAtPrefix: 'Last updated: ',
  },
  default: {
    navTitle: 'Documents',
    usageLabel: 'Usage Guide',
    openclawLabel: 'Feishu Miaoda OpenClaw Manual',
    tocTitle: 'On this page',
    updatedAtPrefix: 'Last updated: ',
  },
};

const DOC_NAV_ITEMS = [
  { key: 'usage', labelKey: 'usageLabel' },
  { key: 'openclaw', labelKey: 'openclawLabel' },
];

const generateAnchorId = (rawText, anchorCountMap) => {
  const normalizedText = rawText
    .toLowerCase()
    .trim()
    .replace(/[^\p{L}\p{N}\p{Script=Han}]+/gu, '-')
    .replace(/^-+|-+$/g, '');
  const baseAnchor = normalizedText || 'section';
  const count = anchorCountMap.get(baseAnchor) || 0;

  anchorCountMap.set(baseAnchor, count + 1);
  if (count === 0) {
    return baseAnchor;
  }

  return `${baseAnchor}-${count + 1}`;
};

const Docs = () => {
  const { i18n } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();

  const language = normalizeLanguage(
    i18n.resolvedLanguage || i18n.language || 'zh-CN',
  );
  const docsUiText = DOCS_UI_TEXT_MAP[language] || DOCS_UI_TEXT_MAP.default;

  const requestedDocKey = searchParams.get('doc');
  const activeDocKey = DOCS_SECTION_MAP[requestedDocKey]
    ? requestedDocKey
    : 'usage';
  const activeSection = DOCS_SECTION_MAP[activeDocKey];
  const docsMarkdown =
    activeSection.contentMap[language] ||
    activeSection.contentMap.default ||
    docsOpenClawMd;
  const pageTitle =
    activeSection.titles[language] || activeSection.titles.default;

  useEffect(() => {
    if (requestedDocKey && !DOCS_SECTION_MAP[requestedDocKey]) {
      setSearchParams({}, { replace: true });
    }
  }, [requestedDocKey, setSearchParams]);

  const { htmlContent, toc } = useMemo(() => {
    const renderer = new marked.Renderer();
    const headings = [];
    const anchorCountMap = new Map();

    renderer.heading = function (text, level, raw) {
      const headingText = String(text).replace(/<[^>]+>/g, '').trim();
      const anchor = generateAnchorId(
        String(raw || headingText).replace(/<[^>]+>/g, ''),
        anchorCountMap,
      );
      headings.push({ id: anchor, level, text: headingText });
      return `<h${level} id="${anchor}">${text}</h${level}>\n`;
    };

    return {
      htmlContent: marked.parse(docsMarkdown, { renderer }),
      toc: headings,
    };
  }, [docsMarkdown]);

  const defaultAnchor = toc.length > 0 ? `#${toc[0].id}` : '';

  const handleDocChange = (docKey) => {
    const next = new URLSearchParams(searchParams);
    if (docKey === 'usage') {
      next.delete('doc');
    } else {
      next.set('doc', docKey);
    }
    setSearchParams(next);

    const search = next.toString();
    window.history.replaceState(
      null,
      '',
      `${window.location.pathname}${search ? `?${search}` : ''}`,
    );
    window.scrollTo({ top: 0, behavior: 'auto' });
  };

  return (
    <div className='docs-shell mt-[60px] pb-[60px] max-w-[1600px] mx-auto px-4 sm:px-6 lg:px-8'>
      <style>{`
        .docs-shell {
          --docs-shell-width: 1600px;
          --docs-shell-padding: 16px;
          --docs-left-width: 280px;
          --docs-right-width: 260px;
        }
        .docs-page-grid {
          display: grid;
          grid-template-columns: var(--docs-left-width) minmax(0, 1fr) var(--docs-right-width);
          gap: 24px;
          align-items: start;
        }
        .docs-panel {
          background: var(--semi-color-bg-0);
          border: 1px solid var(--semi-color-border);
          border-radius: 20px;
          box-shadow: 0 10px 30px rgba(15, 23, 42, 0.06);
        }
        .docs-sticky {
          position: sticky;
          top: 80px;
        }
        .docs-left-nav {
          padding: 20px;
        }
        .docs-left-nav-title {
          font-size: 13px;
          font-weight: 700;
          color: var(--semi-color-text-2);
          letter-spacing: 0.04em;
          text-transform: uppercase;
          margin-bottom: 12px;
        }
        .docs-left-nav-list {
          display: flex;
          flex-direction: column;
          gap: 10px;
        }
        .docs-left-nav-button {
          width: 100%;
          border: 1px solid transparent;
          border-radius: 14px;
          background: var(--semi-color-fill-0);
          color: var(--semi-color-text-1);
          cursor: pointer;
          font-size: 14px;
          font-weight: 600;
          line-height: 1.5;
          padding: 14px 16px;
          text-align: left;
          white-space: normal;
          word-break: break-word;
          transition: all 0.2s ease;
        }
        .docs-left-nav-button:hover {
          border-color: var(--semi-color-primary-light-hover);
          background: var(--semi-color-primary-light-default);
          color: var(--semi-color-text-0);
        }
        .docs-left-nav-button.is-active {
          border-color: var(--semi-color-primary);
          background: var(--semi-color-primary-light-default);
          color: var(--semi-color-primary);
          box-shadow: inset 0 0 0 1px var(--semi-color-primary-light-hover);
        }
        .docs-content-panel {
          min-width: 0;
          padding: 40px 48px;
        }
        .docs-content-header {
          margin-bottom: 32px;
          padding-bottom: 24px;
          border-bottom: 1px solid var(--semi-color-border);
        }
        .docs-content-title {
          font-size: 34px;
          line-height: 1.2;
          font-weight: 800;
          color: var(--semi-color-text-0);
          margin: 0 0 12px;
        }
        .docs-content-updated {
          font-size: 14px;
          color: var(--semi-color-text-2);
        }
        .docs-content h1 {
          font-size: 2.1em;
          font-weight: 700;
          margin: 1.5em 0 0.8em;
          padding-bottom: 0.3em;
          border-bottom: 1px solid var(--semi-color-border);
          color: var(--semi-color-text-0);
          scroll-margin-top: 90px;
        }
        .docs-content h2 {
          font-size: 1.55em;
          font-weight: 700;
          margin: 1.35em 0 0.65em;
          color: var(--semi-color-text-0);
          scroll-margin-top: 90px;
        }
        .docs-content h3 {
          font-size: 1.2em;
          font-weight: 700;
          margin: 1.2em 0 0.5em;
          color: var(--semi-color-text-0);
          scroll-margin-top: 90px;
        }
        .docs-content h4,
        .docs-content h5,
        .docs-content h6 {
          font-size: 1.05em;
          font-weight: 700;
          margin: 1.1em 0 0.5em;
          color: var(--semi-color-text-1);
          scroll-margin-top: 90px;
        }
        .docs-content p,
        .docs-content li,
        .docs-content blockquote,
        .docs-content td,
        .docs-content th {
          color: var(--semi-color-text-1);
          line-height: 1.85;
          word-break: break-word;
          overflow-wrap: anywhere;
        }
        .docs-content p {
          margin: 0.85em 0;
        }
        .docs-content ul,
        .docs-content ol {
          margin: 1em 0;
          padding-left: 24px;
        }
        .docs-content li + li {
          margin-top: 0.45em;
        }
        .docs-content img {
          display: block;
          max-width: 100%;
          height: auto;
          margin: 24px auto;
          border-radius: 12px;
          box-shadow: 0 8px 24px rgba(15, 23, 42, 0.08);
        }
        .docs-content a {
          color: var(--semi-color-primary);
          text-decoration: none;
          word-break: break-word;
        }
        .docs-content a:hover {
          text-decoration: underline;
        }
        .docs-content code {
          background: var(--semi-color-fill-0);
          border-radius: 6px;
          color: #d9485f;
          font-size: 0.9em;
          padding: 2px 6px;
        }
        .docs-content pre {
          background: #0f172a;
          border-radius: 14px;
          color: #e2e8f0;
          margin: 1.5em 0;
          overflow-x: auto;
          padding: 18px 20px;
        }
        .docs-content pre code {
          background: transparent;
          color: inherit;
          padding: 0;
        }
        .docs-content blockquote {
          border-left: 4px solid var(--semi-color-primary);
          background: var(--semi-color-primary-light-default);
          border-radius: 0 12px 12px 0;
          color: var(--semi-color-text-1);
          margin: 1.5em 0;
          padding: 12px 16px;
        }
        .docs-content table {
          width: 100%;
          border-collapse: collapse;
          margin: 1.5em 0;
          display: block;
          overflow-x: auto;
          overflow-y: hidden;
        }
        .docs-content tbody,
        .docs-content thead,
        .docs-content tr {
          width: 100%;
        }
        .docs-content table th,
        .docs-content table td {
          border: 1px solid var(--semi-color-border);
          padding: 10px 12px;
          text-align: left;
        }
        .docs-content table th {
          background: var(--semi-color-fill-0);
          color: var(--semi-color-text-0);
          font-weight: 700;
        }
        .docs-toc {
          padding: 20px 18px 20px 20px;
          max-height: calc(100vh - 100px);
          overflow-y: auto;
          scrollbar-width: thin;
          scrollbar-color: var(--semi-color-border) transparent;
        }
        .docs-toc::-webkit-scrollbar {
          width: 4px;
        }
        .docs-toc::-webkit-scrollbar-thumb {
          background: var(--semi-color-border);
          border-radius: 999px;
        }
        .docs-toc-title {
          font-size: 13px;
          font-weight: 700;
          color: var(--semi-color-text-2);
          letter-spacing: 0.04em;
          text-transform: uppercase;
          margin-bottom: 10px;
          padding-left: 12px;
        }
        .docs-toc-anchor .semi-anchor-line {
          display: none !important;
        }
        .docs-toc-anchor.semi-anchor {
          overflow-y: visible !important;
          overflow-x: visible !important;
          max-height: none !important;
          height: auto !important;
        }
        .docs-toc-anchor .semi-anchor-slider {
          background-color: var(--semi-color-primary) !important;
          width: 2px !important;
          border-radius: 999px;
          left: 0 !important;
        }
        .docs-toc-anchor .semi-anchor-link {
          padding: 6px 0;
        }
        .docs-toc-item {
          display: block;
          max-width: 220px;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
          color: var(--semi-color-text-2);
          transition: color 0.2s ease;
        }
        .semi-anchor-link-active .docs-toc-item,
        .docs-toc-item:hover {
          color: var(--semi-color-primary);
        }
        @media (max-width: 1280px) {
          .docs-shell {
            --docs-shell-padding: 24px;
            --docs-left-width: 260px;
          }
          .docs-page-grid {
            grid-template-columns: var(--docs-left-width) minmax(0, 1fr);
          }
          .docs-right-toc {
            display: none;
          }
        }
        @media (min-width: 1281px) {
          .docs-left-column {
            width: var(--docs-left-width);
          }
          .docs-right-toc {
            width: var(--docs-right-width);
          }
          .docs-left-column .docs-panel,
          .docs-right-toc .docs-panel {
            position: fixed;
            top: 80px;
          }
          .docs-left-column .docs-panel {
            left: max(
              16px,
              calc(
                50vw - var(--docs-shell-width) / 2 + var(--docs-shell-padding)
              )
            );
            width: var(--docs-left-width);
          }
          .docs-right-toc .docs-panel {
            right: max(
              16px,
              calc(
                50vw - var(--docs-shell-width) / 2 + var(--docs-shell-padding)
              )
            );
            width: var(--docs-right-width);
          }
          .docs-page-grid {
            grid-template-columns: var(--docs-left-width) minmax(0, 1fr) var(--docs-right-width);
          }
        }
        @media (max-width: 960px) {
          .docs-shell {
            overflow-x: hidden;
          }
          .docs-page-grid {
            grid-template-columns: 1fr;
            gap: 16px;
          }
          .docs-right-toc {
            display: none;
          }
          .docs-sticky {
            position: static;
          }
          .docs-left-column .docs-panel {
            position: static;
            width: auto;
          }
          .docs-left-nav {
            padding: 16px;
          }
          .docs-left-nav-list {
            gap: 8px;
          }
          .docs-left-nav-button {
            padding: 12px 14px;
            font-size: 13px;
          }
          .docs-content-panel {
            padding: 28px 20px;
            overflow-x: hidden;
          }
          .docs-content-title {
            font-size: 28px;
          }
          .docs-content img,
          .docs-content pre,
          .docs-content table {
            max-width: 100%;
          }
        }
      `}</style>

      <div className='docs-page-grid'>
        <aside className='docs-left-column'>
          <div className='docs-panel docs-sticky docs-left-nav'>
            <div className='docs-left-nav-title'>{docsUiText.navTitle}</div>
            <div className='docs-left-nav-list'>
              {DOC_NAV_ITEMS.map((item) => (
                <button
                  key={item.key}
                  type='button'
                  className={`docs-left-nav-button ${
                    activeDocKey === item.key ? 'is-active' : ''
                  }`}
                  onClick={() => handleDocChange(item.key)}
                >
                  {docsUiText[item.labelKey]}
                </button>
              ))}
            </div>
          </div>
        </aside>

        <main className='docs-panel docs-content-panel'>
          <div className='docs-content-header'>
            <h1 className='docs-content-title'>{pageTitle}</h1>
            {/* <div className='docs-content-updated'>
              {docsUiText.updatedAtPrefix}
              {DOCS_UPDATED_AT}
            </div> */}
          </div>

          <div
            className='docs-content'
            dangerouslySetInnerHTML={{ __html: htmlContent }}
          />
        </main>

        <aside className='docs-right-toc'>
          <div className='docs-panel docs-sticky docs-toc'>
            <div className='docs-toc-title'>{docsUiText.tocTitle}</div>
            {toc.length > 0 && (
              <div style={{ position: 'relative' }}>
                <div
                  style={{
                    position: 'absolute',
                    left: 0,
                    top: 0,
                    bottom: 0,
                    width: 1,
                    backgroundColor: 'var(--semi-color-border)',
                  }}
                />
                <Anchor
                  key={`${activeDocKey}-${defaultAnchor}`}
                  defaultAnchor={defaultAnchor}
                  showTooltip
                  targetOffset={88}
                  style={{ background: 'transparent' }}
                  className='docs-toc-anchor'
                >
                  {toc.map((item) => {
                    let paddingLeft = 12;
                    if (item.level === 2) paddingLeft = 24;
                    if (item.level === 3) paddingLeft = 36;
                    if (item.level >= 4) paddingLeft = 48;

                    return (
                      <Anchor.Link
                        key={item.id}
                        href={`#${item.id}`}
                        title={
                          <span
                            className='docs-toc-item'
                            style={{
                              paddingLeft,
                              fontSize: 13,
                              fontWeight: 500,
                              lineHeight: '18px',
                            }}
                          >
                            {item.text}
                          </span>
                        }
                      />
                    );
                  })}
                </Anchor>
              </div>
            )}
          </div>
        </aside>
      </div>
    </div>
  );
};

export default Docs;
