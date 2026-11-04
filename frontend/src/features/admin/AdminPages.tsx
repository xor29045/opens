import { useState, type ReactNode } from 'react';
import { useQuery, useQueryClient, keepPreviousData } from '@tanstack/react-query';
import { Badge, Button, Field, Grid, HStack, Input, NativeSelect, Stack, Stat, Text, Code } from '@chakra-ui/react';
import Providers from '../../components/Providers';
import Pagination from '../../components/Pagination';
import { Section, StatusBadge } from '../../components/ui';
import { QueryBlock, DataTable, ActionDialog, type Column } from '../dashboard/shared';
import { api } from '../../lib/api/client';
import { dateFr, dateTimeFr, eur, reasonLabels } from '../../lib/format';
import type { components } from '../../lib/api/schema';

type S = components['schemas'];
type PageOf<T> = { items: T[]; page: number; page_size: number; total: number; total_pages: number };

function useList<T>(key: string, path: string, params: Record<string, string>, page: number) {
  const qs = new URLSearchParams({ page: String(page), page_size: '25' });
  for (const [k, v] of Object.entries(params)) if (v) qs.set(k, v);
  return useQuery({
    queryKey: ['admin', key, qs.toString()],
    queryFn: ({ signal }) => api<PageOf<T>>(`${path}?${qs}`, { signal }),
    placeholderData: keepPreviousData,
  });
}

function Select({ label, value, onChange, options }: { label: string; value: string; onChange: (v: string) => void; options: [string, string][] }) {
  return (
    <Field.Root w="52">
      <Field.Label>{label}</Field.Label>
      <NativeSelect.Root><NativeSelect.Field value={value} onChange={(e) => onChange(e.target.value)}>
        {options.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
      </NativeSelect.Field></NativeSelect.Root>
    </Field.Root>
  );
}

function Wrap({ children }: { children: ReactNode }) {
  return <Providers>{children}</Providers>;
}

// ---------------------------------------------------------------- KPIs

function DashboardInner() {
  const q = useQuery({ queryKey: ['admin', 'stats'], queryFn: ({ signal }) => api<S['AdminStats']>('/admin/stats', { signal }) });
  return (
    <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
      {q.data && (
        <Stack gap="6">
          <Grid templateColumns={{ base: '1fr 1fr', md: 'repeat(3, 1fr)' }} gap="4">
            {[
              ['Profils publiés', q.data.published_profiles, '/admin/profils'],
              ['MRR', eur(q.data.mrr_cents), '/admin/abonnements'],
            ].map(([label, value, href]) => (
              <Stat.Root key={String(label)} borderWidth="1px" borderColor="border" borderRadius="lg" p="4">
                <Stat.Label><a href={String(href)}>{label}</a></Stat.Label>
                <Stat.ValueText>{value}</Stat.ValueText>
              </Stat.Root>
            ))}
          </Grid>
          <Section title="Abonnements par statut">
            {Object.entries(q.data.subscriptions_by_status ?? {}).map(([k, v]) => (
              <HStack key={k} justify="space-between"><StatusBadge value={k} /><Text>{v}</Text></HStack>
            ))}
          </Section>
          <Section title="Échéances des 7 prochains jours">
            <Text fontSize="sm" color="fg.muted" mb="3">Sans e-mail ni prélèvement : prévenir l’établissement par téléphone si nécessaire.</Text>
            <DataTable
              rows={(q.data.upcoming_due ?? []).map((d) => ({ ...d, id: d.subscription_id }))}
              empty="Aucune échéance proche." caption="Échéances proches"
              columns={[
                { key: 'name', header: 'Établissement', cell: (d) => d.establishment_name },
                { key: 'status', header: 'Statut', cell: (d) => <StatusBadge value={d.status} /> },
                { key: 'end', header: 'Échéance', cell: (d) => dateFr(d.current_period_end) },
              ]}
            />
          </Section>
        </Stack>
      )}
    </QueryBlock>
  );
}

// ---------------------------------------------------------------- establishments

function EstablishmentDetail({ id, onBack }: { id: string; onBack: () => void }) {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ['admin', 'establishment', id], queryFn: ({ signal }) => api<{
    establishment: S['Establishment']; manager: S['User']; profiles: S['AdminProfile'][]; subscription: S['Subscription'] | null;
    blocking_reasons: string[];
  }>(`/admin/establishments/${id}`, { signal }) });
  const refresh = () => qc.invalidateQueries({ queryKey: ['admin'] });
  const post = (path: string, body: unknown) => api(`/admin/establishments/${id}${path}`, { method: 'POST', body }).then(refresh);
  return (
    <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
      {q.data && (() => {
        const e = q.data.establishment;
        const verif = e.contacts.find((c) => c.is_verification_phone);
        return (
          <Stack gap="6">
            <Button alignSelf="flex-start" size="sm" variant="ghost" onClick={onBack}>← Retour</Button>
            <Section title={e.display_name || '(sans nom)'} action={e.is_hidden ? <Badge colorPalette="red">Masqué</Badge> : undefined}>
              <Grid templateColumns={{ base: '1fr', md: '1fr 1fr' }} gap="2" fontSize="sm">
                <Text>Raison sociale : {e.legal_name || '—'}</Text>
                <Text>Gérant : {q.data.manager.username} ({q.data.manager.status})</Text>
                <Text>Ville : {e.city?.name ?? '—'}</Text>
                <Text>Numéro pour la plateforme : {verif ? <Code>{verif.value}</Code> : '—'}</Text>
                <Text>Créé le : {dateFr(e.created_at)}</Text>
                <Text>Abonnement : {q.data.subscription ? <StatusBadge value={q.data.subscription.status} /> : 'aucun'}</Text>
              </Grid>
              {q.data.blocking_reasons.length > 0 && (
                <Text mt="2" fontSize="sm" color="fg.muted">Bloquants : {q.data.blocking_reasons.map((r) => reasonLabels[r] ?? r).join(' · ')}</Text>
              )}
              <HStack mt="4" gap="2" wrap="wrap">
                {e.is_hidden ? (
                  <ActionDialog trigger="Réafficher" title="Réafficher l’établissement ?" confirmLabel="Réafficher"
                    description="Ses profils pourront de nouveau être publiés." onConfirm={() => post('/unhide', {})} />
                ) : (
                  <ActionDialog trigger="Masquer" title="Masquer l’établissement ?" withReason danger confirmLabel="Masquer"
                    description="Tous ses profils sont dépubliés." onConfirm={(reason) => post('/hide', { reason })} />
                )}
              </HStack>
            </Section>
            <Section title={`Profils (${q.data.profiles.length})`}>
              <DataTable rows={q.data.profiles} empty="Aucun profil." caption="Profils de l’établissement" columns={[
                { key: 'name', header: 'Nom', cell: (p) => p.display_name },
                { key: 'status', header: 'Statut', cell: (p) => <StatusBadge value={p.status} /> },
                { key: 'pub', header: 'Publié', cell: (p) => (p.is_published ? 'Oui' : 'Non') },
              ]} />
            </Section>
          </Stack>
        );
      })()}
    </QueryBlock>
  );
}

