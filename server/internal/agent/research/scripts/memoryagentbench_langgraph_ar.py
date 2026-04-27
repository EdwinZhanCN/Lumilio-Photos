#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "datasets>=2.20.0",
#   "editdistance>=0.8.1",
#   "httpx>=0.27.0",
#   "langgraph>=1.0.0",
#   "nltk>=3.9.0",
#   "numpy>=1.26.0",
#   "openai>=1.50.0",
#   "pyyaml>=6.0.0",
#   "redis>=5.0.0",
#   "rouge-score>=0.1.2",
#   "tiktoken>=0.7.0",
# ]
# ///
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
import time
import uuid
from collections import defaultdict
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Literal, Protocol, TypedDict

import httpx
import numpy as np
import tiktoken
import yaml
from langgraph.graph import END, START, StateGraph
from openai import OpenAI


DEFAULT_DATASET_CONFIG = "Accurate_Retrieval/Ruler/QA/Ruler_qa1_197k.yaml"
DEFAULT_COLLECTION = "memoryagentbench_langgraph_ar"
DEFAULT_AGENT_NAME = "Embedding_rag_qwen3_lora"
DEFAULT_EPISODE_SCENARIO = "read_corpus"
DEFAULT_EPISODE_INTENT = "retain_evidence_for_future_qa"
DEFAULT_EPISODE_TOOL_NAME = "read_text"
EPISODE_WRITER_PROMPT_VERSION = "mab-episode-writer-v1"
EPISODE_QUERY_REWRITER_PROMPT_VERSION = "mab-episode-query-rewriter-v1"
EPISODE_ENTITY_TYPES = (
    "person",
    "place",
    "organization",
    "date",
    "event",
    "work",
    "concept",
    "object",
    "number",
    "other",
)


class MemoryGraphState(TypedDict, total=False):
    mode: Literal["update", "query"]
    text: str
    context_id: int
    chunk_id: int
    query_id: int
    output: dict[str, Any]


class KVStore(Protocol):
    def set_json(self, key: str, value: dict[str, Any], *, run_id: str, context_id: int) -> None:
        ...

    def get_json(self, key: str) -> dict[str, Any] | None:
        ...

    def clear_context(self, *, run_id: str, context_id: int) -> None:
        ...


class DictKVStore:
    def __init__(self) -> None:
        self._items: dict[str, dict[str, Any]] = {}
        self._context_keys: dict[tuple[str, int], set[str]] = defaultdict(set)

    def set_json(self, key: str, value: dict[str, Any], *, run_id: str, context_id: int) -> None:
        self._items[key] = value
        self._context_keys[(run_id, context_id)].add(key)

    def get_json(self, key: str) -> dict[str, Any] | None:
        return self._items.get(key)

    def clear_context(self, *, run_id: str, context_id: int) -> None:
        context_key = (run_id, context_id)
        for key in self._context_keys.pop(context_key, set()):
            self._items.pop(key, None)


class RedisKVStore:
    def __init__(self, *, url: str, prefix: str) -> None:
        import redis

        self._client = redis.Redis.from_url(url)
        self._prefix = prefix.rstrip(":")

    def set_json(self, key: str, value: dict[str, Any], *, run_id: str, context_id: int) -> None:
        _ = run_id, context_id
        self._client.set(key, json.dumps(value, ensure_ascii=False))

    def get_json(self, key: str) -> dict[str, Any] | None:
        value = self._client.get(key)
        if value is None:
            return None
        if isinstance(value, bytes):
            value = value.decode("utf-8")
        return dict(json.loads(value))

    def clear_context(self, *, run_id: str, context_id: int) -> None:
        pattern = f"{self._prefix}:{run_id}:{context_id}:*"
        keys = list(self._client.scan_iter(match=pattern))
        if keys:
            self._client.delete(*keys)


class OllamaCompatibleEmbedder:
    def __init__(
        self,
        *,
        base_url: str,
        model: str,
        dimensions: int,
        keep_alive: str,
        timeout: float,
        batch_size: int,
    ) -> None:
        if not model.strip():
            raise ValueError("embedding model is required")
        if dimensions <= 0:
            raise ValueError("embedding dimensions must be positive")

        self.base_url = base_url.rstrip("/")
        self.model = model
        self.dimensions = dimensions
        self.keep_alive = keep_alive
        self.batch_size = batch_size
        self.client = httpx.Client(timeout=timeout)

    def embed_text(self, text: str) -> list[float]:
        return self.embed_texts([text])[0]

    def embed_texts(self, texts: list[str]) -> list[list[float]]:
        embeddings: list[list[float]] = []
        for start in range(0, len(texts), self.batch_size):
            batch = texts[start : start + self.batch_size]
            embeddings.extend(self._embed_batch(batch))
        return embeddings

    def _embed_batch(self, texts: list[str]) -> list[list[float]]:
        safe_texts = [text if text.strip() else " " for text in texts]
        body: dict[str, Any] = {
            "model": self.model,
            "input": safe_texts if len(safe_texts) > 1 else safe_texts[0],
            "dimensions": self.dimensions,
        }
        if self.keep_alive.strip():
            body["keep_alive"] = self.keep_alive

        response = self.client.post(f"{self.base_url}/api/embed", json=body)
        response.raise_for_status()
        payload = response.json()
        raw_embeddings = payload.get("embeddings")
        if not raw_embeddings:
            raw_embedding = payload.get("embedding")
            if raw_embedding:
                raw_embeddings = [raw_embedding]
        if not raw_embeddings:
            raise ValueError("embedding response did not include embeddings")

        vectors = [[float(value) for value in vector] for vector in raw_embeddings]
        if len(vectors) != len(safe_texts):
            raise ValueError(f"embedding count mismatch: expected {len(safe_texts)}, got {len(vectors)}")
        for vector in vectors:
            if len(vector) != self.dimensions:
                raise ValueError(
                    f"embedding dimension mismatch: expected {self.dimensions}, got {len(vector)}"
                )
        return vectors


class QdrantVectorStore:
    def __init__(
        self,
        *,
        base_url: str,
        api_key: str,
        collection: str,
        dimensions: int,
        distance: str,
        run_id: str,
        timeout: float,
    ) -> None:
        if not collection.strip():
            raise ValueError("Qdrant collection is required")

        self.base_url = base_url.rstrip("/")
        self.collection = collection
        self.dimensions = dimensions
        self.distance = distance
        self.run_id = run_id
        self.client = httpx.Client(
            timeout=timeout,
            headers={"api-key": api_key} if api_key.strip() else None,
        )

    def ensure_collection(self, *, recreate: bool) -> None:
        collection_url = f"{self.base_url}/collections/{self.collection}"
        if recreate:
            response = self.client.delete(collection_url)
            if response.status_code not in (200, 202, 404):
                response.raise_for_status()

        response = self.client.get(collection_url)
        if response.status_code == 200:
            return
        if response.status_code != 404:
            response.raise_for_status()

        create_body = {
            "vectors": {
                "size": self.dimensions,
                "distance": self.distance,
            }
        }
        response = self.client.put(collection_url, json=create_body)
        response.raise_for_status()

    def delete_context(self, *, context_id: int) -> None:
        body = {"filter": self._context_filter(context_id)}
        response = self.client.post(
            f"{self.base_url}/collections/{self.collection}/points/delete",
            params={"wait": "true"},
            json=body,
        )
        response.raise_for_status()

    def upsert_points(self, points: list[dict[str, Any]]) -> None:
        if not points:
            return
        response = self.client.put(
            f"{self.base_url}/collections/{self.collection}/points",
            params={"wait": "true"},
            json={"points": points},
        )
        response.raise_for_status()

    def query(self, *, vector: list[float], context_id: int, limit: int) -> list[dict[str, Any]]:
        body: dict[str, Any] = {
            "query": vector,
            "limit": limit,
            "filter": self._context_filter(context_id),
            "with_payload": True,
            "with_vector": False,
        }
        response = self.client.post(
            f"{self.base_url}/collections/{self.collection}/points/query",
            json=body,
        )
        response.raise_for_status()
        result = response.json().get("result", {})
        if isinstance(result, dict):
            return list(result.get("points", result.get("hits", [])))
        return list(result)

    def _context_filter(self, context_id: int) -> dict[str, Any]:
        return {
            "must": [
                {"key": "run_id", "match": {"value": self.run_id}},
                {"key": "context_id", "match": {"value": context_id}},
            ]
        }


