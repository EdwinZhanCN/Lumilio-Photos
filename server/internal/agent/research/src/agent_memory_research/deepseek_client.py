from __future__ import annotations

import json
import os
import time
from collections.abc import Callable
from typing import Any

import httpx

from .generation_matrix import (
    ALLOWED_OUTPUT_KINDS,
    INTENT_VOCAB,
    MEDIA_TOOLS,
    SCENARIO_VOCAB,
    format_plan_for_prompt,
)


class DeepSeekClient:
    def __init__(
        self, api_key: str, base_url: str = "https://api.deepseek.com"
    ) -> None:
        if not api_key.strip():
            raise ValueError("DeepSeek API key is required")
        self.api_key = api_key
        self.base_url = base_url.rstrip("/")

    @classmethod
    def from_env(cls, api_key_env: str, base_url: str) -> "DeepSeekClient":
        api_key = os.getenv(api_key_env, "").strip()
        if not api_key:
            raise ValueError(f"environment variable {api_key_env} is required")
        return cls(api_key=api_key, base_url=base_url)

    def generate_spec_bundle(
        self,
        *,
        model: str,
        episode_count: int,
        query_count: int,
        seed: int,
        schema_text: str,
        generation_plan: dict[str, Any],
        temperature: float,
        timeout_seconds: float = 120.0,
        max_tokens: int = 7000,
        progress: Callable[[str], None] | None = None,
    ) -> dict[str, Any]:
        system_prompt = (
            "You generate research-quality synthetic datasets for a media-management agent. "
            "Return only valid JSON. Do not wrap the response in markdown. "
            "Use only the approved mock tools and keep scenarios grounded in photo/media workflows."
        )
        user_prompt = (
            f"Generate a spec bundle with exactly {episode_count} episodes and {query_count} queries.\n"
            f"The schema version must be agent-memory/episode-spec-bundle/v1.\n"
            f"Allowed tool names: {', '.join(MEDIA_TOOLS)}.\n"
            f"Allowed tool_name -> output_kind mapping: {json.dumps(ALLOWED_OUTPUT_KINDS, ensure_ascii=False)}.\n"
            f"Allowed intent values: {', '.join(INTENT_VOCAB)}.\n"
            f"Allowed scenario values: {', '.join(SCENARIO_VOCAB)}.\n"
            "Every episode must contain a unique symbolic episode_id such as "
            "'ep_inspect_camera_metadata_canon_r5_001'.\n"
            "Every episode must include cluster_id. Episodes in the same minimal-difference cluster must reuse the same cluster_id.\n"
            "Every query must include target_episode_ids as a one-element array containing exactly one "
            "exact episode_id value from the generated episode set.\n"
            f"Vary entities such as location, camera_model, album, failure_mode, rating, liked state, and time windows.\n"
            f"Include both successful and recovered episodes, but no aborted episodes.\n"
            "Use compact symbolic labels for scenario and intent. Do not write them as long natural-language sentences.\n"
            "query.entity must be a concrete entity value such as 'Paris', 'Yosemite', or 'Johnson Wedding', not an entity type like 'location' or 'album'.\n"
            "Do not invent output_kind labels outside the allowed mapping.\n"
            "Avoid repetitive traces such as calling mock_create_album multiple times in a row unless the goal explicitly compares albums.\n"
            "Ensure the query set covers every scenario+intent group present in the episode set at least once.\n"
            "Queries should target specific prior episodes, not just the broad task family.\n"
            "Each query blueprint includes required_slots and hard_negative_episode_ids. "
            "Express every required slot explicitly in the query using natural language.\n"
            "If minimal_difference_axis is not 'baseline', the query must explicitly express that differentiating axis.\n"
            "Before finalizing each query, compare it against the target episode and the listed hard negatives in the generation plan. "
            "Rewrite the query until it can only match the target episode within that neighborhood.\n"
            "Follow the supplied generation plan exactly. Do not invent extra clusters or replace the provided episode_id values.\n"
            f"Queries must paraphrase the task goals instead of copying them verbatim.\n"
            f"Use this JSON schema as the contract:\n{schema_text}\n"
            f"Use this generation plan as the required batch matrix:\n{format_plan_for_prompt(generation_plan)}\n"
            f"Use seed hint {seed} for deterministic variety."
        )

        attempts = 3
        delay_seconds = 1.0

        for attempt in range(1, attempts + 1):
            if progress is not None:
                progress(
                    f"sending DeepSeek request attempt {attempt}/{attempts} "
                    f"(episodes={episode_count}, queries={query_count}, model={model})"
                )

            try:
                response = httpx.post(
                    f"{self.base_url}/chat/completions",
                    headers={
                        "Authorization": f"Bearer {self.api_key}",
                        "Content-Type": "application/json",
                    },
                    json={
                        "model": model,
                        "temperature": temperature,
                        "max_tokens": max_tokens,
                        "response_format": {"type": "json_object"},
                        "messages": [
                            {"role": "system", "content": system_prompt},
                            {"role": "user", "content": user_prompt},
                        ],
                    },
                    timeout=timeout_seconds,
                )
                response.raise_for_status()
                payload = response.json()
                content = payload["choices"][0]["message"]["content"]
                content = content.strip()
                if content.startswith("```"):
                    content = content.strip("`")
                    content = content.replace("json\n", "", 1).strip()
                result = json.loads(content)
                if progress is not None:
                    progress(
                        f"received and parsed DeepSeek response on attempt {attempt}"
                    )
                return result
            except (httpx.HTTPError, ValueError) as exc:
                if attempt >= attempts:
                    if progress is not None:
                        progress(f"final attempt failed: {exc}")
                    raise

                if progress is not None:
                    progress(
                        f"attempt {attempt} failed: {exc}. retrying in {delay_seconds:.0f}s"
                    )
                time.sleep(delay_seconds)
                delay_seconds = min(delay_seconds * 2, 8.0)

        raise RuntimeError("unreachable")
