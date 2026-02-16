import React, { useState, useRef, useEffect, useCallback, useMemo } from "react";
import { useExtractExifdata } from "@/hooks/util-hooks/useExtractExifdata.tsx";
import { useWorker } from "@/contexts/WorkerProvider.tsx";
import { useMessage } from "@/hooks/util-hooks/useMessage.tsx";
import {
  StudioHeader,
  StudioSidebar,
  StudioToolsPanel,
  StudioViewport,
} from "@/features/studio/components";
import { useStudioPluginCatalog } from "@/features/studio/hooks/useStudioPluginCatalog";
import { useStudioPluginInstall } from "@/features/studio/hooks/useStudioPluginInstall";
import { fetchAndVerifyManifest } from "@/features/studio/plugins/registryClient";
import { loadPluginUiModule } from "@/features/studio/plugins/uiLoader";
import type {
  RuntimeManifestV1,
  StudioPluginUiModule,
} from "@/features/studio/plugins/types";

type FrameProcessingProgress = {
  processed: number;
  total: number;
  error?: string;
  failedAt?: number | null;
} | null;

export type PanelType = "exif" | "develop" | "frames";

const pluginRuntimeEnabled =
  import.meta.env.VITE_STUDIO_PLUGIN_RUNTIME_ENABLED !== "false";

