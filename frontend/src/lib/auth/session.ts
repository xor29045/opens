import { atom } from 'nanostores';

// No client accounts anymore (ADR-023).
export type Role = 'ADMIN' | 'MANAGER' | 'PROFESSIONAL';

export interface SessionUser {
  id: string;
  username: string;
  role: Role;
  status: string;
  must_change_password: boolean;
  recovery_codes_confirmed: boolean;
}

export interface SessionState {
  /** Access token, kept in memory only (never in storage, §12.1). */
  token: string | null;
  user: SessionUser | null;
  expiresAt: string | null;
}

const empty: SessionState = { token: null, user: null, expiresAt: null };

/** Shared across islands through nanostores (contexts do not cross islands). */
export const sessionAtom = atom<SessionState>(empty);

export const sessionStore = {
  get: (): SessionState => sessionAtom.get(),
  set: (s: SessionState): void => sessionAtom.set(s),
  clear: (): void => sessionAtom.set(empty),
};

/** Home page for each role. */
export const homeFor: Record<Role, string> = {
  ADMIN: '/admin',
  MANAGER: '/gerant/dashboard',
  PROFESSIONAL: '/pro/dashboard',
};

/** The login page of a role's space. */
export const loginFor: Record<Role, string> = {
  ADMIN: '/admin/login',
  MANAGER: '/gerant/login',
  PROFESSIONAL: '/pro/login',
};
