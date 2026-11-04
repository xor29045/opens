import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Badge, Box, Button, Grid, HStack, Heading, Link, List, Stack, Text, Image, Table } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import { Section } from '../../components/ui';
import { QueryBlock } from '../dashboard/shared';
import { useStore } from '@nanostores/react';
import { forget, like, likesAtom } from '../../lib/likes';
import { api } from '../../lib/api/client';
import { bodyTypes, eyeColors, genders, hairColors, labelOf, skinTones } from '../../lib/attributes';
import { contactHref, contactLabels, type PublicProfile } from './types';
import { Roses } from '../../components/Rose';

function Details({ p }: { p: PublicProfile }) {
  const attrs = p.extra_attributes as Record<string, string>;
  const rows: [string, string | null | undefined][] = [
    ['Âge', p.age ? `${p.age} ans` : null],
    ['Genre', labelOf(genders, attrs.gender)],
    ['Taille', p.height_cm ? `${p.height_cm} cm` : null],
    ['Poids', p.weight_kg ? `${p.weight_kg} kg` : null],
    ['Cheveux', labelOf(hairColors, p.hair_color)],
    ['Yeux', labelOf(eyeColors, p.eye_color)],
    ['Peau', labelOf(skinTones, attrs.skin_tone)],
    ['Morphologie', labelOf(bodyTypes, p.body_type)],
    ['Langues', p.languages.length ? p.languages.join(', ') : null],
  ];
  return (
    <Table.Root size="sm" variant="outline">
      <Table.Body>
        {rows
          .filter(([, v]) => v)
          .map(([k, v]) => (
            <Table.Row key={k}>
              <Table.Cell fontWeight="medium" w="40">{k}</Table.Cell>
              <Table.Cell>{v}</Table.Cell>
            </Table.Row>
          ))}
      </Table.Body>
    </Table.Root>
  );
}

// Likes are kept in this browser (ADR-023), shared with the "Découvrir" deck.
function FavoriteButton({ p }: { p: PublicProfile }) {
  const { liked } = useStore(likesAtom);
  const added = liked.some((l) => l.id === p.id);
  const toggle = () => {
    if (added) forget(p.id);
    else like({ id: p.id, slug: p.slug, display_name: p.display_name, establishment_name: p.establishment.display_name,
      city: p.establishment.city?.name ?? null, photo: p.photos[0]?.url ?? null });
  };
  return (
    <Button variant={added ? 'solid' : 'outline'} colorPalette="brand" size="sm" alignSelf="flex-start" onClick={toggle}
      aria-pressed={added} aria-label={`${added ? 'Retirer' : 'Ajouter'} ${p.display_name} ${added ? 'des' : 'aux'} favoris`}>
      {added ? 'Dans mes favoris' : 'Ajouter aux favoris'}
    </Button>
  );
}

function Contacts({ p }: { p: PublicProfile }) {
  return (
    <Stack gap="2">
      {p.contacts.length === 0 && <Text color="fg.muted">Aucune coordonnée publique.</Text>}
      {p.contacts.map((c) => {
        const href = contactHref(c.type, c.value);
        return (
          <HStack key={`${c.type}-${c.value}`} justify="space-between">
            <Text fontSize="sm" color="fg.muted">{contactLabels[c.type] ?? c.type}</Text>
            {href ? (
              <Link href={href} rel="noopener noreferrer nofollow" target={href.startsWith('https') ? '_blank' : undefined}>{c.value}</Link>
            ) : (
              <Text>{c.value}</Text>
            )}
          </HStack>
        );
      })}
    </Stack>
  );
}

export function ProfileInner({ profile: p }: { profile: PublicProfile }) {
  const [active, setActive] = useState(0);
  const photos = p.photos;
  const current = photos[active];
  return (
    <Stack gap="6">
      <Grid templateColumns={{ base: '1fr', md: '3fr 2fr' }} gap="6">
        <Box>
          <Box borderRadius="lg" overflow="hidden" bg="bg.subtle" aspectRatio={3 / 4} maxH="75vh" mx="auto">
            {current ? (
              <Image src={current.url} alt={`${p.display_name} — photo ${active + 1} sur ${photos.length}`} width="100%" height="100%" objectFit="cover" />
            ) : (
              <Text p="8" color="fg.muted">Aucune photo</Text>
            )}
          </Box>
          {photos.length > 1 && (
            <HStack mt="3" gap="2" wrap="wrap" role="group" aria-label="Galerie">
              {photos.map((ph, i) => (
                <Box
                  as="button"
                  key={ph.id}
                  onClick={() => setActive(i)}
                  aria-label={`Afficher la photo ${i + 1}`}
                  aria-pressed={i === active}
                  w="16"
                  h="20"
                  borderRadius="md"
                  overflow="hidden"
                  borderWidth="2px"
                  borderColor={i === active ? 'brand.solid' : 'transparent'}
                  opacity={i === active ? 1 : 0.65}
                  transition="opacity 150ms, border-color 150ms"
                  _hover={{ opacity: 1 }}
                  cursor="pointer"
                  p="0"
                  bg="bg.muted"
                >
                  <Image src={ph.url} alt="" width="100%" height="100%" objectFit="cover" loading="lazy" />
                </Box>
              ))}
            </HStack>
          )}
        </Box>
        <Stack gap="4">
          <Heading size="4xl" fontWeight="500">{p.display_name}</Heading>
          {p.headline && <Text fontSize="lg">{p.headline}</Text>}
          <HStack wrap="wrap" gap="2">
            {p.categories.map((c) => <Badge key={c.slug} variant="subtle">{c.name}</Badge>)}
          </HStack>
          <FavoriteButton p={p} />
          <Section title="Tarifs">
            {p.rates.length === 0 ? <Text color="fg.muted">Non renseignés.</Text> : (
              <List.Root gap="2" listStyle="none">
                {p.rates.map((r) => (
                  <List.Item key={`${r.label}-${r.amount_cents}`} display="flex" justifyContent="space-between">
                    <span>{r.label}{r.duration_minutes ? ` · ${r.duration_minutes} min` : ''}</span>
                    <strong><Roses cents={r.amount_cents} /></strong>
                  </List.Item>
                ))}
              </List.Root>
            )}
          </Section>
          <Section title="Coordonnées"><Contacts p={p} /></Section>
          <Section title="Établissement">
            <Stack gap="1">
              <Link href={`/etablissements/${p.establishment.slug}`} fontWeight="semibold">{p.establishment.display_name}</Link>
              {p.establishment.city && <Text fontSize="sm" color="fg.muted">{p.establishment.city.name}</Text>}
            </Stack>
          </Section>
        </Stack>
      </Grid>
      {p.description && (
        <Section title="Présentation"><Text whiteSpace="pre-line">{p.description}</Text></Section>
      )}
      <Section title="Détails"><Details p={p} /></Section>
    </Stack>
  );
}

// The catalogue is public (ADR-023); a staff session, when present, is still
// sent so a house sees its own restricted contacts.
function ProfileLoader({ slug }: { slug: string }) {
  const q = useQuery({
    queryKey: ['catalog', 'profile', slug],
    queryFn: ({ signal }) => api<PublicProfile>(`/profiles/${encodeURIComponent(slug)}`, { signal }),
  });
  return (
    <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
      {q.data && <ProfileInner profile={q.data} />}
    </QueryBlock>
  );
}

export default function ProfileView({ slug }: { slug: string }) {
  return (
    <Providers>
      <ProfileLoader slug={slug} />
    </Providers>
  );
}
