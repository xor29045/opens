import { useEffect, useState } from 'react';
import { useForm, type SubmitHandler } from 'react-hook-form';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { Button, Field, Grid, Heading, HStack, Input, NativeSelect, Stack, Text, Textarea, Checkbox, Box } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import { Section, StatusBadge } from '../../components/ui';
import { QueryBlock, ActionDialog } from '../dashboard/shared';
import { api } from '../../lib/api/client';
import { ApiError, describeError, messageFor } from '../../lib/api/problem';
import { zodResolver } from '../../lib/forms';
import { reasonLabels } from '../../lib/format';
import { profileBase, profileLabels, contactTypes, visibilities, type Profile, type Category, type Contact } from './profileApi';
import PhotoManager from './PhotoManager';
import AttestationForm from './AttestationForm';

const text = (max: number) => z.string().trim().max(max, 'too_long');
const profileSchema = z.object({
  display_name: z.string().trim().min(1, 'required').max(60, 'too_long'),
  headline: text(120),
  description: text(5000),
  height_cm: z.string().refine((v) => v === '' || (Number(v) >= 120 && Number(v) <= 230), 'out_of_range'),
  weight_kg: z.string().refine((v) => v === '' || (Number(v) >= 35 && Number(v) <= 250), 'out_of_range'),
  hair_color: z.string(),
  eye_color: z.string(),
  body_type: z.string(),
  languages: z.string(),
  gender: z.string(),
  tattoos: z.string(),
  piercings: z.string(),
  smoker: z.string(),
});
type FormValues = z.infer<typeof profileSchema>;

const rateSchema = z.array(z.object({
  label: z.string().trim().min(1, 'required').max(60, 'too_long'),
  duration: z.string(),
  price: z.string().refine((v) => Number(v.replace(',', '.')) > 0, 'out_of_range'),
})).min(1, 'must_contain_1_to_15_items').max(15, 'must_contain_1_to_15_items');

function FieldError({ msg }: { msg?: string }) {
  return msg ? <Field.ErrorText>{msg}</Field.ErrorText> : null;
}

function toForm(p: Profile): FormValues {
  const attrs = p.extra_attributes as Record<string, string>;
  return {
    display_name: p.display_name, headline: p.headline, description: p.description,
    height_cm: p.height_cm ? String(p.height_cm) : '', weight_kg: p.weight_kg ? String(p.weight_kg) : '',
    hair_color: p.hair_color ?? '', eye_color: p.eye_color ?? '', body_type: p.body_type ?? '',
    languages: p.languages.join(', '),
    gender: attrs.gender ?? '', tattoos: attrs.tattoos ?? '', piercings: attrs.piercings ?? '', smoker: attrs.smoker ?? '',
  };
}

function extraFrom(v: FormValues): Record<string, string> {
  const out: Record<string, string> = {};
  for (const k of ['gender', 'tattoos', 'piercings', 'smoker'] as const) if (v[k]) out[k] = v[k];
  return out;
}

