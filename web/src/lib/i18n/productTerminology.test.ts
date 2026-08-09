import { describe, expect, it } from "vitest";
import en from "@/locales/en/translation.json";
import zh from "@/locales/zh/translation.json";

function translationValues(value: unknown): string[] {
  if (typeof value === "string") return [value];
  if (!value || typeof value !== "object") return [];
  return Object.values(value).flatMap(translationValues);
}

describe("canonical product terminology", () => {
  it("does not expose Library as a Repository synonym", () => {
    const violations = translationValues(en).filter((value) => /\blibrar(?:y|ies)\b/i.test(value));
    expect(violations).toEqual([]);
  });

  it("does not expose 仓库 or 图库 as a 资源库 synonym", () => {
    const violations = translationValues(zh).filter(
      (value) => /仓库|图库/.test(value) && value !== "打开项目仓库",
    );
    expect(violations).toEqual([]);
  });

  it("keeps all four Lumen capability labels canonical", () => {
    const englishLabels = new Set(translationValues(en));
    const chineseLabels = new Set(translationValues(zh));
    for (const label of [
      "Image Semantic Analysis",
      "Person Recognition",
      "OCR Text Recognition",
      "BioCLIP Species Recognition",
    ]) {
      expect(englishLabels.has(label), label).toBe(true);
    }
    for (const label of ["图像语义分析", "人物识别", "OCR文字识别", "BioCLIP物种识别"]) {
      expect(chineseLabels.has(label), label).toBe(true);
    }
    for (const forbidden of ["Semantic Search", "Face Recognition", "OCR", "Species Recognition"]) {
      expect(englishLabels.has(forbidden), forbidden).toBe(false);
    }
    for (const forbidden of ["语义搜索", "人脸识别", "物种识别"]) {
      expect(chineseLabels.has(forbidden), forbidden).toBe(false);
    }
  });

  it("keeps repository removal safety copy explicit in both languages", () => {
    expect(en.manage.repositories.removeSafetyWarning).toBe(
      "Files on disk will be preserved; some metadata in the Lumilio catalog may not be recoverable after reopening this Repository.",
    );
    expect(en.manage.repositories.removeAction).toBe("Remove from Lumilio");
    expect(en.manage.repositories.removeConfirmationLabel).toBe('Type "{{name}}" to confirm');
    expect(zh.manage.repositories.removeSafetyWarning).toBe(
      "磁盘中的文件将保留；流明集目录中的部分元数据可能无法在重新打开后恢复。",
    );
    expect(zh.manage.repositories.removeAction).toBe("从流明集中移除");
    expect(zh.manage.repositories.removeConfirmationLabel).toBe("输入“{{name}}”以确认");
  });
});
