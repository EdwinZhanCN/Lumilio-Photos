from __future__ import annotations

import hashlib
import json
import os
import time
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import typer

from .bundle_lint import lint_bundle
from .deepseek_client import DeepSeekClient
from .ollama_embedder import OllamaEmbedder
from .qdrant_client import QdrantClient
from .schema_utils import (
    AGENT_ROOT,
    load_json,
    load_schema,
    validate_with_schema,
    write_json,
)

app = typer.Typer(help="Research CLI for agent episodic memory datasets.")


@app.command("print-schema-path")
def print_schema_path(schema_name: str = "episode_spec_bundle.schema.json") -> None:
    typer.echo(str(AGENT_ROOT / "schemas" / schema_name))


@app.command("validate-bundle")
def validate_bundle(
    input_path: Path,
    schema_name: str = "episode_spec_bundle.schema.json",
) -> None:
    payload = load_json(input_path)
    if schema_name == "episode_spec_bundle.schema.json":
        payload = normalize_bundle_for_schema(payload)
    validate_with_schema(payload, schema_name)
    if schema_name == "episode_spec_bundle.schema.json":
        issues = lint_bundle(payload)
        if issues:
            raise typer.BadParameter(
                "bundle passed schema validation but failed research lint:\n"
                + "\n".join(issues[:20])
            )
    typer.echo(f"validated: {input_path}")


@app.command("generate-spec-bundle")
def generate_spec_bundle(
    output_path: Path,
    episode_count: int = 200,
    query_count: int = 120,
    batch_episode_count: int = 12,
    batch_query_count: int = 6,
    seed: int = 42,
    model: str = "deepseek-chat",
    api_key_env: str = "DEEPSEEK_API_KEY",
    base_url: str = "https://api.deepseek.com",
    temperature: float = 0.7,
    timeout_seconds: float = 120.0,
    max_tokens: int = 7000,
    quiet: bool = False,
) -> None:
    def log(message: str, *, color: typer.colors.Color = typer.colors.CYAN) -> None:
        if quiet:
            return
        typer.secho(f"[generate-spec-bundle] {message}", fg=color, err=True)

    log("loading episode spec bundle schema")
    schema = load_schema("episode_spec_bundle.schema.json")
    if batch_episode_count <= 0 or batch_query_count <= 0:
        raise typer.BadParameter("batch sizes must be positive")
    if episode_count >= 150 or query_count >= 80:
        log(
            "large generation request detected; first run may take a while or hit provider limits",
            color=typer.colors.YELLOW,
        )
        log(
            "for a smoke test, consider something like --episode-count 20 --query-count 8",
            color=typer.colors.YELLOW,
        )

    log(f"loading DeepSeek API key from {api_key_env}")
    client = DeepSeekClient.from_env(api_key_env=api_key_env, base_url=base_url)
    log(
        f"requesting bundle from {model} at {base_url} with timeout={timeout_seconds:.0f}s "
        f"and max_tokens={max_tokens}"
    )
    bundle = generate_spec_bundle_in_batches(
        client=client,
        schema=schema,
        episode_count=episode_count,
        query_count=query_count,
        batch_episode_count=batch_episode_count,
        batch_query_count=batch_query_count,
        seed=seed,
        model=model,
        temperature=temperature,
        timeout_seconds=timeout_seconds,
        max_tokens=max_tokens,
        log=log,
    )
    backfill_missing_query_coverage(bundle, desired_query_count=query_count, log=log)
    log("validating generated bundle against episode_spec_bundle.schema.json")
    validate_with_schema(bundle, "episode_spec_bundle.schema.json")
    lint_issues = lint_bundle(bundle)
    if lint_issues:
        raise typer.BadParameter(
            "generated bundle failed research lint:\n" + "\n".join(lint_issues[:20])
        )
    log(f"writing validated bundle to {output_path}")
    write_json(output_path, bundle)
    typer.secho(f"wrote: {output_path}", fg=typer.colors.GREEN, err=True)


