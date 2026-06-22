import type { AgentRefDTO } from "../types";

type Facets = NonNullable<AgentRefDTO["facets"]>;

export type FacetBucket = NonNullable<Facets["histogram"]>[number];
export type FacetGranularity = NonNullable<Facets["histogram_granularity"]>;

export const granularityFallbacks: Record<FacetGranularity, string> = {
  hour: "By hour",
  day: "By day",
  month: "By month",
  year: "By year",
};

export function inferFacetGranularity(buckets: FacetBucket[]): FacetGranularity {
  const first = buckets[0]?.bucket ?? "";
  if (/^\d{4}-\d{2}-\d{2} \d{2}:00$/.test(first)) return "hour";
  if (/^\d{4}-\d{2}-\d{2}$/.test(first)) return "day";
  if (/^\d{4}-\d{2}$/.test(first)) return "month";
  return "year";
}

export function compressFacetBuckets(buckets: FacetBucket[], maxBuckets: number): FacetBucket[] {
  if (buckets.length <= maxBuckets) return buckets;
  if (maxBuckets <= 1) return [buckets[buckets.length - 1]];
  const lastIndex = buckets.length - 1;
  const out: FacetBucket[] = [];
  for (let i = 0; i < maxBuckets; i += 1) {
    const bucket = buckets[Math.round((i / (maxBuckets - 1)) * lastIndex)];
    if (out[out.length - 1]?.bucket !== bucket.bucket) {
      out.push(bucket);
    }
  }
  return out;
}

export function shortTimeBucket(
  bucket: string | undefined,
  granularity: FacetGranularity,
  locale?: string,
): string {
  if (!bucket) return "";
  const date = parseBucketDate(bucket, granularity);
  if (!date) return bucket;
  if (granularity === "hour") {
    return new Intl.DateTimeFormat(locale, { hour: "numeric" }).format(date).replace(/\s/g, "");
  }
  if (granularity === "day") {
    return new Intl.DateTimeFormat(locale, { day: "numeric" }).format(date);
  }
  if (granularity === "month") {
    return new Intl.DateTimeFormat(locale, { month: "short" }).format(date);
  }
  return new Intl.DateTimeFormat(locale, { year: "numeric" }).format(date);
}

/** Start instant of a histogram bucket, used to place assets onto the
 * timeline by nearest capture time. */
export function bucketStartDate(
  bucket: string | undefined,
  granularity: FacetGranularity,
): Date | null {
  if (!bucket) return null;
  return parseBucketDate(bucket, granularity);
}

export function formatTimeBucketTitle(
  bucket: string | undefined,
  granularity: FacetGranularity,
  locale?: string,
): string {
  if (!bucket) return "";
  const date = parseBucketDate(bucket, granularity);
  if (!date) return bucket;
  const options: Intl.DateTimeFormatOptions =
    granularity === "hour"
      ? { month: "short", day: "numeric", hour: "numeric" }
      : granularity === "day"
        ? { year: "numeric", month: "short", day: "numeric" }
        : granularity === "month"
          ? { year: "numeric", month: "short" }
          : { year: "numeric" };
  return new Intl.DateTimeFormat(locale, options).format(date);
}

function parseBucketDate(bucket: string, granularity: FacetGranularity): Date | null {
  if (granularity === "year") {
    const match = /^(\d{4})$/.exec(bucket);
    return match ? new Date(Number(match[1]), 0, 1) : null;
  }
  const match = /^(\d{4})-(\d{2})(?:-(\d{2})(?: (\d{2}):00)?)?$/.exec(bucket);
  if (!match) return null;
  const [, year, month, day = "01", hour = "0"] = match;
  return new Date(Number(year), Number(month) - 1, Number(day), Number(hour));
}
