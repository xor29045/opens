import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { Button, Field, Grid, HStack, Input, NativeSelect, Stack, Text, Textarea, Checkbox } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import { Section, StatusBadge } from '../../components/ui';
import { QueryBlock } from '../dashboard/shared';
import { api } from '../../lib/api/client';
import { ApiError, describeError, messageFor } from '../../lib/api/problem';
import { zodResolver } from '../../lib/forms';
import { dateFr } from '../../lib/format';
import { contactTypes, visibilities, profileLabels } from '../profile/profileApi';
import type { components } from '../../lib/api/schema';

type Establishment = components['schemas']['Establishment'];
type Region = components['schemas']['Region'];

const schema = z.object({
  display_name: z.string().trim().min(1, 'required').max(80, 'too_long'),
  legal_name: z.string().trim().min(1, 'required').max(160, 'too_long'),
  company_number: z.string().trim(),
  approval_number: z.string().trim(),
  description: z.string().max(5000, 'too_long'),
  region_id: z.string(),
  city_id: z.string(),
  address: z.string().max(300, 'too_long'),
  address_public: z.boolean(),
});
type Values = z.infer<typeof schema>;

interface ContactRow { type: string; value: string; visibility: string; is_verification_phone: boolean }

function Inner() {
  const qc = useQueryClient();
  const est = useQuery({ queryKey: ['manager', 'establishment'], queryFn: ({ signal }) => api<Establishment>('/manager/establishment', { signal }) });
  const regions = useQuery({ queryKey: ['regions'], queryFn: ({ signal }) => api<{ items: Region[] }>('/regions', { auth: false, signal }) });
  return (
    <QueryBlock isPending={est.isPending || regions.isPending} isError={est.isError || regions.isError} error={est.error ?? regions.error}
      onRetry={() => { est.refetch(); regions.refetch(); }}>
      {est.data && regions.data && (
        <Form est={est.data} regions={regions.data.items} onSaved={() => qc.invalidateQueries({ queryKey: ['manager'] })} />
      )}
    </QueryBlock>
  );
}

