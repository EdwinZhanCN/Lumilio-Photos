import React from "react";
import { AlertTriangle, CheckCircle, FolderPlus, HardDrive, RefreshCw } from "lucide-react";
import { StorageRiskConfirmation, StorageStrategyPicker } from "@/features/repositories";
import { useI18n } from "@/lib/i18n.tsx";
import { formatBytes } from "@/lib/utils/formatters";
import type { TFunction } from "i18next";
import { Btn, Field, InlineError, TextInput } from "../../components/ui";
import { repositoryNameErrorMessage } from "../../model/repositoryNameValidation.ts";
import type { SetupDefaultStorage, SetupPrimaryReview } from "../../api/useSetupPrimaryTarget.ts";
import { usePrimarySetup, type PrimarySetupOutcome } from "./usePrimarySetup.ts";

type Props = {
  /** `bootstrap` inside the first-run wizard, `gate` for a later sign-in. */
  variant: "bootstrap" | "gate";
  onCompleted: (outcome: PrimarySetupOutcome) => void | Promise<void>;
};

/**
 * Renders the Server's review of the Primary Repository target and the one
 * decision it allows: create, adopt what is already there, or start fresh
 * elsewhere through the host. The copy never guesses past the review state.
 */
export default function PrimarySetupPanel({ variant, onCompleted }: Props) {
  const { t } = useI18n();
  const setup = usePrimarySetup({ onCompleted });

  if (setup.targetQuery.isPending) {
    return (
      <p className="animate-pulse py-6 text-sm text-base-content/55">
        {t("auth.primaryRepository.loading", {
          defaultValue: "Checking Primary Repository setup...",
        })}
      </p>
    );
  }
  if (setup.targetQuery.isError || !setup.target) {
    return (
      <div className="flex flex-col gap-3">
        <InlineError>
          {t("auth.primarySetup.loadFailed", "Unable to load the Primary Repository setup review.")}
        </InlineError>
        <Btn
          variant="outline"
          icon={RefreshCw}
          className="self-start"
          onClick={() => void setup.targetQuery.refetch()}
        >
          {t("auth.storageRecovery.checkAgain", "Check Again")}
        </Btn>
      </div>
    );
  }

  const defaultStorage = setup.defaultStorage;
  const review = setup.review;
  const freshElsewhere = <FreshElsewhere nativeHost={setup.nativeHost} />;

  if (defaultStorage && defaultStorage.state !== "ready") {
    return (
      <div className="flex flex-col gap-4">
        <Notice
          tone="warning"
          title={t(
            "auth.primarySetup.defaultStorage.title",
            "The Default Storage Location needs attention",
          )}
        >
          <p>{defaultStorageCopy(defaultStorage, t)}</p>
          {defaultStorage.path ? (
            <code className="mt-1 block truncate text-xs">{defaultStorage.path}</code>
          ) : null}
        </Notice>
        {freshElsewhere}
        <CheckAgain onClick={() => void setup.refreshReview()} />
      </div>
    );
  }
  if (!review) {
    return (
      <div className="flex flex-col gap-4">
        <Notice
          tone="warning"
          title={t("auth.primaryRepository.targetUnavailable", {
            defaultValue:
              "The Default Storage Location cannot hold the Primary Repository right now.",
          })}
        >
          {setup.reviewProblem ? <p>{setup.reviewProblem}</p> : null}
        </Notice>
        {freshElsewhere}
        <CheckAgain onClick={() => void setup.refreshReview()} />
      </div>
    );
  }

  const existing = review.existing;
  const recovery = review.recovery;
  const layoutMissing = review.layout_missing ?? [];

  if (setup.canAdopt && existing) {
    return (
      <div className="flex flex-col gap-4">
        {setup.error ? <InlineError>{setup.error}</InlineError> : null}
        <Notice
          tone="info"
          title={t("auth.primarySetup.adopt.title", "A Primary Repository is already here")}
        >
          <p>
            {t(
              "auth.primarySetup.adopt.body",
              "{{name}} was found in the primary/ folder of the Default Storage Location. Adopt it to keep its identity and every file, or start fresh elsewhere. Nothing changes until you choose.",
              {
                name:
                  existing.name ||
                  t("auth.primarySetup.adopt.unnamed", "An earlier Primary Repository"),
              },
            )}
          </p>
          <dl className="m-0 mt-2 grid grid-cols-1 gap-x-6 gap-y-1 text-xs sm:grid-cols-2">
            <div className="flex gap-2">
              <dt className="shrink-0 text-base-content/55">
                {t("auth.primarySetup.adopt.identity", "Identity")}
              </dt>
              <dd className="m-0 min-w-0 truncate font-mono">{existing.repository_id}</dd>
            </div>
            {recovery ? (
              <>
                <div className="flex gap-2">
                  <dt className="shrink-0 text-base-content/55">
                    {t("auth.primarySetup.adopt.heldTrash", "Held in Trash")}
                  </dt>
                  <dd className="m-0">
                    {t("auth.primarySetup.adopt.heldTrashValue", "{{count}} files ({{size}})", {
                      count: recovery.trash_files ?? 0,
                      size: formatBytes(recovery.trash_bytes ?? 0),
                    })}
                  </dd>
                </div>
                <div className="flex gap-2">
                  <dt className="shrink-0 text-base-content/55">
                    {t("auth.primarySetup.adopt.studio", "Studio edits")}
                  </dt>
                  <dd className="m-0">{recovery.studio_records ?? 0}</dd>
                </div>
                <div className="flex gap-2">
                  <dt className="shrink-0 text-base-content/55">
                    {t("auth.primarySetup.adopt.staging", "Unfinished uploads")}
                  </dt>
                  <dd className="m-0">{recovery.staging_files ?? 0}</dd>
                </div>
                <div className="flex gap-2">
                  <dt className="shrink-0 text-base-content/55">
                    {t("auth.primarySetup.adopt.unknown", "Other private files")}
                  </dt>
                  <dd className="m-0">{recovery.unknown_files ?? 0}</dd>
                </div>
              </>
            ) : null}
          </dl>
          {layoutMissing.length > 0 ? (
            <p className="mt-2 text-xs">
              {t(
                "auth.primarySetup.adopt.layoutRepair",
                "Its {{folders}} folder is missing and will be added empty; nothing else in the folder changes.",
                { folders: layoutMissing.join(", ") },
              )}
            </p>
          ) : null}
        </Notice>
        <p className="text-sm text-base-content/70">
          {t(
            "auth.primarySetup.adopt.disclosure",
            "Albums, People, ratings, Event names and share links live in the catalog, not in the folder. To get them back, restore a compatible database backup of the earlier installation instead of adopting. Adoption rebuilds only what the folder carries: trashed files stay held until you decide, and Studio edits reattach once their photos are proven.",
          )}
        </p>
        <StorageRiskConfirmation
          risks={setup.placementRisks}
          checked={setup.riskAcknowledged}
          onChange={setup.setRiskAcknowledged}
          disabled={setup.isPending}
        />
        <Btn
          type="button"
          variant={variant === "bootstrap" ? "neutral" : "primary"}
          icon={CheckCircle}
          loading={setup.isPending}
          disabled={!setup.canSubmitAdopt}
          className="self-start px-6"
          onClick={() => void setup.submitAdopt()}
        >
          {t("auth.primarySetup.adopt.submit", "Adopt Primary Repository")}
        </Btn>
        {freshElsewhere}
      </div>
    );
  }

  if (!setup.canCreate) {
    return (
      <div className="flex flex-col gap-4">
        <Notice
          tone="warning"
          title={t(
            "auth.primarySetup.blocked.title",
            "The primary/ folder cannot become the Primary Repository",
          )}
        >
          <p>{blockedReviewCopy(review, t)}</p>
          {setup.reviewProblem && review.state === "unavailable" ? (
            <p>{setup.reviewProblem}</p>
          ) : null}
        </Notice>
        {freshElsewhere}
        <CheckAgain onClick={() => void setup.refreshReview()} />
      </div>
    );
  }

  return (
    <form className="flex flex-col gap-4" onSubmit={(event) => void setup.submitCreate(event)}>
      {setup.error ? <InlineError>{setup.error}</InlineError> : null}
      <Field
        label={t("auth.primaryRepository.name", { defaultValue: "Name" })}
        hint={t(
          "auth.primaryRepository.nameHint",
          "This display name can be changed later. The Primary Repository directory remains primary/.",
        )}
        error={
          setup.name.length > 0 && setup.nameError
            ? repositoryNameErrorMessage(setup.nameError, t)
            : undefined
        }
      >
        <TextInput
          icon={FolderPlus}
          type="text"
          value={setup.name}
          onChange={(event) => setup.setName(event.target.value)}
          disabled={setup.isPending}
          invalid={setup.name.length > 0 && setup.nameError !== null}
          required
        />
      </Field>
      <Field
        label={t("auth.primaryRepository.storageLocation", {
          defaultValue: "Default Storage Location",
        })}
        hint={t("auth.primaryRepository.storageLocationHint", {
          defaultValue:
            "Set by server configuration. Lumilio creates the Primary Repository in this Default Storage Location's primary/ folder.",
        })}
      >
        <TextInput
          icon={HardDrive}
          type="text"
          value={setup.storageLocationPath}
          readOnly
          tabIndex={-1}
          className="bg-base-200 font-mono text-sm"
        />
      </Field>
      <StorageStrategyPicker
        value={setup.storageStrategy}
        onChange={setup.setStorageStrategy}
        disabled={setup.isPending}
        idPrefix={variant === "bootstrap" ? "bootstrap-primary" : "primary-gate"}
      />
      {setup.reviewProblem ? (
        <div role="alert" className="alert alert-warning text-sm">
          {setup.reviewProblem}
        </div>
      ) : null}
      <StorageRiskConfirmation
        risks={setup.placementRisks}
        checked={setup.riskAcknowledged}
        onChange={setup.setRiskAcknowledged}
        disabled={setup.isPending}
      />
      <Btn
        type="submit"
        variant={variant === "bootstrap" ? "neutral" : "primary"}
        icon={variant === "bootstrap" ? CheckCircle : FolderPlus}
        loading={setup.isPending}
        disabled={!setup.canSubmitCreate}
        className="self-start px-6"
      >
        {variant === "bootstrap"
          ? t("auth.bootstrap.repository.submit", {
              defaultValue: "Create Primary Repository & open dashboard",
            })
          : t("auth.primaryRepository.submit", { defaultValue: "Create Primary Repository" })}
      </Btn>
    </form>
  );
}

