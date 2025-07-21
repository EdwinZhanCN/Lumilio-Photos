import grpc
import time
import logging
import sys
import os
from concurrent import futures
from typing import Dict, Any, Optional
from dotenv import load_dotenv

# Add 'src' directory to path for local module resolution. This allows finding modules
# like 'image_classification' and 'proto' which are located in the 'src' folder.
SRC_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "src")
if SRC_DIR not in sys.path:
    sys.path.insert(0, SRC_DIR)

from utils.logging_config import setup_logging

# Load environment variables
load_dotenv()

# Get environment variables with defaults
MODEL_PATH = os.environ.get('MODEL_PATH', './pt')

from proto import ml_service_pb2, ml_service_pb2_grpc
from image_classification.clip_service import CLIPService

# Configure logging
# The log level can be controlled via environment variable in a real app
setup_logging(log_level=logging.INFO)
logger = logging.getLogger(__name__)


class ModelRegistry:
    """Registry for managing multiple ML models and services"""

    def __init__(self):
        self.services: Dict[str, Any] = {}
        self.start_time = time.time()

    def register_service(self, name: str, service: Any):
        """Register a service in the registry"""
        self.services[name] = service
        logger.info(f"✅ Service '{name}' registered successfully.")

    def get_service(self, name: str) -> Optional[Any]:
        """Get a service from the registry"""
        return self.services.get(name)

    def list_services(self) -> list:
        """List all registered services"""
        return list(self.services.keys())

    def get_uptime(self) -> int:
        """Get server uptime in seconds"""
        return int(time.time() - self.start_time)


class PredictionServiceServicer(ml_service_pb2_grpc.PredictionServiceServicer):
    """Main gRPC service implementation that delegates to specific model services"""

    def __init__(self):
        self.model_registry = ModelRegistry()
        self._initialize_services()

    def _initialize_services(self):
        """Initialize all available services"""
        try:
            # Initialize CLIP service
            clip_service = CLIPService(model_name='ViT-B-32', pretrained='laion2b_s34b_b79k')
            clip_service.initialize()
            self.model_registry.register_service('clip', clip_service)
            logger.info("✅ CLIP service initialized successfully.")
        except Exception as e:
            logger.error(f"❌ Failed to initialize CLIP service: {e}", exc_info=True)

    def ProcessImageForCLIP(self, request, context):
        """Process image with CLIP model - delegates to CLIP service"""
        try:
            clip_service = self.model_registry.get_service('clip')
            if not clip_service:
                context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
                context.set_details('CLIP service not available')
                return ml_service_pb2.ImageProcessResponse()

            return clip_service.process_image_for_clip(request, context)

        except Exception as e:
            logger.error(f"❌ Error in ProcessImageForCLIP: {e}", exc_info=True)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f'Internal error: {str(e)}')
            return ml_service_pb2.ImageProcessResponse()

    def GetTextEmbeddingForCLIP(self, request, context):
        """Get text embedding with CLIP model - delegates to CLIP service"""
        try:
            clip_service = self.model_registry.get_service('clip')
            if not clip_service:
                context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
                context.set_details('CLIP service not available')
                return ml_service_pb2.TextEmbeddingResponse()

            return clip_service.get_text_embedding_for_clip(request, context)

        except Exception as e:
            logger.error(f"❌ Error in GetTextEmbeddingForCLIP: {e}", exc_info=True)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f'Internal error: {str(e)}')
            return ml_service_pb2.TextEmbeddingResponse()

    def Predict(self, request, context):
        """Generic prediction interface"""
        try:
            model_name = request.model_name.lower()

            if model_name == 'clip':
                clip_service = self.model_registry.get_service('clip')
                if not clip_service:
                    context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
                    context.set_details('CLIP service not available')
                    return ml_service_pb2.PredictResponse()

                # Handle CLIP-specific prediction
                if request.HasField('raw_data'):
                    # Assume it's image data
                    features = clip_service.clip_model.encode_image(request.raw_data)
                    prediction_floats = ml_service_pb2.PredictionFloats(values=features.tolist())

                    return ml_service_pb2.PredictResponse(
                        prediction_floats=prediction_floats,
                        confidence=1.0,
                        model_name=model_name,
                        model_version=clip_service.clip_model.model_name,
                        prediction_time_ms=int(time.time() * 1000)
                    )
                elif request.HasField('text_input'):
                    features = clip_service.clip_model.encode_text(request.text_input)
                    prediction_floats = ml_service_pb2.PredictionFloats(values=features.tolist())

                    return ml_service_pb2.PredictResponse(
                        prediction_floats=prediction_floats,
                        confidence=1.0,
                        model_name=model_name,
                        model_version=clip_service.clip_model.model_name,
                        prediction_time_ms=int(time.time() * 1000)
                    )

            context.set_code(grpc.StatusCode.UNIMPLEMENTED)
            context.set_details(f'Model {model_name} not implemented')
            return ml_service_pb2.PredictResponse()

        except Exception as e:
            logger.error(f"❌ Error in generic prediction: {e}", exc_info=True)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f'Internal error: {str(e)}')
            return ml_service_pb2.PredictResponse()

    def BatchPredict(self, request, context):
        """Batch prediction interface"""
        try:
            responses = []
            success_count = 0
            failed_count = 0

            for req in request.requests:
                try:
                    # Override model name if specified at batch level
                    if request.model_name:
                        req.model_name = request.model_name

                    response = self.Predict(req, context)
                    responses.append(response)
                    success_count += 1
                except Exception as e:
                    logger.warning(f"⚠️ Failed to process a request in batch: {e}")
                    failed_count += 1
                    # Add empty response for failed request
                    responses.append(ml_service_pb2.PredictResponse())

            return ml_service_pb2.BatchPredictResponse(
                responses=responses,
                success_count=success_count,
                failed_count=failed_count
            )

        except Exception as e:
            logger.error(f"❌ Error in batch prediction: {e}", exc_info=True)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f'Internal error: {str(e)}')
            return ml_service_pb2.BatchPredictResponse()

    def HealthCheck(self, request, context):
        """Health check for services"""
        try:
            service_name = request.service_name.lower()

            if service_name == "all":
                # Check all services
                if self.model_registry.list_services():
                    status = ml_service_pb2.HealthCheckResponse.SERVING
                    message = f"All services healthy. Available: {', '.join(self.model_registry.list_services())}"
                else:
                    status = ml_service_pb2.HealthCheckResponse.NOT_SERVING
                    message = "No services available"

                return ml_service_pb2.HealthCheckResponse(
                    status=status,
                    model_name="all",
                    model_version="mixed",
                    uptime_seconds=self.model_registry.get_uptime(),
                    message=message
                )

            elif service_name == "clip":
                clip_service = self.model_registry.get_service('clip')
                if clip_service:
                    return clip_service.health_check()
                else:
                    return ml_service_pb2.HealthCheckResponse(
                        status=ml_service_pb2.HealthCheckResponse.NOT_SERVING,
                        model_name="clip",
                        model_version="unknown",
                        uptime_seconds=self.model_registry.get_uptime(),
                        message="CLIP service not available"
                    )

            else:
                return ml_service_pb2.HealthCheckResponse(
                    status=ml_service_pb2.HealthCheckResponse.UNKNOWN,
                    model_name=service_name,
                    model_version="unknown",
                    uptime_seconds=self.model_registry.get_uptime(),
                    message=f"Unknown service: {service_name}"
                )

        except Exception as e:
            logger.error(f"❌ Error in health check: {e}", exc_info=True)
            return ml_service_pb2.HealthCheckResponse(
                status=ml_service_pb2.HealthCheckResponse.SERVICE_SPECIFIC_ERROR,
                model_name=request.service_name,
                model_version="unknown",
                uptime_seconds=self.model_registry.get_uptime(),
                message=f"Health check error: {str(e)}"
            )


