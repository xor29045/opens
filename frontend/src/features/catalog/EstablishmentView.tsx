import { useQuery } from '@tanstack/react-query';
import { Grid, Heading, Link, Stack, Text } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import { Section, EmptyState } from '../../components/ui';
import { QueryBlock } from '../dashboard/shared';
import { api } from '../../lib/api/client';
import ProfileCardView from './ProfileCardView';
import { contactHref, contactLabels, type PublicEstablishment } from './types';

const dayLabels: Record<string, string> = { mon: 'Lundi', tue: 'Mardi', wed: 'Mercredi', thu: 'Jeudi', fri: 'Vendredi', sat: 'Samedi', sun: 'Dimanche' };

function Hours({ hours }: { hours: Record<string, { open: string; close: string }[]> }) {
  const entries = Object.entries(hours);
  if (entries.length === 0) return <Text color="fg.muted">Non renseignés.</Text>;
  return (
    <Stack gap="1">
      {Object.keys(dayLabels).map((d) => {
        const ranges = hours[d];
        if (!ranges) return null;
        return <Text key={d} fontSize="sm">{dayLabels[d]} : {ranges.map((r) => `${r.open}–${r.close}`).join(', ')}</Text>;
      })}
    </Stack>
  );
}

function EstablishmentInner({ est }: { est: PublicEstablishment }) {
  const hours = (est.opening_hours ?? {}) as Record<string, { open: string; close: string }[]>;
  return (
    <Stack gap="6">
      <Stack gap="1">
        <Heading size="2xl">{est.display_name}</Heading>
        {est.city && <Text fontSize="sm" color="fg.muted">{est.city.name}{est.region ? ` · ${est.region.name}` : ''}</Text>}
        {est.address && <Text fontSize="sm">{est.address}</Text>}
      </Stack>
      <Grid templateColumns={{ base: '1fr', md: '2fr 1fr' }} gap="6">
        <Section title="Présentation">
          <Text whiteSpace="pre-line">{est.description || 'Pas de description.'}</Text>
        </Section>
        <Stack gap="6">
          <Section title="Horaires"><Hours hours={hours} /></Section>
          <Section title="Contact">
            <Stack gap="2">
              {est.contacts.map((c) => {
                const href = contactHref(c.type, c.value);
                return (
                  <Text key={`${c.type}-${c.value}`} fontSize="sm">
                    {contactLabels[c.type] ?? c.type} :{' '}
                    {href ? <Link href={href} rel="noopener noreferrer nofollow">{c.value}</Link> : c.value}
                  </Text>
                );
              })}
              {est.contacts_restricted && (
                <Text fontSize="sm" color="fg.muted">Certaines coordonnées sont réservées aux clients inscrits.</Text>
              )}
            </Stack>
          </Section>
        </Stack>
      </Grid>
      <Section title={`Professionnel·les (${est.profiles.length})`}>
        {est.profiles.length === 0 ? (
          <EmptyState title="Aucun profil publié pour le moment" />
        ) : (
          <Grid templateColumns={{ base: 'repeat(2, 1fr)', md: 'repeat(3, 1fr)', lg: 'repeat(4, 1fr)' }} gap="4">
            {est.profiles.map((c) => <ProfileCardView key={c.slug} card={c} />)}
          </Grid>
        )}
      </Section>
    </Stack>
  );
}

function EstablishmentLoader({ slug }: { slug: string }) {
  const q = useQuery({
    queryKey: ['catalog', 'establishment', slug],
    queryFn: ({ signal }) => api<PublicEstablishment>(`/establishments/${encodeURIComponent(slug)}`, { signal }),
  });
  return (
    <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
      {q.data && <EstablishmentInner est={q.data} />}
    </QueryBlock>
  );
}

export default function EstablishmentView({ slug }: { slug: string }) {
  return (
    <Providers>
      <EstablishmentLoader slug={slug} />
    </Providers>
  );
}
