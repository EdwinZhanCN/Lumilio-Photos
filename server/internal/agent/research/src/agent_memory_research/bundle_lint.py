from __future__ import annotations

from typing import Any

from .generation_matrix import ALLOWED_OUTPUT_KINDS, INTENT_VOCAB, SCENARIO_VOCAB

ENTITY_TYPE_NAMES = {
    "location",
    "album",
    "camera_model",
    "failure_mode",
    "rating",
    "liked_state",
    "time_window",
}


def lint_bundle(payload: dict[str, Any]) -> list[str]:
    issues: list[str] = []
    episodes = list(payload.get("episodes", []))
    queries = list(payload.get("queries", []))
    episode_targets: set[tuple[str, str]] = set()
    query_targets: set[tuple[str, str]] = set()
    episode_ids: set[str] = set()
    episode_by_id: dict[str, dict[str, Any]] = {}

    for index, episode in enumerate(episodes):
        episode_id = str(episode.get("episode_id", "")).strip()
        cluster_id = str(episode.get("cluster_id", "")).strip()
        if not episode_id:
            issues.append(f"episodes[{index}].episode_id is required")
        elif episode_id in episode_ids:
            issues.append(f"episodes[{index}].episode_id={episode_id!r} is duplicated")
        else:
            episode_ids.add(episode_id)
            episode_by_id[episode_id] = episode
        if not cluster_id:
            issues.append(f"episodes[{index}].cluster_id is required")

        scenario = str(episode.get("scenario", "")).strip()
        intent = str(episode.get("intent", "")).strip()
        if scenario and intent:
            episode_targets.add((scenario, intent))
        if scenario not in SCENARIO_VOCAB:
            issues.append(
                f"episodes[{index}].scenario={scenario!r} is outside the controlled scenario vocabulary"
            )
        if intent not in INTENT_VOCAB:
            issues.append(
                f"episodes[{index}].intent={intent!r} is outside the controlled intent vocabulary"
            )
        if cluster_id and scenario and not cluster_id.startswith(f"{scenario}_"):
            issues.append(
                f"episodes[{index}].cluster_id={cluster_id!r} does not align with scenario {scenario!r}"
            )

        previous_tool = None
        repeat_count = 0
        for step_index, step in enumerate(episode.get("steps", [])):
            tool_name = str(step.get("tool_name", "")).strip()
            output_kind = str(step.get("output_kind", "")).strip()
            expected_output_kind = ALLOWED_OUTPUT_KINDS.get(tool_name)
            if expected_output_kind is None:
                issues.append(
                    f"episodes[{index}].steps[{step_index}].tool_name={tool_name!r} is not allowed"
                )
            elif output_kind != expected_output_kind:
                issues.append(
                    f"episodes[{index}].steps[{step_index}] output_kind mismatch: "
                    f"expected {expected_output_kind!r}, got {output_kind!r}"
                )

            if tool_name == previous_tool:
                repeat_count += 1
            else:
                previous_tool = tool_name
                repeat_count = 1
            if repeat_count >= 3:
                issues.append(
                    f"episodes[{index}] repeats tool {tool_name!r} {repeat_count} times consecutively"
                )

    for index, query in enumerate(queries):
        entity = str(query.get("entity", "")).strip()
        if entity in ENTITY_TYPE_NAMES:
            issues.append(
                f"queries[{index}].entity={entity!r} looks like an entity type, not an entity value"
            )

        target_scenario = str(query.get("target_scenario", "")).strip()
        target_intent = str(query.get("target_intent", "")).strip()
        target_episode_ids = [
            str(value).strip()
            for value in query.get("target_episode_ids", [])
            if str(value).strip()
        ]
        if target_scenario and target_intent:
            query_targets.add((target_scenario, target_intent))
        if target_scenario and target_scenario not in SCENARIO_VOCAB:
            issues.append(
                f"queries[{index}].target_scenario={target_scenario!r} is outside the controlled scenario vocabulary"
            )
        if target_intent and target_intent not in INTENT_VOCAB:
            issues.append(
                f"queries[{index}].target_intent={target_intent!r} is outside the controlled intent vocabulary"
            )
        if not target_episode_ids:
            issues.append(
                f"queries[{index}].target_episode_ids must contain at least one episode_id"
            )
            continue

        for target_episode_id in target_episode_ids:
            episode = episode_by_id.get(target_episode_id)
            if episode is None:
                issues.append(
                    f"queries[{index}].target_episode_ids contains unknown episode_id {target_episode_id!r}"
                )
                continue
            episode_target = (
                str(episode.get("scenario", "")).strip(),
                str(episode.get("intent", "")).strip(),
            )
            query_target = (target_scenario, target_intent)
            if target_scenario and target_intent and episode_target != query_target:
                issues.append(
                    f"queries[{index}] target mismatch: query declares {query_target!r} "
                    f"but episode {target_episode_id!r} belongs to {episode_target!r}"
                )

    missing_query_targets = sorted(episode_targets - query_targets)
    if missing_query_targets:
        issues.append(
            "query coverage is incomplete for scenario+intent groups: "
            + ", ".join(
                f"{scenario}/{intent}" for scenario, intent in missing_query_targets
            )
        )

    return issues
