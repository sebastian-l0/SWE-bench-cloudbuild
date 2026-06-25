import { useQuery, useMutation } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import { Alert, Button, Card, Descriptions, Space, Tag, Typography } from 'antd';
import { api, ApiError, cpRecordUrl } from '../api/client';
import { statusColor } from './RunList';

export function ImageDetail() {
  const { id = '' } = useParams();

  const { data, isLoading, error } = useQuery({
    queryKey: ['image', id],
    queryFn: () => api.getImage(id),
    enabled: !!id
  });

  const { data: config } = useQuery({ queryKey: ['config'], queryFn: api.getConfig });

  const logMut = useMutation({
    mutationFn: () => api.getImageLog(id)
  });

  if (isLoading) return <Card loading />;
  if (error || !data) return <Alert type="error" message="Failed to load image" />;

  const consoleUrl = cpRecordUrl(config?.tos?.Region ?? '', data);

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      <Card
        title={`Image: ${data.LocalKey}`}
        extra={
          <Space>
            {consoleUrl && (
              <a href={consoleUrl} target="_blank" rel="noreferrer">
                Open in CP Console
              </a>
            )}
            <Link to={`/runs/${data.RunID}`}>Back to run</Link>
          </Space>
        }
      >
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label="Status">
            <Tag color={statusColor(data.Status)}>{data.Status}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="Layer">{data.Layer}</Descriptions.Item>
          <Descriptions.Item label="Target Image">{data.TargetImage}</Descriptions.Item>
          <Descriptions.Item label="Depends On">{data.DependsOnKey || '-'}</Descriptions.Item>
          <Descriptions.Item label="Attempts">{data.Attempts}</Descriptions.Item>
          <Descriptions.Item label="Workspace ID">{data.WorkspaceID || '-'}</Descriptions.Item>
          <Descriptions.Item label="Pipeline ID">{data.PipelineID || '-'}</Descriptions.Item>
          <Descriptions.Item label="Last Run ID">{data.LastRunID || '-'}</Descriptions.Item>
          {data.Error && <Descriptions.Item label="Error">{data.Error}</Descriptions.Item>}
        </Descriptions>
      </Card>

      <Card
        title="Logs"
        extra={
          <Button onClick={() => logMut.mutate()} loading={logMut.isPending}>
            Fetch Logs
          </Button>
        }
      >
        {logMut.isError && (
          <Alert
            type="error"
            message={logMut.error instanceof ApiError ? logMut.error.message : 'log fetch failed'}
          />
        )}
        <Typography.Paragraph>
          <pre style={{ whiteSpace: 'pre-wrap', margin: 0 }}>{logMut.data?.log ?? ''}</pre>
        </Typography.Paragraph>
      </Card>
    </Space>
  );
}