function EstablishmentsInner() {
  const [sub, setSub] = useState('');
  const [query, setQuery] = useState('');
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<string | null>(null);
  const q = useList<S['AdminEstablishmentRow']>('establishments', '/admin/establishments', { subscription_status: sub, q: query }, page);
  if (selected) return <EstablishmentDetail id={selected} onBack={() => setSelected(null)} />;
  const columns: Column<S['AdminEstablishmentRow']>[] = [
    { key: 'name', header: 'Établissement', cell: (e) => <Button variant="plain" size="sm" p="0" onClick={() => setSelected(e.id)}>{e.display_name || '(sans nom)'}</Button> },
    { key: 'legal', header: 'Raison sociale', cell: (e) => e.legal_name || '—' },
    { key: 'manager', header: 'Gérant', cell: (e) => `${e.manager_username} (${e.manager_status === 'ACTIVE' ? 'actif' : 'suspendu'})` },
    { key: 'sub', header: 'Abonnement', cell: (e) => (e.subscription_status ? <StatusBadge value={e.subscription_status} /> : '—') },
    { key: 'profiles', header: 'Profils', cell: (e) => `${e.published_count}/${e.profiles_count}` },
    { key: 'hidden', header: '', cell: (e) => (e.is_hidden ? <Badge colorPalette="red">Masqué</Badge> : null) },
  ];
  return (
    <Section title="Établissements">
      <HStack gap="4" mb="4" wrap="wrap" align="flex-end">
        <Select label="Abonnement" value={sub} onChange={(v) => { setSub(v); setPage(1); }}
          options={[['', 'Tous'], ['ACTIVE', 'Actif'], ['PAST_DUE', 'Impayé'], ['INCOMPLETE', 'Paiement en attente'], ['CANCELED', 'Résilié'], ['NONE', 'Aucun']]} />
        <Field.Root w="64"><Field.Label>Recherche</Field.Label><Input value={query} onChange={(e) => { setQuery(e.target.value); setPage(1); }} placeholder="Nom, raison sociale, gérant" /></Field.Root>
      </HStack>
      <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
        {q.data && <><DataTable columns={columns} rows={q.data.items} empty="Aucun établissement." caption="Établissements" />
          <Pagination page={q.data.page} totalPages={q.data.total_pages} onPage={setPage} /></>}
      </QueryBlock>
    </Section>
  );
}

