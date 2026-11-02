/// <reference path="../.astro/types.d.ts" />

interface ImportMetaEnv {
  /** Public API base seen by the browser, e.g. https://example.be/api/v1 */
  readonly PUBLIC_API_BASE_URL: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
