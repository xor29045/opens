import { useQuery } from '@tanstack/react-query';
import { Badge, List, Stack, Text } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import { Section, StatusBadge } from '../../components/ui';
import { QueryBlock } from '../dashboard/shared';
import { api } from '../../lib/api/client';
import { dateFr, reasonLabels } from '../../lib/format';
import ProfileView from '../catalog/ProfileView';
import type { components } from '../../lib/api/schema';

type PublicationStatus = components['schemas']['PublicationStatus'];
type PublicProfile = components['schemas']['PublicProfile'];

// The professional sees whether the profile is published and why, never
// amounts or invoices (§2.4).
function StatusInner() {
  const q = useQuery({ queryKey: ['pro', 'status'], queryFn: ({ signal }) => api<PublicationStatus>('/professional/me/publication-status', { signal }) });
  return (
    <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
      {q.data && (
        <Section title="Statut de publication" action={<StatusBadge value={q.data.status} />}>
          <Stack gap="3">
            {q.data.published ? (
              <Badge colorPalette="green" alignSelf="flex-start">Publié depuis le {dateFr(q.data.published_at)}</Badge>
            ) : (
              <>
                <Text>Ton profil n’est pas encore visible. Il manque :</Text>
                <List.Root ps="5">
                  {q.data.reasons.map((r) => <List.Item key={r}>{reasonLabels[r] ?? r}</List.Item>)}
                </List.Root>
              </>
            )}
            <Text fontSize="sm" color="fg.muted">
              L’attestation de ton âge, l’activation et l’archivage relèvent du gérant de ta maison.
            </Text>
          </Stack>
        </Section>
      )}
    </QueryBlock>
  );
}

function PreviewInner() {
  const q = useQuery({ queryKey: ['pro', 'preview'], queryFn: ({ signal }) => api<PublicProfile>('/professional/me/preview', { signal }) });
  return (
    <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
      {q.data && (
        <Stack gap="4">
          <Text fontSize="sm" color="fg.muted">Aperçu tel que le public le verra. Les coordonnées « clients inscrits » ne s’affichent qu’aux clients connectés.</Text>
          <ProfileView profile={q.data} />
        </Stack>
      )}
    </QueryBlock>
  );
}

export function ProStatus() {
  return <Providers><StatusInner /></Providers>;
}
export function ProPreview() {
  return <Providers><PreviewInner /></Providers>;
}
