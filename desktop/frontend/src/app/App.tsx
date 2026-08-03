import {
  ArrowUpRight,
  BrainCircuit,
  Check,
  Download,
  FolderOpen,
  FolderPlus,
  HardDrive,
  Info,
  RefreshCw,
  RotateCcw,
  Server,
  Settings2,
  X,
} from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useCallback, useEffect, useLayoutEffect, useMemo, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import {
  DesktopService,
  LumenService,
  RuntimeService,
  StorageService,
  UpdateService,
} from "../../bindings/desktop/internal/control/index.js";
import {
  LumenInstallPhase,
  LumenControlPhase,
  LumenProcessPhase,
  RuntimePhase,
  type DesktopPreferences,
  type DesktopSnapshot,
  type LumenLogEntry,
  type LumenSnapshot,
  type ProcessPresentation,
  type RuntimeConfigSettings,
  type StorageShortcut,
} from "../../bindings/desktop/internal/control/dto/models.js";
import { AnimatedBadge, type AnimatedBadgeStatus } from "@/components/motion/animated-badge";
import {
  AnimatedToastStack,
  type ToastInput,
  useAnimatedToastStack,
} from "@/components/motion/animated-toast-stack";
import { Button, StatefulButton, type ButtonState } from "@/components/motion/button";
import { Dock, DockItem, DockSeparator } from "@/components/motion/dock";
import { Loader } from "@/components/motion/loader";
import { ThemeToggle } from "@/components/motion/theme-toggle";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/motion/select";
import { Switch } from "@/components/motion/switch";
import { Tooltip } from "@/components/motion/tooltip";
import {
  InlineNotice,
  PageHeading,
  SettingRow,
  SettingsSection,
} from "@/components/settings/setting-layout";
import { RuntimeConfigWorkspace } from "@/features/runtime/RuntimeConfigWorkspace";
import {
  type SettingsDraftController,
  useSettingsDraft,
} from "@/features/settings/use-settings-draft";
import { SnapshotClient } from "@/lib/desktop/SnapshotClient";
import { errorMessage } from "@/lib/desktop/errors";
import { applyLocale } from "@/lib/i18n";
import { useTheme } from "@/lib/use-theme";

const appIconURL = new URL("../../../build/appicon.png", import.meta.url).href;

const dockRoutes = [
  { route: "/general", key: "general", icon: Settings2 },
  { route: "/server", key: "server", icon: Server },
  { route: "/storage", key: "storage", icon: HardDrive },
  { route: "/lumen", key: "lumen", icon: BrainCircuit },
  { route: "/updates", key: "updates", icon: Download },
] as const;

type MainRoute = (typeof dockRoutes)[number]["route"];

type PresetCapability = {
  id: "semantic" | "ocr" | "people" | "species";
  model: string;
  dataset?: string;
};

type PresetInfo = {
  id: "minimal" | "basic" | "brave";
  capabilities: PresetCapability[];
};

