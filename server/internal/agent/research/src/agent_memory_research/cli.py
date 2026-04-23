from __future__ import annotations

import hashlib
import json
import os
import re
import time
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import typer

from .bundle_lint import lint_bundle
from .deepseek_client import DeepSeekClient
from .generation_matrix import build_generation_plan, slice_generation_plan
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

NUMERIC_SLOT_PATTERN = re.compile(r"\b(?:\d+(?:\.\d+)?\+?)\b")


@app.command("print-schema-path")
def print_schema_path(schema_name: str = "episode_spec_bundle.schema.json") -> None:
    typer.echo(str(AGENT_ROOT / "schemas" / schema_name))


@app.command("print-generation-plan")
def print_generation_plan(
    episode_count: int = 12,
    query_count: int = 4,
    seed: int = 42,
) -> None:
    plan = build_generation_plan(
        episode_count=episode_count,
        query_count=query_count,
        seed=seed,
    )
    typer.echo(json.dumps(plan, ensure_ascii=False, indent=2))


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
    batch_episode_count: int = 12,
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
    if batch_episode_count <= 0:
        raise typer.BadParameter("batch sizes must be positive")
    if episode_count >= 150:
        log(
            "large generation request detected; first run may take a while or hit provider limits",
            color=typer.colors.YELLOW,
        )
        log(
            "for a smoke test, consider something like --episode-count 20",
            color=typer.colors.YELLOW,
        )

    log(f"loading DeepSeek API key from {api_key_env}")
    client = DeepSeekClient.from_env(api_key_env=api_key_env, base_url=base_url)
    log(
        f"requesting bundle from {model} at {base_url} with timeout={timeout_seconds:.0f}s "
        f"and max_tokens={max_tokens}"
    )
    bundle = generate_episode_bundle_in_batches(
        client=client,
        schema=schema,
        episode_count=episode_count,
        batch_episode_count=batch_episode_count,
        seed=seed,
        model=model,
        temperature=temperature,
        timeout_seconds=timeout_seconds,
        max_tokens=max_tokens,
        log=log,
    )
    bundle = normalize_bundle_for_schema(bundle)
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
    min_test_clusters_per_scenario: int = 3,
    min_val_clusters_per_scenario: int = 1,
) -> None:
    if train_ratio <= 0 or val_ratio <= 0 or train_ratio + val_ratio >= 1:
        raise typer.BadParameter(
            "train_ratio and val_ratio must be positive and sum to less than 1"
        )
    if min_test_clusters_per_scenario < 0 or min_val_clusters_per_scenario < 0:
        raise typer.BadParameter("minimum cluster counts must be non-negative")

    bundle = load_json(input_path)
    bundle = normalize_bundle_for_schema(bundle)
    validate_with_schema(bundle, "episode_spec_bundle.schema.json")

    episodes = list(bundle.get("episodes", []))
    episode_groups = group_episodes_by_scenario_cluster(episodes)

    train: list[dict[str, Any]] = []
    val: list[dict[str, Any]] = []
    test: list[dict[str, Any]] = []
    ordered_scenarios = sorted(
        episode_groups,
        key=lambda scenario: hashlib.sha256(
            f"{scenario}|{split_seed}".encode("utf-8")
        ).hexdigest(),
    )
    for scenario in ordered_scenarios:
        cluster_groups = episode_groups[scenario]
        ordered_clusters = sorted(
            cluster_groups.items(),
            key=lambda item: hashlib.sha256(
                f"{scenario}|{item[0]}|{split_seed}".encode("utf-8")
            ).hexdigest(),
        )
        train_group_count, val_group_count, test_group_count = (
            compute_group_split_counts(
                len(ordered_clusters),
                train_ratio,
                val_ratio,
                min_test_groups=min_test_clusters_per_scenario,
                min_val_groups=min_val_clusters_per_scenario,
            )
        )
        for index, (_, cluster_episodes) in enumerate(ordered_clusters):
            if index < train_group_count:
                train.extend(cluster_episodes)
            elif index < train_group_count + val_group_count:
                val.extend(cluster_episodes)
            else:
                test.extend(cluster_episodes)

    output_dir.mkdir(parents=True, exist_ok=True)
    base_payload = {"schema_version": bundle["schema_version"]}
    write_json(
        output_dir / "train.bundle.json",
        {**base_payload, "episodes": train, "queries": []},
    )
    write_json(
        output_dir / "val.bundle.json",
        {**base_payload, "episodes": val, "queries": []},
    )
    write_json(
        output_dir / "test.bundle.json",
        {**base_payload, "episodes": test, "queries": []},
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
        "query_generation_protocol": "split_specific",
        "counts": {
            "episodes": len(episodes),
            "queries": 0,
            "train": len(train),
            "val": len(val),
            "test": len(test),
        },
        "source_bundle": str(input_path),
    }
    validate_with_schema(manifest, "dataset_manifest.schema.json")
    write_json(output_dir / "manifest.json", manifest)
    typer.echo(f"wrote splits to: {output_dir}")