function Notice({
  tone,
  title,
  children,
}: {
  tone: "info" | "warning";
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div
      role={tone === "warning" ? "alert" : "status"}
      className={`rounded-xl border px-4 py-3 text-sm ${
        tone === "warning" ? "border-warning/40 bg-warning/10" : "border-info/40 bg-info/10"
      }`}
    >
      <p className="flex items-center gap-2 font-medium">
        {tone === "warning" ? (
          <AlertTriangle size={16} className="shrink-0" aria-hidden />
        ) : (
          <HardDrive size={16} className="shrink-0" aria-hidden />
        )}
        {title}
      </p>
      <div className="mt-1 text-base-content/80">{children}</div>
    </div>
  );
}

function CheckAgain({ onClick }: { onClick: () => void }) {
  const { t } = useI18n();
  return (
    <Btn variant="outline" icon={RefreshCw} className="self-start" onClick={onClick}>
      {t("auth.storageRecovery.checkAgain", "Check Again")}
    </Btn>
  );
}

/**
 * The only way out when the fixed target cannot be used: the host changes the
 * Default Storage Location, the current folder is left untouched, and setup
 * creates the Primary Repository in the new one.
 */
function FreshElsewhere({ nativeHost }: { nativeHost: boolean }) {
  const { t } = useI18n();
  return (
    <section className="rounded-xl border border-base-200 px-4 py-3 text-sm">
      <h3 className="m-0 text-sm font-medium">
        {t("auth.primarySetup.freshElsewhere.title", "Start fresh elsewhere")}
      </h3>
      <p className="mt-1 text-base-content/70">
        {nativeHost
          ? t(
              "auth.primarySetup.freshElsewhere.desktop",
              "Open Settings → Storage in Lumilio Photos Desktop, choose another folder and apply it. The current folder is left untouched, and a new Primary Repository is created in the new one.",
            )
          : t(
              "auth.primarySetup.freshElsewhere.server",
              "Point storage.path in server.toml (Docker: the media volume) at another folder and restart the server. The current folder is left untouched, and a new Primary Repository is created in the new one.",
            )}
      </p>
    </section>
  );
}

