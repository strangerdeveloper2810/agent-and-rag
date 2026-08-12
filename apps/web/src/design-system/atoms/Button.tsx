import { forwardRef } from "react";

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "ghost" | "danger" | "outline";
  size?: "sm" | "md" | "lg";
  loading?: boolean;
  leftIcon?: React.ReactNode;
  rightIcon?: React.ReactNode;
}

const sizeStyles = {
  sm: "text-[11px] px-2.5 py-1.5 gap-1.5 min-h-[32px] rounded-lg",
  md: "text-[12px] px-3.5 py-2 gap-2 min-h-[38px] rounded-xl",
  lg: "text-[13px] px-4 py-2.5 gap-2.5 min-h-[44px] rounded-xl",
} as const;

const variantStyles: Record<string, string> = {
  primary: "text-white font-semibold shadow-sm",
  secondary:
    "bg-[var(--bg-raised)] text-[var(--text)] border border-[var(--border)] hover:bg-[var(--bg-hover)] hover:border-[var(--border-hover)]",
  ghost:
    "text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text)]",
  outline:
    "border border-[var(--border)] text-[var(--text)] hover:bg-[var(--bg-hover)] hover:border-[var(--accent)]",
  danger:
    "bg-[var(--danger-bg)] text-[var(--danger)] border border-red-500/20 hover:bg-red-500 hover:text-white",
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      variant = "primary",
      size = "md",
      loading = false,
      leftIcon,
      rightIcon,
      children,
      className = "",
      disabled,
      type = "button",
      ...props
    },
    ref,
  ) => {
    const base =
      "inline-flex items-center justify-center font-medium transition-all duration-150 active:scale-[0.98] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--accent)] disabled:pointer-events-none disabled:opacity-40 select-none";

    const isPrimary = variant === "primary";

    return (
      <button
        ref={ref}
        type={type}
        disabled={disabled || loading}
        className={`${base} ${sizeStyles[size]} ${variantStyles[variant]} ${className}`}
        style={
          isPrimary
            ? {
                background:
                  "linear-gradient(135deg, var(--accent-soft), var(--accent))",
              }
            : undefined
        }
        {...props}
      >
        {loading ? (
          <span className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
        ) : (
          leftIcon
        )}
        {children}
        {!loading && rightIcon}
      </button>
    );
  },
);

Button.displayName = "Button";
