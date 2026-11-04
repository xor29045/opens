import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Button, Code, Field, HStack, Input, SimpleGrid, Stack, Text } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import { Section } from '../../components/ui';
import { QueryBlock } from '../dashboard/shared';
import { api } from '../../lib/api/client';
import { describeError } from '../../lib/api/problem';
import type { components } from '../../lib/api/schema';

type RecoveryStatus = components['schemas']['RecoveryStatus'];

/** Recovery codes, shared by the manager and professional spaces. */
function SecurityInner() {
  const qc = useQueryClient();
  const codes = useQuery({ queryKey: ['auth', 'codes'], queryFn: ({ signal }) => api<RecoveryStatus>('/auth/recovery-codes/status', { signal }) });
  const [password, setPassword] = useState('');
  const [newCodes, setNewCodes] = useState<string[] | null>(null);
  const [error, setError] = useState<string | null>(null);

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

  return (
    <Stack gap="6">
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
      {error && <Text color="danger" role="alert">{error}</Text>}
    </Stack>
  );
}

export default function Security() {
  return <Providers><SecurityInner /></Providers>;
}
