"""
gRPC server for ML prediction services.

This module initializes and serves a gRPC endpoint that delegates
prediction requests to registered model services, such as CLIP.
"""

import grpc
import time
import logging
import sys
import os
from concurrent import futures
from typing import Dict, Any, Optional
from dotenv import load_dotenv

# Include local source for module resolution.
SRC_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "src")
if SRC_DIR not in sys.path:
    sys.path.insert(0, SRC_DIR)

from utils.logging_config import setup_logging
from proto import ml_service_pb2, ml_service_pb2_grpc
from image_classification.clip_service import CLIPService

# Load and configure environment
load_dotenv()
MODEL_PATH = os.environ.get("MODEL_PATH", "./pt")

# Configure root logger
setup_logging(log_level=logging.INFO)
logger = logging.getLogger(__name__)


class ModelRegistry:
    """
    Manage registration and lookup of ML model services.

    Attributes:
        services: Mapping of service names to service instances.
        start_time: Timestamp when the registry was created.
    """

    def __init__(self) -> None:
        self.services: Dict[str, Any] = {}
        self.start_time = time.time()

    def register_service(self, name: str, service: Any) -> None:
        """
        Add a model service to the registry.

        Args:
            name: Identifier for the service.
            service: Instance providing the service interface.
        """
        self.services[name] = service
        logger.info("Service '%s' registered.", name)

    def get_service(self, name: str) -> Optional[Any]:
        """
        Retrieve a registered service by name.

        Args:
            name: Identifier of the service to fetch.

        Returns:
            The service instance if found; otherwise None.
        """
        return self.services.get(name)

    def list_services(self) -> list:
        """
        List all registered service names.

        Returns:
            A list of service identifiers.
        """
        return list(self.services.keys())

    def get_uptime(self) -> int:
        """
        Calculate uptime since registry initialization.

        Returns:
            Uptime in seconds.
        """
        return int(time.time() - self.start_time)


