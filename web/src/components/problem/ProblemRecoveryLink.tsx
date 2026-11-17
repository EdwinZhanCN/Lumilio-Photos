import { Link } from "react-router-dom";
import { useI18n } from "@/lib/i18n";
import { getProblemType, normalizeProblem } from "@/lib/http-commons/problem";
import { PROBLEM_RECOVERY, type RecoveryEntry } from "@/lib/http-commons/problemRecovery";

/**
 * The executable half of a Problem: the Server's registry names the Web
 * route or Desktop host action that follows from each Problem type, and
 * this renders it beside the localized message. A `route` becomes a link, a
 * `host_action` a link to the Storage page that starts it, a `sign_in` the
 * sign-in route; `retry`, `operator` and `none` render nothing, because the
 * message already says what remains. No feature decides the entry itself.
 */
export function ProblemRecoveryLink({
  problem,
  className = "link link-primary text-xs",
}: {
  /** A Problem response, a reference, or the error a request threw. */
  problem: unknown;
  className?: string;
}) {
  const { t } = useI18n();
  const type = getProblemType(problem);
  if (!type) return null;
  const entry = recoveryEntryFor(type);
  if (!entry) return null;
  const normalized = normalizeProblem(problem);
  const repositoryID = normalized.kind === "problem" ? normalized.repositoryID : undefined;
  const target = resolveRecoveryRoute(entry, { repositoryID });
  if (!target) return null;
  return (
    <Link to={target} className={className} data-recovery-kind={entry.kind}>
      {recoveryLabel(entry, t)}
    </Link>
  );
}

/** The registry entry of a Problem type, or undefined for a type the Web does not know. */
export function recoveryEntryFor(type: string): RecoveryEntry | undefined {
  return Object.hasOwn(PROBLEM_RECOVERY, type)
    ? PROBLEM_RECOVERY[type as keyof typeof PROBLEM_RECOVERY]
    : undefined;
}

/**
 * The concrete route of an entry: `:repositoryId` is filled from the
 * Problem's repository_id, and an entry that needs one without having it
 * falls back to the Storage page rather than to a route that cannot render.
 */
export function resolveRecoveryRoute(
  entry: RecoveryEntry,
  facts: { repositoryID?: string },
): string | undefined {
  if (!entry.route) return undefined;
  if (!entry.route.includes(":repositoryId")) return entry.route;
  if (!facts.repositoryID) return "/storage";
  return entry.route.replace(":repositoryId", encodeURIComponent(facts.repositoryID));
}

function recoveryLabel(entry: RecoveryEntry, t: ReturnType<typeof useI18n>["t"]): string {
  if (entry.kind === "host_action") {
    return t(
      "problemRecovery.desktopApproval",
      "Open Storage and choose the folder on the Desktop",
    );
  }
  if (entry.kind === "sign_in") {
    return t("problemRecovery.signIn", "Sign in");
  }
  switch (entry.route) {
    case "/storage":
      return t("problemRecovery.openStorage", "Open Storage");
    case "/storage/:repositoryId/trash":
      return t("problemRecovery.openTrash", "Open the Repository's Trash");
    case "/storage/:repositoryId/missing":
      return t("problemRecovery.openMissing", "Open the Repository's missing files");
    case "/bootstrap":
      return t("problemRecovery.openSetup", "Open setup");
    case "/settings":
      return t("problemRecovery.openSettings", "Open Settings");
    default:
      return t("problemRecovery.continue", "Continue");
  }
}
