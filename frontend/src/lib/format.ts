export const eur = (cents: number | null | undefined): string =>
  cents == null ? '—' : new Intl.NumberFormat('fr-BE', { style: 'currency', currency: 'EUR' }).format(cents / 100);

export const dateFr = (iso: string | null | undefined): string =>
  iso ? new Intl.DateTimeFormat('fr-BE', { dateStyle: 'medium', timeZone: 'Europe/Brussels' }).format(new Date(iso)) : '—';

export const dateTimeFr = (iso: string | null | undefined): string =>
  iso ? new Intl.DateTimeFormat('fr-BE', { dateStyle: 'short', timeStyle: 'short', timeZone: 'Europe/Brussels' }).format(new Date(iso)) : '—';

/** Labels of the publication reasons returned by the API (RM-22). */
export const reasonLabels: Record<string, string> = {
  MANAGER_ACCOUNT_INACTIVE: 'Compte du gérant inactif',
  ESTABLISHMENT_HIDDEN: 'Établissement masqué par la plateforme',
  ESTABLISHMENT_SUBSCRIPTION_INACTIVE: 'Abonnement de l’établissement inactif',
  PENDING_MANAGER_APPROVAL: 'En attente de validation par le gérant',
  PROFILE_NOT_ACTIVE: 'Profil non activé',
  PROFILE_NOT_ATTESTED: 'Âge non attesté par la maison',
  PROFILE_HIDDEN: 'Profil masqué par la plateforme',
  NO_PRIMARY_PHOTO: 'Aucune photo principale',
  PROFESSIONAL_ACCOUNT_INACTIVE: 'Compte professionnel suspendu',
  ACTIVE_PROFILES_QUOTA_EXCEEDED: 'Quota de profils publiés atteint',
};