@dataclass(frozen=True)
class MemoryRecord:
    memory_id: str
    kv_key: str
    text: str
    metadata: dict[str, Any]


class MemoryBackend:
    def __init__(
        self,
        *,
        run_id: str,
        namespace: str,
        embedder: OllamaCompatibleEmbedder,
        vector_store: QdrantVectorStore,
        kv_store: KVStore,
        episode_writer: EpisodeWriter | None = None,
        episode_config: EpisodeMemoryConfig | None = None,
    ) -> None:
        self.run_id = run_id
        self.namespace = namespace.rstrip(":")
        self.embedder = embedder
        self.vector_store = vector_store
        self.kv_store = kv_store
        self.episode_writer = episode_writer
        self.episode_config = episode_config

    def clear_context(self, *, context_id: int) -> None:
        self.vector_store.delete_context(context_id=context_id)
        self.kv_store.clear_context(run_id=self.run_id, context_id=context_id)

    def insert(self, *, text: str, context_id: int, chunk_id: int) -> MemoryRecord:
        if self.episode_writer is not None:
            return self._insert_episode(text=text, context_id=context_id, chunk_id=chunk_id)
        return self._insert_raw(text=text, context_id=context_id, chunk_id=chunk_id)

    def _insert_raw(self, *, text: str, context_id: int, chunk_id: int) -> MemoryRecord:
        memory_id = str(uuid.uuid5(uuid.NAMESPACE_URL, f"{self.run_id}:{context_id}:{chunk_id}"))
        kv_key = f"{self.namespace}:{self.run_id}:{context_id}:{memory_id}"
        metadata = {
            "run_id": self.run_id,
            "context_id": context_id,
            "chunk_id": chunk_id,
            "memory_id": memory_id,
            "kv_key": kv_key,
            "memory_mode": "raw",
        }
        record = MemoryRecord(memory_id=memory_id, kv_key=kv_key, text=text, metadata=metadata)
        vector = self.embedder.embed_text(text)
        point = {
            "id": memory_id,
            "vector": vector,
            "payload": {
                **metadata,
                "text_preview": text[:1024],
                "text_sha256": hashlib.sha256(text.encode("utf-8")).hexdigest(),
            },
        }
        self.kv_store.set_json(
            kv_key,
            {"text": text, "metadata": metadata},
            run_id=self.run_id,
            context_id=context_id,
        )
        self.vector_store.upsert_points([point])
        return record

    def _insert_episode(self, *, text: str, context_id: int, chunk_id: int) -> MemoryRecord:
        if self.episode_writer is None or self.episode_config is None:
            raise RuntimeError("episode memory mode requires an episode writer and config")

        memory_id = str(uuid.uuid5(uuid.NAMESPACE_URL, f"{self.run_id}:{context_id}:{chunk_id}"))
        episode_id = f"mab_ctx{context_id}_chunk{chunk_id}"
        kv_key = f"{self.namespace}:{self.run_id}:{context_id}:{memory_id}"
        fields = self.episode_writer.write_fields(text=text)
        episode = build_episode_record(
            episode_id=episode_id,
            run_id=self.run_id,
            context_id=context_id,
            chunk_id=chunk_id,
            text=text,
            fields=fields,
            config=self.episode_config,
        )
        retrieval_text = str(episode["retrieval_text"])
        answer_text = build_episode_answer_text(
            episode=episode,
            evidence_text=text,
            tool_name=self.episode_config.tool_name,
        )
        entities = [entity for entity in episode.get("entities") or [] if isinstance(entity, dict)]
        metadata = {
            "run_id": self.run_id,
            "context_id": context_id,
            "chunk_id": chunk_id,
            "memory_id": memory_id,
            "episode_id": episode_id,
            "kv_key": kv_key,
            "memory_mode": "episode",
            "scenario": self.episode_config.scenario,
            "intent": self.episode_config.intent,
        }
        record = MemoryRecord(memory_id=memory_id, kv_key=kv_key, text=answer_text, metadata=metadata)
        vector = self.embedder.embed_text(retrieval_text)
        point = {
            "id": memory_id,
            "vector": vector,
            "payload": {
                **metadata,
                "summary": episode.get("summary", ""),
                "goal": episode.get("goal", ""),
                "entity_names": [str(entity.get("name") or "") for entity in entities],
                "entity_types": [str(entity.get("type") or "") for entity in entities],
                "tool_names": [self.episode_config.tool_name],
                "text_preview": answer_text[:1024],
                "text_sha256": hashlib.sha256(answer_text.encode("utf-8")).hexdigest(),
                "evidence_sha256": hashlib.sha256(text.encode("utf-8")).hexdigest(),
                "retrieval_text_sha256": hashlib.sha256(retrieval_text.encode("utf-8")).hexdigest(),
            },
        }
        self.kv_store.set_json(
            kv_key,
            {
                "text": answer_text,
                "evidence_text": text,
                "retrieval_text": retrieval_text,
                "episode": episode,
                "metadata": metadata,
            },
            run_id=self.run_id,
            context_id=context_id,
        )
        self.vector_store.upsert_points([point])
        return record

    def search(self, *, query: str, context_id: int, top_k: int) -> list[dict[str, Any]]:
        vector = self.embedder.embed_text(query)
        points = self.vector_store.query(vector=vector, context_id=context_id, limit=top_k)
        hits: list[dict[str, Any]] = []
        for rank, point in enumerate(points, start=1):
            payload = dict(point.get("payload") or {})
            kv_key = str(payload.get("kv_key") or "")
            stored = self.kv_store.get_json(kv_key) if kv_key else None
            text = str((stored or {}).get("text") or payload.get("text_preview") or "")
            hits.append(
                {
                    "rank": rank,
                    "score": float(point.get("score", 0.0)),
                    "memory_id": str(payload.get("memory_id") or point.get("id")),
                    "episode_id": str(payload.get("episode_id") or ""),
                    "memory_mode": str(payload.get("memory_mode") or "raw"),
                    "context_id": payload.get("context_id"),
                    "chunk_id": payload.get("chunk_id"),
                    "text": text,
                    "payload": payload,
                }
            )
        return hits


class OpenAITextGenerator:
    def __init__(
        self,
        *,
        model: str,
        api_key: str,
        base_url: str | None,
        temperature: float,
        max_tokens: int,
        timeout: float,
        extra_body: dict[str, Any] | None,
    ) -> None:
        if not model.strip():
            raise ValueError("LLM model is required")
        resolved_api_key = api_key.strip()
        if not resolved_api_key and base_url:
            resolved_api_key = "EMPTY"
        if not resolved_api_key:
            raise ValueError("OpenAI API key is required; pass --llm-api-key or set OPENAI_API_KEY")

        self.model = model
        self.temperature = temperature
        self.max_tokens = max_tokens
        self.extra_body = extra_body or None
        self.client = OpenAI(api_key=resolved_api_key, base_url=base_url, timeout=timeout)

    def generate(self, *, system: str, user: str) -> str:
        response = self.client.chat.completions.create(
            model=self.model,
            messages=[
                {"role": "system", "content": system},
                {"role": "user", "content": user},
            ],
            temperature=self.temperature,
            max_tokens=self.max_tokens,
            extra_body=self.extra_body,
        )
        return (response.choices[0].message.content or "").strip()