@app.command("make-splits")
def make_splits(
    input_path: Path,
    output_dir: Path,
    dataset_version: str = "v1",
    split_seed: int = 42,
    train_ratio: float = 0.8,
    val_ratio: float = 0.1,
) -> None:
    if train_ratio <= 0 or val_ratio <= 0 or train_ratio + val_ratio >= 1:
        raise typer.BadParameter(
            "train_ratio and val_ratio must be positive and sum to less than 1"
        )

    bundle = load_json(input_path)
    validate_with_schema(bundle, "episode_spec_bundle.schema.json")

    episodes = list(bundle.get("episodes", []))
    queries = list(bundle.get("queries", []))
    episode_groups = group_episodes_by_target(episodes)

    train: list[dict[str, Any]] = []
    val: list[dict[str, Any]] = []
    test: list[dict[str, Any]] = []
    group_to_split: dict[tuple[str, str], str] = {}

    ordered_groups = sorted(
        episode_groups.items(),
        key=lambda item: hashlib.sha256(
            f"{item[0][0]}|{item[0][1]}|{split_seed}".encode("utf-8")
        ).hexdigest(),
    )
    train_group_count, val_group_count, test_group_count = compute_group_split_counts(
        len(ordered_groups), train_ratio, val_ratio
    )

    for index, (group_key, group_episodes) in enumerate(ordered_groups):
        if index < train_group_count:
            split_name = "train"
            train.extend(group_episodes)
        elif index < train_group_count + val_group_count:
            split_name = "val"
            val.extend(group_episodes)
        else:
            split_name = "test"
            test.extend(group_episodes)
        group_to_split[group_key] = split_name

    train_queries, val_queries, test_queries = split_queries_by_target(
        queries, group_to_split
    )
    ensure_queries_match_split(train, train_queries, "train")
    ensure_queries_match_split(val, val_queries, "val")
    ensure_queries_match_split(test, test_queries, "test")

    output_dir.mkdir(parents=True, exist_ok=True)
    base_payload = {"schema_version": bundle["schema_version"]}
    write_json(
        output_dir / "train.bundle.json",
        {**base_payload, "episodes": train, "queries": train_queries},
    )
    write_json(
        output_dir / "val.bundle.json",
        {**base_payload, "episodes": val, "queries": val_queries},
    )
    write_json(
        output_dir / "test.bundle.json",
        {**base_payload, "episodes": test, "queries": test_queries},
    )

    manifest = {
        "dataset_version": dataset_version,
        "schema_version": bundle["schema_version"],
        "created_at": datetime.now(UTC).isoformat(),
        "generator": {
            "provider": "deepseek",
            "model": "unknown",
            "seed": split_seed,
        },
        "split_seed": split_seed,
        "counts": {
            "episodes": len(episodes),
            "queries": len(queries),
            "train": len(train),
            "val": len(val),
            "test": len(test),
        },
        "source_bundle": str(input_path),
    }
    validate_with_schema(manifest, "dataset_manifest.schema.json")
    write_json(output_dir / "manifest.json", manifest)
    typer.echo(f"wrote splits to: {output_dir}")


