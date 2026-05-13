"use client";

import * as Dialog from "@radix-ui/react-dialog";
import { ReactNode } from "react";
import { X } from "../icons";

export type SheetProps = {
  open: boolean;
  onOpenChange: (next: boolean) => void;
  title: ReactNode;
  subtitle?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
};

export function Sheet({ open, onOpenChange, title, subtitle, children, footer }: SheetProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="scrim" />
        <Dialog.Content className="sheet" aria-describedby={undefined}>
          <div className="sheet-hd">
            <div>
              <Dialog.Title className="ttl">{title}</Dialog.Title>
              {subtitle && (
                <div
                  className="sub"
                  style={{ fontFamily: "var(--f-mono)", fontSize: 11.5, color: "var(--fg-3)", marginTop: 4 }}
                >
                  {subtitle}
                </div>
              )}
            </div>
            <Dialog.Close className="icon-btn" aria-label="Close">
              <X />
            </Dialog.Close>
          </div>
          <div className="sheet-body">{children}</div>
          {footer && <div className="sheet-foot">{footer}</div>}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
