from __future__ import annotations

import json
import random
from collections import Counter
from typing import Any

MEDIA_TOOLS = [
    "mock_filter_assets",
    "mock_group_assets",
    "mock_inspect_asset_metadata",
    "mock_find_duplicate_assets",
    "mock_bulk_like_assets",
    "mock_bulk_archive_assets",
    "mock_create_album",
    "mock_add_assets_to_album",
    "mock_summarize_selection",
]

ALLOWED_OUTPUT_KINDS = {
    "mock_filter_assets": "asset_selection",
    "mock_group_assets": "asset_groups",
    "mock_inspect_asset_metadata": "asset_metadata_report",
    "mock_find_duplicate_assets": "duplicate_report",
    "mock_bulk_like_assets": "bulk_like_update",
    "mock_bulk_archive_assets": "bulk_archive_update",
    "mock_create_album": "album_record",
    "mock_add_assets_to_album": "album_membership_update",
    "mock_summarize_selection": "selection_summary",
}

INTENT_VOCAB = [
    "curate_album",
    "cleanup_duplicates",
    "bulk_like",
    "bulk_archive",
    "inspect_metadata",
    "group_assets",
    "summarize_selection",
]

SCENARIO_VOCAB = [
    "curate_trip_album",
    "cleanup_duplicate_shoot",
    "bulk_like_highlights",
    "archive_low_rated_assets",
    "inspect_camera_metadata",
    "group_assets_for_review",
    "summarize_selected_assets",
]