@dataclass(frozen=True)
class EpisodeMemoryConfig:
    scenario: str
    intent: str
    tool_name: str
    max_entities: int


class EpisodeWriterCache:
    def __init__(self, path: Path | None) -> None:
        self.path = path
        self._items: dict[str, dict[str, Any]] = {}
        if path is None or not path.exists():
            return
        for line in path.read_text(encoding="utf-8").splitlines():
            if not line.strip():
                continue
            try:
                row = json.loads(line)
            except json.JSONDecodeError:
                continue
            key = str(row.get("key") or "")
            value = row.get("value")
            if key and isinstance(value, dict):
                self._items[key] = value

    def get(self, key: str) -> dict[str, Any] | None:
        value = self._items.get(key)
        return dict(value) if value is not None else None

    def set(self, key: str, value: dict[str, Any]) -> None:
        self._items[key] = dict(value)
        if self.path is None:
            return
        self.path.parent.mkdir(parents=True, exist_ok=True)
        row = {"key": key, "value": value}
        with self.path.open("a", encoding="utf-8") as file:
            file.write(json.dumps(row, ensure_ascii=False, sort_keys=True) + "\n")


class EpisodeWriter:
    def __init__(
        self,
        *,
        llm: OpenAITextGenerator,
        cache: EpisodeWriterCache,
        config: EpisodeMemoryConfig,
    ) -> None:
        self.llm = llm
        self.cache = cache
        self.config = config

    def write_fields(self, *, text: str) -> dict[str, Any]:
        cache_key = episode_cache_key(text=text, config=self.config)
        cached = self.cache.get(cache_key)
        if cached is not None:
            cached["writer_cache_hit"] = True
            return cached

        raw = self.llm.generate(
            system=build_episode_writer_system_prompt(),
            user=build_episode_writer_user_prompt(text=text, config=self.config),
        )
        writer_error = ""
        try:
            parsed = parse_json_object(raw)
        except Exception as exc:
            parsed = {}
            writer_error = f"{type(exc).__name__}: {compact_text(str(exc), max_chars=240)}"
        fields = normalize_episode_writer_fields(
            parsed,
            source_text=text,
            max_entities=self.config.max_entities,
        )
        if writer_error:
            fields["writer_error"] = writer_error
            fields["writer_raw_output"] = compact_text(raw, max_chars=800)
        fields["writer_cache_hit"] = False
        fields["writer_prompt_version"] = EPISODE_WRITER_PROMPT_VERSION
        self.cache.set(cache_key, fields)
        return dict(fields)


class EpisodeQueryRewriter:
    def __init__(self, *, llm: OpenAITextGenerator, cache: EpisodeWriterCache) -> None:
        self.llm = llm
        self.cache = cache

    def rewrite(self, *, query: str) -> dict[str, Any]:
        cache_key = episode_query_rewrite_cache_key(query=query)
        cached = self.cache.get(cache_key)
        if cached is not None:
            cached["query_rewrite_cache_hit"] = True
            return cached

        raw = self.llm.generate(
            system=build_episode_query_rewriter_system_prompt(),
            user=build_episode_query_rewriter_user_prompt(query=query),
        )
        rewrite_error = ""
        try:
            parsed = parse_json_object(raw)
        except Exception as exc:
            parsed = {}
            rewrite_error = f"{type(exc).__name__}: {compact_text(str(exc), max_chars=240)}"

        retrieval_query = compact_text(parsed.get("query"), max_chars=360)
        if not retrieval_query:
            retrieval_query = fallback_episode_retrieval_query(query)
        if not retrieval_query.lower().startswith("which episode"):
            retrieval_query = "Which episode " + retrieval_query[0].lower() + retrieval_query[1:]
        if not retrieval_query.endswith("?"):
            retrieval_query = retrieval_query.rstrip(".") + "?"

        result: dict[str, Any] = {
            "retrieval_query": retrieval_query,
            "query_rewrite_cache_hit": False,
            "query_rewrite_prompt_version": EPISODE_QUERY_REWRITER_PROMPT_VERSION,
        }
        if rewrite_error:
            result["query_rewrite_error"] = rewrite_error
            result["query_rewrite_raw_output"] = compact_text(raw, max_chars=800)
        self.cache.set(cache_key, result)
        return dict(result)


def build_episode_writer_system_prompt() -> str:
    return (
        "You write compact episodic memory fields from a text-reading observation. "
        "Return valid JSON only. Preserve named entities, dates, numbers, and relationships "
        "that could help future accurate retrieval."
    )


def build_episode_writer_user_prompt(*, text: str, config: EpisodeMemoryConfig) -> str:
    entity_types = ", ".join(EPISODE_ENTITY_TYPES)
    return (
        "You observed the output of a tool call.\n\n"
        f"Tool: {config.tool_name}\n"
        f"Fixed scenario: {config.scenario}\n"
        f"Fixed intent: {config.intent}\n\n"
        "Tool output:\n"
        f"{text}\n\n"
        "Write fields for an episodic memory record for future accurate retrieval.\n"
        "Rules:\n"
        "- Do not invent facts not present in the tool output.\n"
        "- goal should be one short sentence about what this read_text observation is preserving.\n"
        "- summary should be concise but include salient facts, names, dates, counts, and relationships.\n"
        f"- entities must use only these types: {entity_types}.\n"
        f"- return at most {config.max_entities} entities.\n\n"
        "Return JSON exactly in this shape:\n"
        '{"goal":"...","summary":"...","entities":[{"type":"person","name":"..."}]}'
    )


def build_episode_query_rewriter_system_prompt() -> str:
    return (
        "You rewrite benchmark QA prompts into concise episode-locator retrieval queries "
        "for an episodic memory benchmark. Return valid JSON only."
    )


def build_episode_query_rewriter_user_prompt(*, query: str) -> str:
    extracted_question = extract_benchmark_question(query)
    return (
        "Rewrite the original benchmark query into one English retrieval query for an episodic memory index.\n\n"
        "Target style:\n"
        "- Start with \"Which episode ...?\"\n"
        "- Ask for the prior episode that contains or preserved the relevant evidence.\n"
        "- Preserve explicit names, dates, numbers, entities, and relations from the question.\n"
        "- Do not answer the question.\n"
        "- Do not invent facts not present in the question.\n"
        "- Remove benchmark instructions such as \"Only give me the answer\".\n"
        "- Keep it short and retrieval-oriented, like: "
        "\"Which episode archived 18 low-rated assets from Tokyo Collection spring 2023?\"\n\n"
        "Examples:\n"
        "Original question: In what country is Normandy located?\n"
        "Rewrite: Which episode preserved evidence about the country where Normandy is located?\n\n"
        "Original question: When were the Normans in Normandy?\n"
        "Rewrite: Which episode preserved evidence about when the Normans were in Normandy?\n\n"
        "Original question: From which countries did the Norse originate?\n"
        "Rewrite: Which episode preserved evidence about the countries where the Norse originated?\n\n"
        f"Original benchmark query:\n{query}\n\n"
        f"Extracted question:\n{extracted_question}\n\n"
        "Return JSON exactly in this shape:\n"
        '{"query":"Which episode ...?"}'
    )


