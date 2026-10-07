"use client";

import { useEffect, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { canvasAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useFlag } from "@/contexts/FlagContext";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import StatusAlert from "@/components/ui/StatusAlert";
import Modal from "@/components/ui/Modal";
import Input from "@/components/ui/Input";
import FormError from "@/components/ui/FormError";
import FormFooter from "@/components/ui/FormFooter";
import { SkeletonCard } from "@/components/ui/Skeleton";
import CanvasBoard from "./_components/CanvasBoard";

export default function CanvasPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const flag = useFlag();
  const router = useRouter();
  const qc = useQueryClient();
  const id = Number(useParams<{ id: string }>().id);
  const canEdit = user?.role === "admin" || user?.role === "editor";

  // The board owns its state after load: no refetch on focus, or a background
  // refresh would reset what the user is drawing.
  const { data: canvas, isLoading, error } = useQuery({
    queryKey: ["canvas", id],
    queryFn: () => canvasAPI.get(id),
    staleTime: Infinity, refetchOnWindowFocus: false,
  });

  // The board's live version (autosave bumps it); 0 until loaded.
  const version = useRef(0);
  useEffect(() => { if (canvas && version.current === 0) version.current = canvas.version; }, [canvas]);
  const [renaming, setRenaming] = useState(false);
  const [title, setTitle] = useState("");
  const [formError, setFormError] = useState("");
  const rename = useMutation({
    // Title only (content omitted = kept), at the board's current version.
    mutationFn: () => canvasAPI.update(id, { version: version.current, title: title.trim() }),
    onSuccess: (res) => {
      version.current = res.version;
      qc.setQueryData(["canvas", id], { ...canvas!, title: res.title });
      setRenaming(false);
    },
    onError: (e) => setFormError(e instanceof Error ? e.message : t("form.saveFailed")),
  });
  const del = useMutation({
    mutationFn: () => canvasAPI.delete(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["canvases"] }); router.push("/canvas"); },
    onError: (e) => flag({ appearance: "error", title: t("common.delete"), description: e instanceof Error ? e.message : undefined }),
  });

  return (
    <PageShell>
      {isLoading ? (
        <SkeletonCard />
      ) : error || !canvas ? (
        <StatusAlert variant="error">{t("canvas.notFound")}</StatusAlert>
      ) : (
        <>
          <PageHeader
            title={canvas.title}
            subtitle={canvas.entidade_name}
            onEdit={canEdit ? () => { setTitle(canvas.title); setFormError(""); setRenaming(true); } : undefined}
            onDelete={canEdit ? () => del.mutate() : undefined}
            deleteConfirmMessage={t("canvas.deleteHint")}
          />
          <CanvasBoard canvas={canvas} canEdit={canEdit} version={version} />
        </>
      )}

      <Modal open={renaming} onClose={() => setRenaming(false)} title={t("canvas.rename")} size="sm"
        footer={<FormFooter onCancel={() => setRenaming(false)} submitLabel={t("common.save")} loading={rename.isPending}
          onSubmit={() => (title.trim() ? rename.mutate() : setFormError(t("canvas.titleRequired")))} />}>
        <div className="space-y-4">
          <Input label={t("canvas.titleLabel")} value={title} onChange={(e) => setTitle(e.target.value)} required autoFocus />
          <FormError message={formError} />
        </div>
      </Modal>
    </PageShell>
  );
}