function ProfileForm({ mode, id, profile }: { mode: 'manager' | 'professional'; id?: string; profile: Profile }) {
  const qc = useQueryClient();
  const base = profileBase(mode, id);
  const key = ['profile', mode, id ?? 'me'];
  const refresh = () => qc.invalidateQueries({ queryKey: key });
  const [saved, setSaved] = useState<string | null>(null);
  const [serverError, setServerError] = useState<string | null>(null);

  const form = useForm<FormValues>({ defaultValues: toForm(profile), resolver: zodResolver(profileSchema) });
  const { register, handleSubmit, reset, setError, formState: { errors, isDirty } } = form;
  useEffect(() => reset(toForm(profile)), [profile, reset]);

  const save: SubmitHandler<FormValues> = async (v) => {
    setSaved(null);
    setServerError(null);
    try {
      await api(base, {
        method: 'PUT',
        body: {
          display_name: v.display_name, headline: v.headline, description: v.description,
          height_cm: v.height_cm ? Number(v.height_cm) : null, weight_kg: v.weight_kg ? Number(v.weight_kg) : null,
          hair_color: v.hair_color || null, eye_color: v.eye_color || null, body_type: v.body_type || null,
          languages: v.languages.split(',').map((s) => s.trim().toLowerCase()).filter(Boolean),
          extra_attributes: extraFrom(v),
        },
      });
      setSaved('Modifications enregistrées.');
      refresh();
    } catch (e) {
      if (e instanceof ApiError) {
        for (const f of e.fields) {
          const name = f.field === 'languages' ? 'languages' : f.field;
          if (name in toForm(profile)) setError(name as keyof FormValues, { type: f.code, message: f.code });
        }
      }
      setServerError(e instanceof ApiError ? (e.fields.length ? 'Vérifie les champs signalés.' : e.message) : 'Erreur réseau.');
    }
  };

  return (
    <Stack gap="6">
      <Section title="Publication" action={<StatusBadge value={profile.status} />}>
        <Stack gap="2">
          {profile.publication.published ? (
            <Text color="success">Ce profil est publié{profile.publication.published_at ? ` depuis le ${new Date(profile.publication.published_at).toLocaleDateString('fr-BE')}` : ''}.</Text>
          ) : (
            <>
              <Text color="fg.muted">Non publié. Raisons :</Text>
              <ul>
                {profile.publication.reasons.map((r) => <li key={r}>{reasonLabels[r] ?? r}</li>)}
              </ul>
            </>
          )}
          {profile.is_hidden && <Text color="danger">Masqué par la plateforme{profile.hidden_reason ? ` : ${profile.hidden_reason}` : ''}.</Text>}
          {profile.last_edit.by && <Text fontSize="xs" color="fg.muted">Dernière modification par {profile.last_edit.by} ({profile.last_edit.role}).</Text>}
        </Stack>
      </Section>

      <Section title="Identité et présentation">
        <form onSubmit={handleSubmit(save)} noValidate>
          <Grid templateColumns={{ base: '1fr', md: '1fr 1fr' }} gap="4">
            <Field.Root invalid={!!errors.display_name} required>
              <Field.Label>Nom affiché</Field.Label>
              <Input {...register('display_name')} maxLength={60} />
              <FieldError msg={errors.display_name && messageFor(errors.display_name.message ?? '')} />
            </Field.Root>
            <Field.Root invalid={!!errors.headline}>
              <Field.Label>Accroche</Field.Label>
              <Input {...register('headline')} maxLength={120} />
              <FieldError msg={errors.headline && messageFor(errors.headline.message ?? '')} />
            </Field.Root>
            <Field.Root invalid={!!errors.height_cm}>
              <Field.Label>Taille (cm)</Field.Label>
              <Input type="number" inputMode="numeric" {...register('height_cm')} />
              <FieldError msg={errors.height_cm && messageFor(errors.height_cm.message ?? '')} />
            </Field.Root>
            <Field.Root invalid={!!errors.weight_kg}>
              <Field.Label>Poids (kg)</Field.Label>
              <Input type="number" inputMode="numeric" {...register('weight_kg')} />
              <FieldError msg={errors.weight_kg && messageFor(errors.weight_kg.message ?? '')} />
            </Field.Root>
            <Field.Root>
              <Field.Label>Cheveux</Field.Label>
              <NativeSelect.Root><NativeSelect.Field {...register('hair_color')}>
                <option value="">—</option>
                {['BLONDE', 'BROWN', 'BLACK', 'RED', 'GREY', 'OTHER'].map((v) => <option key={v} value={v}>{v}</option>)}
              </NativeSelect.Field></NativeSelect.Root>
            </Field.Root>
            <Field.Root>
              <Field.Label>Yeux</Field.Label>
              <NativeSelect.Root><NativeSelect.Field {...register('eye_color')}>
                <option value="">—</option>
                {['BLUE', 'GREEN', 'BROWN', 'HAZEL', 'GREY', 'BLACK', 'OTHER'].map((v) => <option key={v} value={v}>{v}</option>)}
              </NativeSelect.Field></NativeSelect.Root>
            </Field.Root>
            <Field.Root>
              <Field.Label>Morphologie</Field.Label>
              <NativeSelect.Root><NativeSelect.Field {...register('body_type')}>
                <option value="">—</option>
                {['SLIM', 'ATHLETIC', 'AVERAGE', 'CURVY', 'PLUS_SIZE', 'MUSCULAR'].map((v) => <option key={v} value={v}>{v}</option>)}
              </NativeSelect.Field></NativeSelect.Root>
            </Field.Root>
            <Field.Root>
              <Field.Label>Langues (codes ISO, séparés par des virgules)</Field.Label>
              <Input {...register('languages')} placeholder="fr, nl, en" />
            </Field.Root>
            <Field.Root>
              <Field.Label>Genre</Field.Label>
              <NativeSelect.Root><NativeSelect.Field {...register('gender')}>
                <option value="">Non précisé</option>
                <option value="FEMALE">Femme</option><option value="MALE">Homme</option>
                <option value="TRANS_WOMAN">Femme trans</option><option value="TRANS_MAN">Homme trans</option>
                <option value="NON_BINARY">Non binaire</option>
              </NativeSelect.Field></NativeSelect.Root>
            </Field.Root>
            <Field.Root>
              <Field.Label>Tatouages</Field.Label>
              <NativeSelect.Root><NativeSelect.Field {...register('tattoos')}>
                <option value="">Non précisé</option><option value="true">Oui</option><option value="false">Non</option>
              </NativeSelect.Field></NativeSelect.Root>
            </Field.Root>
            <Field.Root>
              <Field.Label>Piercings</Field.Label>
              <NativeSelect.Root><NativeSelect.Field {...register('piercings')}>
                <option value="">Non précisé</option><option value="true">Oui</option><option value="false">Non</option>
              </NativeSelect.Field></NativeSelect.Root>
            </Field.Root>
            <Field.Root>
              <Field.Label>Fumeur·se</Field.Label>
              <NativeSelect.Root><NativeSelect.Field {...register('smoker')}>
                <option value="">Non précisé</option><option value="true">Oui</option><option value="false">Non</option>
              </NativeSelect.Field></NativeSelect.Root>
            </Field.Root>
          </Grid>
          <Field.Root mt="4" invalid={!!errors.description}>
            <Field.Label>Description</Field.Label>
            <Textarea rows={6} {...register('description')} maxLength={5000} />
            <Field.HelperText>Texte brut. Les liens et le code HTML ne sont pas interprétés.</Field.HelperText>
          </Field.Root>
          <HStack mt="4" gap="3">
            <Button type="submit" colorPalette="brand" disabled={!isDirty} loading={form.formState.isSubmitting}>Enregistrer</Button>
            {saved && <Text color="success" role="status">{saved}</Text>}
          </HStack>
          {serverError && <Text color="danger" role="alert" mt="2">{serverError}</Text>}
        </form>
      </Section>

      <PhotoManager base={base} photos={profile.photos} onChange={refresh} />
      <RatesSection base={base} rates={profile.rates} onChange={refresh} />
      <CategoriesSection base={base} selected={profile.categories} onChange={refresh} />
      <ContactsSection base={base} contacts={profile.contacts} onChange={refresh} />
      {mode === 'manager' && id && <AttestationForm profile={profile} base={base} onChange={refresh} />}
      {mode === 'manager' && id && <ManagerLifecycle profile={profile} base={base} onChange={refresh} />}
    </Stack>
  );
}

