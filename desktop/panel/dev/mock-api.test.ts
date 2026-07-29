import { describe, expect, it } from "vite-plus/test";
import { lumenPayload, networkPayload, statePayload, validationPayload } from "./mock-api.ts";

describe("control-panel development scenarios", () => {
  it("models all network deployment summaries from resolved facts", () => {
    expect(networkPayload("local")).toMatchObject({
      mode: "local",
      listen: "127.0.0.1:6680",
    });
    expect(networkPayload("lan")).toMatchObject({
      mode: "lan_http",
      listen: "0.0.0.0:6680",
    });
  });

  it("models disabled, transitional, running, and failed Lumen states", () => {
    expect(lumenPayload("disabled")).toMatchObject({ enabled: false, state: "off" });
    expect(lumenPayload("starting")).toMatchObject({
      enabled: true,
      state: "starting",
      phase: "downloading",
    });
    expect(lumenPayload("running")).toMatchObject({ enabled: true, state: "running" });
    expect(lumenPayload("failed")).toMatchObject({
      enabled: true,
      state: "failed",
      phase: "failed",
    });
  });

  it("exposes only the typed runtime contract and host-only network facts", () => {
    const payload = statePayload("failed", "dashboard", "lan", "failed");
    expect(payload).not.toHaveProperty("ready");
    expect(payload).not.toHaveProperty("serverURL");
    expect(payload).not.toHaveProperty("stage");
    expect(payload).not.toHaveProperty("network");
    expect(payload.runtime).toMatchObject({
      phase: "failed",
      errorCode: "startup_failed",
      network: { mode: "lan_http", listen: "0.0.0.0:6680" },
    });
    expect(payload.networkHost).toEqual({
      lanWarningAcceptedVersion: 0,
      lanAddresses: ["192.168.1.42"],
    });
  });

  it("models raw validation issues and semantic restart warnings", () => {
    expect(validationPayload('[server.tls]\nmode = "acme"', "local")).toMatchObject({
      valid: false,
      issues: [{ field: "server.tls.mode", code: "unsupported_desktop_tls" }],
    });
    expect(validationPayload('[logging]\nlevel = "debug"', "lan")).toMatchObject({
      valid: true,
      requiresRestart: true,
      semanticChanges: [{ field: "logging.level", before: "info", after: "debug" }],
      network: { mode: "lan_http" },
    });
  });
});
