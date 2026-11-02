// RFC 9457 Problem Details, as returned by the API (§5.1).
export interface FieldError {
  field: string;
  code: string;
}

export interface Problem {
  type: string;
  title: string;
  status: number;
  detail?: string;
  instance?: string;
  code?: string;
  request_id?: string;
  errors?: FieldError[];
}

/** Error raised by the API client. Messages are French and safe to display. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId?: string;
  readonly fields: FieldError[];
  readonly retryAfterSeconds?: number;

  constructor(problem: Problem, retryAfterSeconds?: number) {
    super(problem.detail ?? problem.title);
    this.name = 'ApiError';
    this.status = problem.status;
    this.code = problem.code ?? 'unknown';
    this.requestId = problem.request_id;
    this.fields = problem.errors ?? [];
    this.retryAfterSeconds = retryAfterSeconds;
  }

  /** Field-level message, or the generic one. */
  fieldMessage(field: string): string | undefined {
    const f = this.fields.find((e) => e.field === field);
    return f ? messageFor(f.code) : undefined;
  }
}

const codeMessages: Record<string, string> = {
  required: 'Champ obligatoire.',
  invalid_username: 'Entre 3 et 30 caractères : lettres, chiffres, _ . -',
  reserved_username: 'Ce pseudo est réservé.',
  password_too_short: 'Le mot de passe est trop court (12 caractères minimum).',
  password_too_long: 'Le mot de passe est trop long (128 caractères maximum).',
  password_breached: 'Ce mot de passe figure dans une liste de mots de passe compromis.',
  password_equals_username: 'Le mot de passe ne peut pas être le pseudo.',
  password_unchanged: 'Choisis un mot de passe différent de l’actuel.',
  invalid_phone: 'Numéro de téléphone invalide (format international, ex. +32 470 12 34 56).',
  invalid_url: 'Lien invalide (https uniquement).',
  invalid_url_domain: 'Le lien doit pointer vers le réseau social correspondant.',
  invalid_company_number: 'Numéro BCE invalide (10 chiffres, clé de contrôle).',
  invalid_approval_number: 'Numéro d’agrément invalide.',
  invalid_opening_hours: 'Horaires invalides.',
  too_long: 'Texte trop long.',
  invalid_characters: 'Caractères non autorisés.',
  under_18: 'La personne doit être majeure (18 ans minimum).',
  invalid_date: 'Date invalide.',
  out_of_range: 'Valeur hors des bornes autorisées.',
  invalid_value: 'Valeur non autorisée.',
  invalid_language: 'Langue non reconnue.',
  must_contain_1_to_15_items: 'Entre 1 et 15 tarifs.',
  unsupported_image_format: 'Format non supporté (JPEG, PNG ou WebP).',
  image_too_large: 'Image trop grande.',
  invalid_image: 'Image invalide.',
  file_too_large: 'Fichier trop volumineux (10 Mo maximum).',
  too_many_photos: 'Maximum 12 photos par profil.',
  establishment_not_selectable: 'Cette maison n’accepte pas d’inscription pour le moment.',
  establishment_pending_quota: 'Cette maison a trop d’inscriptions en attente.',
  attestation_required: 'Atteste le profil avant de l’activer.',
  already_subscribed: 'Un abonnement existe déjà pour cet établissement.',
  establishment_not_verified: 'L’établissement doit être vérifié avant de souscrire.',
  payment_provider_unavailable: 'Le prestataire de paiement est indisponible, réessaie plus tard.',
  username_taken: 'Ce pseudo est déjà utilisé.',
};

export function messageFor(code: string): string {
  return codeMessages[code] ?? 'Valeur invalide.';
}

/** Turns any thrown value into a message that can be shown to the user. */
export function describeError(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 429) return 'Trop de tentatives. Réessaie dans quelques minutes.';
    if (err.status >= 500) return 'Une erreur est survenue. Réessaie ou contacte le support.';
    if (err.fields.length > 0) return messageFor(err.fields[0]!.code);
    return err.message;
  }
  return 'Impossible de joindre le serveur.';
}
