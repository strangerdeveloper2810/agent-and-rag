import React from "react";

export interface BadgeProps {
  children: React.ReactNode;
  variant?: "default" | "accent" | "success" | "warning" | "danger" | "mono";
  size?: "sm" | "md";
  className?: string;
  dot?: boolean;
}

export function Badge({
  children,
  variant = "default",
  size = "sm",
  className = "",
  dot = false,
}: BadgeProps) {
  const sizeStyles = {
    sm: "text-[10px] px-2 py-0.5 font-medium",
    md: "text-xs px-2.5 py-1 font-medium",
  };

  const variantStyles = {
    default:
      "bg-[var(--bg-raised)] text-[var(--text-secondary)] border border-[var(--border)]",
    accent:
      "bg-[var(--accent-bg)] text-[var(--accent)] border border-[var(--accent)]/30",
    success:
      "bg-[var(--success-bg)] text-[var(--success)] border border-[var(--success)]/30",
    warning: "bg-amber-500/10 text-amber-500 border border-amber-500/20",
    danger:
      "bg-[var(--danger-bg)] text-[var(--danger)] border border-[var(--danger)]/30",
    mono: "bg-[var(--bg-hover)] text-[var(--text-tertiary)] border border-[var(--border)] font-mono uppercase tracking-wider",
  };

  const dotColors = {
    default: "bg-[var(--text-tertiary)]",
    accent: "bg-[var(--accent)]",
    success: "bg-[var(--success)]",
    warning: "bg-amber-500",
    danger: "bg-[var(--danger)]",
    mono: "bg-[var(--text-tertiary)]",
  };

  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full ${sizeStyles[size]} ${variantStyles[variant]} ${className}`}
    >
      {dot && (
        <span className={`h-1.5 w-1.5 rounded-full ${dotColors[variant]}`} />
      )}
      {children}
    </span>
  );
}
