import IconButton from "./IconButton";
import { useLocale } from "@/contexts/LocaleContext";
import Icon from "./Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

interface DetailActionsProps {
  canEdit: boolean;
  isAdmin: boolean;
  onEdit: () => void;
  onDelete: () => void;
  deleteConfirmMessage: string;
}

export default function DetailActions({
  canEdit,
  isAdmin,
  onEdit,
  onDelete,
  deleteConfirmMessage,
}: DetailActionsProps) {
  const { t } = useLocale();
  return (
    <div className="hidden md:flex items-center gap-1.5">
      {canEdit && (
        <IconButton variant="outline" size="sm" onClick={onEdit} title={t("common.edit")}>
          <Icon path={ICON_PATHS.edit} />
        </IconButton>
      )}
      {isAdmin && (
        <IconButton
          variant="danger"
          size="sm"
          onClick={() => {
            if (confirm(deleteConfirmMessage)) onDelete();
          }}
          title={t("common.delete")}
        >
          <Icon path={ICON_PATHS.trash} />
        </IconButton>
      )}
    </div>
  );
}
