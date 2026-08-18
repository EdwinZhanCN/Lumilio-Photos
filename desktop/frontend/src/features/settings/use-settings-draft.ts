import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  DesktopService,
  RuntimeService,
  StorageService,
} from "../../../bindings/desktop/internal/control/index.js";
import type {
  ConfigValidation,
  DesktopPreferences,
  DesktopSnapshot,
  OperationReceipt,
  RuntimeConfigDraft,
  RuntimeConfigSettings,
} from "../../../bindings/desktop/internal/control/dto/models.js";
import type { ToastInput } from "@/components/motion/animated-toast-stack";
import { errorMessage } from "@/lib/desktop/errors";

export type SettingsDraftPhase =
  | "loading"
  | "saved"
  | "preparing"
  | "draft"
  | "saving"
  | "success"
  | "error";

export interface SettingsDraftController {
  runtimeDraft: RuntimeConfigDraft | null;
  runtime: RuntimeConfigSettings | null;
  preferences: DesktopPreferences | null;
  validation: ConfigValidation | null;
  phase: SettingsDraftPhase;
  dirty: boolean;
  error: string | null;
  updateRuntime: <K extends keyof RuntimeConfigSettings>(
    key: K,
    value: RuntimeConfigSettings[K],
  ) => void;
  updatePreference: <K extends keyof DesktopPreferences>(
    key: K,
    value: DesktopPreferences[K],
  ) => void;
  chooseDefaultStorage: () => Promise<void>;
  cancel: () => Promise<void>;
  save: () => Promise<boolean>;
}

