import type { components } from '../../lib/api/schema';

export type ProfileCard = components['schemas']['ProfileCard'];
export type PublicProfile = components['schemas']['PublicProfile'];
export type PublicEstablishment = components['schemas']['PublicEstablishment'];
export type Attributes = components['schemas']['Attributes'];
export interface Paginated<T> {
  items: T[];
  page: number;
  page_size: number;
  total: number;
  total_pages: number;
}

/** Labels for contact types (display only). */
export const contactLabels: Record<string, string> = {
  PHONE: 'Téléphone',
  WHATSAPP: 'WhatsApp',
  TELEGRAM: 'Telegram',
  INSTAGRAM: 'Instagram',
  FACEBOOK: 'Facebook',
  WEBSITE: 'Site',
  OTHER: 'Autre',
};

/** Turns a stored contact value into a safe link (https only, §13.2). */
export function contactHref(type: string, value: string): string | null {
  if (type === 'PHONE') return `tel:${value}`;
  if (type === 'WHATSAPP') return `https://wa.me/${value.replace(/\D/g, '')}`;
  if (type === 'TELEGRAM') return value.startsWith('@') ? `https://t.me/${value.slice(1)}` : value;
  if (value.startsWith('https://')) return value;
  return null;
}