const presetCatalog: PresetInfo[] = [
  {
    id: "minimal",
    capabilities: [
      { id: "semantic", model: "siglip2-base-patch16-224" },
      { id: "people", model: "antelopev2" },
    ],
  },
  {
    id: "basic",
    capabilities: [
      { id: "semantic", model: "siglip2-base-patch16-224" },
      { id: "ocr", model: "pp-ocrv6-small" },
      { id: "people", model: "antelopev2" },
      { id: "species", model: "bioclip-2", dataset: "TreeOfLife200MCore" },
    ],
  },
  {
    id: "brave",
    capabilities: [
      { id: "semantic", model: "siglip2-so400m-patch14-384" },
      { id: "ocr", model: "pp-ocrv6-small" },
      { id: "people", model: "antelopev2" },
      { id: "species", model: "bioclip-2", dataset: "TreeOfLife200M" },
    ],
  },
];

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
    <div className={`desktop-shell${theme === "dark" ? " dark" : ""}${onboarding ? " onboarding-shell" : ""}`}>
      {!onboarding ? (
        <header className="window-header">
          <button className="brand" type="button" onClick={() => navigate("/general")}>
            <img src={appIconURL} alt="" className="brand-icon" />
            <span>Lumilio</span>
          </button>
          <div className="header-actions">
            <AnimatedBadge
              status={presentationStatus(snapshot.runtime.presentation)}
              size="sm"
            >
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

      {!onboarding && !recovery && !setupRequired ? <AppDock route={activeRoute} navigate={navigate} draft={draft} resolvedTheme={theme} /> : null}
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
      <img src={appIconURL} alt="Lumilio Photos" className="boot-icon" />
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
          <span>Lumilio Photos</span>
        </div>
      </header>
      <div className="bootstrap-slide">
        <PageHeading
          title={t("setupRequired.title")}
          description={t("setupRequired.description")}
        />
        <SettingsSection title={t("setupRequired.nextStep")}>
          <SettingRow title={t("setupRequired.finishSetup")} description={t("setupRequired.minutes")}>
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
  const saveDisabled = draft.phase === "preparing" || draft.phase === "saving" || draft.validation?.valid === false;
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

function Onboarding({
  snapshot,
  draft,
  onComplete,
}: {
  snapshot: DesktopSnapshot;
  draft: SettingsDraftController;
  onComplete: () => void;
}) {
  const { t } = useTranslation();
  const [step, setStep] = useState(0);
  const total = 4;

  if (!draft.runtime || !draft.preferences) {
    return (
      <div className="bootstrap-loading">
        <img src={appIconURL} alt="Lumilio Photos" className="bootstrap-icon" />
        <Loader variant="ascii-braille" size={24} label={t("onboarding.preparing")} />
        <span>{t("onboarding.preparingDescription")}</span>
      </div>
    );
  }

  const finish = async () => {
    if (await draft.save()) onComplete();
  };
  const busy = draft.phase === "preparing" || draft.phase === "saving";

  return (
    <section className="bootstrap" aria-label={t("onboarding.ariaLabel", "Lumilio Desktop setup")}>
      <header className="bootstrap-header">
        <div className="bootstrap-brand">
          <img src={appIconURL} alt="" className="bootstrap-icon" />
          <span>Lumilio Photos</span>
        </div>
        <span>{t("onboarding.step", { current: step + 1, total })}</span>
      </header>

      <div className="bootstrap-progress" aria-hidden>
        {Array.from({ length: total }, (_, index) => <i key={index} className={index <= step ? "active" : ""} />)}
      </div>

      <AnimatePresence mode="wait" initial={false}>
        <motion.div
          key={step}
          className="bootstrap-slide"
          initial={{ opacity: 0, x: 18 }}
          animate={{ opacity: 1, x: 0 }}
          exit={{ opacity: 0, x: -14 }}
          transition={{ duration: 0.2 }}
        >
          {step === 0 ? (
            <>
              <PageHeading title={t("onboarding.generalTitle")} description={t("onboarding.generalDescription")} />
              <SettingsSection title={t("onboarding.desktop")}>
                <SettingRow title={t("onboarding.language")} description={t("onboarding.languageDescription")}>
                  <LanguageSelect preferences={draft.preferences} update={draft.updatePreference} disabled={busy} />
                </SettingRow>
                <SettingRow title={t("onboarding.region")} description={t("onboarding.regionDescription")}>
                  <RegionSelect preferences={draft.preferences} update={draft.updatePreference} disabled={busy} />
                </SettingRow>
              </SettingsSection>
            </>
          ) : step === 1 ? (
            <>
              <PageHeading title={t("onboarding.storageTitle")} description={t("onboarding.storageDescription")} />
              <SettingsSection title={t("onboarding.storage")}>
                <SettingRow title={t("onboarding.storageLocation")} description={draft.runtime.storagePath}>
                  <Button variant="secondary" disabled={busy} onClick={() => void draft.chooseDefaultStorage()}>
                    <FolderOpen className="size-4" /> {t("common.choose")}
                  </Button>
                </SettingRow>
              </SettingsSection>
            </>
          ) : step === 2 ? (
            <>
              <PageHeading title={t("onboarding.networkTitle")} description={t("onboarding.networkDescription")} />
              <SettingsSection title={t("onboarding.server")}>
                <SettingRow title={t("onboarding.networkAccess")} description={t("onboarding.networkRecommended")}>
                  <NetworkSelect settings={draft.runtime} update={draft.updateRuntime} disabled={busy} />
                </SettingRow>
              </SettingsSection>
            </>
          ) : (
            <>
              <PageHeading title={t("onboarding.readyTitle")} description={t("onboarding.readyDescription")} />
              <SettingsSection title={t("onboarding.summary")}>
                <SettingRow title={t("onboarding.storageLocation")} description={draft.runtime.storagePath}>
                  <span className="setting-value">{t("onboarding.selected")}</span>
                </SettingRow>
                <SettingRow title={t("onboarding.networkAccess")} description={networkLabel(draft.runtime.networkMode, t)}>
                  <span className="setting-value">{t("onboarding.configured")}</span>
                </SettingRow>
                <SettingRow title={t("onboarding.lumenAI")} description={snapshot.lumen.installerAvailable && snapshot.lumen.processAvailable ? t("onboarding.lumenAvailable") : t("onboarding.lumenNotIncluded")}>
                  <AnimatedBadge status="neutral" size="sm">
                    {snapshot.lumen.installerAvailable && snapshot.lumen.processAvailable ? t("onboarding.optional") : t("onboarding.unavailable")}
                  </AnimatedBadge>
                </SettingRow>
              </SettingsSection>
              {draft.phase === "error" && draft.error ? (
                <InlineNotice tone="danger" title={t("onboarding.setupFailed")}>{draft.error}</InlineNotice>
              ) : null}
            </>
          )}
        </motion.div>
      </AnimatePresence>

      <footer className="bootstrap-actions">
        <Button variant="ghost" disabled={step === 0 || busy} onClick={() => setStep((current) => current - 1)}>
          {t("common.back")}
        </Button>
        {step < total - 1 ? (
          <Button disabled={busy || (step === 1 && !draft.runtime.storagePath)} onClick={() => setStep((current) => current + 1)}>
            {t("common.continue")}
          </Button>
        ) : (
          <Button disabled={busy || draft.validation?.valid === false} onClick={() => void finish()}>
            {draft.phase === "saving" ? t("common.finishing") : t("onboarding.finishSetup")}
          </Button>
        )}
      </footer>
    </section>
  );
}

function GeneralPanel({ draft }: { draft: SettingsDraftController }) {
  const { t } = useTranslation();
  if (!draft.preferences) return <SettingsLoading label={t("common.loadingPreferences")} />;
  const busy = draft.phase === "preparing" || draft.phase === "saving";
  return (
    <>
      <PageHeading title={t("general.title")} description={t("general.description")} />
      <SettingsSection title={t("general.display")}>
        <SettingRow title={t("general.language")} description={t("general.languageDescription")}>
          <LanguageSelect preferences={draft.preferences} update={draft.updatePreference} disabled={busy} />
        </SettingRow>
        <SettingRow title={t("general.region")} description={t("general.regionDescription")}>
          <RegionSelect preferences={draft.preferences} update={draft.updatePreference} disabled={busy} />
        </SettingRow>
        <SettingRow title={t("general.theme")} description={t("general.themeDescription")}>
          <ThemeStatusControl preferences={draft.preferences} update={draft.updatePreference} disabled={busy} />
        </SettingRow>
        <SettingRow title={t("general.openOnLaunch")} description={t("general.openOnLaunchDescription")}>
          <Switch
            checked={draft.preferences.openProductOnLaunch}
            onCheckedChange={(value) => draft.updatePreference("openProductOnLaunch", value)}
            disabled={busy}
            ariaLabel={t("general.openOnLaunch")}
          />
        </SettingRow>
      </SettingsSection>
    </>
  );
}

function ServerPanel({
  snapshot,
  draft,
  showToast,
}: {
  snapshot: DesktopSnapshot;
  draft: SettingsDraftController;
  showToast: (input: ToastInput) => string;
}) {
  const { t } = useTranslation();
  const runtime = snapshot.runtime;
  const [actionState, setActionState] = useState<ButtonState>("idle");
  const [actionError, setActionError] = useState<string | null>(null);

  const invoke = async (action: "start" | "stop" | "restart" | "retry") => {
    setActionState("loading");
    setActionError(null);
    const requestID = `runtime-${crypto.randomUUID()}`;
    try {
      if (action === "start") await RuntimeService.Start(requestID, runtime.version);
      else if (action === "stop") await RuntimeService.Stop(requestID, runtime.version);
      else if (action === "restart") await RuntimeService.Restart(requestID, runtime.version);
      else await RuntimeService.RetryCleanup(requestID, runtime.version);
      setActionState("success");
      window.setTimeout(() => setActionState("idle"), 1200);
    } catch (reason: unknown) {
      const message = errorMessage(reason);
      setActionState("error");
      setActionError(message);
      showToast({ title: t("server.actionFailed"), description: message, status: "error" });
    }
  };

  return (
    <>
      <PageHeading title={t("server.title")} description={t("server.description")} />
      <SettingsSection title={t("server.runtime")}>
        <SettingRow title={t("server.status")} description={t("server.statusDescription")}>
          <RowActions>
            <AnimatedBadge status={presentationStatus(runtime.presentation)} size="sm">
              {runtime.presentation.label}
            </AnimatedBadge>
            {runtime.capabilities.canStartRuntime ? (
              <Button variant="secondary" size="sm" disabled={actionState === "loading"} onClick={() => void invoke("start")}>{t("common.start")}</Button>
            ) : null}
            {runtime.capabilities.canStopRuntime || runtime.capabilities.canRetryCleanupRuntime ? (
              <Button
                variant="secondary"
                size="sm"
                disabled={actionState === "loading"}
                onClick={() => void invoke(runtime.capabilities.canRetryCleanupRuntime ? "retry" : "stop")}
              >
                {runtime.capabilities.canRetryCleanupRuntime ? t("common.retryCleanup") : t("common.stop")}
              </Button>
            ) : null}
            {runtime.capabilities.canRestartRuntime ? (
              <StatefulButton
                variant="secondary"
                size="sm"
                state={actionState}
                loadingText={t("server.restarting")}
                successText={t("server.restarted")}
                icon={<RotateCcw className="size-3.5" />}
                onClick={() => void invoke("restart")}
              >
                {t("common.restart")}
              </StatefulButton>
            ) : null}
          </RowActions>
        </SettingRow>
      </SettingsSection>

      {actionError ? (
        <ActionNotice component={t("dock.server")} message={actionError} actionLabel={t("common.retry")} onAction={() => void invoke("start")} />
      ) : runtime.phase === RuntimePhase.RuntimeFailed ? (
        <ActionNotice
          component={t("dock.server")}
          message={t("server.failedMessage")}
          actionLabel={runtime.capabilities.canRetryCleanupRuntime ? t("common.retryCleanup") : undefined}
          onAction={runtime.capabilities.canRetryCleanupRuntime ? () => void invoke("retry") : undefined}
        />
      ) : null}

      {draft.runtime ? (
        <RuntimeConfigWorkspace
          settings={draft.runtime}
          disabled={draft.phase === "preparing" || draft.phase === "saving"}
          updateSetting={draft.updateRuntime}
          showToast={showToast}
        />
      ) : <SettingsLoading label={t("server.loadingSettings")} />}
    </>
  );
}

function StoragePanel({
  snapshot,
  draft,
  showToast,
}: {
  snapshot: DesktopSnapshot;
  draft: SettingsDraftController;
  showToast: (input: ToastInput) => string;
}) {
  const { t } = useTranslation();
  const [items, setItems] = useState<StorageShortcut[]>([]);
  const [state, setState] = useState<ButtonState>("idle");
  const [loadError, setLoadError] = useState<string | null>(null);

  const refresh = async () => {
    try {
      setLoadError(null);
      setItems((await StorageService.ListShortcuts()) || []);
    } catch (reason: unknown) {
      setLoadError(errorMessage(reason));
    }
  };

  useEffect(() => {
    void refresh();
  }, [snapshot.storage.version, snapshot.runtime.phase]);

  const addLocation = async () => {
    setState("loading");
    try {
      const path = await StorageService.PickLocation(t("storage.addLocationDialogTitle", "Add a Lumilio storage location"));
      if (!path) {
        setState("idle");
        return;
      }
      await StorageService.AddLocation(`storage-${crypto.randomUUID()}`, snapshot.storage.version, path, "");
      setState("success");
      window.setTimeout(() => setState("idle"), 1200);
    } catch (reason: unknown) {
      setState("error");
      showToast({ title: t("storage.addFailed"), description: errorMessage(reason), status: "error" });
    }
  };

  const configuredPath = draft.runtime?.storagePath || "";
  const defaultLocation = items.find((item) => item.kind === "default")
    || items.find((item) => configuredPath !== "" && item.path === configuredPath);
  const additional = items.filter((item) => item.id !== defaultLocation?.id);

  return (
    <>
      <PageHeading
        title={t("storage.title")}
        description={t("storage.description")}
        action={
          <StatefulButton
            state={state}
            loadingText={t("storage.choosing")}
            successText={t("storage.added")}
            icon={<FolderPlus className="size-4" />}
            disabled={snapshot.runtime.phase !== RuntimePhase.RuntimeRunning}
            onClick={() => void addLocation()}
          >
            {t("storage.addLocation")}
          </StatefulButton>
        }
      />

      {loadError ? (
        <ActionNotice component={t("dock.storage")} message={loadError} actionLabel={t("common.retry")} onAction={() => void refresh()} />
      ) : null}

      <SettingsSection title={t("storage.defaultLocation")}>
        <SettingRow
          title={t("storage.defaultLocation")}
          description={defaultLocation?.path || configuredPath || t("storage.noDefault")}
        >
          <Button
            variant="secondary"
            size="sm"
            disabled={!defaultLocation?.canOpen}
            onClick={() => defaultLocation && void StorageService.OpenLocation(defaultLocation.id)}
          >
            <FolderOpen className="size-3.5" /> {t("common.open")}
          </Button>
        </SettingRow>
      </SettingsSection>

      <SettingsSection title={t("storage.additional")}>
        {additional.length ? additional.map((item) => (
          <SettingRow key={item.id} title={item.name || t("storage.location")} description={item.path || item.status}>
            <Button variant="secondary" size="sm" disabled={!item.canOpen} onClick={() => void StorageService.OpenLocation(item.id)}>
              <FolderOpen className="size-3.5" /> {t("common.open")}
            </Button>
          </SettingRow>
        )) : (
          <div className="empty-state compact-empty-state">
            <div>
              <strong>{t("storage.noAdditional")}</strong>
              <span>{snapshot.runtime.phase === RuntimePhase.RuntimeRunning ? t("storage.noAdditionalHint") : t("storage.startServerHint")}</span>
            </div>
          </div>
        )}
      </SettingsSection>
    </>
  );
}

function LumenPanel({ snapshot, showToast }: { snapshot: DesktopSnapshot; showToast: (input: ToastInput) => string }) {
  const { t } = useTranslation();
  const lumen = snapshot.lumen;
  const [state, setState] = useState<ButtonState>("idle");
  const [actionError, setActionError] = useState<string | null>(null);
  const [selectedProfile, setSelectedProfile] = useState(lumen.profile || lumen.availableProfiles?.[0] || "");
  const [selectedPreset, setSelectedPreset] = useState(lumen.preset || lumen.availablePresets?.[0] || "basic");
  const [selectedCacheDir, setSelectedCacheDir] = useState(lumen.cacheDir || "");
  const [presetInfoOpen, setPresetInfoOpen] = useState(false);
  const [logLevel, setLogLevel] = useState("INFO");
  const [logs, setLogs] = useState<LumenLogEntry[]>([]);
  const [logsLoading, setLogsLoading] = useState(false);
  const [logsError, setLogsError] = useState<string | null>(null);
  const releaseReady = lumen.installerAvailable && lumen.processAvailable;

  const loadLogs = useCallback(async () => {
    if (lumen.processPhase !== LumenProcessPhase.LumenRunning || !lumen.control.connected) {
      setLogs([]);
      setLogsError(null);
      return;
    }
    setLogsLoading(true);
    try {
      setLogs((await LumenService.GetLogs(200, logLevel)) ?? []);
      setLogsError(null);
    } catch (reason: unknown) {
      setLogsError(errorMessage(reason));
    } finally {
      setLogsLoading(false);
    }
  }, [logLevel, lumen.control.connected, lumen.processPhase]);

  useEffect(() => {
    if (lumen.processPhase !== LumenProcessPhase.LumenRunning || !lumen.control.connected) {
      setLogs([]);
      setLogsError(null);
      return;
    }
    void loadLogs();
    const timer = window.setInterval(() => void loadLogs(), 5000);
    return () => window.clearInterval(timer);
  }, [loadLogs, lumen.control.connected, lumen.processPhase]);

  const invoke = async (action: "install" | "start" | "stop" | "restart" | "retry") => {
    setState("loading");
    setActionError(null);
    const requestID = `lumen-${crypto.randomUUID()}`;
    try {
      if (action === "install") await LumenService.Install(requestID, lumen.version, selectedProfile, selectedPreset, selectedCacheDir);
      else if (action === "start") await LumenService.Start(requestID, lumen.version);
      else if (action === "stop") await LumenService.Stop(requestID, lumen.version);
      else if (action === "restart") await LumenService.Restart(requestID, lumen.version);
      else await LumenService.RetryCleanup(requestID, lumen.version);
      setState("success");
      window.setTimeout(() => setState("idle"), 1200);
    } catch (reason: unknown) {
      const message = errorMessage(reason);
      setState("error");
      setActionError(message);
      showToast({ title: t("lumen.actionFailed"), description: message, status: "error" });
    }
  };

  const setupMutable = lumen.installPhase === LumenInstallPhase.LumenAbsent || lumen.installPhase === LumenInstallPhase.LumenInstallFailed;
  const canInstall = lumen.installerAvailable
    && Boolean(selectedProfile)
    && Boolean(selectedPreset)
    && Boolean(selectedCacheDir)
    && setupMutable;
  const canChooseSetup = setupMutable && state !== "loading";

  const chooseCacheDirectory = async () => {
    try {
      const path = await LumenService.PickCacheDirectory(t("lumen.chooseCacheDirectory", "Choose the Lumen model cache directory"));
      if (path) {
        setSelectedCacheDir(path);
        setActionError(null);
      }
    } catch (reason: unknown) {
      const message = errorMessage(reason);
      setActionError(message);
      showToast({ title: t("lumen.cachePickFailed", "Cache directory could not be selected"), description: message, status: "error" });
    }
  };

  const presetLabel = (preset: string) => {
    if (preset === "minimal") return t("lumen.presetNameMinimal", "Minimal");
    if (preset === "brave") return t("lumen.presetNameBrave", "Brave");
    if (preset === "basic") return t("lumen.presetNameBasic", "Basic");
    return preset;
  };

  const backendLabel = (profile: string) => {
    if (profile.endsWith("-metal")) return t("lumen.backendMetal", "Metal");
    if (profile.endsWith("-gpu")) return t("lumen.backendGPU", "GPU (WGPU)");
    return t("lumen.backendCPU", "CPU");
  };

  return (
    <>
      <PageHeading title={t("dock.lumen")} description={t("lumen.description")} />
      <SettingsSection title={t("lumen.availability")}>
        <SettingRow
          title="Lumen Hub"
          description={releaseReady ? t("lumen.releaseReady") : t("lumen.notIncluded")}
        >
          <AnimatedBadge status={releaseReady ? presentationStatus(lumen.presentation) : "neutral"} size="sm">
            {releaseReady ? lumen.presentation.label : t("onboarding.unavailable")}
          </AnimatedBadge>
        </SettingRow>
      </SettingsSection>

      {releaseReady ? (
        <SettingsSection title={t("lumen.service")}>
          <SettingRow
            title={
              <span className="lumen-preset-title">
                {t("lumen.preset", "Preset")}
                <Button
                  variant="ghost"
                  size="icon"
                  className="lumen-preset-info"
                  aria-label={t("lumen.presetInfo", "Show preset capabilities")}
                  title={t("lumen.presetInfo", "Show preset capabilities")}
                  onClick={() => setPresetInfoOpen(true)}
                >
                  <Info className="size-3.5" />
                </Button>
              </span>
            }
            description={t("lumen.presetDescription", "Choose which AI services and model sizes to install.")}
          >
            <Select value={selectedPreset} onValueChange={setSelectedPreset} disabled={!canChooseSetup} className="w-80 max-w-full">
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {(lumen.availablePresets ?? []).map((preset) => (
                  <SelectItem key={preset} value={preset}>{presetLabel(preset)}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </SettingRow>
          <SettingRow
            title={t("lumen.backend", "Backend")}
            description={t("lumen.backendDescription", "Backend determines which platform-specific Lumen Hub package is downloaded.")}
          >
            <Select value={selectedProfile} onValueChange={setSelectedProfile} disabled={!canChooseSetup} className="w-80 max-w-full">
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {(lumen.availableProfiles ?? []).map((profile) => (
                  <SelectItem key={profile} value={profile}>{backendLabel(profile)} · {profile}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </SettingRow>
          <SettingRow
            title={t("lumen.cacheDirectory", "Model cache")}
            description={selectedCacheDir || t("lumen.cacheDirectoryDescription", "Choose where Lumen stores downloaded models.")}
          >
            <Button variant="secondary" size="sm" disabled={!canChooseSetup} onClick={() => void chooseCacheDirectory()}>
              <FolderOpen className="size-3.5" /> {t("common.choose")}
            </Button>
          </SettingRow>
          <SettingRow title={t("lumen.installation")} description={selectedProfile ? `${t("lumen.profile")}: ${selectedProfile}` : t("lumen.noProfile")}>
            <StatefulButton
              variant="secondary"
              size="sm"
              state={state}
              disabled={!canInstall}
              loadingText={t("lumen.installing")}
              successText={t("lumen.installed")}
              onClick={() => void invoke("install")}
            >
              {t("lumen.install")}
            </StatefulButton>
          </SettingRow>
          <SettingRow title={t("lumen.processStatus")} description={`${t("lumen.desiredState")}: ${lumen.desiredState || "disabled"}.`}>
            <RowActions>
              <AnimatedBadge status={presentationStatus(lumen.presentation)} size="sm">{lumen.presentation.label}</AnimatedBadge>
              {lumen.capabilities.canStartLumen ? <Button variant="secondary" size="sm" onClick={() => void invoke("start")}>{t("common.start")}</Button> : null}
              {lumen.capabilities.canStopLumen || lumen.capabilities.canRetryCleanupLumen ? (
                <Button variant="secondary" size="sm" onClick={() => void invoke(lumen.capabilities.canRetryCleanupLumen ? "retry" : "stop")}>
                  {lumen.capabilities.canRetryCleanupLumen ? t("common.retryCleanup") : t("common.stop")}
                </Button>
              ) : null}
              {lumen.capabilities.canRestartLumen ? <Button variant="secondary" size="sm" onClick={() => void invoke("restart")}>{t("common.restart")}</Button> : null}
            </RowActions>
          </SettingRow>
        </SettingsSection>
      ) : null}

      {releaseReady && lumen.installPhase === LumenInstallPhase.LumenInstalled ? (
        <LumenControlPanel lumen={lumen} />
      ) : null}

      {releaseReady && lumen.installPhase === LumenInstallPhase.LumenInstalled ? (
        <SettingsSection
          title={t("lumen.logs", "Control logs")}
          description={t("lumen.logsDescription", "Structured logs from Lumen Control. The view refreshes every five seconds while Hub is running.")}
        >
          <div className="lumen-log-toolbar">
            <Select value={logLevel} onValueChange={setLogLevel} disabled={!lumen.control.connected} className="compact-select">
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {(["TRACE", "DEBUG", "INFO", "WARN", "ERROR"] as const).map((level) => (
                  <SelectItem key={level} value={level}>{level}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button variant="secondary" size="sm" disabled={!lumen.control.connected || logsLoading} onClick={() => void loadLogs()}>
              <RefreshCw className={logsLoading ? "size-3.5 animate-spin" : "size-3.5"} />
              {t("common.refresh", "Refresh")}
            </Button>
          </div>
          {logsError ? <InlineNotice tone="danger" title={t("lumen.logsUnavailable", "Logs unavailable")}>{logsError}</InlineNotice> : null}
          <LumenLogViewer logs={logs} connected={lumen.control.connected} loading={logsLoading} />
        </SettingsSection>
      ) : null}

      {actionError ? <ActionNotice component={t("dock.lumen")} message={actionError} /> : null}
      <PresetInfoModal open={presetInfoOpen} onClose={() => setPresetInfoOpen(false)} />
    </>
  );
}

const lumenPhaseOrder = [
  LumenControlPhase.LumenControlStarting,
  LumenControlPhase.LumenControlDownloading,
  LumenControlPhase.LumenControlLoading,
  LumenControlPhase.LumenControlWarmup,
  LumenControlPhase.LumenControlReady,
] as const;

function LumenControlPanel({ lumen }: { lumen: LumenSnapshot }) {
  const { t } = useTranslation();
  const control = lumen.control;
  const currentIndex = lumenPhaseOrder.indexOf(control.phase as (typeof lumenPhaseOrder)[number]);
  const downloadPercent = control.download?.bytesTotal
    ? Math.min(100, Math.max(0, (control.download.bytesDone / control.download.bytesTotal) * 100))
    : null;

  return (
    <SettingsSection
      title={t("lumen.controlStatus", "Control status")}
      description={t("lumen.controlDescription", "Live lifecycle and model state reported by lumen.control.v1.")}
    >
      <div className="lumen-control-summary">
        <div>
          <span className="lumen-control-kicker">{t("lumen.inference", "Inference")}</span>
          <strong>{control.inferenceReady ? t("lumen.ready", "Ready") : control.connected ? t("lumen.preparing", "Preparing") : t("lumen.disconnected", "Disconnected")}</strong>
        </div>
        <AnimatedBadge status={controlPhaseStatus(control.phase)} size="sm">{controlPhaseLabel(control.phase, t)}</AnimatedBadge>
        <dl className="lumen-control-meta">
          <div><dt>{t("lumen.version", "Version")}</dt><dd>{control.version || "—"}</dd></div>
          <div><dt>{t("lumen.backend", "Backend")}</dt><dd>{control.backend || "—"}</dd></div>
          <div><dt>{t("lumen.sequence", "Sequence")}</dt><dd>{control.sequence || "—"}</dd></div>
        </dl>
      </div>

      {control.connected ? (
        <ol className="lumen-phase-track" aria-label={t("lumen.lifecycle", "Lumen startup lifecycle")}>
          {lumenPhaseOrder.map((phase, index) => (
            <li key={phase} className={index < currentIndex ? "complete" : index === currentIndex ? "current" : undefined} aria-current={index === currentIndex ? "step" : undefined}>
              <span aria-hidden />
              <small>{controlPhaseLabel(phase, t)}</small>
            </li>
          ))}
        </ol>
      ) : (
        <InlineNotice title={t("lumen.controlWaiting", "Waiting for Control")}>{t("lumen.controlWaitingDescription", "Start Lumen Hub to connect to its local control plane.")}</InlineNotice>
      )}

      {control.download ? (
        <div className="lumen-download" aria-label={t("lumen.downloadProgress", "Model download progress")}>
          <div className="lumen-download-heading">
            <div><strong>{control.download.model || t("lumen.model", "Model")}</strong><span>{control.download.file || t("lumen.preparingDownload", "Preparing download")}</span></div>
            <span className="tabular-value">{downloadPercent === null ? t("lumen.downloading", "Downloading") : `${downloadPercent.toFixed(1)}%`}</span>
          </div>
          <progress value={control.download.bytesDone} max={control.download.bytesTotal || undefined} />
          <div className="lumen-download-meta">
            <span>{formatBytes(control.download.bytesDone)}{control.download.bytesTotal ? ` / ${formatBytes(control.download.bytesTotal)}` : ""}</span>
            <span>{t("lumen.filesProgress", "Files {{done}} / {{total}}", { done: control.download.filesDone, total: control.download.filesTotal })}</span>
          </div>
        </div>
      ) : null}

      {control.error ? <InlineNotice tone="danger" title={t("lumen.controlFailed", "Lumen startup failed")}>{control.error.message}</InlineNotice> : null}

      <div className="lumen-services">
        <div className="lumen-subheading"><h3>{t("lumen.services", "AI services")}</h3><span>{t("lumen.servicesReported", "{{count}} reported", { count: control.services?.length ?? 0 })}</span></div>
        {control.services?.length ? control.services.map((service) => (
          <div className="lumen-service-row" key={service.service}>
            <div><strong>{serviceDisplayName(service.service)}</strong>{service.error ? <span>{service.error.message}</span> : null}</div>
            <AnimatedBadge status={controlPhaseStatus(service.phase)} size="sm">{controlPhaseLabel(service.phase, t)}</AnimatedBadge>
          </div>
        )) : <p className="lumen-empty-copy">{control.connected ? t("lumen.servicesPending", "Service states will appear after model construction.") : t("lumen.servicesOffline", "No service state is available while Hub is stopped.")}</p>}
      </div>
    </SettingsSection>
  );
}

function presetNameLabel(id: PresetInfo["id"], t: ReturnType<typeof useTranslation>["t"]) {
  if (id === "minimal") return t("lumen.presetNameMinimal", "Minimal");
  if (id === "brave") return t("lumen.presetNameBrave", "Brave");
  return t("lumen.presetNameBasic", "Basic");
}

function presetDescriptionLabel(id: PresetInfo["id"], t: ReturnType<typeof useTranslation>["t"]) {
  if (id === "minimal") return t("lumen.presetDescriptionMinimal", "Core semantic search and people recognition.");
  if (id === "brave") return t("lumen.presetDescriptionBrave", "Higher-capacity semantic and species recognition models.");
  return t("lumen.presetDescriptionBasic", "The complete everyday photo analysis set.");
}

function capabilityLabel(id: PresetCapability["id"], t: ReturnType<typeof useTranslation>["t"]) {
  if (id === "semantic") return t("lumen.capabilitySemantic", "Image semantic analysis");
  if (id === "ocr") return t("lumen.capabilityOCR", "OCR text recognition");
  if (id === "people") return t("lumen.capabilityPeople", "People recognition");
  return t("lumen.capabilitySpecies", "BioCLIP species recognition");
}

function PresetInfoModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();

  useEffect(() => {
    if (!open) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [onClose, open]);

  if (typeof document === "undefined") return null;
  return createPortal(
    <AnimatePresence>
      {open ? (
        <motion.div
          className="lumen-modal-root"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.16 }}
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) onClose();
          }}
        >
          <motion.section
            className="lumen-preset-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="lumen-preset-modal-title"
            initial={{ opacity: 0, y: 12, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 8, scale: 0.98 }}
            transition={{ type: "spring", stiffness: 360, damping: 30, mass: 0.75 }}
            onMouseDown={(event) => event.stopPropagation()}
          >
            <header className="lumen-preset-modal-header">
              <div>
                <p className="lumen-control-kicker">{t("lumen.presetDetailsEyebrow", "Lumen setup")}</p>
                <h2 id="lumen-preset-modal-title">{t("lumen.presetDetails", "Preset capabilities")}</h2>
                <p>{t("lumen.presetDetailsDescription", "Compare the AI services, models, and datasets included with each preset.")}</p>
              </div>
              <Button
                variant="ghost"
                size="icon"
                aria-label={t("common.close", "Close")}
                title={t("common.close", "Close")}
                onClick={onClose}
              >
                <X className="size-4" />
              </Button>
            </header>
            <div className="lumen-preset-grid">
              {presetCatalog.map((preset) => (
                <article className="lumen-preset-card" key={preset.id}>
                  <div className="lumen-preset-card-heading">
                    <div>
                      <h3>{presetNameLabel(preset.id, t)}</h3>
                      <p>{presetDescriptionLabel(preset.id, t)}</p>
                    </div>
                    <span className="lumen-preset-code">{preset.id}</span>
                  </div>
                  <div className="lumen-capability-list">
                    {preset.capabilities.map((capability) => (
                      <div className="lumen-capability" key={capability.id}>
                        <strong>{capabilityLabel(capability.id, t)}</strong>
                        <span><em>{t("lumen.model", "Model")}</em>{capability.model}</span>
                        <span><em>{t("lumen.dataset", "Dataset")}</em>{capability.dataset || t("lumen.datasetDefault", "Upstream model default")}</span>
                      </div>
                    ))}
                  </div>
                </article>
              ))}
            </div>
          </motion.section>
        </motion.div>
      ) : null}
    </AnimatePresence>,
    document.body,
  );
}

function LumenLogViewer({ logs, connected, loading }: { logs: LumenLogEntry[]; connected: boolean; loading: boolean }) {
  const { t } = useTranslation();
  if (!connected) return <p className="lumen-empty-copy lumen-log-empty">{t("lumen.logsOffline", "Start Lumen Hub to read Control logs.")}</p>;
  if (!logs.length && loading) return <p className="lumen-empty-copy lumen-log-empty">{t("lumen.logsLoading", "Reading Control logs…")}</p>;
  if (!logs.length) return <p className="lumen-empty-copy lumen-log-empty">{t("lumen.logsEmpty", "No log entries match this level.")}</p>;
  return (
    <div className="lumen-log-view" role="log" aria-label={t("lumen.logs", "Control logs")}>
      {logs.map((entry, index) => (
        <div className="lumen-log-line" key={`${entry.timeUnixMS}-${index}`} data-level={entry.level}>
          <time>{formatLogTime(entry.timeUnixMS)}</time>
          <span className="lumen-log-level">{entry.level}</span>
          <span className="lumen-log-target">{entry.target}</span>
          <span className="lumen-log-message">{entry.message}{formatLogFields(entry.fields)}</span>
        </div>
      ))}
    </div>
  );
}

function controlPhaseStatus(phase: LumenControlPhase): AnimatedBadgeStatus {
  if (phase === LumenControlPhase.LumenControlReady) return "success";
  if (phase === LumenControlPhase.LumenControlFailed) return "danger";
  if (phase === LumenControlPhase.LumenControlUnspecified || phase === LumenControlPhase.LumenControlStopping) return "neutral";
  return "warning";
}

function controlPhaseLabel(phase: LumenControlPhase, t: ReturnType<typeof useTranslation>["t"]) {
  if (phase === LumenControlPhase.LumenControlStarting) return t("lumen.phaseStarting", "Starting");
  if (phase === LumenControlPhase.LumenControlDownloading) return t("lumen.phaseDownloading", "Downloading");
  if (phase === LumenControlPhase.LumenControlLoading) return t("lumen.phaseLoading", "Loading");
  if (phase === LumenControlPhase.LumenControlWarmup) return t("lumen.phaseWarmup", "Warmup");
  if (phase === LumenControlPhase.LumenControlReady) return t("lumen.phaseReady", "Ready");
  if (phase === LumenControlPhase.LumenControlFailed) return t("lumen.phaseFailed", "Failed");
  if (phase === LumenControlPhase.LumenControlStopping) return t("lumen.phaseStopping", "Stopping");
  return t("lumen.phaseUnavailable", "Unavailable");
}

function serviceDisplayName(service: string) {
  const names: Record<string, string> = { siglip: "SigLIP", face: "InsightFace", insightface: "InsightFace", ocr: "PP-OCR", ppocr: "PP-OCR", bioclip: "BioCLIP" };
  return names[service] || service;
}

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const index = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
  return `${(bytes / 1024 ** index).toFixed(index > 1 ? 1 : 0)} ${units[index]}`;
}

function formatLogTime(unixMS: number) {
  if (!unixMS) return "--:--:--";
  return new Date(unixMS).toLocaleTimeString([], { hour12: false, hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function formatLogFields(fields: LumenLogEntry["fields"]) {
  const entries = Object.entries(fields ?? {});
  return entries.length ? ` · ${entries.map(([key, value]) => `${key}=${value}`).join(" ")}` : "";
}

function UpdatesPanel({
  snapshot,
  draft,
  showToast,
}: {
  snapshot: DesktopSnapshot;
  draft: SettingsDraftController;
  showToast: (input: ToastInput) => string;
}) {
  const { t } = useTranslation();
  const update = snapshot.update;
  const [state, setState] = useState<ButtonState>("idle");

  const invoke = async (action: "check" | "download" | "apply") => {
    setState("loading");
    const requestID = `update-${crypto.randomUUID()}`;
    try {
      if (action === "check") await UpdateService.Check(requestID, update.version);
      else if (action === "download") await UpdateService.Download(requestID, update.version);
      else await UpdateService.RestartAndApply(requestID, update.version);
      setState("success");
      window.setTimeout(() => setState("idle"), 1200);
    } catch (reason: unknown) {
      setState("error");
      showToast({ title: t("updates.actionFailed"), description: errorMessage(reason), status: "error" });
    }
  };

  return (
    <>
      <PageHeading title={t("dock.updates")} description={t("updates.description")} />
      <SettingsSection title={t("dock.updates")}>
        <SettingRow title={t("updates.currentVersion")} description={t("updates.currentVersionDescription")}>
          <span className="setting-value tabular-value">{update.currentVersion || t("common.unknown")}</span>
        </SettingRow>
        <SettingRow title={t("updates.channel")} description={t("updates.channelDescription")}>
          {draft.preferences ? (
            <Select
              value={draft.preferences.updateChannel}
              onValueChange={(value) => draft.updatePreference("updateChannel", value)}
              disabled={draft.phase === "saving"}
              className="compact-select"
            >
              <SelectTrigger><SelectValue placeholder={t("updates.channel")} /></SelectTrigger>
              <SelectContent>
                <SelectItem value="stable">{t("updates.stable")}</SelectItem>
                <SelectItem value="beta">{t("updates.beta")}</SelectItem>
              </SelectContent>
            </Select>
          ) : <span className="setting-value">{t("common.loadingEllipsis")}</span>}
        </SettingRow>
        <SettingRow
          title={t("updates.checkTitle")}
          description={update.providerAvailable ? t("updates.checkDescription") : t("updates.noProvider")}
        >
          <StatefulButton
            variant="secondary"
            size="sm"
            state={state}
            disabled={!update.providerAvailable}
            loadingText={t("updates.checking")}
            successText={t("updates.checked")}
            icon={<RefreshCw className="size-3.5" />}
            onClick={() => void invoke("check")}
          >
            {update.providerAvailable ? t("updates.checkNow") : t("onboarding.unavailable")}
          </StatefulButton>
        </SettingRow>
        {update.availableVersion ? (
          <SettingRow title={t("updates.available")} description={t("updates.availableDescription", { version: update.availableVersion })}>
            <RowActions>
              <Button variant="secondary" size="sm" disabled={state === "loading"} onClick={() => void invoke("download")}>{t("updates.download")}</Button>
              {update.canApply ? <Button size="sm" disabled={state === "loading"} onClick={() => void invoke("apply")}>{t("updates.restartInstall")}</Button> : null}
            </RowActions>
          </SettingRow>
        ) : null}
      </SettingsSection>
      {update.error?.code ? (
        <ActionNotice
          component={t("dock.updates")}
          message={update.error.message}
          actionLabel={update.providerAvailable ? t("common.retry") : undefined}
          onAction={update.providerAvailable ? () => void invoke("check") : undefined}
        />
      ) : null}
    </>
  );
}

function LanguageSelect({
  preferences,
  update,
  disabled,
}: {
  preferences: DesktopPreferences;
  update: SettingsDraftController["updatePreference"];
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Select value={preferences.locale} onValueChange={(value) => update("locale", value)} disabled={disabled} className="compact-select">
      <SelectTrigger><SelectValue placeholder={t("general.language")} /></SelectTrigger>
      <SelectContent>
        <SelectItem value="en">English</SelectItem>
        <SelectItem value="zh-CN">简体中文</SelectItem>
      </SelectContent>
    </Select>
  );
}

function ThemeStatusControl({
  preferences,
  update,
  disabled,
}: {
  preferences: DesktopPreferences;
  update: SettingsDraftController["updatePreference"];
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  const system = !preferences.theme || preferences.theme === "system";
  const currentLabel = system ? t("general.themeSystem") : preferences.theme === "dark" ? t("general.themeDark") : t("general.themeLight");
  return (
    <div className="flex items-center justify-end gap-2">
      <span className="setting-value">{currentLabel}</span>
      {!system ? (
        <Button
          variant="ghost"
          size="sm"
          disabled={disabled}
          onClick={() => update("theme", "system")}
        >
          {t("general.themeSystem")}
        </Button>
      ) : null}
    </div>
  );
}

function RegionSelect({
  preferences,
  update,
  disabled,
}: {
  preferences: DesktopPreferences;
  update: SettingsDraftController["updatePreference"];
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Select value={preferences.region} onValueChange={(value) => update("region", value)} disabled={disabled} className="compact-select">
      <SelectTrigger><SelectValue placeholder={t("general.region")} /></SelectTrigger>
      <SelectContent>
        <SelectItem value="global">{t("region.global")}</SelectItem>
        <SelectItem value="china">{t("region.china")}</SelectItem>
      </SelectContent>
    </Select>
  );
}

function NetworkSelect({
  settings,
  update,
  disabled,
}: {
  settings: RuntimeConfigSettings;
  update: SettingsDraftController["updateRuntime"];
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Select value={settings.networkMode} onValueChange={(value) => update("networkMode", value)} disabled={disabled} className="compact-select">
      <SelectTrigger><SelectValue placeholder={t("network.access")} /></SelectTrigger>
      <SelectContent>
        <SelectItem value="local">{t("network.local")}</SelectItem>
        <SelectItem value="lan">{t("network.lan")}</SelectItem>
      </SelectContent>
    </Select>
  );
}

function ActionNotice({
  component,
  message,
  actionLabel,
  onAction,
}: {
  component: string;
  message: string;
  actionLabel?: string;
  onAction?: () => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="action-notice" role="alert">
      <div>
        <strong>{t("actionNotice.needsAttention", { component })}</strong>
        <span>{message}</span>
      </div>
      {actionLabel && onAction ? <Button variant="secondary" size="sm" onClick={onAction}>{actionLabel}</Button> : null}
    </div>
  );
}

function SettingsLoading({ label }: { label: string }) {
  const { t } = useTranslation();
  return (
    <div className="loading-panel">
      <Loader variant="ascii-braille" size={20} label={label} />
      <span>{t("common.loading", { label })}</span>
    </div>
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
  const [state, setState] = useState<ButtonState>("idle");
  const restore = async () => {
    setState("loading");
    try {
      await RuntimeService.RestoreLastKnownGood(`recovery-${crypto.randomUUID()}`, snapshot.runtime.version);
      setState("success");
      navigate("/server");
    } catch (reason: unknown) {
      setState("error");
      showToast({ title: t("recovery.failed"), description: errorMessage(reason), status: "error" });
    }
  };
  return (
    <div className="recovery-page">
      <div className="recovery-icon"><RotateCcw className="size-5" /></div>
      <PageHeading title={t("recovery.title")} description={snapshot.host.recovery?.message || t("recovery.description")} />
      <div className="recovery-actions">
        <StatefulButton state={state} loadingText={t("recovery.restoring")} successText={t("recovery.restored")} onClick={() => void restore()}>
          {t("recovery.restore")}
        </StatefulButton>
        <Button variant="secondary" onClick={() => navigate("/server")}>{t("recovery.reviewSettings")}</Button>
      </div>
    </div>
  );
}

function RecoveryFallback({ message }: { message: string }) {
  const { t } = useTranslation();
  return (
    <main className="boot-screen recovery-fallback dark">
      <div className="recovery-icon"><RotateCcw className="size-5" /></div>
      <h1>{t("recovery.fallbackTitle")}</h1>
      <p>{message}</p>
      <Button onClick={() => window.location.reload()}>{t("recovery.tryAgain")}</Button>
    </main>
  );
}

function RowActions({ children }: { children: ReactNode }) {
  return <div className="row-actions">{children}</div>;
}

function presentationStatus(presentation: ProcessPresentation): AnimatedBadgeStatus {
  if (presentation.color === "green") return "success";
  if (presentation.color === "yellow") return "warning";
  if (presentation.color === "red") return "danger";
  return "neutral";
}

function normalizeRoute(route: string): MainRoute {
  if (route === "/runtime") return "/server";
  if (route === "/overview" || route === "/settings" || route === "/diagnostics") return "/general";
  const found = dockRoutes.find((item) => item.route === route);
  return found?.route ?? "/general";
}

function networkLabel(mode: string, t: (key: string) => string) {
  if (mode === "lan") return t("network.lan");
  if (mode === "custom") return t("network.custom");
  return t("network.local");
}

async function openProduct(showToast: (input: ToastInput) => string, t: (key: string) => string) {
  try {
    await DesktopService.OpenProduct();
  } catch (reason: unknown) {
    showToast({ title: t("toast.openFailed"), description: errorMessage(reason), status: "error" });
  }
}