class PredictionServiceServicer(ml_service_pb2_grpc.PredictionServiceServicer):
    """
    gRPC servicer implementing prediction and health-check endpoints.

    Delegates incoming calls to specific model services registered
    in the ModelRegistry.
    """

    def __init__(self) -> None:
        self.model_registry = ModelRegistry()
        self._initialize_services()

    def _initialize_services(self) -> None:
        """
        Initialize and register available model services.

        Currently registers a CLIPService instance.
        """
        try:
            clip = CLIPService(model_name="ViT-B-32", pretrained="laion2b_s34b_b79k")
            clip.initialize()
            self.model_registry.register_service("clip", clip)
            logger.info("CLIP service initialized.")
        except Exception:
            logger.exception("Failed to initialize CLIP service.")

    def ProcessImageForCLIP(self, request, context):
        """
        Handle image processing requests for CLIP.

        Args:
            request: Protobuf request containing raw image bytes.
            context: gRPC context for status and metadata.

        Returns:
            ImageProcessResponse with processed image features.
        """
        service = self.model_registry.get_service("clip")
        if not service:
            context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
            context.set_details("CLIP service not available")
            return ml_service_pb2.ImageProcessResponse()
        try:
            return service.process_image_for_clip(request, context)
        except Exception:
            logger.exception("Error in ProcessImageForCLIP")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details("Internal server error")
            return ml_service_pb2.ImageProcessResponse()

    def GetTextEmbeddingForCLIP(self, request, context):
        """
        Handle text embedding requests for CLIP.

        Args:
            request: Protobuf request containing text input.
            context: gRPC context for status and metadata.

        Returns:
            TextEmbeddingResponse with embedding vector.
        """
        service = self.model_registry.get_service("clip")
        if not service:
            context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
            context.set_details("CLIP service not available")
            return ml_service_pb2.TextEmbeddingResponse()
        try:
            return service.get_text_embedding_for_clip(request, context)
        except Exception:
            logger.exception("Error in GetTextEmbeddingForCLIP")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details("Internal server error")
            return ml_service_pb2.TextEmbeddingResponse()

    def Predict(self, request, context):
        """
        Generic prediction endpoint supporting multiple models.

        Dispatches based on request.model_name. Supports CLIP for
        both image and text inputs.

        Args:
            request: PredictRequest containing model name and input data.
            context: gRPC context for status and metadata.

        Returns:
            PredictResponse with float values and metadata.
        """
        model_name = request.model_name.lower()
        try:
            if model_name == "clip":
                service = self.model_registry.get_service("clip")
                if not service:
                    context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
                    context.set_details("CLIP service not available")
                    return ml_service_pb2.PredictResponse()

                if request.HasField("raw_data"):
                    features = service.clip_model.encode_image(request.raw_data)
                elif request.HasField("text_input"):
                    features = service.clip_model.encode_text(request.text_input)
                else:
                    context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                    context.set_details("No input data provided")
                    return ml_service_pb2.PredictResponse()

                floats = ml_service_pb2.PredictionFloats(values=features.tolist())
                return ml_service_pb2.PredictResponse(
                    prediction_floats=floats,
                    confidence=1.0,
                    model_name=model_name,
                    model_version=service.clip_model.model_name,
                    prediction_time_ms=int(time.time() * 1000),
                )
            context.set_code(grpc.StatusCode.UNIMPLEMENTED)
            context.set_details(f"Model '{model_name}' not implemented")
            return ml_service_pb2.PredictResponse()
        except Exception:
            logger.exception("Error in Predict")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details("Internal server error")
            return ml_service_pb2.PredictResponse()

    def BatchPredict(self, request, context):
        """
        Process a batch of prediction requests.

        Args:
            request: BatchPredictRequest with multiple PredictRequests.
            context: gRPC context for status and metadata.

        Returns:
            BatchPredictResponse summarizing successes and failures.
        """
        responses = []
        success = failed = 0
        for req in request.requests:
            if request.model_name:
                req.model_name = request.model_name
            try:
                resp = self.Predict(req, context)
                responses.append(resp)
                success += 1
            except Exception:
                logger.warning("Failed to process batch item", exc_info=True)
                responses.append(ml_service_pb2.PredictResponse())
                failed += 1

        return ml_service_pb2.BatchPredictResponse(
            responses=responses, success_count=success, failed_count=failed
        )

    def HealthCheck(self, request, context):
        """
        Health check endpoint for individual or all services.

        Args:
            request: HealthCheckRequest specifying a service name or 'all'.
            context: gRPC context for status and metadata.

        Returns:
            HealthCheckResponse with status, uptime, and message.
        """
        name = request.service_name.lower()
        try:
            if name == "all":
                services = self.model_registry.list_services()
                status = (
                    ml_service_pb2.HealthCheckResponse.SERVING
                    if services
                    else ml_service_pb2.HealthCheckResponse.NOT_SERVING
                )
                message = (
                    f"Available services: {', '.join(services)}"
                    if services
                    else "No services registered"
                )
                return ml_service_pb2.HealthCheckResponse(
                    status=status,
                    model_name="all",
                    model_version="mixed",
                    uptime_seconds=self.model_registry.get_uptime(),
                    message=message,
                )
            if name == "clip":
                service = self.model_registry.get_service("clip")
                if service:
                    return service.health_check()
                return ml_service_pb2.HealthCheckResponse(
                    status=ml_service_pb2.HealthCheckResponse.NOT_SERVING,
                    model_name="clip",
                    model_version="unknown",
                    uptime_seconds=self.model_registry.get_uptime(),
                    message="CLIP service not available",
                )
            return ml_service_pb2.HealthCheckResponse(
                status=ml_service_pb2.HealthCheckResponse.UNKNOWN,
                model_name=name,
                model_version="unknown",
                uptime_seconds=self.model_registry.get_uptime(),
                message=f"Unknown service '{name}'",
            )
        except Exception:
            logger.exception("Error in HealthCheck")
            return ml_service_pb2.HealthCheckResponse(
                status=ml_service_pb2.HealthCheckResponse.SERVICE_SPECIFIC_ERROR,
                model_name=request.service_name,
                model_version="unknown",
                uptime_seconds=self.model_registry.get_uptime(),
                message="Health check failed",
            )


def serve(port: int = 50051, max_workers: int = 10) -> None:
    """
    Start the gRPC server for ML prediction.

    Args:
        port: TCP port number to listen on.
        max_workers: Maximum number of thread pool workers.
    """
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=max_workers))
    ml_service_pb2_grpc.add_PredictionServiceServicer_to_server(
        PredictionServiceServicer(), server
    )
    server.add_insecure_port(f"[::]:{port}")
    server.start()

    logger.info("gRPC server listening on port %d", port)
    try:
        server.wait_for_termination()
    except KeyboardInterrupt:
        server.stop(0)
        logger.info("Server shut down")


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description="ML Prediction gRPC Server")
    parser.add_argument("--port", type=int, default=50051, help="Server port")
    parser.add_argument(
        "--workers", type=int, default=10, help="Max worker threads"
    )
    args = parser.parse_args()
    serve(port=args.port, max_workers=args.workers)
