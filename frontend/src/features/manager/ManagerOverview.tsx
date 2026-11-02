import { useQuery } from '@tanstack/react-query';
import { Badge, Button, Grid, HStack, Stack, Text, Alert } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import { Section, StatusBadge } from '../../components/ui';
import { QueryBlock, DataTable, type Column } from '../dashboard/shared';
import { api } from '../../lib/api/client';
import { reasonLabels, dateFr } from '../../lib/format';
import type { components } from '../../lib/api/schema';

type Overview = components['schemas']['ManagerOverview'];
type ProfileSummary = components['schemas']['ProfileSummary'];

const checklistLabels: Record<string, string> = {
  establishment_complete: 'Fiche complète (BCE, agrément, localisation, téléphone de contrôle)',
  establishment_submitted: 'Fiche soumise pour vérification',
  establishment_verified: 'Établissement vérifié par la plateforme',
  subscription_active: 'Abonnement actif',
};

function Inner() {
  const q = useQuery({ queryKey: ['manager', 'overview'], queryFn: ({ signal }) => api<Overview>('/manager/publication-status', { signal }) });
  return (
    <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
      {q.data && <OverviewBody o={q.data} />}
    </QueryBlock>
  );
}

function OverviewBody({ o }: { o: Overview }) {
  const pending = o.profiles.filter((p) => p.status === 'PENDING_APPROVAL');
  const columns: Column<ProfileSummary>[] = [
    { key: 'name', header: 'Professionnel·le', cell: (p) => <a href={`/gerant/dashboard/profils/${p.id}`}>{p.display_name}</a> },
    { key: 'status', header: 'Statut', cell: (p) => <StatusBadge value={p.status} /> },
    { key: 'attested', header: 'Attesté', cell: (p) => (p.attested ? 'Oui' : 'Non') },
    { key: 'pub', header: 'Publication', cell: (p) => (p.publication.published ? <Badge colorPalette="green">Publié</Badge> : (
      <Text fontSize="sm" color="fg.muted">{p.publication.reasons.map((r) => reasonLabels[r] ?? r).join(' · ') || '—'}</Text>
    )) },
    { key: 'edit', header: 'Dernière modif.', cell: (p) => <Text fontSize="sm">{dateFr(p.last_edit.at)}{p.last_edit.by ? ` · ${p.last_edit.by}` : ''}</Text> },
  ];
  return (
    <Stack gap="6">
      {o.establishment.verification_status === 'REJECTED' && (
        <Alert.Root status="error"><Alert.Content>
          <Alert.Title>Fiche refusée</Alert.Title>
          <Alert.Description>{o.verification_note ?? 'Corrige la fiche puis soumets-la à nouveau.'}</Alert.Description>
        </Alert.Content></Alert.Root>
      )}
      {o.subscription?.status === 'PAST_DUE' && (
        <Alert.Root status="warning"><Alert.Content>
          <Alert.Title>Abonnement impayé</Alert.Title>
          <Alert.Description>Tes profils restent publiés pendant la période de grâce. <a href="/gerant/dashboard/abonnement">Régulariser</a>.</Alert.Description>
        </Alert.Content></Alert.Root>
      )}
      <Grid templateColumns={{ base: '1fr', md: '1fr 1fr' }} gap="6">
        <Section title="Check-list de l’établissement">
          <Stack gap="2">
            {o.checklist.map((c) => (
              <HStack key={c.key} justify="space-between">
                <Text fontSize="sm">{checklistLabels[c.key] ?? c.key}</Text>
                <Badge colorPalette={c.done ? 'green' : 'gray'}>{c.done ? 'OK' : 'À faire'}</Badge>
              </HStack>
            ))}
          </Stack>
          {o.blocking_reasons.length > 0 && (
            <Text mt="3" fontSize="sm" color="fg.muted">Bloquants : {o.blocking_reasons.map((r) => reasonLabels[r] ?? r).join(' · ')}</Text>
          )}
          <HStack mt="4">
            <Button asChild size="sm" variant="outline"><a href="/gerant/dashboard/etablissement">Modifier la fiche</a></Button>
          </HStack>
        </Section>
        <Section title={`Rattachements en attente (${o.pending_count})`}>
          {pending.length === 0 ? (
            <Text color="fg.muted">Aucune inscription en attente.</Text>
          ) : (
            <Stack gap="2">
              {pending.map((p) => (
                <HStack key={p.id} justify="space-between">
                  <Text>{p.display_name}</Text>
                  <Button asChild size="xs" colorPalette="brand"><a href={`/gerant/dashboard/profils/${p.id}`}>Examiner</a></Button>
                </HStack>
              ))}
            </Stack>
          )}
        </Section>
      </Grid>
      <Section title={`Professionnel·les (${o.profiles.length})`} action={<Button asChild size="xs" variant="outline"><a href="/gerant/dashboard/profils">Gérer</a></Button>}>
        <DataTable columns={columns} rows={o.profiles} empty="Aucun·e professionnel·le pour le moment." caption="Liste des profils" />
      </Section>
    </Stack>
  );
}

export default function ManagerOverview() {
  return (
    <Providers>
      <Inner />
    </Providers>
  );
}
