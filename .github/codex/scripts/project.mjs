import { transitionDecision } from "./policy.mjs";

export const PROJECT_CONTRACT = Object.freeze({
  Status: Object.freeze([
    "Inbox",
    "Triaging",
    "Needs input",
    "Human review",
    "Ready",
    "Running",
    "PR review",
    "Blocked",
    "Done",
  ]),
  Track: Object.freeze(["Small", "Exec Plan"]),
  Area: Object.freeze([
    "Server",
    "Web",
    "Desktop",
    "Deploy",
    "Docs",
    "Cross-cutting",
    "Unknown",
  ]),
  Priority: Object.freeze(["P0", "P1", "P2", "P3"]),
  "Agent run": "TEXT",
});

const PROJECT_QUERY = `
  query AgentProject($owner: String!, $number: Int!, $contentId: ID!) {
    user(login: $owner) {
      projectV2(number: $number) {
        id
        fields(first: 50) {
          nodes {
            __typename
            ... on ProjectV2SingleSelectField {
              id
              name
              options { id name }
            }
            ... on ProjectV2Field {
              id
              name
              dataType
            }
          }
        }
      }
    }
    node(id: $contentId) {
      ... on Issue {
        projectItems(first: 50) {
          nodes {
            id
            project { id }
            fieldValues(first: 20) {
              nodes {
                ... on ProjectV2ItemFieldSingleSelectValue {
                  name
                  field { ... on ProjectV2SingleSelectField { name } }
                }
              }
            }
          }
        }
      }
      ... on PullRequest {
        projectItems(first: 50) {
          nodes {
            id
            project { id }
            fieldValues(first: 20) {
              nodes {
                ... on ProjectV2ItemFieldSingleSelectValue {
                  name
                  field { ... on ProjectV2SingleSelectField { name } }
                }
              }
            }
          }
        }
      }
    }
  }
`;

const ADD_ITEM_MUTATION = `
  mutation AddAgentProjectItem($projectId: ID!, $contentId: ID!) {
    addProjectV2ItemById(input: { projectId: $projectId, contentId: $contentId }) {
      item { id }
    }
  }
`;

const UPDATE_ITEM_MUTATION = `
  mutation UpdateAgentProjectItem(
    $projectId: ID!
    $itemId: ID!
    $fieldId: ID!
    $value: ProjectV2FieldValue!
  ) {
    updateProjectV2ItemFieldValue(
      input: { projectId: $projectId, itemId: $itemId, fieldId: $fieldId, value: $value }
    ) {
      projectV2Item { id }
    }
  }
`;

export function inspectProject(project) {
  const errors = [];
  if (!project?.id || !Array.isArray(project?.fields?.nodes)) {
    return { valid: false, errors: ["personal Project was not found"], fields: new Map() };
  }

  const fields = new Map(
    project.fields.nodes
      .filter((field) => field?.name && field?.id)
      .map((field) => [field.name, field]),
  );

  for (const [name, contract] of Object.entries(PROJECT_CONTRACT)) {
    const field = fields.get(name);
    if (!field) {
      errors.push(`missing required Project field: ${name}`);
      continue;
    }

    if (Array.isArray(contract)) {
      if (field.__typename !== "ProjectV2SingleSelectField") {
        errors.push(`${name} must be a single-select field`);
        continue;
      }
      const actual = (field.options ?? []).map((option) => option.name).sort();
      const expected = [...contract].sort();
      if (
        actual.length !== expected.length ||
        actual.some((option, index) => option !== expected[index])
      ) {
        errors.push(`${name} options do not match the frozen contract`);
      }
    } else if (field.__typename !== "ProjectV2Field" || field.dataType !== contract) {
      errors.push(`${name} must be a ${contract} field`);
    }
  }

  return { valid: errors.length === 0, errors, fields };
}

export function currentProjectStatus(item) {
  const value = item?.fieldValues?.nodes?.find((node) => node?.field?.name === "Status");
  return value?.name ?? null;
}

