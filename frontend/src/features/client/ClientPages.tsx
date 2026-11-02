import { useState, type FormEvent } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Badge, Button, Field, HStack, Input, SimpleGrid, Stack, Text, Box } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import Pagination from '../../components/Pagination';
import { Section, EmptyState } from '../../components/ui';
import { QueryBlock, ActionDialog } from '../dashboard/shared';
import { api } from '../../lib/api/client';
import { describeError } from '../../lib/api/problem';
import { eur } from '../../lib/format';
import { sessionStore } from '../../lib/auth/session';
import type { components } from '../../lib/api/schema';

type Favorite = components['schemas']['Favorite'];
type PageOf<T> = { items: T[]; page: number; page_size: number; total: number; total_pages: number };
type User = components['schemas']['User'];

function FavoritesInner() {
  const qc = useQueryClient();
  const [page, setPage] = useState(1);
  const [error, setError] = useState<string | null>(null);
  const q = useQuery({ queryKey: ['client', 'favorites', page], queryFn: ({ signal }) => api<PageOf<Favorite>>(`/client/favorites?page=${page}`, { signal }) });
  const remove = async (f: Favorite) => {
    setError(null);
    try {
      await api(`/client/favorites/${f.profile_id}`, { method: 'DELETE' });
      qc.invalidateQueries({ queryKey: ['client', 'favorites'] });
    } catch (e) {
      setError(describeError(e));
    }
  };
  return (
    <Section title="Mes favoris">
      <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
        {q.data && (q.data.items.length === 0 ? (
          <EmptyState title="Aucun favori">Ajoute des profils depuis le <a href="/profils">catalogue</a>.</EmptyState>
        ) : (
          <>
            <SimpleGrid columns={{ base: 1, md: 2 }} gap="3">
              {q.data.items.map((f) => (
                <Box key={f.profile_id} borderWidth="1px" borderColor="border" borderRadius="md" p="3">
                  <HStack justify="space-between">
                    <Stack gap="0">
                      {f.available ? <a href={`/profils/${f.slug}`}><strong>{f.display_name}</strong></a> : <Text fontWeight="semibold">{f.display_name}</Text>}
                      {f.available ? (
                        <Text fontSize="sm" color="fg.muted">{f.establishment_name}{f.city ? ` · ${f.city}` : ''}{f.price_from_cents ? ` · dès ${eur(f.price_from_cents)}` : ''}</Text>
                      ) : <Badge alignSelf="flex-start">Indisponible</Badge>}
                    </Stack>
                    <Button size="xs" variant="ghost" onClick={() => remove(f)} aria-label={`Retirer ${f.display_name} des favoris`}>Retirer</Button>
                  </HStack>
                </Box>
              ))}
            </SimpleGrid>
            <Pagination page={q.data.page} totalPages={q.data.total_pages} onPage={setPage} />
          </>
        ))}
      </QueryBlock>
      {error && <Text color="danger" role="alert">{error}</Text>}
    </Section>
  );
}

function AccountInner() {
  const qc = useQueryClient();
  const me = useQuery({ queryKey: ['client', 'me'], queryFn: ({ signal }) => api<User>('/client/me', { signal }) });
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const rename = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setMessage(null);
    try {
      await api('/client/me', { method: 'PUT', body: { username } });
      setMessage('Pseudo modifié.');
      setUsername('');
      qc.invalidateQueries({ queryKey: ['client', 'me'] });
    } catch (err) {
      setError(describeError(err));
    }
  };

  return (
    <QueryBlock isPending={me.isPending} isError={me.isError} error={me.error} onRetry={() => me.refetch()}>
      {me.data && (
        <Stack gap="6">
          <Section title="Mon compte">
            <Stack gap="4">
              <Text>Pseudo actuel : <strong>{me.data.username}</strong></Text>
              <form onSubmit={rename}>
                <HStack align="flex-end" wrap="wrap">
                  <Field.Root w={{ base: 'full', md: '72' }}>
                    <Field.Label>Nouveau pseudo</Field.Label>
                    <Input value={username} onChange={(e) => setUsername(e.target.value)} />
                  </Field.Root>
                  <Button type="submit" disabled={!username.trim()}>Modifier</Button>
                </HStack>
              </form>
              <HStack gap="4" wrap="wrap">
                <a href="/compte/mot-de-passe">Changer mon mot de passe</a>
                <a href="/client/espace/sessions">Mes sessions et codes de récupération</a>
              </HStack>
            </Stack>
          </Section>
          <Section title="Supprimer mon compte">
            <Stack gap="3">
              <Text fontSize="sm" color="fg.muted">Suppression immédiate, données purgées sous 30 jours. Tes favoris sont perdus.</Text>
              <Field.Root w={{ base: 'full', md: '72' }}>
                <Field.Label>Mot de passe</Field.Label>
                <Input type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} />
              </Field.Root>
              <HStack>
                <ActionDialog trigger="Supprimer mon compte" title="Supprimer définitivement ton compte ?" danger confirmLabel="Supprimer"
                  description="Cette action ne peut pas être annulée."
                  onConfirm={async () => {
                    await api('/client/me', { method: 'DELETE', body: { password } });
                    sessionStore.clear();
                    window.location.replace('/');
                  }} />
              </HStack>
            </Stack>
          </Section>
          {message && <Text color="success" role="status">{message}</Text>}
          {error && <Text color="danger" role="alert">{error}</Text>}
        </Stack>
      )}
    </QueryBlock>
  );
}

export function ClientFavorites() {
  return <Providers><FavoritesInner /></Providers>;
}
export function ClientAccount() {
  return <Providers><AccountInner /></Providers>;
}
