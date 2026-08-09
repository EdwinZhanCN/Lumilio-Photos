import {
  ArrowUpRight,
  BrainCircuit,
  Check,
  Download,
  FolderOpen,
  HardDrive,
  RefreshCw,
  RotateCcw,
  Server,
  Settings2,
  X,
} from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
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
  type HostActionTicket,
  type LumenLogEntry,
  type LumenSnapshot,
  type OperationReceipt,
  type OperationSnapshot,
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

interface TrackedOperationOptions {
  onSucceeded?: () => void;
  onFailed?: (message: string) => void;
}

/** Desktop service methods acknowledge an operation before their actor work is
 * complete. This hook makes the shared operation registry, rather than the RPC
 * return, authoritative for button success and failure. */
function useTrackedOperation(
  operations: OperationSnapshot[] | null | undefined,
  options: TrackedOperationOptions = {},
) {
  const [operationID, setOperationID] = useState<string | null>(null);
  const [state, setState] = useState<ButtonState>("idle");
  const [error, setError] = useState<string | null>(null);
  const optionsRef = useRef(options);
  const resetTimer = useRef<number | null>(null);
  optionsRef.current = options;

  useEffect(
    () => () => {
      if (resetTimer.current !== null) window.clearTimeout(resetTimer.current);
    },
    [],
  );

  useEffect(() => {
    if (!operationID) return;
    const operation = operations?.find((item) => item.operationID === operationID);
    if (!operation) return;
    if (operation.state === "accepted" || operation.state === "running") {
      setState("loading");
      return;
    }
    if (operation.state === "succeeded") {
      setOperationID(null);
      setError(null);
      setState("success");
      optionsRef.current.onSucceeded?.();
      resetTimer.current = window.setTimeout(() => {
        setState("idle");
        resetTimer.current = null;
      }, 1200);
      return;
    }
    if (operation.state === "failed" || operation.state === "cancelled") {
      const message = operation.error?.message || `Desktop operation ${operation.state}.`;
      setOperationID(null);
      setError(message);
      setState("error");
      optionsRef.current.onFailed?.(message);
    }
  }, [operationID, operations]);

  const begin = () => {
    if (resetTimer.current !== null) {
      window.clearTimeout(resetTimer.current);
      resetTimer.current = null;
    }
    setOperationID(null);
    setError(null);
    setState("loading");
  };

  const track = (receipt: OperationReceipt) => {
    setOperationID(receipt.operationID);
  };

  const reject = (reason: unknown) => {
    const message = errorMessage(reason);
    setOperationID(null);
    setError(message);
    setState("error");
    optionsRef.current.onFailed?.(message);
  };

  const reset = () => {
    setOperationID(null);
    setError(null);
    setState("idle");
  };

  return { state, error, begin, track, reject, reset };
}

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
        <img
          src={appIconURL}
          alt={t("product.compactName", "Lumilio Photos")}
          className="bootstrap-icon"
        />
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
    <section
      className="bootstrap"
      aria-label={t("onboarding.ariaLabel", "Lumilio Photos Desktop setup")}
    >
      <header className="bootstrap-header">
        <div className="bootstrap-brand">
          <img src={appIconURL} alt="" className="bootstrap-icon" />
          <span>{t("product.compactName", "Lumilio Photos")}</span>
        </div>
        <span>{t("onboarding.step", { current: step + 1, total })}</span>
      </header>

      <div className="bootstrap-progress" aria-hidden>
        {Array.from({ length: total }, (_, index) => (
          <i key={index} className={index <= step ? "active" : ""} />
        ))}
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
              <PageHeading
                title={t("onboarding.generalTitle")}
                description={t("onboarding.generalDescription")}
              />
              <SettingsSection title={t("onboarding.desktop")}>
                <SettingRow
                  title={t("onboarding.language")}
                  description={t("onboarding.languageDescription")}
                >
                  <LanguageSelect
                    preferences={draft.preferences}
                    update={draft.updatePreference}
                    disabled={busy}
                  />
                </SettingRow>
                <SettingRow
                  title={t("onboarding.region")}
                  description={t("onboarding.regionDescription")}
                >
                  <RegionSelect
                    preferences={draft.preferences}
                    update={draft.updatePreference}
                    disabled={busy}
                  />
                </SettingRow>
              </SettingsSection>
            </>
          ) : step === 1 ? (
            <>
              <PageHeading
                title={t("onboarding.storageTitle")}
                description={t("onboarding.storageDescription")}
              />
              <SettingsSection title={t("onboarding.storage")}>
                <SettingRow
                  title={t("onboarding.storageLocation")}
                  description={draft.runtime.storagePath}
                >
                  <Button
                    variant="secondary"
                    disabled={busy}
                    onClick={() => void draft.chooseDefaultStorage()}
                  >
                    <FolderOpen className="size-4" /> {t("common.choose")}
                  </Button>
                </SettingRow>
              </SettingsSection>
            </>
          ) : step === 2 ? (
            <>
              <PageHeading
                title={t("onboarding.networkTitle")}
                description={t("onboarding.networkDescription")}
              />
              <SettingsSection title={t("onboarding.server")}>
                <SettingRow
                  title={t("onboarding.networkAccess")}
                  description={t("onboarding.networkRecommended")}
                >
                  <NetworkSelect
                    settings={draft.runtime}
                    update={draft.updateRuntime}
                    disabled={busy}
                  />
                </SettingRow>
              </SettingsSection>
            </>
          ) : (
            <>
              <PageHeading
                title={t("onboarding.readyTitle")}
                description={t("onboarding.readyDescription")}
              />
              <SettingsSection title={t("onboarding.summary")}>
                <SettingRow
                  title={t("onboarding.storageLocation")}
                  description={draft.runtime.storagePath}
                >
                  <span className="setting-value">{t("onboarding.selected")}</span>
                </SettingRow>
                <SettingRow
                  title={t("onboarding.networkAccess")}
                  description={networkLabel(draft.runtime.networkMode, t)}
                >
                  <span className="setting-value">{t("onboarding.configured")}</span>
                </SettingRow>
                <SettingRow
                  title={t("onboarding.lumenAI")}
                  description={
                    snapshot.lumen.installerAvailable && snapshot.lumen.processAvailable
                      ? t("onboarding.lumenAvailable")
                      : t("onboarding.lumenNotIncluded")
                  }
                >
                  <AnimatedBadge status="neutral" size="sm">
                    {snapshot.lumen.installerAvailable && snapshot.lumen.processAvailable
                      ? t("onboarding.optional")
                      : t("onboarding.unavailable")}
                  </AnimatedBadge>
                </SettingRow>
              </SettingsSection>
              {draft.phase === "error" && draft.error ? (
                <InlineNotice tone="danger" title={t("onboarding.setupFailed")}>
                  {draft.error}
                </InlineNotice>
              ) : null}
            </>
          )}
        </motion.div>
      </AnimatePresence>

      <footer className="bootstrap-actions">
        <Button
          variant="ghost"
          disabled={step === 0 || busy}
          onClick={() => setStep((current) => current - 1)}
        >
          {t("common.back")}
        </Button>
        {step < total - 1 ? (
          <Button
            disabled={busy || (step === 1 && !draft.runtime.storagePath)}
            onClick={() => setStep((current) => current + 1)}
          >
            {t("common.continue")}
          </Button>
        ) : (
          <Button
            disabled={busy || draft.validation?.valid === false}
            onClick={() => void finish()}
          >
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
          <LanguageSelect
            preferences={draft.preferences}
            update={draft.updatePreference}
            disabled={busy}
          />
        </SettingRow>
        <SettingRow title={t("general.region")} description={t("general.regionDescription")}>
          <RegionSelect
            preferences={draft.preferences}
            update={draft.updatePreference}
            disabled={busy}
          />
        </SettingRow>
        <SettingRow title={t("general.theme")} description={t("general.themeDescription")}>
          <ThemeStatusControl
            preferences={draft.preferences}
            update={draft.updatePreference}
            disabled={busy}
          />
        </SettingRow>
        <SettingRow
          title={t("general.openOnLaunch")}
          description={t("general.openOnLaunchDescription")}
        >
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
  const operation = useTrackedOperation(snapshot.operations, {
    onFailed: (message) =>
      showToast({ title: t("server.actionFailed"), description: message, status: "error" }),
  });
  const actionState = operation.state;
  const actionError = operation.error;

  const invoke = async (action: "start" | "stop" | "restart" | "retry") => {
    operation.begin();
    const requestID = `runtime-${crypto.randomUUID()}`;
    try {
      let receipt: OperationReceipt;
      if (action === "start") receipt = await RuntimeService.Start(requestID, runtime.version);
      else if (action === "stop") receipt = await RuntimeService.Stop(requestID, runtime.version);
      else if (action === "restart")
        receipt = await RuntimeService.Restart(requestID, runtime.version);
      else receipt = await RuntimeService.RetryCleanup(requestID, runtime.version);
      operation.track(receipt);
    } catch (reason: unknown) {
      operation.reject(reason);
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
              <Button
                variant="secondary"
                size="sm"
                disabled={actionState === "loading"}
                onClick={() => void invoke("start")}
              >
                {t("common.start")}
              </Button>
            ) : null}
            {runtime.capabilities.canStopRuntime || runtime.capabilities.canRetryCleanupRuntime ? (
              <Button
                variant="secondary"
                size="sm"
                disabled={actionState === "loading"}
                onClick={() =>
                  void invoke(runtime.capabilities.canRetryCleanupRuntime ? "retry" : "stop")
                }
              >
                {runtime.capabilities.canRetryCleanupRuntime
                  ? t("common.retryCleanup")
                  : t("common.stop")}
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
        <ActionNotice component={t("dock.server")} message={actionError} />
      ) : runtime.phase === RuntimePhase.RuntimeFailed ? (
        <ActionNotice
          component={t("dock.server")}
          message={t("server.failedMessage")}
          actionLabel={
            runtime.capabilities.canRetryCleanupRuntime ? t("common.retryCleanup") : undefined
          }
          onAction={
            runtime.capabilities.canRetryCleanupRuntime ? () => void invoke("retry") : undefined
          }
        />
      ) : null}

      {draft.runtime ? (
        <RuntimeConfigWorkspace
          settings={draft.runtime}
          disabled={draft.phase === "preparing" || draft.phase === "saving"}
          updateSetting={draft.updateRuntime}
          showToast={showToast}
        />
      ) : (
        <SettingsLoading label={t("server.loadingSettings")} />
      )}
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
  const [hostActions, setHostActions] = useState<HostActionTicket[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [hostActionError, setHostActionError] = useState<string | null>(null);
  const [decliningActionID, setDecliningActionID] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoadError(null);
      setItems((await StorageService.ListShortcuts()) || []);
    } catch (reason: unknown) {
      setLoadError(errorMessage(reason));
    }
  }, []);
  const refreshHostActions = useCallback(async () => {
    try {
      setHostActionError(null);
      setHostActions((await StorageService.ListHostActions()) || []);
    } catch (reason: unknown) {
      setHostActionError(errorMessage(reason));
    }
  }, []);
  const hostOperation = useTrackedOperation(snapshot.operations, {
    onSucceeded: () => {
      void refresh();
      void refreshHostActions();
    },
    onFailed: (message) =>
      showToast({
        title: t("storage.hostActionFailed", "Request could not be completed"),
        description: message,
        status: "error",
      }),
  });

  useEffect(() => {
    void refresh();
    void refreshHostActions();
    if (snapshot.runtime.phase !== RuntimePhase.RuntimeRunning) return undefined;
    const timer = window.setInterval(() => void refreshHostActions(), 3000);
    return () => window.clearInterval(timer);
  }, [refresh, refreshHostActions, snapshot.storage.version, snapshot.runtime.phase]);

  const approveHostAction = async (action: HostActionTicket) => {
    hostOperation.begin();
    try {
      const receipt = await StorageService.ApproveHostAction(
        `host-action-${crypto.randomUUID()}`,
        snapshot.storage.version,
        action.id,
        action.nonce,
      );
      hostOperation.track(receipt);
    } catch (reason: unknown) {
      hostOperation.reject(reason);
    }
  };

  const declineHostAction = async (action: HostActionTicket) => {
    setDecliningActionID(action.id);
    try {
      await StorageService.DeclineHostAction(action.id);
      await refreshHostActions();
    } catch (reason: unknown) {
      const message = errorMessage(reason);
      setHostActionError(message);
      showToast({
        title: t("storage.hostActionDeclineFailed", "Request could not be declined"),
        description: message,
        status: "error",
      });
    } finally {
      setDecliningActionID(null);
    }
  };

  const configuredPath = draft.runtime?.storagePath || "";
  const defaultLocation =
    items.find((item) => item.kind === "default") ||
    items.find((item) => configuredPath !== "" && item.path === configuredPath);
  const additional = items.filter((item) => item.id !== defaultLocation?.id);

  return (
    <>
      <PageHeading title={t("storage.title")} description={t("storage.description")} />

      {loadError ? (
        <ActionNotice
          component={t("dock.storage")}
          message={loadError}
          actionLabel={t("common.retry")}
          onAction={() => void refresh()}
        />
      ) : null}

      <SettingsSection title={t("storage.webRequests", "Requests from Web")}>
        {hostActionError ? (
          <ActionNotice
            component={t("storage.webRequests", "Requests from Web")}
            message={hostActionError}
            actionLabel={t("common.retry")}
            onAction={() => void refreshHostActions()}
          />
        ) : null}
        {hostActions.length ? (
          hostActions.map((action) => (
            <SettingRow
              key={action.id}
              title={hostActionLabel(action.kind, t)}
              description={t(
                "storage.hostActionRequestedBy",
                "{{purpose}} · Requested by {{actor}}",
                {
                  purpose:
                    action.purpose ||
                    t(
                      "storage.hostActionDescription",
                      "A signed-in administrator requested access to a folder on this computer.",
                    ),
                  actor: action.actor,
                },
              )}
            >
              {action.status === "needs_decision" && action.riskWarnings?.length ? (
                <div className="rounded-md border border-warning/35 bg-warning/10 px-3 py-2 text-xs text-warning-content">
                  <strong>
                    {t(
                      "storage.hostActionRiskTitle",
                      "Confirm storage risks before continuing",
                    )}
                  </strong>
                  <div className="mt-1">
                    {action.riskWarnings.map((warning) => hostActionRiskLabel(warning, t)).join(" · ")}
                  </div>
                </div>
              ) : null}
              <RowActions>
                <span className="setting-value">
                  {t("storage.hostActionExpires", "Expires {{time}}", {
                    time: new Date(action.expiresAt).toLocaleTimeString(),
                  })}
                </span>
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={decliningActionID === action.id || hostOperation.state === "loading"}
                  onClick={() => void declineHostAction(action)}
                >
                  <X className="size-3.5" /> {t("common.decline", "Decline")}
                </Button>
                <StatefulButton
                  variant="secondary"
                  size="sm"
                  state={hostOperation.state}
                  loadingText={
                    action.status === "needs_decision"
                      ? t("storage.hostActionConfirmingRisk", "Confirming risks")
                      : t("storage.hostActionChoosing", "Choosing folder")
                  }
                  successText={t("storage.hostActionApproved", "Approved")}
                  icon={<FolderOpen className="size-3.5" />}
                  disabled={decliningActionID !== null}
                  onClick={() => void approveHostAction(action)}
                >
                  {action.status === "needs_decision"
                    ? t("storage.confirmRiskAndContinue", "Confirm risks and continue")
                    : t("storage.reviewAndChoose", "Review and choose folder")}
                </StatefulButton>
              </RowActions>
            </SettingRow>
          ))
        ) : (
          <div className="empty-state compact-empty-state">
            <div>
              <strong>{t("storage.noWebRequests", "No pending Web requests")}</strong>
              <span>
                {t(
                  "storage.noWebRequestsHint",
                  "Native folder requests from the Web app will appear here for local approval.",
                )}
              </span>
            </div>
          </div>
        )}
      </SettingsSection>

      <SettingsSection title={t("storage.defaultLocation")}>
        <SettingRow
          title={t("storage.defaultLocation")}
          description={
            defaultLocation
              ? storageLocationDescription(defaultLocation, t)
              : configuredPath || t("storage.noDefault")
          }
        >
          <Button
            variant="secondary"
            size="sm"
            disabled={!defaultLocation?.canOpen}
            onClick={() => defaultLocation && void StorageService.OpenLocation(defaultLocation.id)}
          >
            <FolderOpen className="size-3.5" /> {t("common.open")}
          </Button>
          <Button
            variant="secondary"
            size="sm"
            disabled={draft.phase === "preparing" || draft.phase === "saving"}
            onClick={() => void draft.chooseDefaultStorage()}
          >
            {t("storage.locateDefaultLocation", "Locate Default Storage Location")}
          </Button>
        </SettingRow>
      </SettingsSection>

      <SettingsSection title={t("storage.additional")}>
        {additional.length ? (
          additional.map((item) => (
            <SettingRow
              key={item.id}
              title={item.name || t("storage.location")}
              description={storageLocationDescription(item, t)}
            >
              <RowActions>
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={!item.canOpen}
                  onClick={() => void StorageService.OpenLocation(item.id)}
                >
                  <FolderOpen className="size-3.5" /> {t("common.open")}
                </Button>
              </RowActions>
            </SettingRow>
          ))
        ) : (
          <div className="empty-state compact-empty-state">
            <div>
              <strong>{t("storage.noAdditional")}</strong>
              <span>
                {snapshot.runtime.phase === RuntimePhase.RuntimeRunning
                  ? t("storage.noAdditionalHint")
                  : t("storage.startServerHint")}
              </span>
            </div>
          </div>
        )}
      </SettingsSection>
    </>
  );
}