def episode_cache_key(*, text: str, config: EpisodeMemoryConfig) -> str:
    body = {
        "prompt_version": EPISODE_WRITER_PROMPT_VERSION,
        "scenario": config.scenario,
        "intent": config.intent,
        "tool_name": config.tool_name,
        "text_sha256": hashlib.sha256(text.encode("utf-8")).hexdigest(),
    }
    encoded = json.dumps(body, ensure_ascii=False, sort_keys=True).encode("utf-8")
    return hashlib.sha256(encoded).hexdigest()


def episode_query_rewrite_cache_key(*, query: str) -> str:
    body = {
        "prompt_version": EPISODE_QUERY_REWRITER_PROMPT_VERSION,
        "query_sha256": hashlib.sha256(query.encode("utf-8")).hexdigest(),
    }
    encoded = json.dumps(body, ensure_ascii=False, sort_keys=True).encode("utf-8")
    return hashlib.sha256(encoded).hexdigest()


def extract_benchmark_question(query: str) -> str:
    text = " ".join(query.split())
    match = re.search(r"now answer the question:\s*(.+)$", text, flags=re.IGNORECASE)
    if match:
        return match.group(1).strip()
    return text


def fallback_episode_retrieval_query(query: str) -> str:
    question = extract_benchmark_question(query).rstrip("?")
    if question.lower().startswith("which episode"):
        return question + "?"
    return f"Which episode preserved evidence answering: {question}?"


def parse_json_object(raw: str) -> dict[str, Any]:
    text = raw.strip()
    if text.startswith("```"):
        text = re.sub(r"^```(?:json)?\s*", "", text)
        text = re.sub(r"\s*```$", "", text).strip()
    try:
        payload = json.loads(text)
    except json.JSONDecodeError:
        start = text.find("{")
        if start < 0:
            raise
        payload, _ = json.JSONDecoder().raw_decode(text[start:])
    if not isinstance(payload, dict):
        raise ValueError("episode writer response must be a JSON object")
    return payload


def normalize_episode_writer_fields(
    payload: dict[str, Any],
    *,
    source_text: str,
    max_entities: int,
) -> dict[str, Any]:
    goal = compact_text(payload.get("goal"), max_chars=240)
    summary = compact_text(payload.get("summary"), max_chars=700)
    if not goal:
        goal = "Remember evidence from a benchmark text chunk for future question answering."
    if not summary:
        summary = fallback_summary(source_text)

    entities: list[dict[str, str]] = []
    seen_entities: set[tuple[str, str]] = set()
    raw_entities = payload.get("entities")
    if max_entities > 0 and isinstance(raw_entities, list):
        for item in raw_entities:
            if not isinstance(item, dict):
                continue
            entity_name = compact_text(item.get("name"), max_chars=120)
            if not entity_name:
                continue
            entity_type = normalize_entity_type(item.get("type"))
            key = (entity_type, entity_name.lower())
            if key in seen_entities:
                continue
            entities.append({"type": entity_type, "name": entity_name})
            seen_entities.add(key)
            if len(entities) >= max_entities:
                break

    return {"goal": goal, "summary": summary, "entities": entities}


def compact_text(value: Any, *, max_chars: int) -> str:
    text = "" if value is None else str(value)
    text = " ".join(text.split())
    if len(text) <= max_chars:
        return text
    return text[: max_chars - 3].rstrip() + "..."


def fallback_summary(text: str) -> str:
    return compact_text(text, max_chars=420)


def normalize_entity_type(value: Any) -> str:
    entity_type = re.sub(r"[^a-z_]+", "_", str(value or "").strip().lower()).strip("_")
    if entity_type in EPISODE_ENTITY_TYPES:
        return entity_type
    return "other"


def build_episode_record(
    *,
    episode_id: str,
    run_id: str,
    context_id: int,
    chunk_id: int,
    text: str,
    fields: dict[str, Any],
    config: EpisodeMemoryConfig,
) -> dict[str, Any]:
    evidence_hash = hashlib.sha256(text.encode("utf-8")).hexdigest()
    episode = {
        "id": episode_id,
        "episode_id": episode_id,
        "thread_id": f"mab:{run_id}:{context_id}",
        "user_id": "memoryagentbench",
        "agent_name": DEFAULT_AGENT_NAME,
        "scenario": config.scenario,
        "goal": fields["goal"],
        "intent": config.intent,
        "summary": fields["summary"],
        "workspace": "memoryagentbench",
        "route": "/benchmark/memoryagentbench/accurate_retrieval",
        "status": "succeeded",
        "write_trigger": "goal_resolved",
        "entities": list(fields.get("entities") or []),
        "tool_trace": [
            {
                "index": 0,
                "tool": {"name": config.tool_name},
                "operation": config.tool_name,
                "output_summary": "Read a benchmark text chunk for future retrieval.",
                "status": "succeeded",
            }
        ],
        "context_blocks": [
            {"id": f"{episode_id}:summary", "kind": "summary", "text": fields["summary"], "weight": 1.0},
            {"id": f"{episode_id}:tool_trace", "kind": "tool_trace", "text": config.tool_name, "weight": 0.8},
        ],
        "metadata": {
            "source": "memoryagentbench",
            "source_context_id": str(context_id),
            "source_chunk_id": str(chunk_id),
            "evidence_sha256": evidence_hash,
            "writer_prompt_version": str(fields.get("writer_prompt_version") or EPISODE_WRITER_PROMPT_VERSION),
            "writer_cache_hit": str(bool(fields.get("writer_cache_hit"))).lower(),
        },
    }
    if fields.get("writer_error"):
        episode["metadata"]["writer_error"] = str(fields["writer_error"])
    episode["retrieval_text"] = build_episode_retrieval_text(episode)
    return episode


def build_episode_retrieval_text(episode: dict[str, Any]) -> str:
    sections: list[str] = []
    what_parts: list[str] = []
    for key in ("scenario", "intent", "summary"):
        value = str(episode.get(key) or "").strip()
        if value:
            what_parts.append(f"{key}={value}")
    if what_parts:
        sections.append("what:\n" + "\n".join(what_parts))

    goal = str(episode.get("goal") or "").strip()
    if goal:
        sections.append("goal:\n" + goal)

    entity_parts: list[str] = []
    for entity in episode.get("entities") or []:
        if not isinstance(entity, dict):
            continue
        name = str(entity.get("name") or "").strip()
        if not name:
            continue
        entity_type = str(entity.get("type") or "").strip()
        entity_parts.append(f"{entity_type}={name}" if entity_type else name)
    if entity_parts:
        sections.append("task_content:\n" + "\n".join(entity_parts))

    tool_names: list[str] = []
    for step in episode.get("tool_trace") or []:
        if not isinstance(step, dict):
            continue
        tool = step.get("tool") if isinstance(step.get("tool"), dict) else {}
        tool_name = str(tool.get("name") or step.get("operation") or "").strip()
        if tool_name:
            tool_names.append(tool_name)
    if tool_names:
        sections.append("procedure:\n" + " -> ".join(tool_names))

    return "\n\n".join(sections)


def build_episode_answer_text(*, episode: dict[str, Any], evidence_text: str, tool_name: str) -> str:
    entity_text = "\n".join(
        f"- {entity.get('type', 'other')}={entity.get('name', '')}"
        for entity in episode.get("entities") or []
        if isinstance(entity, dict) and str(entity.get("name") or "").strip()
    )
    return (
        f"Episode ID: {episode['id']}\n"
        f"Scenario: {episode['scenario']}\n"
        f"Intent: {episode['intent']}\n"
        f"Goal: {episode['goal']}\n"
        f"Summary: {episode['summary']}\n"
        f"Procedure: {tool_name}\n"
        f"Entities:\n{entity_text if entity_text else '- none'}\n\n"
        f"Evidence text:\n{evidence_text}"
    )


