import { useCallback } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import { Alert, Button, Card, Col, Row, Space, Statistic, Table, Tag, Typography, message } from 'antd';
import { api, ApiError, ImageBuild, eventsUrl } from '../api/client';
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

const LAYERS = ['base', 'env', 'instance'];

export function RunDetail() {
  const { id = '' } = useParams();
  const qc = useQueryClient();

  const { data, isLoading, error } = useQuery({
    queryKey: ['run', id],
    queryFn: () => api.getRun(id),
    enabled: !!id
  });

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
        <Space wrap>
          {PHASES.map((p) => (
            <Tag key={p} color={run.Phase === p ? 'blue' : 'default'}>
              {p}
            </Tag>
          ))}
        </Space>
      </Card>

      <Row gutter={16}>
        {LAYERS.map((layer) => {
          const s = summary?.[layer] ?? {};
          return (
            <Col span={8} key={layer}>
              <Card title={layer} size="small">
                <Space size="large">
                  <Statistic title="total" value={s.total ?? 0} />
                  <Statistic title="success" value={s.success ?? 0} valueStyle={{ color: '#3f8600' }} />
                  <Statistic title="failed" value={s.failed ?? 0} valueStyle={{ color: '#cf1322' }} />
                </Space>
              </Card>
            </Col>
          );
        })}
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