function Form({ est, regions, onSaved }: { est: Establishment; regions: Region[]; onSaved: () => void }) {
  const [serverError, setServerError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [contacts, setContacts] = useState<ContactRow[]>(() => est.contacts.map((c) => ({ ...c, is_verification_phone: c.is_verification_phone ?? false })));
  const toValues = (e: Establishment): Values => ({
    display_name: e.display_name, legal_name: e.legal_name, company_number: e.company_number ?? '', approval_number: e.approval_number ?? '',
    description: e.description, region_id: e.region?.id ?? '', city_id: e.city?.id ?? '', address: e.address ?? '', address_public: e.address_public,
  });
  const { register, handleSubmit, reset, setError, formState: { errors, isDirty }, watch, setValue } = useForm<Values>({ defaultValues: toValues(est), resolver: zodResolver(schema) });
  useEffect(() => { reset(toValues(est)); setContacts(est.contacts.map((c) => ({ ...c, is_verification_phone: c.is_verification_phone ?? false }))); }, [est]);

  const regionId = watch('region_id');
  const cities = regions.filter((r) => r.kind === 'CITY' && r.parent_id === regionId);
  const locked = est.verification_status === 'VERIFIED';

  const save = handleSubmit(async (v) => {
    setServerError(null);
    setMessage(null);
    try {
      const hours = est.opening_hours;
      await api('/manager/establishment', {
        method: 'PUT',
        body: {
          display_name: v.display_name, legal_name: v.legal_name, company_number: v.company_number, approval_number: v.approval_number,
          description: v.description, opening_hours: hours, region_id: v.region_id || null, city_id: v.city_id || null,
          address: v.address || null, address_public: v.address_public,
        },
      });
      await api('/manager/establishment/contacts', {
        method: 'PUT',
        body: { contacts: contacts.filter((c) => c.value.trim()).map((c) => ({ type: c.type, value: c.value.trim(), visibility: c.visibility, is_verification_phone: c.is_verification_phone })) },
      });
      setMessage(locked ? 'Enregistré. Une modification de l’agrément, de la raison sociale ou du BCE remet la fiche en vérification.' : 'Fiche enregistrée.');
      onSaved();
    } catch (e) {
      if (e instanceof ApiError) {
        for (const f of e.fields) {
          if (f.field in toValues(est)) setError(f.field as keyof Values, { type: f.code, message: f.code });
        }
      }
      setServerError(describeError(e));
    }
  });

  const submit = async () => {
    setServerError(null);
    try {
      await api('/manager/establishment/submit', { method: 'POST' });
      setMessage('Fiche soumise. La plateforme vérifie le numéro BCE et l’agrément.');
      onSaved();
    } catch (e) {
      setServerError(describeError(e));
    }
  };

  const verificationPhones = contacts.filter((c) => c.is_verification_phone).length;

  return (
    <Stack gap="6">
      <HStack justify="space-between" wrap="wrap">
        <HStack><Text fontWeight="semibold">Statut :</Text><StatusBadge value={est.verification_status} /></HStack>
        {est.verification_status === 'VERIFIED' && est.verified_at && <Text fontSize="sm" color="fg.muted">Vérifié le {dateFr(est.verified_at)}</Text>}
      </HStack>
      {locked && <Text fontSize="sm" color="fg.muted">Un changement d’agrément, de raison sociale ou de BCE repasse la fiche en vérification et dépublie ses profils.</Text>}

      <form onSubmit={save} noValidate>
        <Stack gap="6">
          <Section title="Identité de la maison">
            <Grid templateColumns={{ base: '1fr', md: '1fr 1fr' }} gap="4">
              <Field.Root invalid={!!errors.display_name} required>
                <Field.Label>Nom commercial</Field.Label>
                <Input {...register('display_name')} maxLength={80} />
                {errors.display_name && <Field.ErrorText>{messageFor(errors.display_name.message ?? '')}</Field.ErrorText>}
              </Field.Root>
              <Field.Root invalid={!!errors.legal_name} required>
                <Field.Label>Raison sociale</Field.Label>
                <Input {...register('legal_name')} maxLength={160} />
                {errors.legal_name && <Field.ErrorText>{messageFor(errors.legal_name.message ?? '')}</Field.ErrorText>}
              </Field.Root>
              <Field.Root>
                <Field.Label>Numéro BCE</Field.Label>
                <Input {...register('company_number')} placeholder="0123.456.789" />
                <Field.HelperText>Vérifié par la clé de contrôle, puis par la plateforme.</Field.HelperText>
              </Field.Root>
              <Field.Root>
                <Field.Label>Numéro d’agrément</Field.Label>
                <Input {...register('approval_number')} />
                <Field.HelperText>Loi du 3 mai 2024. Affiché sur chaque fiche profil.</Field.HelperText>
              </Field.Root>
            </Grid>
          </Section>

          <Section title="Localisation">
            <Grid templateColumns={{ base: '1fr', md: '1fr 1fr 1fr' }} gap="4">
              <Field.Root>
                <Field.Label>Région</Field.Label>
                <NativeSelect.Root><NativeSelect.Field {...register('region_id', { onChange: () => setValue('city_id', '') })}>
                  <option value="">Choisir</option>
                  {regions.filter((r) => r.kind === 'REGION').map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
                </NativeSelect.Field></NativeSelect.Root>
              </Field.Root>
              <Field.Root>
                <Field.Label>Ville</Field.Label>
                <NativeSelect.Root disabled={!regionId}><NativeSelect.Field {...register('city_id')}>
                  <option value="">Choisir</option>
                  {cities.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
                </NativeSelect.Field></NativeSelect.Root>
              </Field.Root>
              <Field.Root>
                <Field.Label>Adresse (facultative)</Field.Label>
                <Input {...register('address')} maxLength={300} />
              </Field.Root>
            </Grid>
            <Checkbox.Root mt="3" checked={watch('address_public')} onCheckedChange={(d) => setValue('address_public', d.checked === true, { shouldDirty: true })}>
              <Checkbox.HiddenInput />
              <Checkbox.Control />
              <Checkbox.Label>Afficher l’adresse publiquement</Checkbox.Label>
            </Checkbox.Root>
          </Section>

          <Section title="Présentation">
            <Field.Root>
              <Field.Label>Description</Field.Label>
              <Textarea rows={5} {...register('description')} maxLength={5000} />
            </Field.Root>
          </Section>

          <Section title="Coordonnées">
            <Stack gap="3">
              <Text fontSize="sm" color="fg.muted">
                Exactement un numéro de téléphone « de contrôle » est requis pour la soumission (RM-07) : il n’est jamais affiché.
              </Text>
              {contacts.map((c, i) => (
                <HStack key={i} align="flex-end" wrap="wrap">
                  <Field.Root w="40">
                    <Field.Label>Type</Field.Label>
                    <NativeSelect.Root><NativeSelect.Field value={c.type} onChange={(e) => setContacts(contacts.map((x, j) => (j === i ? { ...x, type: e.target.value } : x)))}>
                      {contactTypes.map((t) => <option key={t} value={t}>{profileLabels.contactType[t]}</option>)}
                    </NativeSelect.Field></NativeSelect.Root>
                  </Field.Root>
                  <Field.Root w={{ base: 'full', md: '60' }}>
                    <Field.Label>Valeur</Field.Label>
                    <Input value={c.value} onChange={(e) => setContacts(contacts.map((x, j) => (j === i ? { ...x, value: e.target.value } : x)))} />
                  </Field.Root>
                  <Field.Root w="44">
                    <Field.Label>Visibilité</Field.Label>
                    <NativeSelect.Root><NativeSelect.Field value={c.visibility} onChange={(e) => setContacts(contacts.map((x, j) => (j === i ? { ...x, visibility: e.target.value } : x)))}>
                      {visibilities.map((v) => <option key={v} value={v}>{profileLabels.visibility[v]}</option>)}
                    </NativeSelect.Field></NativeSelect.Root>
                  </Field.Root>
                  <Checkbox.Root checked={c.is_verification_phone} disabled={c.type !== 'PHONE'} onCheckedChange={(d) => setContacts(contacts.map((x, j) => (j === i ? { ...x, is_verification_phone: d.checked === true, visibility: 'HIDDEN' } : x)))}>
                    <Checkbox.HiddenInput />
                    <Checkbox.Control />
                    <Checkbox.Label>Téléphone de contrôle</Checkbox.Label>
                  </Checkbox.Root>
                  <Button size="sm" variant="ghost" onClick={() => setContacts(contacts.filter((_, j) => j !== i))}>Retirer</Button>
                </HStack>
              ))}
              <HStack>
                <Button size="sm" variant="outline" onClick={() => setContacts([...contacts, { type: 'PHONE', value: '', visibility: 'PUBLIC', is_verification_phone: false }])}>Ajouter une coordonnée</Button>
                {verificationPhones !== 1 && <Text fontSize="sm" color="danger">Il faut exactement un téléphone de contrôle.</Text>}
              </HStack>
            </Stack>
          </Section>

          <HStack gap="3" wrap="wrap">
            <Button type="submit" colorPalette="brand" disabled={!isDirty && contacts.length === est.contacts.length}>Enregistrer</Button>
            {(est.verification_status === 'DRAFT' || est.verification_status === 'REJECTED') && (
              <Button variant="outline" onClick={submit}>Soumettre pour vérification</Button>
            )}
          </HStack>
          {message && <Text color="success" role="status">{message}</Text>}
          {serverError && <Text color="danger" role="alert">{serverError}</Text>}
        </Stack>
      </form>
    </Stack>
  );
}

export default function EstablishmentForm() {
  return (
    <Providers>
      <Inner />
    </Providers>
  );
}
