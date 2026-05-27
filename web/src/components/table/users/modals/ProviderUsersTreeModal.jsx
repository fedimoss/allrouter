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

import React, { useEffect, useMemo, useState } from 'react';
import {
  FreeLayoutEditor,
  LineType,
  WorkflowNodeRenderer,
  useNodeRender,
  usePlaygroundTools,
} from '@flowgram.ai/free-layout-editor';
import '@flowgram.ai/free-layout-editor/index.css';
import {
  Avatar,
  Banner,
  Button,
  Empty,
  Radio,
  RadioGroup,
  SideSheet,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconClose,
  IconMinus,
  IconPlus,
  IconRefresh,
  IconTreeTriangleDown,
} from '@douyinfe/semi-icons';
import { GitBranch, UserRound } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, getUserIdFromLocalStorage, showError } from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const DEFAULT_TREE_PAGE_SIZE = 50;
const FLOWGRAM_NODE_TYPE = 'provider-user-tree-node';
const FLOWGRAM_NODE_RENDER_KEY = 'provider-user-tree-node-render';
const FLOWGRAM_NODE_WIDTH = 260;
const FLOWGRAM_NODE_HEIGHT = 86;
const FLOWGRAM_LEVEL_GAP = 360;
const FLOWGRAM_SIBLING_GAP = 42;
const FLOWGRAM_CANVAS_PADDING = 80;
const FLOWGRAM_INPUT_PORT_ID = 'in';
const FLOWGRAM_OUTPUT_PORT_ID = 'out';
const FLOWGRAM_LINE_COLOR = '#22c55e';

const sortById = (a, b) => (a?.id || 0) - (b?.id || 0);

const deduplicateUsers = (users = []) => {
  const userMap = new Map();
  users.forEach((user) => {
    if (user?.id) {
      userMap.set(user.id, user);
    }
  });
  return Array.from(userMap.values()).sort(sortById);
};

const normalizeNode = (user, isLoaded = false) => ({
  ...user,
  children: Array.isArray(user?.children) ? user.children : [],
  has_more_children: Boolean(user?.has_more_children),
  isLoaded,
  pagination: user?.pagination || null,
});

const buildRootChildren = (users, rootUserId) => {
  return deduplicateUsers(users)
    .filter((user) => user?.id && user.id !== rootUserId)
    .map((user) => normalizeNode(user, false));
};

const buildChildNodes = (users) => {
  return deduplicateUsers(users).map((user) => normalizeNode(user, false));
};

const updateTreeNode = (node, targetId, updater) => {
  if (!node) {
    return node;
  }
  if (node.id === targetId) {
    return updater(node);
  }
  if (!Array.isArray(node.children) || node.children.length === 0) {
    return node;
  }
  return {
    ...node,
    children: node.children.map((child) => updateTreeNode(child, targetId, updater)),
  };
};

const countNodes = (node) => {
  if (!node) {
    return 0;
  }
  return (
    1 +
    (Array.isArray(node.children)
      ? node.children.reduce((sum, child) => sum + countNodes(child), 0)
      : 0)
  );
};

const mergeChildrenById = (oldChildren = [], newChildren = []) => {
  const childMap = new Map();
  oldChildren.forEach((child) => {
    if (child?.id) {
      childMap.set(child.id, child);
    }
  });
  newChildren.forEach((child) => {
    if (child?.id) {
      childMap.set(child.id, child);
    }
  });
  return Array.from(childMap.values()).sort(sortById);
};

const getNodeTitle = (node) => node?.display_name || node?.username || '-';

const getNodeRelationText = (node, t) => {
  if (node?.isRoot) {
    return t('服务商主账号');
  }
  return node?.inviter_id ? `${t('邀请人')}: #${node.inviter_id}` : '';
};

const getFlowgramNodeId = (node) => `provider-user-${node.id}`;

const getNodeDepth = (node) => {
  if (!node || !Array.isArray(node.children) || node.children.length === 0) {
    return 1;
  }
  return 1 + Math.max(...node.children.map((child) => getNodeDepth(child)));
};

const getNodeLeafCount = (node) => {
  if (!node || !Array.isArray(node.children) || node.children.length === 0) {
    return 1;
  }
  return node.children.reduce((sum, child) => sum + getNodeLeafCount(child), 0);
};

const getFlowgramNodePositionMap = (nodes = []) => {
  return nodes.reduce((positionMap, node) => {
    if (node?.id && node?.meta?.position) {
      positionMap[node.id] = node.meta.position;
    }
    return positionMap;
  }, {});
};