class LangGraphRetrievalAgent:
    def __init__(
        self,
        *,
        memory: MemoryBackend,
        llm: OpenAITextGenerator,
        top_k: int,
        retrieval_char_budget: int,
        answer_system_prompt: str,
        query_rewriter: EpisodeQueryRewriter | None = None,
    ) -> None:
        self.memory = memory
        self.llm = llm
        self.top_k = top_k
        self.retrieval_char_budget = retrieval_char_budget
        self.answer_system_prompt = answer_system_prompt
        self.query_rewriter = query_rewriter
        self.context_construction_times: dict[int, float] = {}
        self.graph = self._build_graph()

    def _build_graph(self) -> Any:
        workflow = StateGraph(MemoryGraphState)
        workflow.add_node("route", lambda state: state)
        workflow.add_node("memory_update", self._memory_update_node)
        workflow.add_node("memory_query", self._memory_query_node)
        workflow.add_edge(START, "route")
        workflow.add_conditional_edges(
            "route",
            lambda state: state["mode"],
            {"update": "memory_update", "query": "memory_query"},
        )
        workflow.add_edge("memory_update", END)
        workflow.add_edge("memory_query", END)
        return workflow.compile()

    def start_context(self, *, context_id: int) -> None:
        self.memory.clear_context(context_id=context_id)
        self.context_construction_times.pop(context_id, None)

    def set_context_construction_time(self, *, context_id: int, seconds: float) -> None:
        self.context_construction_times[context_id] = seconds

    def send_message(
        self,
        message: str,
        *,
        memorizing: bool,
        query_id: int | None = None,
        context_id: int = 0,
        chunk_id: int | None = None,
    ) -> dict[str, Any]:
        mode: Literal["update", "query"] = "update" if memorizing else "query"
        state: MemoryGraphState = {
            "mode": mode,
            "text": message,
            "context_id": context_id,
        }
        if query_id is not None:
            state["query_id"] = query_id
        if chunk_id is not None:
            state["chunk_id"] = chunk_id

        result = self.graph.invoke(state)
        return dict(result.get("output") or {})

    def _memory_update_node(self, state: MemoryGraphState) -> MemoryGraphState:
        context_id = int(state["context_id"])
        chunk_id = int(state.get("chunk_id", 0))
        start = time.perf_counter()
        record = self.memory.insert(text=state["text"], context_id=context_id, chunk_id=chunk_id)
        elapsed = time.perf_counter() - start
        return {
            **state,
            "output": {
                "memory_id": record.memory_id,
                "kv_key": record.kv_key,
                "memory_mode": record.metadata.get("memory_mode", "raw"),
                "episode_id": record.metadata.get("episode_id", ""),
                "context_id": context_id,
                "chunk_id": chunk_id,
                "memory_update_time": elapsed,
            },
        }

    def _memory_query_node(self, state: MemoryGraphState) -> MemoryGraphState:
        query = state["text"]
        context_id = int(state["context_id"])
        query_start = time.perf_counter()
        rewrite_result: dict[str, Any] = {}
        retrieval_query = query
        if self.query_rewriter is not None:
            rewrite_result = self.query_rewriter.rewrite(query=query)
            retrieval_query = str(rewrite_result.get("retrieval_query") or query)

        hits = self.memory.search(query=retrieval_query, context_id=context_id, top_k=self.top_k)
        retrieval_context = build_retrieval_context(hits, char_budget=self.retrieval_char_budget)
        user_prompt = build_answer_prompt(query=query, retrieval_context=retrieval_context)
        answer = self.llm.generate(system=self.answer_system_prompt, user=user_prompt)
        query_time = time.perf_counter() - query_start

        output = {
            "output": answer,
            "input_len": count_tokens(self.answer_system_prompt + "\n" + user_prompt),
            "output_len": count_tokens(answer),
            "memory_construction_time": self.context_construction_times.get(context_id, 0.0),
            "query_time_len": query_time,
            "retrieval_query": retrieval_query,
            "query_rewrite_cache_hit": rewrite_result.get("query_rewrite_cache_hit"),
            "query_rewrite_error": rewrite_result.get("query_rewrite_error", ""),
            "retrieval_context": retrieval_context,
            "retrieved_memory_ids": [hit["memory_id"] for hit in hits],
            "retrieved_episode_ids": [hit.get("episode_id", "") for hit in hits],
            "retrieved_memory_modes": [hit.get("memory_mode", "raw") for hit in hits],
            "retrieved_scores": [hit["score"] for hit in hits],
            "retrieved_chunk_ids": [hit["chunk_id"] for hit in hits],
            "retrieved_texts": [hit["text"] for hit in hits],
        }
        return {**state, "output": output}


def build_retrieval_context(hits: list[dict[str, Any]], *, char_budget: int) -> str:
    parts: list[str] = []
    used_chars = 0
    for hit in hits:
        header_fields = [
            f"Memory {hit['rank']}",
            f"mode={hit.get('memory_mode', 'raw')}",
            f"chunk_id={hit.get('chunk_id')}",
        ]
        if hit.get("episode_id"):
            header_fields.append(f"episode_id={hit['episode_id']}")
        header_fields.append(f"score={hit['score']:.6f}")
        header = "[" + " | ".join(header_fields) + "]\n"
        text = str(hit.get("text") or "").strip()
        block = header + text
        remaining = char_budget - used_chars
        if remaining <= 0:
            break
        if len(block) > remaining:
            block = block[:remaining]
        parts.append(block)
        used_chars += len(block)
    return "\n\n".join(parts)


def build_answer_prompt(*, query: str, retrieval_context: str) -> str:
    return (
        "Use only the retrieved memory below to answer the task. "
        "Return only the final answer, without explanations or citations.\n\n"
        f"Retrieved memory:\n{retrieval_context}\n\n"
        f"Task:\n{query}\n\n"
        "Answer:"
    )


def count_tokens(text: str, *, model_name: str = "gpt-4o-mini") -> int:
    try:
        encoding = tiktoken.encoding_for_model(model_name)
    except Exception:
        encoding = tiktoken.get_encoding("cl100k_base")
    return len(encoding.encode(text))


def local_chunk_text(text: str, *, chunk_size: int) -> list[str]:
    try:
        encoding = tiktoken.get_encoding("cl100k_base")
    except Exception as exc:
        raise RuntimeError("tiktoken is required for local chunking") from exc

    sentences = split_sentences(text)
    chunks: list[str] = []
    current: list[str] = []
    current_tokens = 0

    for sentence in sentences:
        sentence_token_ids = encoding.encode(sentence, allowed_special={"<|endoftext|>"})
        if len(sentence_token_ids) > chunk_size:
            if current:
                chunks.append(" ".join(current).strip())
                current = []
                current_tokens = 0
            for start in range(0, len(sentence_token_ids), chunk_size):
                chunks.append(encoding.decode(sentence_token_ids[start : start + chunk_size]).strip())
            continue

        if current and current_tokens + len(sentence_token_ids) > chunk_size:
            chunks.append(" ".join(current).strip())
            current = [sentence]
            current_tokens = len(sentence_token_ids)
        else:
            current.append(sentence)
            current_tokens += len(sentence_token_ids)

    if current:
        chunks.append(" ".join(current).strip())
    return [chunk for chunk in chunks if chunk]


def split_sentences(text: str) -> list[str]:
    pieces = re.split(r"(?<=[.!?])\s+|\n{2,}", text)
    return [piece.strip() for piece in pieces if piece.strip()]


