import { createSystem, defaultConfig, defineConfig } from '@chakra-ui/react';

// Design system (§6.3): slate neutrals, a single accent (deep rose), no
// gradients, 4 px grid, 6–8 px radii, system fonts only (sans for text, serif
// for display headings). The site is dark-first: <html class="dark">.
// Raw values are mirrored as CSS variables in styles/global.css for the parts
// rendered by Astro without React.
const config = defineConfig({
  globalCss: {
    'html, body': { bg: 'bg', color: 'fg' },
    '*:focus-visible': { outline: '2px solid', outlineColor: 'brand.solid', outlineOffset: '2px' },
  },
  theme: {
    tokens: {
      fonts: {
        body: { value: 'Inter, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif' },
        heading: { value: '"Iowan Old Style", "Palatino Linotype", Palatino, "Book Antiqua", Georgia, serif' },
      },
      radii: {
        l1: { value: '4px' },
        l2: { value: '6px' },
        l3: { value: '8px' },
      },
      colors: {
        brand: {
          50: { value: '#fdf0f4' },
          100: { value: '#fadbe5' },
          200: { value: '#f4b4c8' },
          300: { value: '#ec85a4' },
          400: { value: '#e25b83' },
          500: { value: '#d23f6b' },
          600: { value: '#b52f58' },
          700: { value: '#912648' },
          800: { value: '#6d1f38' },
          900: { value: '#4a1727' },
          950: { value: '#2c0d17' },
        },
        slate: {
          50: { value: '#f4f6f9' },
          100: { value: '#e6eaf0' },
          200: { value: '#c9d0db' },
          300: { value: '#a3adbc' },
          400: { value: '#7c8798' },
          500: { value: '#5b6577' },
          600: { value: '#434c5c' },
          700: { value: '#2e3542' },
          800: { value: '#1d222c' },
          900: { value: '#141820' },
          950: { value: '#0c0f14' },
        },
      },
    },
    semanticTokens: {
      colors: {
        brand: {
          solid: { value: { base: '{colors.brand.600}', _dark: '{colors.brand.500}' } },
          contrast: { value: '#ffffff' },
          fg: { value: { base: '{colors.brand.700}', _dark: '{colors.brand.300}' } },
          muted: { value: { base: '{colors.brand.100}', _dark: '{colors.brand.900}' } },
          subtle: { value: { base: '{colors.brand.50}', _dark: '{colors.brand.950}' } },
          emphasized: { value: { base: '{colors.brand.200}', _dark: '{colors.brand.800}' } },
          focusRing: { value: { base: '{colors.brand.500}', _dark: '{colors.brand.400}' } },
        },
        // Legacy alias kept for components written with `accent.*`.
        accent: {
          solid: { value: '{colors.brand.solid}' },
          contrast: { value: '#ffffff' },
          fg: { value: '{colors.brand.fg}' },
          muted: { value: '{colors.brand.muted}' },
        },
        bg: {
          DEFAULT: { value: { base: '#ffffff', _dark: '{colors.slate.950}' } },
          subtle: { value: { base: '{colors.slate.50}', _dark: '{colors.slate.900}' } },
          muted: { value: { base: '{colors.slate.100}', _dark: '{colors.slate.800}' } },
          emphasized: { value: { base: '{colors.slate.200}', _dark: '{colors.slate.700}' } },
          panel: { value: { base: '#ffffff', _dark: '{colors.slate.900}' } },
        },
        fg: {
          DEFAULT: { value: { base: '{colors.slate.950}', _dark: '#eef1f5' } },
          muted: { value: { base: '{colors.slate.600}', _dark: '{colors.slate.300}' } },
          subtle: { value: { base: '{colors.slate.500}', _dark: '{colors.slate.400}' } },
        },
        border: {
          DEFAULT: { value: { base: '{colors.slate.200}', _dark: '{colors.slate.700}' } },
          muted: { value: { base: '{colors.slate.100}', _dark: '{colors.slate.800}' } },
          subtle: { value: { base: '{colors.slate.50}', _dark: '{colors.slate.800}' } },
        },
        danger: { value: { base: '#b91c1c', _dark: '#fca5a5' } },
        success: { value: { base: '#15803d', _dark: '#86efac' } },
        warning: { value: { base: '#92400e', _dark: '#fcd34d' } },
      },
    },
  },
});

export const system = createSystem(defaultConfig, config);
