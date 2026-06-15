import React from 'react';
import ReactDOM from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { App, Card, Typography } from 'antd';
import 'antd/dist/reset.css';

const queryClient = new QueryClient();

function Root() {
  return (
    <App>
      <Card title="SWE CloudBuild" style={{ margin: 24 }}>
        <Typography.Paragraph>
          Local demo shell for materializing SWE-bench Dockerfiles, uploading them to TOS,
          and orchestrating Volcengine CP image builds.
        </Typography.Paragraph>
      </Card>
    </App>
  );
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <Root />
    </QueryClientProvider>
  </React.StrictMode>
);
