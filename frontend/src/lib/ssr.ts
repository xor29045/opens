import { internalApiBase } from './config';

/**
 * Server-side GET on the public API with a short timeout. Returns null on any
 * failure so that pages degrade gracefully instead of erroring.
 */
export async function ssrGet<T>(path: string, timeoutMs = 5_000): Promise<T | null> {
  try {
    const res = await fetch(`${internalApiBase()}${path}`, { signal: AbortSignal.timeout(timeoutMs) });
    if (!res.ok) return null;
    return (await res.json()) as T;
  } catch {
    return null;
  }
}
