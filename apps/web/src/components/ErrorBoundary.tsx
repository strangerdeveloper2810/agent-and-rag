/**
 * ErrorBoundary — bắt lỗi rendering trong toàn bộ cây component.
 *
 * Khi một component con throw error trong lúc render, ErrorBoundary
 * bắt lỗi và hiển thị fallback UI thay vì crash trắng toàn bộ app.
 *
 * Dùng class component vì React hiện tại chỉ hỗ trợ
 * `componentDidCatch` + `getDerivedStateFromError` trên class.
 */

import { Component, type ErrorInfo, type ReactNode } from "react";
import { withTranslation, type WithTranslation } from "react-i18next";

interface Props extends WithTranslation {
  children: ReactNode;
  /** Fallback UI tùy chỉnh (nếu không truyền thì dùng mặc định). */
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

/**
 * Class component không dùng được hook `useTranslation` — inject `t`/`i18n`
 * qua `withTranslation()` HOC thay vào đó (xem export ở cuối file).
 */
class ErrorBoundaryBase extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    // Log ra console để dev debug; production có thể gửi về Sentry/etc.
    console.error("[ErrorBoundary]", error, info.componentStack);
  }

  handleReset = () => {
    this.setState({ hasError: false, error: null });
  };

  render() {
    if (this.state.hasError) {
      // Nếu có fallback tùy chỉnh thì dùng
      if (this.props.fallback) {
        return this.props.fallback;
      }

      // Fallback mặc định — Luxury Dark theme
      return (
        <div
          className="flex min-h-screen items-center justify-center px-4"
          style={{ backgroundColor: "#0b0d14" }}
        >
          <div
            className="w-full max-w-md rounded-2xl p-8 text-center"
            style={{
              backgroundColor: "#131724",
              border: "1px solid #21283b",
              boxShadow: "0 16px 48px rgba(0, 0, 0, 0.4)",
            }}
          >
            {/* Icon */}
            <div
              className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-xl text-2xl"
              style={{
                backgroundColor: "rgba(239, 68, 68, 0.12)",
                color: "#ef4444",
              }}
            >
              !
            </div>

            <h2
              className="mb-2 text-lg font-semibold"
              style={{ color: "#f8fafc" }}
            >
              {this.props.t("errors:boundary.title")}
            </h2>

            <p
              className="mb-6 text-sm leading-relaxed"
              style={{ color: "#94a3b8" }}
            >
              {this.props.t("errors:boundary.description")}
            </p>

            {/* Error details — chỉ hiện trong development */}
            {import.meta.env.DEV && this.state.error && (
              <div
                className="mb-6 rounded-xl px-4 py-3 text-left text-xs font-mono"
                style={{
                  backgroundColor: "rgba(239, 68, 68, 0.08)",
                  border: "1px solid rgba(239, 68, 68, 0.2)",
                  color: "#fca5a5",
                  maxHeight: "160px",
                  overflowY: "auto",
                  whiteSpace: "pre-wrap",
                  wordBreak: "break-word",
                }}
              >
                {this.state.error.message}
              </div>
            )}

            <button
              type="button"
              onClick={this.handleReset}
              className="rounded-xl px-6 py-2.5 text-sm font-semibold text-white transition-colors"
              style={{ backgroundColor: "#f59e0b" }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor = "#d97706";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = "#f59e0b";
              }}
            >
              {this.props.t("common:retry")}
            </button>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}

export const ErrorBoundary = withTranslation()(ErrorBoundaryBase);
export default ErrorBoundary;
