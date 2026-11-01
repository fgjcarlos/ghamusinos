// ErrorBoundary: catches render errors of a subtree and shows a
// fallback instead of unmounting the whole SPA. React still requires a
// class component for this.
//
// Used around the lazily loaded dashboard route: if the ECharts chunk
// fails to download (offline, stale deploy), the user gets a message
// instead of a blank page.

import { Component, type ErrorInfo, type ReactNode } from 'react';

export interface ErrorBoundaryProps {
  children: ReactNode;
  /** Copy shown when the subtree fails. */
  message?: string;
}

interface ErrorBoundaryState {
  failed: boolean;
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { failed: false };

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error('ErrorBoundary captured a render error', error, info);
  }

  render(): ReactNode {
    if (this.state.failed) {
      return (
        <div role="alert" data-testid="error-boundary-fallback">
          {this.props.message ?? 'No se pudo cargar esta sección.'}
        </div>
      );
    }
    return this.props.children;
  }
}
