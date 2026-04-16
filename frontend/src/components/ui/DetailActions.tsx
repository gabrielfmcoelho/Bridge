import IconButton from "./IconButton";
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
  return (
    <div className="hidden md:flex items-center gap-1.5">
      {canEdit && (
        <IconButton variant="outline" size="sm" onClick={onEdit} title="Edit">
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
          title="Delete"
        >
          <Icon path={ICON_PATHS.trash} />
        </IconButton>
      )}
    </div>
  );
}
