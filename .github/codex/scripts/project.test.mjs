import assert from "node:assert/strict";
import test from "node:test";

import {
  PROJECT_CONTRACT,
  currentProjectStatus,
  inspectProject,
  planProjectUpdates,
} from "./project.mjs";

function projectFixture() {
  const nodes = [];
  let fieldIndex = 0;
  for (const [name, contract] of Object.entries(PROJECT_CONTRACT)) {
    fieldIndex += 1;
    if (Array.isArray(contract)) {
      nodes.push({
        __typename: "ProjectV2SingleSelectField",
        id: `field-${fieldIndex}`,
        name,
        options: contract.map((option, optionIndex) => ({
          id: `option-${fieldIndex}-${optionIndex}`,
          name: option,
        })),
      });
    } else {
      nodes.push({
        __typename: "ProjectV2Field",
        id: `field-${fieldIndex}`,
        name,
        dataType: contract,
      });
    }
  }
  return { id: "project-1", fields: { nodes } };
}

function itemWithStatus(status) {
  return {
    id: "item-1",
    fieldValues: {
      nodes: status
        ? [{ name: status, field: { name: "Status" } }]
        : [],
    },
  };
}

test("accepts the exact Project field contract", () => {
  const result = inspectProject(projectFixture());
  assert.equal(result.valid, true);
  assert.equal(result.fields.get("Status").name, "Status");
});

test("rejects a missing field, renamed option, or wrong text field type", () => {
  const missing = projectFixture();
  missing.fields.nodes = missing.fields.nodes.filter((field) => field.name !== "Priority");
  assert.match(inspectProject(missing).errors.join("\n"), /Priority/);

  const renamed = projectFixture();
  renamed.fields.nodes.find((field) => field.name === "Track").options[0].name = "Tiny";
  assert.equal(inspectProject(renamed).valid, false);

  const wrongType = projectFixture();
  wrongType.fields.nodes.find((field) => field.name === "Agent run").dataType = "NUMBER";
  assert.equal(inspectProject(wrongType).valid, false);
});

test("finds the current Project Status by field name", () => {
  assert.equal(currentProjectStatus(itemWithStatus("Human review")), "Human review");
  assert.equal(currentProjectStatus(itemWithStatus(null)), null);
});

test("bootstraps a new item through Inbox before Triaging", () => {
  const result = planProjectUpdates({
    project: projectFixture(),
    item: itemWithStatus(null),
    updates: { Status: "Triaging", "Agent run": "https://example.test/run/1" },
    bootstrapStatus: true,
  });
  assert.equal(result.valid, true);
  assert.equal(result.operations.length, 3);
  assert.deepEqual(result.operations.at(-1).value, { text: "https://example.test/run/1" });
});

test("Project retries are idempotent and invalid skips are blocked", () => {
  const retry = planProjectUpdates({
    project: projectFixture(),
    item: itemWithStatus("Triaging"),
    updates: { Status: "Triaging" },
  });
  assert.deepEqual(retry, { valid: true, errors: [], operations: [] });

  const skipped = planProjectUpdates({
    project: projectFixture(),
    item: itemWithStatus("Inbox"),
    updates: { Status: "PR review" },
  });
  assert.equal(skipped.valid, false);
  assert.match(skipped.errors.join("\n"), /Inbox -> PR review/);
});

test("new items cannot bypass Inbox", () => {
  const result = planProjectUpdates({
    project: projectFixture(),
    item: itemWithStatus(null),
    updates: { Status: "Human review" },
  });
  assert.equal(result.valid, false);
  assert.match(result.errors.join("\n"), /enter through Inbox/);
});