@app.command("generate-split-queries")
def generate_split_queries(
    input_path: Path,
    output_path: Path | None = None,
    model: str = "deepseek-chat",
    api_key_env: str = "DEEPSEEK_API_KEY",
    base_url: str = "https://api.deepseek.com",
    temperature: float = 0.7,
    timeout_seconds: float = 120.0,
    max_tokens: int = 7000,
    queries_per_episode: int = 1,
    query_style: str = "precise",
    replace_existing: bool = True,
    quiet: bool = False,
) -> None:
    def log(message: str, *, color: typer.colors.Color = typer.colors.CYAN) -> None:
        if quiet:
            return
        typer.secho(f"[generate-split-queries] {message}", fg=color, err=True)

    if queries_per_episode <= 0:
        raise typer.BadParameter("queries_per_episode must be positive")
    if query_style not in {"precise", "reduced_slot"}:
        raise typer.BadParameter("query_style must be one of: precise, reduced_slot")

    bundle = load_json(input_path)
    bundle = normalize_bundle_for_schema(bundle)
    validate_with_schema(bundle, "episode_spec_bundle.schema.json")

    episodes = list(bundle.get("episodes", []))
    if not episodes:
        raise typer.BadParameter("bundle does not contain episodes")

    existing_queries = list(bundle.get("queries", []))
    if existing_queries and not replace_existing:
        raise typer.BadParameter(
            "bundle already contains queries; pass --replace-existing to overwrite"
        )

    split_name = infer_split_name(input_path)
    log(f"loading DeepSeek API key from {api_key_env}")
    client = DeepSeekClient.from_env(api_key_env=api_key_env, base_url=base_url)

    generated_queries = generate_queries_for_split_groups(
        client=client,
        episodes=episodes,
        split_name=split_name,
        model=model,
        temperature=temperature,
        timeout_seconds=timeout_seconds,
        max_tokens=max_tokens,
        queries_per_episode=queries_per_episode,
        query_style=query_style,
        log=log,
    )

    output_bundle = {
        "schema_version": bundle["schema_version"],
        "episodes": episodes,
        "queries": generated_queries,
    }
    output_bundle = normalize_bundle_for_schema(output_bundle)
    validate_with_schema(output_bundle, "episode_spec_bundle.schema.json")
    lint_issues = lint_bundle(output_bundle)
    if lint_issues:
        raise typer.BadParameter(
            "generated split queries failed research lint:\n"
            + "\n".join(lint_issues[:20])
        )

    target_path = output_path or input_path
    write_json(target_path, output_bundle)
    typer.echo(f"wrote split queries to: {target_path}")


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
    use_filters: bool = False,
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
                "target_episode_ids": list(query_spec.get("target_episode_ids", [])),
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


@app.command("analyze-benchmark-report")
def analyze_benchmark_report(
    report_paths: list[Path],
    output_path: Path | None = None,
    output_format: str = "markdown",
    top_examples: int = 5,
) -> None:
    if top_examples <= 0:
        raise typer.BadParameter("top_examples must be positive")
    if not report_paths:
        raise typer.BadParameter("at least one report path is required")
    if output_format not in {"markdown", "json", "text"}:
        raise typer.BadParameter("output_format must be one of: markdown, json, text")

    reports = [load_and_validate_report(path) for path in report_paths]
    analysis = build_report_analysis_payload(reports, top_examples=top_examples)

    if output_path is not None:
        rendered = render_analysis_output(analysis, output_format=output_format)
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_text(rendered, encoding="utf-8")
        typer.echo(f"wrote analysis report: {output_path}")
        return

    rendered = render_analysis_output(analysis, output_format=output_format)
    typer.echo(rendered)


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


def load_and_validate_report(path: Path) -> dict[str, Any]:
    payload = load_json(path)
    validate_with_schema(payload, "retrieval_benchmark_report.schema.json")
    payload["_report_path"] = str(path)
    return payload


def build_report_analysis_payload(
    reports: list[dict[str, Any]], *, top_examples: int
) -> dict[str, Any]:
    report_summaries = [
        summarize_report_for_analysis(report, top_examples=top_examples)
        for report in reports
    ]
    return {
        "generated_at": datetime.now(UTC).isoformat(),
        "report_count": len(report_summaries),
        "reports": report_summaries,
        "comparison": summarize_report_comparison(report_summaries),
    }


def summarize_report_for_analysis(
    report: dict[str, Any], *, top_examples: int
) -> dict[str, Any]:
    report_name = Path(str(report.get("_report_path", ""))).name or "<report>"
    embedder = dict(report.get("embedder", {}))
    per_query = list(report.get("per_query", []))
    numeric_rows = [
        row for row in per_query if NUMERIC_SLOT_PATTERN.search(str(row.get("query", "")))
    ]
    non_numeric_rows = [
        row
        for row in per_query
        if not NUMERIC_SLOT_PATTERN.search(str(row.get("query", "")))
    ]
    scenario_rows = summarize_by_scenario(per_query)
    misses = [row for row in per_query if row.get("rank") != 1]
    same_scenario_misses = 0
    same_intent_misses = 0
    for row in misses:
        top_result = first_result(row)
        if top_result.get("scenario") == row.get("target_scenario"):
            same_scenario_misses += 1
        if top_result.get("intent") == row.get("target_intent"):
            same_intent_misses += 1

    top_misses = []
    for row in misses[:top_examples]:
        top_result = first_result(row)
        top_misses.append(
            {
                "rank": row.get("rank"),
                "query": row.get("query", ""),
                "target_scenario": row.get("target_scenario", ""),
                "target_intent": row.get("target_intent", ""),
                "top1_scenario": top_result.get("scenario", ""),
                "top1_intent": top_result.get("intent", ""),
                "top1_episode_id": top_result.get("episode_id", ""),
            }
        )

    return {
        "report_name": report_name,
        "report_path": str(report.get("_report_path", "")),
        "collection": report.get("collection", ""),
        "embedder": embedder,
        "query_count": int(report.get("query_count", 0)),
        "overall": {
            "recall@1": report_metric(report, "recall@1"),
            "recall@5": report_metric(report, "recall@5"),
            "mrr@10": report_metric(report, "mrr@10"),
        },
        "numeric_slots": {
            "with_number": summarize_query_subset(numeric_rows),
            "without_number": summarize_query_subset(non_numeric_rows),
        },
        "by_scenario": [
            {"scenario": scenario, **summary} for scenario, summary in scenario_rows
        ],
        "error_topology": {
            "misses": len(misses),
            "same_scenario_top1": same_scenario_misses,
            "same_intent_top1": same_intent_misses,
        },
        "top_misses": top_misses,
    }


