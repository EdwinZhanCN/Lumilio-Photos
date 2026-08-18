import { useEffect, useState, type FormEvent } from "react";
import { Pencil, X } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { useMessage } from "@/features/notifications";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n";
import type { RepositoryOption } from "../../types";
import { validateRepositoryName, type RepositoryNameError } from "../../model/repositorySetup";

export default function RenameRepositoryModal({
  repository,
  isOpen,
  onClose,
}: {
  repository: RepositoryOption;
  isOpen: boolean;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const queryClient = useQueryClient();
  const renameMutation = $api.useMutation("post", "/api/v1/repositories/{id}/rename");
  const [name, setName] = useState(repository.name);
  const nameError = validateRepositoryName(name);

  useEffect(() => {
    if (isOpen) setName(repository.name);
  }, [isOpen, repository.name]);

  if (!isOpen) return null;

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (nameError || renameMutation.isPending || name === repository.name) return;
    try {
      await renameMutation.mutateAsync({
        params: { path: { id: repository.id } },
        body: { name },
      });
      void queryClient.invalidateQueries({
        queryKey: ["get", "/api/v1/assets/indexing/repositories"],
      });
      showMessage(
        "success",
        t("manage.repositories.renameSuccess", 'Renamed Repository to "{{name}}".', { name }),
      );
      onClose();
    } catch (error) {
      showMessage(
        "error",
        error instanceof Error
          ? error.message
          : t("manage.repositories.renameFailed", "Repository could not be renamed."),
      );
    }
  };

  return (
    <div className="modal modal-open z-modal">
      <div className="modal-box max-w-md">
        <div className="mb-5 flex items-start justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Pencil size={20} />
            </div>
            <div>
              <h3 className="text-base font-semibold">
                {t("manage.repositories.renameTitle", "Rename Repository")}
              </h3>
              <p className="mt-0.5 text-sm text-base-content/60">{repository.name}</p>
            </div>
          </div>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-circle"
            onClick={onClose}
            disabled={renameMutation.isPending}
            aria-label={t("common.close", { defaultValue: "Close" })}
          >
            <X size={18} />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-5">
          <div className="fieldset w-full gap-1">
            <label
              className="fieldset-legend p-0 text-sm font-medium"
              htmlFor="rename-repository-name"
            >
              {t("manage.repositories.renameLabel", "Display name")}
            </label>
            <input
              id="rename-repository-name"
              type="text"
              className={`input input-bordered w-full ${name.length > 0 && nameError ? "input-error" : ""}`}
              value={name}
              onChange={(event) => setName(event.target.value)}
              disabled={renameMutation.isPending}
              aria-invalid={name.length > 0 && nameError !== null}
              aria-describedby="rename-repository-name-hint"
              autoFocus
              required
            />
            <span
              id="rename-repository-name-hint"
              className={`label text-xs leading-snug ${
                name.length > 0 && nameError ? "text-error" : "text-base-content/55"
              }`}
            >
              {name.length > 0 && nameError
                ? renameNameErrorMessage(nameError, t)
                : t(
                    "manage.repositories.createNameHint",
                    "This display name can be changed later without moving files.",
                  )}
            </span>
          </div>

          <div className="modal-action">
            <button
              type="button"
              className="btn btn-ghost"
              onClick={onClose}
              disabled={renameMutation.isPending}
            >
              {t("common.cancel", { defaultValue: "Cancel" })}
            </button>
            <button
              type="submit"
              className="btn btn-primary gap-2"
              disabled={nameError !== null || name === repository.name || renameMutation.isPending}
            >
              {renameMutation.isPending ? (
                <span className="loading loading-spinner loading-xs" />
              ) : (
                <Pencil size={16} />
              )}
              {t("manage.repositories.renameSubmit", "Save name")}
            </button>
          </div>
        </form>
      </div>
      <button
        type="button"
        className="modal-backdrop"
        onClick={onClose}
        aria-label={t("common.close", { defaultValue: "Close" })}
      />
    </div>
  );
}

function renameNameErrorMessage(
  error: RepositoryNameError | null,
  t: ReturnType<typeof useI18n>["t"],
): string {
  switch (error) {
    case "required":
      return t("manage.repositories.createNameRequired", "Enter a Repository name.");
    case "leadingOrTrailingSpace":
      return t(
        "manage.repositories.createNameEdgeSpace",
        "The name cannot start or end with a space.",
      );
    case "tooManyCharacters":
      return t(
        "manage.repositories.createNameTooManyCharacters",
        "The name cannot exceed 80 characters.",
      );
    case "tooManyBytes":
      return t(
        "manage.repositories.createNameTooManyBytes",
        "The name cannot exceed 240 UTF-8 bytes.",
      );
    case "controlCharacter":
      return t(
        "manage.repositories.createNameControlCharacter",
        "The name cannot contain control characters.",
      );
    default:
      return "";
  }
}