const getFlowgramEdgePath = (sourcePosition, targetPosition) => {
  const sourceX = sourcePosition.x + FLOWGRAM_NODE_WIDTH;
  const sourceY = sourcePosition.y + FLOWGRAM_NODE_HEIGHT / 2;
  const targetX = targetPosition.x;
  const targetY = targetPosition.y + FLOWGRAM_NODE_HEIGHT / 2;
  const midX = sourceX + Math.max((targetX - sourceX) / 2, 48);

  return `M ${sourceX} ${sourceY} C ${midX} ${sourceY}, ${midX} ${targetY}, ${targetX} ${targetY}`;
};

const FLOWGRAM_DEFAULT_PORTS = [
  {
    portID: FLOWGRAM_INPUT_PORT_ID,
    type: 'input',
    location: 'left',
    disabled: true,
  },
  {
    portID: FLOWGRAM_OUTPUT_PORT_ID,
    type: 'output',
    location: 'right',
    disabled: true,
  },
];

const convertTreeToFlowgramData = (treeNode, t) => {
  if (!treeNode) {
    return { nodes: [], edges: [] };
  }

  const nodes = [];
  const edges = [];
  let nextLeafIndex = 0;

  const walk = (node, depth = 0, parent = null) => {
    const children = Array.isArray(node.children) ? node.children : [];
    let currentLeafIndex;

    if (children.length === 0) {
      currentLeafIndex = nextLeafIndex;
      nextLeafIndex += 1;
    } else {
      const childLeafIndexes = children.map((child) => walk(child, depth + 1, node));
      currentLeafIndex =
        childLeafIndexes.reduce((sum, leafIndex) => sum + leafIndex, 0) /
        childLeafIndexes.length;
    }

    const nodeId = getFlowgramNodeId(node);
    nodes.push({
      id: nodeId,
      type: FLOWGRAM_NODE_TYPE,
      data: {
        id: node.id,
        username: node.username,
        display_name: node.display_name,
        title: getNodeTitle(node),
        relationText: getNodeRelationText(node, t),
        inviter_id: node.inviter_id,
        status: node.status,
        isRoot: Boolean(node.isRoot),
        hasMoreChildren: Boolean(node.has_more_children || node.pagination?.hasMore),
        loadedChildrenCount: Array.isArray(node.children) ? node.children.length : 0,
      },
      meta: {
        renderKey: FLOWGRAM_NODE_RENDER_KEY,
        draggable: false,
        selectable: false,
        copyDisable: true,
        deleteDisable: true,
        inputDisable: true,
        outputDisable: true,
        autoResizeDisable: true,
        size: {
          width: FLOWGRAM_NODE_WIDTH,
          height: FLOWGRAM_NODE_HEIGHT,
        },
        position: {
          x: FLOWGRAM_CANVAS_PADDING + depth * FLOWGRAM_LEVEL_GAP,
          y:
            FLOWGRAM_CANVAS_PADDING +
            currentLeafIndex * (FLOWGRAM_NODE_HEIGHT + FLOWGRAM_SIBLING_GAP),
        },
        defaultPorts: FLOWGRAM_DEFAULT_PORTS,
      },
    });

    if (parent) {
      edges.push({
        sourceNodeID: getFlowgramNodeId(parent),
        targetNodeID: nodeId,
        sourcePortID: FLOWGRAM_OUTPUT_PORT_ID,
        targetPortID: FLOWGRAM_INPUT_PORT_ID,
        data: {
          relation: 'inviter',
        },
      });
    }

    return currentLeafIndex;
  };

  walk(treeNode);

  return { nodes, edges };
};

const flowgramNodeRegistries = [
  {
    type: FLOWGRAM_NODE_TYPE,
    meta: {
      renderKey: FLOWGRAM_NODE_RENDER_KEY,
      draggable: false,
      selectable: false,
      copyDisable: true,
      deleteDisable: true,
      inputDisable: true,
      outputDisable: true,
      autoResizeDisable: true,
      size: {
        width: FLOWGRAM_NODE_WIDTH,
        height: FLOWGRAM_NODE_HEIGHT,
      },
      defaultPorts: FLOWGRAM_DEFAULT_PORTS,
    },
  },
];

