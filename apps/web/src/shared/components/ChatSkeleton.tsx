import React from "react";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * ChatSkeleton component — Ultra-sleek, realistic conversation skeleton UI.
 */
export const ChatSkeleton: React.FC = () => {
  return (
    <div className="mx-auto max-w-3xl space-y-7 px-4 py-6 sm:px-6 animate-fade-in">
      {/* User Message Skeleton 1 */}
      <div className="flex justify-end">
        <div className="w-[65%] sm:w-[50%] rounded-[22px] rounded-tr-md p-4 bg-muted/40 border border-border/40 space-y-2">
          <Skeleton className="h-4 w-[85%] bg-primary/20" />
          <Skeleton className="h-3.5 w-[55%] bg-muted/60" />
        </div>
      </div>

      {/* AI Assistant Message Skeleton 1 */}
      <div className="flex gap-3.5">
        <Skeleton className="h-9 w-9 rounded-full bg-primary/20 shrink-0 border border-primary/30" />

        <div className="min-w-0 flex-1 space-y-3.5">
          {/* Agent Badge skeleton */}
          <div className="flex items-center gap-2">
            <Skeleton className="h-5 w-28 rounded-lg bg-primary/15" />
            <Skeleton className="h-4 w-16 rounded-md bg-muted/50" />
          </div>

          {/* Tool execution group skeleton */}
          <Skeleton className="h-10 w-full rounded-2xl bg-card/70 border border-border/60" />

          {/* Prose lines skeleton */}
          <div className="space-y-2.5 pt-1">
            <Skeleton className="h-4 w-[92%] bg-muted/80" />
            <Skeleton className="h-4 w-[85%] bg-muted/70" />
            <Skeleton className="h-4 w-[60%] bg-muted/60" />
          </div>

          {/* Code block skeleton */}
          <div className="my-3 overflow-hidden rounded-2xl border border-border/60 bg-[#0d1117]/80 p-4 space-y-3">
            <div className="flex justify-between items-center border-b border-white/10 pb-2">
              <Skeleton className="h-3.5 w-16 bg-indigo-500/30" />
              <Skeleton className="h-3.5 w-12 bg-white/15" />
            </div>
            <Skeleton className="h-3.5 w-[75%] bg-slate-700/50" />
            <Skeleton className="h-3.5 w-[50%] bg-slate-700/40" />
            <Skeleton className="h-3.5 w-[65%] bg-slate-700/50" />
          </div>
        </div>
      </div>

      {/* User Message Skeleton 2 */}
      <div className="flex justify-end">
        <div className="w-[55%] sm:w-[40%] rounded-[22px] rounded-tr-md p-4 bg-muted/40 border border-border/40 space-y-2">
          <Skeleton className="h-4 w-[80%] bg-primary/20" />
        </div>
      </div>

      {/* AI Assistant Message Skeleton 2 */}
      <div className="flex gap-3.5">
        <Skeleton className="h-9 w-9 rounded-full bg-primary/20 shrink-0 border border-primary/30" />
        <div className="min-w-0 flex-1 space-y-3">
          <div className="flex items-center gap-2">
            <Skeleton className="h-5 w-24 rounded-lg bg-primary/15" />
          </div>
          <div className="space-y-2.5">
            <Skeleton className="h-4 w-[88%] bg-muted/80" />
            <Skeleton className="h-4 w-[70%] bg-muted/60" />
          </div>
        </div>
      </div>
    </div>
  );
};

export default ChatSkeleton;
