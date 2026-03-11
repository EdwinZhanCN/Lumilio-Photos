import React from "react";

function getInitials(name: string): string {
  return (
    name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase() ?? "")
      .join("") || "U"
  );
}

type UserAvatarProps = {
  /** URL of the avatar image. When falsy, renders initials placeholder. */
  src?: string | null;
  /** Name used for alt text and initials fallback. */
  name: string;
  /** Tailwind size class applied to the outer circle, e.g. "size-10", "size-24". */
  size?: string;
  /** Text size class for the initials, e.g. "text-sm", "text-3xl". */
  textSize?: string;
  /** Extra classes on the outermost wrapper. */
  className?: string;
};

export default function UserAvatar({
  src,
  name,
  size = "size-24",
  textSize = "text-3xl",
  className = "",
}: UserAvatarProps): React.ReactNode {
  if (src) {
    return (
      <div className={`avatar ${className}`}>
        <div className={`${size} rounded-full`}>
          <img src={src} alt={name} className="rounded-full object-cover" />
        </div>
      </div>
    );
  }

  return (
    <div className={`avatar avatar-placeholder ${className}`}>
      <div
        className={`bg-neutral text-neutral-content ${size} rounded-full`}
      >
        <span className={textSize}>{getInitials(name)}</span>
      </div>
    </div>
  );
}
