'use client';
import React from 'react';
import { Alert } from '@cloudscape-design/components';

interface State { hasError: boolean; message: string }
export class ErrorBoundary extends React.Component<React.PropsWithChildren, State> {
  state: State = { hasError: false, message: '' };
  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, message: error.message };
  }
  render() {
    if (this.state.hasError) {
      return (
        <Alert type="error" header="Something went wrong">
          {this.state.message}
        </Alert>
      );
    }
    return this.props.children;
  }
}