@app.command("benchmark-retrieval")
def benchmark_retrieval(
    input_path: Path,
    output_path: Path | None = None,
    qdrant_url: str = "http://localhost:6333",
    qdrant_api_key_env: str = "AGENT_MEMORY_QDRANT_API_KEY",
    collection: str = "agent_episodic_memory",
    embed_base_url: str = "http://localhost:11434",
    embed_model: str = "qwen3-embedding:0.6b",
    embed_dims: int = 1024,
    embed_keep_alive: str = "5m",
    limit: int = 10,
    ks: str = "1,5,10",
    use_filters: bool = True,
    user_id: str = "",
) -> None:
    bundle = load_json(input_path)
    validate_with_schema(bundle, "episode_spec_bundle.schema.json")
    queries = list(bundle.get("queries", []))
    if not queries:
        raise typer.BadParameter("bundle does not contain queries")

    top_ks = parse_ks(ks, limit)
    qdrant_api_key = os.getenv(qdrant_api_key_env, "").strip()

    embedder = OllamaEmbedder(
        base_url=embed_base_url,
        model=embed_model,
        dimensions=embed_dims,
        keep_alive=embed_keep_alive,
    )
    qdrant = QdrantClient(
        base_url=qdrant_url,
        collection=collection,
        api_key=qdrant_api_key,
    )

    per_query: list[dict[str, Any]] = []
    embed_latencies: list[float] = []
    search_latencies: list[float] = []
    e2e_latencies: list[float] = []
    reciprocal_ranks: list[float] = []
    recall_hits = {k: 0 for k in top_ks}

    for query_spec in queries:
        started = time.perf_counter()

        embed_started = time.perf_counter()
        vector = embedder.embed_text(query_spec["query"])
        embed_ms = (time.perf_counter() - embed_started) * 1000

        search_started = time.perf_counter()
        results = qdrant.search(
            vector=vector,
            limit=limit,
            user_id=user_id,
            entity=query_spec.get("entity", "") if use_filters else "",
            status=query_spec.get("status", "") if use_filters else "",
            tags=list(query_spec.get("tags", [])) if use_filters else [],
        )
        search_ms = (time.perf_counter() - search_started) * 1000
        e2e_ms = (time.perf_counter() - started) * 1000

        rank = match_rank(results, query_spec)
        reciprocal_ranks.append(0.0 if rank is None else 1.0 / rank)
        for k in top_ks:
            if rank is not None and rank <= k:
                recall_hits[k] += 1

        per_query.append(
            {
                "query": query_spec["query"],
                "target_scenario": query_spec.get("target_scenario", ""),
                "target_intent": query_spec.get("target_intent", ""),
                "rank": rank,
                "embed_ms": round(embed_ms, 3),
                "search_ms": round(search_ms, 3),
                "end_to_end_ms": round(e2e_ms, 3),
                "results": [
                    {
                        "rank": index + 1,
                        "score": point.get("score"),
                        "episode_id": extract_episode(point).get("id", ""),
                        "scenario": extract_episode(point).get("scenario", ""),
                        "intent": extract_episode(point).get("intent", ""),
                        "summary": extract_episode(point).get("summary", ""),
                    }
                    for index, point in enumerate(results)
                ],
            }
        )
        embed_latencies.append(embed_ms)
        search_latencies.append(search_ms)
        e2e_latencies.append(e2e_ms)

    evaluated = len(queries)
    report = {
        "schema_version": "agent-memory/retrieval-benchmark-report/v1",
        "created_at": datetime.now(UTC).isoformat(),
        "input_bundle": str(input_path),
        "collection": collection,
        "embedder": {
            "provider": "ollama",
            "base_url": embed_base_url,
            "model": embed_model,
            "dimensions": embed_dims,
        },
        "limit": limit,
        "ks": top_ks,
        "query_count": evaluated,
        "filters_enabled": use_filters,
        "metrics": {
            "recall": {
                f"recall@{k}": round(recall_hits[k] / evaluated, 6) for k in top_ks
            },
            "mrr": {f"mrr@{limit}": round(sum(reciprocal_ranks) / evaluated, 6)},
        },
        "latency_ms": {
            "embed": summarize_latency(embed_latencies),
            "search": summarize_latency(search_latencies),
            "end_to_end": summarize_latency(e2e_latencies),
        },
        "per_query": per_query,
    }
    validate_with_schema(report, "retrieval_benchmark_report.schema.json")

    if output_path is None:
        typer.echo(json.dumps(report, ensure_ascii=False, indent=2))
        return

    write_json(output_path, report)
    typer.echo(f"wrote benchmark report: {output_path}")


def main() -> None:
    app()