function RatesSection({ base, rates, onChange }: { base: string; rates: Profile['rates']; onChange: () => void }) {
  const [rows, setRows] = useState(() => rates.map((r) => ({ label: r.label, duration: r.duration_minutes ? String(r.duration_minutes) : '', price: String(r.amount_cents / 100) })));
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  useEffect(() => setRows(rates.map((r) => ({ label: r.label, duration: r.duration_minutes ? String(r.duration_minutes) : '', price: String(r.amount_cents / 100) }))), [rates]);
  const save = async () => {
    setError(null);
    setSaved(false);
    const parsed = rateSchema.safeParse(rows);
    if (!parsed.success) {
      setError(messageFor(parsed.error.issues[0]?.message ?? ''));
      return;
    }
    try {
      await api(`${base}/rates`, {
        method: 'PUT',
        body: { rates: rows.map((r) => ({
          label: r.label.trim(), duration_minutes: r.duration ? Number(r.duration) : null,
          amount_cents: Math.round(Number(r.price.replace(',', '.')) * 100),
        })) },
      });
      setSaved(true);
      onChange();
    } catch (e) {
      setError(e instanceof ApiError && e.fields[0] ? messageFor(e.fields[0].code) : 'Enregistrement impossible.');
    }
  };
  return (
    <Section title="Tarifs" action={<Button size="xs" variant="outline" onClick={() => setRows([...rows, { label: '', duration: '', price: '' }])} disabled={rows.length >= 15}>Ajouter</Button>}>
      <Stack gap="3">
        {rows.length === 0 && <Text color="fg.muted">Aucun tarif. Au moins un tarif est requis pour publier ton profil.</Text>}
        {rows.map((r, i) => (
          <HStack key={i} align="flex-end" wrap="wrap">
            <Field.Root w={{ base: 'full', md: '48' }}>
              <Field.Label>Prestation</Field.Label>
              <Input value={r.label} maxLength={60} onChange={(e) => setRows(rows.map((x, j) => (j === i ? { ...x, label: e.target.value } : x)))} />
            </Field.Root>
            <Field.Root w="28">
              <Field.Label>Durée (min)</Field.Label>
              <Input type="number" value={r.duration} onChange={(e) => setRows(rows.map((x, j) => (j === i ? { ...x, duration: e.target.value } : x)))} />
            </Field.Root>
            <Field.Root w="32">
              <Field.Label>Montant (€)</Field.Label>
              <Input inputMode="decimal" value={r.price} onChange={(e) => setRows(rows.map((x, j) => (j === i ? { ...x, price: e.target.value } : x)))} />
            </Field.Root>
            <Button size="sm" variant="ghost" aria-label={`Supprimer le tarif ${i + 1}`} onClick={() => setRows(rows.filter((_, j) => j !== i))}>Retirer</Button>
          </HStack>
        ))}
        <HStack>
          <Button size="sm" colorPalette="brand" onClick={save}>Enregistrer les tarifs</Button>
          {saved && <Text color="success" role="status">Tarifs enregistrés.</Text>}
        </HStack>
        {error && <Text color="danger" role="alert">{error}</Text>}
      </Stack>
    </Section>
  );
}

