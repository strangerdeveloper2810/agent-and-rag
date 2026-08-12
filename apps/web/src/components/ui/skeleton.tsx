import { cn } from "@/lib/utils";

function Skeleton({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "animate-pulse rounded-xl bg-muted/70 dark:bg-muted/40 backdrop-blur-xs",
        className
      )}
      {...props}
    />
  );
}

export { Skeleton };
