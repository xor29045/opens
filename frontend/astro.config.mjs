// @ts-check
import { defineConfig } from 'astro/config';
import react from '@astrojs/react';
import node from '@astrojs/node';

// SSR (`output: 'server'`) with the Node adapter (§6.1). Interactive islands
// are React + Chakra v3; private spaces are `client:only` islands.
export default defineConfig({
  output: 'server',
  adapter: node({ mode: 'standalone' }),
  integrations: [react()],
  server: { host: '0.0.0.0', port: 4321 },
  security: {
    checkOrigin: true,
    // Scripts: hashes computed by Astro (no 'unsafe-inline'). Styles: Chakra v3
    // (Emotion) injects <style> elements at runtime, so styles need
    // 'unsafe-inline' — decision recorded for the POC T4.2.
    csp: {
      directives: [
        "default-src 'self'",
        "img-src 'self' data: blob: https:",
        "connect-src 'self'",
        "font-src 'self'",
        "object-src 'none'",
        "base-uri 'none'",
        "form-action 'self'",
      ],
      styleDirective: { resources: ["'self'", "'unsafe-inline'"] },
    },
  },
  devToolbar: { enabled: false },
  // No Markdown content: disable Shiki (its inline styles conflict with the CSP).
  markdown: { syntaxHighlight: false },
  vite: {
    ssr: { noExternal: ['@chakra-ui/react'] },
  },
});
