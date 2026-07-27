import { api } from "./api.ts";
import type {
  ConfigIssue,
  RuntimeConfigValidation,
  RuntimeConfigView,
  RuntimeNetworkSummary,
  SemanticChange,
} from "./types.ts";

interface RuntimeDraftState {
  session: number;
  view: RuntimeConfigView | null;
  candidateToml: string;
  issues: ConfigIssue[];
  semanticChanges: SemanticChange[];
  resolvedNetwork: RuntimeNetworkSummary | null;
  valid: boolean | null;
  validatedCandidate: string;
  loading: boolean;
  error: string;
}

export const runtimeDraft = $state<RuntimeDraftState>({
  session: -1,
  view: null,
  candidateToml: "",
  issues: [],
  semanticChanges: [],
  resolvedNetwork: null,
  valid: null,
  validatedCandidate: "",
  loading: false,
  error: "",
});

let loadToken = 0;

export async function loadRuntimeDraft(session: number): Promise<void> {
  if (runtimeDraft.session === session && (runtimeDraft.loading || runtimeDraft.view)) return;
  runtimeDraft.session = session;
  runtimeDraft.loading = true;
  runtimeDraft.error = "";
  const token = ++loadToken;
  try {
    const view = await api.runtimeConfig();
    if (token !== loadToken) return;
    runtimeDraft.view = view;
    runtimeDraft.candidateToml = view.candidateToml;
    runtimeDraft.issues = view.issues;
    runtimeDraft.semanticChanges = view.semanticChanges;
    runtimeDraft.resolvedNetwork = view.issues.length === 0 ? view.network : null;
    runtimeDraft.valid = null;
    runtimeDraft.validatedCandidate = view.issues.length > 0 ? view.candidateToml : "";
  } catch (cause) {
    if (token === loadToken) {
      runtimeDraft.error = cause instanceof Error ? cause.message : String(cause);
    }
  } finally {
    if (token === loadToken) runtimeDraft.loading = false;
  }
}

export function acceptRuntimeValidation(validation: RuntimeConfigValidation): void {
  runtimeDraft.issues = validation.issues;
  runtimeDraft.semanticChanges = validation.semanticChanges;
  runtimeDraft.valid = validation.valid;
  if (validation.valid) runtimeDraft.resolvedNetwork = validation.network;
  if (validation.candidateToml) runtimeDraft.candidateToml = validation.candidateToml;
  runtimeDraft.validatedCandidate = runtimeDraft.candidateToml;
}

export function resetRuntimeDraft(): void {
  if (!runtimeDraft.view) return;
  runtimeDraft.candidateToml = runtimeDraft.view.currentToml;
  runtimeDraft.issues = [];
  runtimeDraft.semanticChanges = [];
  runtimeDraft.resolvedNetwork =
    runtimeDraft.view.issues.length === 0 ? runtimeDraft.view.network : null;
  runtimeDraft.valid = null;
  runtimeDraft.validatedCandidate = "";
  runtimeDraft.error = "";
}

export function runtimeDraftDirty(): boolean {
  return Boolean(runtimeDraft.view && runtimeDraft.candidateToml !== runtimeDraft.view.currentToml);
}

export function resolvedRuntimeDraftNetwork(): RuntimeNetworkSummary | null {
  if (!runtimeDraft.view) return null;
  if (
    runtimeDraft.candidateToml === runtimeDraft.view.currentToml &&
    runtimeDraft.view.issues.length === 0
  ) {
    return runtimeDraft.view.network;
  }
  if (
    runtimeDraft.valid === true &&
    runtimeDraft.candidateToml === runtimeDraft.validatedCandidate
  ) {
    return runtimeDraft.resolvedNetwork;
  }
  return null;
}
