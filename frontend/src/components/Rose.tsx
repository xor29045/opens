// "Rose" currency of the profiles' prices: 1 rose = 1 euro (display only, the
// API still works in euro cents). Font Awesome has no rose in its free set,
// so the icon is drawn here in the same style (filled, currentColor, 1em).

export function RoseIcon({ title }: { title?: string }) {
  return (
    <svg className="os-rose" viewBox="0 0 512 512" width="1em" height="1em" fill="currentColor"
      role={title ? 'img' : undefined} aria-hidden={title ? undefined : true} aria-label={title} focusable="false">
      <path fillRule="evenodd" d="M256 44c-72 0-132 52-132 122 0 64 56 110 132 110s132-46 132-110c0-70-60-122-132-122z M206 116c22-30 72-38 102-12 24 20 22 56-4 72l-14-16c14-10 16-32 0-44-22-18-56-10-70 10z M160 168c10 44 52 72 100 68 32-2 60-20 74-46l22 10c-18 34-56 58-96 60-60 4-110-30-122-84z M228 150c4-14 20-22 34-16 12 6 14 22 4 30-6-8-16-12-26-10-6 0-10-2-12-4z" />
      <path d="M240 272h32v200c0 9-7 16-16 16s-16-7-16-16z" />
      <path d="M244 390c-18-46-62-74-120-70 6 56 50 96 112 100z" />
      <path d="M268 346c20-42 62-66 116-60-6 50-46 86-104 90z" />
    </svg>
  );
}

const number = new Intl.NumberFormat('fr-FR', { maximumFractionDigits: 2 });

/** Amount in roses from euro cents (1 rose = 1 €). */
export function rosesText(cents: number): string {
  const n = cents / 100;
  return `${number.format(n)} rose${n >= 2 ? 's' : ''}`;
}

/** "150 🌹" with an accessible label "150 roses". */
export function Roses({ cents }: { cents: number | null | undefined }) {
  if (cents == null) return <>—</>;
  return (
    <span className="os-roses" aria-label={rosesText(cents)}>
      <span aria-hidden="true">{number.format(cents / 100)}</span>
      <RoseIcon />
    </span>
  );
}