function CategoriesSection({ base, selected, onChange }: { base: string; selected: Category[]; onChange: () => void }) {
  const all = useQuery({ queryKey: ['categories'], queryFn: ({ signal }) => api<{ items: Category[] }>('/categories', { auth: false, signal }) });
  const [ids, setIds] = useState<string[]>(selected.map((c) => c.id));
  const [error, setError] = useState<string | null>(null);
  useEffect(() => setIds(selected.map((c) => c.id)), [selected]);
  const save = async () => {
    setError(null);
    try {
      await api(`${base}/categories`, { method: 'PUT', body: { category_ids: ids } });
      onChange();
    } catch (e) {
      setError(e instanceof ApiError && e.fields[0] ? messageFor(e.fields[0].code) : 'Enregistrement impossible.');
    }
  };
  return (
    <Section title="Services">
      <QueryBlock isPending={all.isPending} isError={all.isError} error={all.error} onRetry={() => all.refetch()}>
        <Stack gap="3">
          <Stack gap="2">
            {(all.data?.items ?? []).filter((c) => c.parent_id).map((c) => (
              <Checkbox.Root key={c.id} checked={ids.includes(c.id)} onCheckedChange={(d) => setIds(d.checked ? [...ids, c.id] : ids.filter((x) => x !== c.id))}>
                <Checkbox.HiddenInput />
                <Checkbox.Control />
                <Checkbox.Label>{c.name}</Checkbox.Label>
              </Checkbox.Root>
            ))}
          </Stack>
          <HStack><Button size="sm" colorPalette="brand" onClick={save}>Enregistrer les services</Button></HStack>
          {error && <Text color="danger" role="alert">{error}</Text>}
        </Stack>
      </QueryBlock>
    </Section>
  );
}