def summarize_report_comparison(report_summaries: list[dict[str, Any]]) -> dict[str, Any]:
    scenario_names = sorted(
        {
            entry["scenario"]
            for report in report_summaries
            for entry in report.get("by_scenario", [])
        }
    )
    scenario_comparison = []
    for scenario in scenario_names:
        row = {"scenario": scenario, "reports": []}
        for report in report_summaries:
            scenario_summary = next(
                (
                    entry
                    for entry in report.get("by_scenario", [])
                    if entry["scenario"] == scenario
                ),
                None,
            )
            if scenario_summary is None:
                continue
            row["reports"].append(
                {
                    "report_name": report["report_name"],
                    "recall@1": scenario_summary["recall@1"],
                    "recall@5": scenario_summary["recall@5"],
                    "mrr": scenario_summary["mrr"],
                }
            )
        scenario_comparison.append(row)

    return {
        "overall": [
            {
                "report_name": report["report_name"],
                "recall@1": report["overall"]["recall@1"],
                "recall@5": report["overall"]["recall@5"],
                "mrr@10": report["overall"]["mrr@10"],
            }
            for report in report_summaries
        ],
        "by_scenario": scenario_comparison,
    }


def render_analysis_output(
    analysis: dict[str, Any], *, output_format: str
) -> str:
    if output_format == "json":
        return json.dumps(analysis, ensure_ascii=False, indent=2)
    if output_format == "markdown":
        return render_analysis_markdown(analysis)
    return render_analysis_text(analysis)


def render_analysis_markdown(analysis: dict[str, Any]) -> str:
    lines: list[str] = []
    lines.append("# Benchmark Analysis")
    lines.append("")
    lines.append(f"- Generated at: `{analysis['generated_at']}`")
    lines.append(f"- Report count: `{analysis['report_count']}`")
    lines.append("")

    for report in analysis["reports"]:
        lines.append(f"## {report['report_name']}")
        lines.append("")
        lines.append(f"- Collection: `{report['collection']}`")
        lines.append(
            f"- Model: `{report['embedder'].get('model', '')}` @ `{report['embedder'].get('dimensions', '')}d`"
        )
        lines.append(f"- Queries: `{report['query_count']}`")
        lines.append(
            f"- Overall: `recall@1={report['overall']['recall@1']:.3f}`, "
            f"`recall@5={report['overall']['recall@5']:.3f}`, "
            f"`mrr@10={report['overall']['mrr@10']:.3f}`"
        )
        lines.append(
            f"- Numeric slots / with number: "
            f"`n={int(report['numeric_slots']['with_number']['count'])}`, "
            f"`r@1={report['numeric_slots']['with_number']['recall@1']:.3f}`, "
            f"`mrr={report['numeric_slots']['with_number']['mrr']:.3f}`"
        )
        lines.append(
            f"- Numeric slots / without number: "
            f"`n={int(report['numeric_slots']['without_number']['count'])}`, "
            f"`r@1={report['numeric_slots']['without_number']['recall@1']:.3f}`, "
            f"`mrr={report['numeric_slots']['without_number']['mrr']:.3f}`"
        )
        lines.append(
            f"- Error topology: `misses={report['error_topology']['misses']}`, "
            f"`same_scenario_top1={report['error_topology']['same_scenario_top1']}`, "
            f"`same_intent_top1={report['error_topology']['same_intent_top1']}`"
        )
        lines.append("")
        lines.append("### By Scenario")
        lines.append("")
        lines.append("| Scenario | N | Recall@1 | Recall@5 | MRR |")
        lines.append("| --- | ---: | ---: | ---: | ---: |")
        for entry in report["by_scenario"]:
            lines.append(
                f"| `{entry['scenario']}` | {int(entry['count'])} | "
                f"{entry['recall@1']:.3f} | {entry['recall@5']:.3f} | {entry['mrr']:.3f} |"
            )
        lines.append("")
        if report["top_misses"]:
            lines.append("### Top Misses")
            lines.append("")
            for miss in report["top_misses"]:
                lines.append(
                    f"- Rank `{miss['rank']}` target `{miss['target_scenario']}/{miss['target_intent']}` "
                    f"vs top1 `{miss['top1_scenario']}/{miss['top1_intent']}`"
                )
                lines.append(f"  Query: {miss['query']}")
                lines.append(f"  Top1 episode: `{miss['top1_episode_id']}`")
            lines.append("")

    if analysis["report_count"] > 1:
        lines.append("## Comparison")
        lines.append("")
        lines.append("### Overall")
        lines.append("")
        lines.append("| Report | Recall@1 | Recall@5 | MRR@10 |")
        lines.append("| --- | ---: | ---: | ---: |")
        for row in analysis["comparison"]["overall"]:
            lines.append(
                f"| `{row['report_name']}` | {row['recall@1']:.3f} | "
                f"{row['recall@5']:.3f} | {row['mrr@10']:.3f} |"
            )
        lines.append("")
        lines.append("### By Scenario")
        lines.append("")
        for row in analysis["comparison"]["by_scenario"]:
            lines.append(f"- `{row['scenario']}`")
            for report in row["reports"]:
                lines.append(
                    f"  - `{report['report_name']}`: "
                    f"`r@1={report['recall@1']:.3f}`, "
                    f"`r@5={report['recall@5']:.3f}`, "
                    f"`mrr={report['mrr']:.3f}`"
                )
        lines.append("")

    return "\n".join(lines).rstrip() + "\n"