const normalizeTreeResponse = (data, page, pageSize) => {
  if (Array.isArray(data)) {
    return {
      items: data,
      page,
      pageSize,
      total: null,
      hasMore: data.length >= pageSize,
    };
  }

  const items = Array.isArray(data?.items) ? data.items : [];
  const responsePage = Number(data?.page) > 0 ? Number(data.page) : page;
  const responsePageSize =
    Number(data?.page_size) > 0 ? Number(data.page_size) : pageSize;
  const total = Number(data?.total);
  const hasReliableTotal = Number.isFinite(total) && total > 0;

  return {
    items,
    page: responsePage,
    pageSize: responsePageSize,
    total: hasReliableTotal ? total : null,
    hasMore: hasReliableTotal
      ? responsePage * responsePageSize < total
      : items.length >= responsePageSize,
  };
};

const TreeLoadingPlaceholder = ({ t }) => {
  return (
    <div
      style={{
        minHeight: 260,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 14,
        border: '1px dashed var(--semi-color-border)',
        borderRadius: 18,
        background: 'var(--semi-color-bg-1)',
      }}
    >
      <Spin size='large' />
      <Text type='secondary'>{t('正在加载服务商用户树形结构')}</Text>
    </div>
  );
};

const DependencyTreeNodeBox = ({ node, hasChildren, isExpanded, isLoading, onToggle, t }) => {
  const relationText = getNodeRelationText(node, t);
  const title = getNodeTitle(node);

  return (
    <div
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 8,
        minWidth: node.isRoot ? 220 : 200,
        maxWidth: 360,
        padding: '10px 12px',
        border: node.isRoot
          ? '2px solid var(--semi-color-primary)'
          : '1px solid var(--semi-color-border)',
        borderRadius: 4,
        background: node.isRoot
          ? 'var(--semi-color-primary-light-default)'
          : 'var(--semi-color-bg-0)',
        boxShadow: node.isRoot
          ? '0 0 0 3px var(--semi-color-primary-light-hover)'
          : '0 2px 8px rgba(15, 23, 42, 0.08)',
        color: 'var(--semi-color-text-0)',
      }}
    >
      {hasChildren ? (
        <Button
          icon={
            <IconTreeTriangleDown
              style={{
                transform: isExpanded ? 'rotate(0deg)' : 'rotate(-90deg)',
                transition: 'transform 0.2s ease',
              }}
            />
          }
          loading={isLoading}
          onClick={onToggle}
          size='small'
          theme='borderless'
          type='tertiary'
          style={{ flexShrink: 0 }}
        />
      ) : (
        <span style={{ width: 24, flexShrink: 0 }} />
      )}
      <Avatar size='small' color={node.isRoot ? 'blue' : 'green'}>
        <UserRound size={15} />
      </Avatar>
      <div style={{ minWidth: 0, flex: 1 }}>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            minWidth: 0,
          }}
        >
          <Text strong ellipsis={{ showTooltip: true }} style={{ maxWidth: 150 }}>
            {title}
          </Text>
          {node.username && node.display_name && node.display_name !== node.username && (
            <Text
              type='secondary'
              size='small'
              ellipsis={{ showTooltip: true }}
              style={{ maxWidth: 100 }}
            >
              @{node.username}
            </Text>
          )}
        </div>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 8,
            flexWrap: 'wrap',
            marginTop: 4,
            fontSize: 12,
          }}
        >
          <Text type='secondary' size='small'>ID #{node.id}</Text>
          {relationText && <Text type='secondary' size='small'>{relationText}</Text>}
        </div>
      </div>
      {!node.isRoot && (
        <Tag color={node.status === 1 ? 'green' : 'red'} size='small'>
          {node.status === 1 ? t('启用') : t('禁用')}
        </Tag>
      )}
    </div>
  );
};

