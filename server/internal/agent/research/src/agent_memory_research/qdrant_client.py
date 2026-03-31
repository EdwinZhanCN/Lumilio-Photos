from __future__ import annotations

from typing import Any

import httpx


class QdrantClient:
    def __init__(
        self,
        *,
        base_url: str = "http://localhost:6333",
        collection: str,
        api_key: str = "",
        timeout: float = 30.0,
    ) -> None:
        if not collection.strip():
            raise ValueError("Qdrant collection is required")
        self.base_url = base_url.rstrip("/")
        self.collection = collection
        self.client = httpx.Client(
            timeout=timeout,
            headers={"api-key": api_key} if api_key.strip() else None,
        )

    def search(
        self,
        *,
        vector: list[float],
        limit: int,
        user_id: str = "",
        goal: str = "",
        intent: str = "",
        entity: str = "",
        status: str = "",
        tags: list[str] | None = None,
    ) -> list[dict[str, Any]]:
        body: dict[str, Any] = {
            "query": vector,
            "limit": limit,
            "with_payload": True,
            "with_vector": False,
        }
        filter_payload = build_filter(
            user_id=user_id,
            goal=goal,
            intent=intent,
            entity=entity,
            status=status,
            tags=tags or [],
        )
        if filter_payload is not None:
            body["filter"] = filter_payload

        response = self.client.post(
            f"{self.base_url}/collections/{self.collection}/points/query",
            json=body,
        )
        response.raise_for_status()
        payload = response.json()
        result = payload.get("result", {})
        return list(result.get("points", []))


def build_filter(
    *,
    user_id: str = "",
    goal: str = "",
    intent: str = "",
    entity: str = "",
    status: str = "",
    tags: list[str] | None = None,
) -> dict[str, Any] | None:
    must: list[dict[str, Any]] = []

    def append_match(key: str, value: str) -> None:
        if not value.strip():
            return
        must.append({"key": key, "match": {"value": value}})

    append_match("user_id", user_id)
    append_match("goal", goal)
    append_match("intent", intent)
    append_match("entity_names", entity)
    append_match("status", status)

    # Tags are too noisy to use as hard filters in retrieval benchmarks.
    # Keep entity/status as exact constraints and let tags influence evaluation later.
    _ = tags

    if not must:
        return None
    return {"must": must}
