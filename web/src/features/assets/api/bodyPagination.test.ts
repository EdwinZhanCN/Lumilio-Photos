import { describe, expect, it } from "vite-plus/test";
import { withBodyPaginationOffset } from "./bodyPagination";

describe("withBodyPaginationOffset", () => {
  it("writes the infinite-query offset into the JSON pagination body", () => {
    const request = {
      query: "",
      pagination: { limit: 50, offset: 0 },
    };

    expect(withBodyPaginationOffset(request, 100)).toEqual({
      query: "",
      pagination: { limit: 50, offset: 100 },
    });
    expect(request.pagination.offset).toBe(0);
  });
});
