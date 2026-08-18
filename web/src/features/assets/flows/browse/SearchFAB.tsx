import {
  useEffect,
  useRef,
  useState,
  type ChangeEvent,
  type ClipboardEvent,
  type DragEvent,
  type FormEvent,
  type MouseEvent,
} from "react";
import { FolderOpen, Image as ImageIcon, Search, X } from "lucide-react";
import Modal from "@/components/ui/Modal";
import { $api } from "@/lib/http-commons/queryClient";
import { useCapabilities } from "@/lib/capabilities/useCapabilities";
import { useI18n } from "@/lib/i18n";
import {
  classifySearchError,
  isSearchByImageFile,
  searchByImageAccept,
} from "../../model/visualSearch";
import PhotoPicker from "../../picker/PhotoPicker";

interface SearchFABProps {
  className?: string;
  query: string;
  similarAssetId?: string;
  fileQuery?: File | null;
  searchError?: string | null;
  onQueryChange: (query: string) => void;
  onSimilarChange: (assetId: string | null) => void;
  onFileQueryChange: (file: File | null) => void;
}

function firstImageFile(list: FileList | DataTransferItemList | null | undefined): File | null {
  if (!list || list.length === 0) return null;
  if (list[0] instanceof File) {
    return Array.from(list as FileList).find((file) => isSearchByImageFile(file)) ?? null;
  }
  for (let i = 0; i < list.length; i += 1) {
    const item = (list as DataTransferItemList)[i];
    if (item.kind !== "file") continue;
    const file = item.getAsFile();
    if (file && isSearchByImageFile(file)) return file;
  }
  return null;
}

