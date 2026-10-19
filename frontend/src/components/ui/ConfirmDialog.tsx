import { useEffect, useId, useRef, type ReactNode } from "react";
import { Button } from "./Button";

type ConfirmDialogProps = {
  open: boolean;
  title: string;
  children: ReactNode;
  confirmLabel: string;
  onConfirm: () => void;
  onCancel: () => void;
};

/**
 * Modal confirmation built on <dialog>. The browser traps focus inside it and
 * closes it on Escape; a click on the backdrop cancels too. Cancel comes first
 * so it is the button focused on open.
 */
export function ConfirmDialog({ open, title, children, confirmLabel, onConfirm, onCancel }: ConfirmDialogProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      onClose={onCancel}
      onClick={(event) => {
        if (event.target === event.currentTarget) onCancel();
      }}
      className="m-auto w-[calc(100%-2rem)] max-w-sm rounded-md border border-rule bg-paper-raised p-6 text-ink backdrop:bg-[rgb(0_0_0/0.5)]"
    >
      <h2 id={titleId} className="font-serif text-xl">
        {title}
      </h2>
      <div className="mt-3 text-ink-muted">{children}</div>
      <div className="mt-6 flex justify-end gap-3">
        <Button variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
        <Button variant="danger" onClick={onConfirm}>
          {confirmLabel}
        </Button>
      </div>
    </dialog>
  );
}
