import { Component, ErrorInfo, ReactNode } from "react";

// One panel throwing must cost that panel, not the page. Without a boundary
// a render error anywhere (a stale brush range, a malformed response) unmounts
// the whole app and the reader is left with a blank tab and no reading.
export default class ErrorBoundary extends Component<
  { label: string; children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error(`[${this.props.label}] render failed`, error, info.componentStack);
  }

  render() {
    if (this.state.failed) {
      return (
        <p role="alert" style={{ color: "var(--status-critical-text)" }}>
          {this.props.label} could not be displayed — this is a fault in the page, not a
          sign that nothing happened. The rest of the dashboard is unaffected;
          reloading usually clears it.{" "}
          <button className="linklike" onClick={() => this.setState({ failed: false })}>
            try again
          </button>
        </p>
      );
    }
    return this.props.children;
  }
}
