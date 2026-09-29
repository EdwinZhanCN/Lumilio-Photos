import {
  ArrowUpRight,
  BrainCircuit,
  Check,
  Download,
  HardDrive,
  RotateCcw,
  Server,
  Settings2,
  X,
} from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useEffect, useLayoutEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { DesktopService, RuntimeService } from "../../bindings/desktop/internal/control/index.js";
import { type DesktopSnapshot } from "../../bindings/desktop/internal/control/dto/models.js";
import { AnimatedBadge } from "@/components/motion/animated-badge";
import {
  AnimatedToastStack,
  type ToastInput,
  useAnimatedToastStack,
} from "@/components/motion/animated-toast-stack";
import { Button, StatefulButton } from "@/components/motion/button";
import { Dock, DockItem, DockSeparator } from "@/components/motion/dock";
import { Loader } from "@/components/motion/loader";
import { ThemeToggle } from "@/components/motion/theme-toggle";
import { Tooltip } from "@/components/motion/tooltip";
import { PageHeading, SettingRow, SettingsSection } from "@/components/settings/setting-layout";
import {
  type SettingsDraftController,
  useSettingsDraft,
} from "@/features/settings/use-settings-draft";
import { SnapshotClient } from "@/lib/desktop/SnapshotClient";
import { errorMessage } from "@/lib/desktop/errors";
import { applyLocale } from "@/lib/i18n";
import { useTheme } from "@/lib/use-theme";
import { appIconURL, presentationStatus, useTrackedOperation } from "./shared";
import { Onboarding } from "./Onboarding";
import { GeneralPanel } from "./panels/GeneralPanel";
import { ServerPanel } from "./panels/ServerPanel";
import { StoragePanel } from "./panels/StoragePanel";
import { LumenPanel } from "./panels/LumenPanel";
import { UpdatesPanel } from "./panels/UpdatesPanel";

const dockRoutes = [
  { route: "/general", key: "general", icon: Settings2 },
  { route: "/server", key: "server", icon: Server },
  { route: "/storage", key: "storage", icon: HardDrive },
  { route: "/lumen", key: "lumen", icon: BrainCircuit },
  { route: "/updates", key: "updates", icon: Download },
] as const;

type MainRoute = (typeof dockRoutes)[number]["route"];

