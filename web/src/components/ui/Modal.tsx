import {
  useEffect,
  useId,
  useRef,
  type MouseEvent,
  type ReactNode,
  type SyntheticEvent,
} from "react";
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
  /** Extra classes for the body wrapper. */
  bodyClassName?: string;
  /**
   * Whether the body itself scrolls. Set false when the body owns panes that
   * scroll independently (a master-detail layout), so the body is never a second
   * scroll container around them.
   */
  bodyScrollable?: boolean;
  /** Extra classes for the modal-box (e.g. a fixed height). */
  className?: string;
  /** Disable backdrop-click / Esc dismissal (e.g. while a sub-flow is open). */
  dismissable?: boolean;
}

/**
 * Shared, controlled modal shell used by every edit/create flow so they share
 * one mental model: header (icon + title + close), scrollable body, optional
 * footer. Pure daisyUI/lumilio tokens.
 *
 * Built on the native `<dialog>` element opened with `showModal()`, so focus
 * trapping, focus restoration, Escape handling, `role="dialog"`, and inerting
 * the page behind the modal come from the browser instead of hand-rolled
 * listeners. Children are mounted only while `open` is true, which keeps the
 * previous lifecycle semantics (closed modals hold no state and run no hooks).
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
  bodyScrollable = true,
  className = "",
  dismissable = true,
}: ModalProps): ReactNode {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  // `open` is part of the dependency list because the dialog element is only
  // committed to the DOM while open; the ref is null on the closed render.
  useEffect(() => {
    const dialog = dialogRef.current;
    if (open && dialog && !dialog.open) {
      dialog.showModal();
    }
  }, [open]);

  // Escape fires `cancel`; keep the browser from closing the element directly so
  // the parent stays the single owner of `open`.
  const handleCancel = (event: SyntheticEvent<HTMLDialogElement>) => {
    event.preventDefault();
    if (dismissable) onClose();
  };

  // The dialog element covers the viewport, so a click that lands on it (rather
  // than inside the box) is a backdrop click.
  const handleClick = (event: MouseEvent<HTMLDialogElement>) => {
    if (event.target === dialogRef.current && dismissable) onClose();
  };

  if (!open) return null;

  return (
    <dialog
      ref={dialogRef}
      className="modal modal-bottom sm:modal-middle z-modal"
      aria-labelledby={titleId}
      onCancel={handleCancel}
      onClick={handleClick}
    >
      <div
        className={`modal-box flex max-h-[85vh] w-full flex-col overflow-hidden p-0 rounded-b-none sm:rounded-b-2xl ${SIZE_CLASS[size]} ${className}`}
      >
        <header className="flex flex-shrink-0 items-center justify-between gap-3 border-b border-base-200 bg-base-200/40 px-4 sm:px-6 py-4">
          <div className="flex items-center gap-3">
            {icon && <span className="text-primary">{icon}</span>}
            <h3 id={titleId} className="text-lg font-bold">
              {title}
            </h3>
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

        <div
          className={`relative min-h-0 flex-1 ${
            bodyScrollable ? "overflow-y-auto" : "overflow-hidden"
          } ${bodyClassName}`}
        >
          {children}
        </div>

        {footer && (
          <footer className="flex flex-shrink-0 justify-end gap-3 border-t border-base-200 bg-base-200/40 px-4 sm:px-6 py-4">
            {footer}
          </footer>
        )}
      </div>
    </dialog>
  );
}

export default Modal;