const ProviderFlowgramNode = () => {
  const { t } = useTranslation();
  const { data, node } = useNodeRender();
  const relationText = data?.relationText || '';
  const title = data?.title || data?.display_name || data?.username || '-';

  return (
    <WorkflowNodeRenderer
      node={node}
      style={{ cursor: 'default' }}
      portStyle={{ opacity: 0, pointerEvents: 'none' }}
    >
      <div
        className='provider-flowgram-node flow-canvas-not-draggable'
        data-flow-editor-selectable='false'
        style={{
          width: FLOWGRAM_NODE_WIDTH,
          minHeight: FLOWGRAM_NODE_HEIGHT,
          border: data?.isRoot
            ? '2px solid var(--semi-color-primary)'
            : '1px solid var(--semi-color-border)',
          borderRadius: 8,
          background: data?.isRoot
            ? 'var(--semi-color-primary-light-default)'
            : 'var(--semi-color-bg-0)',
          boxShadow: data?.isRoot
            ? '0 0 0 3px var(--semi-color-primary-light-hover), 0 10px 24px rgba(15, 23, 42, 0.12)'
            : '0 8px 18px rgba(15, 23, 42, 0.08)',
          color: 'var(--semi-color-text-0)',
          padding: 12,
          cursor: 'default',
          pointerEvents: 'auto',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: 10 }}>
          <Avatar size='small' color={data?.isRoot ? 'blue' : 'green'}>
            <UserRound size={15} />
          </Avatar>
          <div style={{ minWidth: 0, flex: 1 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <Text strong ellipsis={{ showTooltip: true }} style={{ maxWidth: 148 }}>
                {title}
              </Text>
              {!data?.isRoot && (
                <Tag color={data?.status === 1 ? 'green' : 'red'} size='small'>
                  {data?.status === 1 ? t('启用') : t('禁用')}
                </Tag>
              )}
            </div>
            {data?.username && data?.display_name && data.display_name !== data.username && (
              <Text
                type='secondary'
                size='small'
                ellipsis={{ showTooltip: true }}
                style={{ maxWidth: 190 }}
              >
                @{data.username}
              </Text>
            )}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                flexWrap: 'wrap',
                marginTop: 6,
              }}
            >
              <Text type='secondary' size='small'>
                ID #{data?.id}
              </Text>
              {relationText && (
                <Text type='secondary' size='small'>
                  {relationText}
                </Text>
              )}
            </div>
            {data?.hasMoreChildren && (
              <Text type='tertiary' size='small' style={{ marginTop: 4, display: 'block' }}>
                {t('已加载')} {data.loadedChildrenCount || 0}
              </Text>
            )}
          </div>
        </div>
      </div>
    </WorkflowNodeRenderer>
  );
};

const FlowgramToolbar = () => {
  const { t } = useTranslation();
  const { zoomin, zoomout, fitView, zoom } = usePlaygroundTools({
    minZoom: 0.25,
    maxZoom: 1.6,
  });

  return (
    <Space
      style={{
        position: 'absolute',
        top: 12,
        right: 12,
        zIndex: 30,
        padding: 8,
        border: '1px solid var(--semi-color-border)',
        borderRadius: 10,
        background: 'var(--semi-color-bg-0)',
        boxShadow: '0 8px 20px rgba(15, 23, 42, 0.1)',
      }}
    >
      <Button size='small' icon={<IconMinus />} onClick={() => zoomout(true)} />
      <Text type='secondary' size='small' style={{ minWidth: 42, textAlign: 'center' }}>
        {Math.round(zoom * 100)}%
      </Text>
      <Button size='small' icon={<IconPlus />} onClick={() => zoomin(true)} />
      <Button size='small' theme='outline' onClick={() => fitView(true, 300, 48)}>
        {t('适应画布')}
      </Button>
    </Space>
  );
};

const FlowgramAutoFit = () => {
  const { fitView } = usePlaygroundTools({
    minZoom: 0.25,
    maxZoom: 1.6,
  });

  useEffect(() => {
    const timer = window.setTimeout(() => fitView(true), 120);
    return () => window.clearTimeout(timer);
  }, [fitView]);

  return null;
};

