"""
clip_service.py

A gRPC service for the general-purpose CLIP model, structured to mirror the
elegant design of the BioCLIP service. It uses the streaming Inference protocol
to expose three distinct tasks:
  - embed: Creates a vector embedding from a text string.
  - classify: Classifies an image against the default ImageNet dataset.
  - classify_scene: Performs a high-level scene analysis on an image.
"""

import json
import logging
import time
from typing import Dict, Iterable, Tuple

import grpc
from google.protobuf import empty_pb2

from proto import ml_service_pb2 as pb
from proto import ml_service_pb2_grpc as rpc
from .clip_model import CLIPModelManager

logger = logging.getLogger(__name__)


def _now_ms() -> int:
    """Returns the current time in milliseconds."""
    return int(time.time() * 1000)


class CLIPService(rpc.InferenceServicer):
    """
    Implements the streaming Inference service contract for a general-purpose
    OpenCLIP model, offering text embedding, ImageNet classification, and
    scene classification tasks.
    """

    SERVICE_NAME = "clip-general"

    def __init__(self, model_name: str = "ViT-B-32", pretrained: str = "laion2b_s34b_b79k") -> None:
        self.model = CLIPModelManager(model_name=model_name, pretrained=pretrained)
        self.is_initialized = False

    def initialize(self) -> None:
        """Loads the model and prepares it for inference."""
        logger.info("Initializing CLIPModelManager...")
        self.model.initialize()
        self.is_initialized = True
        info = self.model.info()
        logger.info(
            "CLIP model ready: %s (%s) on %s (loaded in %.2fs)",
            info.get("model_name"),
            info.get("pretrained"),
            info.get("device"),
            info.get("load_time_seconds"),
        )

    # -------- gRPC Service Methods ----------

    def Infer(self, request_iterator: Iterable[pb.InferRequest], context: grpc.ServicerContext):
        """
        Handles the bidirectional streaming inference RPC. It routes incoming requests
        to the appropriate task handler.
        """
        if not self.is_initialized:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, "Model not initialized")

        buffers: Dict[str, bytearray] = {}  # Buffers for reassembling chunked requests

        for req in request_iterator:
            cid = req.correlation_id or f"cid-{_now_ms()}"
            t0 = _now_ms()

            try:
                # 1. Reassemble payload if it was sent in chunks
                payload, ready = self._assemble(cid, req, buffers)
                if not ready:
                    continue

                # 2. Route to the correct handler based on the task
                if req.task == "embed":
                    result_bytes, result_mime, extra_meta = self._handle_embed(payload, dict(req.meta))
                elif req.task == "classify":
                    result_bytes, result_mime, extra_meta = self._handle_classify(payload, dict(req.meta))
                elif req.task == "classify_scene":
                    result_bytes, result_mime, extra_meta = self._handle_classify_scene(payload, dict(req.meta))
                else:
                    yield pb.InferResponse(
                        correlation_id=cid,
                        is_final=True,
                        error=pb.Error(code=pb.ERROR_CODE_INVALID_ARGUMENT, message=f"Unknown task: {req.task}"),
                    )
                    continue

                # 3. Yield a successful response
                meta = dict(extra_meta or {})
                meta["lat_ms"] = str(_now_ms() - t0)

                yield pb.InferResponse(
                    correlation_id=cid,
                    is_final=True,
                    result=result_bytes,
                    result_mime=result_mime,
                    meta=meta,
                )

            except Exception as e:
                logger.exception("Error during inference for task '%s': %s", req.task, e)
                yield pb.InferResponse(
                    correlation_id=cid,
                    is_final=True,
                    error=pb.Error(code=pb.ERROR_CODE_INTERNAL, message=str(e)),
                )

    def GetCapabilities(self, request, context) -> pb.Capability:
        """Returns the capabilities of the service in a single response. [cite: 21]"""
        return self._build_capability()

    def StreamCapabilities(self, request, context) -> Iterable[pb.Capability]:
        """Streams the capabilities of the service."""
        yield self._build_capability()

    def Health(self, request, context):
        """A simple health check endpoint. [cite: 22]"""
        return empty_pb2.Empty()

    # -------- Task Handlers ----------

    def _handle_embed(self, payload: bytes, meta: Dict[str, str]) -> Tuple[bytes, str, Dict[str, str]]:
        """Handles text embedding requests."""
        text = payload.decode("utf-8")
        vec = self.model.encode_text(text).tolist()

        info = self.model.info()
        model_id = f"{info.get('model_name')}:{info.get('pretrained')}"

        obj = {"vector": vec, "dim": len(vec), "model_id": model_id}
        return (
            json.dumps(obj, separators=(",", ":")).encode("utf-8"),
            "application/json;schema=embedding_v1",
            {"dim": str(len(vec))},
        )

    def _handle_classify(self, payload: bytes, meta: Dict[str, str]) -> Tuple[bytes, str, Dict[str, str]]:
        """Handles ImageNet classification requests."""
        top_k = int(meta.get("topk", "5"))
        scores = self.model.classify_image(payload, top_k=top_k)

        info = self.model.info()
        model_id = f"{info.get('model_name')}:{info.get('pretrained')}"

        obj = {
            "labels": [{"label": label, "score": float(score)} for label, score in scores],
            "model_id": model_id,
        }
        return (
            json.dumps(obj, separators=(",", ":")).encode("utf-8"),
            "application/json;schema=labels_v1",
            {"labels_count": str(len(scores))},
        )

    def _handle_classify_scene(self, payload: bytes, meta: Dict[str, str]) -> Tuple[bytes, str, Dict[str, str]]:
        """Handles scene classification requests."""
        label, score = self.model.classify_scene(payload)

        info = self.model.info()
        model_id = f"{info.get('model_name')}:{info.get('pretrained')}"

        # The labels_v1 schema supports multiple labels, so we format the single result into a list
        obj = {
            "labels": [{"label": label, "score": float(score)}],
            "model_id": model_id,
        }
        return (
            json.dumps(obj, separators=(",", ":")).encode("utf-8"),
            "application/json;schema=labels_v1",
            {"labels_count": "1"},
        )


    # -------- Helpers ----------

    def _assemble(self, cid: str, req: pb.InferRequest, buffers: Dict[str, bytearray]) -> Tuple[bytes, bool]:
        """
        Reassembles chunked request payloads.

        Returns a tuple of (payload_bytes, ready). If a request is not chunked,
        it is returned immediately with ready=True. If chunked, data is buffered
        until the final chunk arrives.
        """
        # Default path for non-chunked requests
        if req.total <= 1:
            return bytes(req.payload), True

        # Append chunk to buffer
        buf = buffers.setdefault(cid, bytearray())
        buf.extend(req.payload)

        # Check if all chunks have arrived
        if req.total and (req.seq + 1 == req.total):
            data = bytes(buf)
            del buffers[cid]
            return data, True

        return b"", False # Not ready yet

    def _build_capability(self) -> pb.Capability:
        """Constructs the capability message based on the model's current state."""
        info = self.model.info()
        model_id = f"{info.get('model_name')}:{info.get('pretrained', 'unknown')}"

        return pb.Capability(
            service_name=self.SERVICE_NAME,
            model_ids=[model_id],
            runtime="torch",
            max_concurrency=4,
            precisions=["fp32", "fp16"],
            extra={"device": str(info.get("device", "cpu"))},
            tasks=[
                pb.IOTask(
                    name="embed",
                    input_mimes=["text/plain;charset=utf-8"],
                    output_mimes=["application/json;schema=embedding_v1"],
                ),
                pb.IOTask(
                    name="classify",
                    input_mimes=["image/jpeg", "image/png"],
                    output_mimes=["application/json;schema=labels_v1"],
                    limits={"topk_max": "50"},
                ),
                pb.IOTask(
                    name="classify_scene",
                    input_mimes=["image/jpeg", "image/png"],
                    output_mimes=["application/json;schema=labels_v1"],
                ),
            ],
        )
