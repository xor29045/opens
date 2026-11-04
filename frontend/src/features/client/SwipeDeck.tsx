import { useCallback, useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Button, HStack, Stack, Text } from '@chakra-ui/react';
import { EmptyState } from '../../components/ui';
import { QueryBlock } from '../dashboard/shared';
import { api } from '../../lib/api/client';
import { describeError } from '../../lib/api/problem';
import { clearPassed, excluded, forget, like, pass } from '../../lib/likes';
import DeckFilters, { countFilters, filtersFromQuery, filtersToQuery, loadFilters, saveFilters, type Filters } from './DeckFilters';
import type { components } from '../../lib/api/schema';
import { Roses } from '../../components/Rose';

type Card = components['schemas']['DiscoverCard'];
type Deck = { items: Card[]; remaining: number };
type Choice = 'like' | 'pass';

const SWIPE_THRESHOLD = 110; // px before a release counts as a choice
const EXIT_MS = 260;
const BATCH = 15;

// The deck is anonymous (ADR-023): the ids already liked or passed, kept in
// this browser, are left out by the server.
function fetchDeck(filters: Filters, also: Iterable<string> = [], signal?: AbortSignal) {
  return api<Deck>('/discover', {
    method: 'POST', auth: false, signal,
    body: { exclude: [...new Set([...excluded(), ...also])], limit: BATCH, filters },
  });
}

/**
 * "Découvrir": one card at a time. Swipe (or ← / →, or the buttons) to pass
 * or like. A like adds the profile to the favorites of this browser; a pass
 * hides it from the deck. The last choices can be undone.
 */
export default function SwipeDeck() {
  // A query string (navbar search, shared link) wins over the filters
  // remembered in this browser, and becomes the remembered set.
  const [filters, setFilters] = useState<Filters>(() => {
    const fromUrl = filtersFromQuery(new URLSearchParams(window.location.search));
    if (fromUrl) saveFilters(fromUrl);
    return fromUrl ?? loadFilters();
  });
  const filterKey = useMemo(() => JSON.stringify(filters), [filters]);
  // Keep the address bar in step with the filters (no new history entry).
  useEffect(() => {
    const qs = filtersToQuery(filters);
    window.history.replaceState(null, '', qs ? `${window.location.pathname}?${qs}` : window.location.pathname);
  }, [filters]);
  const first = useQuery({
    queryKey: ['discover', 'first', filterKey],
    queryFn: ({ signal }) => fetchDeck(filters, [], signal),
    staleTime: 0,
    gcTime: 0,
  });
  const apply = (f: Filters) => {
    saveFilters(f);
    setFilters(f);
  };
  return (
    <Stack gap="4" align="center">
      <DeckFilters value={filters} onApply={apply} />
      <QueryBlock isPending={first.isPending} isError={first.isError} error={first.error} onRetry={() => first.refetch()}>
        {first.data && <DeckBody key={filterKey} initial={first.data} filters={filters} onClearFilters={() => apply({})} />}
      </QueryBlock>
    </Stack>
  );
}

