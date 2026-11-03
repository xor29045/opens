import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Badge, Button, HStack, Stack, Text, Grid, Field, NativeSelect } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import Pagination from '../../components/Pagination';
import { Section, StatusBadge } from '../../components/ui';
import { QueryBlock, DataTable, ActionDialog, type Column } from '../dashboard/shared';
import { api, apiBlob } from '../../lib/api/client';
import { ApiError, describeError } from '../../lib/api/problem';
import { dateFr, eur } from '../../lib/format';
import type { components } from '../../lib/api/schema';

type SubscriptionView = components['schemas']['SubscriptionView'];
type Plan = components['schemas']['Plan'];
type Payment = components['schemas']['Payment'];
type Invoice = components['schemas']['Invoice'];
type Stats = components['schemas']['EstablishmentStats'];
type PageOf<T> = { items: T[]; page: number; page_size: number; total: number; total_pages: number };

/** Opens the hosted crypto invoice (NOWPayments). Only https URLs are followed. */
function goToInvoice(p: Payment) {
  if (p.invoice_url && p.invoice_url.startsWith('https://')) window.location.assign(p.invoice_url);
}

function Reminder({ v }: { v: SubscriptionView }) {
  const r = v.reminder;
  if (r.level === 'renewal_due') {
    return (
      <Alert.Root status="info"><Alert.Content>
        <Alert.Title>Renouvellement à prévoir</Alert.Title>
        <Alert.Description>Échéance le {dateFr(r.due_at)} ({r.days_left} jour(s)). Le paiement en crypto n’est pas automatique.</Alert.Description>
      </Alert.Content></Alert.Root>
    );
  }
  if (r.level === 'past_due') {
    return (
      <Alert.Root status="warning"><Alert.Content>
        <Alert.Title>Abonnement impayé</Alert.Title>
        <Alert.Description>Tes profils restent publiés jusqu’au {dateFr(r.due_at)}, puis ils seront dépubliés.</Alert.Description>
      </Alert.Content></Alert.Root>
    );
  }
  if (r.level === 'grace_over') {
    return (
      <Alert.Root status="error"><Alert.Content>
        <Alert.Title>Profils dépubliés</Alert.Title>
        <Alert.Description>La période de grâce est terminée. Règle la facture pour republier tes profils.</Alert.Description>
      </Alert.Content></Alert.Root>
    );
  }
  return null;
}

