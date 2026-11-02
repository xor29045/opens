import { useEffect, useMemo, useState } from 'react';
import { useQuery, keepPreviousData } from '@tanstack/react-query';
import { SimpleGrid, Stack, HStack, Input, NativeSelect, Field, Button, Text, Box } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import Pagination from '../../components/Pagination';
import { EmptyState, ErrorState, PageLoader } from '../../components/ui';
import { api } from '../../lib/api/client';
import { describeError } from '../../lib/api/problem';
import ProfileCardView from './ProfileCardView';
import type { Paginated, ProfileCard } from './types';
import type { components } from '../../lib/api/schema';

type Region = components['schemas']['Region'];

// Filters live in the URL so that a result page can be shared (§T6.6).
const FILTER_KEYS = ['region', 'city', 'age_min', 'age_max', 'price_max', 'sort'] as const;
type FilterKey = (typeof FILTER_KEYS)[number];
type Filters = Record<FilterKey, string>;

const emptyFilters = Object.fromEntries(FILTER_KEYS.map((k) => [k, ''])) as Filters;

function readUrl(): { filters: Filters; page: number } {
  const p = new URLSearchParams(window.location.search);
  const filters = Object.fromEntries(FILTER_KEYS.map((k) => [k, p.get(k) ?? ''])) as Filters;
  const page = Math.max(1, Number(p.get('page')) || 1);
  return { filters, page };
}

function writeUrl(filters: Filters, page: number): void {
  const p = new URLSearchParams();
  for (const k of FILTER_KEYS) if (filters[k]) p.set(k, filters[k]);
  if (page > 1) p.set('page', String(page));
  const qs = p.toString();
  window.history.replaceState(null, '', qs ? `?${qs}` : window.location.pathname);
}

function CatalogInner() {
  const [init] = useState(readUrl);
  const [filters, setFilters] = useState<Filters>(init.filters);
  const [page, setPage] = useState(init.page);

  useEffect(() => writeUrl(filters, page), [filters, page]);

  const regions = useQuery({
    queryKey: ['regions'],
    queryFn: ({ signal }) => api<{ items: Region[] }>('/regions', { auth: false, signal }),
    staleTime: 10 * 60_000,
  });
  const regionList = (regions.data?.items ?? []).filter((r) => r.kind === 'REGION');
  const selectedRegion = regionList.find((r) => r.slug === filters.region);
  const cities = (regions.data?.items ?? []).filter(
    (r) => r.kind === 'CITY' && (!selectedRegion || r.parent_id === selectedRegion.id),
  );

  const query = useMemo(() => {
    const p = new URLSearchParams({ page: String(page), page_size: '20' });
    for (const k of FILTER_KEYS) if (filters[k]) p.set(k, filters[k]);
    return p.toString();
  }, [filters, page]);

  const results = useQuery({
    queryKey: ['profiles', query],
    queryFn: ({ signal }) => api<Paginated<ProfileCard>>(`/profiles?${query}`, { auth: false, signal }),
    placeholderData: keepPreviousData,
  });

  const update = (key: FilterKey, value: string) => {
    setFilters((f) => ({ ...f, [key]: value, ...(key === 'region' ? { city: '' } : {}) }));
    setPage(1);
  };
  const active = FILTER_KEYS.some((k) => k !== 'sort' && filters[k]);

  return (
    <Stack gap="8">
      <Box
        as="section"
        aria-label="Filtrer le catalogue"
        role="search"
        bg="bg.subtle"
        borderWidth="1px"
        borderColor="border"
        borderRadius="xl"
        p={{ base: '4', md: '6' }}
      >
        <SimpleGrid columns={{ base: 2, md: 3, lg: 6 }} gap="4">
          <Field.Root>
            <Field.Label>Région</Field.Label>
            <NativeSelect.Root>
              <NativeSelect.Field value={filters.region} onChange={(e) => update('region', e.target.value)}>
                <option value="">Toutes</option>
                {regionList.map((r) => <option key={r.id} value={r.slug}>{r.name}</option>)}
              </NativeSelect.Field>
            </NativeSelect.Root>
          </Field.Root>
          <Field.Root>
            <Field.Label>Ville</Field.Label>
            <NativeSelect.Root>
              <NativeSelect.Field value={filters.city} onChange={(e) => update('city', e.target.value)}>
                <option value="">Toutes</option>
                {cities.map((c) => <option key={c.id} value={c.slug}>{c.name}</option>)}
              </NativeSelect.Field>
            </NativeSelect.Root>
          </Field.Root>
          <Field.Root>
            <Field.Label>Âge min.</Field.Label>
            <Input type="number" inputMode="numeric" min={18} max={99} placeholder="18" value={filters.age_min} onChange={(e) => update('age_min', e.target.value)} />
          </Field.Root>
          <Field.Root>
            <Field.Label>Âge max.</Field.Label>
            <Input type="number" inputMode="numeric" min={18} max={99} placeholder="99" value={filters.age_max} onChange={(e) => update('age_max', e.target.value)} />
          </Field.Root>
          <Field.Root>
            <Field.Label>Budget max. (€)</Field.Label>
            <Input type="number" inputMode="numeric" min={0} placeholder="—" value={filters.price_max} onChange={(e) => update('price_max', e.target.value)} />
          </Field.Root>
          <Field.Root>
            <Field.Label>Trier par</Field.Label>
            <NativeSelect.Root>
              <NativeSelect.Field value={filters.sort} onChange={(e) => update('sort', e.target.value)}>
                <option value="">Les plus récents</option>
                <option value="price_asc">Prix croissant</option>
                <option value="price_desc">Prix décroissant</option>
              </NativeSelect.Field>
            </NativeSelect.Root>
          </Field.Root>
        </SimpleGrid>
      </Box>

      {results.isPending && <PageLoader label="Recherche…" />}
      {results.isError && <ErrorState message={describeError(results.error)} onRetry={() => results.refetch()} />}
      {results.data && (
        <>
          <HStack justify="space-between" wrap="wrap">
            <Text color="fg.muted" aria-live="polite">
              {results.data.total} profil{results.data.total > 1 ? 's' : ''}
              {results.isFetching ? ' · mise à jour…' : ''}
            </Text>
            {active && (
              <Button size="sm" variant="ghost" color="fg.muted" onClick={() => { setFilters(emptyFilters); setPage(1); }}>
                Effacer les filtres
              </Button>
            )}
          </HStack>
          {results.data.items.length === 0 ? (
            <EmptyState title="Aucun profil ne correspond">Essaie d’élargir ta recherche.</EmptyState>
          ) : (
            <div className="os-cards">
              {results.data.items.map((c) => (
                <ProfileCardView key={c.slug} card={c} />
              ))}
            </div>
          )}
          <Pagination page={results.data.page} totalPages={results.data.total_pages} onPage={setPage} />
        </>
      )}
    </Stack>
  );
}

export default function Catalog() {
  return (
    <Providers>
      <CatalogInner />
    </Providers>
  );
}
