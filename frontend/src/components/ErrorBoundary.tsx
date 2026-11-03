import { Component, type ErrorInfo, type ReactNode } from 'react';
import { ErrorState } from './ui';

interface Props {
  children: ReactNode;
}
interface State {
  error: Error | null;
}

// Catches render errors of an island so that the page shows a message
// instead of an empty dark screen.
export default class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('render error', error, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return <ErrorState message={`Erreur d’affichage : ${this.state.error.message}`} onRetry={() => window.location.reload()} />;
    }
    return this.props.children;
  }
}
