import { defineMiddleware } from 'astro:middleware';

// Private spaces: without a refresh cookie there is no session, so the user
// is sent to the login page before any island renders (UX only, §6.2; the
// API remains the only authority).
const privatePrefixes = ['/gerant/dashboard', '/pro/dashboard', '/client/espace', '/admin', '/compte/mot-de-passe'];
const publicExceptions = new Set(['/admin/login']);
// `__Host-rt` in production; `rt` when COOKIE_SECURE=false (local http only).
const refreshCookies = ['__Host-rt', 'rt'];

export function isPrivate(path: string): boolean {
  if (publicExceptions.has(path)) return false;
  return privatePrefixes.some((p) => path === p || path.startsWith(`${p}/`));
}

export function loginFor(path: string): string {
  if (path.startsWith('/gerant')) return '/gerant/login';
  if (path.startsWith('/pro')) return '/pro/login';
  if (path.startsWith('/admin')) return '/admin/login';
  return '/client/login';
}

export const onRequest = defineMiddleware(async (ctx, next) => {
  const path = ctx.url.pathname;
  if (isPrivate(path) && !refreshCookies.some((c) => ctx.cookies.has(c))) {
    return ctx.redirect(`${loginFor(path)}?next=${encodeURIComponent(path)}`);
  }
  const res = await next();
  res.headers.set('X-Content-Type-Options', 'nosniff');
  res.headers.set('X-Frame-Options', 'DENY');
  res.headers.set('Referrer-Policy', res.headers.get('Referrer-Policy') ?? 'strict-origin-when-cross-origin');
  res.headers.set('Permissions-Policy', 'camera=(), microphone=(), geolocation=(), payment=()');
  res.headers.set('Cross-Origin-Opener-Policy', 'same-origin');
  if (isPrivate(path)) res.headers.set('Cache-Control', 'no-store');
  return res;
});
