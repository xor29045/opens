import type { ReactNode } from 'react';
import { useEffect } from 'react';
import { Center, Spinner } from '@chakra-ui/react';
import { useSession } from '../lib/auth/useSession';
import { loginFor, type Role } from '../lib/auth/session';

/**
 * Client-side guard (UX only — the API enforces access, §5.1). Redirects to
 * the login page of the space, or to the password change page when required.
 */
export default function Guard({ role, children }: { role: Role; children: ReactNode }) {
  const { status, user } = useSession();
  const mustChange = user?.must_change_password ?? false;
  useEffect(() => {
    if (status === 'anonymous') {
      const next = encodeURIComponent(window.location.pathname);
      window.location.replace(`${loginFor[role]}?next=${next}`);
    } else if (status === 'authenticated' && user && user.role !== role) {
      window.location.replace(loginFor[role]);
    } else if (status === 'authenticated' && mustChange && window.location.pathname !== '/compte/mot-de-passe') {
      window.location.replace('/compte/mot-de-passe');
    }
  }, [status, user, role, mustChange]);

  if (status !== 'authenticated' || !user || user.role !== role || mustChange) {
    return (
      <Center minH="60vh" role="status" aria-live="polite">
        <Spinner size="lg" color="accent.solid" />
      </Center>
    );
  }
  return <>{children}</>;
}