const FlowgramReadonlyEdges = ({ initialData, width, height }) => {
  const nodePositionMap = useMemo(
    () => getFlowgramNodePositionMap(initialData.nodes),
    [initialData.nodes],
  );

  const edgePaths = useMemo(() => {
    return (initialData.edges || [])
      .map((edge) => {
        const sourcePosition = nodePositionMap[edge.sourceNodeID];
        const targetPosition = nodePositionMap[edge.targetNodeID];
        if (!sourcePosition || !targetPosition) {
          return null;
        }
        return {
          id: `${edge.sourceNodeID}-${edge.targetNodeID}`,
          path: getFlowgramEdgePath(sourcePosition, targetPosition),
        };
      })
      .filter(Boolean);
  }, [initialData.edges, nodePositionMap]);

  if (edgePaths.length === 0) {
    return null;
  }

  return (
    <div
      style={{
        position: 'absolute',
        inset: 0,
        zIndex: 8,
        pointerEvents: 'none',
        overflow: 'visible',
      }}
    >
      <svg
        width={width}
        height={height}
        viewBox={`0 0 ${width} ${height}`}
        style={{
          position: 'absolute',
          left: 0,
          top: 0,
          overflow: 'visible',
        }}
      >
        <defs>
          <marker
            id='provider-flowgram-edge-arrow'
            markerWidth='10'
            markerHeight='10'
            refX='9'
            refY='5'
            orient='auto'
            markerUnits='strokeWidth'
          >
            <path d='M 0 0 L 10 5 L 0 10 z' fill={FLOWGRAM_LINE_COLOR} />
          </marker>
        </defs>
        {edgePaths.map((edge) => (
          <path
            key={edge.id}
            d={edge.path}
            fill='none'
            stroke={FLOWGRAM_LINE_COLOR}
            strokeWidth='2'
            strokeLinecap='round'
            strokeLinejoin='round'
            markerEnd='url(#provider-flowgram-edge-arrow)'
            opacity='0.9'
          />
        ))}
      </svg>
    </div>
  );
};

const ProviderUsersFlowgramView = ({ treeData, t }) => {
  const initialData = useMemo(
    () => convertTreeToFlowgramData(treeData, t),
    [treeData, t],
  );
  const flowgramWidth = Math.max(
    760,
    FLOWGRAM_CANVAS_PADDING * 2 + getNodeDepth(treeData) * FLOWGRAM_LEVEL_GAP,
  );
  const flowgramHeight = Math.max(
    420,
    FLOWGRAM_CANVAS_PADDING * 2 +
      getNodeLeafCount(treeData) * (FLOWGRAM_NODE_HEIGHT + FLOWGRAM_SIBLING_GAP),
  );

  return (
    <div
      style={{
        position: 'relative',
        height: 620,
        minHeight: 420,
        border: '1px solid var(--semi-color-border)',
        borderRadius: 8,
        overflow: 'hidden',
        background: 'var(--semi-color-bg-1)',
      }}
    >
      <FreeLayoutEditor
        key={`provider-flowgram-${treeData?.id || 'empty'}-${countNodes(treeData)}`}
        initialData={initialData}
        readonly
        enableReadonlyNodeDragging={false}
        nodeRegistries={flowgramNodeRegistries}
        materials={{
          renderDefaultNode: ProviderFlowgramNode,
          renderNodes: {
            [FLOWGRAM_NODE_RENDER_KEY]: ProviderFlowgramNode,
          },
        }}
        history={{ enable: false, disableShortcuts: true }}
        selectBox={{
          enable: false,
          canSelect: () => false,
        }}
        scroll={{
          enableScrollLimit: false,
          disableScrollBar: false,
          disableScroll: false,
        }}
        playground={{
          width: flowgramWidth,
          height: flowgramHeight,
          autoResize: true,
          zoomEnable: true,
        }}
        background={{
          backgroundColor: 'var(--semi-color-bg-1)',
          dotColor: 'var(--semi-color-border)',
          dotOpacity: 0.35,
        }}
        lineColor={{
          default: FLOWGRAM_LINE_COLOR,
          hovered: FLOWGRAM_LINE_COLOR,
          selected: FLOWGRAM_LINE_COLOR,
          flowing: FLOWGRAM_LINE_COLOR,
        }}
        twoWayConnection={false}
        isDisabledPort={() => true}
        isDisabledLine={() => false}
        canAddLine={() => false}
        canDeleteNode={() => false}
        canDeleteLine={() => false}
        canResetLine={() => false}
        setLineRenderType={() => LineType.LINE_CHART}
      >
        <FlowgramReadonlyEdges
          initialData={initialData}
          width={flowgramWidth}
          height={flowgramHeight}
        />
        <FlowgramAutoFit />
        <FlowgramToolbar />
      </FreeLayoutEditor>
    </div>
  );
};

