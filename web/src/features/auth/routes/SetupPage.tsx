import React, { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { HardDrive, Image as ImageIcon, Server, ShieldCheck } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n.tsx";
import { setupStatusQueryKey } from "../hooks/useSetupStatus.ts";
import type { ApiResult } from "../auth.type.ts";
import { AuthShell, Btn, CardHead, Field, InlineError, TextInput } from "../components/ui.tsx";

function getApiMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message;
  if (error && typeof error === "object") {
    const apiError = error as { message?: string; error?: string };
    if (apiError.message) return apiError.message;
    if (apiError.error) return apiError.error;
  }
  return fallback;
}

/**
 * First-run system setup. While the system configuration payload is missing on
 * disk, all traffic is routed here. Submitting rotates the database credential
 * away from the temporary bootstrap password and persists system metadata; the
 * app then continues into the administrator bootstrap wizard.
 */
const SetupPage: React.FC = () => {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const setupMutation = $api.useMutation("post", "/api/v1/setup");

  const [siteName, setSiteName] = useState("");
  const [adminUsername, setAdminUsername] = useState("");
  const [error, setError] = useState<string | null>(null);

  const appName = t("app.name", { defaultValue: "Lumilio" });

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);
    try {
      const response = await setupMutation.mutateAsync({
        body: {
          site_name: siteName.trim(),
          admin_username: adminUsername.trim(),
        },
      });
      const payload = response as ApiResult | undefined;
      if (payload && payload.code !== undefined && payload.code !== 0) {
        throw new Error(payload.message || t("auth.setup.error"));
      }
      // The system is now initialized — re-evaluate the gate so the admin
      // bootstrap wizard takes over.
      await queryClient.invalidateQueries({ queryKey: setupStatusQueryKey });
    } catch (setupError) {
      setError(
        getApiMessage(
          setupError,
          t("auth.setup.error", {
            defaultValue: "Setup failed. Check the server logs and try again.",
          }),
        ),
      );
    }
  };

  return (
    <div className="grid min-h-screen place-items-center bg-base-200 px-4 py-10">
      <AuthShell appName={appName}>
        <CardHead
          icon={Server}
          title={t("auth.setup.title", {
            defaultValue: "Initialize your server",
          })}
          sub={t("auth.setup.subtitle", {
            defaultValue:
              "Secure this Lumilio server before creating any accounts. We’ll rotate the database credential and store it locally.",
          })}
        />

        <div className="grid gap-2.5">
          {(
            [
              [
                ShieldCheck,
                t("auth.setup.rotateTitle", {
                  defaultValue: "Database credential rotated",
                }),
                t("auth.setup.rotateBody", {
                  defaultValue: "A new 32-character password replaces the temporary bootstrap one.",
                }),
              ],
              [
                HardDrive,
                t("auth.setup.localTitle", {
                  defaultValue: "Stored locally only",
                }),
                t("auth.setup.localBody", {
                  defaultValue: "The secret is written with locked-down permissions on this host.",
                }),
              ],
            ] as Array<[typeof ShieldCheck, string, string]>
          ).map(([Icon, title, body]) => (
            <div
              key={title}
              className="flex items-start gap-3 rounded-xl border border-base-200 px-4 py-3"
            >
              <Icon size={18} className="mt-0.5 shrink-0 text-base-content/45" />
              <div>
                <p className="text-sm font-medium text-base-content">{title}</p>
                <p className="text-xs text-base-content/55">{body}</p>
              </div>
            </div>
          ))}
        </div>

        {error && <InlineError>{error}</InlineError>}

        <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
          <Field
            label={t("auth.setup.siteName", { defaultValue: "Library name" })}
            hint={t("auth.setup.siteNameHint", {
              defaultValue: "shown on the dashboard",
            })}
          >
            <TextInput
              icon={ImageIcon}
              type="text"
              placeholder={t("auth.setup.siteNamePlaceholder", {
                defaultValue: "My Photos",
              })}
              value={siteName}
              onChange={(e) => setSiteName(e.target.value)}
              autoFocus
            />
          </Field>

          <Field
            label={t("auth.setup.adminUsername", {
              defaultValue: "Administrator username",
            })}
            hint={t("auth.setup.adminUsernameHint", {
              defaultValue: "you’ll set the password next",
            })}
          >
            <TextInput
              icon={Server}
              type="text"
              placeholder="admin"
              value={adminUsername}
              onChange={(e) => setAdminUsername(e.target.value)}
            />
          </Field>

          <Btn type="submit" variant="primary" loading={setupMutation.isPending}>
            {t("auth.setup.submit", {
              defaultValue: "Initialize & continue",
            })}
          </Btn>
        </form>
      </AuthShell>
    </div>
  );
};

export default SetupPage;