def group_episodes_by_target(
    episodes: list[dict[str, Any]],
) -> dict[tuple[str, str], list[dict[str, Any]]]:
    groups: dict[tuple[str, str], list[dict[str, Any]]] = {}
    for episode in episodes:
        group_key = (
            str(episode.get("scenario", "")).strip(),
            str(episode.get("intent", "")).strip(),
        )
        groups.setdefault(group_key, []).append(episode)
    return groups


def compute_group_split_counts(
    total_groups: int, train_ratio: float, val_ratio: float
) -> tuple[int, int, int]:
    if total_groups <= 0:
        return 0, 0, 0

    train_count = int(total_groups * train_ratio)
    val_count = int(total_groups * val_ratio)
    test_count = total_groups - train_count - val_count

    if total_groups >= 3:
        if train_count == 0:
            train_count = 1
        if val_count == 0:
            val_count = 1
        test_count = total_groups - train_count - val_count
        if test_count <= 0:
            test_count = 1
            if train_count >= val_count and train_count > 1:
                train_count -= 1
            elif val_count > 1:
                val_count -= 1
            else:
                train_count = max(1, train_count - 1)
            test_count = total_groups - train_count - val_count

    if test_count < 0:
        test_count = 0
    return train_count, val_count, test_count


def split_queries_by_target(
    queries: list[dict[str, Any]],
    group_to_split: dict[tuple[str, str], str],
) -> tuple[list[dict[str, Any]], list[dict[str, Any]], list[dict[str, Any]]]:
    train_queries: list[dict[str, Any]] = []
    val_queries: list[dict[str, Any]] = []
    test_queries: list[dict[str, Any]] = []

    for query in queries:
        group_key = (
            str(query.get("target_scenario", "")).strip(),
            str(query.get("target_intent", "")).strip(),
        )
        split_name = group_to_split.get(group_key)
        if split_name == "train":
            train_queries.append(query)
        elif split_name == "val":
            val_queries.append(query)
        elif split_name == "test":
            test_queries.append(query)

    return train_queries, val_queries, test_queries


def ensure_queries_match_split(
    episodes: list[dict[str, Any]], queries: list[dict[str, Any]], split_name: str
) -> None:
    available_targets = {
        (
            str(episode.get("scenario", "")).strip(),
            str(episode.get("intent", "")).strip(),
        )
        for episode in episodes
    }
    for query in queries:
        query_target = (
            str(query.get("target_scenario", "")).strip(),
            str(query.get("target_intent", "")).strip(),
        )
        if query_target not in available_targets:
            raise typer.BadParameter(
                f"{split_name} split contains query target {query_target!r} without a matching episode"
            )


def generate_spec_bundle_in_batches(
    *,
    client: DeepSeekClient,
    schema: dict[str, Any],
    episode_count: int,
    query_count: int,
    batch_episode_count: int,
    batch_query_count: int,
    seed: int,
    model: str,
    temperature: float,
    timeout_seconds: float,
    max_tokens: int,
    log,
) -> dict[str, Any]:
    remaining_episodes = episode_count
    remaining_queries = query_count
    batch_index = 0
    schema_text = json.dumps(schema, ensure_ascii=False, indent=2)
    merged_bundle: dict[str, Any] = {
        "schema_version": "agent-memory/episode-spec-bundle/v1",
        "episodes": [],
        "queries": [],
    }

    while remaining_episodes > 0 or remaining_queries > 0:
        batch_index += 1
        target_episode_count = min(batch_episode_count, remaining_episodes)
        target_query_count = min(batch_query_count, remaining_queries)
        current_episode_count = max(1, target_episode_count)
        current_query_count = max(1, target_query_count)

        log(
            f"starting batch {batch_index}: episodes={target_episode_count}, "
            f"queries={target_query_count}"
        )
        batch_bundle = client.generate_spec_bundle(
            model=model,
            episode_count=current_episode_count,
            query_count=current_query_count,
            seed=seed + batch_index - 1,
            schema_text=schema_text,
            temperature=temperature,
            timeout_seconds=timeout_seconds,
            max_tokens=max_tokens,
            progress=log,
        )
        batch_bundle = normalize_bundle_for_schema(batch_bundle)
        validate_with_schema(batch_bundle, "episode_spec_bundle.schema.json")

        generated_episodes = list(batch_bundle.get("episodes", []))
        generated_queries = list(batch_bundle.get("queries", []))
        merged_bundle["episodes"].extend(generated_episodes[:target_episode_count])
        merged_bundle["queries"].extend(generated_queries[:target_query_count])
        remaining_episodes -= target_episode_count
        remaining_queries -= target_query_count

        log(
            f"finished batch {batch_index}: accumulated "
            f"{len(merged_bundle['episodes'])}/{episode_count} episodes, "
            f"{len(merged_bundle['queries'])}/{query_count} queries",
            color=typer.colors.GREEN,
        )

    return merged_bundle