def flatten_answers(answer: Any) -> list[str]:
    if isinstance(answer, str):
        return [answer]
    if isinstance(answer, list):
        flattened: list[str] = []
        for item in answer:
            flattened.extend(flatten_answers(item))
        return flattened
    if answer is None:
        return []
    return [str(answer)]


def normalize_for_match(text: str) -> str:
    text = text.lower()
    text = "".join(char for char in text if char not in r"""!"#$%&'()*+,-./:;<=>?@[\]^_`{|}~""")
    text = re.sub(r"\b(a|an|the)\b", " ", text)
    return " ".join(text.split())


def compute_retrieval_diagnostics(
    *,
    retrieved_texts: list[str],
    answer: Any,
    top_ks: tuple[int, ...] = (1, 5, 10),
) -> dict[str, Any]:
    answers = [normalize_for_match(value) for value in flatten_answers(answer) if str(value).strip()]
    normalized_texts = [normalize_for_match(text) for text in retrieved_texts]

    hit_rank: int | None = None
    for index, text in enumerate(normalized_texts, start=1):
        if answers and any(answer_text and answer_text in text for answer_text in answers):
            hit_rank = index
            break

    diagnostics: dict[str, Any] = {
        "retrieval_answer_found": hit_rank is not None,
        "retrieval_answer_hit_rank": hit_rank,
        "retrieval_mrr": 0.0 if hit_rank is None else 1.0 / hit_rank,
    }
    for top_k in top_ks:
        diagnostics[f"retrieval_answer_hit@{top_k}"] = bool(hit_rank is not None and hit_rank <= top_k)
    return diagnostics


def append_retrieval_metrics(metrics: dict[str, list[Any]], diagnostics: dict[str, Any]) -> None:
    for key, value in diagnostics.items():
        if value is not None:
            metrics[key].append(value)


def averaged_metrics(metrics: dict[str, list[Any]]) -> dict[str, float]:
    averaged: dict[str, float] = {}
    for key, values in metrics.items():
        if not values:
            continue
        mean_value = float(np.mean(values))
        multiplier = 1.0 if ("_len" in key or "_time" in key or key.endswith("_rank")) else 100.0
        averaged[key] = mean_value * multiplier
    return averaged


def save_results(
    *,
    output_path: Path,
    agent_config: dict[str, Any],
    dataset_config: dict[str, Any],
    results: list[dict[str, Any]],
    metrics: dict[str, list[Any]],
    retrieval_metrics: dict[str, list[Any]],
    time_cost: list[float],
    started_at: float,
    run_metadata: dict[str, Any],
) -> None:
    output_path.parent.mkdir(parents=True, exist_ok=True)
    time_cost.append(time.time() - started_at)
    output = {
        "agent_config": agent_config,
        "dataset_config": dataset_config,
        "run_metadata": run_metadata,
        "data": results,
        "metrics": dict(metrics),
        "retrieval_metrics": dict(retrieval_metrics),
        "time_cost": time_cost,
        "averaged_metrics": averaged_metrics(metrics),
        "averaged_retrieval_metrics": averaged_metrics(retrieval_metrics),
    }
    output_path.write_text(json.dumps(output, ensure_ascii=False, indent=2), encoding="utf-8")


def resolve_memoryagentbench_root(raw_path: str) -> Path:
    path = Path(raw_path).expanduser()
    if not path.is_absolute():
        path = Path.cwd() / path
    path = path.resolve()
    if not (path / "conversation_creator.py").exists():
        raise FileNotFoundError(f"MemoryAgentBench root not found or invalid: {path}")
    return path


def resolve_dataset_config(mab_root: Path, raw_path: str) -> Path:
    path = Path(raw_path).expanduser()
    candidates = []
    if path.is_absolute():
        candidates.append(path)
    else:
        candidates.extend(
            [
                Path.cwd() / path,
                mab_root / path,
                mab_root / "configs" / "data_conf" / path,
            ]
        )
    for candidate in candidates:
        if candidate.exists():
            return candidate.resolve()
    raise FileNotFoundError(f"dataset config not found: {raw_path}")


def load_yaml(path: Path) -> dict[str, Any]:
    return dict(yaml.safe_load(path.read_text(encoding="utf-8")) or {})


def load_conversations(
    *,
    mab_root: Path,
    agent_config: dict[str, Any],
    dataset_config: dict[str, Any],
    chunker: str,
) -> tuple[list[list[str]], list[list[tuple[Any, Any, Any]]]]:
    sys.path.insert(0, str(mab_root))
    from conversation_creator import ConversationCreator

    creator = ConversationCreator(agent_config, dataset_config)
    query_answer_pairs = creator.get_query_and_answers()

    if chunker == "official":
        chunks = creator.get_chunks()
    else:
        chunks = [
            local_chunk_text(context, chunk_size=int(dataset_config["chunk_size"]))
            for context in creator.contexts
        ]
    return chunks, query_answer_pairs


def create_kv_store(args: argparse.Namespace) -> KVStore:
    if args.kv_backend == "dict":
        return DictKVStore()
    return RedisKVStore(url=args.redis_url, prefix=args.kv_namespace)


def create_episode_writer(args: argparse.Namespace) -> EpisodeWriter | None:
    if args.memory_mode != "episode":
        return None

    cache_path: Path | None = None
    if args.episode_cache_path.strip():
        cache_path = Path(args.episode_cache_path).expanduser()
        if not cache_path.is_absolute():
            cache_path = (Path.cwd() / cache_path).resolve()

    writer_model = args.episode_writer_model.strip() or args.llm_model
    writer_llm = OpenAITextGenerator(
        model=writer_model,
        api_key=args.llm_api_key,
        base_url=args.llm_base_url or None,
        temperature=args.episode_writer_temperature,
        max_tokens=args.episode_writer_max_tokens,
        timeout=args.llm_timeout,
        extra_body=build_llm_extra_body(args.llm_extra_body_json, base_url=args.llm_base_url),
    )
    config = EpisodeMemoryConfig(
        scenario=args.episode_scenario,
        intent=args.episode_intent,
        tool_name=args.episode_tool_name,
        max_entities=args.episode_max_entities,
    )
    return EpisodeWriter(
        llm=writer_llm,
        cache=EpisodeWriterCache(cache_path),
        config=config,
    )


def create_episode_query_rewriter(args: argparse.Namespace) -> EpisodeQueryRewriter | None:
    if args.memory_mode != "episode":
        return None

    cache_path: Path | None = None
    if args.episode_query_cache_path.strip():
        cache_path = Path(args.episode_query_cache_path).expanduser()
        if not cache_path.is_absolute():
            cache_path = (Path.cwd() / cache_path).resolve()

    rewriter_model = args.episode_query_rewriter_model.strip() or args.llm_model
    rewriter_llm = OpenAITextGenerator(
        model=rewriter_model,
        api_key=args.llm_api_key,
        base_url=args.llm_base_url or None,
        temperature=args.episode_query_rewriter_temperature,
        max_tokens=args.episode_query_rewriter_max_tokens,
        timeout=args.llm_timeout,
        extra_body=build_llm_extra_body(args.llm_extra_body_json, base_url=args.llm_base_url),
    )
    return EpisodeQueryRewriter(
        llm=rewriter_llm,
        cache=EpisodeWriterCache(cache_path),
    )


def parse_extra_body(raw_json: str) -> dict[str, Any] | None:
    if not raw_json.strip():
        return None
    payload = json.loads(raw_json)
    if not isinstance(payload, dict):
        raise ValueError("--llm-extra-body-json must decode to a JSON object")
    return payload