function SubscriptionInner() {
  const qc = useQueryClient();
  const [error, setError] = useState<string | null>(null);
  const [planCode, setPlanCode] = useState('');
  const [page, setPage] = useState(1);
  const sub = useQuery({ queryKey: ['billing', 'subscription'], queryFn: ({ signal }) => api<SubscriptionView>('/billing/subscription', { signal }) });
  const plans = useQuery({ queryKey: ['billing', 'plans'], queryFn: ({ signal }) => api<{ items: Plan[] }>('/billing/plans', { auth: false, signal }) });
  const payments = useQuery({ queryKey: ['billing', 'payments', page], queryFn: ({ signal }) => api<PageOf<Payment>>(`/billing/payments?page=${page}`, { signal }) });
  const refresh = () => qc.invalidateQueries({ queryKey: ['billing'] });

  const call = async (path: string, body?: unknown) => {
    setError(null);
    try {
      const p = await api<Payment>(path, { method: 'POST', body });
      refresh();
      return p;
    } catch (e) {
      setError(e instanceof ApiError && e.status === 502 ? 'Le prestataire de paiement est indisponible, réessaie plus tard.' : describeError(e));
      throw e;
    }
  };

  const columns: Column<Payment>[] = [
    { key: 'date', header: 'Créé le', cell: (p) => dateFr(p.created_at) },
    { key: 'amount', header: 'Montant', cell: (p) => eur(p.amount_cents) },
    { key: 'period', header: 'Période', cell: (p) => (p.period_start ? `${dateFr(p.period_start)} → ${dateFr(p.period_end)}` : '—') },
    { key: 'crypto', header: 'Crypto', cell: (p) => (p.pay_amount_received ? `${p.pay_amount_received} ${p.pay_currency ?? ''}` : '—') },
    { key: 'status', header: 'Statut', cell: (p) => <StatusBadge value={p.status} /> },
    { key: 'pay', header: '', cell: (p) => ((p.status === 'PENDING' || p.status === 'CONFIRMING') && p.invoice_url ? <Button size="xs" onClick={() => goToInvoice(p)}>Payer</Button> : null) },
  ];

  return (
    <QueryBlock isPending={sub.isPending} isError={sub.isError} error={sub.error} onRetry={() => sub.refetch()}>
      {sub.data && (
        <Stack gap="6">
          <Reminder v={sub.data} />
          <Section title="Abonnement de l’établissement">
            {sub.data.subscription ? (
              <Stack gap="3">
                <HStack><StatusBadge value={sub.data.subscription.status} />{sub.data.entitled ? <Badge colorPalette="green">Profils publiables</Badge> : <Badge>Aucune publication</Badge>}</HStack>
                <Grid templateColumns={{ base: '1fr', md: 'repeat(3, 1fr)' }} gap="3">
                  <Text fontSize="sm">Formule : {sub.data.plan?.name} ({eur(sub.data.plan?.amount_cents)} / {sub.data.plan?.interval_days} jours)</Text>
                  <Text fontSize="sm">Période en cours : {dateFr(sub.data.subscription.current_period_start)} → {dateFr(sub.data.subscription.current_period_end)}</Text>
                  {sub.data.subscription.cancel_at_period_end && <Text fontSize="sm" color="warning">Résiliation à la fin de la période.</Text>}
                </Grid>
                <HStack gap="2" wrap="wrap">
                  {sub.data.open_payment?.invoice_url && (
                    <Button colorPalette="brand" onClick={() => goToInvoice(sub.data.open_payment!)}>Payer la facture en attente</Button>
                  )}
                  {['ACTIVE', 'TRIALING', 'PAST_DUE'].includes(sub.data.subscription.status) && !sub.data.open_payment && (
                    <Button variant="outline" onClick={() => call('/billing/subscription/renew').then(goToInvoice).catch(() => undefined)}>Payer la période suivante</Button>
                  )}
                  {sub.data.subscription.status === 'INCOMPLETE' && !sub.data.open_payment && (
                    <Button colorPalette="brand" onClick={() => call('/billing/subscription', {}).then(goToInvoice).catch(() => undefined)}>Relancer le paiement</Button>
                  )}
                  {['ACTIVE', 'TRIALING'].includes(sub.data.subscription.status) && (sub.data.subscription.cancel_at_period_end ? (
                    <Button variant="outline" onClick={() => call('/billing/subscription/resume').catch(() => undefined)}>Annuler la résiliation</Button>
                  ) : (
                    <ActionDialog trigger="Résilier" title="Résilier l’abonnement ?" danger confirmLabel="Résilier"
                      description="Tes profils restent publiés jusqu’à la fin de la période payée. Aucun remboursement au prorata (CGV)."
                      onConfirm={() => call('/billing/subscription/cancel')} />
                  ))}
                  {sub.data.subscription.status === 'CANCELED' && (
                    <Button colorPalette="brand" onClick={() => call('/billing/subscription', {}).then(goToInvoice).catch(() => undefined)}>Souscrire à nouveau</Button>
                  )}
                </HStack>
              </Stack>
            ) : (
              <Stack gap="3">
                <Text>Aucun abonnement. Souscris pour publier les profils de ta maison.</Text>
                <QueryBlock isPending={plans.isPending} isError={plans.isError} error={plans.error} onRetry={() => plans.refetch()}>
                  <HStack align="flex-end">
                    <Field.Root w="72">
                      <Field.Label>Formule</Field.Label>
                      <NativeSelect.Root><NativeSelect.Field value={planCode} onChange={(e) => setPlanCode(e.target.value)}>
                        <option value="">Formule par défaut</option>
                        {(plans.data?.items ?? []).map((p) => <option key={p.code} value={p.code}>{p.name} — {eur(p.amount_cents)}</option>)}
                      </NativeSelect.Field></NativeSelect.Root>
                    </Field.Root>
                    <Button colorPalette="brand" onClick={() => call('/billing/subscription', planCode ? { plan_code: planCode } : {}).then(goToInvoice).catch(() => undefined)}>
                      Souscrire et payer
                    </Button>
                  </HStack>
                  <Text fontSize="sm" color="fg.muted">Prix en euros, réglé en cryptomonnaie via NOWPayments au taux fixé à la création de la facture.</Text>
                </QueryBlock>
              </Stack>
            )}
            {error && <Text color="danger" role="alert" mt="3">{error}</Text>}
          </Section>
          <Section title="Paiements">
            <QueryBlock isPending={payments.isPending} isError={payments.isError} error={payments.error} onRetry={() => payments.refetch()}>
              {payments.data && (
                <>
                  <DataTable columns={columns} rows={payments.data.items} empty="Aucun paiement." caption="Historique des paiements" />
                  <Pagination page={payments.data.page} totalPages={payments.data.total_pages} onPage={setPage} />
                </>
              )}
            </QueryBlock>
          </Section>
        </Stack>
      )}
    </QueryBlock>
  );
}

