import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Button, Code, Field, HStack, Input, SimpleGrid, Stack, Text } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import { Section } from '../../components/ui';
import { QueryBlock, DataTable, type Column } from '../dashboard/shared';
import { api } from '../../lib/api/client';
import { describeError } from '../../lib/api/problem';
import { dateTimeFr } from '../../lib/format';
import { sessionStore } from '../../lib/auth/session';
import type { components } from '../../lib/api/schema';

type Session = components['schemas']['Session'];
type RecoveryStatus = components['schemas']['RecoveryStatus'];

/** Sessions and recovery codes, shared by every non-admin space. */
function SecurityInner({ withCodes }: { withCodes: boolean }) {
  const qc = useQueryClient();
  const sessions = useQuery({ queryKey: ['auth', 'sessions'], queryFn: ({ signal }) => api<{ items: Session[] }>('/auth/sessions', { signal }) });
  const codes = useQuery({ queryKey: ['auth', 'codes'], enabled: withCodes, queryFn: ({ signal }) => api<RecoveryStatus>('/auth/recovery-codes/status', { signal }) });
  const [password, setPassword] = useState('');
  const [newCodes, setNewCodes] = useState<string[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const revoke = async (s: Session) => {
    setError(null);
    try {
      await api(`/auth/sessions/${s.id}`, { method: 'DELETE' });
      if (s.current) {
        sessionStore.clear();
        window.location.replace('/');
        return;
      }
      qc.invalidateQueries({ queryKey: ['auth', 'sessions'] });
    } catch (e) {
      setError(describeError(e));
    }
  };
  const logoutAll = async () => {
    try {
      await api('/auth/logout-all', { method: 'POST' });
    } finally {
      sessionStore.clear();
      window.location.replace('/');
    }
  };
  const regenerate = async () => {
    setError(null);
    try {
      const r = await api<{ recovery_codes: string[] }>('/auth/recovery-codes/regenerate', { method: 'POST', body: { password } });
      setNewCodes(r.recovery_codes);
      setPassword('');
      await api('/auth/recovery-codes/confirm', { method: 'POST' });
      qc.invalidateQueries({ queryKey: ['auth', 'codes'] });
    } catch (e) {
      setError(describeError(e));
    }
  };

  const columns: Column<Session>[] = [
    { key: 'agent', header: 'Appareil', cell: (s) => <Text fontSize="sm" lineClamp={1}>{s.user_agent || 'Inconnu'}{s.current ? ' (cette session)' : ''}</Text> },
    { key: 'last', header: 'Dernière activité', cell: (s) => dateTimeFr(s.last_used_at) },
    { key: 'exp', header: 'Expire', cell: (s) => dateTimeFr(s.expires_at) },
    { key: 'act', header: '', cell: (s) => <Button size="xs" variant="outline" onClick={() => revoke(s)}>Fermer</Button> },
  ];

  return (
    <Stack gap="6">
      <Section title="Sessions actives" action={<Button size="xs" variant="outline" colorPalette="red" onClick={logoutAll}>Tout fermer</Button>}>
        <QueryBlock isPending={sessions.isPending} isError={sessions.isError} error={sessions.error} onRetry={() => sessions.refetch()}>
          <DataTable columns={columns} rows={sessions.data?.items ?? []} empty="Aucune session." caption="Sessions actives" />
        </QueryBlock>
      </Section>
      {withCodes && (
        <Section title="Codes de récupération">
          <QueryBlock isPending={codes.isPending} isError={codes.isError} error={codes.error} onRetry={() => codes.refetch()}>
            <Stack gap="3">
              <Text color={codes.data?.low ? 'danger' : undefined}>Codes restants : {codes.data?.remaining}{codes.data?.low ? ' — pense à en générer de nouveaux.' : ''}</Text>
              {newCodes ? (
                <Stack gap="2">
                  <Text fontWeight="semibold">Nouveaux codes (affichés une seule fois, les anciens ne fonctionnent plus) :</Text>
                  <SimpleGrid columns={2} gap="2">{newCodes.map((c) => <Code key={c} p="2" userSelect="all">{c}</Code>)}</SimpleGrid>
                </Stack>
              ) : (
                <HStack align="flex-end" wrap="wrap">
                  <Field.Root w={{ base: 'full', md: '72' }}>
                    <Field.Label>Mot de passe</Field.Label>
                    <Input type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} />
                  </Field.Root>
                  <Button disabled={!password} onClick={regenerate}>Générer de nouveaux codes</Button>
                </HStack>
              )}
            </Stack>
          </QueryBlock>
        </Section>
      )}
      {error && <Text color="danger" role="alert">{error}</Text>}
    </Stack>
  );
}

export default function Security({ withCodes = true }: { withCodes?: boolean }) {
  return <Providers><SecurityInner withCodes={withCodes} /></Providers>;
}
