import { describe, expect, it } from "vitest";
import { summarizePlaces } from "./PlacesRail";

describe("summarizePlaces", () => {
  it("combines clusters for the same locality and uses a weighted center", () => {
    const places = summarizePlaces(
      [
        {
          cluster_id: "a",
          city: "Paris",
          country: "France",
          photo_count: 2,
          centroid_latitude: 48,
          centroid_longitude: 2,
        },
        {
          cluster_id: "b",
          city: "Paris",
          country: "France",
          photo_count: 4,
          centroid_latitude: 49.5,
          centroid_longitude: 3.5,
        },
        {
          cluster_id: "c",
          label: "Unresolved cluster",
          photo_count: 1,
          centroid_latitude: 10,
          centroid_longitude: 20,
        },
      ],
      "Unknown place",
    );

    expect(places).toHaveLength(2);
    expect(places[0]).toMatchObject({
      label: "Paris, France",
      photoCount: 6,
      latitude: 49,
      longitude: 3,
    });
    expect(places[1]).toMatchObject({
      label: "Unresolved cluster",
      photoCount: 1,
    });
  });

  it("uses the localized fallback when a cluster has no place label", () => {
    expect(summarizePlaces([{ cluster_id: "a", photo_count: 1 }], "Unknown place")).toEqual([
      {
        id: "cluster:a",
        label: "Unknown place",
        photoCount: 1,
        latitude: undefined,
        longitude: undefined,
      },
    ]);
  });
});
