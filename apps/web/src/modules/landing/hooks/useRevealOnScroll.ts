import { useEffect, useRef, useState } from "react";

/**
 * Trigger 1 lần khi phần tử cuộn vào viewport — dùng để phối với animate-fade-in/
 * animate-slide-up (index.css) cho hiệu ứng "reveal on scroll" ở landing page.
 * Tôn trọng prefers-reduced-motion: hiện luôn, không animate.
 */
export function useRevealOnScroll<T extends HTMLElement>(threshold = 0.2) {
  const ref = useRef<T | null>(null);
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    const node = ref.current;
    if (!node) return;

    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      setIsVisible(true);
      return;
    }

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setIsVisible(true);
          observer.disconnect();
        }
      },
      { threshold, rootMargin: "0px 0px -60px 0px" },
    );

    observer.observe(node);
    return () => observer.disconnect();
  }, [threshold]);

  return { ref, isVisible };
}
