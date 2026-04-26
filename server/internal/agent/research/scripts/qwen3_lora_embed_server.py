#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "accelerate>=1.0.0",
#   "peft>=0.18.0",
#   "safetensors>=0.4.5",
#   "torch>=2.6.0",
#   "transformers>=4.57.0",
# ]
# ///
from __future__ import annotations

import argparse
import json
import sys
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any

import torch
import torch.nn.functional as F
from peft import PeftModel
from transformers import AutoModel, AutoTokenizer


DEFAULT_MODEL_NAME = "Qwen/Qwen3-Embedding-0.6B"
DEFAULT_ADAPTER_PATH = "models/qwen3-episodic-lora"


class Qwen3LoraEmbedder:
    def __init__(
        self,
        *,
        model_name: str,
        adapter_path: Path,
        device_name: str,
        dtype_name: str,
        max_length: int,
        batch_size: int,
    ) -> None:
        self.model_name = model_name
        self.adapter_path = adapter_path
        self.device = choose_device(device_name)
        self.dtype = choose_dtype(dtype_name, self.device)
        self.max_length = max_length
        self.batch_size = batch_size

        print(
            json.dumps(
                {
                    "event": "loading_model",
                    "model_name": self.model_name,
                    "adapter_path": str(self.adapter_path),
                    "device": str(self.device),
                    "dtype": str(self.dtype),
                    "max_length": self.max_length,
                    "batch_size": self.batch_size,
                },
                ensure_ascii=False,
            ),
            flush=True,
        )

        self.tokenizer = AutoTokenizer.from_pretrained(
            self.adapter_path if (self.adapter_path / "tokenizer_config.json").exists() else self.model_name,
            trust_remote_code=True,
        )
        if self.tokenizer.pad_token is None:
            self.tokenizer.pad_token = self.tokenizer.eos_token
        self.tokenizer.padding_side = "right"

        base_model = AutoModel.from_pretrained(
            self.model_name,
            trust_remote_code=True,
            torch_dtype=self.dtype,
        )
        self.model = PeftModel.from_pretrained(base_model, self.adapter_path)
        self.model.eval()
        self.model.to(self.device)
        if hasattr(self.model.config, "use_cache"):
            self.model.config.use_cache = False

        hidden_size = getattr(self.model.config, "hidden_size", None)
        if not isinstance(hidden_size, int):
            hidden_size = getattr(base_model.config, "hidden_size", None)
        if not isinstance(hidden_size, int):
            raise ValueError("could not determine model hidden size")
        self.dimensions = hidden_size

        print(
            json.dumps(
                {
                    "event": "model_ready",
                    "dimensions": self.dimensions,
                    "device": str(self.device),
                },
                ensure_ascii=False,
            ),
            flush=True,
        )

    @torch.inference_mode()
    def embed_texts(self, texts: list[str]) -> list[list[float]]:
        embeddings: list[list[float]] = []
        for start in range(0, len(texts), self.batch_size):
            batch_texts = texts[start : start + self.batch_size]
            embeddings.extend(self._embed_batch(batch_texts))
        return embeddings

    def _embed_batch(self, texts: list[str]) -> list[list[float]]:
        eos = self.tokenizer.eos_token or ""
        if eos:
            texts = [
                text if text.rstrip().endswith(eos) else text + eos
                for text in texts
            ]

        batch = self.tokenizer(
            texts,
            padding=True,
            truncation=True,
            max_length=self.max_length,
            return_tensors="pt",
        )
        batch = {key: value.to(self.device) for key, value in batch.items()}

        outputs = self.model(**batch)
        hidden = outputs.last_hidden_state
        attention_mask = batch["attention_mask"]
        last_token_indices = attention_mask.sum(dim=1) - 1
        pooled = hidden[
            torch.arange(hidden.size(0), device=hidden.device),
            last_token_indices,
        ]
        normalized = F.normalize(pooled.float(), p=2, dim=-1)
        return normalized.cpu().tolist()


