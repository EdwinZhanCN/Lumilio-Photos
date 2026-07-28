import { describe, expect, it } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import type { MediaCompositionFacts } from "../../../../types";
import MediaCompositionBadges from "./MediaCompositionBadges";

const facts = (overrides: Partial<MediaCompositionFacts> = {}): MediaCompositionFacts => ({
  componentCount: 1,
  hasRaw: false,
  hasJpeg: true,
  hasEdited: false,
  hasLiveMotion: false,
  ...overrides,
});

describe("MediaCompositionBadges", () => {
  it("renders nothing for a plain single-component media item", async () => {
    const screen = await renderWithProviders(<MediaCompositionBadges composition={facts()} />, {
      router: false,
    });

    await expect.element(screen.getByText("RAW")).not.toBeInTheDocument();
    await expect.element(screen.getByText("JPEG+RAW")).not.toBeInTheDocument();
  });

  it("labels an unpaired RAW as RAW and a paired one as JPEG+RAW", async () => {
    const unpaired = await renderWithProviders(
      <MediaCompositionBadges composition={facts({ hasRaw: true, hasJpeg: false })} />,
      { router: false },
    );
    await expect.element(unpaired.getByText("RAW")).toBeVisible();

    const paired = await renderWithProviders(
      <MediaCompositionBadges composition={facts({ hasRaw: true, hasJpeg: true })} />,
      { router: false },
    );
    await expect.element(paired.getByText("JPEG+RAW")).toBeVisible();
  });

  it("shows live and edited badges alongside composition", async () => {
    const screen = await renderWithProviders(
      <MediaCompositionBadges
        composition={facts({ hasRaw: true, hasLiveMotion: true, hasEdited: true })}
      />,
      { router: false },
    );

    await expect.element(screen.getByText("JPEG+RAW")).toBeVisible();
    await expect.element(screen.getByText("LIVE")).toBeVisible();
    await expect.element(screen.getByText("EDITED")).toBeVisible();
  });

  it("renders nothing when composition facts are missing", async () => {
    const screen = await renderWithProviders(<MediaCompositionBadges composition={undefined} />, {
      router: false,
    });

    await expect.element(screen.getByText("LIVE")).not.toBeInTheDocument();
  });
});