SCENARIO_SPECS: dict[str, dict[str, Any]] = {
    "curate_trip_album": {
        "intent": "curate_album",
        "statuses": ["succeeded"],
        "cluster_axes": ["time_window", "camera_model", "album_name"],
        "entity_templates": [
            {
                "location": "Paris",
                "camera_model": "Canon EOS R5",
                "time_window": "spring_2024",
                "album_name": "Paris Spring Selects",
            },
            {
                "location": "Yosemite",
                "camera_model": "Sony A7III",
                "time_window": "summer_2023",
                "album_name": "Yosemite Keeps",
            },
            {
                "location": "Tokyo",
                "camera_model": "Fujifilm X-T5",
                "time_window": "autumn_2024",
                "album_name": "Tokyo Street Album",
            },
            {
                "location": "Hawaii",
                "camera_model": "Nikon Z7 II",
                "time_window": "winter_2022",
                "album_name": "Hawaii Blue Hour",
            },
        ],
        "required_tools": [
            "mock_filter_assets",
            "mock_group_assets",
            "mock_create_album",
            "mock_add_assets_to_album",
        ],
        "query_focus": ["album creation", "trip curation", "grouping by date"],
    },
    "cleanup_duplicate_shoot": {
        "intent": "cleanup_duplicates",
        "statuses": ["recovered", "succeeded"],
        "cluster_axes": ["similarity_threshold", "failure_mode", "location"],
        "entity_templates": [
            {
                "location": "Yosemite",
                "camera_model": "Sony A7C II",
                "failure_mode": "false_positive_duplicate",
                "similarity_threshold": "0.82",
            },
            {
                "location": "Johnson Wedding",
                "camera_model": "Canon EOS R6",
                "failure_mode": "false_positive_duplicate",
                "similarity_threshold": "0.79",
            },
            {
                "location": "Portrait Session",
                "camera_model": "Canon EOS R5",
                "failure_mode": "near_duplicate_burst",
                "similarity_threshold": "0.76",
            },
            {
                "location": "Safari Trip",
                "camera_model": "Nikon Z8",
                "failure_mode": "burst_sequence_overlap",
                "similarity_threshold": "0.81",
            },
        ],
        "required_tools": [
            "mock_find_duplicate_assets",
            "mock_inspect_asset_metadata",
            "mock_bulk_archive_assets",
        ],
        "query_focus": [
            "duplicate cleanup",
            "false positive repair",
            "best-shot retention",
        ],
    },
    "bulk_like_highlights": {
        "intent": "bulk_like",
        "statuses": ["succeeded"],
        "cluster_axes": ["rating_threshold", "liked_state", "location"],
        "entity_templates": [
            {
                "location": "Johnson Wedding",
                "rating_threshold": "4",
                "liked_state": "false",
                "album_name": "Wedding Highlights",
            },
            {
                "location": "Paris",
                "rating_threshold": "5",
                "liked_state": "false",
                "album_name": "Paris Favorites",
            },
            {
                "location": "Yosemite",
                "rating_threshold": "4",
                "liked_state": "false",
                "album_name": "Yosemite Picks",
            },
            {
                "location": "Hawaii",
                "rating_threshold": "5",
                "liked_state": "false",
                "album_name": "Hawaii Best Of",
            },
        ],
        "required_tools": ["mock_filter_assets", "mock_bulk_like_assets"],
        "query_focus": ["bulk like", "highlight favorites", "rating threshold"],
    },
    "archive_low_rated_assets": {
        "intent": "bulk_archive",
        "statuses": ["succeeded"],
        "cluster_axes": ["rating_threshold", "time_window", "location"],
        "entity_templates": [
            {
                "location": "Tokyo",
                "rating_threshold": "2",
                "time_window": "spring_2023",
                "album_name": "Tokyo Collection",
            },
            {
                "location": "Johnson Wedding",
                "rating_threshold": "2",
                "time_window": "summer_2024",
                "album_name": "Wedding Cleanup",
            },
            {
                "location": "Hawaii",
                "rating_threshold": "1",
                "time_window": "winter_2022",
                "album_name": "Hawaii Rejects",
            },
            {
                "location": "Family Holiday",
                "rating_threshold": "2",
                "time_window": "autumn_2023",
                "album_name": "Holiday Cull",
            },
        ],
        "required_tools": ["mock_filter_assets", "mock_bulk_archive_assets"],
        "query_focus": ["archive low-rated", "cleanup rejects", "bulk archive"],
    },
    "inspect_camera_metadata": {
        "intent": "inspect_metadata",
        "statuses": ["succeeded"],
        "cluster_axes": ["location", "camera_model", "time_window"],
        "entity_templates": [
            {
                "location": "Yosemite",
                "camera_model": "Canon EOS R5",
                "time_window": "spring_2024",
            },
            {
                "location": "Paris",
                "camera_model": "Canon EOS R5",
                "time_window": "spring_2024",
            },
            {
                "location": "Alps",
                "camera_model": "Sony A7III",
                "time_window": "winter_2023",
            },
            {
                "location": "Tokyo",
                "camera_model": "Nikon Z7 II",
                "time_window": "autumn_2022",
            },
        ],
        "required_tools": ["mock_inspect_asset_metadata"],
        "query_focus": ["camera details", "metadata inspection", "camera settings"],
    },
    "group_assets_for_review": {
        "intent": "group_assets",
        "statuses": ["succeeded"],
        "cluster_axes": ["group_by", "location", "tag_focus"],
        "entity_templates": [
            {
                "location": "Family Holiday",
                "group_by": "camera_model",
                "tag_focus": "family",
            },
            {"location": "Concert", "group_by": "date", "tag_focus": "stage"},
            {"location": "Safari", "group_by": "lens", "tag_focus": "wildlife"},
            {
                "location": "Portrait Session",
                "group_by": "rating",
                "tag_focus": "portrait",
            },
        ],
        "required_tools": ["mock_filter_assets", "mock_group_assets"],
        "query_focus": ["group for review", "review buckets", "grouping criteria"],
    },
    "summarize_selected_assets": {
        "intent": "summarize_selection",
        "statuses": ["succeeded"],
        "cluster_axes": ["selection_theme", "location", "time_window"],
        "entity_templates": [
            {
                "location": "Beach",
                "selection_theme": "sunset",
                "time_window": "summer_2023",
            },
            {
                "location": "Holiday",
                "selection_theme": "family_highlights",
                "time_window": "winter_2024",
            },
            {
                "location": "Paris",
                "selection_theme": "street_moments",
                "time_window": "spring_2024",
            },
            {
                "location": "Yosemite",
                "selection_theme": "landscape_favorites",
                "time_window": "autumn_2023",
            },
        ],
        "required_tools": ["mock_filter_assets", "mock_summarize_selection"],
        "query_focus": ["selection summary", "recap", "what was in the chosen set"],
    },
}

