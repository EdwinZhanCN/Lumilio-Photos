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
  primaryOrigin: "http://localhost:6680",
  tlsMode: "off",
  proxyMode: "disabled",
  trustedProxyCIDRs: [],
  passkeyOrigin: "http://localhost:6680",
  rpID: "localhost",
  passkeyEnabled: true,
  remotePasskeyAvailable: false,
};

const candidateNetwork: RuntimeNetworkSummary = {
  ...currentNetwork,
  mode: "external_https",
  primaryOrigin: "https://photos.example.com",
  tlsMode: "external",
  proxyMode: "required",
  trustedProxyCIDRs: ["127.0.0.1/32", "::1/128"],
  passkeyOrigin: "https://photos.example.com",
  rpID: "photos.example.com",
  remotePasskeyAvailable: true,
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
