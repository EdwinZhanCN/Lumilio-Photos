const restoreOperationStorageKey = "lumilio.restore-operation-id";

export const readRestoreOperationID = (): string | null =>
  typeof window === "undefined" ? null : window.localStorage.getItem(restoreOperationStorageKey);

export const saveRestoreOperationID = (operationID: string): void => {
  window.localStorage.setItem(restoreOperationStorageKey, operationID);
};

export const clearRestoreOperationID = (): void => {
  window.localStorage.removeItem(restoreOperationStorageKey);
};
