import { useState } from 'react';
import { Badge, Box, Button, Grid, HStack, Heading, Link, List, Stack, Text, Image, Table } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import { Section } from '../../components/ui';
import { useSession } from '../../lib/auth/useSession';
import { api } from '../../lib/api/client';
import { describeError } from '../../lib/api/problem';
import { eur } from '../../lib/format';
import { contactHref, contactLabels, type PublicProfile } from './types';

function Details({ p }: { p: PublicProfile }) {
  const rows: [string, string | null | undefined][] = [
    ['Taille', p.height_cm ? `${p.height_cm} cm` : null],
    ['Poids', p.weight_kg ? `${p.weight_kg} kg` : null],
    ['Cheveux', p.hair_color],
    ['Yeux', p.eye_color],
    ['Morphologie', p.body_type],
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

function FavoriteButton({ slug, profileId }: { slug: string; profileId: string }) {
  const { user } = useSession();
  const [added, setAdded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  if (!user || user.role !== 'CLIENT') return null;
  const toggle = async () => {
    setError(null);
    try {
      if (added) await api(`/client/favorites/${profileId}`, { method: 'DELETE' });
      else await api(`/client/favorites/${profileId}`, { method: 'PUT' });
      setAdded(!added);
    } catch (e) {
      setError(describeError(e));
    }
  };
  return (
    <Stack>
      <Button variant={added ? 'solid' : 'outline'} colorPalette="brand" size="sm" onClick={toggle} aria-label={`${added ? 'Retirer' : 'Ajouter'} ${slug} des favoris`}>
        {added ? 'Dans mes favoris' : 'Ajouter aux favoris'}
      </Button>
      {error && <Text color="danger" fontSize="sm" role="alert">{error}</Text>}
    </Stack>
  );
}

function Contacts({ p }: { p: PublicProfile }) {
  const { user } = useSession();
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
      {p.contacts_restricted && (
        <Text fontSize="sm" color="fg.muted">
          {user ? 'Certaines coordonnées ne sont pas accessibles avec ton compte.' : (
            <>Certaines coordonnées sont réservées aux clients inscrits. <Link href="/client/login">Connexion</Link> · <Link href="/client/register">Créer un compte</Link></>
          )}
        </Text>
      )}
    </Stack>
  );
}

function ProfileInner({ profile: p }: { profile: PublicProfile }) {
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
          <Heading size="4xl" fontWeight="500">{p.display_name}{p.age != null ? `, ${p.age} ans` : ''}</Heading>
          {p.headline && <Text fontSize="lg">{p.headline}</Text>}
          <HStack wrap="wrap" gap="2">
            {p.categories.map((c) => <Badge key={c.slug} variant="subtle">{c.name}</Badge>)}
          </HStack>
          <FavoriteButton slug={p.slug} profileId={p.id} />
          <Section title="Tarifs">
            {p.rates.length === 0 ? <Text color="fg.muted">Non renseignés.</Text> : (
              <List.Root gap="2" listStyle="none">
                {p.rates.map((r) => (
                  <List.Item key={`${r.label}-${r.amount_cents}`} display="flex" justifyContent="space-between">
                    <span>{r.label}{r.duration_minutes ? ` · ${r.duration_minutes} min` : ''}</span>
                    <strong>{eur(r.amount_cents)}</strong>
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

export default function ProfileView({ profile }: { profile: PublicProfile }) {
  return (
    <Providers>
      <ProfileInner profile={profile} />
    </Providers>
  );
}
