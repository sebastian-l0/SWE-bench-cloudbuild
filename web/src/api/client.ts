export interface ApiErrorBody {
  error: { code: string; message: string };
}

export class ApiError extends Error {
  code: string;
  status: number;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) }
  });
  if (!response.ok) {
    let code = 'error';
    let message = `request failed: ${response.status}`;
    try {
      const body = (await response.json()) as ApiErrorBody;
      if (body?.error) {
        code = body.error.code;
        message = body.error.message;
      }
    } catch {
      // non-JSON error body; keep defaults
    }
    throw new ApiError(response.status, code, message);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

export interface SecretPresence {
  volcAccessKey: boolean;
  volcSecretKey: boolean;
  databaseUrl: boolean;
}

export interface PublicConfig {
  httpAddr: string;
  volcTarget: string;
  tos: { Bucket: string; ParentPath: string; Region: string };
  dataset: { Name: string; Split: string };
  materializer: { RepoURL: string; Ref: string };
  registryNamespace: string;
  concurrency: { Base: number; Env: number; Instance: number };
  cp: { WorkspacePrefix: string; PipelinePrefix: string };
  mockMode: boolean;
  secrets: SecretPresence;
}

export interface Run {
  ID: string;
  Name: string;
  Status: string;
  Phase: string;
  Dataset: string;
  OutputDir: string;
  TOSBucket: string;
  TOSPrefix: string;
  Registry: string;
  Error: string;
  CreatedAt: string;
}

export interface ImageBuild {
  ID: string;
  RunID: string;
  Layer: string;
  LocalKey: string;
  TargetImage: string;
  Status: string;
  DependsOnKey: string;
  Attempts: number;
  Error: string;
  WorkspaceID: string;
  PipelineID: string;
  LastRunID: string;
}

export type LayerSummary = Record<string, Record<string, number>>;

export interface RunDetail {
  run: Run;
  images: ImageBuild[];
  summary: LayerSummary;
}

export const api = {
  getConfig: () => request<PublicConfig>('/api/config'),
  listRuns: () => request<{ runs: Run[] }>('/api/runs'),
  createRun: (body: { name?: string; outputDir?: string; dataset?: string }) =>
    request<Run>('/api/runs', { method: 'POST', body: JSON.stringify(body) }),
  startRun: (id: string) =>
    request<{ status: string }>(`/api/runs/${id}/start`, { method: 'POST' }),
  cancelRun: (id: string) =>
    request<{ status: string }>(`/api/runs/${id}/cancel`, { method: 'POST' }),
  getRun: (id: string) => request<RunDetail>(`/api/runs/${id}`),
  getImage: (id: string) => request<ImageBuild>(`/api/images/${id}`),
  retryImage: (id: string) =>
    request<{ status: string }>(`/api/images/${id}/retry`, { method: 'POST' }),
  getImageLog: (id: string) => request<{ log: string }>(`/api/images/${id}/log`)
};

export function eventsUrl(runID: string): string {
  return `/api/runs/${runID}/events`;
}

// cpRecordUrl builds a deep link to the CP console for a pipeline. When the
// image has a pipeline run it links to that specific record; otherwise it links
// to the pipeline's record list. Returns null until the pipeline exists.
export function cpRecordUrl(region: string, img: ImageBuild): string | null {
  if (!region || !img.WorkspaceID || !img.PipelineID) {
    return null;
  }
  const base =
    `https://console.volcengine.com/cp/region:cp+${region}` +
    `/v2/workspace/${img.WorkspaceID}/pipeline/${img.PipelineID}/record`;
  return img.LastRunID ? `${base}/${img.LastRunID}` : base;
}

// cpWorkspaceUrl builds a link to the CP workspace console.
export function cpWorkspaceUrl(region: string, workspaceID: string): string | null {
  if (!region || !workspaceID) {
    return null;
  }
  return `https://console.volcengine.com/cp/region:cp+${region}/v2/workspace/${workspaceID}`;
}
