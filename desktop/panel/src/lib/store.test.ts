import { describe, expect, it } from "vite-plus/test";
import {
  anyServiceBusy,
  canOpenLumilio,
  canRestartRuntime,
  photosStatus,
  runtimeFailed,
  runtimeRunning,
} from "./store.svelte.ts";
import type { PanelState, RuntimePhase } from "./types.ts";

const state: PanelState = {
  mode: "dashboard",
  lang: "en",
  region: "other",
  path: "/tmp/storage",
  validation: { reachable: true, writable: true },
  version: "dev",
  tosRev: "dev",
  runtime: {
    phase: "running",
    stage: "ready",
    browserURL: "http://localhost:6680",
    canOpen: true,
    canRestart: true,
    lastKnownGoodAvailable: false,
    operationActive: false,
    network: {
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
    },
  },
  paths: {},
  networkHost: {
    lanWarningAcceptedVersion: 0,
    lanAddresses: [],
  },
  lumen: {
    enabled: false,
    state: "off",
    error: "",
    preset: "",
    backend: "",
    profile: "",
    cacheDir: "",
    previousCacheDir: "",
    installedVersion: "",
    latestVersion: "",
  },
  backends: [],
  presets: [],
  recommendedPreset: "",
  memoryGB: 0,
  cacheValidation: { reachable: false, writable: false },
};

function withPhase(phase: RuntimePhase): PanelState {
  return {
    ...state,
    runtime: {
      ...state.runtime,
      phase,
      operationActive: phase === "starting" || phase === "restarting",
    },
  };
}

describe("typed runtime state helpers", () => {
  it.each([
    ["stopped", "off"],
    ["starting", "starting"],
    ["running", "running"],
    ["restarting", "restarting"],
    ["failed", "failed"],
  ] satisfies Array<[RuntimePhase, ReturnType<typeof photosStatus>]>)(
    "maps %s to service status %s",
    (phase, expected) => {
      expect(photosStatus(withPhase(phase))).toBe(expected);
    },
  );

  it("polls only while a runtime operation or Lumen transition is active", () => {
    expect(anyServiceBusy(withPhase("running"))).toBe(false);
    expect(anyServiceBusy(withPhase("restarting"))).toBe(true);
    expect(
      anyServiceBusy({
        ...state,
        lumen: { ...state.lumen, state: "installing" },
      }),
    ).toBe(true);
  });

  it("enables Server actions only for supported snapshot states", () => {
    expect(runtimeRunning(withPhase("running"))).toBe(true);
    expect(runtimeFailed(withPhase("failed"))).toBe(true);
    expect(canOpenLumilio(withPhase("running"))).toBe(true);
    expect(canOpenLumilio(withPhase("failed"))).toBe(false);
    expect(canRestartRuntime(withPhase("running"))).toBe(true);
    expect(canRestartRuntime(withPhase("failed"))).toBe(true);
    expect(canRestartRuntime(withPhase("restarting"))).toBe(false);
  });
});