function DeckBody({ initial, filters, onClearFilters }: { initial: Deck; filters: Filters; onClearFilters: () => void }) {
  const [queue, setQueue] = useState<Card[]>(initial.items);
  const [remaining, setRemaining] = useState(initial.remaining);
  const [history, setHistory] = useState<{ card: Card; choice: Choice }[]>([]);
  const [drag, setDrag] = useState({ x: 0, y: 0, active: false });
  const [exiting, setExiting] = useState<Choice | null>(null);
  const [photo, setPhoto] = useState(0);
  const [announce, setAnnounce] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const start = useRef<{ x: number; y: number; moved: boolean } | null>(null);
  const seen = useRef(new Set(initial.items.map((c) => c.id)));

  const top = queue[0];
  const next = queue[1];

  // Fetch the next batch when the queue runs low.
  useEffect(() => {
    if (queue.length > 3 || remaining <= queue.length || loadingMore) return;
    setLoadingMore(true);
    fetchDeck(filters, seen.current)
      .then((d) => {
        const fresh = d.items.filter((c) => !seen.current.has(c.id));
        fresh.forEach((c) => seen.current.add(c.id));
        setQueue((q) => [...q, ...fresh]);
        setRemaining(d.remaining + queue.length);
      })
      .catch(() => undefined)
      .finally(() => setLoadingMore(false));
  }, [queue.length, remaining, loadingMore, filters]);

  const choose = useCallback(
    (choice: Choice) => {
      if (!top || exiting) return;
      setExiting(choice);
      setError(null);
      const card = top;
      window.setTimeout(() => {
        setQueue((q) => q.slice(1));
        setRemaining((r) => Math.max(0, r - 1));
        setHistory((h) => [...h.slice(-9), { card, choice }]);
        setDrag({ x: 0, y: 0, active: false });
        setExiting(null);
        setPhoto(0);
        setAnnounce(choice === 'like' ? `${card.display_name} ajouté·e à tes favoris` : `${card.display_name} passé·e`);
      }, EXIT_MS);
      if (choice === 'like') {
        like({ id: card.id, slug: card.slug, display_name: card.display_name, establishment_name: card.establishment_name,
          city: card.city ?? null, photo: card.photos[0] ?? null });
      } else {
        pass(card.id);
      }
    },
    [top, exiting],
  );

  const undo = useCallback(async () => {
    const last = history[history.length - 1];
    if (!last || exiting) return;
    setError(null);
    forget(last.card.id);
    setHistory((h) => h.slice(0, -1));
    setQueue((q) => [last.card, ...q]);
    setRemaining((r) => r + 1);
    setPhoto(0);
    setAnnounce(`${last.card.display_name} revient dans la pile`);
  }, [history, exiting]);

  // Keyboard: ← pass, → like, Backspace undo.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (target && ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)) return;
      if (e.key === 'ArrowLeft') { e.preventDefault(); choose('pass'); }
      else if (e.key === 'ArrowRight') { e.preventDefault(); choose('like'); }
      else if (e.key === 'Backspace') { e.preventDefault(); void undo(); }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [choose, undo]);

  const onPointerDown = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (exiting) return;
    (e.currentTarget as HTMLDivElement).setPointerCapture(e.pointerId);
    start.current = { x: e.clientX, y: e.clientY, moved: false };
    setDrag({ x: 0, y: 0, active: true });
  };
  const onPointerMove = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (!start.current) return;
    const dx = e.clientX - start.current.x;
    const dy = e.clientY - start.current.y;
    if (Math.abs(dx) > 6 || Math.abs(dy) > 6) start.current.moved = true;
    setDrag({ x: dx, y: dy, active: true });
  };
  const onPointerUp = (e: ReactPointerEvent<HTMLDivElement>) => {
    const s = start.current;
    start.current = null;
    if (!s) return;
    const dx = e.clientX - s.x;
    if (!s.moved && top) {
      // A tap changes the photo: left third = previous, elsewhere = next.
      const rect = e.currentTarget.getBoundingClientRect();
      const left = e.clientX - rect.left < rect.width / 3;
      setPhoto((p) => (left ? Math.max(0, p - 1) : Math.min(top.photos.length - 1, p + 1)));
      setDrag({ x: 0, y: 0, active: false });
      return;
    }
    if (dx > SWIPE_THRESHOLD) choose('like');
    else if (dx < -SWIPE_THRESHOLD) choose('pass');
    else setDrag({ x: 0, y: 0, active: false });
  };

  const restore = async () => {
    setError(null);
    try {
      clearPassed();
      const d = await fetchDeck(filters);
      seen.current = new Set(d.items.map((c) => c.id));
      setQueue(d.items);
      setRemaining(d.remaining);
      setHistory([]);
    } catch (e) {
      setError(describeError(e));
    }
  };

  // Position of the top card: follows the pointer, flies away on a choice.
  const exitX = exiting === 'like' ? 1.6 : exiting === 'pass' ? -1.6 : 0;
  const x = exiting ? `${exitX * 100}%` : `${drag.x}px`;
  const rotate = exiting ? exitX * 12 : drag.x / 18;
  const likeOpacity = exiting === 'like' ? 1 : Math.max(0, Math.min(1, drag.x / SWIPE_THRESHOLD));
  const passOpacity = exiting === 'pass' ? 1 : Math.max(0, Math.min(1, -drag.x / SWIPE_THRESHOLD));

  return (
    <Stack gap="5" align="center">
      <span className="visually-hidden" aria-live="assertive">{announce}</span>

      {top ? (
        <div className="os-deck">
          {next && <DeckCard card={next} photo={0} behind />}
          <div
            className={`os-deck__card${drag.active && !exiting ? ' is-dragging' : ''}`}
            style={{ transform: `translate(${x}, ${exiting ? 0 : drag.y * 0.15}px) rotate(${rotate}deg)` }}
            onPointerDown={onPointerDown}
            onPointerMove={onPointerMove}
            onPointerUp={onPointerUp}
            onPointerCancel={() => { start.current = null; setDrag({ x: 0, y: 0, active: false }); }}
            role="group"
            aria-roledescription="carte"
            aria-label={top.display_name}
          >
            <span className="os-deck__stamp os-deck__stamp--like" style={{ opacity: likeOpacity }}>J’aime</span>
            <span className="os-deck__stamp os-deck__stamp--pass" style={{ opacity: passOpacity }}>Passer</span>
            <DeckCard card={top} photo={photo} />
          </div>
        </div>
      ) : (
        <EmptyState title={countFilters(filters) ? 'Aucun profil ne correspond à tes filtres' : 'Tu as vu tous les profils disponibles'}>
          {countFilters(filters)
            ? 'Élargis tes filtres, ou revois les profils que tu as passés.'
            : 'Reviens plus tard pour découvrir de nouvelles personnes, ou revois celles que tu as passées.'}
        </EmptyState>
      )}

      {top ? (
        <HStack gap="5" justify="center">
          <button type="button" className="os-deck__btn" onClick={() => void undo()} disabled={history.length === 0 || !!exiting} aria-label="Annuler le dernier choix (Retour arrière)" title="Annuler (Retour arrière)">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true"><path d="M9 14L4 9l5-5" /><path d="M4 9h10a6 6 0 010 12h-3" /></svg>
          </button>
          <button type="button" className="os-deck__btn os-deck__btn--lg os-deck__btn--pass" onClick={() => choose('pass')} disabled={!!exiting} aria-label="Passer (flèche gauche)" title="Passer (←)">
            <svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" aria-hidden="true"><path d="M6 6l12 12M18 6L6 18" /></svg>
          </button>
          <button type="button" className="os-deck__btn os-deck__btn--lg os-deck__btn--like" onClick={() => choose('like')} disabled={!!exiting} aria-label="J’aime, ajouter aux favoris (flèche droite)" title="J’aime (→)">
            <svg width="30" height="30" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M12 21s-7.5-4.6-9.5-9.2C1.1 8.5 3.2 5 6.6 5c2 0 3.4 1.1 4.2 2.4h2.4C14 6.1 15.4 5 17.4 5c3.4 0 5.5 3.5 4.1 6.8C19.5 16.4 12 21 12 21z" /></svg>
          </button>
          <a className="os-deck__btn" href={`/profils/${top.slug}`} aria-label={`Voir la fiche de ${top.display_name}`} title="Voir la fiche">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true"><circle cx="12" cy="12" r="9" /><path d="M12 11v6M12 7.5v.5" /></svg>
          </a>
        </HStack>
      ) : (
        <HStack gap="3" wrap="wrap" justify="center">
          <Button colorPalette="brand" onClick={() => void restore()}>Revoir les profils passés</Button>
          {countFilters(filters) > 0 && <Button variant="outline" onClick={onClearFilters}>Effacer les filtres</Button>}
          <Button asChild variant="outline"><a href="/favoris">Voir mes favoris</a></Button>
          {history.length > 0 && <Button variant="ghost" onClick={() => void undo()}>Annuler le dernier choix</Button>}
        </HStack>
      )}
      <Text fontSize="xs" color="fg.subtle" textAlign="center">
        Glisse la carte · ← passer · → j’aime · Retour arrière pour annuler · touche la photo pour la suivante
      </Text>
      <Text fontSize="xs" color="fg.subtle" textAlign="center">
        Sans compte : tes choix restent dans ce navigateur. <a href="/favoris">Mes favoris</a>
      </Text>
      {error && <Text color="danger" role="alert">{error}</Text>}
    </Stack>
  );
}

