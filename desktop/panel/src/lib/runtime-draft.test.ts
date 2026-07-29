import { beforeEach, describe, expect, it } from "vite-plus/test";
import {
  acceptRuntimeValidation,
  resolvedRuntimeDraftNetwork,
  runtimeDraft,
} from "./runtime-draft.svelte.ts";
import type { RuntimeConfigValidation, RuntimeConfigView, RuntimeNetworkSummary } from "./types.ts";

const currentNetwork: RuntimeNetworkSummary = {
  mode: "local",
  listen: "127.0.0.1:6680",
  passkeyEnabled: true,
};

const candidateNetwork: RuntimeNetworkSummary = {
  ...currentNetwork,
  mode: "lan_http",
  listen: "0.0.0.0:6680",
};

const view: RuntimeConfigView = {
  currentToml: "current",
  candidateToml: "current",
  baseFingerprint: "sha256:current",
  lastKnownGoodAvailable: true,
  hostManagedPaths: [],
  network: currentNetwork,
  issues: [],
  semanticChanges: [],
};

beforeEach(() => {
  Object.assign(runtimeDraft, {
    session: 1,
    view,
    candidateToml: view.candidateToml,
    issues: [],
    semanticChanges: [],
    resolvedNetwork: currentNetwork,
    valid: null,
    validatedCandidate: "",
    loading: false,
    error: "",
  });
});

describe("shared runtime candidate", () => {
  it("uses the current resolved network until raw TOML becomes unvalidated", () => {
    expect(resolvedRuntimeDraftNetwork()).toEqual(currentNetwork);
    runtimeDraft.candidateToml = "raw edit";
    expect(resolvedRuntimeDraftNetwork()).toBeNull();
  });

  it("does not present fallback network facts as a resolved invalid candidate", () => {
    runtimeDraft.view = {
      ...view,
      issues: [{ code: "invalid_active_manifest", message: "strict load failed" }],
    };
    runtimeDraft.resolvedNetwork = null;
    expect(resolvedRuntimeDraftNetwork()).toBeNull();
  });

  it("publishes the backend-resolved network for a validated raw candidate", () => {
    runtimeDraft.candidateToml = "validated candidate";
    acceptRuntimeValidation({
      valid: true,
      candidateToml: "validated candidate",
      baseFingerprint: view.baseFingerprint,
      network: candidateNetwork,
      issues: [],
      semanticChanges: [],
      requiresRestart: true,
    } satisfies RuntimeConfigValidation);

    expect(resolvedRuntimeDraftNetwork()).toEqual(candidateNetwork);
  });
});
