import { useState, type FormEvent, type ReactNode } from 'react';
import { Box, Button, chakra, Field, Heading, Input, Stack, Text } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import { PageLoader } from '../../components/ui';
import { useSession } from '../../lib/auth/useSession';
import { api } from '../../lib/api/client';
import { describeError } from '../../lib/api/problem';
import { sessionStore, homeFor } from '../../lib/auth/session';

/**
 * Conteneur commun des pages d'authentification.
 * Mobile : pleine largeur avec marges. Tablette+ : carte centrée.
 */
function FormCard({ onSubmit, children }: { onSubmit?: (e: FormEvent) => void; children: ReactNode }) {
  const card = {
    w: 'full',
    maxW: { base: 'full', sm: 'md', md: 'lg' },
    bg: { base: 'transparent', md: 'bg.subtle' },
    borderWidth: { base: '0', md: '1px' },
    borderColor: 'border',
    borderRadius: { base: 'none', md: '2xl' },
    p: { base: '0', md: '8' },
  } as const;

  return (
    <Box
      display="flex"
      justifyContent="center"
      alignItems={{ base: 'flex-start', md: 'center' }}
      minH={{ base: 'auto', md: '70vh' }}
      pt={{ base: '6', md: '12' }}
      pb={{ base: '10', md: '12' }}
      pl="calc(env(safe-area-inset-left, 0px) + 16px)"
      pr="calc(env(safe-area-inset-right, 0px) + 16px)"
    >
      {onSubmit ? (
        <chakra.form onSubmit={onSubmit} {...card}>{children}</chakra.form>
      ) : (
        <Box {...card}>{children}</Box>
      )}
    </Box>
  );
}

function ChangePasswordInner() {
  const { status, user } = useSession();
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [confirm, setConfirm] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (status === 'loading') return <PageLoader />;
  if (status === 'anonymous' || !user) {
    if (typeof window !== 'undefined') window.location.replace('/gerant/login');
    return null;
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    if (next !== confirm) {
      setError('Les deux mots de passe ne correspondent pas.');
      return;
    }
    setBusy(true);
    try {
      await api('/auth/password', { method: 'PUT', body: { current_password: current, new_password: next } });
      sessionStore.set({ ...sessionStore.get(), user: { ...user, must_change_password: false } });
      window.location.replace(homeFor[user.role]);
    } catch (err) {
      setError(describeError(err));
      setBusy(false);
    }
  };

  return (
    <FormCard onSubmit={submit}>
      <Stack gap="5">
        <Heading size={{ base: 'xl', md: '2xl' }}>Changer mon mot de passe</Heading>
        {user.must_change_password && (
          <Text color="fg.muted">Ton mot de passe a été réinitialisé. Choisis-en un nouveau pour accéder à ton espace.</Text>
        )}
        <Field.Root>
          <Field.Label>Mot de passe actuel</Field.Label>
          <Input size="lg" type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} required />
        </Field.Root>
        <Field.Root>
          <Field.Label>Nouveau mot de passe</Field.Label>
          <Input size="lg" type="password" autoComplete="new-password" value={next} onChange={(e) => setNext(e.target.value)} required />
          <Field.HelperText>12 caractères minimum (16 pour un administrateur).</Field.HelperText>
        </Field.Root>
        <Field.Root>
          <Field.Label>Confirmer</Field.Label>
          <Input size="lg" type="password" autoComplete="new-password" value={confirm} onChange={(e) => setConfirm(e.target.value)} required />
        </Field.Root>
        {error && <Text color="danger" role="alert">{error}</Text>}
        <Button type="submit" size="lg" colorPalette="brand" loading={busy}>Enregistrer</Button>
      </Stack>
    </FormCard>
  );
}

export function ChangePassword() {
  return (
    <Providers>
      <ChangePasswordInner />
    </Providers>
  );
}

function RecoverInner() {
  const [username, setUsername] = useState('');
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api('/auth/password/recover', { method: 'POST', auth: false, body: { username, recovery_code: code, new_password: password } });
      setDone(true);
    } catch (err) {
      setError(describeError(err));
    } finally {
      setBusy(false);
    }
  };

  if (done) {
    return (
      <FormCard>
        <Stack gap="4">
          <Heading size={{ base: 'xl', md: '2xl' }}>Mot de passe réinitialisé</Heading>
          <Text color="fg.muted">Tes autres sessions ont été fermées. Connecte-toi avec ton nouveau mot de passe.</Text>
          <Stack gap="3" direction={{ base: 'column', sm: 'row' }}>
            <Button asChild size="lg" colorPalette="brand" flex="1">
              <a href="/gerant/login">Connexion gérant</a>
            </Button>
            <Button asChild size="lg" variant="outline" flex="1">
              <a href="/pro/login">Connexion professionnel·le</a>
            </Button>
          </Stack>
        </Stack>
      </FormCard>
    );
  }

  return (
    <FormCard onSubmit={submit}>
      <Stack gap="5">
        <Heading size={{ base: 'xl', md: '2xl' }}>Mot de passe oublié</Heading>
        <Text color="fg.muted">
          Il n’y a pas d’e-mail : utilise un de tes codes de récupération (usage unique). Sans code, un professionnel·le s’adresse à son gérant ;
          un gérant contacte l’éditeur.
        </Text>
        <Field.Root>
          <Field.Label>Pseudo</Field.Label>
          <Input size="lg" value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required />
        </Field.Root>
        <Field.Root>
          <Field.Label>Code de récupération</Field.Label>
          <Input size="lg" value={code} onChange={(e) => setCode(e.target.value)} placeholder="XXXX-XXXX-XXXX-XXXX" autoComplete="off" required />
        </Field.Root>
        <Field.Root>
          <Field.Label>Nouveau mot de passe</Field.Label>
          <Input size="lg" type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" required />
          <Field.HelperText>12 caractères minimum.</Field.HelperText>
        </Field.Root>
        {error && <Text color="danger" role="alert">{error}</Text>}
        <Button type="submit" size="lg" colorPalette="brand" loading={busy}>Réinitialiser</Button>
      </Stack>
    </FormCard>
  );
}

export function Recover() {
  return (
    <Providers>
      <RecoverInner />
    </Providers>
  );
}
