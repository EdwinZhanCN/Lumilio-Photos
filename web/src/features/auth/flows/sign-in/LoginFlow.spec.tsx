import { describe, expect, it } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import LoginFlow from "./LoginFlow";
import type { BrowserCapabilities, LoginOptionsResponse } from "../../types";

describe("LoginFlow deployment capabilities", () => {
  it("shows the LAN HTTP warning and does not offer a passkey", async () => {
    const capabilities = {
      primary_origin: "http://localhost:6680",
      current_origin: "http://192.168.1.20:6680",
      passkey_available: false,
      passkey_unavailable_reason: "secure_origin_required",
      insecure_transport: true,
      proxy_required: false,
      via_trusted_proxy: false,
    } satisfies BrowserCapabilities;
    const loginOptions = {
      password: true,
      // Deliberately inconsistent with capabilities: the flow must fail closed
      // even if an older server returned account-only login options.
      passkey: true,
    } satisfies LoginOptionsResponse;
    worker.use(
      http.get("*/api/v1/auth/browser-capabilities", () => HttpResponse.json(capabilities)),
      http.post("*/api/v1/auth/login/options", () => HttpResponse.json(loginOptions)),
    );

    const screen = await renderWithProviders(<LoginFlow />, { auth: true });

    await expect.element(screen.getByText(t("auth.browserSecurity.insecureTitle"))).toBeVisible();
    await screen.getByLabelText(t("auth.login.username"), { exact: true }).fill("admin");
    await screen.getByRole("button", { name: t("auth.login.continue") }).click();

    await expect
      .element(screen.getByLabelText(t("auth.login.password"), { exact: true }))
      .toBeVisible();
    await expect
      .element(screen.getByRole("button", { name: t("auth.login.passkeySubmit") }))
      .not.toBeInTheDocument();
  });

  it("offers a passkey only when both browser and server policy allow it", async () => {
    const capabilities = {
      primary_origin: "http://localhost:6680",
      current_origin: "http://localhost:6680",
      passkey_available: true,
      insecure_transport: false,
      proxy_required: false,
      via_trusted_proxy: false,
    } satisfies BrowserCapabilities;
    const loginOptions = {
      password: true,
      passkey: true,
    } satisfies LoginOptionsResponse;
    worker.use(
      http.get("*/api/v1/auth/browser-capabilities", () => HttpResponse.json(capabilities)),
      http.post("*/api/v1/auth/login/options", () => HttpResponse.json(loginOptions)),
    );

    const screen = await renderWithProviders(<LoginFlow />, { auth: true });
    await screen.getByLabelText(t("auth.login.username"), { exact: true }).fill("admin");
    await screen.getByRole("button", { name: t("auth.login.continue") }).click();

    await expect
      .element(screen.getByRole("button", { name: t("auth.login.passkeySubmit") }))
      .toBeVisible();
  });
});
