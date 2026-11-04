import { useState, type FormEvent } from 'react';
import { Button, chakra, Field, Heading, Input, Stack, Text } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import { PageLoader } from '../../components/ui';
import { useSession } from '../../lib/auth/useSession';
import { api } from '../../lib/api/client';
import { describeError } from '../../lib/api/problem';
import { sessionStore, homeFor } from '../../lib/auth/session';

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
    <chakra.form maxW="md" mx="auto" py="8" onSubmit={submit}>
    <Stack gap="5">
      <Heading size="xl">Changer mon mot de passe</Heading>
      {user.must_change_password && (
        <Text color="fg.muted">Ton mot de passe a été réinitialisé. Choisis-en un nouveau pour accéder à ton espace.</Text>
      )}
      <Field.Root>
        <Field.Label>Mot de passe actuel</Field.Label>
        <Input type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} required />
      </Field.Root>
      <Field.Root>
        <Field.Label>Nouveau mot de passe</Field.Label>
        <Input type="password" autoComplete="new-password" value={next} onChange={(e) => setNext(e.target.value)} required />
        <Field.HelperText>12 caractères minimum (16 pour un administrateur).</Field.HelperText>
      </Field.Root>
      <Field.Root>
        <Field.Label>Confirmer</Field.Label>
        <Input type="password" autoComplete="new-password" value={confirm} onChange={(e) => setConfirm(e.target.value)} required />
      </Field.Root>
      {error && <Text color="danger" role="alert">{error}</Text>}
      <Button type="submit" colorPalette="brand" loading={busy}>Enregistrer</Button>
    </Stack>
    </chakra.form>
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
      <Stack gap="4" maxW="md" mx="auto" py="8">
        <Heading size="xl">Mot de passe réinitialisé</Heading>
        <Text>Tes autres sessions ont été fermées. Connecte-toi avec ton nouveau mot de passe.</Text>
        <Stack gap="1"><a href="/gerant/login">Connexion gérant</a><a href="/pro/login">Connexion professionnel·le</a></Stack>
      </Stack>
    );
  }
  return (
    <chakra.form maxW="md" mx="auto" py="8" onSubmit={submit}>
    <Stack gap="5">
      <Heading size="xl">Mot de passe oublié</Heading>
      <Text color="fg.muted">
        Il n’y a pas d’e-mail : utilise un de tes codes de récupération (usage unique). Sans code, un professionnel·le s’adresse à son gérant ;
        un gérant contacte l’éditeur.
      </Text>
      <Field.Root>
        <Field.Label>Pseudo</Field.Label>
        <Input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required />
      </Field.Root>
      <Field.Root>
        <Field.Label>Code de récupération</Field.Label>
        <Input value={code} onChange={(e) => setCode(e.target.value)} placeholder="XXXX-XXXX-XXXX-XXXX" autoComplete="off" required />
      </Field.Root>
      <Field.Root>
        <Field.Label>Nouveau mot de passe</Field.Label>
        <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" required />
        <Field.HelperText>12 caractères minimum.</Field.HelperText>
      </Field.Root>
      {error && <Text color="danger" role="alert">{error}</Text>}
      <Button type="submit" colorPalette="brand" loading={busy}>Réinitialiser</Button>
    </Stack>
    </chakra.form>
  );
}

export function Recover() {
  return (
    <Providers>
      <RecoverInner />
    </Providers>
  );
}