def render_analysis_text(analysis: dict[str, Any]) -> str:
    lines: list[str] = []
    for index, report in enumerate(analysis["reports"]):
        if index > 0:
            lines.append("")
        lines.append(f"Report: {report['report_name']}")
        lines.append(f"Collection: {report['collection']}")
        lines.append(
            f"Model: {report['embedder'].get('model', '')} @ {report['embedder'].get('dimensions', '')}d"
        )
        lines.append(
            "Overall: "
            f"queries={report['query_count']} "
            f"recall@1={report['overall']['recall@1']:.3f} "
            f"recall@5={report['overall']['recall@5']:.3f} "
            f"mrr@10={report['overall']['mrr@10']:.3f}"
        )
        lines.append(
            "Numeric Slots: "
            + format_subset_line(
                "with_number", report["numeric_slots"]["with_number"]
            )
        )
        lines.append(
            "Numeric Slots: "
            + format_subset_line(
                "without_number", report["numeric_slots"]["without_number"]
            )
        )
        lines.append("By Scenario:")
        for entry in report["by_scenario"]:
            lines.append(f"  {format_subset_line(entry['scenario'], entry)}")
        lines.append(
            "Error Topology: "
            f"misses={report['error_topology']['misses']} "
            f"same_scenario_top1={report['error_topology']['same_scenario_top1']}/"
            f"{report['error_topology']['misses'] if report['error_topology']['misses'] else 0} "
            f"same_intent_top1={report['error_topology']['same_intent_top1']}/"
            f"{report['error_topology']['misses'] if report['error_topology']['misses'] else 0}"
        )
        if report["top_misses"]:
            lines.append("Top Misses:")
            for miss in report["top_misses"]:
                lines.append(
                    "  "
                    f"rank={miss['rank']} "
                    f"target={miss['target_scenario']}/{miss['target_intent']} "
                    f"top1={miss['top1_scenario']}/{miss['top1_intent']}"
                )
                lines.append(f"    q: {miss['query']}")
                lines.append(f"    top1 episode: {miss['top1_episode_id']}")

    if analysis["report_count"] > 1:
        lines.append("")
        lines.append("Comparison:")
        for row in analysis["comparison"]["overall"]:
            lines.append(
                "  "
                f"{row['report_name']}: "
                f"r@1={row['recall@1']:.3f} "
                f"r@5={row['recall@5']:.3f} "
                f"mrr@10={row['mrr@10']:.3f}"
            )
        lines.append("Scenario Comparison:")
        for row in analysis["comparison"]["by_scenario"]:
            parts = [row["scenario"]]
            for report in row["reports"]:
                parts.append(
                    f"{Path(report['report_name']).stem}: r@1={report['recall@1']:.3f}"
                )
            lines.append("  " + " | ".join(parts))

    return "\n".join(lines).rstrip() + "\n"


def report_metric(report: dict[str, Any], metric_name: str) -> float:
    recall_metrics = report.get("metrics", {}).get("recall", {})
    mrr_metrics = report.get("metrics", {}).get("mrr", {})
    if metric_name in recall_metrics:
        return float(recall_metrics[metric_name])
    if metric_name in mrr_metrics:
        return float(mrr_metrics[metric_name])
    return 0.0


def summarize_by_scenario(
    per_query: list[dict[str, Any]],
) -> list[tuple[str, dict[str, float]]]:
    grouped: dict[str, list[dict[str, Any]]] = {}
    for row in per_query:
        scenario = str(row.get("target_scenario", "")).strip() or "unknown"
        grouped.setdefault(scenario, []).append(row)
    return sorted(
        ((scenario, summarize_query_subset(rows)) for scenario, rows in grouped.items()),
        key=lambda item: item[0],
    )


def summarize_query_subset(rows: list[dict[str, Any]]) -> dict[str, float]:
    count = len(rows)
    if count == 0:
        return {"count": 0.0, "recall@1": 0.0, "recall@5": 0.0, "mrr": 0.0}

    hits_at_1 = 0
    hits_at_5 = 0
    reciprocal_rank_sum = 0.0
    for row in rows:
        rank = row.get("rank")
        if isinstance(rank, int):
            if rank <= 1:
                hits_at_1 += 1
            if rank <= 5:
                hits_at_5 += 1
            reciprocal_rank_sum += 1.0 / rank

    return {
        "count": float(count),
        "recall@1": hits_at_1 / count,
        "recall@5": hits_at_5 / count,
        "mrr": reciprocal_rank_sum / count,
    }


