import { useState } from 'react';
import { Button, Checkbox, Code, SimpleGrid, Stack, Text } from '@chakra-ui/react';
import { api } from '../../lib/api/client';
import { describeError } from '../../lib/api/problem';
import { homeFor, type Role } from '../../lib/auth/session';

/**
 * Shown once after registration (RM-04). The codes cannot be displayed again:
 * the user must confirm they saved them before entering the space.
 */
export default function RecoveryCodesDialog({ codes, role, next }: { codes: string[]; role: Role; next?: string | null }) {
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const download = () => {
    const blob = new Blob([`Opensex — codes de récupération (usage unique)\n\n${codes.join('\n')}\n`], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'opensex-codes-de-recuperation.txt';
    a.click();
    URL.revokeObjectURL(url);
  };

  const confirm = async () => {
    setBusy(true);
    setError(null);
    try {
      await api('/auth/recovery-codes/confirm', { method: 'POST' });
      window.location.replace(next && next.startsWith('/') ? next : homeFor[role]);
    } catch (e) {
      setError(describeError(e));
      setBusy(false);
    }
  };

  return (
    <Stack gap="4" role="dialog" aria-labelledby="recovery-title">
      <Text id="recovery-title" fontWeight="semibold" fontSize="lg">Tes codes de récupération</Text>
      <Text>
        Il n’y a pas d’e-mail sur cette plateforme : ces 10 codes sont le seul moyen de réinitialiser ton mot de passe.
        Chacun ne sert qu’une fois. Ils ne seront plus jamais affichés.
      </Text>
      <SimpleGrid columns={2} gap="2">
        {codes.map((c) => (
          <Code key={c} fontSize="md" p="2" textAlign="center" userSelect="all">{c}</Code>
        ))}
      </SimpleGrid>
      <Button variant="outline" onClick={download}>Télécharger les codes</Button>
      <Checkbox.Root checked={saved} onCheckedChange={(d) => setSaved(d.checked === true)}>
        <Checkbox.HiddenInput />
        <Checkbox.Control />
        <Checkbox.Label>J’ai enregistré ces codes en lieu sûr.</Checkbox.Label>
      </Checkbox.Root>
      {error && <Text color="danger" role="alert">{error}</Text>}
      <Button colorPalette="brand" disabled={!saved || busy} onClick={confirm}>Continuer</Button>
    </Stack>
  );
}