export function App() {
  const { t } = useTranslation();
  const client = useMemo(() => new SnapshotClient(), []);
  const { toasts, showToast, dismissToast } = useAnimatedToastStack({ limit: 4 });
  const [snapshot, setSnapshot] = useState<DesktopSnapshot | null>(null);
  const [route, setRoute] = useState("/general");
  const [error, setError] = useState<string | null>(null);
  const draft = useSettingsDraft(snapshot, showToast);
  const preferences = draft.preferences ?? snapshot?.host.preferences ?? null;
  const theme = useTheme(preferences);

  // Preview draft preferences immediately. Cancel reloads the persisted
  // snapshot, so the preview still has one authoritative save boundary.
  useEffect(() => {
    if (preferences?.locale) applyLocale(preferences.locale);
  }, [preferences?.locale]);

  useLayoutEffect(() => {
    // beUI overlays portal to document.body, so the theme marker must live on
    // the document root as well as the shell to keep their semantic tokens in sync.
    document.documentElement.classList.toggle("dark", theme === "dark");
  }, [theme]);

  useEffect(() => {
    let mounted = true;
    const unsubscribe = client.subscribe((next) => {
      if (!mounted) return;
      setSnapshot(next);
      setRoute(next.host.settingsNavigation.route || "/general");
    });
    void client.start().catch((reason: unknown) => {
      if (mounted) setError(errorMessage(reason));
    });
    return () => {
      mounted = false;
      unsubscribe();
      client.close();
    };
  }, [client]);

  const navigate = (next: string) => {
    setRoute(next);
    void DesktopService.ShowSettings(next).catch((reason: unknown) => {
      showToast({
        title: t("toast.pageFailed"),
        description: errorMessage(reason),
        status: "error",
      });
    });
  };

  if (error) return <RecoveryFallback message={error} />;
  if (!snapshot) return <LoadingScreen />;

  const onboarding = route === "/onboarding";
  const recovery = route === "/recovery";
  // Any non-wizard route while the runtime has no saved configuration shows
  // the setup-required guide instead of a bare panel: entries like the tray's
  // storage menu must never strand the user in a Not Configured dead end.
  const setupRequired = !onboarding && !recovery && !snapshot.runtime.configured;
  const activeRoute = normalizeRoute(route);

  return (
    <div
      className={`desktop-shell${theme === "dark" ? " dark" : ""}${onboarding ? " onboarding-shell" : ""}`}
    >
      {!onboarding ? (
        <header className="window-header">
          <button className="brand" type="button" onClick={() => navigate("/general")}>
            <img src={appIconURL} alt="" className="brand-icon" />
            <span>{t("product.compactName", "Lumilio Photos")}</span>
          </button>
          <div className="header-actions">
            <AnimatedBadge status={presentationStatus(snapshot.runtime.presentation)} size="sm">
              {snapshot.runtime.presentation.label}
            </AnimatedBadge>
            <Button
              size="sm"
              disabled={!snapshot.runtime.capabilities.canOpenProduct}
              onClick={() => void openProduct(showToast, t)}
            >
              {t("header.openProduct")} <ArrowUpRight className="size-3.5" />
            </Button>
          </div>
        </header>
      ) : null}

      <main className={onboarding ? "bootstrap-main" : "content-shell"}>
        <AnimatePresence mode="wait" initial={false}>
          <motion.div
            key={route}
            initial={{ opacity: 0, y: 18 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
            transition={{ duration: 0.16 }}
            className={onboarding || setupRequired ? "bootstrap-frame" : "page-frame"}
          >
            {onboarding ? (
              <Onboarding
                snapshot={snapshot}
                draft={draft}
                onComplete={() => navigate("/general")}
              />
            ) : recovery ? (
              <RecoveryPage snapshot={snapshot} showToast={showToast} navigate={navigate} />
            ) : setupRequired ? (
              <SetupRequiredPage onStart={() => navigate("/onboarding")} />
            ) : activeRoute === "/general" ? (
              <GeneralPanel draft={draft} />
            ) : activeRoute === "/server" ? (
              <ServerPanel snapshot={snapshot} draft={draft} showToast={showToast} />
            ) : activeRoute === "/storage" ? (
              <StoragePanel snapshot={snapshot} draft={draft} showToast={showToast} />
            ) : activeRoute === "/lumen" ? (
              <LumenPanel snapshot={snapshot} showToast={showToast} />
            ) : (
              <UpdatesPanel snapshot={snapshot} draft={draft} showToast={showToast} />
            )}
          </motion.div>
        </AnimatePresence>
      </main>

      {!onboarding && !recovery && !setupRequired ? (
        <AppDock route={activeRoute} navigate={navigate} draft={draft} resolvedTheme={theme} />
      ) : null}
      <AnimatedToastStack
        toasts={toasts}
        onDismiss={dismissToast}
        placement="fixed"
        position="bottom-right"
        className="desktop-toasts"
      />
    </div>
  );
}

function LoadingScreen() {
  const { t } = useTranslation();
  return (
    <main className="boot-screen">
      <img
        src={appIconURL}
        alt={t("product.compactName", "Lumilio Photos")}
        className="boot-icon"
      />
      <Loader variant="ascii-braille" size={22} label={t("loading.loadingState")} />
      <span>{t("loading.opening")}</span>
    </main>
  );
}

function SetupRequiredPage({ onStart }: { onStart: () => void }) {
  const { t } = useTranslation();
  return (
    <section className="bootstrap" aria-label={t("setupRequired.title")}>
      <header className="bootstrap-header">
        <div className="bootstrap-brand">
          <img src={appIconURL} alt="" className="bootstrap-icon" />
          <span>{t("product.compactName", "Lumilio Photos")}</span>
        </div>
      </header>
      <div className="bootstrap-slide">
        <PageHeading
          title={t("setupRequired.title")}
          description={t("setupRequired.description")}
        />
        <SettingsSection title={t("setupRequired.nextStep")}>
          <SettingRow
            title={t("setupRequired.finishSetup")}
            description={t("setupRequired.minutes")}
          >
            <Button onClick={onStart}>{t("setupRequired.backToSetup")}</Button>
          </SettingRow>
        </SettingsSection>
      </div>
    </section>
  );
}

function AppDock({
  route,
  navigate,
  draft,
  resolvedTheme,
}: {
  route: MainRoute;
  navigate: (route: string) => void;
  draft: SettingsDraftController;
  resolvedTheme: "light" | "dark";
}) {
  const { t } = useTranslation();
  const busy = draft.phase === "preparing" || draft.phase === "saving";
  const system = !draft.preferences?.theme || draft.preferences.theme === "system";
  const currentLabel = system
    ? t("general.themeSystem")
    : resolvedTheme === "dark"
      ? t("general.themeDark")
      : t("general.themeLight");
  const nextTheme = resolvedTheme === "dark" ? "light" : "dark";
  const cancelDisabled = draft.phase === "saving";
  const saveDisabled =
    draft.phase === "preparing" || draft.phase === "saving" || draft.validation?.valid === false;
  return (
    <nav className="dock-position" aria-label={t("dock.label")}>
      <Dock>
        {dockRoutes.map((item) => {
          const Icon = item.icon;
          const label = t(`dock.${item.key}`);
          return (
            <Tooltip key={item.route} content={label} side="top">
              <DockItem
                active={route === item.route}
                onClick={() => navigate(item.route)}
                aria-label={label}
              >
                <Icon className="size-[18px]" strokeWidth={1.8} />
              </DockItem>
            </Tooltip>
          );
        })}
        <DockSeparator />
        <Tooltip content={`${t("general.theme")}: ${currentLabel}`} side="top">
          <DockItem>
            <ThemeToggle
              isDark={resolvedTheme === "dark"}
              onToggle={() => draft.updatePreference("theme", nextTheme)}
              disabled={!draft.preferences || busy}
              variant="circle-blur"
              start="center"
              aria-label={`${t("general.theme")}: ${currentLabel}`}
              title={`${t("general.theme")}: ${currentLabel}`}
              className="size-full"
              iconClassName="size-[18px]"
            />
          </DockItem>
        </Tooltip>
        {draft.dirty ? (
          <>
            <DockSeparator />
            <Tooltip content={t("common.cancel")} side="top">
              <DockItem
                aria-label={t("common.cancel")}
                onClick={() => {
                  if (!cancelDisabled) void draft.cancel();
                }}
              >
                <X className="size-[18px]" strokeWidth={1.8} />
              </DockItem>
            </Tooltip>
            <Tooltip content={t("common.save", "Save")} side="top">
              <DockItem
                aria-label={t("common.save", "Save")}
                onClick={() => {
                  if (!saveDisabled) void draft.save();
                }}
              >
                <Check className="size-[18px] text-green-500" strokeWidth={2} />
              </DockItem>
            </Tooltip>
          </>
        ) : null}
      </Dock>
    </nav>
  );
}

function RecoveryPage({
  snapshot,
  showToast,
  navigate,
}: {
  snapshot: DesktopSnapshot;
  showToast: (input: ToastInput) => string;
  navigate: (route: string) => void;
}) {
  const { t } = useTranslation();
  const operation = useTrackedOperation(snapshot.operations, {
    onSucceeded: () => navigate("/server"),
    onFailed: (message) =>
      showToast({ title: t("recovery.failed"), description: message, status: "error" }),
  });
  const state = operation.state;
  const restore = async () => {
    operation.begin();
    try {
      const receipt = await RuntimeService.RestoreLastKnownGood(
        `recovery-${crypto.randomUUID()}`,
        snapshot.runtime.version,
      );
      operation.track(receipt);
    } catch (reason: unknown) {
      operation.reject(reason);
    }
  };
  return (
    <div className="recovery-page">
      <div className="recovery-icon">
        <RotateCcw className="size-5" />
      </div>
      <PageHeading
        title={t("recovery.title")}
        description={snapshot.host.recovery?.message || t("recovery.description")}
      />
      <div className="recovery-actions">
        <StatefulButton
          state={state}
          loadingText={t("recovery.restoring")}
          successText={t("recovery.restored")}
          onClick={() => void restore()}
        >
          {t("recovery.restore")}
        </StatefulButton>
        <Button variant="secondary" onClick={() => navigate("/server")}>
          {t("recovery.reviewSettings")}
        </Button>
      </div>
    </div>
  );
}

function RecoveryFallback({ message }: { message: string }) {
  const { t } = useTranslation();
  return (
    <main className="boot-screen recovery-fallback dark">
      <div className="recovery-icon">
        <RotateCcw className="size-5" />
      </div>
      <h1>{t("recovery.fallbackTitle")}</h1>
      <p>{message}</p>
      <Button onClick={() => window.location.reload()}>{t("recovery.tryAgain")}</Button>
    </main>
  );
}

function normalizeRoute(route: string): MainRoute {
  if (route === "/runtime") return "/server";
  if (route === "/overview" || route === "/settings" || route === "/diagnostics") return "/general";
  const found = dockRoutes.find((item) => item.route === route);
  return found?.route ?? "/general";
}

async function openProduct(showToast: (input: ToastInput) => string, t: (key: string) => string) {
  try {
    await DesktopService.OpenProduct();
  } catch (reason: unknown) {
    showToast({ title: t("toast.openFailed"), description: errorMessage(reason), status: "error" });
  }
}
