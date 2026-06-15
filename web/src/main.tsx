import React from 'react';
import ReactDOM from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Link, Route, Routes } from 'react-router-dom';
import { App, ConfigProvider, Layout, Menu } from 'antd';
import 'antd/dist/reset.css';
import { RunList } from './pages/RunList';
import { RunDetail } from './pages/RunDetail';
import { ImageDetail } from './pages/ImageDetail';
import { Settings } from './pages/Settings';

const queryClient = new QueryClient();

function Shell() {
  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Layout.Header style={{ display: 'flex', alignItems: 'center' }}>
        <div style={{ color: '#fff', fontWeight: 600, marginRight: 32 }}>SWE CloudBuild</div>
        <Menu
          theme="dark"
          mode="horizontal"
          selectable={false}
          items={[
            { key: 'runs', label: <Link to="/">Runs</Link> },
            { key: 'settings', label: <Link to="/settings">Settings</Link> }
          ]}
        />
      </Layout.Header>
      <Layout.Content style={{ padding: 24 }}>
        <Routes>
          <Route path="/" element={<RunList />} />
          <Route path="/runs/:id" element={<RunDetail />} />
          <Route path="/images/:id" element={<ImageDetail />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </Layout.Content>
    </Layout>
  );
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <ConfigProvider>
        <App>
          <BrowserRouter>
            <Shell />
          </BrowserRouter>
        </App>
      </ConfigProvider>
    </QueryClientProvider>
  </React.StrictMode>
);
