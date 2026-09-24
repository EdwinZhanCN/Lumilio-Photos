import type { TFunction } from "i18next";
import type { components } from "@/lib/http-commons/schema";

/**
 * Lumen discovery vocabulary.
 *
 * Two snapshots feed this module and they must stay distinguishable:
 * `GET /api/v1/capabilities` is the de-sensitized public aggregate, while
 * `GET /api/v1/admin/lumen/runtime` is the authenticated diagnostic view
 * ([decision](../../../../.agents/decisions/2026-08-25-lumen-dynamic-discovery.md)).
 * The module owns the state vocabulary only; which snapshot a surface reads
 * stays with that surface.
 */

type LumenNodeRuntime = components["schemas"]["dto.LumenNodeRuntimeDTO"];

/** Composite discovery lifecycle, owned by the Server. */
export type RuntimeState = "disabled" | "starting" | "healthy" | "degraded";

/** A node's effective verdict, derived from transport then compatibility. */
export type NodeVerdict = "unavailable" | "connecting" | "pending" | "incompatible" | "active";

/** Presentation tone. Badge classes stay in the flow that renders them. */
export type StatusTone = "ok" | "danger" | "warning" | "neutral";

/**
 * Tone for any Server-reported state word: backend state, node verdict,
 * transport, or compatibility. Unknown or disabled values stay neutral rather
 * than implying health.
 */
export function statusTone(state?: string): StatusTone {
  switch (state) {
    case "healthy":
    case "compatible":
    case "ready":
      return "ok";
    case "degraded":
    case "incompatible":
    case "unavailable":
      return "danger";
    case "starting":
    case "pending":
    case "connecting":
      return "warning";
    default:
      return "neutral";
  }
}

export function stateLabel(t: TFunction, state?: string): string {
  switch (state) {
    case "disabled":
      return t("monitor.capabilities.states.disabled", "disabled");
    case "starting":
      return t("monitor.capabilities.states.starting", "starting");
    case "healthy":
      return t("monitor.capabilities.states.healthy", "healthy");
    case "degraded":
      return t("monitor.capabilities.states.degraded", "degraded");
    case "connecting":
      return t("monitor.capabilities.states.connecting", "connecting");
    case "ready":
      return t("monitor.capabilities.states.ready", "ready");
    case "unavailable":
      return t("monitor.capabilities.states.unavailable", "unavailable");
    case "pending":
      return t("monitor.capabilities.states.pending", "pending");
    case "compatible":
      return t("monitor.capabilities.states.compatible", "compatible");
    case "incompatible":
      return t("monitor.capabilities.states.incompatible", "incompatible");
    case "active":
      return t("monitor.capabilities.states.active", "active");
    default:
      return t("monitor.capabilities.states.unknown", "unknown");
  }
}

/**
 * Transport outranks compatibility: a node that cannot be reached has no
 * compatibility verdict worth showing, and an unreachable or incompatible node
 * must never present its advertised tasks as available.
 */
export function nodeVerdict(node: LumenNodeRuntime): NodeVerdict {
  if (node.transport === "unavailable") return "unavailable";
  if (node.transport !== "ready") return "connecting";
  if (node.compatibility === "incompatible") return "incompatible";
  if (node.compatibility !== "compatible") return "pending";
  return "active";
}

/** Typed discovery failure, or the raw code with underscores made readable. */
export function errorCodeLabel(t: TFunction, code?: string): string {
  switch (code) {
    case "query_timed_out":
      return t("monitor.capabilities.errors.queryTimedOut", "Discovery scan timed out");
    case "query_failed":
      return t("monitor.capabilities.errors.queryFailed", "Discovery scan failed");
    case "socket_open_failed":
      return t("monitor.capabilities.errors.socketOpenFailed", "Discovery socket could not open");
    case "socket_send_failed":
      return t("monitor.capabilities.errors.socketSendFailed", "Discovery query could not be sent");
    case "socket_read_failed":
      return t(
        "monitor.capabilities.errors.socketReadFailed",
        "Discovery replies could not be read",
      );
    case "resolve_failed":
      return t(
        "monitor.capabilities.errors.resolveFailed",
        "A matching service had no usable address",
      );
    case "watch_start_failed":
      return t("monitor.capabilities.errors.watchStartFailed", "Discovery backend could not start");
    case "watch_closed":
      return t("monitor.capabilities.errors.watchClosed", "Discovery backend stopped unexpectedly");
    case "transport_unavailable":
      return t("monitor.capabilities.errors.transportUnavailable", "Transport unavailable");
    case "capability_rpc_unimplemented":
      return t(
        "monitor.capabilities.errors.capabilityUnimplemented",
        "Capability exchange is not implemented",
      );
    case "protocol_incompatible":
      return t(
        "monitor.capabilities.errors.protocolIncompatible",
        "Protocol version is incompatible",
      );
    case "incompatible":
      return t("monitor.capabilities.errors.incompatible", "Node is incompatible");
    default:
      return code ? code.replaceAll("_", " ") : "—";
  }
}
