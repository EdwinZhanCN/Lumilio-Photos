import type { SVGProps } from "react";

/**
 * Clean Apple-style Live Photo concentric-circle icon.
 *
 * Three concentric rings with a filled centre dot — inherits colour from the
 * parent via `currentColor` with no hard-coded fills or backgrounds.
 */
export function LivePhotos(props: SVGProps<SVGSVGElement>) {
  return (
    <svg
      viewBox="0 0 24 24"
      width="1em"
      height="1em"
      xmlns="http://www.w3.org/2000/svg"
      fill="none"
      aria-hidden="true"
      {...props}
    >
      {/* Centre dot */}
      <circle cx="12" cy="12" r="2.8" fill="currentColor" />

      {/* Inner ring */}
      <circle
        cx="12"
        cy="12"
        r="5.5"
        stroke="currentColor"
        strokeWidth="1.5"
        fill="none"
        opacity="0.75"
      />

      {/* Outer ring — dashed to hint at motion */}
      <circle
        cx="12"
        cy="12"
        r="9.5"
        stroke="currentColor"
        strokeWidth="1.25"
        fill="none"
        strokeDasharray="2.5 2"
        opacity="0.45"
      />
    </svg>
  );
}
