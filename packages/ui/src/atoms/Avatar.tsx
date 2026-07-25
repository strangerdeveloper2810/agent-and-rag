export interface AvatarProps {
  type: "ai" | "user";
  size?: "sm" | "md" | "lg";
  className?: string;
}

export function Avatar({ type, size = "md", className = "" }: AvatarProps) {
  const sizeStyles = {
    sm: "h-7 w-7 text-xs rounded-lg",
    md: "h-8 w-8 text-sm rounded-xl",
    lg: "h-10 w-10 text-base rounded-2xl",
  };

  if (type === "user") {
    return (
      <div
        className={`flex shrink-0 items-center justify-center font-bold font-mono border shadow-sm ${sizeStyles[size]} ${className}`}
        style={{
          backgroundColor: "var(--user-bubble-bg)",
          borderColor: "var(--user-bubble-border)",
          color: "var(--user-bubble-text)",
        }}
      >
        U
      </div>
    );
  }

  return (
    <div
      className={`flex shrink-0 items-center justify-center border shadow-sm ${sizeStyles[size]} ${className}`}
      style={{
        borderColor: "var(--border)",
        backgroundColor: "var(--surface)",
      }}
    >
      <svg
        width={16}
        height={16}
        viewBox="0 0 24 24"
        fill="none"
        stroke="var(--accent)"
        strokeWidth={1.8}
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M12 3.5c.6 3.7 1.8 4.9 5.5 5.5-3.7.6-4.9 1.8-5.5 5.5-.6-3.7-1.8-4.9-5.5-5.5 3.7-.6 4.9-1.8 5.5-5.5Z" />
      </svg>
    </div>
  );
}