def backfill_missing_query_coverage(
    bundle: dict[str, Any], desired_query_count: int, log
) -> None:
    episodes = list(bundle.get("episodes", []))
    queries = list(bundle.get("queries", []))
    episode_groups = group_episodes_by_target(episodes)
    missing_targets = find_missing_query_targets(episodes, queries)
    if not missing_targets:
        return

    log(
        "backfilling synthetic queries for missing scenario+intent groups: "
        + ", ".join(f"{scenario}/{intent}" for scenario, intent in missing_targets),
        color=typer.colors.YELLOW,
    )

    protected_targets: set[tuple[str, str]] = set()
    for target in missing_targets:
        group = episode_groups.get(target, [])
        if not group:
            continue
        queries.append(synthesize_query_from_episode(group[0]))
        protected_targets.add(target)

    trim_queries_to_target_count(
        queries,
        desired_query_count=desired_query_count,
        protected_targets=protected_targets,
        log=log,
    )
    bundle["queries"] = queries


def find_missing_query_targets(
    episodes: list[dict[str, Any]], queries: list[dict[str, Any]]
) -> list[tuple[str, str]]:
    episode_targets = {
        (
            str(episode.get("scenario", "")).strip(),
            str(episode.get("intent", "")).strip(),
        )
        for episode in episodes
        if str(episode.get("scenario", "")).strip()
        and str(episode.get("intent", "")).strip()
    }
    query_targets = {
        (
            str(query.get("target_scenario", "")).strip(),
            str(query.get("target_intent", "")).strip(),
        )
        for query in queries
        if str(query.get("target_scenario", "")).strip()
        and str(query.get("target_intent", "")).strip()
    }
    return sorted(episode_targets - query_targets)


def synthesize_query_from_episode(episode: dict[str, Any]) -> dict[str, Any]:
    goal = str(episode.get("goal", "")).strip()
    summary = str(episode.get("summary", "")).strip()
    scenario = str(episode.get("scenario", "")).strip()
    intent = str(episode.get("intent", "")).strip()
    status = str(episode.get("status", "")).strip()
    tags = [str(tag).strip() for tag in episode.get("tags", []) if str(tag).strip()]
    entity = pick_query_entity(episode)

    if goal and entity:
        query_text = f"How did I handle {goal.lower()} for {entity} last time?"
    elif goal:
        query_text = f"How did I handle {goal.lower()} last time?"
    elif summary and entity:
        query_text = f"Find the previous {intent} episode for {entity}."
    else:
        query_text = f"Find the previous {intent} episode for this task."

    return {
        "query": query_text,
        "target_scenario": scenario,
        "target_intent": intent,
        "entity": entity,
        "status": status,
        "tags": tags[:4],
        "notes": "Synthetic coverage backfill generated from episode goal and entities.",
    }


