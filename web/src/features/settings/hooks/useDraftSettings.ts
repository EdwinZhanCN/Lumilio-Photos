import { useCallback, useEffect, useRef, useState } from "react";

export interface UseDraftSettingsOptions<T> {
  server: T | undefined;
  isLoading: boolean;
  isSaving: boolean;
  saveError: unknown;
  onSave: (value: T) => Promise<unknown>;
}

export interface DraftSettings<T> {
  draft: T | undefined;
  setField: <K extends keyof T>(key: K, value: T[K]) => void;
  setDraft: (next: T) => void;
  isLoading: boolean;
  isDirty: boolean;
  isSaving: boolean;
  saveError: unknown;
  justSaved: boolean;
  canSave: boolean;
  save: () => void;
  saveAsync: () => Promise<void>;
  reset: () => void;
}

function equal<T>(a: T | undefined, b: T | undefined): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

export function useDraftSettings<T>(
  options: UseDraftSettingsOptions<T>,
): DraftSettings<T> {
  const { server, isLoading, isSaving, saveError, onSave } = options;

  const [draft, setDraftState] = useState<T | undefined>(server);
  const [touched, setTouched] = useState(false);
  const [justSaved, setJustSaved] = useState(false);
  const savedTimer = useRef<ReturnType<typeof setTimeout> | undefined>(
    undefined,
  );

  const serverKey =
    server === undefined ? "__undefined__" : JSON.stringify(server);

  useEffect(() => {
    if (touched) return;
    setDraftState((prev) => (equal(prev, server) ? prev : server));
    // serverKey is a value snapshot; server is read when the snapshot changes.
  }, [serverKey, touched]);

  const setDraft = useCallback((next: T) => {
    setTouched(true);
    setJustSaved(false);
    setDraftState(next);
  }, []);

  const setField = useCallback(
    <K extends keyof T>(key: K, value: T[K]) => {
      setTouched(true);
      setJustSaved(false);
      setDraftState((prev) =>
        prev === undefined ? prev : { ...prev, [key]: value },
      );
    },
    [],
  );

  const reset = useCallback(() => {
    setTouched(false);
    setJustSaved(false);
    setDraftState(server);
  }, [server]);

  const isDirty = touched && !equal(draft, server);

  const saveAsync = useCallback(async () => {
    if (draft === undefined || isSaving) return;
    await onSave(draft);
    setTouched(false);
    setJustSaved(true);
    if (savedTimer.current) clearTimeout(savedTimer.current);
    savedTimer.current = setTimeout(() => setJustSaved(false), 2500);
  }, [draft, isSaving, onSave]);

  const save = useCallback(() => {
    void saveAsync().catch(() => {});
  }, [saveAsync]);

  useEffect(
    () => () => {
      if (savedTimer.current) clearTimeout(savedTimer.current);
    },
    [],
  );

  return {
    draft,
    setField,
    setDraft,
    isLoading,
    isDirty,
    isSaving,
    saveError,
    justSaved,
    canSave: isDirty && !isSaving,
    save,
    saveAsync,
    reset,
  };
}
