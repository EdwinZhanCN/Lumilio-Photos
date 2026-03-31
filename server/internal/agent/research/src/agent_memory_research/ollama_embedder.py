from __future__ import annotations

from typing import Any

import httpx


class OllamaEmbedder:
    def __init__(
        self,
        *,
        base_url: str = "http://localhost:11434",
        model: str,
        dimensions: int,
        keep_alive: str = "5m",
        timeout: float = 30.0,
    ) -> None:
        if not model.strip():
            raise ValueError("embedding model is required")
        if dimensions <= 0:
            raise ValueError("embedding dimensions must be positive")
        self.base_url = base_url.rstrip("/")
        self.model = model
        self.dimensions = dimensions
        self.keep_alive = keep_alive
        self.client = httpx.Client(timeout=timeout)

    def embed_text(self, text: str) -> list[float]:
        if not text.strip():
            return [0.0] * self.dimensions

        body: dict[str, Any] = {
            "model": self.model,
            "input": text,
            "dimensions": self.dimensions,
        }
        if self.keep_alive.strip():
            body["keep_alive"] = self.keep_alive

        response = self.client.post(f"{self.base_url}/api/embed", json=body)
        response.raise_for_status()
        payload = response.json()
        embeddings = payload.get("embeddings", [])
        if not embeddings:
            raise ValueError("ollama embed response did not include embeddings")

        vector = embeddings[0]
        if len(vector) != self.dimensions:
            raise ValueError(
                f"embedding dimension mismatch: expected {self.dimensions}, got {len(vector)}"
            )
        return [float(value) for value in vector]
