import Providers from '../../components/Providers';
import ErrorBoundary from '../../components/ErrorBoundary';
import SwipeDeck from './SwipeDeck';

// Public island of the anonymous deck (ADR-023).
export default function DeckIsland() {
  return (
    <Providers>
      <ErrorBoundary>
        <SwipeDeck />
      </ErrorBoundary>
    </Providers>
  );
}
