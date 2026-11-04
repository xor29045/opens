import { useState, type FormEvent } from 'react';
import { Alert, Button, chakra, Field, Input, Stack, Text, Heading } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import { Section } from '../../components/ui';
import RecoveryCodesDialog from './RecoveryCodesDialog';
import { api } from '../../lib/api/client';
import { ApiError, describeError } from '../../lib/api/problem';
import { sessionStore, homeFor, type Role, type SessionUser } from '../../lib/auth/session';

export type AuthMode = 'login' | 'register';
// No client accounts (ADR-023).
type Kind = 'manager' | 'professional' | 'admin';

interface Props {
  kind: Kind;
  mode: AuthMode;
  next?: string | null;
}

interface AuthResponse {
  access_token: string;
  expires_at: string;
  user: SessionUser;
  recovery_codes?: string[];
}

const titles: Record<Kind, Record<AuthMode, string>> = {
  manager: { login: 'Connexion gérant', register: 'Inscrire mon établissement' },
  // Professionals do not self-register: their house creates their login (ADR-022).
  professional: { login: 'Connexion professionnel·le', register: '' },
  admin: { login: 'Administration', register: '' },
};

const roleOf: Record<Kind, Role> = { manager: 'MANAGER', professional: 'PROFESSIONAL', admin: 'ADMIN' };

function safeNext(next: string | null | undefined): string | null {
  return next && next.startsWith('/') && !next.startsWith('//') ? next : null;
}

function AuthInner({ kind, mode, next }: Props) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [codes, setCodes] = useState<string[] | null>(null);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setFieldErrors({});
    try {
      let res: AuthResponse;
      if (mode === 'login') {
        res = await api<AuthResponse>(kind === 'admin' ? '/auth/admin/login' : '/auth/login', {
          method: 'POST', body: { username, password }, auth: false,
        });
      } else {
        res = await api<AuthResponse>('/auth/register/manager', {
          method: 'POST', auth: false, body: { username, password },
        });
      }
      sessionStore.set({ token: res.access_token, user: res.user, expiresAt: res.expires_at });
      if (res.recovery_codes?.length) {
        setCodes(res.recovery_codes);
        return;
      }
      if (res.user.must_change_password) {
        window.location.replace('/compte/mot-de-passe');
        return;
      }
      const role = res.user.role;
      if (mode === 'login' && kind !== 'admin' && role !== roleOf[kind]) {
        // Logged in with another kind of account: send it to its own space.
        window.location.replace(homeFor[role]);
        return;
      }
      window.location.replace(safeNext(next) ?? homeFor[role]);
    } catch (err) {
      if (err instanceof ApiError) {
        const fe: Record<string, string> = {};
        for (const f of err.fields) fe[f.field] = describeError(new ApiError({ type: '', title: '', status: 422, errors: [f] }));
        setFieldErrors(fe);
      }
      setError(describeError(err));
    } finally {
      setBusy(false);
    }
  };

  if (codes) {
    return <RecoveryCodesDialog codes={codes} role={roleOf[kind]} next={safeNext(next)} />;
  }

  const title = titles[kind][mode];
  const isRegister = mode === 'register' && kind !== 'professional';
  const isPro = kind === 'professional';

  return (
    <chakra.form maxW="md" mx="auto" my={{ base: 2, md: 8 }} p={{ base: 6, md: 8 }} bg="bg.subtle" borderWidth="1px" borderColor="border" borderRadius="xl" onSubmit={submit} aria-labelledby="auth-title" noValidate>
    <Stack gap="6">
      <Heading id="auth-title" size="xl">{title}</Heading>
      {kind === 'admin' && <Alert.Root status="warning"><Alert.Content>Accès réservé aux administrateurs de la plateforme.</Alert.Content></Alert.Root>}
      {isPro && (
        <Text color="fg.muted">Connecte-toi avec le pseudo et le mot de passe que ta maison t’a donnés : tu arrives directement dans l’espace de ta maison.</Text>
      )}
      <Field.Root invalid={!!fieldErrors.username}>
        <Field.Label>Pseudo</Field.Label>
        <Input name="username" autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} required />
        {fieldErrors.username ? <Field.ErrorText>{fieldErrors.username}</Field.ErrorText> : isRegister && <Field.HelperText>3 à 30 caractères : lettres, chiffres, _ . -</Field.HelperText>}
      </Field.Root>
      <Field.Root invalid={!!fieldErrors.password}>
        <Field.Label>Mot de passe</Field.Label>
        <Input type="password" name="password" autoComplete={isRegister ? 'new-password' : 'current-password'} value={password} onChange={(e) => setPassword(e.target.value)} required />
        {fieldErrors.password ? <Field.ErrorText>{fieldErrors.password}</Field.ErrorText> : isRegister && <Field.HelperText>12 caractères minimum. Pas de règles de composition : une phrase longue est idéale.</Field.HelperText>}
      </Field.Root>
      {error && <Text color="danger" role="alert">{error}</Text>}
      <Button type="submit" colorPalette="brand" loading={busy} disabled={busy}>
        {isRegister ? 'Créer le compte' : 'Se connecter'}
      </Button>
      {!isRegister && kind !== 'admin' && (
        <Stack gap="1" fontSize="sm">
          {isPro
            ? <Text color="fg.muted">Mot de passe oublié ? Demande à ta maison de le réinitialiser.</Text>
            : <a href="/compte/recuperation">Mot de passe oublié ? (code de récupération)</a>}
          {kind === 'manager' && <a href="/gerant/register">Inscrire mon établissement</a>}
        </Stack>
      )}
      {isRegister && (
        <Text fontSize="sm" color="fg.muted">
          Déjà un compte ? <a href="/gerant/login">Connexion</a>
        </Text>
      )}
      {!isPro && <Section title="Bon à savoir">
        <Text fontSize="sm" color="fg.muted">
          Cette plateforme ne collecte aucune adresse e-mail. À l’inscription, tu reçois des codes de récupération à conserver.
        </Text>
      </Section>}
    </Stack>
    </chakra.form>
  );
}

export default function AuthForms(props: Props) {
  return (
    <Providers>
      <AuthInner {...props} />
    </Providers>
  );
}