export function planProjectUpdates({ project, item, updates, bootstrapStatus = false }) {
  const inspection = inspectProject(project);
  if (!inspection.valid) return { valid: false, errors: inspection.errors, operations: [] };

  const errors = [];
  const operations = [];
  let status = currentProjectStatus(item);
  const desiredStatus = updates.Status;

  if (desiredStatus) {
    const statusField = inspection.fields.get("Status");
    const statusValues = status ? [desiredStatus] : bootstrapStatus ? ["Inbox", desiredStatus] : [desiredStatus];

    for (const next of [...new Set(statusValues)]) {
      if (status === null && next !== "Inbox") {
        errors.push("an item without Status must enter through Inbox");
        break;
      }
      if (status !== null) {
        const transition = transitionDecision(status, next);
        if (!transition.allowed) {
          errors.push(`invalid Project transition: ${status} -> ${next}`);
          break;
        }
        if (transition.idempotent) continue;
      }
      operations.push(singleSelectOperation(statusField, next));
      status = next;
    }
  }

  for (const name of ["Track", "Area", "Priority"]) {
    const value = updates[name];
    if (value === undefined) continue;
    operations.push(singleSelectOperation(inspection.fields.get(name), value, errors));
  }

  if (updates["Agent run"] !== undefined) {
    if (typeof updates["Agent run"] !== "string" || updates["Agent run"].length > 1_000) {
      errors.push("Agent run must be a text value no longer than 1000 characters");
    } else {
      operations.push({
        fieldId: inspection.fields.get("Agent run").id,
        value: { text: updates["Agent run"] },
      });
    }
  }

  return { valid: errors.length === 0, errors, operations };
}

export async function syncProjectItem({
  token,
  owner,
  number,
  contentId,
  updates,
  bootstrapStatus = false,
  fetchImpl = fetch,
}) {
  if (!token) throw new Error("AGENT_PROJECT_TOKEN is required");
  if (!owner) throw new Error("AGENT_PROJECT_OWNER is required");
  if (!Number.isSafeInteger(number) || number <= 0) {
    throw new Error("AGENT_PROJECT_NUMBER must be a positive integer");
  }
  if (!contentId) throw new Error("a GitHub content node ID is required");

  const data = await graphql(
    token,
    PROJECT_QUERY,
    { owner, number, contentId },
    fetchImpl,
  );
  const project = data.user?.projectV2;
  const inspection = inspectProject(project);
  if (!inspection.valid) throw new Error(inspection.errors.join("; "));

  let item = data.node?.projectItems?.nodes?.find((candidate) => candidate.project?.id === project.id);
  if (!item) {
    const added = await graphql(
      token,
      ADD_ITEM_MUTATION,
      { projectId: project.id, contentId },
      fetchImpl,
    );
    item = added.addProjectV2ItemById?.item;
    if (!item?.id) throw new Error("GitHub did not return the added Project item");
  }

  const planned = planProjectUpdates({ project, item, updates, bootstrapStatus });
  if (!planned.valid) throw new Error(planned.errors.join("; "));

  for (const operation of planned.operations) {
    await graphql(
      token,
      UPDATE_ITEM_MUTATION,
      {
        projectId: project.id,
        itemId: item.id,
        fieldId: operation.fieldId,
        value: operation.value,
      },
      fetchImpl,
    );
  }

  return { projectId: project.id, itemId: item.id, operations: planned.operations.length };
}

function singleSelectOperation(field, optionName, errors = []) {
  const option = field?.options?.find((candidate) => candidate.name === optionName);
  if (!option) {
    errors.push(`${field?.name ?? "unknown field"} has no option named ${optionName}`);
    return { fieldId: field?.id ?? "", value: { singleSelectOptionId: "" } };
  }
  return { fieldId: field.id, value: { singleSelectOptionId: option.id } };
}

async function graphql(token, query, variables, fetchImpl) {
  const response = await fetchImpl("https://api.github.com/graphql", {
    method: "POST",
    headers: {
      Accept: "application/vnd.github+json",
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
      "User-Agent": "lumilio-agent-loop",
      "X-GitHub-Api-Version": "2022-11-28",
    },
    body: JSON.stringify({ query, variables }),
  });
  const payload = await response.json();
  if (!response.ok || payload.errors?.length) {
    const details = payload.errors?.map((error) => error.message).join("; ") ?? response.statusText;
    throw new Error(`GitHub GraphQL request failed: ${details}`);
  }
  return payload.data;
}