// ---------------------------------------------------------------- profiles

function ProfilesInner() {
  const qc = useQueryClient();
  const [published, setPublished] = useState('');
  const [hidden, setHidden] = useState('');
  const [query, setQuery] = useState('');
  const [page, setPage] = useState(1);
  const q = useList<S['AdminProfile']>('profiles', '/admin/profiles', { published, hidden, q: query }, page);
  const post = (id: string, path: string, body: unknown) => api(`/admin/profiles/${id}${path}`, { method: 'POST', body }).then(() => qc.invalidateQueries({ queryKey: ['admin'] }));
  const columns: Column<S['AdminProfile']>[] = [
    { key: 'name', header: 'Nom', cell: (p) => p.display_name },
    { key: 'status', header: 'Statut', cell: (p) => <StatusBadge value={p.status} /> },
    { key: 'pub', header: 'Publié', cell: (p) => (p.is_published ? <a href={`/profils/${p.slug}`} target="_blank" rel="noopener noreferrer">Voir</a> : 'Non') },
    { key: 'act', header: '', cell: (p) => (p.is_hidden ? (
      <ActionDialog trigger="Réafficher" title="Réafficher ce profil ?" confirmLabel="Réafficher" description={`Masqué pour : ${p.hidden_reason ?? '—'}`} onConfirm={() => post(p.id, '/unhide', {})} />
    ) : (
      <ActionDialog trigger="Masquer" title="Masquer ce profil ?" withReason danger confirmLabel="Masquer" description="Le profil est immédiatement dépublié." onConfirm={(reason) => post(p.id, '/hide', { reason })} />
    )) },
  ];
  return (
    <Section title="Profils">
      <HStack gap="4" mb="4" wrap="wrap" align="flex-end">
        <Select label="Publication" value={published} onChange={(v) => { setPublished(v); setPage(1); }} options={[['', 'Tous'], ['true', 'Publiés'], ['false', 'Non publiés']]} />
        <Select label="Masquage" value={hidden} onChange={(v) => { setHidden(v); setPage(1); }} options={[['', 'Tous'], ['true', 'Masqués'], ['false', 'Visibles']]} />
        <Field.Root w="64"><Field.Label>Nom</Field.Label><Input value={query} onChange={(e) => { setQuery(e.target.value); setPage(1); }} /></Field.Root>
      </HStack>
      <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
        {q.data && <><DataTable columns={columns} rows={q.data.items} empty="Aucun profil." caption="Profils" />
          <Pagination page={q.data.page} totalPages={q.data.total_pages} onPage={setPage} /></>}
      </QueryBlock>
    </Section>
  );
}

// ---------------------------------------------------------------- users

