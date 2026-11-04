// Runtime configuration. The browser calls the API through the same origin
// (/api/v1, Caddy); the SSR server calls it on the internal network.
export const publicApiBase = (): string =>
  (import.meta.env.PUBLIC_API_BASE_URL as string | undefined) ?? '/api/v1';

export const internalApiBase = (): string =>
  process.env.API_INTERNAL_URL ? `${process.env.API_INTERNAL_URL}/api/v1` : publicApiBase();
