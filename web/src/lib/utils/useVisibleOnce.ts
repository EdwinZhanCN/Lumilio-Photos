import { useEffect, useRef, useState, type RefObject } from "react";

/**
 * Mount-once visibility hook backed by IntersectionObserver.
 *
 * Attach the returned `ref` to the shell element you always want in the DOM.
 * Once that element comes within `rootMargin` of the viewport, `visible`
 * flips to `true` and stays `true` permanently – use it to defer mounting
 * expensive children (images, heavy component trees) until they are near the
 * visible area.
 *
 * After first trigger the observer is disconnected, so there is zero ongoing
 * overhead per item.
 *
 * @param rootMargin - IntersectionObserver rootMargin (default: 500 px above/below viewport)
 */
export function useVisibleOnce(
  rootMargin = "500px 0px",
): [RefObject<HTMLDivElement | null>, boolean] {
  const ref = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el || visible) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) {
          setVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin, threshold: 0 },
    );

    observer.observe(el);
    return () => observer.disconnect();
  }, [visible, rootMargin]);

  return [ref, visible];
}