function UsersInner() {
  const qc = useQueryClient();
  const [role, setRole] = useState('');
  const [status, setStatus] = useState('');
  const [query, setQuery] = useState('');
  const [page, setPage] = useState(1);
  const [temp, setTemp] = useState<{ user: string; password: string } | null>(null);
  const q = useList<S['AdminUser']>('users', '/admin/users', { role, status, q: query }, page);
  const refresh = () => qc.invalidateQueries({ queryKey: ['admin'] });
  const columns: Column<S['AdminUser']>[] = [
    { key: 'name', header: 'Pseudo', cell: (u) => u.username },
    { key: 'role', header: 'Rôle', cell: (u) => u.role },
    { key: 'status', header: 'Statut', cell: (u) => <StatusBadge value={u.status} /> },
    { key: 'login', header: 'Dernière connexion', cell: (u) => dateTimeFr(u.last_login_at) },
    { key: 'act', header: '', cell: (u) => (u.role === 'ADMIN' ? null : (
      <HStack gap="1" wrap="wrap">
        {u.status === 'ACTIVE' && (
          <ActionDialog trigger="Suspendre" title={`Suspendre ${u.username} ?`} withReason danger confirmLabel="Suspendre"
            description={u.role === 'MANAGER' ? 'Tous les profils de l’établissement sont dépubliés et les sessions de ses professionnel·les fermées.' : 'Les sessions sont fermées immédiatement.'}
            onConfirm={(reason) => api(`/admin/users/${u.id}/suspend`, { method: 'POST', body: { reason } }).then(refresh)} />
        )}
        {u.status === 'SUSPENDED' && (
          <ActionDialog trigger="Réactiver" title={`Réactiver ${u.username} ?`} confirmLabel="Réactiver" description="Le compte pourra se reconnecter."
            onConfirm={() => api(`/admin/users/${u.id}/restore`, { method: 'POST', body: {} }).then(refresh)} />
        )}
        {u.role === 'MANAGER' && u.status === 'ACTIVE' && (
          <ActionDialog trigger="Mot de passe" title="Réinitialiser le mot de passe du gérant ?" withReason confirmLabel="Générer"
            description="Un mot de passe temporaire est généré ; le gérant devra le changer à sa prochaine connexion."
            onConfirm={async (reason) => {
              const r = await api<{ temporary_password: string }>(`/admin/users/${u.id}/password-reset`, { method: 'POST', body: { reason } });
              setTemp({ user: u.username, password: r.temporary_password });
            }} />
        )}
        {u.status !== 'DELETED' && (
          <ActionDialog trigger="Supprimer" title={`Supprimer ${u.username} ?`} withReason danger confirmLabel="Supprimer"
            description={u.role === 'MANAGER' ? 'L’établissement, ses profils et les comptes de ses professionnel·les sont supprimés ; la facturation est conservée.' : 'Le compte est anonymisé puis purgé sous 30 jours.'}
            onConfirm={(reason) => api(`/admin/users/${u.id}`, { method: 'DELETE', body: { reason } }).then(refresh)} />
        )}
      </HStack>
    )) },
  ];
  return (
    <Section title="Utilisateurs">
      {temp && (
        <Stack mb="4" p="3" borderWidth="1px" borderColor="warning" borderRadius="md" role="status">
          <Text>Mot de passe temporaire de <strong>{temp.user}</strong> (affiché une seule fois, à transmettre au gérant) :</Text>
          <Code p="2" userSelect="all">{temp.password}</Code>
          <Button size="xs" alignSelf="flex-start" onClick={() => setTemp(null)}>Masquer</Button>
        </Stack>
      )}
      <HStack gap="4" mb="4" wrap="wrap" align="flex-end">
        <Select label="Rôle" value={role} onChange={(v) => { setRole(v); setPage(1); }} options={[['', 'Tous'], ['MANAGER', 'Gérants'], ['PROFESSIONAL', 'Professionnel·les'], ['ADMIN', 'Admins']]} />
        <Select label="Statut" value={status} onChange={(v) => { setStatus(v); setPage(1); }} options={[['', 'Actifs et suspendus'], ['ACTIVE', 'Actifs'], ['SUSPENDED', 'Suspendus'], ['DELETED', 'Supprimés']]} />
        <Field.Root w="64"><Field.Label>Pseudo</Field.Label><Input value={query} onChange={(e) => { setQuery(e.target.value); setPage(1); }} /></Field.Root>
      </HStack>
      <QueryBlock isPending={q.isPending} isError={q.isError} error={q.error} onRetry={() => q.refetch()}>
        {q.data && <><DataTable columns={columns} rows={q.data.items} empty="Aucun utilisateur." caption="Utilisateurs" />
          <Pagination page={q.data.page} totalPages={q.data.total_pages} onPage={setPage} /></>}
      </QueryBlock>
    </Section>
  );
}

// ---------------------------------------------------------------- billing

