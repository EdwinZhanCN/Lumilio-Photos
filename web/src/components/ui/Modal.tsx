import { useEffect, type ReactNode } from "react";
import { X } from "lucide-react";

const SIZE_CLASS: Record<NonNullable<ModalProps["size"]>, string> = {
  sm: "max-w-md",
  md: "max-w-2xl",
  lg: "max-w-4xl",
  xl: "max-w-6xl",
};

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  /** Header title. */
  title: ReactNode;
  /** Optional leading icon shown next to the title. */
  icon?: ReactNode;
  /** Controls the modal-box max width. */
  size?: "sm" | "md" | "lg" | "xl";
  /** Footer content — typically the cancel/confirm buttons. Omit for none. */
  footer?: ReactNode;
  /** Body content. */
  children: ReactNode;
  /** Extra classes for the scrollable body wrapper. */
  bodyClassName?: string;
  /** Extra classes for the modal-box (e.g. a fixed height). */
  className?: string;
  /** Disable backdrop-click / Esc dismissal (e.g. while a sub-flow is open). */
  dismissable?: boolean;
}

/**
 * Shared, controlled modal shell used by every edit/create flow so they share
 * one mental model: header (icon + title + close), scrollable body, optional
 * footer. Pure daisyUI/lumilio tokens. Esc and backdrop click call `onClose`
 * unless `dismissable` is false.
 */
export function Modal({
  open,
  onClose,
  title,
  icon,
  size = "md",
  footer,
  children,
  bodyClassName = "",
  className = "",
  dismissable = true,
}: ModalProps): ReactNode {
  useEffect(() => {
    if (!open || !dismissable) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, dismissable, onClose]);

  if (!open) return null;

  return (
    <div className="modal modal-open modal-bottom sm:modal-middle z-modal">
      <div
        className={`modal-box flex max-h-[85vh] w-full flex-col overflow-hidden p-0 rounded-b-none sm:rounded-b-2xl ${SIZE_CLASS[size]} ${className}`}
      >
        <header className="flex flex-shrink-0 items-center justify-between gap-3 border-b border-base-200 bg-base-200/40 px-4 sm:px-6 py-4">
          <div className="flex items-center gap-3">
            {icon && <span className="text-primary">{icon}</span>}
            <h3 className="text-lg font-bold">{title}</h3>
          </div>
          <button
            type="button"
            className="btn btn-circle btn-ghost btn-sm"
            onClick={onClose}
            aria-label="Close"
          >
            <X size={20} />
          </button>
        </header>

        <div className={`relative min-h-0 flex-1 overflow-y-auto ${bodyClassName}`}>{children}</div>

        {footer && (
          <footer className="flex flex-shrink-0 justify-end gap-3 border-t border-base-200 bg-base-200/40 px-4 sm:px-6 py-4">
            {footer}
          </footer>
        )}
      </div>
      <div
        className="modal-backdrop bg-base-300/60 backdrop-blur-sm"
        onClick={dismissable ? onClose : undefined}
      />
    </div>
  );
}

export default Modal;
