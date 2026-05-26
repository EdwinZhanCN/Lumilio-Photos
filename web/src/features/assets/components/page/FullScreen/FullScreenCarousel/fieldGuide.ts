export type SpeciesPrediction = {
  label?: string;
  score?: number;
};

export type TaxonomyRank =
  | "kingdom"
  | "phylum"
  | "class"
  | "order"
  | "family"
  | "genus"
  | "species";

export const TAXONOMY_RANKS: TaxonomyRank[] = [
  "kingdom",
  "phylum",
  "class",
  "order",
  "family",
  "genus",
  "species",
];

export type ParsedSpeciesPrediction = SpeciesPrediction & {
  displayName: string;
  commonName?: string;
  scientificName?: string;
  taxonomy: Record<TaxonomyRank, string | undefined>;
};

const TAXONOMY_SLOT_COUNT = 8;

function cleanSlot(value: string | undefined): string | undefined {
  const cleaned = value?.trim();
  if (!cleaned || cleaned === "*") {
    return undefined;
  }
  return cleaned;
}

export function normalizeSpeciesPredictions(input: unknown): SpeciesPrediction[] {
  if (typeof input === "string") {
    try {
      return normalizeSpeciesPredictions(JSON.parse(input));
    } catch {
      return [];
    }
  }

  if (!Array.isArray(input)) {
    return [];
  }

  const predictions: SpeciesPrediction[] = [];
  for (const item of input) {
    if (!item || typeof item !== "object") {
      continue;
    }

    const prediction = item as Record<string, unknown>;
    const label =
      typeof prediction.label === "string" ? prediction.label : undefined;
    const score =
      typeof prediction.score === "number" ? prediction.score : undefined;

    if (label) {
      predictions.push({ label, score });
    }
  }

  return predictions;
}

export function parseSpeciesPrediction(
  prediction: SpeciesPrediction,
): ParsedSpeciesPrediction | undefined {
  if (!prediction.label) {
    return undefined;
  }

  const slots = prediction.label
    .split("/")
    .map((slot) => slot.trim())
    .slice(0, TAXONOMY_SLOT_COUNT);
  while (slots.length < TAXONOMY_SLOT_COUNT) {
    slots.push("*");
  }

  const [kingdom, phylum, className, order, family, genus, species, commonName] =
    slots.map(cleanSlot);
  const scientificName =
    genus && species ? `${genus} ${species}` : genus ?? species;
  const fallbackName = [...slots].reverse().map(cleanSlot).find(Boolean);

  return {
    ...prediction,
    displayName: commonName ?? scientificName ?? fallbackName ?? prediction.label,
    commonName,
    scientificName,
    taxonomy: {
      kingdom,
      phylum,
      class: className,
      order,
      family,
      genus,
      species: scientificName ?? species,
    },
  };
}

export function getSpeciesScorePercent(score: number | undefined): number {
  if (score == null || Number.isNaN(score)) {
    return 0;
  }
  const percent = score <= 1 ? score * 100 : score;
  return Math.max(0, Math.min(100, percent));
}

export function formatSpeciesScore(score: number | undefined): string {
  if (score == null || Number.isNaN(score)) {
    return "--";
  }
  return score.toFixed(2);
}