const ProviderUsersTreeNode = ({
  node,
  expandedKeys,
  loadingNodeIds,
  loadingMoreNodeIds,
  onToggle,
  onLoadMore,
  t,
}) => {
  const isExpanded = Boolean(expandedKeys[node.id]);
  const isLoading = Boolean(loadingNodeIds[node.id]);
  const isLoadingMore = Boolean(loadingMoreNodeIds[node.id]);
  const hasChildren =
    Boolean(node?.has_more_children) ||
    (Array.isArray(node?.children) && node.children.length > 0);
  const hasMorePage = Boolean(node?.pagination?.hasMore);
  const children = Array.isArray(node.children) ? node.children : [];

  return (
    <div
      style={{
        position: 'relative',
        paddingLeft: node.isRoot ? 0 : 34,
        paddingTop: node.isRoot ? 0 : 14,
      }}
    >
      {!node.isRoot && (
        <>
          <span
            style={{
              position: 'absolute',
              left: 13,
              top: 0,
              height: 35,
              borderLeft: '1px dashed var(--semi-color-success)',
            }}
          />
          <span
            style={{
              position: 'absolute',
              left: 13,
              top: 35,
              width: 20,
              borderTop: '1px dashed var(--semi-color-success)',
            }}
          />
        </>
      )}

      <DependencyTreeNodeBox
        node={node}
        hasChildren={hasChildren}
        isExpanded={isExpanded}
        isLoading={isLoading}
        onToggle={() => onToggle(node)}
        t={t}
      />

      {isExpanded && (
        <div
          style={{
            position: 'relative',
            marginLeft: node.isRoot ? 110 : 30,
            paddingLeft: 0,
          }}
        >
          {(isLoading || children.length > 0 || hasMorePage) && (
            <span
              style={{
                position: 'absolute',
                left: -1,
                top: 0,
                bottom: hasMorePage ? 42 : 18,
                borderLeft: '1px dashed var(--semi-color-success)',
              }}
            />
          )}
          {isLoading ? (
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                padding: '14px 0 6px',
                color: 'var(--semi-color-text-2)',
              }}
            >
              <Spin size='small' />
              <Text type='secondary'>{t('正在加载下级用户')}</Text>
            </div>
          ) : (
            children.map((child) => (
              <ProviderUsersTreeNode
                key={child.id}
                node={child}
                expandedKeys={expandedKeys}
                loadingNodeIds={loadingNodeIds}
                loadingMoreNodeIds={loadingMoreNodeIds}
                onToggle={onToggle}
                onLoadMore={onLoadMore}
                t={t}
              />
            ))
          )}
          {!isLoading && hasMorePage && (
            <div
              style={{
                position: 'relative',
                padding: '14px 0 0 34px',
              }}
            >
              <span
                style={{
                  position: 'absolute',
                  left: 0,
                  top: 31,
                  width: 24,
                  borderTop: '1px dashed var(--semi-color-success)',
                }}
              />
              <Button
                size='small'
                theme='outline'
                loading={isLoadingMore}
                onClick={() => onLoadMore(node)}
              >
                {isLoadingMore ? t('加载中...') : t('加载更多')}
              </Button>
              {node.pagination?.total ? (
                <Text type='secondary' size='small' style={{ marginLeft: 10 }}>
                  {t('已加载')} {node.children?.length || 0} / {node.pagination.total}
                </Text>
              ) : null}
            </div>
          )}
        </div>
      )}
    </div>
  );
};