CLUSTER_VARIANTS = (
    ("anchor", None),
    ("near_time", "time_window"),
    ("near_entity", "location"),
    ("near_threshold", "rating_threshold"),
)


def build_generation_plan(
    *,
    episode_count: int,
    query_count: int,
    seed: int,
) -> dict[str, Any]:
    rng = random.Random(seed)
    ordered_scenarios = SCENARIO_VOCAB[:]
    rng.shuffle(ordered_scenarios)
    scenario_cluster_counts = {scenario: 0 for scenario in SCENARIO_VOCAB}

    episode_blueprints: list[dict[str, Any]] = []
    query_blueprints: list[dict[str, Any]] = []

    scenario_pointer = 0
    while len(episode_blueprints) < episode_count:
        scenario = ordered_scenarios[scenario_pointer % len(ordered_scenarios)]
        scenario_pointer += 1
        spec = SCENARIO_SPECS[scenario]
        scenario_cluster_index = scenario_cluster_counts[scenario]
        entity_template = spec["entity_templates"][
            scenario_cluster_index % len(spec["entity_templates"])
        ]
        available_variants = build_cluster_variants(
            entity_template,
            spec["cluster_axes"],
            scenario_cluster_index,
            rng,
        )
        cluster_size = min(
            len(available_variants), episode_count - len(episode_blueprints)
        )
        cluster_id = f"{scenario}_c{scenario_cluster_index + 1:02d}"

        anchor_episode_id = ""
        for variant_index in range(cluster_size):
            variant_role, diff_axis, entity_bundle = available_variants[variant_index]
            episode_id = make_episode_id(
                scenario=scenario,
                entity_bundle=entity_bundle,
                cluster_id=cluster_id,
                variant_role=variant_role,
            )
            if variant_index == 0:
                anchor_episode_id = episode_id
            episode_blueprints.append(
                {
                    "episode_id": episode_id,
                    "scenario": scenario,
                    "intent": spec["intent"],
                    "status": spec["statuses"][variant_index % len(spec["statuses"])],
                    "cluster_id": cluster_id,
                    "variant_role": variant_role,
                    "minimal_difference_axis": diff_axis or "baseline",
                    "entity_bundle": entity_bundle,
                    "required_tools": spec["required_tools"],
                    "query_focus": spec["query_focus"][
                        variant_index % len(spec["query_focus"])
                    ],
                    "target_anchor_episode_id": anchor_episode_id or episode_id,
                }
            )

        scenario_cluster_counts[scenario] = scenario_cluster_index + 1

    anchor_episodes = [
        episode for episode in episode_blueprints if episode["variant_role"] == "anchor"
    ]
    if not anchor_episodes:
        anchor_episodes = episode_blueprints[:]

    query_blueprints = build_query_blueprints(
        anchor_episodes=anchor_episodes,
        ordered_scenarios=ordered_scenarios,
        query_count=query_count,
    )

    return {
        "seed": seed,
        "episode_count": episode_count,
        "query_count": query_count,
        "episode_blueprints": episode_blueprints,
        "query_blueprints": query_blueprints,
        "coverage_summary": summarize_plan(episode_blueprints, query_blueprints),
    }


