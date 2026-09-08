/** Thin client for /admin/api. Same-origin, cookie session. */
import { reactive } from 'vue';

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`/admin/api${path}`, {
    method,
    headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    credentials: 'same-origin',
  });
  const text = await res.text();
  let data: unknown = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = { error: text };
  }
  if (!res.ok) {
    const msg = (data as { error?: string } | null)?.error ?? `${res.status} ${res.statusText}`;
    if (res.status === 401) session.user = null;
    throw new ApiError(res.status, msg);
  }
  return data as T;
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body ?? {}),
  del: <T>(path: string) => request<T>('DELETE', path),
};

export interface Me {
  user: string;
  storage: string;
  assetBase: string;
  prefixes: string[];
}

export const session = reactive({
  user: null as string | null,
  info: null as Me | null,
  checked: false,
  async ensure(): Promise<boolean> {
    if (this.user) return true;
    try {
      const me = await api.get<Me>('/me');
      this.user = me.user;
      this.info = me;
      return true;
    } catch {
      this.user = null;
      return false;
    } finally {
      this.checked = true;
    }
  },
  async login(username: string, password: string) {
    await api.post('/login', { username, password });
    this.user = username;
    this.info = await api.get<Me>('/me');
  },
  async logout() {
    try {
      await api.post('/logout');
    } finally {
      this.user = null;
      this.info = null;
    }
  },
});

export interface Asset {
  id: number;
  key: string;
  assetPath: string;
  publicUrl: string;
  prefix: string;
  originalName: string;
  size: number;
  contentType: string;
  sha256: string;
  createdAt: string;
}

export interface PresignResponse {
  key: string;
  assetPath: string;
  publicUrl: string;
  upload: { url: string; method: string; headers: Record<string, string>; expiresAt: string };
}

export interface BuildJob {
  id: number;
  status: 'queued' | 'running' | 'succeeded' | 'failed';
  trigger: string;
  createdAt: string;
  startedAt: string | null;
  finishedAt: string | null;
  exitCode: number | null;
}

export async function sha256Hex(file: File): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', await file.arrayBuffer());
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, '0')).join('');
}

/** Upload one file: presign -> PUT with progress -> complete. */
export async function uploadFile(
  file: File,
  prefix: string,
  onProgress: (fraction: number) => void,
  demo?: { name: string; version: string; relativePath: string },
): Promise<Asset> {
  const contentType = file.type || guessType(file.name);
  const pre = await api.post<PresignResponse>('/assets/presign', {
    prefix,
    filename: file.name,
    size: file.size,
    contentType,
    ...(demo ? { demoName: demo.name, demoVersion: demo.version, relativePath: demo.relativePath } : {}),
  });
  const [hash] = await Promise.all([
    sha256Hex(file),
    new Promise<void>((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open(pre.upload.method, pre.upload.url);
      for (const [k, v] of Object.entries(pre.upload.headers)) xhr.setRequestHeader(k, v);
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) onProgress(e.loaded / e.total);
      };
      xhr.onload = () => (xhr.status >= 200 && xhr.status < 300 ? resolve() : reject(new Error(`upload failed: ${xhr.status} ${xhr.responseText}`)));
      xhr.onerror = () => reject(new Error('upload failed: network error'));
      xhr.send(file);
    }),
  ]);
  onProgress(1);
  return api.post<Asset>('/assets/complete', {
    key: pre.key,
    originalName: file.name,
    size: file.size,
    contentType,
    sha256: hash,
  });
}

function guessType(name: string): string {
  const ext = name.toLowerCase().split('.').pop() ?? '';
  const map: Record<string, string> = {
    webp: 'image/webp', png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', gif: 'image/gif', svg: 'image/svg+xml', avif: 'image/avif',
    mp4: 'video/mp4', webm: 'video/webm', woff: 'font/woff', woff2: 'font/woff2', ttf: 'font/ttf', otf: 'font/otf',
    glb: 'model/gltf-binary', gltf: 'model/gltf+json', bin: 'application/octet-stream', wasm: 'application/wasm',
    js: 'text/javascript', css: 'text/css', html: 'text/html', json: 'application/json', data: 'application/octet-stream',
    br: 'application/octet-stream', zip: 'application/zip', pdf: 'application/pdf',
  };
  return map[ext] ?? 'application/octet-stream';
}

export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(2)} MB`;
}
