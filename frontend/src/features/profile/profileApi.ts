import type { components } from '../../lib/api/schema';

export type Profile = components['schemas']['Profile'];
export type ProfileInput = components['schemas']['ProfileInput'];
export type Contact = components['schemas']['Contact'];
export type Rate = components['schemas']['Rate'];
export type Photo = components['schemas']['Photo'];
export type Category = components['schemas']['Category'];
export type ContactType = components['schemas']['ContactType'];
export type Visibility = components['schemas']['Visibility'];

/**
 * Endpoints of the profile editor. The manager addresses a profile by id
 * (scoped server-side); a professional never sends an identifier (§11.4b).
 */
export function profileBase(mode: 'manager' | 'professional', id?: string): string {
  return mode === 'manager' ? `/manager/profiles/${id}` : '/professional/me';
}

export const contactTypes: ContactType[] = ['PHONE', 'WHATSAPP', 'TELEGRAM', 'INSTAGRAM', 'FACEBOOK', 'WEBSITE', 'OTHER'];
export const visibilities: Visibility[] = ['PUBLIC', 'HIDDEN'];

export const profileLabels = {
  contactType: {
    PHONE: 'Téléphone', WHATSAPP: 'WhatsApp', TELEGRAM: 'Telegram', INSTAGRAM: 'Instagram',
    FACEBOOK: 'Facebook', WEBSITE: 'Site', OTHER: 'Autre',
  } as Record<string, string>,
  visibility: { PUBLIC: 'Public', HIDDEN: 'Masqué' } as Record<string, string>,
};