def format_subset_line(label: str, summary: dict[str, float]) -> str:
    return (
        f"{label}: "
        f"n={int(summary['count'])} "
        f"r@1={summary['recall@1']:.3f} "
        f"r@5={summary['recall@5']:.3f} "
        f"mrr={summary['mrr']:.3f}"
    )


def first_result(row: dict[str, Any]) -> dict[str, Any]:
    results = list(row.get("results", []))
    if not results:
        return {}
    return dict(results[0])


def group_episodes_by_scenario_cluster(
    episodes: list[dict[str, Any]],
) -> dict[str, dict[str, list[dict[str, Any]]]]:
    groups: dict[str, dict[str, list[dict[str, Any]]]] = {}
    for index, episode in enumerate(episodes, start=1):
        scenario = str(episode.get("scenario", "")).strip()
        cluster_id = str(episode.get("cluster_id", "")).strip()
        if not scenario:
            scenario = "unknown"
        if not cluster_id:
            cluster_id = derive_cluster_id(episode, index)
            episode["cluster_id"] = cluster_id
        groups.setdefault(scenario, {}).setdefault(cluster_id, []).append(episode)
    return groups


def compute_group_split_counts(
    total_groups: int,
    train_ratio: float,
    val_ratio: float,
    *,
    min_test_groups: int = 1,
    min_val_groups: int = 1,
) -> tuple[int, int, int]:
    if total_groups <= 0:
        return 0, 0, 0

    train_count = int(total_groups * train_ratio)
    val_count = int(total_groups * val_ratio)
    test_count = total_groups - train_count - val_count

    if train_count <= 0:
        train_count = 1
        if test_count > 0:
            test_count -= 1
        elif val_count > 0:
            val_count -= 1

    max_test_groups = max(total_groups - 1, 0)
    min_test_groups = min(min_test_groups, max_test_groups)
    max_val_groups = max(total_groups - 1 - min_test_groups, 0)
    min_val_groups = min(min_val_groups, max_val_groups)

    if test_count < min_test_groups:
        deficit = min_test_groups - test_count
        take_from_train = min(deficit, max(train_count - 1, 0))
        train_count -= take_from_train
        test_count += take_from_train
        deficit -= take_from_train
        if deficit > 0:
            take_from_val = min(deficit, max(val_count - min_val_groups, 0))
            val_count -= take_from_val
            test_count += take_from_val

    if val_count < min_val_groups:
        deficit = min_val_groups - val_count
        take_from_train = min(deficit, max(train_count - 1, 0))
        train_count -= take_from_train
        val_count += take_from_train
        deficit -= take_from_train
        if deficit > 0:
            take_from_test = min(deficit, max(test_count - min_test_groups, 0))
            test_count -= take_from_test
            val_count += take_from_test

    test_count = max(total_groups - train_count - val_count, 0)
    return train_count, val_count, test_count


def generate_episode_bundle_in_batches(
    *,
    client: DeepSeekClient,
    schema: dict[str, Any],
    episode_count: int,
    batch_episode_count: int,
    seed: int,
    model: str,
    temperature: float,
    timeout_seconds: float,
    max_tokens: int,
    log,
) -> dict[str, Any]:
    remaining_episodes = episode_count
    batch_index = 0
    schema_text = json.dumps(schema, ensure_ascii=False, indent=2)
    global_plan = build_generation_plan(
        episode_count=episode_count,
        query_count=0,
        seed=seed,
    )
    merged_bundle: dict[str, Any] = {
        "schema_version": "agent-memory/episode-spec-bundle/v1",
        "episodes": [],
    }

    while remaining_episodes > 0:
        batch_index += 1
        target_episode_count = min(batch_episode_count, remaining_episodes)

        log(f"starting batch {batch_index}: episodes={target_episode_count}")
        generation_plan = slice_generation_plan(
            global_plan,
            episode_offset=len(merged_bundle["episodes"]),
            episode_count=target_episode_count,
            query_offset=0,
            query_count=0,
        )
        log(
            "batch generation matrix: "
            + json.dumps(generation_plan["coverage_summary"], ensure_ascii=False),
            color=typer.colors.BLUE,
        )
        batch_bundle = generate_episode_bundle_from_plan(
            client=client,
            generation_plan=generation_plan,
            schema_text=schema_text,
            model=model,
            seed=seed + batch_index - 1,
            temperature=temperature,
            timeout_seconds=timeout_seconds,
            max_tokens=max_tokens,
            log=log,
            batch_label=f"batch {batch_index}",
        )
        batch_bundle = normalize_bundle_for_schema(batch_bundle)
        validate_with_schema(batch_bundle, "episode_spec_bundle.schema.json")

        generated_episodes = list(batch_bundle.get("episodes", []))
        merged_bundle["episodes"].extend(generated_episodes[:target_episode_count])
        remaining_episodes -= target_episode_count

        log(
            f"finished batch {batch_index}: accumulated "
            f"{len(merged_bundle['episodes'])}/{episode_count} episodes",
            color=typer.colors.GREEN,
        )

    return merged_bundle


