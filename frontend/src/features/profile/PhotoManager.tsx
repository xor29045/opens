import { useState } from 'react';
import { Button, Box, Grid, HStack, Image, Stack, Text, Badge } from '@chakra-ui/react';
import { Section } from '../../components/ui';
import { api } from '../../lib/api/client';
import { describeError } from '../../lib/api/problem';
import type { Photo } from './profileApi';

const MAX_BYTES = 10 * 1024 * 1024;

/** Upload, primary photo, ordering and deletion (RM-26). The server re-encodes every image. */
export default function PhotoManager({ base, photos, onChange }: { base: string; photos: Photo[]; onChange: () => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const ordered = [...photos].sort((a, b) => a.position - b.position);

  const act = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
      onChange();
    } catch (e) {
      setError(describeError(e));
    } finally {
      setBusy(false);
    }
  };

  const upload = (file: File | undefined) => {
    if (!file) return;
    if (file.size > MAX_BYTES) {
      setError('Fichier trop volumineux (10 Mo maximum).');
      return;
    }
    const form = new FormData();
    form.append('file', file);
    return act(() => api(`${base}/photos`, { method: 'POST', formData: form, timeoutMs: 60_000 }));
  };

  const move = (index: number, dir: -1 | 1) => {
    const next = [...ordered];
    const target = index + dir;
    if (target < 0 || target >= next.length) return;
    [next[index], next[target]] = [next[target]!, next[index]!];
    return act(() => api(`${base}/photos/order`, { method: 'PUT', body: { photo_ids: next.map((p) => p.id) } }));
  };

  return (
    <Section title={`Photos (${photos.length}/12)`}>
      <Stack gap="4">
        <Text fontSize="sm" color="fg.muted">
          1 photo principale obligatoire pour publier. JPEG, PNG ou WebP, 10 Mo maximum. Les métadonnées (dont la géolocalisation) sont supprimées.
        </Text>
        <Box>
          <label>
            <span style={{ display: 'inline-block', marginBottom: 8 }}>Ajouter une photo</span>
            <input
              type="file"
              accept="image/jpeg,image/png,image/webp"
              disabled={busy || photos.length >= 12}
              onChange={(e) => upload(e.target.files?.[0])}
              aria-label="Ajouter une photo"
            />
          </label>
        </Box>
        {photos.length > 0 && (
          <Grid templateColumns={{ base: 'repeat(2, 1fr)', md: 'repeat(4, 1fr)' }} gap="3">
            {ordered.map((p, i) => (
              <Stack key={p.id} borderWidth="1px" borderColor="border" borderRadius="md" p="2" gap="2">
                <Box aspectRatio={3 / 4} bg="bg.subtle" borderRadius="sm" overflow="hidden">
                  <Image src={p.url} alt={`Photo ${i + 1}`} width="100%" height="100%" objectFit="cover" />
                </Box>
                <HStack justify="space-between">
                  {p.is_primary ? <Badge colorPalette="brand">Principale</Badge> : (
                    <Button size="xs" variant="outline" disabled={busy} onClick={() => act(() => api(`${base}/photos/${p.id}`, { method: 'PATCH', body: { is_primary: true } }))}>
                      Définir principale
                    </Button>
                  )}
                </HStack>
                <HStack justify="space-between">
                  <Button size="xs" variant="ghost" aria-label={`Monter la photo ${i + 1}`} disabled={busy || i === 0} onClick={() => move(i, -1)}>↑</Button>
                  <Button size="xs" variant="ghost" aria-label={`Descendre la photo ${i + 1}`} disabled={busy || i === ordered.length - 1} onClick={() => move(i, 1)}>↓</Button>
                  <Button size="xs" variant="ghost" colorPalette="red" aria-label={`Supprimer la photo ${i + 1}`} disabled={busy} onClick={() => act(() => api(`${base}/photos/${p.id}`, { method: 'DELETE' }))}>Supprimer</Button>
                </HStack>
              </Stack>
            ))}
          </Grid>
        )}
        {error && <Text color="danger" role="alert">{error}</Text>}
      </Stack>
    </Section>
  );
}
