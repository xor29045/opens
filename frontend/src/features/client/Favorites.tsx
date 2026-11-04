import { useQuery } from '@tanstack/react-query';
import { useStore } from '@nanostores/react';
import { Badge, Box, Button, HStack, SimpleGrid, Stack, Text } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import { Section, EmptyState } from '../../components/ui';
import { QueryBlock } from '../dashboard/shared';
import { api } from '../../lib/api/client';
import { eur } from '../../lib/format';
import { forget, likesAtom } from '../../lib/likes';
import type { components } from '../../lib/api/schema';

type Card = components['schemas']['DiscoverCard'];

// Favorites of this browser (ADR-023): the ids come from localStorage, the
// API describes those still published; the others show as unavailable.
function FavoritesInner() {
  const { liked } = useStore(likesAtom);
  const ids = liked.map((l) => l.id);
  const q = useQuery({
    queryKey: ['favorites', ids.join(',')],
    queryFn: ({ signal }) => api<{ items: Card[] }>('/discover/cards', { method: 'POST', auth: false, signal, body: { ids } }),
    enabled: ids.length > 0,
  });
  if (liked.length === 0) {
    return (
      <Section title="Mes favoris">
        <EmptyState title="Aucun favori">Like des profils depuis <a href="/decouvrir">Découvrir</a> : ils s’affichent ici.</EmptyState>
      </Section>
    );
  }
  const live = new Map((q.data?.items ?? []).map((c) => [c.id, c]));
  return (
    <Section title={`Mes favoris (${liked.length})`}>
      <Text fontSize="sm" color="fg.muted" mb="4">Tes favoris sont enregistrés dans ce navigateur uniquement, sans compte.</Text>
      <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
        <SimpleGrid columns={{ base: 1, md: 2, lg: 3 }} gap="4">
          {liked.map((l) => {
            const c = live.get(l.id);
            const photo = c?.photos[0] ?? null;
            return (
              <Box key={l.id} borderWidth="1px" borderColor="border" borderRadius="lg" overflow="hidden" bg="bg.subtle">
                <HStack align="stretch" gap="0">
                  <Box w="28" minH="36" flexShrink={0} bg="bg.muted">
                    {photo && <img src={photo} alt="" loading="lazy" style={{ width: '100%', height: '100%', objectFit: 'cover' }} />}
                  </Box>
                  <Stack p="3" gap="1" flex="1" minW="0">
                    {c ? <a href={`/profils/${c.slug}`}><strong>{c.display_name}</strong></a> : <Text fontWeight="semibold">{l.display_name}</Text>}
                    {c ? (
                      <Text fontSize="sm" color="fg.muted">
                        {c.establishment_name}{c.city ? ` · ${c.city}` : ''}{c.price_from_cents != null ? ` · dès ${eur(c.price_from_cents)}` : ''}
                      </Text>
                    ) : <Badge alignSelf="flex-start">Indisponible</Badge>}
                    {c?.headline && <Text fontSize="sm" lineClamp={2}>{c.headline}</Text>}
                    <Button size="xs" variant="ghost" alignSelf="flex-start" mt="auto" onClick={() => forget(l.id)} aria-label={`Retirer ${l.display_name} des favoris`}>
                      Retirer
                    </Button>
                  </Stack>
                </HStack>
              </Box>
            );
          })}
        </SimpleGrid>
      </QueryBlock>
    </Section>
  );
}

export default function Favorites() {
  return <Providers><FavoritesInner /></Providers>;
}