def generate_episode_bundle_from_plan(
    *,
    client: DeepSeekClient,
    generation_plan: dict[str, Any],
    schema_text: str,
    model: str,
    seed: int,
    temperature: float,
    timeout_seconds: float,
    max_tokens: int,
    log,
    batch_label: str,
) -> dict[str, Any]:
    episode_blueprints = list(generation_plan.get("episode_blueprints", []))
    episode_count = len(episode_blueprints)

    try:
        return client.generate_episode_bundle(
            model=model,
            episode_count=episode_count,
            seed=seed,
            schema_text=schema_text,
            generation_plan=generation_plan,
            temperature=temperature,
            timeout_seconds=timeout_seconds,
            max_tokens=max_tokens,
            progress=log,
        )
    except Exception as exc:
        if episode_count <= 1:
            raise

        left_plan, right_plan = split_generation_plan_for_retry(generation_plan)
        if not left_plan or not right_plan:
            raise

        log(
            f"{batch_label} failed with {type(exc).__name__}: {exc}. "
            "Retrying with two smaller sub-batches.",
            color=typer.colors.YELLOW,
        )

        left_bundle = generate_episode_bundle_from_plan(
            client=client,
            generation_plan=left_plan,
            schema_text=schema_text,
            model=model,
            seed=seed,
            temperature=temperature,
            timeout_seconds=timeout_seconds,
            max_tokens=max_tokens,
            log=log,
            batch_label=f"{batch_label}.a",
        )
        right_bundle = generate_episode_bundle_from_plan(
            client=client,
            generation_plan=right_plan,
            schema_text=schema_text,
            model=model,
            seed=seed + 1000,
            temperature=temperature,
            timeout_seconds=timeout_seconds,
            max_tokens=max_tokens,
            log=log,
            batch_label=f"{batch_label}.b",
        )
        return {
            "schema_version": generation_plan.get(
                "schema_version", "agent-memory/episode-spec-bundle/v1"
            ),
            "episodes": list(left_bundle.get("episodes", []))
            + list(right_bundle.get("episodes", [])),
        }


