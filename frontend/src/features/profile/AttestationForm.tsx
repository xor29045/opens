import { useEffect, useState } from 'react';
import { Button, Checkbox, Field, HStack, Input, Stack, Text } from '@chakra-ui/react';
import { Section } from '../../components/ui';
import { api } from '../../lib/api/client';
import { describeError } from '../../lib/api/problem';
import { dateFr } from '../../lib/format';
import type { Profile } from './profileApi';

/**
 * Attestation by the manager only (RM-21): birth date read on an ID document
 * (≥ 18 years, checked by the server) and the person's consent to be
 * published. No ID copy is stored.
 */
export default function AttestationForm({ profile, base, onChange }: { profile: Profile; base: string; onChange: () => void }) {
  const [birth, setBirth] = useState('');
  const [consent, setConsent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setBirth(profile.attestation.birth_date ?? '');
    setConsent(false);
  }, [profile.attestation.birth_date]);

  const attest = async () => {
    setError(null);
    setBusy(true);
    try {
      await api(`${base}/attestation`, { method: 'PUT', body: { birth_date: birth, attested: true } });
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
  // With a paid subscription the attestation puts the profile in service (ADR-020).
  const subscribed = !profile.publication.reasons.includes('ESTABLISHMENT_SUBSCRIPTION_INACTIVE');
  const goesLive = subscribed && (profile.status === 'PENDING_APPROVAL' || profile.status === 'DRAFT');
  return (
    <Section title="Attestation du gérant">
      <Stack gap="4">
        {attested ? (
          <Text>
            Attestée le {dateFr(profile.attestation.attested_at)} · âge : {profile.attestation.age} ans
          </Text>
        ) : (
          <Text color="fg.muted">
            Non attestée. Un profil ne peut être publié sans attestation.
            {goesLive ? ' Ton abonnement est actif : l’attestation met le profil en service immédiatement.' : ''}
            {!subscribed ? ' Le profil sera mis en service automatiquement dès le paiement de ton abonnement.' : ''}
          </Text>
        )}
        <HStack align="flex-end" wrap="wrap">
          <Field.Root w="48">
            <Field.Label>Date de naissance (pièce d’identité)</Field.Label>
            <Input type="date" value={birth} onChange={(e) => setBirth(e.target.value)} />
            <Field.HelperText>18 ans minimum, vérifié par le serveur.</Field.HelperText>
          </Field.Root>
        </HStack>
        <Checkbox.Root checked={consent} onCheckedChange={(d) => setConsent(d.checked === true)}>
          <Checkbox.HiddenInput />
          <Checkbox.Control />
          <Checkbox.Label>
            Âge vérifié sur une pièce d’identité (18 ans minimum) et accord de la personne pour cette publication.
          </Checkbox.Label>
        </Checkbox.Root>
        <HStack>
          <Button colorPalette="brand" disabled={!consent || !birth || busy} loading={busy} onClick={attest}>
            {attested ? 'Mettre à jour l’attestation' : goesLive ? 'Attester et mettre en service' : 'Attester'}
          </Button>
          {attested && <Button variant="outline" colorPalette="red" disabled={busy} onClick={remove}>Retirer l’attestation</Button>}
        </HStack>
        {error && <Text color="danger" role="alert">{error}</Text>}
      </Stack>
    </Section>
  );
}
