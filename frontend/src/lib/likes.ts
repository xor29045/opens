import { atom } from 'nanostores';

/**
 * Likes and passes of the "Découvrir" deck (ADR-023). There are no client
 * accounts: choices are kept in this browser only (localStorage) and the API
 * only receives the ids to leave out of the deck or to describe.
 */
export interface LikedProfile {
  id: string;
  slug: string;
  display_name: string;
  establishment_name: string;
  city: string | null;
  photo: string | null;
  at: string;
}

interface State {
  liked: LikedProfile[];
  passed: string[];
}

const KEY = 'os-deck-v1';
// Keep the request bodies small (the API caps the exclusion list at 1000).
const MAX_LIKED = 200;
const MAX_PASSED = 800;

function read(): State {
  if (typeof window === 'undefined') return { liked: [], passed: [] };
  try {
    const raw = JSON.parse(window.localStorage.getItem(KEY) ?? '{}') as Partial<State>;
    return {
      liked: Array.isArray(raw.liked) ? raw.liked.filter((l) => l && typeof l.id === 'string') : [],
      passed: Array.isArray(raw.passed) ? raw.passed.filter((id) => typeof id === 'string') : [],
    };
  } catch {
    return { liked: [], passed: [] };
  }
}

export const likesAtom = atom<State>(read());

function write(next: State) {
  const state = { liked: next.liked.slice(0, MAX_LIKED), passed: next.passed.slice(-MAX_PASSED) };
  likesAtom.set(state);
  try {
    window.localStorage.setItem(KEY, JSON.stringify(state));
  } catch {
    // Private mode or full storage: the choices last for this visit only.
  }
}

// Another tab changed the choices.
if (typeof window !== 'undefined') {
  window.addEventListener('storage', (e) => {
    if (e.key === KEY) likesAtom.set(read());
  });
}

export function like(p: Omit<LikedProfile, 'at'>) {
  const s = likesAtom.get();
  write({
    liked: [{ ...p, at: new Date().toISOString() }, ...s.liked.filter((l) => l.id !== p.id)],
    passed: s.passed.filter((id) => id !== p.id),
  });
}

export function pass(id: string) {
  const s = likesAtom.get();
  write({ liked: s.liked.filter((l) => l.id !== id), passed: [...s.passed.filter((x) => x !== id), id] });
}

/** Cancels a like or a pass: the profile comes back in the deck. */
export function forget(id: string) {
  const s = likesAtom.get();
  write({ liked: s.liked.filter((l) => l.id !== id), passed: s.passed.filter((x) => x !== id) });
}

export function clearPassed() {
  write({ ...likesAtom.get(), passed: [] });
}

export function excluded(): string[] {
  const s = likesAtom.get();
  return [...s.liked.map((l) => l.id), ...s.passed];
}

