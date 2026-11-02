import { useEffect, useState, type FormEvent } from 'react';
import { Alert, Button, chakra, Checkbox, Field, Input, NativeSelect, Stack, Text, Heading } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import { Section } from '../../components/ui';
import RecoveryCodesDialog from './RecoveryCodesDialog';
import { api } from '../../lib/api/client';
import { ApiError, describeError } from '../../lib/api/problem';
import { sessionStore, homeFor, type Role, type SessionUser } from '../../lib/auth/session';

export type AuthMode = 'login' | 'register';
type Kind = 'client' | 'manager' | 'professional' | 'admin';

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

interface EstablishmentOption {
  slug: string;
  display_name: string;
  city: string | null;
  region: string | null;
}

const titles: Record<Kind, Record<AuthMode, string>> = {
  client: { login: 'Connexion client', register: 'Créer un compte client' },
  manager: { login: 'Connexion gérant', register: 'Inscrire mon établissement' },
  professional: { login: 'Connexion professionnel·le', register: 'Inscription professionnel·le' },
  admin: { login: 'Administration', register: '' },
};

const roleOf: Record<Kind, Role> = { client: 'CLIENT', manager: 'MANAGER', professional: 'PROFESSIONAL', admin: 'ADMIN' };

function safeNext(next: string | null | undefined): string | null {
  return next && next.startsWith('/') && !next.startsWith('//') ? next : null;
}