const ProviderUsersTreeModal = ({ visible, handleClose, rootUser }) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [initialLoaded, setInitialLoaded] = useState(false);
  const [treeData, setTreeData] = useState(null);
  const [viewMode, setViewMode] = useState('simple');
  const [expandedKeys, setExpandedKeys] = useState({});
  const [loadingNodeIds, setLoadingNodeIds] = useState({});
  const [loadingMoreNodeIds, setLoadingMoreNodeIds] = useState({});
  const [errorMessage, setErrorMessage] = useState('');

  const rootUserId = rootUser?.id || getUserIdFromLocalStorage() || 0;

  const resetTreeState = () => {
    setLoading(false);
    setInitialLoaded(false);
    setTreeData(null);
    setViewMode('simple');
    setExpandedKeys({});
    setLoadingNodeIds({});
    setLoadingMoreNodeIds({});
    setErrorMessage('');
  };

  const buildRootNode = (treeResponse) => {
    const rootChildren = buildRootChildren(treeResponse.items, rootUserId);
    return {
      id: rootUserId,
      username: rootUser?.username || '',
      display_name: rootUser?.display_name || rootUser?.username || t('当前服务商'),
      group: rootUser?.group || '',
      role: rootUser?.role || 0,
      status: rootUser?.status || 1,
      inviter_id: 0,
      isRoot: true,
      isLoaded: true,
      has_more_children: rootChildren.length > 0 || treeResponse.hasMore,
      children: rootChildren,
      pagination: {
        page: treeResponse.page,
        pageSize: treeResponse.pageSize,
        total: treeResponse.total,
        hasMore: treeResponse.hasMore,
      },
    };
  };

  const fetchTreeUsers = async (
    parentId,
    page = 1,
    pageSize = DEFAULT_TREE_PAGE_SIZE,
  ) => {
    const res = await API.get('/api/provider/tree/users', {
      params: {
        parentId,
        parent_id: parentId,
        p: page,
        page_size: pageSize,
      },
      disableDuplicate: true,
    });
    const { success, message, data } = res.data;
    if (!success) {
      throw new Error(message || t('加载树形结构失败'));
    }
    return normalizeTreeResponse(data, page, pageSize);
  };

  const loadChildrenForNode = async (node, page = 1) => {
    const pageSize = node?.pagination?.pageSize || DEFAULT_TREE_PAGE_SIZE;
    const treeResponse = await fetchTreeUsers(node.id, page, pageSize);
    const children = buildChildNodes(treeResponse.items);
    return {
      ...node,
      isLoaded: true,
      has_more_children: children.length > 0,
      children,
      pagination: {
        page: treeResponse.page,
        pageSize: treeResponse.pageSize,
        total: treeResponse.total,
        hasMore: treeResponse.hasMore,
      },
    };
  };

  const loadRootTree = async () => {
    if (!rootUserId) {
      setInitialLoaded(true);
      setErrorMessage(t('无法获取当前服务商账号，请重新登录后再试'));
      setTreeData(null);
      return;
    }
    setLoading(true);
    setErrorMessage('');
    try {
      const treeResponse = await fetchTreeUsers(rootUserId, 1);
      const rootNode = buildRootNode(treeResponse);
      setTreeData(rootNode);
      setExpandedKeys({ [rootUserId]: true });
    } catch (error) {
      const message = error.message || t('加载树形结构失败');
      setErrorMessage(message);
      showError(message);
      setTreeData(null);
    } finally {
      setInitialLoaded(true);
      setLoading(false);
    }
  };

  const loadChildNodes = async (parentNode) => {
    if (!parentNode?.id || loadingNodeIds[parentNode.id]) {
      return;
    }
    setLoadingNodeIds((prev) => ({ ...prev, [parentNode.id]: true }));
    try {
      const loadedParent = await loadChildrenForNode(parentNode, 1);
      setTreeData((prev) =>
        updateTreeNode(prev, parentNode.id, (current) => ({
          ...current,
          isLoaded: loadedParent.isLoaded,
          has_more_children: loadedParent.has_more_children,
          children: loadedParent.children,
          pagination: loadedParent.pagination,
        })),
      );
    } catch (error) {
      showError(error.message || t('加载树形结构失败'));
    } finally {
      setLoadingNodeIds((prev) => {
        const next = { ...prev };
        delete next[parentNode.id];
        return next;
      });
    }
  };

  const loadMoreChildren = async (parentNode) => {
    if (!parentNode?.id || loadingMoreNodeIds[parentNode.id]) {
      return;
    }
    const nextPage = (parentNode.pagination?.page || 1) + 1;
    setLoadingMoreNodeIds((prev) => ({ ...prev, [parentNode.id]: true }));
    try {
      const loadedParent = await loadChildrenForNode(parentNode, nextPage);
      setTreeData((prev) =>
        updateTreeNode(prev, parentNode.id, (current) => ({
          ...current,
          isLoaded: true,
          has_more_children:
            loadedParent.pagination?.hasMore || loadedParent.children.length > 0,
          children: mergeChildrenById(current.children, loadedParent.children),
          pagination: loadedParent.pagination,
        })),
      );
    } catch (error) {
      showError(error.message || t('加载树形结构失败'));
    } finally {
      setLoadingMoreNodeIds((prev) => {
        const next = { ...prev };
        delete next[parentNode.id];
        return next;
      });
    }
  };

  const handleToggleNode = async (node) => {
    if (!node?.id) {
      return;
    }
    const nextExpanded = !expandedKeys[node.id];
    setExpandedKeys((prev) => ({ ...prev, [node.id]: nextExpanded }));

    if (
      nextExpanded &&
      !node.isRoot &&
      node.has_more_children &&
      !node.isLoaded &&
      !loadingNodeIds[node.id]
    ) {
      await loadChildNodes(node);
    }
  };

  useEffect(() => {
    if (!visible) {
      resetTreeState();
      return;
    }
    loadRootTree();
  }, [visible, rootUserId]);

  const loadedNodeCount = useMemo(() => countNodes(treeData), [treeData]);
  const userNodeCount = Math.max(loadedNodeCount - (treeData ? 1 : 0), 0);
  const hasUserNodes = userNodeCount > 0;

  return (
    <SideSheet
      placement='right'
      title={
        <Space>
          <Tag color='blue' shape='circle'>
            {t('组织架构')}
          </Tag>
          <Title heading={4} className='m-0'>
            {t('服务商用户树形结构')}
          </Title>
        </Space>
      }
      bodyStyle={{ padding: 0 }}
      visible={visible}
      width={isMobile ? '100%' : 880}
      closeIcon={null}
      onCancel={handleClose}
      footer={
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            gap: 12,
            background: 'var(--semi-color-bg-0)',
          }}
        >
          <Text type='secondary'>
            {t('已加载节点')}：{userNodeCount}
          </Text>
          <Space>
            {treeData && (
              <RadioGroup
                type='button'
                buttonSize='middle'
                value={viewMode}
                onChange={(e) => setViewMode(e.target.value)}
              >
                <Radio value='simple'>{t('简略模式')}</Radio>
                <Radio value='flowgram'>{t('FlowGram视图')}</Radio>
              </RadioGroup>
            )}
            <Button
              icon={<IconRefresh />}
              onClick={loadRootTree}
              loading={loading}
            >
              {t('刷新')}
            </Button>
            <Button icon={<IconClose />} onClick={handleClose}>
              {t('关闭')}
            </Button>
          </Space>
        </div>
      }
    >
      <div style={{ padding: 20 }}>
        <div
          style={{
            marginBottom: 16,
            border: '1px solid var(--semi-color-border)',
            borderRadius: 14,
            padding: 14,
            background: 'var(--semi-color-fill-0)',
          }}
        >
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: isMobile ? 'flex-start' : 'center',
              gap: 12,
              flexDirection: isMobile ? 'column' : 'row',
            }}
          >
            <Text>
              {viewMode === 'flowgram'
                ? t('FlowGram视图为只读画布，可拖动画布、缩放查看已加载的用户关系。')
                : t('点击节点左侧箭头可展开下级用户，查看服务商与用户之间的邀请关系。')}
            </Text>
            {treeData && (
              <Button
                icon={<GitBranch size={15} />}
                theme='outline'
                onClick={() =>
                  setViewMode((current) => (current === 'flowgram' ? 'simple' : 'flowgram'))
                }
              >
                {viewMode === 'flowgram' ? t('返回简略模式') : t('使用FlowGram渲染')}
              </Button>
            )}
          </div>
        </div>

        {errorMessage && (
          <Banner
            type='warning'
            description={errorMessage}
            closeIcon={null}
            className='!rounded-lg'
            style={{ marginBottom: 16 }}
          />
        )}

        {!initialLoaded && loading ? (
          <TreeLoadingPlaceholder t={t} />
        ) : !treeData ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            title={t('暂无树形结构数据')}
            description={t('当前服务商下暂无可展示的用户关系')}
          />
        ) : (
          <Spin spinning={loading}>
            <div
              style={{
                minHeight: 240,
                border: '1px solid var(--semi-color-border)',
                borderRadius: 8,
                padding: 24,
                background: 'var(--semi-color-fill-0)',
                overflowX: 'auto',
              }}
            >
              {!hasUserNodes && (
                <div
                  style={{
                    marginBottom: 14,
                    border: '1px dashed var(--semi-color-border)',
                    borderRadius: 14,
                    padding: 14,
                    background: 'var(--semi-color-fill-0)',
                  }}
                >
                  <Text type='secondary'>
                    {t('当前服务商下暂无用户关系，仅展示服务商主账号。')}
                  </Text>
                </div>
              )}
              {viewMode === 'flowgram' ? (
                <ProviderUsersFlowgramView treeData={treeData} t={t} />
              ) : (
                <ProviderUsersTreeNode
                  node={treeData}
                  expandedKeys={expandedKeys}
                  loadingNodeIds={loadingNodeIds}
                  loadingMoreNodeIds={loadingMoreNodeIds}
                  onToggle={handleToggleNode}
                  onLoadMore={loadMoreChildren}
                  t={t}
                />
              )}
            </div>
          </Spin>
        )}
      </div>
    </SideSheet>
  );
};

export default ProviderUsersTreeModal;

