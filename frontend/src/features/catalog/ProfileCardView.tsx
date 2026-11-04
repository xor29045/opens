import { eur } from '../../lib/format';
import type { ProfileCard } from './types';

// Same markup and styles (.os-card in styles/global.css) as the server-rendered
// card of the home page, so both catalogues look identical.
export default function ProfileCardView({ card }: { card: ProfileCard }) {
  const place = card.establishment.city?.name ?? card.establishment.region?.name;
  return (
    <a className="os-card" href={`/profils/${card.slug}`}>
      <div className="os-card__media">
        {card.photo_url && <img src={card.photo_url} alt="" loading="lazy" decoding="async" />}
      </div>
      {card.price_from_cents != null && <span className="os-card__badge">dès {eur(card.price_from_cents)}</span>}
      <div className="os-card__body">
        <h3 className="os-card__name">
          {card.display_name}
        </h3>
        <p className="os-card__meta">
          {card.establishment.display_name}
          {place ? ` · ${place}` : ''}
        </p>
        {card.headline && <p className="os-card__headline">{card.headline}</p>}
      </div>
    </a>
  );
}
