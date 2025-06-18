import grpc
import time
import logging
import sys
import os
from typing import Optional

# Add parent directory to path for proto imports
sys.path.append(os.path.dirname(os.path.dirname(__file__)))

from proto import ml_service_pb2, ml_service_pb2_grpc
from .clip_model import CLIPModelManager

# Configure logging
logger = logging.getLogger(__name__)


class CLIPService:
    """CLIP-specific service implementation"""

    def __init__(self, model_path: str = './pt/mobileclip_s1.pt', imagenet_classes_path: str = './imagenet_class_index.json'):
        self.clip_model = CLIPModelManager(model_path, imagenet_classes_path)
        self.start_time = time.time()
        self.is_initialized = False

    def initialize(self):
        """Initialize the CLIP model"""
        try:
            self.clip_model.initialize()
            self.is_initialized = True
            logger.info("CLIP service initialized successfully")
        except Exception as e:
            logger.error(f"Failed to initialize CLIP service: {e}")
            raise

    def process_image_for_clip(self, request: ml_service_pb2.ImageProcessRequest, context) -> ml_service_pb2.ImageProcessResponse:
        """Process image with CLIP model"""
        start_time = time.time()

        try:
            if not self.is_initialized:
                context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
                context.set_details('CLIP model not initialized')
                return ml_service_pb2.ImageProcessResponse()

            # Get image features
            image_features = self.clip_model.encode_image(request.image_data)

            # Get predictions if target labels are provided or use ImageNet
            target_labels = list(request.target_labels) if request.target_labels else None

            # The model method now returns a list of (label, score) tuples for top_k
            predicted_scores_list = self.clip_model.classify_image_with_labels(
                request.image_data, target_labels, top_k=3
            )

            # Create LabelScore messages for the response
            label_scores = [
                ml_service_pb2.LabelScore(label=label, similarity_score=score)
                for label, score in predicted_scores_list
            ]

            processing_time = int((time.time() - start_time) * 1000)

            response = ml_service_pb2.ImageProcessResponse(
                image_id=request.image_id,
                image_feature_vector=image_features.tolist(),
                predicted_scores=label_scores,
                model_version=self.clip_model.model_version,
                processing_time_ms=processing_time
            )

            logger.info(f"Processed image {request.image_id} in {processing_time}ms")
            return response

        except Exception as e:
            logger.error(f"Error processing image: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f'Internal error: {str(e)}')
            return ml_service_pb2.ImageProcessResponse()

    def get_text_embedding_for_clip(self, request: ml_service_pb2.TextEmbeddingRequest, context) -> ml_service_pb2.TextEmbeddingResponse:
        """Get text embedding with CLIP model"""
        start_time = time.time()

        try:
            if not self.is_initialized:
                context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
                context.set_details('CLIP model not initialized')
                return ml_service_pb2.TextEmbeddingResponse()

            # Encode text
            text_features = self.clip_model.encode_text(request.text)
            processing_time = int((time.time() - start_time) * 1000)

            response = ml_service_pb2.TextEmbeddingResponse(
                text_feature_vector=text_features.tolist(),
                model_version=self.clip_model.model_version,
                processing_time_ms=processing_time
            )

            logger.info(f"Processed text embedding in {processing_time}ms")
            return response

        except Exception as e:
            logger.error(f"Error processing text: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f'Internal error: {str(e)}')
            return ml_service_pb2.TextEmbeddingResponse()

    def health_check(self, service_name: str = "clip") -> ml_service_pb2.HealthCheckResponse:
        """Health check for CLIP service"""
        try:
            if self.is_initialized and self.clip_model.is_loaded:
                status = ml_service_pb2.HealthCheckResponse.SERVING
                message = "CLIP model is healthy"
                model_version = self.clip_model.model_version
            else:
                status = ml_service_pb2.HealthCheckResponse.NOT_SERVING
                message = "CLIP model not available"
                model_version = "unknown"

            return ml_service_pb2.HealthCheckResponse(
                status=status,
                model_name="clip",
                model_version=model_version,
                uptime_seconds=int(time.time() - self.start_time),
                message=message
            )

        except Exception as e:
            logger.error(f"Error in CLIP health check: {e}")
            return ml_service_pb2.HealthCheckResponse(
                status=ml_service_pb2.HealthCheckResponse.SERVICE_SPECIFIC_ERROR,
                model_name="clip",
                model_version="unknown",
                uptime_seconds=int(time.time() - self.start_time),
                message=f"Health check error: {str(e)}"
            )

    def get_model_info(self):
        """Get CLIP model information"""
        return self.clip_model.get_model_info()
