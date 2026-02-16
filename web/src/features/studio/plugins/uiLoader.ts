import type { StudioPluginUiModule } from "./types";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isUiModuleShape(value: unknown): value is StudioPluginUiModule {
  if (!isRecord(value)) return false;

  const meta = value.meta;
  if (!isRecord(meta)) return false;

  if (typeof meta.id !== "string") return false;
  if (typeof meta.version !== "string") return false;
  if (typeof meta.displayName !== "string") return false;

  if (!isRecord(meta.mount)) return false;
  if (meta.mount.panel !== "frames" && meta.mount.panel !== "develop") return false;

  if (!isRecord(value.defaultParams)) return false;
  if (typeof value.Panel !== "function") return false;

  if (
    value.normalizeParams !== undefined &&
    typeof value.normalizeParams !== "function"
  ) {
    return false;
  }

  return true;
}

export async function loadPluginUiModule(entryUrl: string): Promise<StudioPluginUiModule> {
  const mod = await import(/* @vite-ignore */ entryUrl);

  const candidates: unknown[] = [
    mod?.default,
    {
      meta: mod?.meta,
      defaultParams: mod?.defaultParams,
      Panel: mod?.Panel,
      normalizeParams: mod?.normalizeParams,
    },
  ];

  const matched = candidates.find((candidate) => isUiModuleShape(candidate));

  if (!matched) {
    throw new Error(`Invalid Studio plugin UI module: ${entryUrl}`);
  }

  return matched;
}