def serve(port: int = 50051, max_workers: int = 10):
    """Start the gRPC server"""
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=max_workers))

    # Add the prediction service
    ml_service_pb2_grpc.add_PredictionServiceServicer_to_server(
        PredictionServiceServicer(), server
    )

    # Configure server address
    listen_addr = f'[::]:{port}'
    server.add_insecure_port(listen_addr)

    # Start server
    server.start()
    logger.info(f"🚀 gRPC ML Prediction Server started on {listen_addr}")
    logger.info("📡 Available services:")
    logger.info("  ➡️  ProcessImageForCLIP: CLIP image processing")
    logger.info("  ➡️  GetTextEmbeddingForCLIP: CLIP text embedding")
    logger.info("  ➡️  Predict: Generic prediction interface")
    logger.info("  ➡️  BatchPredict: Batch prediction interface")
    logger.info("  ➡️  HealthCheck: Service health monitoring")

    try:
        server.wait_for_termination()
    except KeyboardInterrupt:
        logger.info("🌙 Shutting down server...")
        server.stop(0)


if __name__ == '__main__':
    import argparse

    parser = argparse.ArgumentParser(description='ML Prediction gRPC Server')
    parser.add_argument('--port', type=int, default=50051, help='Server port')
    parser.add_argument('--workers', type=int, default=10, help='Max worker threads')

    args = parser.parse_args()

    print("=" * 60)
    print("🚀 Lumen gRPC Server")
    print("=" * 60)

    try:
        serve(port=args.port, max_workers=args.workers)
    except ImportError as e:
        # Use basicConfig for logging here since our setup might have failed
        logging.basicConfig()
        logging.critical(f"❌ Import error: {e}. A required package might be missing.", exc_info=True)
        logging.error("Please ensure all dependencies are installed (e.g., 'uv pip install .[cpu]').")
        sys.exit(1)
    except Exception as e:
        logging.basicConfig()
        logging.critical(f"Failed to start server: {e}", exc_info=True)
        sys.exit(1)
