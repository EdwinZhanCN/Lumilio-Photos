import { describe, expect, it } from "vite-plus/test";
import { formatSpeciesScore, getSpeciesScorePercent, parseSpeciesPrediction } from "./fieldGuide";

describe("field guide species label parsing", () => {
  it("uses common name when the eighth taxonomy slot is present", () => {
    const parsed = parseSpeciesPrediction({
      label:
        "Animalia / Chordata / Mammalia / Artiodactyla / Cervidae / Rucervus / duvaucelii / Barasingha",
      score: 0.94,
    });

    expect(parsed?.displayName).toBe("Barasingha");
    expect(parsed?.commonName).toBe("Barasingha");
    expect(parsed?.scientificName).toBe("Rucervus duvaucelii");
    expect(parsed?.taxonomy.kingdom).toBe("Animalia");
    expect(parsed?.taxonomy.species).toBe("Rucervus duvaucelii");
  });

  it("falls back to scientific name and ignores placeholder slots", () => {
    const parsed = parseSpeciesPrediction({
      label: "Animalia / * / * / Artiodactyla / * / Rucervus / duvaucelii / *",
      score: 0.88,
    });

    expect(parsed?.displayName).toBe("Rucervus duvaucelii");
    expect(parsed?.taxonomy.phylum).toBeUndefined();
    expect(parsed?.taxonomy.order).toBe("Artiodactyla");
    expect(parsed?.taxonomy.family).toBeUndefined();
  });

  it("formats SigLIP-style scores for labels and progress bars", () => {
    expect(formatSpeciesScore(0.9412)).toBe("0.94");
    expect(getSpeciesScorePercent(0.9412)).toBeCloseTo(94.12);
    expect(getSpeciesScorePercent(120)).toBe(100);
  });
});
