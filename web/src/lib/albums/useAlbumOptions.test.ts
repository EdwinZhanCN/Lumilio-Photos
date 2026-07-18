import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";
import { useAlbumOptions } from "./useAlbumOptions";

const mocks = vi.hoisted(() => ({
  useQuery: vi.fn(() => ({ data: undefined })),
}));

vi.mock("@/lib/http-commons/queryClient", () => ({
  $api: { useQuery: mocks.useQuery },
}));

describe("useAlbumOptions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("owns the bounded shared album-options query contract", () => {
    renderHook(() => useAlbumOptions(false));

    expect(mocks.useQuery).toHaveBeenCalledWith(
      "get",
      "/api/v1/albums",
      { params: { query: { limit: 100, offset: 0 } } },
      { enabled: false, staleTime: 60_000, gcTime: 5 * 60_000 },
    );
  });
});