class EmbedHandler(BaseHTTPRequestHandler):
    server: "EmbedHTTPServer"

    def do_GET(self) -> None:
        if self.path != "/healthz":
            self.send_json({"error": "not found"}, status=404)
            return

        self.send_json(
            {
                "status": "ok",
                "model": self.server.model_label,
                "base_model": self.server.embedder.model_name,
                "adapter_path": str(self.server.embedder.adapter_path),
                "dimensions": self.server.embedder.dimensions,
                "device": str(self.server.embedder.device),
            }
        )

    def do_POST(self) -> None:
        if self.path != "/api/embed":
            self.send_json({"error": "not found"}, status=404)
            return

        try:
            request = self.read_json()
            started = time.perf_counter()
            input_value = request.get("input")
            texts = normalize_input(input_value)
            requested_dimensions = request.get("dimensions")
            if requested_dimensions is not None:
                requested_dimensions = int(requested_dimensions)
                if requested_dimensions != self.server.embedder.dimensions:
                    raise ValueError(
                        "dimension mismatch: "
                        f"requested={requested_dimensions} "
                        f"model={self.server.embedder.dimensions}"
                    )

            embeddings = self.server.embedder.embed_texts(texts)
            duration_ns = int((time.perf_counter() - started) * 1_000_000_000)
            self.send_json(
                {
                    "model": request.get("model") or self.server.model_label,
                    "embeddings": embeddings,
                    "total_duration": duration_ns,
                    "prompt_eval_count": sum(len(text.split()) for text in texts),
                }
            )
        except Exception as exc:
            self.send_json({"error": str(exc)}, status=400)

    def read_json(self) -> dict[str, Any]:
        raw_length = self.headers.get("Content-Length")
        if raw_length is None:
            raise ValueError("missing Content-Length")
        length = int(raw_length)
        raw_body = self.rfile.read(length)
        payload = json.loads(raw_body.decode("utf-8"))
        if not isinstance(payload, dict):
            raise ValueError("request body must be a JSON object")
        return payload

    def send_json(self, payload: dict[str, Any], *, status: int = 200) -> None:
        raw = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def log_message(self, format: str, *args: Any) -> None:
        print(
            json.dumps(
                {
                    "event": "request",
                    "client": self.address_string(),
                    "message": format % args,
                },
                ensure_ascii=False,
            ),
            file=sys.stderr,
            flush=True,
        )


class EmbedHTTPServer(ThreadingHTTPServer):
    def __init__(
        self,
        server_address: tuple[str, int],
        handler_class: type[BaseHTTPRequestHandler],
        *,
        embedder: Qwen3LoraEmbedder,
        model_label: str,
    ) -> None:
        super().__init__(server_address, handler_class)
        self.embedder = embedder
        self.model_label = model_label


def normalize_input(input_value: Any) -> list[str]:
    if isinstance(input_value, str):
        return [input_value]
    if isinstance(input_value, list):
        texts = []
        for value in input_value:
            if not isinstance(value, str):
                raise ValueError("all input list items must be strings")
            texts.append(value)
        return texts
    raise ValueError("input must be a string or list of strings")


def choose_device(device_name: str) -> torch.device:
    if device_name != "auto":
        return torch.device(device_name)
    if torch.cuda.is_available():
        return torch.device("cuda")
    if torch.backends.mps.is_available():
        return torch.device("mps")
    return torch.device("cpu")


def choose_dtype(dtype_name: str, device: torch.device) -> torch.dtype:
    if dtype_name == "float32":
        return torch.float32
    if dtype_name == "float16":
        return torch.float16
    if dtype_name == "bfloat16":
        return torch.bfloat16
    if dtype_name != "auto":
        raise ValueError(f"unsupported dtype: {dtype_name}")

    if device.type == "cuda":
        return torch.bfloat16 if torch.cuda.is_bf16_supported() else torch.float16
    if device.type == "mps":
        return torch.float16
    return torch.float32


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Serve a Qwen3 LoRA embedding model through an Ollama-compatible /api/embed endpoint.",
    )
    parser.add_argument("--model-name", default=DEFAULT_MODEL_NAME)
    parser.add_argument("--adapter-path", type=Path, default=Path(DEFAULT_ADAPTER_PATH))
    parser.add_argument("--model-label", default="qwen3-episodic-lora")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=11500)
    parser.add_argument("--device", default="auto")
    parser.add_argument(
        "--dtype",
        choices=("auto", "float32", "float16", "bfloat16"),
        default="auto",
    )
    parser.add_argument("--max-length", type=int, default=512)
    parser.add_argument("--batch-size", type=int, default=16)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    adapter_path = args.adapter_path.expanduser().resolve()
    if not adapter_path.exists():
        raise FileNotFoundError(f"adapter path not found: {adapter_path}")
    if args.max_length <= 0:
        raise ValueError("max-length must be positive")
    if args.batch_size <= 0:
        raise ValueError("batch-size must be positive")

    embedder = Qwen3LoraEmbedder(
        model_name=args.model_name,
        adapter_path=adapter_path,
        device_name=args.device,
        dtype_name=args.dtype,
        max_length=args.max_length,
        batch_size=args.batch_size,
    )
    server = EmbedHTTPServer(
        (args.host, args.port),
        EmbedHandler,
        embedder=embedder,
        model_label=args.model_label,
    )
    print(
        json.dumps(
            {
                "event": "server_started",
                "host": args.host,
                "port": args.port,
                "endpoint": f"http://{args.host}:{args.port}/api/embed",
            },
            ensure_ascii=False,
        ),
        flush=True,
    )
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print(json.dumps({"event": "server_stopped"}), flush=True)


if __name__ == "__main__":
    main()
