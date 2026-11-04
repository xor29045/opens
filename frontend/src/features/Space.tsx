import type { ComponentType } from 'react';
import Providers from '../components/Providers';
import Guard from '../components/Guard';
import ErrorBoundary from '../components/ErrorBoundary';
import AppShell from '../components/AppShell';
import type { Role } from '../lib/auth/session';
import ManagerOverview from './manager/ManagerOverview';
import EstablishmentForm from './manager/EstablishmentForm';
import ProfilesList from './manager/ProfilesList';
import { Subscription, Invoices, ManagerStats } from './manager/Billing';
import ProfileEditor from './profile/ProfileEditor';
import { ProStatus, ProPreview } from './pro/ProPages';
import Security from './account/Security';
import { AdminDashboard, AdminEstablishments, AdminProfiles, AdminUsers, AdminBilling } from './admin/AdminPages';

/**
 * One `client:only` island per private page (§6.1): providers, client-side
 * guard (UX only — the API enforces access), shell and page content.
 * Props stay serialisable (a page key and an optional id).
 */
const pages: Record<string, { role: Role; title: string; component: ComponentType<{ id?: string }> }> = {
  'manager.overview': { role: 'MANAGER', title: 'Vue d’ensemble', component: ManagerOverview },
  'manager.establishment': { role: 'MANAGER', title: 'Établissement', component: EstablishmentForm },
  'manager.profiles': { role: 'MANAGER', title: 'Professionnel·les', component: ProfilesList },
  'manager.profile': { role: 'MANAGER', title: 'Profil', component: ({ id }) => <ProfileEditor mode="manager" id={id} /> },
  'manager.subscription': { role: 'MANAGER', title: 'Abonnement', component: Subscription },
  'manager.invoices': { role: 'MANAGER', title: 'Factures', component: Invoices },
  'manager.stats': { role: 'MANAGER', title: 'Statistiques', component: ManagerStats },
  'manager.security': { role: 'MANAGER', title: 'Sécurité du compte', component: Security },
  'pro.profile': { role: 'PROFESSIONAL', title: 'Mon profil', component: () => <ProfileEditor mode="professional" /> },
  'pro.status': { role: 'PROFESSIONAL', title: 'Statut de publication', component: ProStatus },
  'pro.preview': { role: 'PROFESSIONAL', title: 'Aperçu', component: ProPreview },
  'pro.security': { role: 'PROFESSIONAL', title: 'Sécurité du compte', component: Security },
  'admin.dashboard': { role: 'ADMIN', title: 'Tableau de bord', component: AdminDashboard },
  'admin.establishments': { role: 'ADMIN', title: 'Établissements', component: AdminEstablishments },
  'admin.profiles': { role: 'ADMIN', title: 'Profils', component: AdminProfiles },
  'admin.users': { role: 'ADMIN', title: 'Utilisateurs', component: AdminUsers },
  'admin.billing': { role: 'ADMIN', title: 'Abonnements et paiements', component: AdminBilling },
};

export default function Space({ page, id }: { page: string; id?: string }) {
  const def = pages[page];
  if (!def) return null;
  const Content = def.component;
  return (
    <Providers>
      <Guard role={def.role}>
        <AppShell role={def.role} title={def.title}>
          <ErrorBoundary>
            <Content id={id} />
          </ErrorBoundary>
        </AppShell>
      </Guard>
    </Providers>
  );
}
