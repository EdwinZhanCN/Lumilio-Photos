import { useState, useRef, useEffect } from "react";
import { Check, X, Edit2 } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";

interface InlineTextEditorProps {
  value: string;
  onSave: (newValue: string) => void;
  onCancel?: () => void;
  placeholder?: string;
  disabled?: boolean;
  multiline?: boolean;
  maxLength?: number;
  className?: string;
  emptyStateText?: string;
  editHint?: string;
  saving?: boolean;
}

export default function InlineTextEditor({
  value,
  onSave,
  onCancel,
  placeholder,
  disabled = false,
  multiline = false,
  maxLength,
  className = "",
  emptyStateText,
  editHint,
  saving = false,
}: InlineTextEditorProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [editValue, setEditValue] = useState(value);
  const [hasChanged, setHasChanged] = useState(false);
  const inputRef = useRef<HTMLInputElement | HTMLTextAreaElement>(null);
  const { t } = useI18n();

  // Update editValue when value prop changes
  useEffect(() => {
    setEditValue(value);
    setHasChanged(false);
  }, [value]);

  // Focus input when entering edit mode
  useEffect(() => {
    if (isEditing && inputRef.current) {
      inputRef.current.focus();
      // Select all text for easy replacement
      inputRef.current.select();
    }
  }, [isEditing]);

  const handleStartEdit = () => {
    if (disabled || saving) return;
    setIsEditing(true);
    setEditValue(value);
    setHasChanged(false);
  };

  const handleSave = () => {
    if (saving) return;
    const trimmedValue = editValue.trim();
    setIsEditing(false);
    if (trimmedValue !== value) {
      onSave(trimmedValue);
    }
    setHasChanged(false);
  };

  const handleCancel = () => {
    setIsEditing(false);
    setEditValue(value);
    setHasChanged(false);
    if (onCancel) {
      onCancel();
    }
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const newValue = e.target.value;
    setEditValue(newValue);
    setHasChanged(newValue.trim() !== value);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !multiline) {
      e.preventDefault();
      handleSave();
    } else if (e.key === "Escape") {
      e.preventDefault();
      handleCancel();
    } else if (e.key === "Enter" && multiline && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      handleSave();
    }
  };

  const displayValue = value || emptyStateText || placeholder || "";
  const isEmpty = !value || value.trim().length === 0;

  if (isEditing) {
    const InputComponent = multiline ? "textarea" : "input";
    return (
      <div className={`flex items-start gap-2 ${className}`}>
        <div className="flex-1">
          <InputComponent
            ref={inputRef as any}
            type={multiline ? undefined : "text"}
            value={editValue}
            onChange={handleInputChange}
            onKeyDown={handleKeyDown}
            placeholder={placeholder}
            maxLength={maxLength}
            disabled={saving}
            className={`input input-bordered w-full text-xs ${
              multiline ? "textarea textarea-bordered min-h-[3rem] resize-none" : ""
            } ${saving ? "input-disabled" : ""}`}
            rows={multiline ? 3 : undefined}
          />
          {maxLength && (
            <div className="text-xs text-base-content/50 mt-1 text-right">
              {editValue.length}/{maxLength}
            </div>
          )}
          {multiline && (
            <div className="text-xs text-base-content/50 mt-1">
              {t("common.pressCtrlEnterToSave", { defaultValue: "Press Ctrl+Enter to save" })}
            </div>
          )}
        </div>
        <div className="flex gap-1 mt-1">
          <button
            type="button"
            className={`btn btn-xs btn-success ${saving ? "loading" : ""}`}
            onClick={handleSave}
            disabled={saving || !hasChanged}
            title={t("common.save")}
          >
            {saving ? "" : <Check className="w-3 h-3" />}
          </button>
          <button
            type="button"
            className="btn btn-xs btn-ghost"
            onClick={handleCancel}
            disabled={saving}
            title={t("common.cancel")}
          >
            <X className="w-3 h-3" />
          </button>
        </div>
      </div>
    );
  }

  return (
    <div
      className={`group cursor-pointer rounded p-1 -m-1 hover:bg-base-200/50 transition-colors ${className} ${
        disabled ? "cursor-not-allowed opacity-50" : ""
      }`}
      onClick={handleStartEdit}
      title={disabled ? undefined : editHint || t("common.clickToEdit")}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="flex-1 min-w-0">
          <div
            className={`text-xs break-words ${
              isEmpty ? "italic text-base-content/50" : "text-base-content"
            }`}
          >
            {displayValue}
          </div>
        </div>
        {!disabled && (
          <div className="opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0">
            <Edit2 className="w-3 h-3 text-base-content/40" />
          </div>
        )}
      </div>
    </div>
  );
}
