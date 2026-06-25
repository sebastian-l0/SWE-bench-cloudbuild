import { useQuery } from '@tanstack/react-query';
import { Card, Descriptions, Tag, Typography, Spin, Alert } from 'antd';
import { api } from '../api/client';

export function Settings() {
  const { data, isLoading, error } = useQuery({ queryKey: ['config'], queryFn: api.getConfig });

  if (isLoading) return <Spin />;
  if (error || !data) return <Alert type="error" message="Failed to load config" />;

  const present = (ok: boolean) => (
    <Tag color={ok ? 'green' : 'default'}>{ok ? 'set' : 'missing'}</Tag>
  );

  return (
    <Card title="Settings">
      <Typography.Paragraph type="secondary">
        Effective configuration (secrets are never returned, only presence is shown).
      </Typography.Paragraph>
      <Descriptions bordered column={1} size="small">
        <Descriptions.Item label="Mode">
          {data.mockMode ? <Tag color="blue">mock</Tag> : <Tag color="volcano">live</Tag>}
        </Descriptions.Item>
        <Descriptions.Item label="Volc Target">{data.volcTarget}</Descriptions.Item>
        <Descriptions.Item label="Volc Access Key">{present(data.secrets.volcAccessKey)}</Descriptions.Item>
        <Descriptions.Item label="Volc Secret Key">{present(data.secrets.volcSecretKey)}</Descriptions.Item>
        <Descriptions.Item label="Database URL">{present(data.secrets.databaseUrl)}</Descriptions.Item>
        <Descriptions.Item label="TOS Bucket">{data.tos.Bucket || '-'}</Descriptions.Item>
        <Descriptions.Item label="TOS Parent Path">{data.tos.ParentPath || '-'}</Descriptions.Item>
        <Descriptions.Item label="Dataset">
          {data.dataset.Name} / {data.dataset.Split}
        </Descriptions.Item>
        <Descriptions.Item label="Materializer">
          {data.materializer.RepoURL} @ {data.materializer.Ref}
        </Descriptions.Item>
        <Descriptions.Item label="Registry Namespace">{data.registryNamespace || '-'}</Descriptions.Item>
        <Descriptions.Item label="Concurrency">
          base={data.concurrency.Base} env={data.concurrency.Env} instance={data.concurrency.Instance}
        </Descriptions.Item>
      </Descriptions>
    </Card>
  );
}
