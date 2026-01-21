import React, { useEffect, useRef, useState } from "react";
import "@/styles/pyramid.css";

interface LumilioAvatarProps {
  /** When true, the pyramid will spin. */
  start?: boolean;
  /** Size in rem units. Default is 1 (20rem base size). */
  size?: number;
  /** Additional CSS class name */
  className?: string;
  /** Custom style overrides */
  style?: React.CSSProperties;
}

/** Smoothly resets the element's transform to its initial state.
 *
 * Locks the current position, then uses CSS transition to roll back to the
 * initial (non-rotated) state. This provides a smooth stop animation effect.
 *
 * @param el - The HTML element to reset.
 * @param duration - The duration of the transition in milliseconds. Default is 2000.
 * @param easing - The CSS easing function for the transition. Default is "ease-out".
 */
function smoothReset(
  el: HTMLElement,
  duration: number = 2000,
  easing: string = "ease-out",
) {
  const computed = getComputedStyle(el).transform;

  if (
    !computed ||
    computed === "none" ||
    computed === "matrix(1, 0, 0, 1, 0, 0)"
  ) {
    return;
  }

  el.style.transform = computed;
  el.style.animation = "none";
  el.style.webkitAnimation = "none";
  void el.offsetHeight;
  el.style.transition = `transform ${duration}ms ${easing}`;
  el.style.webkitTransition = `transform ${duration}ms ${easing}`;
  el.style.transform = "";
  const clean = () => {
    el.style.transition = "";
    el.style.webkitTransition = "";
    el.style.transform = "";
    el.style.animation = "";
    el.style.webkitAnimation = "";
    el.removeEventListener("transitionend", clean);
    el.removeEventListener("webkitTransitionEnd", clean);
  };
  el.addEventListener("transitionend", clean);
  el.addEventListener("webkitTransitionEnd", clean);
}

/** Animated pyramid avatar component for the Lumilio assistant.
 *
 * Renders a 3D-styled pyramid that can be animated to spin when the assistant
 * is processing or thinking. The animation smoothly starts and stops, with the
 * stop animation completing the current rotation cycle before resetting to
 * the initial position.
 */
export const LumilioAvatar: React.FC<LumilioAvatarProps> = ({
  start = false,
  size = 1,
  className = "",
  style = {},
}) => {
  const axisRef = useRef<HTMLDivElement>(null);
  const animationStartTimeRef = useRef<number | null>(null);
  const stopRequestedRef = useRef<boolean>(false);
  const [isSpinning, setIsSpinning] = useState(false);

  const ANIMATION_DURATION = 2000;

  /** Calculates the remaining time until the current animation loop completes.
   *
   * @param startTime - The timestamp when the animation started.
   * @param duration - The total duration of one animation loop in milliseconds.
   * @returns The remaining time in milliseconds until the current loop completes.
   */
  const getRemainingTime = (
    startTime: number,
    duration: number = ANIMATION_DURATION,
  ) => {
    const elapsed = Date.now() - startTime;
    return Math.max(0, duration - (elapsed % duration));
  };

  useEffect(() => {
    const el = axisRef.current;
    if (!el) return;

    if (start) {
      stopRequestedRef.current = false;

      if (!isSpinning) {
        setIsSpinning(true);
        animationStartTimeRef.current = Date.now();
      }
    } else {
      stopRequestedRef.current = true;

      if (isSpinning && animationStartTimeRef.current) {
        const remainingTime = getRemainingTime(animationStartTimeRef.current);

        const stopTimeout = setTimeout(() => {
          if (stopRequestedRef.current && el) {
            if (!start) {
              el.classList.remove("spinning");
              requestAnimationFrame(() => {
                smoothReset(el);
              });
              setIsSpinning(false);
              animationStartTimeRef.current = null;
            }
          }
        }, remainingTime);

        return () => clearTimeout(stopTimeout);
      }
    }
  }, [start, isSpinning]);

  useEffect(() => {
    const el = axisRef.current;
    if (!el) return;

    if (isSpinning) {
      el.classList.add("spinning");
    } else {
      el.classList.remove("spinning");
    }
  }, [isSpinning]);

  const containerStyle: React.CSSProperties = {
    fontSize: `${size}rem`,
    ...style,
  };

  return (
    <div className={`lumen-avatar ${className}`.trim()} style={containerStyle}>
      <div className="pyramid">
        <div className="pyramid-gyro">
          <div ref={axisRef} className="pyramid-axis">
            <div className="pyramid-wall front" />
            <div className="pyramid-wall back" />
            <div className="pyramid-wall left" />
            <div className="pyramid-wall right" />
            <div className="bottom" />
            <div className="shadow" />
          </div>
        </div>
      </div>
    </div>
  );
};