def split_generation_plan_for_retry(
    generation_plan: dict[str, Any],
) -> tuple[dict[str, Any] | None, dict[str, Any] | None]:
    episode_blueprints = list(generation_plan.get("episode_blueprints", []))
    if len(episode_blueprints) <= 1:
        return None, None

    midpoint = max(1, len(episode_blueprints) // 2)
    left_episodes = episode_blueprints[:midpoint]
    right_episodes = episode_blueprints[midpoint:]
    if not left_episodes or not right_episodes:
        return None, None

    left_plan = {
        "seed": generation_plan.get("seed"),
        "episode_count": len(left_episodes),
        "query_count": 0,
        "episode_blueprints": left_episodes,
        "query_blueprints": [],
        "coverage_summary": generation_plan_summary(left_episodes, []),
    }
    right_plan = {
        "seed": generation_plan.get("seed"),
        "episode_count": len(right_episodes),
        "query_count": 0,
        "episode_blueprints": right_episodes,
        "query_blueprints": [],
        "coverage_summary": generation_plan_summary(right_episodes, []),
    }
    return left_plan, right_plan


def generation_plan_summary(
    episode_blueprints: list[dict[str, Any]],
    query_blueprints: list[dict[str, Any]],
) -> dict[str, Any]:
    episode_groups: dict[str, int] = {}
    query_groups: dict[str, int] = {}
    for episode in episode_blueprints:
        key = f"{episode.get('scenario', '')}/{episode.get('intent', '')}"
        episode_groups[key] = episode_groups.get(key, 0) + 1
    for query in query_blueprints:
        key = f"{query.get('target_scenario', '')}/{query.get('target_intent', '')}"
        query_groups[key] = query_groups.get(key, 0) + 1
    return {
        "episode_groups": episode_groups,
        "query_groups": query_groups,
    }


def generate_queries_for_split_groups(
    *,
    client: DeepSeekClient,
    episodes: list[dict[str, Any]],
    split_name: str,
    model: str,
    temperature: float,
    timeout_seconds: float,
    max_tokens: int,
    queries_per_episode: int,
    query_style: str,
    log,
) -> list[dict[str, Any]]:
    grouped_episodes = group_episodes_by_target(episodes)
    generated_queries: list[dict[str, Any]] = []

    for index, (group_key, group_episodes) in enumerate(sorted(grouped_episodes.items())):
        scenario, intent = group_key
        ordered_group = sorted(
            group_episodes,
            key=lambda episode: str(episode.get("episode_id", "")).strip(),
        )
        log(
            f"generating queries for {split_name} group {scenario}/{intent}: "
            f"{len(ordered_group)} episodes",
            color=typer.colors.BLUE,
        )
        response = client.generate_queries_for_split(
            model=model,
            episodes=ordered_group,
            split_name=split_name,
            query_style=query_style,
            queries_per_episode=queries_per_episode,
            seed=index + 1,
            temperature=temperature,
            timeout_seconds=timeout_seconds,
            max_tokens=max_tokens,
            progress=log,
        )
        batch_queries = normalize_generated_queries(list(response.get("queries", [])))
        validate_generated_queries(
            queries=batch_queries,
            episodes=ordered_group,
            queries_per_episode=queries_per_episode,
            split_name=split_name,
            group_key=group_key,
        )
        batch_bundle = {
            "schema_version": "agent-memory/episode-spec-bundle/v1",
            "episodes": ordered_group,
            "queries": batch_queries,
        }
        batch_bundle = normalize_bundle_for_schema(batch_bundle)
        try:
            validate_with_schema(batch_bundle, "episode_spec_bundle.schema.json")
        except ValueError as exc:
            sample = json.dumps(batch_queries[:3], ensure_ascii=False, indent=2)
            raise typer.BadParameter(
                f"generated split queries for {split_name} group {group_key} "
                f"failed schema validation:\n{exc}\nSample queries:\n{sample}"
            ) from exc
        lint_issues = lint_bundle(batch_bundle)
        if lint_issues:
            raise typer.BadParameter(
                "generated split queries failed research lint:\n"
                + "\n".join(lint_issues[:20])
            )
        generated_queries.extend(batch_bundle["queries"])

    return generated_queries


def normalize_generated_queries(queries: list[dict[str, Any]]) -> list[dict[str, Any]]:
    normalized: list[dict[str, Any]] = []
    query_field_candidates = ("query", "text", "question", "query_text", "prompt")

    for entry in queries:
        if not isinstance(entry, dict):
            normalized.append(entry)
            continue

        normalized_entry = dict(entry)
        query_value = ""
        for field_name in query_field_candidates:
            value = normalized_entry.get(field_name)
            if isinstance(value, str) and value.strip():
                query_value = value.strip()
                break
        if query_value:
            normalized_entry["query"] = query_value

        normalized.append(normalized_entry)

    return normalized


def validate_generated_queries(
    *,
    queries: list[dict[str, Any]],
    episodes: list[dict[str, Any]],
    queries_per_episode: int,
    split_name: str,
    group_key: tuple[str, str],
) -> None:
    if not isinstance(queries, list):
        raise typer.BadParameter("generated queries payload must be a list")

    expected_query_count = len(episodes) * queries_per_episode
    if len(queries) != expected_query_count:
        raise typer.BadParameter(
            f"generated {len(queries)} queries for {split_name} group {group_key}, "
            f"expected {expected_query_count}"
        )

    expected_episode_ids = {
        str(episode.get("episode_id", "")).strip()
        for episode in episodes
        if str(episode.get("episode_id", "")).strip()
    }
    counts_by_episode_id = {episode_id: 0 for episode_id in expected_episode_ids}

    for query in queries:
        if not isinstance(query, dict):
            raise typer.BadParameter("generated query entries must be objects")
        target_episode_ids = [
            str(value).strip()
            for value in query.get("target_episode_ids", [])
            if str(value).strip()
        ]
        if len(target_episode_ids) != 1:
            raise typer.BadParameter(
                f"generated query for {split_name} group {group_key} must target exactly one episode_id"
            )
        target_episode_id = target_episode_ids[0]
        if target_episode_id not in expected_episode_ids:
            raise typer.BadParameter(
                f"generated query references episode_id {target_episode_id!r} outside "
                f"{split_name} group {group_key}"
            )
        counts_by_episode_id[target_episode_id] += 1

    wrong_counts = {
        episode_id: count
        for episode_id, count in counts_by_episode_id.items()
        if count != queries_per_episode
    }
    if wrong_counts:
        raise typer.BadParameter(
            f"generated query counts do not match queries_per_episode={queries_per_episode} "
            f"for {split_name} group {group_key}: {wrong_counts}"
        )


def infer_split_name(input_path: Path) -> str:
    filename = input_path.name.lower()
    if filename.startswith("train"):
        return "train"
    if filename.startswith("val"):
        return "val"
    if filename.startswith("test"):
        return "test"
    return "unknown"


def ensure_episode_ids(payload: dict[str, Any]) -> None:
    episodes = payload.get("episodes", [])
    if not isinstance(episodes, list):
        return

    used_ids: set[str] = set()
    for index, episode in enumerate(episodes, start=1):
        if not isinstance(episode, dict):
            continue
        episode_id = str(episode.get("episode_id", "")).strip()
        if not episode_id:
            episode_id = make_episode_id(
                str(episode.get("scenario", "")).strip(), index, used_ids
            )
            episode["episode_id"] = episode_id
        if episode_id in used_ids:
            episode_id = make_episode_id(episode_id, index, used_ids)
            episode["episode_id"] = episode_id
        used_ids.add(episode_id)


def ensure_cluster_ids(payload: dict[str, Any]) -> None:
    episodes = payload.get("episodes", [])
    if not isinstance(episodes, list):
        return

    for index, episode in enumerate(episodes, start=1):
        if not isinstance(episode, dict):
            continue
        cluster_id = str(episode.get("cluster_id", "")).strip()
        if cluster_id:
            episode["cluster_id"] = cluster_id
            continue
        episode["cluster_id"] = derive_cluster_id(episode, index)


def ensure_query_target_episode_ids(
    payload: dict[str, Any], *, force: bool = False
) -> None:
    episodes = payload.get("episodes", [])
    queries = payload.get("queries", [])
    if not isinstance(episodes, list) or not isinstance(queries, list):
        return
    episode_by_id = {
        str(episode.get("episode_id", "")).strip(): episode
        for episode in episodes
        if isinstance(episode, dict) and str(episode.get("episode_id", "")).strip()
    }

    for query in queries:
        if not isinstance(query, dict):
            continue
        target_episode_ids = [
            str(value).strip()
            for value in query.get("target_episode_ids", [])
            if str(value).strip()
        ]
        if target_episode_ids and not force:
            query["target_episode_ids"] = target_episode_ids
        else:
            resolved_target_episode_ids = resolve_query_target_episode_ids(
                query, episodes
            )
            if resolved_target_episode_ids:
                query["target_episode_ids"] = resolved_target_episode_ids
                target_episode_ids = resolved_target_episode_ids

        if not target_episode_ids:
            continue

        target_episode = episode_by_id.get(target_episode_ids[0])
        if target_episode is None:
            continue
        if not str(query.get("target_scenario", "")).strip():
            query["target_scenario"] = str(target_episode.get("scenario", "")).strip()
        if not str(query.get("target_intent", "")).strip():
            query["target_intent"] = str(target_episode.get("intent", "")).strip()


def resolve_query_target_episode_ids(
    query: dict[str, Any], episodes: list[dict[str, Any]]
) -> list[str]:
    target_scenario = str(query.get("target_scenario", "")).strip()
    target_intent = str(query.get("target_intent", "")).strip()
    entity = str(query.get("entity", "")).strip()
    status = str(query.get("status", "")).strip()
    query_tags = {str(tag).strip() for tag in query.get("tags", []) if str(tag).strip()}

    candidates: list[tuple[int, str]] = []
    for episode in episodes:
        if not isinstance(episode, dict):
            continue
        episode_id = str(episode.get("episode_id", "")).strip()
        if not episode_id:
            continue
        if (
            target_scenario
            and str(episode.get("scenario", "")).strip() != target_scenario
        ):
            continue
        if target_intent and str(episode.get("intent", "")).strip() != target_intent:
            continue

        score = 0
        if status and str(episode.get("status", "")).strip() == status:
            score += 3
        if entity and episode_matches_entity(episode, entity):
            score += 5

        episode_tags = {
            str(tag).strip() for tag in episode.get("tags", []) if str(tag).strip()
        }
        score += len(query_tags & episode_tags)
        candidates.append((score, episode_id))

    if not candidates:
        return []

    candidates.sort(key=lambda item: (-item[0], item[1]))
    return [candidates[0][1]]


def episode_matches_entity(episode: dict[str, Any], entity: str) -> bool:
    for ref in episode.get("entities", []):
        if not isinstance(ref, dict):
            continue
        if str(ref.get("name", "")).strip() == entity:
            return True

    metadata = episode.get("metadata", {})
    if isinstance(metadata, dict):
        for value in metadata.values():
            if str(value).strip() == entity:
                return True

    searchable_fields = (
        str(episode.get("goal", "")).strip(),
        str(episode.get("summary", "")).strip(),
    )
    return any(entity in value for value in searchable_fields if value)


def make_episode_id(seed: str, index: int, used_ids: set[str]) -> str:
    base = slugify_token(seed) or "episode"
    candidate = f"ep_{base}_{index:03d}"
    suffix = index
    while candidate in used_ids:
        suffix += 1
        candidate = f"ep_{base}_{suffix:03d}"
    return candidate


def slugify_token(value: str) -> str:
    chars = []
    previous_was_sep = False
    for char in value.lower():
        if char.isalnum():
            chars.append(char)
            previous_was_sep = False
            continue
        if previous_was_sep:
            continue
        chars.append("_")
        previous_was_sep = True
    return "".join(chars).strip("_")


def normalize_bundle_for_schema(
    payload: dict[str, Any], *, force_retarget_queries: bool = False
) -> dict[str, Any]:
    ensure_episode_ids(payload)
    ensure_cluster_ids(payload)
    ensure_query_target_episode_ids(payload, force=force_retarget_queries)

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


def derive_cluster_id(episode: dict[str, Any], index: int) -> str:
    scenario_slug = slugify_token(str(episode.get("scenario", "")).strip())
    episode_id = str(episode.get("episode_id", "")).strip()
    if episode_id.startswith("ep_"):
        episode_id = episode_id[3:]

    if scenario_slug and episode_id:
        marker = f"{scenario_slug}_c"
        start = episode_id.find(marker)
        if start >= 0:
            cluster_id = consume_cluster_slug(episode_id[start:])
            if cluster_id:
                return cluster_id

    if scenario_slug:
        return f"{scenario_slug}_singleton_{index:03d}"
    return f"episode_singleton_{index:03d}"


def consume_cluster_slug(value: str) -> str:
    parts = [part.strip() for part in value.split("_") if part.strip()]
    cluster_parts: list[str] = []
    for part in parts:
        cluster_parts.append(part)
        if is_cluster_counter_token(part):
            return "_".join(cluster_parts)
    return ""


def is_cluster_counter_token(value: str) -> bool:
    return len(value) >= 2 and value[0] == "c" and value[1:].isdigit()


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
    target_episode_ids = {
        str(value).strip()
        for value in query_spec.get("target_episode_ids", [])
        if str(value).strip()
    }
    if not target_episode_ids:
        return None

    for index, point in enumerate(results, start=1):
        episode = extract_episode(point)
        episode_id = str(episode.get("id", "")).strip()
        if episode_id in target_episode_ids:
            return index
    return None


def extract_episode(point: dict[str, Any]) -> dict[str, Any]:
    payload = point.get("payload", {})
    episode = payload.get("episode", {})
    return episode if isinstance(episode, dict) else {}
