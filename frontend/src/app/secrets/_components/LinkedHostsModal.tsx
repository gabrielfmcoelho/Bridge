"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { secretsAPI, hostsAPI } from "@/lib/api";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";

// LinkedHostsModal manages the hosts that reuse a shared avulso password
// credential (via host_remote_users.secret_id). Open when secretID != null;
// the caller only opens it for avulso `password` secrets. Linking a host makes
// that host resolve its password from this one credential; unlinking drops it
// back to per-host resolution.
export default function LinkedHostsModal({
  secretID,
  onClose,
}: {
  secretID: number | null;
  onClose: () => void;
}) {
  const open = secretID != null;
  const qc = useQueryClient();
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<number[]>([]);

  const linked = useQuery({
    queryKey: ["secret-linked-hosts", secretID],
    queryFn: () => secretsAPI.listLinkedHosts(secretID as number),
    enabled: open,
  });
  const hosts = useQuery({
    queryKey: ["hosts-list"],
    queryFn: () => hostsAPI.list(),
    enabled: open,
  });

  const linkedIDs = useMemo(() => new Set(linked.data?.host_ids ?? []), [linked.data]);
  const allHosts = hosts.data ?? [];
  const linkedHosts = allHosts.filter((h) => linkedIDs.has(h.id));
  const available = allHosts.filter(
    (h) =>
      !linkedIDs.has(h.id) &&
      (search === "" || h.nickname.toLowerCase().includes(search.toLowerCase())),
  );

  const invalidate = () => qc.invalidateQueries({ queryKey: ["secret-linked-hosts", secretID] });

  const linkMut = useMutation({
    mutationFn: () => secretsAPI.linkHosts(secretID as number, selected),
    onSuccess: () => {
      setSelected([]);
      invalidate();
    },
  });
  const unlinkMut = useMutation({
    mutationFn: (hostID: number) => secretsAPI.unlinkHost(secretID as number, hostID),
    onSuccess: invalidate,
  });

  const toggle = (id: number) =>
    setSelected((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));

  const handleClose = () => {
    setSearch("");
    setSelected([]);
    onClose();
  };

  return (
    <ResponsiveModal open={open} onClose={handleClose} title="Hosts using this credential">
      <div className="space-y-4">
        <div>
          <h3 className="text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] mb-2">
            Linked ({linkedHosts.length})
          </h3>
          {linked.isLoading ? (
            <p className="text-xs text-[var(--text-faint)]">Loading…</p>
          ) : linkedHosts.length === 0 ? (
            <p className="text-xs text-[var(--text-faint)]">No hosts linked yet.</p>
          ) : (
            <div className="space-y-1">
              {linkedHosts.map((h) => (
                <div key={h.id} className="flex items-center justify-between gap-2 text-sm">
                  <span className="text-[var(--text-primary)]">
                    {h.nickname}{" "}
                    <span className="text-[var(--text-faint)] text-xs">({h.user})</span>
                  </span>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => unlinkMut.mutate(h.id)}
                    disabled={unlinkMut.isPending}
                  >
                    Unlink
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>

        <div>
          <h3 className="text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] mb-2">
            Add hosts
          </h3>
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search hosts…"
          />
          <div className="mt-2 max-h-56 overflow-y-auto space-y-1 pr-1">
            {available.length === 0 ? (
              <p className="text-xs text-[var(--text-faint)]">No matching hosts.</p>
            ) : (
              available.map((h) => (
                <label
                  key={h.id}
                  className="flex items-center gap-2 text-sm cursor-pointer text-[var(--text-primary)]"
                >
                  <input
                    type="checkbox"
                    checked={selected.includes(h.id)}
                    onChange={() => toggle(h.id)}
                  />
                  {h.nickname}{" "}
                  <span className="text-[var(--text-faint)] text-xs">({h.user})</span>
                </label>
              ))
            )}
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Button
            size="sm"
            onClick={() => linkMut.mutate()}
            disabled={selected.length === 0 || linkMut.isPending}
          >
            {linkMut.isPending
              ? "Linking…"
              : `Link ${selected.length} host${selected.length === 1 ? "" : "s"}`}
          </Button>
          <Button size="sm" variant="ghost" onClick={handleClose}>
            Close
          </Button>
          {(linkMut.isError || unlinkMut.isError) && (
            <span className="text-xs text-red-400">
              {((linkMut.error || unlinkMut.error) as Error)?.message}
            </span>
          )}
        </div>
      </div>
    </ResponsiveModal>
  );
}
