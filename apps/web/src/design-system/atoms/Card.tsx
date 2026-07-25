import React from "react";

export interface CardProps extends React.HTMLAttributes<HTMLDivElement> {
  children: React.ReactNode;
  variant?: "flat" | "bordered" | "interactive";
  padding?: "none" | "sm" | "md" | "lg";
}

/**
 * Card atom component for structured glassmorphism container panels.
 */
export const Card: React.FC<CardProps> = ({
  children,
  variant = "bordered",
  padding = "md",
  className = "",
  ...props
}) => {
  const paddingStyles = {
    none: "p-0",
    sm: "p-3",
    md: "p-4 sm:p-5",
    lg: "p-6 sm:p-8",
  };

  const variantStyles = {
    flat: "bg-[var(--surface)]",
    bordered:
      "bg-[var(--surface)] border border-[var(--border)] shadow-sm rounded-2xl",
    interactive:
      "bg-[var(--surface)] border border-[var(--border)] rounded-2xl transition-all duration-200 hover:border-[var(--accent)] hover:bg-[var(--bg-hover)] cursor-pointer active:scale-[0.99]",
  };

  return (
    <div
      className={`${variantStyles[variant]} ${paddingStyles[padding]} ${className}`}
      {...props}
    >
      {children}
    </div>
  );
};

export default Card;
