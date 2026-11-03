import { useStore } from '@nanostores/react';
import { useEffect, useState } from 'react';
import { sessionAtom, type Role, type SessionUser } from './session';
import { refreshSession } from '../api/client';

export type SessionStatus = 'loading' | 'authenticated' | 'anonymous';

/**
 * Resolves the session of an island: an in-memory token if present, otherwise
 * one refresh with the HttpOnly cookie. Nothing is stored in web storage.
 */
/** Set by the layout: false when the browser has no refresh cookie at all. */
function serverSaysSessionMayExist(): boolean {
  const meta = document.querySelector<HTMLMetaElement>('meta[name="os-session"]');
  return meta?.content !== '0';
}

export function useSession(): { status: SessionStatus; user: SessionUser | null } {
  const state = useStore(sessionAtom);
  const [resolved, setResolved] = useState(state.user !== null);
  useEffect(() => {
    let alive = true;
    if (state.user) {
      setResolved(true);
      return;
    }
    if (!serverSaysSessionMayExist()) {
      setResolved(true);
      return;
    }
    refreshSession().finally(() => alive && setResolved(true));
    return () => {
      alive = false;
    };
    // Runs once per island mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  if (!resolved) return { status: 'loading', user: null };
  if (!state.user) return { status: 'anonymous', user: null };
  return { status: 'authenticated', user: state.user };
}

export type { Role, SessionUser };
