import { ApiError, type Problem } from './problem';
import { publicApiBase, internalApiBase } from '../config';
import { sessionStore, type SessionUser } from '../auth/session';

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: unknown;
  /** Send the access token and refresh once on 401. Default true. */
  auth?: boolean;
  signal?: AbortSignal;
  timeoutMs?: number;
  /** Multipart upload: the body is sent as is. */
  formData?: FormData;
}

const DEFAULT_TIMEOUT_MS = 15_000;

function baseUrl(): string {
  return typeof window === 'undefined' ? internalApiBase() : publicApiBase();
}

function withTimeout(signal: AbortSignal | undefined, ms: number): { signal: AbortSignal; done: () => void } {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), ms);
  const onAbort = () => controller.abort();
  signal?.addEventListener('abort', onAbort);
  return {
    signal: controller.signal,
    done: () => {
      clearTimeout(timer);
      signal?.removeEventListener('abort', onAbort);
    },
  };
}

// One refresh at a time: concurrent 401s share the same request (§6.2).
let refreshing: Promise<SessionUser | null> | null = null;

/**
 * Exchanges the HttpOnly refresh cookie for a new access token. Resolves to
 * the user, or null when the session is gone.
 */
export function refreshSession(): Promise<SessionUser | null> {
  if (!refreshing) {
    refreshing = (async () => {
      try {
        const res = await fetch(`${baseUrl()}/auth/refresh`, {
          method: 'POST',
          credentials: 'same-origin',
          headers: { 'X-Requested-With': 'XMLHttpRequest' },
        });
        if (!res.ok) {
          sessionStore.clear();
          return null;
        }
        const data = (await res.json()) as { access_token: string; user: SessionUser; expires_at: string };
        sessionStore.set({ token: data.access_token, user: data.user, expiresAt: data.expires_at });
        return data.user;
      } catch {
        return null;
      } finally {
        refreshing = null;
      }
    })();
  }
  return refreshing;
}

async function parseError(res: Response): Promise<ApiError> {
  let problem: Problem = { type: 'about:blank', title: res.statusText, status: res.status };
  try {
    problem = { ...problem, ...((await res.json()) as Partial<Problem>) };
  } catch {
    // Non-JSON error bodies keep the generic problem.
  }
  const retry = Number(res.headers.get('Retry-After'));
  return new ApiError({ ...problem, status: res.status }, Number.isFinite(retry) && retry > 0 ? retry : undefined);
}

/**
 * Calls the API. Errors are thrown as ApiError. A 401 on an authenticated call
 * triggers one refresh and one replay; a failed refresh clears the session.
 */
export async function api<T = unknown>(path: string, opts: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, auth = true, signal, timeoutMs = DEFAULT_TIMEOUT_MS, formData } = opts;
  const attempt = async (retry: boolean): Promise<T> => {
    const headers: Record<string, string> = {};
    if (body !== undefined && !formData) headers['Content-Type'] = 'application/json';
    const token = sessionStore.get().token;
    if (auth && token) headers.Authorization = `Bearer ${token}`;
    const t = withTimeout(signal, timeoutMs);
    let res: Response;
    try {
      res = await fetch(`${baseUrl()}${path}`, {
        method,
        headers,
        credentials: 'same-origin',
        body: formData ?? (body !== undefined ? JSON.stringify(body) : undefined),
        signal: t.signal,
      });
    } catch (err) {
      if (signal?.aborted) throw err;
      throw new ApiError({ type: 'about:blank', title: 'network', status: 0, code: 'network_error' });
    } finally {
      t.done();
    }
    if (res.status === 401 && auth && retry) {
      const user = await refreshSession();
      if (user) return attempt(false);
      throw await parseError(res);
    }
    if (!res.ok) throw await parseError(res);
    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  };
  return attempt(true);
}

/** Downloads a binary resource (invoice PDF) with the access token. */
export async function apiBlob(path: string): Promise<Blob> {
  const token = sessionStore.get().token;
  const res = await fetch(`${baseUrl()}${path}`, {
    credentials: 'same-origin',
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!res.ok) throw await parseError(res);
  return res.blob();
}