def pick_query_entity(episode: dict[str, Any]) -> str:
    entities = episode.get("entities", [])
    if isinstance(entities, list):
        for entity in entities:
            if not isinstance(entity, dict):
                continue
            name = str(entity.get("name", "")).strip()
            if name:
                return name
    metadata = episode.get("metadata", {})
    if isinstance(metadata, dict):
        for value in metadata.values():
            text = str(value).strip()
            if text:
                return text
    return ""


def trim_queries_to_target_count(
    queries: list[dict[str, Any]],
    *,
    desired_query_count: int,
    protected_targets: set[tuple[str, str]],
    log,
) -> None:
    if desired_query_count <= 0 or len(queries) <= desired_query_count:
        return

    counts_by_target: dict[tuple[str, str], int] = {}
    for query in queries:
        target = (
            str(query.get("target_scenario", "")).strip(),
            str(query.get("target_intent", "")).strip(),
        )
        counts_by_target[target] = counts_by_target.get(target, 0) + 1

    removed = 0
    while len(queries) > desired_query_count:
        removed_index = None
        for index in range(len(queries) - 1, -1, -1):
            query = queries[index]
            target = (
                str(query.get("target_scenario", "")).strip(),
                str(query.get("target_intent", "")).strip(),
            )
            if target in protected_targets:
                continue
            if counts_by_target.get(target, 0) <= 1:
                continue
            removed_index = index
            counts_by_target[target] -= 1
            break
        if removed_index is None:
            break
        queries.pop(removed_index)
        removed += 1

    if removed:
        log(
            f"trimmed {removed} duplicate queries after coverage backfill; final query count={len(queries)}",
            color=typer.colors.YELLOW,
        )


def normalize_bundle_for_schema(payload: dict[str, Any]) -> dict[str, Any]:
    episodes = payload.get("episodes", [])
    if not isinstance(episodes, list):
        return payload

    for episode in episodes:
        if not isinstance(episode, dict):
            continue
        metadata = episode.get("metadata")
        if not isinstance(metadata, dict):
            continue
        episode["metadata"] = {
            str(key): stringify_metadata_value(value) for key, value in metadata.items()
        }

    return payload


def stringify_metadata_value(value: Any) -> str:
    if isinstance(value, bool):
        return "true" if value else "false"
    if value is None:
        return ""
    return str(value)


def parse_ks(raw: str, limit: int) -> list[int]:
    values = []
    for part in raw.split(","):
        part = part.strip()
        if not part:
            continue
        value = int(part)
        if value <= 0:
            raise typer.BadParameter("ks values must be positive")
        if value > limit:
            raise typer.BadParameter("ks values cannot exceed limit")
        values.append(value)
    if not values:
        raise typer.BadParameter("at least one k value is required")
    return sorted(set(values))


def summarize_latency(values: list[float]) -> dict[str, float]:
    sorted_values = sorted(values)
    return {
        "p50": round(percentile(sorted_values, 0.50), 3),
        "p95": round(percentile(sorted_values, 0.95), 3),
        "mean": round(sum(sorted_values) / len(sorted_values), 3),
    }


def percentile(sorted_values: list[float], ratio: float) -> float:
    if not sorted_values:
        return 0.0
    index = max(
        0, min(len(sorted_values) - 1, int(round((len(sorted_values) - 1) * ratio)))
    )
    return sorted_values[index]


def match_rank(results: list[dict[str, Any]], query_spec: dict[str, Any]) -> int | None:
    target_scenario = str(query_spec.get("target_scenario", "")).strip()
    target_intent = str(query_spec.get("target_intent", "")).strip()

    for index, point in enumerate(results, start=1):
        episode = extract_episode(point)
        if target_scenario and episode.get("scenario") != target_scenario:
            continue
        if target_intent and episode.get("intent") != target_intent:
            continue
        if not target_scenario and not target_intent:
            continue
        return index
    return None


def extract_episode(point: dict[str, Any]) -> dict[str, Any]:
    payload = point.get("payload", {})
    episode = payload.get("episode", {})
    return episode if isinstance(episode, dict) else {}
