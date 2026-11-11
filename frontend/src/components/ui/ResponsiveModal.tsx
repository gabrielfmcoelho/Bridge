"use client";

import { type ReactNode } from "react";
import Modal, { type ModalSize } from "./Modal";
import Drawer from "./Drawer";
import { useMediaQuery } from "@/hooks/useMediaQuery";

interface ResponsiveModalProps {
  open: boolean;
  onClose: () => void;
  title?: string;
  subHeader?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
  /** Desktop width (phones get the bottom drawer). */
  size?: ModalSize;
  /** Desktop: above an open drawer (see Modal). */
  onTop?: boolean;
}

export default function ResponsiveModal({ open, onClose, title, subHeader, footer, children, size, onTop }: ResponsiveModalProps) {
  const isMobile = useMediaQuery("(max-width: 767px)");

  if (isMobile) {
    return (
      <Drawer open={open} onClose={onClose} title={title} subHeader={subHeader} footer={footer}>
        {children}
      </Drawer>
    );
  }

  return (
    <Modal open={open} onClose={onClose} title={title} subHeader={subHeader} footer={footer} size={size} onTop={onTop}>
      {children}
    </Modal>
  );
}
