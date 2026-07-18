import type { Asset } from "@/lib/assets/types";
import type { AgentRefAssetsDTO, AgentRefDTO } from "../../types";
import type { WidgetSource } from "./types";

type MockWidgetDataset = {
  id: string;
  title: string;
  description: string;
  count: number;
  metadata: AgentRefDTO;
  assets: Asset[];
};

const baseDate = new Date("2025-09-28T08:00:00Z");

export const mockWidgetDatasets: MockWidgetDataset[] = [
  {
    id: "travel-day",
    title: "Kyoto day trip",
    description: "Single-day set with hourly rhythm and a few people/place facets.",
    count: 42,
    metadata: makeMetadata({
      refId: "mock_travel_day",
      count: 42,
      granularity: "hour",
      histogram: [
        ["2025-09-28 08:00", 2],
        ["2025-09-28 09:00", 5],
        ["2025-09-28 10:00", 8],
        ["2025-09-28 11:00", 4],
        ["2025-09-28 13:00", 9],
        ["2025-09-28 14:00", 6],
        ["2025-09-28 17:00", 8],
      ],
      from: "2025-09-28T08:12:00Z",
      to: "2025-09-28T17:42:00Z",
      topPlaces: [
        ["Gion", 18],
        ["Kiyomizu", 12],
        ["Kamogawa", 7],
      ],
      topPeople: [
        ["詹子毅", 8],
        ["Ming", 5],
      ],
      cameras: [["NIKON Z fc", 42]],
    }),
    assets: makeAssets("travel-day", 42, baseDate),
  },
  {
    id: "travel-month",
    title: "September travel album",
    description: "One-month result that switches the histogram to daily buckets.",
    count: 360,
    metadata: makeMetadata({
      refId: "mock_travel_month",
      count: 360,
      granularity: "day",
      histogram: [
        ["2025-09-03", 18],
        ["2025-09-06", 24],
        ["2025-09-09", 44],
        ["2025-09-12", 27],
        ["2025-09-16", 51],
        ["2025-09-19", 36],
        ["2025-09-22", 62],
        ["2025-09-28", 98],
      ],
      from: "2025-09-03T09:10:00Z",
      to: "2025-09-28T21:20:00Z",
      topPlaces: [
        ["Kyoto", 164],
        ["Osaka", 96],
        ["Nara", 71],
      ],
      topPeople: [
        ["詹子毅", 5],
        ["Ari", 4],
      ],
      cameras: [["NIKON Z fc", 360]],
    }),
    assets: makeAssets("travel-month", 72, new Date("2025-09-03T09:00:00Z")),
  },
  {
    id: "archive-years",
    title: "Family archive",
    description: "Long-range archive that collapses into year buckets.",
    count: 1280,
    metadata: makeMetadata({
      refId: "mock_archive_years",
      count: 1280,
      granularity: "year",
      histogram: [
        ["2019", 120],
        ["2020", 86],
        ["2021", 210],
        ["2022", 172],
        ["2023", 260],
        ["2024", 318],
        ["2025", 114],
      ],
      from: "2019-02-14T10:00:00Z",
      to: "2025-12-24T20:00:00Z",
      topPlaces: [
        ["Home", 410],
        ["Tokyo", 133],
        ["Seattle", 88],
      ],
      topPeople: [
        ["Mom", 240],
        ["Dad", 198],
        ["詹子毅", 156],
      ],
      cameras: [
        ["iPhone 15 Pro", 780],
        ["NIKON Z fc", 320],
      ],
    }),
    assets: makeAssets("archive-years", 90, new Date("2019-02-14T10:00:00Z")),
  },
];

export function isMockWidgetSource(
  source: WidgetSource,
): source is { kind: "mock"; mockId: string } {
  return source.kind === "mock";
}

export function getMockWidgetDataset(mockId: string): MockWidgetDataset {
  return mockWidgetDatasets.find((dataset) => dataset.id === mockId) ?? mockWidgetDatasets[0];
}

export function getMockWidgetAssetsPage(
  mockId: string,
  limit: number,
  offset = 0,
): AgentRefAssetsDTO {
  const dataset = getMockWidgetDataset(mockId);
  const assets = dataset.assets.slice(offset, offset + limit);
  return {
    assets,
    total: dataset.count,
    pagination: {
      limit,
      offset,
    },
  };
}

function makeMetadata({
  refId,
  count,
  granularity,
  histogram,
  from,
  to,
  topPlaces,
  topPeople,
  cameras,
}: {
  refId: string;
  count: number;
  granularity: NonNullable<NonNullable<AgentRefDTO["facets"]>["histogram_granularity"]>;
  histogram: [string, number][];
  from: string;
  to: string;
  topPlaces: [string, number][];
  topPeople: [string, number][];
  cameras: [string, number][];
}): AgentRefDTO {
  return {
    ref_id: refId,
    count,
    created_at: "2026-06-20T05:00:00Z",
    facets: {
      count,
      date_range: { from, to },
      histogram_granularity: granularity,
      histogram: histogram.map(([bucket, bucketCount]) => ({
        bucket,
        count: bucketCount,
      })),
      types: { photo: count },
      top_places: topPlaces.map(([name, placeCount]) => ({
        name,
        count: placeCount,
      })),
      top_people: topPeople.map(([name, personCount]) => ({
        name,
        count: personCount,
      })),
      cameras: cameras.map(([name, cameraCount]) => ({
        name,
        count: cameraCount,
      })),
      liked_count: Math.floor(count * 0.14),
      rating_dist: [count - 18, 2, 4, 5, 5, 2],
    },
  };
}

function makeAssets(prefix: string, count: number, startDate: Date): Asset[] {
  return Array.from({ length: count }, (_, index) => {
    const taken = new Date(startDate);
    taken.setHours(taken.getHours() + index * 3);
    return {
      asset_id: `mock-${prefix}-${index + 1}`,
      original_filename: `${prefix}-${String(index + 1).padStart(3, "0")}.jpg`,
      type: "photo",
      mime_type: "image/jpeg",
      taken_time: taken.toISOString(),
      upload_time: taken.toISOString(),
      width: 4032,
      height: 3024,
      liked: index % 7 === 0,
      rating: index % 9 === 0 ? 5 : index % 5 === 0 ? 4 : 0,
    };
  });
}
