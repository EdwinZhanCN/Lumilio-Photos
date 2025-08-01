import logging
import time

import grpc
from proto import ml_service_pb2
from .bioclip_model import BioCLIPModelManager

logger = logging.getLogger(__name__)

class BioCLIPService:
    """gRPC façade around BioCLIPModelManager with fixed bioclip2 version."""

    MODEL_VERSION = "bioclip2"

    def __init__(self) -> None:
        self.biomodel = BioCLIPModelManager()
        self.start_time = time.time()
        self.is_initialized = False
        logger.info("BioCLIPService instantiated with fixed model version %s", self.MODEL_VERSION)

    def initialize(self) -> None:
        logger.info("Initializing BioCLIP model...")
        self.biomodel.initialize()
        self.is_initialized = True
        info = self.biomodel.info()
        logger.info("BioCLIP ready on %s (load %.2fs)", info["device"], info["load_time"])


    def process_image_bioatlas(self, request: ml_service_pb2.ImageProcessRequest, context) -> ml_service_pb2.BioAtlasResponse:
        tic = time.time()
        if not self.is_initialized:
            context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
            return ml_service_pb2.BioAtlasResponse(status=ml_service_pb2.BioAtlasResponse.MODEL_ERROR, message="Model not initialized")
        if not request.image_data:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            return ml_service_pb2.BioAtlasResponse(status=ml_service_pb2.BioAtlasResponse.INVALID_REQUEST, message="No image data provided")
        try:
            result = self.biomodel.classify_image(request.image_data)
            processing_time_ms = int((time.time() - tic) * 1000)
            label_scores = [ml_service_pb2.LabelScore(label=name, similarity_score=score)
                            for name, score in result]
            return ml_service_pb2.BioAtlasResponse(
                image_id=request.image_id,
                predicted_result=label_scores,
                model_version=self.MODEL_VERSION,
                processing_time_ms=processing_time_ms,
                status=ml_service_pb2.BioAtlasResponse.OK,
                message="Processed successfully"
            )

            # message BioAtlasResponse {
            #   // ---- 正常成功时用到的字段 ----
            #   string image_id = 1;
            #   repeated LabelScore predicted_result = 3;   // Top-3 {label, prob}
            #   string model_version = 4;
            #   int64 processing_time_ms = 6;

            #   // ---- 新增：结果状态 ----
            #   enum Status {
            #     OK              = 0;  // 成功返回物种
            #     NOT_ANIMAL      = 1;  // 不是动物，BioAtlas 不适用
            #     MODEL_ERROR     = 2;  // BioCLIP 内部错误
            #     INVALID_REQUEST = 3;  // 图像为空、分辨率过低等
            #   }
            #   Status status = 7;

            #   // （可选）供调试或前端展示的文本
            #   string message = 8;
            # }

        except Exception as exc:
            logger.exception("BioAtlas processing error: %s", exc)
            context.set_code(grpc.StatusCode.INTERNAL)
            return ml_service_pb2.BioAtlasResponse(
                image_id=request.image_id,
                model_version=self.MODEL_VERSION,
                status=ml_service_pb2.BioAtlasResponse.MODEL_ERROR,
                message=str(exc)
            )

    def get_text_embedding(self, request: ml_service_pb2.TextEmbeddingRequest, context) -> ml_service_pb2.TextEmbeddingResponse:
        if not self.is_initialized:
            context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
            return ml_service_pb2.TextEmbeddingResponse()
        if not request.text:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            return ml_service_pb2.TextEmbeddingResponse()
        t0 = time.time()
        vec = self.biomodel.encode_text([request.text])  # type: ignore[attr-defined]
        return ml_service_pb2.TextEmbeddingResponse(
            text_feature_vector=vec[0].tolist(),
            model_version=self.MODEL_VERSION,
            processing_time_ms=int((time.time() - t0) * 1000)
        )

    def health_check(self, service_name: str = "bioclip") -> ml_service_pb2.HealthCheckResponse:
        uptime = int(time.time() - self.start_time)
        status = (
            ml_service_pb2.HealthCheckResponse.SERVING
            if self.is_initialized
            else ml_service_pb2.HealthCheckResponse.NOT_SERVING
        )
        msg = "Model initialized and serving" if status == ml_service_pb2.HealthCheckResponse.SERVING else "Model not available"
        return ml_service_pb2.HealthCheckResponse(
            status=status,
            model_name=service_name,
            model_version=self.MODEL_VERSION,
            uptime_seconds=uptime,
            message=msg,
        )