def build_query_blueprints(
    *,
    anchor_episodes: list[dict[str, Any]],
    ordered_scenarios: list[str],
    query_count: int,
) -> list[dict[str, Any]]:
    anchors_by_scenario: dict[str, list[dict[str, Any]]] = {}
    for episode in anchor_episodes:
        anchors_by_scenario.setdefault(episode["scenario"], []).append(episode)

    scenario_order = [
        scenario for scenario in ordered_scenarios if anchors_by_scenario.get(scenario)
    ]
    if not scenario_order:
        scenario_order = sorted(anchors_by_scenario)

    scenario_offsets = {scenario: 0 for scenario in scenario_order}
    query_blueprints: list[dict[str, Any]] = []
    scenario_pointer = 0
    for query_index in range(query_count):
        scenario = scenario_order[scenario_pointer % len(scenario_order)]
        scenario_pointer += 1
        anchors = anchors_by_scenario[scenario]
        episode = anchors[scenario_offsets[scenario] % len(anchors)]
        scenario_offsets[scenario] += 1
        query_blueprints.append(
            {
                "query_index": query_index + 1,
                "target_episode_ids": [episode["episode_id"]],
                "target_scenario": episode["scenario"],
                "target_intent": episode["intent"],
                "cluster_id": episode["cluster_id"],
                "query_focus": episode["query_focus"],
                "entity_bundle": episode["entity_bundle"],
                "minimal_difference_axis": episode["minimal_difference_axis"],
            }
        )
    return query_blueprints


def slice_generation_plan(
    plan: dict[str, Any],
    *,
    episode_offset: int,
    episode_count: int,
    query_offset: int,
    query_count: int,
) -> dict[str, Any]:
    all_episode_blueprints = list(plan.get("episode_blueprints", []))
    episode_blueprints = all_episode_blueprints[
        episode_offset : episode_offset + episode_count
    ]
    query_blueprints = list(plan.get("query_blueprints", []))[
        query_offset : query_offset + query_count
    ]
    if episode_count > 0 and not episode_blueprints and all_episode_blueprints:
        episode_blueprints = [all_episode_blueprints[-1]]
    return {
        "seed": plan["seed"],
        "episode_count": episode_count,
        "query_count": query_count,
        "episode_blueprints": episode_blueprints,
        "query_blueprints": query_blueprints,
        "coverage_summary": summarize_plan(episode_blueprints, query_blueprints),
    }


def build_cluster_variants(
    entity_template: dict[str, str],
    cluster_axes: list[str],
    cluster_index: int,
    rng: random.Random,
) -> list[tuple[str, str | None, dict[str, str]]]:
    base = dict(entity_template)
    variants: list[tuple[str, str | None, dict[str, str]]] = [("anchor", None, base)]
    preferred_axes = list(
        dict.fromkeys(
            cluster_axes
            + ["time_window", "location", "camera_model", "rating_threshold"]
        )
    )
    rng.shuffle(preferred_axes)
    for axis in preferred_axes[:2]:
        mutated = mutate_entity_bundle(base, axis, cluster_index)
        if mutated == base:
            continue
        variants.append((f"near_{axis}", axis, mutated))
    return variants