def build_llm_extra_body(raw_json: str, *, base_url: str) -> dict[str, Any] | None:
    extra_body = parse_extra_body(raw_json) or {}
    if is_dashscope_base_url(base_url) and "enable_thinking" not in extra_body:
        extra_body["enable_thinking"] = False
    return extra_body or None


def is_dashscope_base_url(base_url: str) -> bool:
    normalized = base_url.lower()
    return "dashscope" in normalized or "aliyuncs.com" in normalized


def build_arg_parser() -> argparse.ArgumentParser:
    default_mab_root = Path(__file__).resolve().parents[1] / "third_party" / "MemoryAgentBench"
    default_episode_cache_path = (
        Path(__file__).resolve().parents[1] / "data" / "cache" / "mab_episode_writer_cache.jsonl"
    )
    default_episode_query_cache_path = (
        Path(__file__).resolve().parents[1] / "data" / "cache" / "mab_episode_query_rewrite_cache.jsonl"
    )
    parser = argparse.ArgumentParser(
        description="Run MemoryAgentBench Accurate Retrieval with a LangGraph memory adapter."
    )
    parser.add_argument("--mab-root", default=str(default_mab_root), help="Path to the MemoryAgentBench repo")
    parser.add_argument("--dataset-config", default=DEFAULT_DATASET_CONFIG, help="AR dataset config path")
    parser.add_argument("--output-path", default="", help="Output JSON path")
    parser.add_argument("--run-id", default="", help="Run namespace; defaults to timestamp")
    parser.add_argument("--agent-name", default=DEFAULT_AGENT_NAME, help="Name used for official query templates")
    parser.add_argument("--max-test-samples", type=int, default=0, help="Override dataset max_test_samples")
    parser.add_argument("--max-test-queries", type=int, default=0, help="Global query limit for smoke tests")
    parser.add_argument("--chunk-size", type=int, default=0, help="Override dataset chunk_size")
    parser.add_argument("--chunker", choices=["local", "official"], default="local")
    parser.add_argument("--top-k", type=int, default=10)
    parser.add_argument("--retrieval-char-budget", type=int, default=30000)
    parser.add_argument("--save-every", type=int, default=1)
    parser.add_argument("--memory-mode", choices=["raw", "episode"], default=os.environ.get("MEMORY_MODE", "raw"))

    parser.add_argument("--episode-scenario", default=os.environ.get("EPISODE_SCENARIO", DEFAULT_EPISODE_SCENARIO))
    parser.add_argument("--episode-intent", default=os.environ.get("EPISODE_INTENT", DEFAULT_EPISODE_INTENT))
    parser.add_argument("--episode-tool-name", default=os.environ.get("EPISODE_TOOL_NAME", DEFAULT_EPISODE_TOOL_NAME))
    parser.add_argument("--episode-max-entities", type=int, default=int(os.environ.get("EPISODE_MAX_ENTITIES", "12")))
    parser.add_argument(
        "--episode-cache-path",
        default=os.environ.get("EPISODE_CACHE_PATH", str(default_episode_cache_path)),
        help="JSONL cache for LLM-written episode fields. Pass empty string to disable.",
    )
    parser.add_argument("--episode-writer-model", default=os.environ.get("EPISODE_WRITER_MODEL", ""))
    parser.add_argument(
        "--episode-writer-temperature",
        type=float,
        default=float(os.environ.get("EPISODE_WRITER_TEMPERATURE", "0")),
    )
    parser.add_argument(
        "--episode-writer-max-tokens",
        type=int,
        default=int(os.environ.get("EPISODE_WRITER_MAX_TOKENS", "512")),
    )
    parser.add_argument(
        "--episode-query-cache-path",
        default=os.environ.get("EPISODE_QUERY_CACHE_PATH", str(default_episode_query_cache_path)),
        help="JSONL cache for episode-locator query rewrites. Pass empty string to disable.",
    )
    parser.add_argument(
        "--episode-query-rewriter-model",
        default=os.environ.get("EPISODE_QUERY_REWRITER_MODEL", ""),
        help="Model for rewriting benchmark queries into Which episode...? retrieval queries.",
    )
    parser.add_argument(
        "--episode-query-rewriter-temperature",
        type=float,
        default=float(os.environ.get("EPISODE_QUERY_REWRITER_TEMPERATURE", "0")),
    )
    parser.add_argument(
        "--episode-query-rewriter-max-tokens",
        type=int,
        default=int(os.environ.get("EPISODE_QUERY_REWRITER_MAX_TOKENS", "128")),
    )

    parser.add_argument("--embed-base-url", default=os.environ.get("EMBED_BASE_URL", "http://127.0.0.1:11500"))
    parser.add_argument("--embed-model", default=os.environ.get("EMBED_MODEL", "qwen3-episodic-lora"))
    parser.add_argument("--embed-dims", type=int, default=int(os.environ.get("EMBED_DIMS", "1024")))
    parser.add_argument("--embed-keep-alive", default=os.environ.get("EMBED_KEEP_ALIVE", "5m"))
    parser.add_argument("--embed-timeout", type=float, default=float(os.environ.get("EMBED_TIMEOUT", "60")))
    parser.add_argument("--embed-batch-size", type=int, default=8)

    parser.add_argument("--qdrant-url", default=os.environ.get("QDRANT_URL", "http://127.0.0.1:6333"))
    parser.add_argument("--qdrant-api-key", default=os.environ.get("QDRANT_API_KEY", ""))
    parser.add_argument("--qdrant-timeout", type=float, default=float(os.environ.get("QDRANT_TIMEOUT", "60")))
    parser.add_argument("--collection", default=os.environ.get("QDRANT_COLLECTION", DEFAULT_COLLECTION))
    parser.add_argument("--distance", default=os.environ.get("QDRANT_DISTANCE", "Cosine"))
    parser.add_argument("--recreate-collection", action="store_true")

    parser.add_argument("--kv-backend", choices=["dict", "redis"], default=os.environ.get("KV_BACKEND", "dict"))
    parser.add_argument("--kv-namespace", default=os.environ.get("KV_NAMESPACE", "memoryagentbench"))
    parser.add_argument("--redis-url", default=os.environ.get("REDIS_URL", "redis://127.0.0.1:6379/0"))

    parser.add_argument("--llm-model", default=os.environ.get("LLM_MODEL", "qwen3-32b"))
    parser.add_argument("--llm-base-url", default=os.environ.get("OPENAI_BASE_URL", ""))
    parser.add_argument("--llm-api-key", default=os.environ.get("OPENAI_API_KEY", ""))
    parser.add_argument("--llm-temperature", type=float, default=float(os.environ.get("LLM_TEMPERATURE", "0")))
    parser.add_argument("--llm-max-tokens", type=int, default=int(os.environ.get("LLM_MAX_TOKENS", "128")))
    parser.add_argument("--llm-timeout", type=float, default=float(os.environ.get("LLM_TIMEOUT", "120")))
    parser.add_argument("--llm-extra-body-json", default=os.environ.get("LLM_EXTRA_BODY_JSON", ""))
    parser.add_argument(
        "--answer-system-prompt",
        default=(
            "You are an accurate retrieval QA system. Answer strictly from retrieved memory. "
            "If the retrieved memory does not contain enough evidence, answer noanswer."
        ),
    )
    return parser


