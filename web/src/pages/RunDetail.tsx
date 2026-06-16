import { useCallback } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import { Alert, Button, Card, Col, Descriptions, Progress, Row, Space, Table, Tag, Typography, message } from 'antd';
import { api, ApiError, ImageBuild, cpRecordUrl, cpWorkspaceUrl, eventsUrl } from '../api/client';
import { useSSE } from '../hooks/useSSE';
import { statusColor } from './RunList';

const PHASES = [
  'materializing_dockerfiles',
  'uploading_dockerfiles',
  'preparing_cp_resources',
  'building_base_images',
  'building_env_images',
  'building_instance_images'
];

const PHASE_LABELS: Record<string, string> = {
  materializing_dockerfiles: 'Materialize',
  uploading_dockerfiles: 'Upload',
  preparing_cp_resources: 'Prepare CP',
  building_base_images: 'Base',
  building_env_images: 'Env',
  building_instance_images: 'Instance'
};

type NodeState = 'done' | 'active' | 'error' | 'pending';

const NODE_STYLE: Record<NodeState, { border: string; bg: string; color: string; label: string }> = {
  done: { border: '#52c41a', bg: '#f6ffed', color: '#389e0d', label: 'done' },
  active: { border: '#1677ff', bg: '#e6f4ff', color: '#0958d9', label: 'running' },
  error: { border: '#ff4d4f', bg: '#fff2f0', color: '#cf1322', label: 'failed' },
  pending: { border: '#d9d9d9', bg: '#fafafa', color: '#8c8c8c', label: 'pending' }
};

// phaseState derives each node's state from the run's current phase and status.
function phaseState(index: number, currentIndex: number, runStatus: string): NodeState {
  if (runStatus === 'success') return 'done';
  if (index < currentIndex) return 'done';
  if (index === currentIndex) {
    if (runStatus === 'failed') return 'error';
    if (runStatus === 'canceled') return 'pending';
    return 'active';
  }
  return 'pending';
}

function PhaseFlow({ phase, status }: { phase: string; status: string }) {
  const currentIndex = PHASES.indexOf(phase);
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 4 }}>
      {PHASES.map((p, i) => {
        const st = phaseState(i, currentIndex, status);
        const s = NODE_STYLE[st];
        return (
          <div key={p} style={{ display: 'flex', alignItems: 'center' }}>
            <div
              style={{
                minWidth: 92,
                padding: '8px 12px',
                border: `1px solid ${s.border}`,
                background: s.bg,
                borderRadius: 8,
                textAlign: 'center',
                boxShadow: st === 'active' ? `0 0 0 3px ${s.bg}` : undefined
              }}
            >
              <div style={{ fontWeight: 600, fontSize: 13, color: s.color }}>{PHASE_LABELS[p]}</div>
              <div style={{ fontSize: 11, color: s.color, opacity: 0.85 }}>{s.label}</div>
            </div>
            {i < PHASES.length - 1 && (
              <span style={{ margin: '0 6px', color: '#bfbfbf', fontSize: 16 }}>→</span>
            )}
          </div>
        );
      })}
    </div>
  );
}

const LAYERS = ['base', 'env', 'instance'];

// LayerStats shows a segmented progress bar plus per-status counts for a layer.
function LayerCard({ layer, summary }: { layer: string; summary: Record<string, number> }) {
  const total = summary.total ?? 0;
  const success = summary.success ?? 0;
  const failed = summary.failed ?? 0;
  const skipped = summary.skipped ?? 0;
  const running = (summary.running ?? 0) + (summary.queued ?? 0);
  const pending = Math.max(total - success - failed - skipped - running, 0);

  const pct = (n: number) => (total > 0 ? (n / total) * 100 : 0);

  const segments: { key: string; count: number; color: string; label: string }[] = [
    { key: 'success', count: success, color: '#52c41a', label: 'success' },
    { key: 'running', count: running, color: '#1677ff', label: 'running' },
    { key: 'failed', count: failed, color: '#ff4d4f', label: 'failed' },
    { key: 'skipped', count: skipped, color: '#faad14', label: 'skipped' },
    { key: 'pending', count: pending, color: '#d9d9d9', label: 'pending' }
  ];

  return (
    <Card
      size="small"
      title={
        <Space>
          <span style={{ textTransform: 'capitalize' }}>{layer}</span>
          <Typography.Text type="secondary" style={{ fontWeight: 400 }}>
            {success}/{total} done
          </Typography.Text>
        </Space>
      }
    >
      {/* Stacked segmented bar */}
      <div
        style={{
          display: 'flex',
          height: 10,
          borderRadius: 5,
          overflow: 'hidden',
          background: '#f0f0f0',
          marginBottom: 12
        }}
      >
        {total === 0
          ? null
          : segments
              .filter((seg) => seg.count > 0)
              .map((seg) => (
                <div
                  key={seg.key}
                  style={{ width: `${pct(seg.count)}%`, background: seg.color }}
                  title={`${seg.label}: ${seg.count}`}
                />
              ))}
      </div>

      {/* Overall success ratio */}
      <Progress
        percent={Math.round(pct(success))}
        size="small"
        status={failed > 0 ? 'exception' : success === total && total > 0 ? 'success' : 'active'}
        style={{ marginBottom: 8 }}
      />

      {/* Per-status count badges */}
      <Space wrap size={4}>
        {segments
          .filter((seg) => seg.count > 0)
          .map((seg) => (
            <Tag key={seg.key} color={seg.color} style={{ marginInlineEnd: 0 }}>
              {seg.label} {seg.count}
            </Tag>
          ))}
        {total === 0 && <Typography.Text type="secondary">no images</Typography.Text>}
      </Space>
    </Card>
  );
}

