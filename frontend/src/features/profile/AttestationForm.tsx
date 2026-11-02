import { useEffect, useState } from 'react';
import { Button, Checkbox, Field, HStack, Input, Stack, Text } from '@chakra-ui/react';
import { Section } from '../../components/ui';
import { api } from '../../lib/api/client';
import { describeError } from '../../lib/api/problem';
import { dateFr } from '../../lib/format';
import type { Profile } from './profileApi';

/**
 * Attestation by the manager only (RM-21): birth date on the ID (≥ 18 years,
 * checked by the server), contract reference and the written consent.
 * No ID copy is stored.
 */
export default function AttestationForm({ profile, base, onChange }: { profile: Profile; base: string; onChange: () => void }) {
  const [birth, setBirth] = useState('');
  const [reference, setReference] = useState('');
  const [consent, setConsent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setBirth(profile.attestation.birth_date ?? '');
    setReference(profile.attestation.employment_reference ?? '');
    setConsent(false);
  }, [profile.attestation.birth_date, profile.attestation.employment_reference]);

  const attest = async () => {
    setError(null);
    setBusy(true);
    try {
      await api(`${base}/attestation`, { method: 'PUT', body: { birth_date: birth, employment_reference: reference, attested: true } });
      onChange();
    } catch (e) {
      setError(describeError(e));
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    setError(null);
    setBusy(true);
    try {
      await api(`${base}/attestation`, { method: 'DELETE' });
      onChange();
    } catch (e) {
      setError(describeError(e));
    } finally {
      setBusy(false);
    }
  };

  const attested = profile.attestation.attested;
  return (
    <Section title="Attestation du gérant">
      <Stack gap="4">
        {attested ? (
          <Text>
            Attestée le {dateFr(profile.attestation.attested_at)} · âge : {profile.attestation.age} ans · contrat : {profile.attestation.employment_reference}
          </Text>
        ) : (
          <Text color="fg.muted">Non attestée. Un profil ne peut être publié sans attestation.</Text>
        )}
        <HStack align="flex-end" wrap="wrap">
          <Field.Root w="48">
            <Field.Label>Date de naissance (pièce d’identité)</Field.Label>
            <Input type="date" value={birth} onChange={(e) => setBirth(e.target.value)} />
            <Field.HelperText>18 ans minimum, vérifié par le serveur.</Field.HelperText>
          </Field.Root>
          <Field.Root w="64">
            <Field.Label>Référence interne du contrat</Field.Label>
            <Input value={reference} maxLength={100} onChange={(e) => setReference(e.target.value)} />
          </Field.Root>
        </HStack>
        <Checkbox.Root checked={consent} onCheckedChange={(d) => setConsent(d.checked === true)}>
          <Checkbox.HiddenInput />
          <Checkbox.Control />
          <Checkbox.Label>
            Identité et âge vérifiés sur pièce ; contrat de travail en cours ; accord écrit de la personne pour cette publication.
          </Checkbox.Label>
        </Checkbox.Root>
        <HStack>
          <Button colorPalette="brand" disabled={!consent || !birth || !reference || busy} loading={busy} onClick={attest}>
            {attested ? 'Mettre à jour l’attestation' : 'Attester'}
          </Button>
          {attested && <Button variant="outline" colorPalette="red" disabled={busy} onClick={remove}>Retirer l’attestation</Button>}
        </HStack>
        {error && <Text color="danger" role="alert">{error}</Text>}
      </Stack>
    </Section>
  );
}
