export const eur = (cents: number | null | undefined): string =>
  cents == null ? '—' : new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'EUR' }).format(cents / 100);

export const dateFr = (iso: string | null | undefined): string =>
  iso ? new Intl.DateTimeFormat('fr-FR', { dateStyle: 'medium', timeZone: 'Europe/Paris' }).format(new Date(iso)) : '—';

export const dateTimeFr = (iso: string | null | undefined): string =>
  iso ? new Intl.DateTimeFormat('fr-FR', { dateStyle: 'short', timeStyle: 'short', timeZone: 'Europe/Paris' }).format(new Date(iso)) : '—';

/** Labels of the publication reasons returned by the API (RM-22). */
export const reasonLabels: Record<string, string> = {
  MANAGER_ACCOUNT_INACTIVE: 'Compte du gérant inactif',
  ESTABLISHMENT_HIDDEN: 'Établissement masqué par la plateforme',
  ESTABLISHMENT_SUBSCRIPTION_INACTIVE: 'Abonnement de l’établissement inactif',
  PROFILE_NOT_ACTIVE: 'Profil non activé',
  PROFILE_HIDDEN: 'Profil masqué par la plateforme',
  PROFESSIONAL_ACCOUNT_INACTIVE: 'Compte professionnel suspendu',
  ACTIVE_PROFILES_QUOTA_EXCEEDED: 'Quota de profils publiés atteint',
};
