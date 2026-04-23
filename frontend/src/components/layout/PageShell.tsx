"use client";

import { useAuth } from "@/contexts/AuthContext";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import Sidebar from "./Sidebar";
import Header from "./Header";
import MobileBottomNav from "./MobileBottomNav";

function LoadingSkeleton() {
  return (
    <div className="flex h-screen" style={{ background: "var(--bg-base)" }}>
      {/* Sidebar skeleton - hidden on mobile */}
      <div className="hidden md:block w-60 border-r border-[var(--border-subtle)] shrink-0" style={{ background: "var(--bg-surface)" }}>
        <div className="p-4 border-b border-[var(--border-subtle)]">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-[var(--radius-md)] skeleton" />
            <div className="space-y-1.5">
              <div className="h-3 w-14 skeleton" />
              <div className="h-2 w-24 skeleton" />
            </div>
          </div>
        </div>
        <div className="p-2.5 space-y-1">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="h-9 rounded-[var(--radius-md)] skeleton" />
          ))}
        </div>
      </div>
      {/* Content skeleton */}
      <div className="flex-1 flex flex-col min-w-0">
        <div className="h-13 border-b border-[var(--border-subtle)]" style={{ background: "var(--bg-surface)" }} />
        <div className="flex-1 p-4 md:p-6">
          <div className="space-y-4">
            <div className="h-7 w-48 skeleton" />
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4">
              {Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="h-24 rounded-[var(--radius-lg)] skeleton" />
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default function PageShell({
  children,
  fullBleed = false,
}: {
  children: React.ReactNode;
  // When true, <main> loses its padding and outer scroll so the page can own
  // a full-height layout (e.g. /wiki with its own internal scroll regions).
  fullBleed?: boolean;
}) {
  const { isAuthenticated, setupRequired, loading } = useAuth();
  const router = useRouter();
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);

  useEffect(() => {
    if (loading) return;
    if (setupRequired) {
      router.push("/setup");
    } else if (!isAuthenticated) {
      router.push("/login");
    }
  }, [isAuthenticated, setupRequired, loading, router]);

  if (loading) {
    return <LoadingSkeleton />;
  }

  if (!isAuthenticated) return null;

  return (
    <div className="flex h-screen overflow-hidden" style={{ background: "var(--bg-base)", color: "var(--text-primary)" }}>
      <Sidebar
        collapsed={collapsed}
        mobileOpen={mobileOpen}
        onCloseMobile={() => setMobileOpen(false)}
      />
      <div className="flex-1 flex flex-col min-w-0">
        <Header
          collapsed={collapsed}
          onToggleCollapse={() => setCollapsed(!collapsed)}
        />
        <main
          className={
            fullBleed
              ? "flex-1 overflow-hidden min-h-0"
              : "flex-1 overflow-y-auto p-3 md:p-6 pb-20 md:pb-6"
          }
        >
          <div className={fullBleed ? "h-full animate-fade-in" : "animate-fade-in"}>
            {children}
          </div>
        </main>
      </div>
      <MobileBottomNav onOpenDrawer={() => setMobileOpen(true)} />
    </div>
  );
}
