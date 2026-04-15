"use client";

import { type ReactNode } from "react";
import Modal from "./Modal";
import Drawer from "./Drawer";
import { useMediaQuery } from "@/hooks/useMediaQuery";

interface ResponsiveModalProps {
  open: boolean;
  onClose: () => void;
  title?: string;
  subHeader?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
}

export default function ResponsiveModal({ open, onClose, title, subHeader, footer, children }: ResponsiveModalProps) {
  const isMobile = useMediaQuery("(max-width: 767px)");

  if (isMobile) {
    return (
      <Drawer open={open} onClose={onClose} title={title} subHeader={subHeader} footer={footer}>
        {children}
      </Drawer>
    );
  }

  return (
    <Modal open={open} onClose={onClose} title={title} subHeader={subHeader}>
      {children}
      {footer}
    </Modal>
  );
}