function BillingInner() {
  const qc = useQueryClient();
  const [subStatus, setSubStatus] = useState('');
  const [payStatus, setPayStatus] = useState('');
  const [subPage, setSubPage] = useState(1);
  const [payPage, setPayPage] = useState(1);
  const subs = useList<S['AdminSubscription']>('subscriptions', '/admin/subscriptions', { status: subStatus }, subPage);
  const pays = useList<S['AdminPayment']>('payments', '/admin/payments', { status: payStatus }, payPage);
  return (
    <Stack gap="6">
      <Section title="Abonnements">
        <HStack mb="4"><Select label="Statut" value={subStatus} onChange={(v) => { setSubStatus(v); setSubPage(1); }}
          options={[['', 'Tous'], ['ACTIVE', 'Actif'], ['PAST_DUE', 'Impayé'], ['INCOMPLETE', 'Paiement en attente'], ['CANCELED', 'Résilié'], ['TRIALING', 'Essai']]} /></HStack>
        <QueryBlock isPending={subs.isPending} isError={subs.isError} error={subs.error} onRetry={() => subs.refetch()}>
          {subs.data && <><DataTable rows={subs.data.items} empty="Aucun abonnement." caption="Abonnements" columns={[
            { key: 'est', header: 'Établissement', cell: (s) => s.establishment_name },
            { key: 'plan', header: 'Formule', cell: (s) => s.plan_code },
            { key: 'status', header: 'Statut', cell: (s) => <StatusBadge value={s.status} /> },
            { key: 'end', header: 'Fin de période', cell: (s) => dateFr(s.current_period_end) },
            { key: 'cancel', header: '', cell: (s) => (s.cancel_at_period_end ? <Badge>Résiliation programmée</Badge> : null) },
          ]} /><Pagination page={subs.data.page} totalPages={subs.data.total_pages} onPage={setSubPage} /></>}
        </QueryBlock>
      </Section>
      <Section title="Paiements">
        <HStack mb="4"><Select label="Statut" value={payStatus} onChange={(v) => { setPayStatus(v); setPayPage(1); }}
          options={[['', 'Tous'], ['PENDING', 'En attente'], ['CONFIRMING', 'En confirmation'], ['PARTIAL', 'Partiel'], ['PAID', 'Payé'], ['EXPIRED', 'Expiré'], ['FAILED', 'Échoué'], ['REFUNDED', 'Remboursé']]} /></HStack>
        <QueryBlock isPending={pays.isPending} isError={pays.isError} error={pays.error} onRetry={() => pays.refetch()}>
          {pays.data && <><DataTable rows={pays.data.items} empty="Aucun paiement." caption="Paiements" columns={[
            { key: 'date', header: 'Créé', cell: (p) => dateFr(p.created_at) },
            { key: 'amount', header: 'Montant', cell: (p) => eur(p.amount_cents) },
            { key: 'crypto', header: 'Reçu / attendu', cell: (p) => `${p.pay_amount_received ?? '—'} / ${p.pay_amount_expected ?? '—'} ${p.pay_currency ?? ''}` },
            { key: 'status', header: 'Statut', cell: (p) => <StatusBadge value={p.status} /> },
            { key: 'provider', header: 'Prestataire', cell: (p) => <Text fontSize="xs">{p.provider} {p.provider_payment_id ?? ''}</Text> },
            { key: 'act', header: '', cell: (p) => (
              <ActionDialog trigger="Régulariser" title="Régulariser ce paiement ?" withReason confirmLabel={p.status === 'PARTIAL' ? 'Accepter comme payé' : 'Re-vérifier'}
                description={p.status === 'PARTIAL' ? 'Le paiement partiel sera accepté et la période activée.' : 'Le statut est re-demandé au prestataire.'}
                onConfirm={(reason) => api(`/admin/payments/${p.id}/reconcile`, { method: 'POST', body: { reason, force_paid: p.status === 'PARTIAL' } })
                  .then(() => qc.invalidateQueries({ queryKey: ['admin'] }))} />
            ) },
          ]} /><Pagination page={pays.data.page} totalPages={pays.data.total_pages} onPage={setPayPage} /></>}
        </QueryBlock>
      </Section>
    </Stack>
  );
}

export const AdminDashboard = () => <Wrap><DashboardInner /></Wrap>;
export const AdminEstablishments = () => <Wrap><EstablishmentsInner /></Wrap>;
export const AdminProfiles = () => <Wrap><ProfilesInner /></Wrap>;
export const AdminUsers = () => <Wrap><UsersInner /></Wrap>;
export const AdminBilling = () => <Wrap><BillingInner /></Wrap>;