def mutate_entity_bundle(
    entity_bundle: dict[str, str], axis: str, cluster_index: int
) -> dict[str, str]:
    mutated = dict(entity_bundle)
    value = mutated.get(axis, "")
    if axis == "time_window":
        replacements = ["spring_2023", "autumn_2024", "winter_2022", "summer_2025"]
        mutated[axis] = choose_alternative(value, replacements, cluster_index)
    elif axis == "location":
        replacements = [
            "Paris",
            "Yosemite",
            "Tokyo",
            "Hawaii",
            "Johnson Wedding",
            "Family Holiday",
            "Alps",
        ]
        mutated[axis] = choose_alternative(value, replacements, cluster_index)
    elif axis == "camera_model":
        replacements = [
            "Canon EOS R5",
            "Sony A7III",
            "Nikon Z7 II",
            "Fujifilm X-T5",
            "Canon EOS R6",
        ]
        mutated[axis] = choose_alternative(value, replacements, cluster_index)
    elif axis == "rating_threshold":
        replacements = ["1", "2", "3", "4", "5"]
        mutated[axis] = choose_alternative(value, replacements, cluster_index)
    elif axis == "album_name":
        replacements = [
            "Paris Selects",
            "Yosemite Picks",
            "Tokyo Review",
            "Wedding Favorites",
        ]
        mutated[axis] = choose_alternative(value, replacements, cluster_index)
    elif axis == "failure_mode":
        replacements = [
            "false_positive_duplicate",
            "near_duplicate_burst",
            "burst_sequence_overlap",
        ]
        mutated[axis] = choose_alternative(value, replacements, cluster_index)
    elif axis == "similarity_threshold":
        replacements = ["0.74", "0.78", "0.82", "0.86"]
        mutated[axis] = choose_alternative(value, replacements, cluster_index)
    elif axis == "group_by":
        replacements = ["date", "camera_model", "rating", "lens", "tag"]
        mutated[axis] = choose_alternative(value, replacements, cluster_index)
    elif axis == "selection_theme":
        replacements = [
            "sunset",
            "family_highlights",
            "street_moments",
            "landscape_favorites",
        ]
        mutated[axis] = choose_alternative(value, replacements, cluster_index)
    elif axis == "tag_focus":
        replacements = ["family", "stage", "wildlife", "portrait"]
        mutated[axis] = choose_alternative(value, replacements, cluster_index)
    return mutated


def choose_alternative(current: str, replacements: list[str], offset: int) -> str:
    ordered = [value for value in replacements if value != current]
    if not ordered:
        return current
    return ordered[offset % len(ordered)]


def make_episode_id(
    *,
    scenario: str,
    entity_bundle: dict[str, str],
    cluster_id: str,
    variant_role: str,
) -> str:
    seed_parts = [scenario, cluster_id, variant_role]
    for key in sorted(entity_bundle):
        seed_parts.append(entity_bundle[key])
    return "ep_" + "_".join(slugify_token(part) for part in seed_parts if part)


def slugify_token(value: str) -> str:
    chars: list[str] = []
    previous_sep = False
    for char in value.lower():
        if char.isalnum():
            chars.append(char)
            previous_sep = False
            continue
        if previous_sep:
            continue
        chars.append("_")
        previous_sep = True
    return "".join(chars).strip("_")


def summarize_plan(
    episode_blueprints: list[dict[str, Any]],
    query_blueprints: list[dict[str, Any]],
) -> dict[str, Any]:
    episode_counts = Counter(
        (episode["scenario"], episode["intent"]) for episode in episode_blueprints
    )
    cluster_counts = Counter(episode["scenario"] for episode in episode_blueprints)
    scenario_cluster_counts = Counter(
        episode["scenario"]
        for episode in episode_blueprints
        if episode["variant_role"] == "anchor"
    )
    query_counts = Counter(
        (query["target_scenario"], query["target_intent"]) for query in query_blueprints
    )
    return {
        "episode_groups": {
            f"{scenario}/{intent}": count
            for (scenario, intent), count in sorted(episode_counts.items())
        },
        "scenario_clusters": {
            scenario: count
            for scenario, count in sorted(scenario_cluster_counts.items())
        },
        "scenario_episodes": {
            scenario: count for scenario, count in sorted(cluster_counts.items())
        },
        "query_groups": {
            f"{scenario}/{intent}": count
            for (scenario, intent), count in sorted(query_counts.items())
        },
    }


def format_plan_for_prompt(plan: dict[str, Any]) -> str:
    compact = {
        "seed": plan["seed"],
        "episode_count": plan["episode_count"],
        "query_count": plan["query_count"],
        "coverage_summary": plan["coverage_summary"],
        "episode_blueprints": plan["episode_blueprints"],
        "query_blueprints": plan["query_blueprints"],
    }
    return json.dumps(compact, ensure_ascii=False, indent=2)