export function SearchFAB({
  className,
  query,
  similarAssetId = "",
  fileQuery = null,
  searchError = null,
  onQueryChange,
  onSimilarChange,
  onFileQueryChange,
}: SearchFABProps) {
  const { t } = useI18n();
  const { capabilities } = useCapabilities();
  const imageEmbed = capabilities?.ml.tasks.clipImageEmbed ?? { enabled: false, available: false };
  const pickerEnabled = imageEmbed.enabled;
  const fileEnabled = imageEmbed.enabled && imageEmbed.available;
  const [localValue, setLocalValue] = useState(query);
  const [imageMode, setImageMode] = useState(Boolean(similarAssetId || fileQuery));
  const [pickerOpen, setPickerOpen] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const similarQuery = $api.useQuery(
    "get",
    "/api/v1/assets/{id}",
    { params: { path: { id: similarAssetId } } },
    { enabled: Boolean(similarAssetId) },
  );

  useEffect(() => {
    setLocalValue(query);
  }, [query]);

  useEffect(() => {
    if (similarAssetId || fileQuery) setImageMode(true);
  }, [fileQuery, similarAssetId]);

  useEffect(() => {
    if (localValue.trim() === query.trim()) return;
    const timer = window.setTimeout(() => onQueryChange(localValue), 300);
    return () => window.clearTimeout(timer);
  }, [localValue, onQueryChange, query]);

  const hasTextSearch = localValue.length > 0;
  const hasImageQuery = Boolean(similarAssetId || fileQuery);
  const fabOpen = hasTextSearch || imageMode || hasImageQuery;
  const errorKind = searchError ? classifySearchError(searchError) : null;
  const wellMessage = !pickerEnabled
    ? t("assets.searchByImageDisabled", "Image Semantic Analysis is off.")
    : errorKind === "invalid_image"
      ? t("assets.searchByImageInvalid", "That file could not be used as a search image.")
      : errorKind === "unavailable"
        ? t("assets.searchByImageUnavailable", "Image Semantic Analysis is currently unreachable.")
        : errorKind === "embedding_missing"
          ? t(
              "assets.searchByImageMissingEmbedding",
              "This photo has no Image Semantic Analysis embedding yet.",
            )
          : null;

  const fromRepositoryLabel = t("assets.searchByImageFromLibrary", "Choose from Repository");
  const chipName =
    fileQuery?.name ??
    similarQuery.data?.original_filename ??
    t("assets.searchByImageChipFallback", "Selected photo");
  const fileActionLabel = t("assets.searchByImageFile", "Search with a file");
  const fileActionTip = fileEnabled
    ? fileActionLabel
    : (wellMessage ??
      t("assets.searchByImageUnavailable", "Image Semantic Analysis is currently unreachable."));

  const enterImageMode = () => {
    setImageMode(true);
    onQueryChange("");
    setLocalValue("");
  };

  const leaveImageMode = () => {
    setImageMode(false);
    setPickerOpen(false);
    onSimilarChange(null);
    onFileQueryChange(null);
  };

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    setLocalValue(e.target.value);
  };

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    onQueryChange(localValue);
  };

  const handleClearInput = () => {
    setLocalValue("");
    onQueryChange("");
    inputRef.current?.focus();
  };

  const handleClose = () => {
    setLocalValue("");
    setImageMode(false);
    setPickerOpen(false);
    onQueryChange("");
    onSimilarChange(null);
    onFileQueryChange(null);
  };

  const handleToggleImageMode = () => {
    if (imageMode) leaveImageMode();
    else enterImageMode();
  };

  const handleWellClick = () => {
    if (!pickerEnabled) return;
    setPickerOpen(true);
  };

  const handleFileChosen = (file: File | null) => {
    if (!file || !fileEnabled || !isSearchByImageFile(file)) return;
    onSimilarChange(null);
    onFileQueryChange(file);
  };

  const handleDrop = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    handleFileChosen(
      firstImageFile(event.dataTransfer.files) ?? firstImageFile(event.dataTransfer.items),
    );
  };

  const handlePaste = (event: ClipboardEvent<HTMLDivElement>) => {
    const file =
      firstImageFile(event.clipboardData.items) ?? firstImageFile(event.clipboardData.files);
    if (!file) return;
    event.preventDefault();
    handleFileChosen(file);
  };

  const handleClearImageQuery = (event: MouseEvent) => {
    event.stopPropagation();
    onSimilarChange(null);
    onFileQueryChange(null);
  };

  return (
    <div className="pointer-events-none fixed inset-0 z-overlay isolate">
      {fabOpen && (
        <div
          aria-hidden="true"
          className="absolute bottom-0 right-0 h-[40vh] w-[28rem]"
          style={{
            backdropFilter: "blur(4px)",
            WebkitBackdropFilter: "blur(4px)",
            maskImage: `
              linear-gradient(to bottom, transparent 0%, black 45%),
              linear-gradient(to right, transparent 0%, black 35%)
            `,
            WebkitMaskImage: `
              linear-gradient(to bottom, transparent 0%, black 45%),
              linear-gradient(to right, transparent 0%, black 35%)
            `,
            maskComposite: "intersect",
            WebkitMaskComposite: "source-in",
          }}
        />
      )}

      <div
        className={`fab pointer-events-auto absolute bottom-6 right-4 flex items-end gap-2 ${fabOpen ? "fab-open" : ""} ${className ?? ""}`}
        style={{ flexDirection: "row-reverse" }}
      >
        <div
          tabIndex={0}
          role="button"
          aria-label={t("assets.searchAriaLabel", "Search assets")}
          onClick={() => inputRef.current?.focus()}
          className="btn btn-circle btn-soft btn-lg btn-info"
        >
          <Search className="size-5" />
        </div>

        <div className="fab-close" onClick={handleClose}>
          <span className="btn btn-circle btn-lg btn-error">
            <X className="size-5" />
          </span>
        </div>

        <div>
          <div className="flex shrink-0 flex-col items-end gap-1">
            <button
              type="button"
              aria-pressed={imageMode}
              className={`btn btn-sm ${imageMode ? "btn-info" : "btn-soft btn-info"}`}
              onClick={handleToggleImageMode}
            >
              {t("assets.searchByImage", "Search by image")}
            </button>

            <div className="flex items-center gap-2">
              <div
                id="gallery-search-slot"
                className="relative shrink-0"
                style={{ width: "18rem", minWidth: "18rem", maxWidth: "calc(100vw - 6rem)" }}
                onDragOver={(event) => event.preventDefault()}
                onDrop={handleDrop}
                onPaste={handlePaste}
              >
                {imageMode ? (
                  <>
                    <button
                      type="button"
                      disabled={!pickerEnabled}
                      aria-label={hasImageQuery ? chipName : fromRepositoryLabel}
                      className={`btn btn-lg btn-primary btn-block rounded-full h-12 min-h-12 gap-2 px-4 ${hasImageQuery ? "justify-start pr-10" : ""}`}
                      style={{ width: "100%" }}
                      onClick={handleWellClick}
                    >
                      <ImageIcon className="size-5 shrink-0" aria-hidden />
                      <span className="min-w-0 flex-1 truncate">
                        {hasImageQuery ? chipName : fromRepositoryLabel}
                      </span>
                    </button>
                    {hasImageQuery && (
                      <button
                        type="button"
                        aria-label={t("assets.clearImageQuery", "Clear image")}
                        className="absolute top-1/2 right-3 z-10 -translate-y-1/2 text-primary-content/70 hover:text-primary-content"
                        onClick={handleClearImageQuery}
                      >
                        <X className="size-4" />
                      </button>
                    )}
                  </>
                ) : (
                  <form onSubmit={handleSubmit} role="search" className="w-full">
                    <div className="relative flex w-full items-center">
                      <input
                        ref={inputRef}
                        id="gallery-search-input"
                        type="search"
                        role="searchbox"
                        aria-label={t("assets.searchAriaLabel", "Search assets")}
                        value={localValue}
                        onChange={handleChange}
                        placeholder={t("assets.searchPlaceholder", "Search assets...")}
                        className="input input-lg input-bordered rounded-full w-full min-h-12 bg-base-100 shadow-md text-sm"
                      />
                      {localValue && (
                        <button
                          type="button"
                          aria-label={t("assets.clearSearchAriaLabel", "Clear search")}
                          onClick={handleClearInput}
                          className="absolute right-3 top-1/2 -translate-y-1/2 text-base-content/40 hover:text-base-content"
                        >
                          <X className="size-4" />
                        </button>
                      )}
                    </div>
                  </form>
                )}
                {imageMode && wellMessage && hasImageQuery && (
                  <p className="absolute top-full right-0 mt-1 max-w-[18rem] text-right text-xs text-warning">
                    {wellMessage}
                  </p>
                )}
              </div>

              {imageMode && (
                <div className="tooltip tooltip-top" data-tip={fileActionTip}>
                  <button
                    type="button"
                    disabled={!fileEnabled}
                    aria-label={fileActionLabel}
                    className="btn btn-circle btn-lg shrink-0"
                    onClick={() => fileInputRef.current?.click()}
                  >
                    <FolderOpen className="size-5" />
                  </button>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      <input
        ref={fileInputRef}
        type="file"
        accept={searchByImageAccept}
        className="hidden"
        onChange={(event) => {
          handleFileChosen(event.target.files?.[0] ?? null);
          event.target.value = "";
        }}
      />

      <Modal
        open={pickerOpen}
        onClose={() => setPickerOpen(false)}
        size="xl"
        icon={<ImageIcon size={20} />}
        title={t("assets.searchByImagePickerTitle", "Choose a photo")}
        className="h-[min(820px,90vh)]"
        bodyClassName="overflow-hidden"
      >
        <PhotoPicker
          scopeId="photo-picker:search-by-image"
          selectionMode="single"
          typeFilter="PHOTO"
          title={t("assets.searchByImagePickerTitle", "Choose a photo")}
          onSelect={(id) => {
            onFileQueryChange(null);
            onSimilarChange(id);
            setPickerOpen(false);
          }}
        />
      </Modal>
    </div>
  );
}