function ContactsSection({ base, contacts, onChange }: { base: string; contacts: Contact[]; onChange: () => void }) {
  const [rows, setRows] = useState(() => contacts.map((c) => ({ type: c.type, value: c.value, visibility: c.visibility })));
  const [error, setError] = useState<string | null>(null);
  useEffect(() => setRows(contacts.map((c) => ({ type: c.type, value: c.value, visibility: c.visibility }))), [contacts]);
  const save = async () => {
    setError(null);
    try {
      await api(`${base}/contacts`, { method: 'PUT', body: { contacts: rows.filter((r) => r.value.trim()) } });
      onChange();
    } catch (e) {
      setError(e instanceof ApiError && e.fields[0] ? `${messageFor(e.fields[0].code)}` : 'Enregistrement impossible.');
    }
  };
  return (
    <Section title="Coordonnées" action={<Button size="xs" variant="outline" onClick={() => setRows([...rows, { type: 'PHONE', value: '', visibility: 'CLIENTS' }])}>Ajouter</Button>}>
      <Stack gap="3">
        <Text fontSize="sm" color="fg.muted">
          Public : visible de tous · Clients inscrits : visible des comptes client · Masqué : jamais affiché.
        </Text>
        {rows.map((r, i) => (
          <HStack key={i} align="flex-end" wrap="wrap">
            <Field.Root w="40">
              <Field.Label>Type</Field.Label>
              <NativeSelect.Root><NativeSelect.Field value={r.type} onChange={(e) => setRows(rows.map((x, j) => (j === i ? { ...x, type: e.target.value as typeof x.type } : x)))}>
                {contactTypes.map((t) => <option key={t} value={t}>{profileLabels.contactType[t]}</option>)}
              </NativeSelect.Field></NativeSelect.Root>
            </Field.Root>
            <Field.Root w={{ base: 'full', md: '64' }}>
              <Field.Label>Valeur</Field.Label>
              <Input value={r.value} onChange={(e) => setRows(rows.map((x, j) => (j === i ? { ...x, value: e.target.value } : x)))} />
            </Field.Root>
            <Field.Root w="44">
              <Field.Label>Visibilité</Field.Label>
              <NativeSelect.Root><NativeSelect.Field value={r.visibility} onChange={(e) => setRows(rows.map((x, j) => (j === i ? { ...x, visibility: e.target.value as typeof x.visibility } : x)))}>
                {visibilities.map((v) => <option key={v} value={v}>{profileLabels.visibility[v]}</option>)}
              </NativeSelect.Field></NativeSelect.Root>
            </Field.Root>
            <Button size="sm" variant="ghost" aria-label={`Supprimer la coordonnée ${i + 1}`} onClick={() => setRows(rows.filter((_, j) => j !== i))}>Retirer</Button>
          </HStack>
        ))}
        <HStack><Button size="sm" colorPalette="brand" onClick={save}>Enregistrer les coordonnées</Button></HStack>
        {error && <Text color="danger" role="alert">{error}</Text>}
      </Stack>
    </Section>
  );
}