export function RunDetail() {
  const { id = '' } = useParams();
  const qc = useQueryClient();

  const { data, isLoading, error } = useQuery({
    queryKey: ['run', id],
    queryFn: () => api.getRun(id),
    enabled: !!id
  });

  const { data: config } = useQuery({ queryKey: ['config'], queryFn: api.getConfig });
  const region = config?.tos?.Region ?? '';

  const refresh = useCallback(() => {
    void qc.invalidateQueries({ queryKey: ['run', id] });
  }, [qc, id]);

  // Subscribe to SSE; on any event, refresh the REST detail (REST is the source
  // of truth and also covers backend restarts / missed events).
  useSSE(id ? eventsUrl(id) : undefined, refresh);

  const cancelMut = useMutation({
    mutationFn: () => api.cancelRun(id),
    onSuccess: () => {
      message.success('Run canceled');
      refresh();
    },
    onError: (e: unknown) => message.error(e instanceof ApiError ? e.message : 'cancel failed')
  });

  const retryMut = useMutation({
    mutationFn: (imageID: string) => api.retryImage(imageID),
    onSuccess: () => {
      message.success('Image retried');
      refresh();
    },
    onError: (e: unknown) => message.error(e instanceof ApiError ? e.message : 'retry failed')
  });

  if (isLoading) return <Card loading />;
  if (error || !data) return <Alert type="error" message="Failed to load run" />;

  const { run, images, summary } = data;
  const active = run.Status === 'running' || run.Status === 'pending';

  const failedImages = images.filter((i) => i.Status === 'failed');

  // Workspace is shared across a run's images; take it from the first one.
  const workspaceID = images.find((i) => i.WorkspaceID)?.WorkspaceID ?? '';
  const workspaceUrl = cpWorkspaceUrl(region, workspaceID);
  const tosPath = run.TOSBucket && run.TOSPrefix ? `tos://${run.TOSBucket}/${run.TOSPrefix}` : '';

  const imageColumns = [
    {
      title: 'Image',
      dataIndex: 'LocalKey',
      render: (_: string, img: ImageBuild) => <Link to={`/images/${img.ID}`}>{img.LocalKey}</Link>
    },
    { title: 'Layer', dataIndex: 'Layer' },
    {
      title: 'Status',
      dataIndex: 'Status',
      render: (s: string) => <Tag color={statusColor(s)}>{s}</Tag>
    },
    { title: 'Attempts', dataIndex: 'Attempts' },
    {
      title: 'Pipeline',
      render: (_: unknown, img: ImageBuild) => {
        const url = cpRecordUrl(region, img);
        return url ? (
          <a href={url} target="_blank" rel="noreferrer">
            CP Console
          </a>
        ) : (
          <Typography.Text type="secondary">-</Typography.Text>
        );
      }
    },
    {
      title: 'Actions',
      render: (_: unknown, img: ImageBuild) =>
        img.Status === 'failed' ? (
          <Button size="small" onClick={() => retryMut.mutate(img.ID)}>
            Retry
          </Button>
        ) : null
    }
  ];

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      <Card
        title={`Run: ${run.Name}`}
        extra={
          <Button danger disabled={!active} onClick={() => cancelMut.mutate()}>
            Cancel
          </Button>
        }
      >
        <Space>
          <Tag color={statusColor(run.Status)}>{run.Status}</Tag>
          <Typography.Text type="secondary">phase: {run.Phase}</Typography.Text>
        </Space>
        {run.Error && <Alert style={{ marginTop: 12 }} type="error" message={run.Error} />}
      </Card>

      <Card title="Phase Timeline" size="small">
        <PhaseFlow phase={run.Phase} status={run.Status} />
      </Card>

      <Card title="Artifacts" size="small">
        <Descriptions column={1} size="small">
          <Descriptions.Item label="Materialize (local output)">
            {run.OutputDir ? (
              <Typography.Text code copyable>
                {run.OutputDir}
              </Typography.Text>
            ) : (
              <Typography.Text type="secondary">-</Typography.Text>
            )}
          </Descriptions.Item>
          <Descriptions.Item label="Upload (TOS)">
            {tosPath ? (
              <Typography.Text code copyable>
                {tosPath}
              </Typography.Text>
            ) : (
              <Typography.Text type="secondary">-</Typography.Text>
            )}
          </Descriptions.Item>
          <Descriptions.Item label="Prepare CP (workspace)">
            {workspaceUrl ? (
              <a href={workspaceUrl} target="_blank" rel="noreferrer">
                Open workspace in CP Console
              </a>
            ) : (
              <Typography.Text type="secondary">-</Typography.Text>
            )}
          </Descriptions.Item>
        </Descriptions>
      </Card>

      <Row gutter={16}>
        {LAYERS.map((layer) => (
          <Col span={8} key={layer}>
            <LayerCard layer={layer} summary={summary?.[layer] ?? {}} />
          </Col>
        ))}
      </Row>

      {failedImages.length > 0 && (
        <Alert type="warning" message={`${failedImages.length} failed image(s)`} />
      )}

      <Card title="Images">
        <Table rowKey="ID" dataSource={images} columns={imageColumns} pagination={false} />
      </Card>
    </Space>
  );
}