function InvoicesInner() {
  const [page, setPage] = useState(1);
  const [error, setError] = useState<string | null>(null);
  const q = useQuery({ queryKey: ['billing', 'invoices', page], queryFn: ({ signal }) => api<PageOf<Invoice>>(`/billing/invoices?page=${page}`, { signal }) });
  const download = async (inv: Invoice) => {
    setError(null);
    try {
      const blob = await apiBlob(`/billing/invoices/${inv.id}/pdf`);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `facture-${inv.number}.pdf`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      setError(describeError(e));
    }
  };
  const columns: Column<Invoice>[] = [
    { key: 'number', header: 'Numéro', cell: (i) => i.number },
    { key: 'date', header: 'Émise le', cell: (i) => dateFr(i.issued_at) },
    { key: 'amount', header: 'Montant', cell: (i) => eur(i.amount_cents) },
    { key: 'pdf', header: '', cell: (i) => <Button size="xs" variant="outline" onClick={() => download(i)}>PDF</Button> },
  ];
  return (
    <Section title="Factures">
      <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
        {q.data && (
          <>
            <DataTable columns={columns} rows={q.data.items} empty="Aucune facture." caption="Factures de l’établissement" />
            <Pagination page={q.data.page} totalPages={q.data.total_pages} onPage={setPage} />
          </>
        )}
      </QueryBlock>
      {error && <Text color="danger" role="alert">{error}</Text>}
    </Section>
  );
}

function StatsInner() {
  const [days, setDays] = useState(30);
  const q = useQuery({ queryKey: ['manager', 'stats', days], queryFn: ({ signal }) => api<Stats>(`/manager/stats?days=${days}`, { signal }), retry: false });
  const forbidden = q.error instanceof ApiError && q.error.status === 403;
  const columns: Column<Stats['profiles'][number] & { id: string }>[] = [
    { key: 'name', header: 'Profil', cell: (p) => p.display_name },
    { key: 'views', header: 'Vues', cell: (p) => p.views },
    { key: 'contacts', header: 'Coordonnées consultées', cell: (p) => p.contact_views },
  ];
  return (
    <Section title="Statistiques" action={
      <NativeSelect.Root w="40"><NativeSelect.Field value={days} onChange={(e) => setDays(Number(e.target.value))} aria-label="Période">
        <option value={7}>7 jours</option><option value={30}>30 jours</option><option value={90}>90 jours</option>
      </NativeSelect.Field></NativeSelect.Root>
    }>
      {forbidden ? (
        <Text color="fg.muted">Les statistiques sont disponibles avec un abonnement actif.</Text>
      ) : (
        <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
          {q.data && (
            <Stack gap="4">
              <HStack gap="6"><Text>Vues : <strong>{q.data.views}</strong></Text><Text>Coordonnées consultées : <strong>{q.data.contact_views}</strong></Text></HStack>
              <DataTable columns={columns} rows={q.data.profiles.map((p) => ({ ...p, id: p.profile_id }))} empty="Aucun profil." caption="Statistiques par profil" />
              <Text fontSize="xs" color="fg.muted">Compteurs agrégés et anonymes : aucun client n’est identifié.</Text>
            </Stack>
          )}
        </QueryBlock>
      )}
    </Section>
  );
}

export function Subscription() {
  return <Providers><SubscriptionInner /></Providers>;
}
export function Invoices() {
  return <Providers><InvoicesInner /></Providers>;
}
export function ManagerStats() {
  return <Providers><StatsInner /></Providers>;
}