function ManagerLifecycle({ profile, base, onChange }: { profile: Profile; base: string; onChange: () => void }) {
  const [error, setError] = useState<string | null>(null);
  const run = async (path: string, method: 'POST' | 'DELETE', body?: unknown) => {
    setError(null);
    try {
      await api(`${base}${path}`, { method, body });
      onChange();
    } catch (e) {
      setError(describeError(e));
      throw e;
    }
  };
  const canActivate = profile.attestation.attested && (profile.status === 'DRAFT' || profile.status === 'ARCHIVED');
  return (
    <Section title="Cycle de vie">
      <Stack gap="3">
        <HStack wrap="wrap" gap="2">
          <Button size="sm" colorPalette="brand" disabled={!canActivate} onClick={() => run('/activate', 'POST').catch(() => undefined)}>Activer</Button>
          <ActionDialog trigger="Archiver" title="Archiver ce profil ?" danger
            description="Le profil est dépublié, l’attestation est retirée et le compte lié est suspendu. Les sessions du/de la professionnel·le sont fermées."
            confirmLabel="Archiver" onConfirm={() => run('/archive', 'POST')} />
          {profile.status === 'PENDING_APPROVAL' && (
            <ActionDialog trigger="Refuser le rattachement" title="Refuser ce rattachement ?" danger withReason
              description="Le profil et le compte seront supprimés. La personne ne sera plus rattachée à la maison."
              confirmLabel="Refuser" onConfirm={(reason) => run('/reject', 'POST', { reason })} />
          )}
          {profile.linked_account && profile.linked_account.status === 'ACTIVE' && (
            <ActionDialog trigger="Suspendre le compte" title="Suspendre le compte lié ?" withReason danger
              description="La personne ne peut plus se connecter et son profil n’est plus publié." confirmLabel="Suspendre"
              onConfirm={(reason) => run('/account/suspend', 'POST', { reason })} />
          )}
          {profile.linked_account && profile.linked_account.status === 'SUSPENDED' && (
            <Button size="sm" variant="outline" onClick={() => run('/account/restore', 'POST').catch(() => undefined)}>Réactiver le compte</Button>
          )}
          {profile.linked_account && (
            <ActionDialog trigger="Réinitialiser le mot de passe" title="Réinitialiser le mot de passe ?" withReason
              description="Un mot de passe temporaire est généré. Remets-le en main propre : il ne sera plus affiché."
              confirmLabel="Générer" onConfirm={async (reason) => {
                const r = await api<{ temporary_password: string }>(`${base}/account/password-reset`, { method: 'POST', body: { reason } });
                setError(`Mot de passe temporaire (à remettre en main propre, une seule fois) : ${r.temporary_password}`);
                onChange();
              }} />
          )}
          <ActionDialog trigger="Supprimer le profil" title="Supprimer ce profil ?" danger
            description="Le profil et le compte lié sont supprimés (conservation de 30 jours, puis purge)." confirmLabel="Supprimer"
            onConfirm={() => run('', 'DELETE')} />
        </HStack>
        {error && <Text color="danger" role="alert" style={{ wordBreak: 'break-all' }}>{error}</Text>}
      </Stack>
    </Section>
  );
}

function Loader({ mode, id }: { mode: 'manager' | 'professional'; id?: string }) {
  const base = profileBase(mode, id);
  const q = useQuery({ queryKey: ['profile', mode, id ?? 'me'], queryFn: ({ signal }) => api<Profile>(base, { signal }) });
  return (
    <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
      {q.data && <ProfileForm mode={mode} id={id} profile={q.data} />}
    </QueryBlock>
  );
}

export default function ProfileEditor(props: { mode: 'manager' | 'professional'; id?: string }) {
  return (
    <Providers>
      <Box>
        <Heading size="lg" mb="4">{props.mode === 'manager' ? 'Modifier le profil' : 'Mon profil'}</Heading>
        <Loader {...props} />
      </Box>
    </Providers>
  );
}
