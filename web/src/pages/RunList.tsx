import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { Button, Card, Form, Input, Space, Table, Tag, message } from 'antd';
import { api, ApiError, Run } from '../api/client';

export function statusColor(status: string): string {
  switch (status) {
    case 'success':
      return 'green';
    case 'failed':
      return 'red';
    case 'running':
      return 'blue';
    case 'canceled':
      return 'default';
    case 'skipped':
      return 'orange';
    default:
      return 'default';
  }
}

export function RunList() {
  const qc = useQueryClient();
  const [form] = Form.useForm();
  const [creating, setCreating] = useState(false);

  const { data, isLoading } = useQuery({ queryKey: ['runs'], queryFn: api.listRuns });

  const createMut = useMutation({
    mutationFn: (values: { name?: string; outputDir?: string; dataset?: string }) =>
      api.createRun(values),
    onSuccess: async (run) => {
      message.success(`Created run ${run.Name}`);
      form.resetFields();
      await qc.invalidateQueries({ queryKey: ['runs'] });
    },
    onError: (e: unknown) => message.error(e instanceof ApiError ? e.message : 'create failed')
  });

  const startMut = useMutation({
    mutationFn: (id: string) => api.startRun(id),
    onSuccess: async () => {
      message.success('Run started');
      await qc.invalidateQueries({ queryKey: ['runs'] });
    },
    onError: (e: unknown) => message.error(e instanceof ApiError ? e.message : 'start failed')
  });

  const columns = [
    {
      title: 'Name',
      dataIndex: 'Name',
      render: (_: string, r: Run) => <Link to={`/runs/${r.ID}`}>{r.Name}</Link>
    },
    { title: 'Phase', dataIndex: 'Phase' },
    {
      title: 'Status',
      dataIndex: 'Status',
      render: (s: string) => <Tag color={statusColor(s)}>{s}</Tag>
    },
    { title: 'Dataset', dataIndex: 'Dataset' },
    {
      title: 'Actions',
      render: (_: unknown, r: Run) => (
        <Button
          size="small"
          disabled={r.Status === 'running'}
          loading={startMut.isPending && startMut.variables === r.ID}
          onClick={() => startMut.mutate(r.ID)}
        >
          Start
        </Button>
      )
    }
  ];

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      <Card title="New Run">
        <Form
          form={form}
          layout="inline"
          onFinish={async (values) => {
            setCreating(true);
            await createMut.mutateAsync(values).finally(() => setCreating(false));
          }}
        >
          <Form.Item name="name" label="Name">
            <Input placeholder="optional" />
          </Form.Item>
          <Form.Item name="outputDir" label="Generated Dir">
            <Input placeholder="optional (test/diagnostic)" style={{ width: 260 }} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={creating}>
              Create
            </Button>
          </Form.Item>
        </Form>
      </Card>

      <Card title="Runs">
        <Table
          rowKey="ID"
          loading={isLoading}
          dataSource={data?.runs ?? []}
          columns={columns}
          pagination={false}
        />
      </Card>
    </Space>
  );
}
