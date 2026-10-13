import { Component, type ErrorInfo, type ReactNode } from "react";

/**
 * Catches render errors of the page below it. AppShell keys it by pathname, so navigating to another page
 * mounts a fresh boundary (error cleared) while header and sidebar stay usable.
 */
export class ErrorBoundary extends Component<{ fallback: ReactNode; children: ReactNode }, { failed: boolean }> {
  override state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  override componentDidCatch(error: Error, info: ErrorInfo) {
    // Client-side only until a UI error reporting endpoint exists.
    console.error("page render failed", error, info.componentStack);
  }

  override render() {
    return this.state.failed ? this.props.fallback : this.props.children;
  }
}