function DeckCard({ card, photo, behind = false }: { card: Card; photo: number; behind?: boolean }) {
  const src = card.photos[Math.min(photo, card.photos.length - 1)];
  return (
    <div className={`os-deck__face${behind ? ' os-deck__face--behind' : ''}`} aria-hidden={behind}>
      {src ? <img src={src} alt="" draggable={false} /> : <div className="os-deck__nophoto" />}
      {!behind && card.photos.length > 1 && (
        <div className="os-deck__bars" aria-hidden="true">
          {card.photos.map((_, i) => <span key={i} className={i === photo ? 'is-on' : undefined} />)}
        </div>
      )}
      {!behind && (
        <div className="os-deck__info">
          <h2>
            {card.display_name}{card.age ? <span className="os-deck__age">, {card.age}</span> : null}
          </h2>
          <p className="os-deck__meta">
            {card.establishment_name}
            {card.city ? ` · ${card.city}` : ''}
            {card.price_from_cents != null && <> · dès <Roses cents={card.price_from_cents} /></>}
          </p>
          {card.headline && <p className="os-deck__headline">{card.headline}</p>}
          {card.categories.length > 0 && (
            <div className="os-deck__tags">
              {card.categories.slice(0, 4).map((c) => <span key={c}>{c}</span>)}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
