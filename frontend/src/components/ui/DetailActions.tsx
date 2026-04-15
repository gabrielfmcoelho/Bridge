import IconButton from "./IconButton";

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
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
          </svg>
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
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        </IconButton>
      )}
    </div>
  );
}
