import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Box, Input } from '@chakra-ui/react';
import { api } from '../lib/api/client';
import { filtersToQuery, loadFilters } from '../features/client/DeckFilters';
import type { components } from '../lib/api/schema';

type Region = components['schemas']['Region'];

const norm = (s: string) => s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().trim();

/**
 * Navbar search by département ("Paris", "75", "paris (75)"…). Suggestions
 * come from GET /regions; submitting opens /decouvrir?city=<slug>, keeping
 * the other filters remembered in this browser.
 */
export default function SearchBar() {
  // Starts empty on the server and on hydration, then picks up ?city=.
  const [text, setText] = useState('');
  useEffect(() => {
    setText(new URLSearchParams(window.location.search).get('city') ?? '');
  }, []);
  const [notFound, setNotFound] = useState(false);
  const regions = useQuery({
    queryKey: ['regions'],
    queryFn: ({ signal }) => api<{ items: Region[] }>('/regions', { auth: false, signal }),
    staleTime: 3_600_000,
  });
  const cities = useMemo(
    () => (regions.data?.items ?? []).filter((r) => r.kind === 'CITY' && r.is_active),
    [regions.data],
  );
  // Once loaded, show the name of the département the URL points at.
  const fromSlug = cities.find((c) => c.slug === text);
  const shown = fromSlug?.name ?? text;

  const find = (raw: string): Region | undefined => {
    const q = norm(raw);
    if (!q) return undefined;
    const code = q.match(/^\d{1,2}[ab]?$/)?.[0]?.padStart(2, '0');
    return (
      cities.find((c) => norm(c.name) === q || c.slug === q) ??
      (code ? cities.find((c) => c.slug.startsWith(`${code}-`)) : undefined) ??
      cities.find((c) => norm(c.name).startsWith(q)) ??
      cities.find((c) => norm(c.name).includes(q))
    );
  };

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const q = shown.trim();
    const qs = filtersToQuery({ ...loadFilters(), city: undefined });
    if (!q) {
      window.location.href = qs ? `/decouvrir?${qs}` : '/decouvrir';
      return;
    }
    const city = find(q);
    if (!city) {
      setNotFound(true);
      return;
    }
    window.location.href = `/decouvrir?${filtersToQuery({ ...loadFilters(), city: city.slug })}`;
  };

  return (
    <Box as="form" role="search" onSubmit={submit} flex="1" minW="0" maxW="420px">
      <Input
        size="sm"
        type="search"
        name="ville"
        list="os-search-cities"
        placeholder="Rechercher une ville ou un département"
        aria-label="Rechercher par ville ou département"
        aria-invalid={notFound || undefined}
        title={notFound ? 'Aucun département ne correspond' : undefined}
        borderRadius="full"
        bg="bg.subtle"
        value={shown}
        onChange={(e) => {
          setText(e.target.value);
          setNotFound(false);
        }}
      />
      <datalist id="os-search-cities">
        {cities.map((c) => <option key={c.id} value={c.name} />)}
      </datalist>
      <Box as="span" className="visually-hidden" aria-live="polite">
        {notFound ? 'Aucun département ne correspond à ta recherche.' : ''}
      </Box>
    </Box>
  );
}
