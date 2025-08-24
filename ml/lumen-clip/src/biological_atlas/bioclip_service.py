# clip_inference_service.py
import json
import logging
import time
from typing import Dict, Iterable, Tuple

import grpc
from google.protobuf import empty_pb2

# 这里就是用"新的proto"生成的模块
from proto import ml_service_pb2 as pb
from proto import ml_service_pb2_grpc as rpc
from .bioclip_model import BioCLIPModelManager

logger = logging.getLogger(__name__)


def _now_ms() -> int:
    return int(time.time() * 1000)


class BioClipService(rpc.InferenceServicer):
    """
    使用新的 Inference 协议，统一承载两个服务：
      - 文本嵌入：task="embed"
      - BioAtlas 分类：task="classify", meta.namespace="bioatlas"
    """

    MODEL_VERSION = "bioclip2"

    def __init__(self) -> None:
        self.model = BioCLIPModelManager()
        self.start_time = time.time()
        self.is_initialized = False

    # -------- lifecycle ----------
    def initialize(self) -> None:
        logger.info("Initializing BioCLIP model...")
        self.model.initialize()
        self.is_initialized = True
        info = self.model.info()
        logger.info("BioCLIP ready on %s (load %.2fs)", info.get("device"), info.get("load_time"))

    # -------- Inference ----------
    def Infer(self, request_iterator: Iterable[pb.InferRequest], context: grpc.ServicerContext):
        """
        双向流的服务端实现（同步 style：生成器按需 yield 响应）。
        embed / classify 都是一发一收；如客户端使用了分片 seq/total，这里也支持重组。
        """
        if not self.is_initialized:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, "Model not initialized")

        buffers: Dict[str, bytearray] = {}  # correlation_id -> buffer

        for req in request_iterator:
            cid = req.correlation_id or f"cid-{_now_ms()}"
            t0 = _now_ms()

            try:
                # --- 分片重组（可选）---
                payload, ready = self._assemble(cid, req, buffers)
                if not ready:
                    # 分片尚未收齐，不返回响应，继续等下一个分片
                    continue

                # --- 路由任务 ---
                if req.task == "embed":
                    result_bytes, result_mime, extra_meta = self._handle_embed(req.payload_mime, payload, dict(req.meta))
                elif req.task == "classify":
                    result_bytes, result_mime, extra_meta = self._handle_classify(req.payload_mime, payload, dict(req.meta))
                else:
                    yield pb.InferResponse(
                        correlation_id=cid,
                        is_final=True,
                        error=pb.Error(code=pb.ERROR_CODE_INVALID_ARGUMENT, message=f"Unknown task: {req.task}"),
                    )
                    continue

                # --- 成功响应 ---
                meta = dict(extra_meta or {})
                meta["lat_ms"] = str(_now_ms() - t0)

                yield pb.InferResponse(
                    correlation_id=cid,
                    is_final=True,
                    result=result_bytes,
                    result_mime=result_mime,      # e.g. application/json;schema=embedding_v1
                    meta=meta,
                    seq=0,
                    total=1,
                    offset=0,
                    result_schema="",              # 可留空；有需要再填 "embedding_v1"/"labels_v1"
                )

            except grpc.RpcError:
                raise
            except Exception as e:
                logger.exception("Infer error: %s", e)
                yield pb.InferResponse(
                    correlation_id=cid,
                    is_final=True,
                    error=pb.Error(code=pb.ERROR_CODE_INTERNAL, message=str(e)),
                )

    # -------- Capabilities / Health ----------
    def GetCapabilities(self, request, context) -> pb.Capability:
        return self._build_capability()

    def StreamCapabilities(self, request, context):
        # 单实例就发一条；如果后续有动态热更，可以定时/按需再发
        yield self._build_capability()

    def Health(self, request, context):
        # 需要更细的健康信息可以扩展；当前协议就是 Empty
        return empty_pb2.Empty()

    # -------- 具体任务：embed / classify ----------
    def _handle_embed(self, payload_mime: str, payload: bytes, meta: Dict[str, str]) -> Tuple[bytes, str, Dict[str, str]]:
        """
        文本嵌入：
          - 期望 payload_mime: "text/plain;charset=utf-8"
          - 输出 result_mime:  "application/json;schema=embedding_v1"
          - 输出 JSON: {"vector":[...], "dim":768, "model_id":"bioclip2"}
        """
        if not payload_mime.startswith("text/"):
            raise ValueError(f"embed expects text/* payload, got {payload_mime!r}")
        text = payload.decode("utf-8")
        vec = self.model.encode_text([text])[0].tolist()  # type: ignore[attr-defined]
        obj = {"vector": vec, "dim": len(vec), "model_id": self.MODEL_VERSION}
        return (
            json.dumps(obj, separators=(",", ":")).encode("utf-8"),
            "application/json;schema=embedding_v1",
            {"dim": str(len(vec))},
        )

    def _handle_classify(self, payload_mime: str, payload: bytes, meta: Dict[str, str]) -> Tuple[bytes, str, Dict[str, str]]:
        """
        BioAtlas 分类：
          - 期望 payload_mime: "image/jpeg" / "image/png"
          - 期望 meta.namespace="bioatlas"
          - 可选 meta.topk（默认 5）
          - 输出 result_mime: "application/json;schema=labels_v1"
          - 输出 JSON: {"labels":[{"label":"...","score":0.91},...], "model_id":"bioclip2"}
        """
        if not payload_mime.startswith("image/"):
            raise ValueError(f"classify expects image/* payload, got {payload_mime!r}")

        namespace = (meta or {}).get("namespace", "bioatlas")
        if namespace != "bioatlas":
            raise ValueError(f"unsupported namespace {namespace!r}, expected 'bioatlas'")

        topk = int((meta or {}).get("topk", "5"))
        pairs = self.model.classify_image(payload)[:topk]  # List[Tuple[str, float]]

        obj = {
            "labels": [{"label": name, "score": float(score)} for name, score in pairs],
            "model_id": self.MODEL_VERSION,
        }
        return (
            json.dumps(obj, separators=(",", ":")).encode("utf-8"),
            "application/json;schema=labels_v1",
            {"labels_count": str(len(pairs))},
        )

    # -------- buffers / 分片重组 ----------
    def _assemble(self, cid: str, req: pb.InferRequest, buffers: Dict[str, bytearray]) -> Tuple[bytes, bool]:
        """
        返回 (payload_bytes, ready)
        - 若客户端不使用 seq/total：直接 ready=True
        - 若使用 seq/total：缓存到 buffers[cid]，直到收齐（seq+1==total）才 ready=True
        """
        # 无分片（默认路径）
        if not req.total and not req.seq and not req.offset:
            return bytes(req.payload), True

        buf = buffers.setdefault(cid, bytearray())
        buf.extend(req.payload)
        if req.total and (req.seq + 1 == req.total):
            data = bytes(buf)
            del buffers[cid]
            return data, True
        return b"", False

    # -------- Capability ----------
    def _build_capability(self) -> pb.Capability:
        info = {}
        try:
            info = self.model.info()
        except Exception:
            pass

        return pb.Capability(
            service_name="clip-bioclip",
            model_ids=[self.MODEL_VERSION],
            runtime=str(info.get("runtime", "onnxrt-cuda")),
            max_concurrency=4,
            precisions=["fp16", "fp32"],
            extra={"device": str(info.get("device", "cuda:0"))},
            tasks=[
                pb.IOTask(
                    name="embed",
                    input_mimes=["text/plain;charset=utf-8"],
                    output_mimes=["application/json;schema=embedding_v1"],
                    limits={"dim": "768"},
                ),
                pb.IOTask(
                    name="classify",
                    input_mimes=["image/jpeg", "image/png"],
                    output_mimes=["application/json;schema=labels_v1"],
                    limits={"topk_max": "50"},
                ),
            ],
        )