export function Studio() {
  // Core State
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [imageUrl, setImageUrl] = useState<string | null>(null);
  const [isCancellingExif, setIsCancellingExif] = useState(false);

  // UI State
  const [activePanel, setActivePanel] = useState<PanelType>("exif");
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Processed EXIF data for display
  const [exifToDisplay, setExifToDisplay] = useState<Record<string, any> | null>(null);

  // Plugin Runtime State
  const [selectedPluginId, setSelectedPluginId] = useState<string | null>(null);
  const [selectedPluginManifest, setSelectedPluginManifest] =
    useState<RuntimeManifestV1 | null>(null);
  const [selectedPluginUiModule, setSelectedPluginUiModule] =
    useState<StudioPluginUiModule | null>(null);
  const [pluginParams, setPluginParams] = useState<Record<string, unknown>>({});
  const [pluginLoading, setPluginLoading] = useState(false);
  const [pluginError, setPluginError] = useState<string | null>(null);
  const [pluginResultUrl, setPluginResultUrl] = useState<string | null>(null);
  const [isGeneratingPlugin, setIsGeneratingPlugin] = useState(false);
  const [pluginProgress, setPluginProgress] =
    useState<FrameProcessingProgress>(null);
  const [isCancellingPlugin, setIsCancellingPlugin] = useState(false);

  // Hooks
  const workerClient = useWorker();
  const showMessage = useMessage();

  const {
    catalog: pluginCatalog,
    isLoading: isCatalogLoading,
    error: catalogError,
  } = useStudioPluginCatalog("frames", pluginRuntimeEnabled);

  const { installed, install, uninstall, isInstalled } = useStudioPluginInstall();

  const {
    isExtracting,
    exifData,
    progress: exifProgress,
    extractExifData,
    cancelExtraction,
  } = useExtractExifdata();

  const selectedInstalledPluginRecord = useMemo(() => {
    if (!selectedPluginId) {
      return null;
    }
    return installed.find((item) => item.pluginId === selectedPluginId) ?? null;
  }, [installed, selectedPluginId]);

  useEffect(() => {
    if (exifData && exifData[0]) {
      setExifToDisplay(exifData[0]);
    } else {
      setExifToDisplay(null);
    }
  }, [exifData]);

  useEffect(() => {
    if (!pluginRuntimeEnabled) {
      setSelectedPluginId(null);
      setSelectedPluginManifest(null);
      setSelectedPluginUiModule(null);
      setPluginParams({});
      setPluginError(null);
      return;
    }

    if (installed.length === 0) {
      setSelectedPluginId(null);
      return;
    }

    if (!selectedPluginId || !installed.some((item) => item.pluginId === selectedPluginId)) {
      setSelectedPluginId(installed[0].pluginId);
    }
  }, [installed, selectedPluginId]);

  useEffect(() => {
    let cancelled = false;

    const loadPlugin = async () => {
      if (!pluginRuntimeEnabled || !selectedInstalledPluginRecord) {
        setSelectedPluginManifest(null);
        setSelectedPluginUiModule(null);
        setPluginParams({});
        setPluginError(null);
        return;
      }

      setPluginLoading(true);
      setPluginError(null);

      try {
        const manifest = await fetchAndVerifyManifest(
          selectedInstalledPluginRecord.pluginId,
          selectedInstalledPluginRecord.version,
        );

        if (manifest.mount.panel !== "frames") {
          throw new Error(
            `Plugin ${manifest.id} cannot mount on frames panel (${manifest.mount.panel})`,
          );
        }

        const uiModule = await loadPluginUiModule(manifest.entries.ui);
        if (
          uiModule.meta.id !== manifest.id ||
          uiModule.meta.version !== manifest.version
        ) {
          throw new Error(
            `Plugin UI entry mismatch for ${manifest.id}@${manifest.version}`,
          );
        }

        if (cancelled) {
          return;
        }

        setSelectedPluginManifest(manifest);
        setSelectedPluginUiModule(uiModule);
        setPluginParams(uiModule.defaultParams);
      } catch (error) {
        if (cancelled) {
          return;
        }

        setSelectedPluginManifest(null);
        setSelectedPluginUiModule(null);
        setPluginParams({});
        setPluginError(
          error instanceof Error
            ? error.message
            : "Failed to load selected Studio plugin",
        );
      } finally {
        if (!cancelled) {
          setPluginLoading(false);
        }
      }
    };

    loadPlugin().catch(() => {
      // handled by state updates above
    });

    return () => {
      cancelled = true;
    };
  }, [selectedInstalledPluginRecord]);

  useEffect(() => {
    const removeListener = workerClient.addProgressListener((detail) => {
      if (detail?.operation !== "plugin") {
        return;
      }

      setPluginProgress({
        processed: Number(detail.processed || 0),
        total: Number(detail.total || 100),
      });
    });

    return () => {
      removeListener();
    };
  }, [workerClient]);

  // Clean up original image URL when it is replaced or component unmounts
  useEffect(() => {
    return () => {
      if (imageUrl) {
        URL.revokeObjectURL(imageUrl);
      }
    };
  }, [imageUrl]);

  // Revoke plugin output object URL when replaced/unmounted
  useEffect(() => {
    return () => {
      if (pluginResultUrl) {
        URL.revokeObjectURL(pluginResultUrl);
      }
    };
  }, [pluginResultUrl]);

  //#region Handlers
  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }

    setSelectedFile(file);

    if (imageUrl) {
      URL.revokeObjectURL(imageUrl);
    }

    setPluginResultUrl((prev) => {
      if (prev) {
        URL.revokeObjectURL(prev);
      }
      return null;
    });

    const newUrl = URL.createObjectURL(file);
    setImageUrl(newUrl);

    // Reset panel-scoped state for the new source image.
    setExifToDisplay(null);
    setPluginProgress(null);
    setActivePanel("exif");
  };

  const handleExtractExif = useCallback(async () => {
    if (!selectedFile) {
      return;
    }

    setExifToDisplay(null);
    await extractExifData([selectedFile]);
    setActivePanel("exif");
  }, [selectedFile, extractExifData]);

  const handleGeneratePlugin = useCallback(async () => {
    if (!selectedFile || !selectedPluginManifest || !selectedPluginUiModule) {
      return;
    }

    setIsGeneratingPlugin(true);
    setPluginProgress({ processed: 0, total: 100 });

    try {
      const normalizedParams = selectedPluginUiModule.normalizeParams
        ? selectedPluginUiModule.normalizeParams(pluginParams)
        : pluginParams;

      const result = await workerClient.runStudioPlugin(
        selectedPluginManifest,
        selectedFile,
        normalizedParams,
      );

      const resultUrl = URL.createObjectURL(result.blob);
      setPluginResultUrl((prev) => {
        if (prev) {
          URL.revokeObjectURL(prev);
        }
        return resultUrl;
      });

      showMessage("success", `Plugin processing complete: ${result.fileName}`);
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Plugin processing failed";
      setPluginProgress((prev) =>
        prev
          ? {
              ...prev,
              error: message,
              failedAt: prev.processed,
            }
          : null,
      );
      showMessage("error", message);
    } finally {
      setIsGeneratingPlugin(false);
    }
  }, [
    selectedFile,
    selectedPluginManifest,
    selectedPluginUiModule,
    pluginParams,
    workerClient,
    showMessage,
  ]);

  const handleCancelPluginGeneration = useCallback(() => {
    setIsCancellingPlugin(true);
    try {
      workerClient.abortStudioPlugin();
      setIsGeneratingPlugin(false);
      setPluginProgress(null);
      showMessage("info", "Plugin generation has been cancelled.");
    } finally {
      setIsCancellingPlugin(false);
    }
  }, [workerClient, showMessage]);

  const handleCancelExtraction = useCallback(() => {
    setIsCancellingExif(true);
    try {
      cancelExtraction();
    } finally {
      setIsCancellingExif(false);
    }
  }, [cancelExtraction]);

  const handleInstallPlugin = useCallback(
    (pluginId: string, version: string) => {
      install(pluginId, version);
      setSelectedPluginId(pluginId);
    },
    [install],
  );

  const handleUninstallPlugin = useCallback(
    (pluginId: string) => {
      uninstall(pluginId);
      if (selectedPluginId === pluginId) {
        setSelectedPluginId(null);
        setSelectedPluginManifest(null);
        setSelectedPluginUiModule(null);
        setPluginParams({});
        setPluginError(null);
      }
    },
    [uninstall, selectedPluginId],
  );

  const handleSelectPlugin = useCallback((pluginId: string) => {
    setSelectedPluginId(pluginId || null);
  }, []);
  //#endregion

  const triggerFileInput = () => {
    fileInputRef.current?.click();
  };

  // Display priority: plugin output > original
  const displayUrl = pluginResultUrl || imageUrl;

  return (
    <div className="flex flex-col h-[calc(85vh)] bg-base-100 overflow-hidden">
      <StudioHeader
        onOpenFile={triggerFileInput}
        fileInputRef={fileInputRef}
        onFileChange={handleFileChange}
      />

      <div className="flex-1 flex overflow-hidden">
        <StudioSidebar
          activePanel={activePanel}
          setActivePanel={setActivePanel}
          isCollapsed={sidebarCollapsed}
          onToggle={() => setSidebarCollapsed(!sidebarCollapsed)}
        />

        <StudioViewport
          key={displayUrl || "empty"}
          imageUrl={displayUrl}
          selectedFile={selectedFile}
          onOpenFile={triggerFileInput}
        />

        <StudioToolsPanel
          selectedFile={selectedFile}
          activePanel={activePanel}
          isExtracting={isExtracting}
          exifProgress={exifProgress}
          exifToDisplay={exifToDisplay}
          onExtractExif={handleExtractExif}
          isGeneratingPlugin={isGeneratingPlugin}
          pluginProgress={pluginProgress}
          onGeneratePlugin={handleGeneratePlugin}
          pluginRuntimeEnabled={pluginRuntimeEnabled}
          installedPlugins={installed}
          catalogPlugins={pluginCatalog}
          selectedPluginId={selectedPluginId}
          onSelectPlugin={handleSelectPlugin}
          onInstallPlugin={handleInstallPlugin}
          onUninstallPlugin={handleUninstallPlugin}
          isPluginInstalled={isInstalled}
          pluginUiModule={selectedPluginUiModule}
          pluginParams={pluginParams}
          onPluginParamsChange={setPluginParams}
          pluginLoading={pluginLoading}
          pluginError={pluginError}
          catalogLoading={isCatalogLoading}
          catalogError={catalogError}
          onCancelPluginGeneration={handleCancelPluginGeneration}
          isCancellingPlugin={isCancellingPlugin}
          onCancelExtraction={handleCancelExtraction}
          isCancellingExif={isCancellingExif}
        />
      </div>
    </div>
  );
}