export function useSettingsDraft(
  snapshot: DesktopSnapshot | null,
  showToast: (input: ToastInput) => string,
): SettingsDraftController {
  const { t } = useTranslation();
  const [runtimeDraft, setRuntimeDraft] = useState<RuntimeConfigDraft | null>(null);
  const [runtime, setRuntime] = useState<RuntimeConfigSettings | null>(null);
  const [preferences, setPreferences] = useState<DesktopPreferences | null>(null);
  const [savedPreferences, setSavedPreferences] = useState<DesktopPreferences | null>(null);
  const [validation, setValidation] = useState<ConfigValidation | null>(null);
  const [runtimeDirty, setRuntimeDirty] = useState(false);
  const [preferenceDirty, setPreferenceDirty] = useState(false);
  const [phase, setPhase] = useState<SettingsDraftPhase>("loading");
  const [error, setError] = useState<string | null>(null);
  const candidateRef = useRef("");
  const runtimeRef = useRef<RuntimeConfigSettings | null>(null);
  const snapshotRef = useRef(snapshot);
  const patchSequence = useRef(0);
  const successTimer = useRef<number | null>(null);
  const operationWaiters = useRef(new Set<number>());
  const loadedInstance = useRef("");
  const savedStoragePath = useRef("");

  snapshotRef.current = snapshot;

  const installRuntimeDraft = useCallback((next: RuntimeConfigDraft) => {
    setRuntimeDraft(next);
    setRuntime(next.settings);
    runtimeRef.current = next.settings;
    candidateRef.current = next.toml;
    setValidation(next.validation);
    const firstRun = next.source === "default";
    setRuntimeDirty(firstRun);
    setPhase(firstRun ? "draft" : "saved");
    setError(null);
    savedStoragePath.current = next.settings.storagePath;
  }, []);

  const load = useCallback(async () => {
    const current = snapshotRef.current;
    if (!current) return;
    setPhase("loading");
    setError(null);
    try {
      const next = await RuntimeService.ReadConfigDraft();
      installRuntimeDraft(next);
      setPreferences(current.host.preferences);
      setSavedPreferences(current.host.preferences);
      setPreferenceDirty(false);
    } catch (reason: unknown) {
      const message = errorMessage(reason);
      setError(message);
      setPhase("error");
      showToast({ title: t("toast.settingsLoadFailed"), description: message, status: "error" });
    }
  }, [installRuntimeDraft, showToast]);

  useEffect(() => {
    if (!snapshot || loadedInstance.current === snapshot.instanceID) return;
    loadedInstance.current = snapshot.instanceID;
    void load();
  }, [load, snapshot]);

  useEffect(
    () => () => {
      if (successTimer.current !== null) window.clearTimeout(successTimer.current);
      for (const timer of operationWaiters.current) window.clearInterval(timer);
      operationWaiters.current.clear();
    },
    [],
  );

  const waitForOperation = useCallback(
    (receipt: OperationReceipt) =>
      new Promise<void>((resolve, reject) => {
        const deadline = Date.now() + 120_000;
        const finish = (timer: number, result?: Error) => {
          window.clearInterval(timer);
          operationWaiters.current.delete(timer);
          if (result) reject(result);
          else resolve();
        };
        const inspect = (timer: number) => {
          const operation = snapshotRef.current?.operations?.find(
            (item) => item.operationID === receipt.operationID,
          );
          if (operation?.state === "succeeded") {
            finish(timer);
            return;
          }
          if (operation?.state === "failed" || operation?.state === "cancelled") {
            finish(
              timer,
              new Error(
                operation.error?.message ||
                  t("toast.settingsSaveFailed", "Settings could not be saved"),
              ),
            );
            return;
          }
          if (Date.now() >= deadline) {
            finish(
              timer,
              new Error(
                t(
                  "toast.settingsOperationTimeout",
                  "The settings operation did not finish in time. Review the Server status before retrying.",
                ),
              ),
            );
          }
        };
        const timer = window.setInterval(() => inspect(timer), 100);
        operationWaiters.current.add(timer);
        inspect(timer);
      }),
    [t],
  );

  const patchRuntime = useCallback(async (next: RuntimeConfigSettings) => {
    runtimeRef.current = next;
    setRuntime(next);
    setRuntimeDirty(true);
    setPhase("preparing");
    setError(null);
    const sequence = ++patchSequence.current;
    try {
      const patched = await RuntimeService.PatchConfigDraft(candidateRef.current, next);
      if (sequence !== patchSequence.current) return;
      setRuntimeDraft((current) =>
        current ? { ...patched, baseFingerprint: current.baseFingerprint } : patched,
      );
      setRuntime(patched.settings);
      runtimeRef.current = patched.settings;
      candidateRef.current = patched.toml;
      setValidation(patched.validation);
      setPhase("draft");
    } catch (reason: unknown) {
      if (sequence !== patchSequence.current) return;
      const message = errorMessage(reason);
      setValidation({ valid: false, issues: [{ path: "", message }] });
      setError(message);
      setPhase("error");
    }
  }, []);

  const updateRuntime = <K extends keyof RuntimeConfigSettings>(
    key: K,
    value: RuntimeConfigSettings[K],
  ) => {
    const current = runtimeRef.current;
    if (!current) return;
    void patchRuntime({ ...current, [key]: value });
  };

  const updatePreference = <K extends keyof DesktopPreferences>(
    key: K,
    value: DesktopPreferences[K],
  ) => {
    setPreferences((current) => {
      if (!current) return current;
      const next = { ...current, [key]: value };
      setPreferenceDirty(!preferencesEqual(next, savedPreferences));
      setPhase("draft");
      setError(null);
      return next;
    });
  };

  const chooseDefaultStorage = async () => {
    try {
      const path = await StorageService.PickLocation(
        t("storage.chooseDefaultLocation", "Choose Default Storage Location"),
      );
      if (!path || path === runtimeRef.current?.storagePath) return;
      if (savedStoragePath.current) {
        const confirmed = window.confirm(
          t(
            "storage.defaultLocationMigrationConfirmation",
            "Before applying this change, move the complete default Storage Location to the selected folder. Lumilio will verify the existing .lumilioroot identity and fixed primary/.lumiliorepo marker during the controlled Server restart. It will not copy files or create a new identity. Continue?",
          ),
        );
        if (!confirmed) return;
      }
      updateRuntime("storagePath", path);
    } catch (reason: unknown) {
      showToast({
        title: t("toast.storagePickFailed", "Folder could not be selected"),
        description: errorMessage(reason),
        status: "error",
      });
    }
  };

  const cancel = async () => {
    patchSequence.current++;
    await load();
  };

  const save = async () => {
    const currentSnapshot = snapshotRef.current;
    const currentDraft = runtimeDraft;
    const currentPreferences = preferences;
    if (
      !currentSnapshot ||
      !currentDraft ||
      !currentPreferences ||
      phase === "preparing" ||
      phase === "saving"
    ) {
      return false;
    }
    setPhase("saving");
    setError(null);
    try {
      let savedPreferenceValue = currentPreferences;
      if (preferenceDirty) {
        savedPreferenceValue = await DesktopService.SavePreferences(currentPreferences);
        setPreferences(savedPreferenceValue);
        setSavedPreferences(savedPreferenceValue);
        setPreferenceDirty(false);
      }

      if (runtimeDirty) {
        const checked = await RuntimeService.ValidateConfig(candidateRef.current);
        setValidation(checked);
        if (!checked.valid) {
          throw new Error(
            checked.issues?.[0]?.message ||
              t("server.settingsInvalid", "Server settings are invalid"),
          );
        }
        const running =
          currentSnapshot.runtime.phase === "running" &&
          currentSnapshot.runtime.ownership === "held";
        const requestID = `config-${crypto.randomUUID()}`;
        let receipt: OperationReceipt;
        if (running) {
          receipt = await RuntimeService.ApplyConfig(
            requestID,
            currentSnapshot.runtime.version,
            currentDraft.baseFingerprint,
            candidateRef.current,
          );
        } else {
          receipt = await RuntimeService.SaveConfig(
            requestID,
            currentSnapshot.runtime.version,
            currentDraft.baseFingerprint,
            candidateRef.current,
          );
        }
        await waitForOperation(receipt);
        const savedRuntime = await RuntimeService.ReadConfigDraft();
        installRuntimeDraft(savedRuntime);
        setRuntimeDirty(false);
      }

      setPreferences(savedPreferenceValue);
      setSavedPreferences(savedPreferenceValue);
      setPreferenceDirty(false);
      setRuntimeDirty(false);
      setPhase("success");
      successTimer.current = window.setTimeout(() => setPhase("saved"), 1800);
      return true;
    } catch (reason: unknown) {
      const message = errorMessage(reason);
      setError(message);
      setPhase("error");
      return false;
    }
  };

  return {
    runtimeDraft,
    runtime,
    preferences,
    validation,
    phase,
    dirty: runtimeDirty || preferenceDirty,
    error,
    updateRuntime,
    updatePreference,
    chooseDefaultStorage,
    cancel,
    save,
  };
}

function preferencesEqual(left: DesktopPreferences, right: DesktopPreferences | null) {
  return (
    right !== null &&
    left.locale === right.locale &&
    left.region === right.region &&
    left.updateChannel === right.updateChannel &&
    left.theme === right.theme &&
    left.openProductOnLaunch === right.openProductOnLaunch
  );
}