function LumenPanel({
  snapshot,
  showToast,
}: {
  snapshot: DesktopSnapshot;
  showToast: (input: ToastInput) => string;
}) {
  const { t } = useTranslation();
  const lumen = snapshot.lumen;
  const [inputError, setInputError] = useState<string | null>(null);
  const operation = useTrackedOperation(snapshot.operations, {
    onFailed: (message) =>
      showToast({ title: t("lumen.actionFailed"), description: message, status: "error" }),
  });
  const state = operation.state;
  const actionError = operation.error ?? inputError;
  const [selectedProfile, setSelectedProfile] = useState(
    lumen.profile || lumen.availableProfiles?.[0] || "",
  );
  const [selectedPreset, setSelectedPreset] = useState(
    lumen.preset || lumen.availablePresets?.[0] || "",
  );
  const [selectedCacheDir, setSelectedCacheDir] = useState(lumen.cacheDir || "");
  const [logLevel, setLogLevel] = useState("INFO");
  const [logs, setLogs] = useState<LumenLogEntry[]>([]);
  const [logsLoading, setLogsLoading] = useState(false);
  const [logsError, setLogsError] = useState<string | null>(null);
  const releaseReady = lumen.installerAvailable && lumen.processAvailable;

  useEffect(() => {
    setSelectedProfile(lumen.profile || lumen.availableProfiles?.[0] || "");
    setSelectedPreset(lumen.preset || lumen.availablePresets?.[0] || "");
    setSelectedCacheDir(lumen.cacheDir || "");
  }, [
    lumen.cacheDir,
    lumen.preset,
    lumen.profile,
    lumen.availablePresets,
    lumen.availableProfiles,
  ]);

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
    operation.begin();
    setInputError(null);
    const requestID = `lumen-${crypto.randomUUID()}`;
    try {
      let receipt: OperationReceipt;
      if (action === "install")
        receipt = await LumenService.Install(
          requestID,
          lumen.version,
          selectedProfile,
          selectedPreset,
          selectedCacheDir,
        );
      else if (action === "start") receipt = await LumenService.Start(requestID, lumen.version);
      else if (action === "stop") receipt = await LumenService.Stop(requestID, lumen.version);
      else if (action === "restart") receipt = await LumenService.Restart(requestID, lumen.version);
      else receipt = await LumenService.RetryCleanup(requestID, lumen.version);
      operation.track(receipt);
    } catch (reason: unknown) {
      operation.reject(reason);
    }
  };

  const installed = lumen.installPhase === LumenInstallPhase.LumenInstalled;
  const setupChanged = selectedPreset !== lumen.preset || selectedCacheDir !== lumen.cacheDir;
  const canSubmitSetup =
    lumen.installerAvailable &&
    Boolean(selectedProfile) &&
    Boolean(selectedPreset) &&
    Boolean(selectedCacheDir) &&
    state !== "loading" &&
    (!installed || setupChanged);
  const canChooseIntent = state !== "loading";
  const canChooseProfile = !installed && canChooseIntent;

  const chooseCacheDirectory = async () => {
    try {
      const path = await LumenService.PickCacheDirectory(
        t("lumen.chooseCacheDirectory", "Choose the Lumen model cache directory"),
      );
      if (path) {
        setSelectedCacheDir(path);
        setInputError(null);
      }
    } catch (reason: unknown) {
      const message = errorMessage(reason);
      setInputError(message);
      showToast({
        title: t("lumen.cachePickFailed", "Cache directory could not be selected"),
        description: message,
        status: "error",
      });
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
          <AnimatedBadge
            status={releaseReady ? presentationStatus(lumen.presentation) : "neutral"}
            size="sm"
          >
            {releaseReady ? lumen.presentation.label : t("onboarding.unavailable")}
          </AnimatedBadge>
        </SettingRow>
      </SettingsSection>

      {releaseReady ? (
        <SettingsSection title={t("lumen.service")}>
          <SettingRow
            title={t("lumen.preset", "Preset")}
            description={t(
              "lumen.presetDescription",
              "Choose a runtime preset exposed by the pinned Lumen Hub release.",
            )}
          >
            <Select
              value={selectedPreset}
              onValueChange={setSelectedPreset}
              disabled={!canChooseIntent}
              className="w-80 max-w-full"
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(lumen.availablePresets ?? []).map((preset) => (
                  <SelectItem key={preset} value={preset}>
                    {presetLabel(preset)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </SettingRow>
          <SettingRow
            title={t("lumen.backend", "Backend")}
            description={t(
              "lumen.backendDescription",
              "Backend determines which platform-specific Lumen Hub package is downloaded.",
            )}
          >
            <Select
              value={selectedProfile}
              onValueChange={setSelectedProfile}
              disabled={!canChooseProfile}
              className="w-80 max-w-full"
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(lumen.availableProfiles ?? []).map((profile) => (
                  <SelectItem key={profile} value={profile}>
                    {backendLabel(profile)} · {profile}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </SettingRow>
          <SettingRow
            title={t("lumen.cacheDirectory", "Model cache")}
            description={
              selectedCacheDir ||
              t("lumen.cacheDirectoryDescription", "Choose where Lumen stores downloaded models.")
            }
          >
            <Button
              variant="secondary"
              size="sm"
              disabled={!canChooseIntent}
              onClick={() => void chooseCacheDirectory()}
            >
              <FolderOpen className="size-3.5" /> {t("common.choose")}
            </Button>
          </SettingRow>
          <SettingRow
            title={installed ? t("lumen.reconfigure", "Reconfigure") : t("lumen.installation")}
            description={
              installed
                ? lumen.processPhase === LumenProcessPhase.LumenRunning
                  ? t(
                      "lumen.reconfigureRunning",
                      "The new preset and cache are validated first, then applied with a controlled restart. The previous setup is restored if startup fails.",
                    )
                  : t(
                      "lumen.reconfigureStopped",
                      "The new preset and cache are validated before they replace the current setup.",
                    )
                : selectedProfile
                  ? `${t("lumen.profile")}: ${selectedProfile}`
                  : t("lumen.noProfile")
            }
          >
            <StatefulButton
              variant="secondary"
              size="sm"
              state={state}
              disabled={!canSubmitSetup}
              loadingText={
                installed
                  ? t("lumen.applyingConfiguration", "Applying configuration")
                  : t("lumen.installing")
              }
              successText={
                installed
                  ? t("lumen.configurationApplied", "Configuration applied")
                  : t("lumen.installed")
              }
              onClick={() => void invoke("install")}
            >
              {installed
                ? t("lumen.applyConfiguration", "Apply configuration")
                : t("lumen.install")}
            </StatefulButton>
          </SettingRow>
          <SettingRow
            title={t("lumen.processStatus")}
            description={`${t("lumen.desiredState")}: ${lumen.desiredState || "disabled"}.`}
          >
            <RowActions>
              <AnimatedBadge status={presentationStatus(lumen.presentation)} size="sm">
                {lumen.presentation.label}
              </AnimatedBadge>
              {lumen.capabilities.canStartLumen ? (
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={state === "loading"}
                  onClick={() => void invoke("start")}
                >
                  {t("common.start")}
                </Button>
              ) : null}
              {lumen.capabilities.canStopLumen || lumen.capabilities.canRetryCleanupLumen ? (
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={state === "loading"}
                  onClick={() =>
                    void invoke(lumen.capabilities.canRetryCleanupLumen ? "retry" : "stop")
                  }
                >
                  {lumen.capabilities.canRetryCleanupLumen
                    ? t("common.retryCleanup")
                    : t("common.stop")}
                </Button>
              ) : null}
              {lumen.capabilities.canRestartLumen ? (
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={state === "loading"}
                  onClick={() => void invoke("restart")}
                >
                  {t("common.restart")}
                </Button>
              ) : null}
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
          description={t(
            "lumen.logsDescription",
            "Structured logs from Lumen Control. The view refreshes every five seconds while Hub is running.",
          )}
        >
          <div className="lumen-log-toolbar">
            <Select
              value={logLevel}
              onValueChange={setLogLevel}
              disabled={!lumen.control.connected}
              className="compact-select"
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(["TRACE", "DEBUG", "INFO", "WARN", "ERROR"] as const).map((level) => (
                  <SelectItem key={level} value={level}>
                    {level}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button
              variant="secondary"
              size="sm"
              disabled={!lumen.control.connected || logsLoading}
              onClick={() => void loadLogs()}
            >
              <RefreshCw className={logsLoading ? "size-3.5 animate-spin" : "size-3.5"} />
              {t("common.refresh", "Refresh")}
            </Button>
          </div>
          {logsError ? (
            <InlineNotice tone="danger" title={t("lumen.logsUnavailable", "Logs unavailable")}>
              {logsError}
            </InlineNotice>
          ) : null}
          <LumenLogViewer logs={logs} connected={lumen.control.connected} loading={logsLoading} />
        </SettingsSection>
      ) : null}

      {actionError ? <ActionNotice component={t("dock.lumen")} message={actionError} /> : null}
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
      description={t(
        "lumen.controlDescription",
        "Live lifecycle and model state reported by lumen.control.v1.",
      )}
    >
      <div className="lumen-control-summary">
        <div>
          <span className="lumen-control-kicker">{t("lumen.inference", "Inference")}</span>
          <strong>
            {control.inferenceReady
              ? t("lumen.ready", "Ready")
              : control.connected
                ? t("lumen.preparing", "Preparing")
                : t("lumen.disconnected", "Disconnected")}
          </strong>
        </div>
        <AnimatedBadge status={controlPhaseStatus(control.phase)} size="sm">
          {controlPhaseLabel(control.phase, t)}
        </AnimatedBadge>
        <dl className="lumen-control-meta">
          <div>
            <dt>{t("lumen.version", "Version")}</dt>
            <dd>{control.version || "—"}</dd>
          </div>
          <div>
            <dt>{t("lumen.backend", "Backend")}</dt>
            <dd>{control.backend || "—"}</dd>
          </div>
          <div>
            <dt>{t("lumen.sequence", "Sequence")}</dt>
            <dd>{control.sequence || "—"}</dd>
          </div>
        </dl>
      </div>

      {control.connected ? (
        <ol
          className="lumen-phase-track"
          aria-label={t("lumen.lifecycle", "Lumen startup lifecycle")}
        >
          {lumenPhaseOrder.map((phase, index) => (
            <li
              key={phase}
              className={
                index < currentIndex ? "complete" : index === currentIndex ? "current" : undefined
              }
              aria-current={index === currentIndex ? "step" : undefined}
            >
              <span aria-hidden />
              <small>{controlPhaseLabel(phase, t)}</small>
            </li>
          ))}
        </ol>
      ) : (
        <InlineNotice title={t("lumen.controlWaiting", "Waiting for Control")}>
          {t(
            "lumen.controlWaitingDescription",
            "Start Lumen Hub to connect to its local control plane.",
          )}
        </InlineNotice>
      )}

      {control.download ? (
        <div
          className="lumen-download"
          aria-label={t("lumen.downloadProgress", "Model download progress")}
        >
          <div className="lumen-download-heading">
            <div>
              <strong>{control.download.model || t("lumen.model", "Model")}</strong>
              <span>
                {control.download.file || t("lumen.preparingDownload", "Preparing download")}
              </span>
            </div>
            <span className="tabular-value">
              {downloadPercent === null
                ? t("lumen.downloading", "Downloading")
                : `${downloadPercent.toFixed(1)}%`}
            </span>
          </div>
          <progress
            value={control.download.bytesDone}
            max={control.download.bytesTotal || undefined}
          />
          <div className="lumen-download-meta">
            <span>
              {formatBytes(control.download.bytesDone)}
              {control.download.bytesTotal ? ` / ${formatBytes(control.download.bytesTotal)}` : ""}
            </span>
            <span>
              {t("lumen.filesProgress", "Files {{done}} / {{total}}", {
                done: control.download.filesDone,
                total: control.download.filesTotal,
              })}
            </span>
          </div>
        </div>
      ) : null}

      {control.error ? (
        <InlineNotice tone="danger" title={t("lumen.controlFailed", "Lumen startup failed")}>
          {control.error.message}
        </InlineNotice>
      ) : null}

      <div className="lumen-services">
        <div className="lumen-subheading">
          <h3>{t("lumen.services", "AI services")}</h3>
          <span>
            {t("lumen.servicesReported", "{{count}} reported", {
              count: control.services?.length ?? 0,
            })}
          </span>
        </div>
        {control.services?.length ? (
          control.services.map((service) => (
            <div className="lumen-service-row" key={service.service}>
              <div>
                <strong>{serviceDisplayName(service.service)}</strong>
                {service.error ? <span>{service.error.message}</span> : null}
              </div>
              <AnimatedBadge status={controlPhaseStatus(service.phase)} size="sm">
                {controlPhaseLabel(service.phase, t)}
              </AnimatedBadge>
            </div>
          ))
        ) : (
          <p className="lumen-empty-copy">
            {control.connected
              ? t("lumen.servicesPending", "Service states will appear after model construction.")
              : t("lumen.servicesOffline", "No service state is available while Hub is stopped.")}
          </p>
        )}
      </div>
    </SettingsSection>
  );
}

function LumenLogViewer({
  logs,
  connected,
  loading,
}: {
  logs: LumenLogEntry[];
  connected: boolean;
  loading: boolean;
}) {
  const { t } = useTranslation();
  if (!connected)
    return (
      <p className="lumen-empty-copy lumen-log-empty">
        {t("lumen.logsOffline", "Start Lumen Hub to read Control logs.")}
      </p>
    );
  if (!logs.length && loading)
    return (
      <p className="lumen-empty-copy lumen-log-empty">
        {t("lumen.logsLoading", "Reading Control logs…")}
      </p>
    );
  if (!logs.length)
    return (
      <p className="lumen-empty-copy lumen-log-empty">
        {t("lumen.logsEmpty", "No log entries match this level.")}
      </p>
    );
  return (
    <div className="lumen-log-view" role="log" aria-label={t("lumen.logs", "Control logs")}>
      {logs.map((entry, index) => (
        <div
          className="lumen-log-line"
          key={`${entry.timeUnixMS}-${index}`}
          data-level={entry.level}
        >
          <time>{formatLogTime(entry.timeUnixMS)}</time>
          <span className="lumen-log-level">{entry.level}</span>
          <span className="lumen-log-target">{entry.target}</span>
          <span className="lumen-log-message">
            {entry.message}
            {formatLogFields(entry.fields)}
          </span>
        </div>
      ))}
    </div>
  );
}

function controlPhaseStatus(phase: LumenControlPhase): AnimatedBadgeStatus {
  if (phase === LumenControlPhase.LumenControlReady) return "success";
  if (phase === LumenControlPhase.LumenControlFailed) return "danger";
  if (
    phase === LumenControlPhase.LumenControlUnspecified ||
    phase === LumenControlPhase.LumenControlStopping
  )
    return "neutral";
  return "warning";
}

function controlPhaseLabel(phase: LumenControlPhase, t: ReturnType<typeof useTranslation>["t"]) {
  if (phase === LumenControlPhase.LumenControlStarting) return t("lumen.phaseStarting", "Starting");
  if (phase === LumenControlPhase.LumenControlDownloading)
    return t("lumen.phaseDownloading", "Downloading");
  if (phase === LumenControlPhase.LumenControlLoading) return t("lumen.phaseLoading", "Loading");
  if (phase === LumenControlPhase.LumenControlWarmup) return t("lumen.phaseWarmup", "Warmup");
  if (phase === LumenControlPhase.LumenControlReady) return t("lumen.phaseReady", "Ready");
  if (phase === LumenControlPhase.LumenControlFailed) return t("lumen.phaseFailed", "Failed");
  if (phase === LumenControlPhase.LumenControlStopping) return t("lumen.phaseStopping", "Stopping");
  return t("lumen.phaseUnavailable", "Unavailable");
}

function serviceDisplayName(service: string) {
  const names: Record<string, string> = {
    siglip: "SigLIP",
    face: "InsightFace",
    insightface: "InsightFace",
    ocr: "PP-OCR",
    ppocr: "PP-OCR",
    bioclip: "BioCLIP",
  };
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
  return new Date(unixMS).toLocaleTimeString([], {
    hour12: false,
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
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
  const operation = useTrackedOperation(snapshot.operations, {
    onFailed: (message) =>
      showToast({ title: t("updates.actionFailed"), description: message, status: "error" }),
  });
  const state = operation.state;

  const invoke = async (action: "check" | "download" | "apply") => {
    operation.begin();
    const requestID = `update-${crypto.randomUUID()}`;
    try {
      let receipt: OperationReceipt;
      if (action === "check") receipt = await UpdateService.Check(requestID, update.version);
      else if (action === "download")
        receipt = await UpdateService.Download(requestID, update.version);
      else receipt = await UpdateService.RestartAndApply(requestID, update.version);
      operation.track(receipt);
    } catch (reason: unknown) {
      operation.reject(reason);
    }
  };

  return (
    <>
      <PageHeading title={t("dock.updates")} description={t("updates.description")} />
      <SettingsSection title={t("dock.updates")}>
        <SettingRow
          title={t("updates.currentVersion")}
          description={t("updates.currentVersionDescription")}
        >
          <span className="setting-value tabular-value">
            {update.currentVersion || t("common.unknown")}
          </span>
        </SettingRow>
        <SettingRow title={t("updates.channel")} description={t("updates.channelDescription")}>
          {draft.preferences ? (
            <Select
              value={draft.preferences.updateChannel}
              onValueChange={(value) => draft.updatePreference("updateChannel", value)}
              disabled={draft.phase === "saving"}
              className="compact-select"
            >
              <SelectTrigger>
                <SelectValue placeholder={t("updates.channel")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="stable">{t("updates.stable")}</SelectItem>
                <SelectItem value="beta">{t("updates.beta")}</SelectItem>
              </SelectContent>
            </Select>
          ) : (
            <span className="setting-value">{t("common.loadingEllipsis")}</span>
          )}
        </SettingRow>
        <SettingRow
          title={t("updates.checkTitle")}
          description={
            update.providerAvailable ? t("updates.checkDescription") : t("updates.noProvider")
          }
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
          <SettingRow
            title={t("updates.available")}
            description={t("updates.availableDescription", { version: update.availableVersion })}
          >
            <RowActions>
              <Button
                variant="secondary"
                size="sm"
                disabled={state === "loading"}
                onClick={() => void invoke("download")}
              >
                {t("updates.download")}
              </Button>
              {update.canApply ? (
                <Button
                  size="sm"
                  disabled={state === "loading"}
                  onClick={() => void invoke("apply")}
                >
                  {t("updates.restartInstall")}
                </Button>
              ) : null}
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
    <Select
      value={preferences.locale}
      onValueChange={(value) => update("locale", value)}
      disabled={disabled}
      className="compact-select"
    >
      <SelectTrigger>
        <SelectValue placeholder={t("general.language")} />
      </SelectTrigger>
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
  const currentLabel = system
    ? t("general.themeSystem")
    : preferences.theme === "dark"
      ? t("general.themeDark")
      : t("general.themeLight");
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
    <Select
      value={preferences.region}
      onValueChange={(value) => update("region", value)}
      disabled={disabled}
      className="compact-select"
    >
      <SelectTrigger>
        <SelectValue placeholder={t("general.region")} />
      </SelectTrigger>
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
    <Select
      value={settings.networkMode}
      onValueChange={(value) => update("networkMode", value)}
      disabled={disabled}
      className="compact-select"
    >
      <SelectTrigger>
        <SelectValue placeholder={t("network.access")} />
      </SelectTrigger>
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
      {actionLabel && onAction ? (
        <Button variant="secondary" size="sm" onClick={onAction}>
          {actionLabel}
        </Button>
      ) : null}
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

function RowActions({ children }: { children: ReactNode }) {
  return <div className="row-actions">{children}</div>;
}

function presentationStatus(presentation: ProcessPresentation): AnimatedBadgeStatus {
  if (presentation.color === "green") return "success";
  if (presentation.color === "yellow") return "warning";
  if (presentation.color === "red") return "danger";
  return "neutral";
}

function hostActionLabel(kind: string, t: ReturnType<typeof useTranslation>["t"]): string {
  if (kind === "authorize_storage_location") {
    return t("storage.hostActionAddLocation", "Add Storage Location");
  }
  if (kind === "open_repository") {
    return t("storage.hostActionOpenRepository", "Open Existing Repository");
  }
  if (kind === "locate_storage_location") {
    return t("storage.hostActionLocateLocation", "Locate Storage Location");
  }
  if (kind === "locate_repository") {
    return t("storage.hostActionLocateRepository", "Locate Repository");
  }
  return t("storage.hostAction", "Storage request");
}

function hostActionRiskLabel(
  warning: string,
  t: ReturnType<typeof useTranslation>["t"],
): string {
  if (warning === "network_filesystem") return t("storage.riskNetwork", "Network filesystem");
  if (warning === "removable_storage") return t("storage.riskRemovable", "Removable storage");
  if (warning === "cloud_sync_directory")
    return t("storage.riskCloudSync", "Cloud-sync managed folder");
  if (warning === "mount_fingerprint_changed")
    return t("storage.riskMountChanged", "Mounted filesystem changed");
  if (warning === "unavailable_cloud_placeholder")
    return t("storage.riskPlaceholder", "Files must be downloaded first");
  return warning;
}

function storageLocationDescription(
  item: StorageShortcut,
  t: ReturnType<typeof useTranslation>["t"],
): string {
  const details = [item.path || item.status];
  details.push(
    item.writable ? t("storage.writable", "Writable") : t("storage.readOnly", "Read-only"),
  );
  if (item.capacityKnown) {
    details.push(
      t("storage.capacityAvailable", "{{available}} of {{total}} available", {
        available: formatStorageBytes(item.availableBytes ?? 0),
        total: formatStorageBytes(item.totalBytes ?? 0),
      }),
    );
  } else {
    details.push(t("storage.capacityUnknown", "Capacity unavailable"));
  }
  details.push(
    t("storage.repositoryCount", "{{count}} repositories", { count: item.repositoryCount }),
  );
  return details.join(" · ");
}

function formatStorageBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  const unit = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** unit).toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
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
