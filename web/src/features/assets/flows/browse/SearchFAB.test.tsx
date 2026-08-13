import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import type { components } from "@/lib/http-commons/schema";
import { SearchFAB } from "./SearchFAB";

vi.mock("../../picker/PhotoPicker", () => ({
  default: ({ onSelect }: { onSelect?: (id: string) => void }) => (
    <button type="button" onClick={() => onSelect?.("550e8400-e29b-41d4-a716-446655440000")}>
      pick-photo
    </button>
  ),
}));

type CapabilitiesResponse = components["schemas"]["dto.CapabilitiesResponseDTO"];

const similarId = "550e8400-e29b-41d4-a716-446655440000";

const healthyCapabilities: CapabilitiesResponse = {
  ml: {
    discovery_state: "healthy",
    tasks: {
      semantic_image_embed: { enabled: true, available: true },
      semantic_text_embed: { enabled: true, available: true },
    },
  },
  llm: { availability: "disabled", agent_enabled: false, configured: false },
};

function serveCapabilities(capabilities = healthyCapabilities) {
  worker.use(
    http.get("*/api/v1/capabilities", () => HttpResponse.json(capabilities)),
    http.get("*/api/v1/assets/:id/thumbnail", () => new HttpResponse(null, { status: 204 })),
    http.get("*/api/v1/assets/:id", ({ params }) =>
      HttpResponse.json({
        asset_id: params.id,
        original_filename: "IMG_1234.jpg",
        type: "PHOTO",
      }),
    ),
  );
}

describe("SearchFAB image mode", () => {
  it("morphs the text slot into an image well of the same width", async () => {
    serveCapabilities();
    const screen = await renderWithProviders(
      <SearchFAB
        query=""
        onQueryChange={vi.fn()}
        onSimilarChange={vi.fn()}
        onFileQueryChange={vi.fn()}
      />,
    );

    await screen.getByRole("button", { name: t("assets.searchAriaLabel") }).click();
    const slot = () => document.getElementById("gallery-search-slot") as HTMLElement;
    await expect.element(screen.getByRole("searchbox")).toBeVisible();
    const inputWidth = slot().getBoundingClientRect().width;

    await screen.getByRole("button", { name: t("assets.searchByImage") }).click();
    await expect.element(screen.getByRole("searchbox")).not.toBeInTheDocument();
    const well = screen.getByRole("button", { name: t("assets.searchByImageFromLibrary") });
    await expect.element(well).toBeVisible();
    expect(slot().getBoundingClientRect().width).toBe(inputWidth);
    expect(Math.round((well.element() as HTMLElement).getBoundingClientRect().width)).toBe(
      Math.round(inputWidth),
    );
    await expect
      .element(screen.getByRole("button", { name: t("assets.searchByImage") }))
      .toHaveAttribute("aria-pressed", "true");
    await expect
      .element(screen.getByRole("button", { name: t("assets.searchByImageFile") }))
      .toBeVisible();
  });

  it("opens PhotoPicker from the repository button and not from FolderOpen", async () => {
    serveCapabilities();
    const onSimilarChange = vi.fn();
    const screen = await renderWithProviders(
      <SearchFAB
        query=""
        onQueryChange={vi.fn()}
        onSimilarChange={onSimilarChange}
        onFileQueryChange={vi.fn()}
      />,
    );

    await screen.getByRole("button", { name: t("assets.searchAriaLabel") }).click();
    await screen.getByRole("button", { name: t("assets.searchByImage") }).click();
    await screen.getByRole("button", { name: t("assets.searchByImageFile") }).click();
    await expect.element(screen.getByText("pick-photo")).not.toBeInTheDocument();

    await screen.getByRole("button", { name: t("assets.searchByImageFromLibrary") }).click();
    await screen.getByText("pick-photo").click();
    expect(onSimilarChange).toHaveBeenCalledWith(similarId);
  });

  it("keeps image mode after clearing the query chip", async () => {
    serveCapabilities();
    const onSimilarChange = vi.fn();
    const screen = await renderWithProviders(
      <SearchFAB
        query=""
        similarAssetId={similarId}
        onQueryChange={vi.fn()}
        onSimilarChange={onSimilarChange}
        onFileQueryChange={vi.fn()}
      />,
    );

    await expect.element(screen.getByText("IMG_1234.jpg")).toBeVisible();
    await screen.getByRole("button", { name: t("assets.clearImageQuery") }).click();
    expect(onSimilarChange).toHaveBeenCalledWith(null);
    await expect
      .element(screen.getByRole("button", { name: t("assets.searchByImage") }))
      .toHaveAttribute("aria-pressed", "true");
  });
});
