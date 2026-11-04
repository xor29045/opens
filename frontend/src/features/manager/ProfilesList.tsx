import { useState } from 'react';
import { useQuery, keepPreviousData } from '@tanstack/react-query';
import { Badge, Button, Field, HStack, Input, NativeSelect, Stack, Text } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import Pagination from '../../components/Pagination';
import { Section, StatusBadge } from '../../components/ui';
import { QueryBlock, DataTable, type Column } from '../dashboard/shared';
import { api } from '../../lib/api/client';
import { describeError } from '../../lib/api/problem';
import { reasonLabels } from '../../lib/format';
import type { components } from '../../lib/api/schema';

type ProfileSummary = components['schemas']['ProfileSummary'];
type Page = components['schemas']['PageProfileSummary'];

function Inner() {
  const [status, setStatus] = useState('');
  const [published, setPublished] = useState('');
  const [page, setPage] = useState(1);
  const [name, setName] = useState('');
  const [login, setLogin] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const qs = new URLSearchParams({ page: String(page), page_size: '20' });
  if (status) qs.set('status', status);
  if (published) qs.set('published', published);
  const q = useQuery({
    queryKey: ['manager', 'profiles', qs.toString()],
    queryFn: ({ signal }) => api<Page>(`/manager/profiles?${qs}`, { signal }),
    placeholderData: keepPreviousData,
  });

  const create = async () => {
    setError(null);
    setBusy(true);
    try {
      const body: Record<string, string> = { display_name: name.trim() };
      if (login.trim()) Object.assign(body, { username: login.trim(), password });
      const p = await api<{ id: string }>('/manager/profiles', { method: 'POST', body });
      window.location.assign(`/gerant/dashboard/profils/${p.id}`);
    } catch (e) {
      setError(describeError(e));
    } finally {
      setBusy(false);
    }
  };

  const columns: Column<ProfileSummary>[] = [
    { key: 'name', header: 'Nom', cell: (p) => <a href={`/gerant/dashboard/profils/${p.id}`}>{p.display_name}</a> },
    { key: 'status', header: 'Statut', cell: (p) => <StatusBadge value={p.status} /> },
    { key: 'account', header: 'Compte lié', cell: (p) => (p.linked_account ? `${p.linked_account.username} (${p.linked_account.status === 'ACTIVE' ? 'actif' : 'suspendu'})` : 'Aucun') },
    { key: 'pub', header: 'Publication', cell: (p) => (p.publication.published ? <Badge colorPalette="green">Publié</Badge> : (
      <Text fontSize="xs" color="fg.muted">{p.publication.reasons.map((r) => reasonLabels[r] ?? r).join(' · ')}</Text>
    )) },
  ];

  return (
    <Stack gap="6">
      <Section title="Créer une professionnelle">
        <Text fontSize="sm" color="fg.muted" mb="3">
          Choisis son pseudo et son mot de passe et transmets-les-lui : elle se connecte sur <a href="/pro/login">/pro/login</a> et
          arrive directement dans l’espace de ta maison. Sans identifiants, tu gères seul·e le profil (tu pourras créer ses accès plus tard).
        </Text>
        <HStack align="flex-end" wrap="wrap">
          <Field.Root w={{ base: 'full', md: '56' }}>
            <Field.Label>Nom affiché</Field.Label>
            <Input value={name} maxLength={60} onChange={(e) => setName(e.target.value)} />
          </Field.Root>
          <Field.Root w={{ base: 'full', md: '48' }}>
            <Field.Label>Pseudo de connexion</Field.Label>
            <Input value={login} maxLength={30} autoComplete="off" onChange={(e) => setLogin(e.target.value)} />
          </Field.Root>
          <Field.Root w={{ base: 'full', md: '56' }}>
            <Field.Label>Mot de passe</Field.Label>
            <Input type="text" value={password} maxLength={128} autoComplete="new-password" onChange={(e) => setPassword(e.target.value)}
              disabled={!login.trim()} placeholder="12 caractères minimum" />
          </Field.Root>
          <Button colorPalette="brand" loading={busy}
            disabled={busy || !name.trim() || (!!login.trim() && password.length < 12)} onClick={create}>Créer</Button>
        </HStack>
        {error && <Text color="danger" role="alert" mt="2">{error}</Text>}
      </Section>
      <Section title="Tous les profils">
        <HStack gap="4" mb="4" wrap="wrap">
          <Field.Root w="56">
            <Field.Label>Statut</Field.Label>
            <NativeSelect.Root><NativeSelect.Field value={status} onChange={(e) => { setStatus(e.target.value); setPage(1); }}>
              <option value="">Tous</option>
              <option value="DRAFT">Brouillon</option>
              <option value="ACTIVE">Actif</option>
              <option value="ARCHIVED">Archivé</option>
            </NativeSelect.Field></NativeSelect.Root>
          </Field.Root>
          <Field.Root w="44">
            <Field.Label>Publication</Field.Label>
            <NativeSelect.Root><NativeSelect.Field value={published} onChange={(e) => { setPublished(e.target.value); setPage(1); }}>
              <option value="">Tous</option>
              <option value="true">Publiés</option>
              <option value="false">Non publiés</option>
            </NativeSelect.Field></NativeSelect.Root>
          </Field.Root>
        </HStack>
        <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
          {q.data && (
            <>
              <DataTable columns={columns} rows={q.data.items} empty="Aucun profil." caption="Profils de l’établissement" />
              <Pagination page={q.data.page} totalPages={q.data.total_pages} onPage={setPage} />
            </>
          )}
        </QueryBlock>
      </Section>
    </Stack>
  );
}

export default function ProfilesList() {
  return (
    <Providers>
      <Inner />
    </Providers>
  );
}