function AuthInner({ kind, mode, next }: Props) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [establishment, setEstablishment] = useState('');
  const [acceptTerms, setAcceptTerms] = useState(false);
  const [acceptCGV, setAcceptCGV] = useState(false);
  const [acceptDPA, setAcceptDPA] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [codes, setCodes] = useState<string[] | null>(null);
  const [houses, setHouses] = useState<EstablishmentOption[] | null>(null);
  const [houseError, setHouseError] = useState<string | null>(null);

  useEffect(() => {
    if (kind === 'professional' && mode === 'register') {
      api<{ items: EstablishmentOption[] }>('/auth/register/establishments', { auth: false })
        .then((r) => setHouses(r.items))
        .catch((e) => setHouseError(describeError(e)));
    }
  }, [kind, mode]);

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
      } else if (kind === 'client') {
        res = await api<AuthResponse>('/auth/register/client', {
          method: 'POST', auth: false, body: { username, password, accept_terms: acceptTerms },
        });
      } else if (kind === 'manager') {
        res = await api<AuthResponse>('/auth/register/manager', {
          method: 'POST', auth: false, body: { username, password, accept_terms: acceptTerms, accept_cgv: acceptCGV, accept_dpa: acceptDPA },
        });
      } else {
        res = await api<AuthResponse>('/auth/register/professional', {
          method: 'POST', auth: false,
          body: { username, password, accept_terms: acceptTerms, establishment_slug: establishment, display_name: displayName },
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
  const isRegister = mode === 'register';
  const invalidHouse = houses !== null && houses.length === 0;

  return (
    <chakra.form maxW="md" mx="auto" my={{ base: 2, md: 8 }} p={{ base: 6, md: 8 }} bg="bg.subtle" borderWidth="1px" borderColor="border" borderRadius="xl" onSubmit={submit} aria-labelledby="auth-title" noValidate>
    <Stack gap="6">
      <Heading id="auth-title" size="xl">{title}</Heading>
      {kind === 'admin' && <Alert.Root status="warning"><Alert.Content>Accès réservé aux administrateurs de la plateforme.</Alert.Content></Alert.Root>}
      {isRegister && kind === 'professional' && (
        <Field.Root invalid={!!fieldErrors.establishment_slug}>
          <Field.Label>Maison</Field.Label>
          <NativeSelect.Root disabled={!houses || invalidHouse}>
            <NativeSelect.Field value={establishment} onChange={(e) => setEstablishment(e.target.value)}>
              <option value="">{houses === null ? 'Chargement…' : 'Choisis ta maison'}</option>
              {(houses ?? []).map((h) => (
                <option key={h.slug} value={h.slug}>{h.display_name}{h.city ? ` — ${h.city}` : ''}</option>
              ))}
            </NativeSelect.Field>
          </NativeSelect.Root>
          {houseError && <Field.ErrorText>{houseError}</Field.ErrorText>}
          {invalidHouse && <Field.HelperText>Aucune maison n’accepte d’inscription pour le moment.</Field.HelperText>}
          {fieldErrors.establishment_slug && <Field.ErrorText>{fieldErrors.establishment_slug}</Field.ErrorText>}
          <Field.HelperText>Ton rattachement est définitif. Ton profil n’est publié qu’après validation de la maison.</Field.HelperText>
        </Field.Root>
      )}
      <Field.Root invalid={!!fieldErrors.username}>
        <Field.Label>Pseudo</Field.Label>
        <Input name="username" autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} required />
        {fieldErrors.username ? <Field.ErrorText>{fieldErrors.username}</Field.ErrorText> : isRegister && <Field.HelperText>3 à 30 caractères : lettres, chiffres, _ . -</Field.HelperText>}
      </Field.Root>
      {isRegister && kind === 'professional' && (
        <Field.Root invalid={!!fieldErrors.display_name}>
          <Field.Label>Nom affiché</Field.Label>
          <Input value={displayName} onChange={(e) => setDisplayName(e.target.value)} maxLength={60} />
          {fieldErrors.display_name && <Field.ErrorText>{fieldErrors.display_name}</Field.ErrorText>}
        </Field.Root>
      )}
      <Field.Root invalid={!!fieldErrors.password}>
        <Field.Label>Mot de passe</Field.Label>
        <Input type="password" name="password" autoComplete={isRegister ? 'new-password' : 'current-password'} value={password} onChange={(e) => setPassword(e.target.value)} required />
        {fieldErrors.password ? <Field.ErrorText>{fieldErrors.password}</Field.ErrorText> : isRegister && <Field.HelperText>12 caractères minimum. Pas de règles de composition : une phrase longue est idéale.</Field.HelperText>}
      </Field.Root>
      {isRegister && (
        <Stack gap="3">
          <Checkbox.Root checked={acceptTerms} onCheckedChange={(d) => setAcceptTerms(d.checked === true)} invalid={!!fieldErrors.accept_terms}>
            <Checkbox.HiddenInput />
            <Checkbox.Control />
            <Checkbox.Label>J’ai 18 ans ou plus et j’accepte les <a href="/legal/cgu">CGU</a>.</Checkbox.Label>
          </Checkbox.Root>
          {kind === 'manager' && (
            <>
              <Checkbox.Root checked={acceptCGV} onCheckedChange={(d) => setAcceptCGV(d.checked === true)}>
                <Checkbox.HiddenInput />
                <Checkbox.Control />
                <Checkbox.Label>J’accepte les <a href="/legal/cgv">CGV</a>.</Checkbox.Label>
              </Checkbox.Root>
              <Checkbox.Root checked={acceptDPA} onCheckedChange={(d) => setAcceptDPA(d.checked === true)}>
                <Checkbox.HiddenInput />
                <Checkbox.Control />
                <Checkbox.Label>J’accepte le <a href="/legal/dpa">contrat de sous-traitance (DPA)</a> pour les données de mes salarié·es.</Checkbox.Label>
              </Checkbox.Root>
            </>
          )}
        </Stack>
      )}
      {error && <Text color="danger" role="alert">{error}</Text>}
      <Button type="submit" colorPalette="brand" loading={busy} disabled={busy || (isRegister && kind === 'professional' && !establishment)}>
        {isRegister ? 'Créer le compte' : 'Se connecter'}
      </Button>
      {!isRegister && kind !== 'admin' && (
        <Stack gap="1" fontSize="sm">
          <a href="/compte/recuperation">Mot de passe oublié ? (code de récupération)</a>
          {kind === 'client' && <a href="/client/register">Pas encore de compte ? Créer un compte client</a>}
          {kind === 'manager' && <a href="/gerant/register">Inscrire mon établissement</a>}
          {kind === 'professional' && <a href="/pro/register">Créer mon compte professionnel·le</a>}
        </Stack>
      )}
      {isRegister && (
        <Text fontSize="sm" color="fg.muted">
          Déjà un compte ? <a href={`/${kind === 'client' ? 'client' : kind === 'manager' ? 'gerant' : 'pro'}/login`}>Connexion</a>
        </Text>
      )}
      <Section title="Bon à savoir">
        <Text fontSize="sm" color="fg.muted">
          Cette plateforme ne collecte aucune adresse e-mail. À l’inscription, tu reçois des codes de récupération à conserver.
        </Text>
      </Section>
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
