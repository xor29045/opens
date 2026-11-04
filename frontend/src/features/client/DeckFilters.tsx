import { useState, type FormEvent } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Box, Button, Field, Grid, HStack, Input, NativeSelect, Stack, Text, Wrap } from '@chakra-ui/react';
import { api } from '../../lib/api/client';
import { eyeColors, genders, hairColors, skinTones } from '../../lib/attributes';
import type { components } from '../../lib/api/schema';

export type Filters = components['schemas']['DiscoverFilters'];
type Region = components['schemas']['Region'];
type ListKey = 'gender' | 'hair_color' | 'eye_color' | 'skin_tone';
type RangeKey = 'age' | 'height' | 'price';

const KEY = 'os-deck-filters-v1';

// The filters are remembered in this browser, like the likes (ADR-023).
export function loadFilters(): Filters {
  if (typeof window === 'undefined') return {};
  try {
    const v = JSON.parse(window.localStorage.getItem(KEY) ?? '{}');
    return v && typeof v === 'object' ? (v as Filters) : {};
  } catch {
    return {};
  }
}

export function saveFilters(f: Filters) {
  try {
    window.localStorage.setItem(KEY, JSON.stringify(f));
  } catch {
    // Storage unavailable: the filters last for this visit.
  }
}

/** Removes empty values so the request and the counter stay clean. */
export function compact(f: Filters): Filters {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(f)) {
    if (Array.isArray(v) ? v.length > 0 : v !== undefined && v !== null && v !== '' && v !== 0) out[k] = v;
  }
  return out as Filters;
}

export function countFilters(f: Filters): number {
  const c = compact(f);
  const ranges = (['age', 'height', 'price'] as const).filter((r) => c[`${r}_min`] || c[`${r}_max`]).length;
  return ranges + (['city', 'gender', 'hair_color', 'eye_color', 'skin_tone'] as const).filter((k) => k in c).length;
}

const lists: { key: ListKey; label: string; options: [string, string][] }[] = [
  { key: 'gender', label: 'Sexe', options: genders },
  { key: 'hair_color', label: 'Cheveux', options: hairColors },
  { key: 'eye_color', label: 'Yeux', options: eyeColors },
  { key: 'skin_tone', label: 'Peau', options: skinTones },
];

const ranges: { key: RangeKey; label: string; unit: string; min: number; max: number }[] = [
  { key: 'age', label: 'Âge', unit: 'ans', min: 18, max: 99 },
  { key: 'height', label: 'Taille', unit: 'cm', min: 120, max: 230 },
  { key: 'price', label: 'Prix', unit: '€', min: 0, max: 100000 },
];

export default function DeckFilters({ value, onApply }: { value: Filters; onApply: (f: Filters) => void }) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState<Filters>(value);
  const regions = useQuery({
    queryKey: ['regions'],
    queryFn: ({ signal }) => api<{ items: Region[] }>('/regions', { auth: false, signal }),
    staleTime: 3_600_000,
  });
  const cities = (regions.data?.items ?? []).filter((r) => r.kind === 'CITY' && r.is_active).sort((a, b) => a.name.localeCompare(b.name, 'fr'));
  const active = countFilters(value);

  const toggle = (key: ListKey, v: string) => {
    const cur = draft[key] ?? [];
    setDraft({ ...draft, [key]: cur.includes(v) ? cur.filter((x) => x !== v) : [...cur, v] });
  };
  const setNum = (k: `${RangeKey}_${'min' | 'max'}`, raw: string) => {
    const n = raw === '' ? undefined : Math.max(0, Math.trunc(Number(raw)));
    setDraft({ ...draft, [k]: Number.isFinite(n) ? n : undefined });
  };
  const apply = (e: FormEvent) => {
    e.preventDefault();
    onApply(compact(draft));
    setOpen(false);
  };
  const reset = () => {
    setDraft({});
    onApply({});
    setOpen(false);
  };

  return (
    <Box w="full" maxW="560px">
      <HStack justify="center" gap="2">
        <Button size="sm" variant={active ? 'solid' : 'outline'} colorPalette="brand" onClick={() => { setDraft(value); setOpen(!open); }}
          aria-expanded={open} aria-controls="deck-filters">
          {open ? 'Fermer les filtres' : active ? `Filtres (${active})` : 'Filtres'}
        </Button>
        {active > 0 && !open && <Button size="sm" variant="ghost" onClick={reset}>Effacer</Button>}
      </HStack>
      {open && (
        <Box as="form" id="deck-filters" onSubmit={apply} mt="3" p="4" borderWidth="1px" borderColor="border" borderRadius="xl" bg="bg.subtle">
          <Stack gap="4">
            <Field.Root>
              <Field.Label>Ville</Field.Label>
              <NativeSelect.Root size="sm" disabled={regions.isPending}>
                <NativeSelect.Field value={draft.city ?? ''} onChange={(e) => setDraft({ ...draft, city: e.target.value || undefined })}>
                  <option value="">Toutes les villes</option>
                  {cities.map((c) => <option key={c.id} value={c.slug}>{c.name}</option>)}
                </NativeSelect.Field>
              </NativeSelect.Root>
            </Field.Root>
            {lists.map((l) => (
              <Box key={l.key} role="group" aria-label={l.label}>
                <Text fontSize="sm" fontWeight="medium" mb="1.5">{l.label}</Text>
                <Wrap gap="1.5">
                  {l.options.map(([v, label]) => {
                    const on = (draft[l.key] ?? []).includes(v);
                    return (
                      <Button key={v} size="xs" borderRadius="full" variant={on ? 'solid' : 'outline'} colorPalette={on ? 'brand' : 'gray'}
                        aria-pressed={on} onClick={() => toggle(l.key, v)}>
                        {label}
                      </Button>
                    );
                  })}
                </Wrap>
              </Box>
            ))}
            <Grid templateColumns={{ base: '1fr', sm: 'repeat(3, 1fr)' }} gap="3">
              {ranges.map((r) => (
                <Box key={r.key} role="group" aria-label={`${r.label} (${r.unit})`}>
                  <Text fontSize="sm" fontWeight="medium" mb="1.5">{r.label} ({r.unit})</Text>
                  <HStack gap="1.5">
                    <Input size="sm" type="number" inputMode="numeric" placeholder="min" min={r.min} max={r.max} aria-label={`${r.label} minimum`}
                      value={draft[`${r.key}_min`] ?? ''} onChange={(e) => setNum(`${r.key}_min`, e.target.value)} />
                    <Text color="fg.muted">–</Text>
                    <Input size="sm" type="number" inputMode="numeric" placeholder="max" min={r.min} max={r.max} aria-label={`${r.label} maximum`}
                      value={draft[`${r.key}_max`] ?? ''} onChange={(e) => setNum(`${r.key}_max`, e.target.value)} />
                  </HStack>
                </Box>
              ))}
            </Grid>
            <HStack justify="flex-end" gap="2">
              <Button size="sm" variant="ghost" onClick={reset}>Réinitialiser</Button>
              <Button size="sm" type="submit" colorPalette="brand">Appliquer</Button>
            </HStack>
          </Stack>
        </Box>
      )}
    </Box>
  );
}
