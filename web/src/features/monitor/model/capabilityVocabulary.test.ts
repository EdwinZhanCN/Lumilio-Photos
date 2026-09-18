import { describe, expect, it } from "vite-plus/test";
import { translateDouble } from "@test/translateDouble";
import { errorCodeLabel, nodeVerdict, stateLabel, statusTone } from "./capabilityVocabulary";

describe("statusTone", () => {
  it("grades the states the Server can report", () => {
    expect(statusTone("healthy")).toBe("ok");
    expect(statusTone("compatible")).toBe("ok");
    expect(statusTone("ready")).toBe("ok");
    expect(statusTone("degraded")).toBe("danger");
    expect(statusTone("incompatible")).toBe("danger");
    expect(statusTone("unavailable")).toBe("danger");
    expect(statusTone("starting")).toBe("warning");
    expect(statusTone("pending")).toBe("warning");
    expect(statusTone("connecting")).toBe("warning");
  });

  it("does not imply health for disabled or unknown words", () => {
    expect(statusTone("disabled")).toBe("neutral");
    expect(statusTone(undefined)).toBe("neutral");
    expect(statusTone("future_state")).toBe("neutral");
  });
});

describe("stateLabel", () => {
  it("resolves every known state to its own key", () => {
    const states = [
      "disabled",
      "starting",
      "healthy",
      "degraded",
      "connecting",
      "ready",
      "unavailable",
      "pending",
      "compatible",
      "incompatible",
      "active",
    ];
    const { t } = translateDouble();
    for (const state of states) {
      expect(stateLabel(t, state), state).toBe(`monitor.capabilities.states.${state}`);
    }
  });

  it("falls back to the unknown key", () => {
    const { t } = translateDouble();
    expect(stateLabel(t, "future_state")).toBe("monitor.capabilities.states.unknown");
    expect(stateLabel(t, undefined)).toBe("monitor.capabilities.states.unknown");
  });
});

describe("nodeVerdict", () => {
  it("reports an unreachable node before any compatibility opinion", () => {
    expect(nodeVerdict({ transport: "unavailable", compatibility: "compatible" })).toBe(
      "unavailable",
    );
  });

  it("treats a node still connecting as unsettled", () => {
    expect(nodeVerdict({ transport: "connecting", compatibility: "incompatible" })).toBe(
      "connecting",
    );
  });

  it("only calls a reachable node active once compatibility is proven", () => {
    expect(nodeVerdict({ transport: "ready", compatibility: "incompatible" })).toBe("incompatible");
    expect(nodeVerdict({ transport: "ready", compatibility: "pending" })).toBe("pending");
    expect(nodeVerdict({ transport: "ready" })).toBe("pending");
    expect(nodeVerdict({ transport: "ready", compatibility: "compatible" })).toBe("active");
  });
});

describe("errorCodeLabel", () => {
  it("maps typed discovery failures to their own keys", () => {
    const { t } = translateDouble();
    expect(errorCodeLabel(t, "query_timed_out")).toBe("monitor.capabilities.errors.queryTimedOut");
    expect(errorCodeLabel(t, "protocol_incompatible")).toBe(
      "monitor.capabilities.errors.protocolIncompatible",
    );
  });

  it("makes an unregistered code readable instead of blank", () => {
    const { t } = translateDouble();
    expect(errorCodeLabel(t, "some_future_code")).toBe("some future code");
  });

  it("shows a placeholder when no code was recorded", () => {
    const { t } = translateDouble();
    expect(errorCodeLabel(t, undefined)).toBe("—");
    expect(errorCodeLabel(t, "")).toBe("—");
  });
});