function defaultStorageCopy(storage: SetupDefaultStorage, t: TFunction): string {
  switch (storage.state) {
    case "offline":
      return t(
        "auth.primarySetup.defaultStorage.offline",
        "The configured folder is not reachable right now. Reconnect the disk or mount, then check again.",
      );
    case "path_changed":
      return t(
        "auth.primarySetup.defaultStorage.pathChanged",
        "The configured path differs from the registered Default Storage Location and was not accepted as a move.",
      );
    case "marker_missing":
      return t(
        "auth.primarySetup.defaultStorage.markerMissing",
        "The folder lost its .lumilioroot marker. Restore it from a known backup; Lumilio never writes a new one over a registered folder.",
      );
    case "marker_invalid":
      return t(
        "auth.primarySetup.defaultStorage.markerInvalid",
        "The folder's .lumilioroot marker cannot be read. It is left untouched; restore it from a known backup or start fresh elsewhere.",
      );
    case "marker_unsupported":
      return t(
        "auth.primarySetup.defaultStorage.markerUnsupported",
        "The folder's .lumilioroot marker was written by a newer Lumilio Photos. Install that version, or start fresh elsewhere.",
      );
    case "identity_mismatch":
      return t(
        "auth.primarySetup.defaultStorage.identityMismatch",
        "The folder carries a different Storage Location identity than the one registered here.",
      );
    case "repository_marker":
      return t(
        "auth.primarySetup.defaultStorage.repositoryMarker",
        "The configured folder is itself a Repository. Point the Default Storage Location at its parent or at another folder.",
      );
    case "unregistered":
      return t(
        "auth.primarySetup.defaultStorage.unregistered",
        "The folder could not be registered when the server started. Check the server log, fix the folder, then restart.",
      );
    default:
      return t(
        "auth.primarySetup.defaultStorage.unavailable",
        "The folder cannot be inspected right now. Check its access and mount, then check again.",
      );
  }
}

function blockedReviewCopy(review: SetupPrimaryReview, t: TFunction): string {
  switch (review.state) {
    case "registered":
      return t(
        "auth.primarySetup.blocked.registered",
        "The Repository in primary/ is already registered in this catalog, so it cannot become the Primary Repository.",
      );
    case "nonempty_unmarked":
      return t(
        "auth.primarySetup.blocked.nonemptyUnmarked",
        "primary/ holds files but no Repository marker. Lumilio never initializes over them. Move them out of primary/ to create a new Primary Repository here, or start fresh elsewhere.",
      );
    case "marker_invalid":
      return t(
        "auth.primarySetup.blocked.markerInvalid",
        "The Repository marker in primary/ cannot be read. It is left untouched. Restore .lumiliorepo from a known backup, or start fresh elsewhere.",
      );
    case "marker_unsupported":
      return t(
        "auth.primarySetup.blocked.markerUnsupported",
        "The Repository in primary/ was written by a newer Lumilio Photos. Install that version, or start fresh elsewhere.",
      );
    default:
      return t(
        "auth.primarySetup.blocked.unavailable",
        "primary/ cannot be inspected right now. Check its access and mount, then check again.",
      );
  }
}