def run(args: argparse.Namespace) -> None:
    started_at = time.time()
    mab_root = resolve_memoryagentbench_root(args.mab_root)
    dataset_config_path = resolve_dataset_config(mab_root, args.dataset_config)
    dataset_config = load_yaml(dataset_config_path)
    if args.max_test_samples > 0:
        dataset_config["max_test_samples"] = args.max_test_samples
    if args.chunk_size > 0:
        dataset_config["chunk_size"] = args.chunk_size
    dataset_config.setdefault("debug", False)

    run_id = args.run_id.strip() or time.strftime("mab-ar-%Y%m%d-%H%M%S")
    output_path = (
        Path(args.output_path).expanduser()
        if args.output_path.strip()
        else Path(__file__).resolve().parents[1]
        / "data"
        / "reports"
        / f"{run_id}-{dataset_config['sub_dataset']}.json"
    )
    if not output_path.is_absolute():
        output_path = (Path.cwd() / output_path).resolve()

    agent_config = {
        "agent_name": args.agent_name,
        "model": args.llm_model,
        "retrieve_num": args.top_k,
        "output_dir": str(output_path.parent),
    }

    vector_store = QdrantVectorStore(
        base_url=args.qdrant_url,
        api_key=args.qdrant_api_key,
        collection=args.collection,
        dimensions=args.embed_dims,
        distance=args.distance,
        run_id=run_id,
        timeout=args.qdrant_timeout,
    )
    vector_store.ensure_collection(recreate=args.recreate_collection)

    embedder = OllamaCompatibleEmbedder(
        base_url=args.embed_base_url,
        model=args.embed_model,
        dimensions=args.embed_dims,
        keep_alive=args.embed_keep_alive,
        timeout=args.embed_timeout,
        batch_size=args.embed_batch_size,
    )
    episode_writer = create_episode_writer(args)
    episode_config = episode_writer.config if episode_writer is not None else None
    memory_backend = MemoryBackend(
        run_id=run_id,
        namespace=args.kv_namespace,
        embedder=embedder,
        vector_store=vector_store,
        kv_store=create_kv_store(args),
        episode_writer=episode_writer,
        episode_config=episode_config,
    )
    llm = OpenAITextGenerator(
        model=args.llm_model,
        api_key=args.llm_api_key,
        base_url=args.llm_base_url or None,
        temperature=args.llm_temperature,
        max_tokens=args.llm_max_tokens,
        timeout=args.llm_timeout,
        extra_body=build_llm_extra_body(args.llm_extra_body_json, base_url=args.llm_base_url),
    )
    agent = LangGraphRetrievalAgent(
        memory=memory_backend,
        llm=llm,
        top_k=args.top_k,
        retrieval_char_budget=args.retrieval_char_budget,
        answer_system_prompt=args.answer_system_prompt,
        query_rewriter=create_episode_query_rewriter(args),
    )

    all_context_chunks, all_query_answer_pairs = load_conversations(
        mab_root=mab_root,
        agent_config=agent_config,
        dataset_config=dataset_config,
        chunker=args.chunker,
    )

    metrics: dict[str, list[Any]] = defaultdict(list)
    retrieval_metrics: dict[str, list[Any]] = defaultdict(list)
    results: list[dict[str, Any]] = []
    time_cost: list[float] = []
    query_index = 0

    sys.path.insert(0, str(mab_root))
    from utils.eval_other_utils import metrics_summarization

    run_metadata = {
        "run_id": run_id,
        "memoryagentbench_root": str(mab_root),
        "memoryagentbench_commit": read_git_commit(mab_root),
        "dataset_config_path": str(dataset_config_path),
        "chunker": args.chunker,
        "memory_mode": args.memory_mode,
        "embed_base_url": args.embed_base_url,
        "embed_model": args.embed_model,
        "embed_dims": args.embed_dims,
        "qdrant_url": args.qdrant_url,
        "collection": args.collection,
        "kv_backend": args.kv_backend,
        "episode_scenario": args.episode_scenario if args.memory_mode == "episode" else "",
        "episode_intent": args.episode_intent if args.memory_mode == "episode" else "",
        "episode_tool_name": args.episode_tool_name if args.memory_mode == "episode" else "",
        "episode_cache_path": args.episode_cache_path if args.memory_mode == "episode" else "",
        "episode_writer_model": (args.episode_writer_model.strip() or args.llm_model)
        if args.memory_mode == "episode"
        else "",
        "episode_query_cache_path": args.episode_query_cache_path if args.memory_mode == "episode" else "",
        "episode_query_rewriter_model": (args.episode_query_rewriter_model.strip() or args.llm_model)
        if args.memory_mode == "episode"
        else "",
        "llm_model": args.llm_model,
        "llm_base_url": args.llm_base_url,
    }

    for context_id, (context_chunks, query_answer_pairs) in enumerate(
        zip(all_context_chunks, all_query_answer_pairs)
    ):
        if args.max_test_queries > 0 and query_index >= args.max_test_queries:
            break

        print(
            json.dumps(
                {
                    "event": "memorize_context",
                    "context_id": context_id,
                    "chunk_count": len(context_chunks),
                    "query_count": len(query_answer_pairs),
                    "memory_mode": args.memory_mode,
                },
                ensure_ascii=False,
            ),
            flush=True,
        )
        agent.start_context(context_id=context_id)
        construction_start = time.perf_counter()
        for chunk_id, chunk in enumerate(context_chunks):
            agent.send_message(chunk, memorizing=True, context_id=context_id, chunk_id=chunk_id)
        construction_time = time.perf_counter() - construction_start
        agent.set_context_construction_time(context_id=context_id, seconds=construction_time)

        for query_data in query_answer_pairs:
            if args.max_test_queries > 0 and query_index >= args.max_test_queries:
                break

            query, answer, qa_pair_id = unpack_query_data(query_data)
            output = agent.send_message(
                query,
                memorizing=False,
                query_id=query_index,
                context_id=context_id,
            )
            output["memory_construction_time"] = construction_time
            retrieval_diagnostics = compute_retrieval_diagnostics(
                retrieved_texts=list(output.get("retrieved_texts") or []),
                answer=answer,
            )
            output.update(retrieval_diagnostics)
            append_retrieval_metrics(retrieval_metrics, retrieval_diagnostics)

            metrics, results = metrics_summarization(
                output,
                query,
                answer,
                dataset_config,
                metrics,
                results,
                query_index,
                qa_pair_id,
            )
            query_index += 1

            if args.save_every > 0 and query_index % args.save_every == 0:
                save_results(
                    output_path=output_path,
                    agent_config=agent_config,
                    dataset_config=dataset_config,
                    results=results,
                    metrics=metrics,
                    retrieval_metrics=retrieval_metrics,
                    time_cost=time_cost,
                    started_at=started_at,
                    run_metadata=run_metadata,
                )

    save_results(
        output_path=output_path,
        agent_config=agent_config,
        dataset_config=dataset_config,
        results=results,
        metrics=metrics,
        retrieval_metrics=retrieval_metrics,
        time_cost=time_cost,
        started_at=started_at,
        run_metadata=run_metadata,
    )
    print(json.dumps({"event": "done", "output_path": str(output_path)}, ensure_ascii=False), flush=True)


def unpack_query_data(query_data: Any) -> tuple[Any, Any, Any]:
    if len(query_data) == 3:
        return query_data
    if len(query_data) == 2:
        query, answer = query_data
        return query, answer, None
    raise ValueError(f"unexpected query data shape: {query_data!r}")


def read_git_commit(path: Path) -> str:
    head = path / ".git" / "HEAD"
    if not head.exists():
        return ""
    content = head.read_text(encoding="utf-8").strip()
    if content.startswith("ref: "):
        ref_path = path / ".git" / content.removeprefix("ref: ").strip()
        if ref_path.exists():
            return ref_path.read_text(encoding="utf-8").strip()
    return content


def main() -> None:
    parser = build_arg_parser()
    args = parser.parse_args()
    run(args)


if __name__ == "__main__":
    main()
