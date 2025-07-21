import grpc
import time
import logging
import sys
import os
from typing import Optional

# Add parent directory to path for proto imports
sys.path.append(os.path.dirname(os.path.dirname(__file__)))

from proto import ml_service_pb2
from .clip_model import CLIPModelManager

# Configure logging
logger = logging.getLogger(__name__)

# Default path will be handled by CLIPModelManager using importlib.resources

class CLIPService:
    """OpenCLIP-specific service implementation"""

    def __init__(
        self,
        model_name: str = 'ViT-B-32',
        pretrained: str = 'laion2b_s34b_b79k',
        model_path: Optional[str] = None,
        imagenet_classes_path: Optional[str] = None
    ):
        """
        Initialize CLIP service with OpenCLIP model.

        Args:
            model_name: Name of the model to use (e.g., 'ViT-B-32')
            pretrained: Name of the pretrained weights to use (e.g., 'laion2b_s34b_b79k')
            model_path: Optional path to custom model checkpoint
            imagenet_classes_path: Path to ImageNet class definitions
        """
        self.model_name = model_name
        self.pretrained = pretrained
        self.clip_model = CLIPModelManager(model_name, pretrained, model_path, imagenet_classes_path)
        self.start_time = time.time()
        self.is_initialized = False

        logger.info(f"CLIPService created with model: {model_name}")

    def initialize(self):
        """Initialize the OpenCLIP model"""
        try:
            logger.info(f"Initializing OpenCLIP service with model: {self.model_name}")
            self.clip_model.initialize()
            self.is_initialized = True

            model_info = self.clip_model.get_model_info()
            logger.info("OpenCLIP service initialized successfully:")
            logger.info(f"  - Model: {model_info['model_name']}")
            logger.info(f"  - Device: {model_info['device']}")
            logger.info(f"  - Load time: {model_info['load_time']:.2f}s")
            logger.info(f"  - ImageNet classes: {model_info['imagenet_classes_count']}")

        except Exception as e:
            logger.error(f"Failed to initialize OpenCLIP service: {e}")
            raise

    def process_image_for_clip(self, request: ml_service_pb2.ImageProcessRequest, context) -> ml_service_pb2.ImageProcessResponse:
        """Process image with OpenCLIP model"""
        start_time = time.time()

        try:
            if not self.is_initialized:
                context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
                context.set_details('OpenCLIP model not initialized')
                return ml_service_pb2.ImageProcessResponse()

            # Validate input
            if not request.image_data:
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details('No image data provided')
                return ml_service_pb2.ImageProcessResponse()

            # Get image features
            try:
                image_features = self.clip_model.encode_image(request.image_data)
            except ValueError as ve:
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details(f'Invalid image: {str(ve)}')
                return ml_service_pb2.ImageProcessResponse()

            # Get predictions if target labels are provided or use ImageNet
            target_labels = list(request.target_labels) if request.target_labels else None

            # Determine top_k from request or use default
            top_k = getattr(request, 'top_k', 3) or 3

            # The model method now returns a list of (label, score) tuples for top_k
            predicted_scores_list = self.clip_model.classify_image_with_labels(
                request.image_data, target_labels, top_k=top_k
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
                model_version=self.clip_model.model_name,
                processing_time_ms=processing_time
            )

            logger.info(f"Processed image {request.image_id} in {processing_time}ms with {len(label_scores)} predictions")

            return response

        except Exception as e:
            logger.error(f"Error processing image: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f'Internal error: {str(e)}')
            return ml_service_pb2.ImageProcessResponse()

    def get_text_embedding_for_clip(self, request: ml_service_pb2.TextEmbeddingRequest, context) -> ml_service_pb2.TextEmbeddingResponse:
        """Get text embedding with OpenCLIP model"""
        start_time = time.time()

        try:
            if not self.is_initialized:
                context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
                context.set_details('OpenCLIP model not initialized')
                return ml_service_pb2.TextEmbeddingResponse()

            # Validate input
            if not request.text or not request.text.strip():
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details('No text provided')
                return ml_service_pb2.TextEmbeddingResponse()

            # Encode text
            text_features = self.clip_model.encode_text(request.text)
            processing_time = int((time.time() - start_time) * 1000)

            response = ml_service_pb2.TextEmbeddingResponse(
                text_feature_vector=text_features.tolist(),
                model_version=self.clip_model.model_name,
                processing_time_ms=processing_time
            )

            logger.info(f"Processed text embedding in {processing_time}ms")

            return response

        except Exception as e:
            logger.error(f"Error processing text: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f'Internal error: {str(e)}')
            return ml_service_pb2.TextEmbeddingResponse()

    def compute_similarity_for_clip(self, image_bytes: bytes, text: str) -> float:
        """Compute similarity between image and text using OpenCLIP"""
        try:
            if not self.is_initialized:
                raise RuntimeError("OpenCLIP model not initialized")

            return self.clip_model.compute_similarity(image_bytes, text)

        except Exception as e:
            logger.error(f"Error computing similarity: {e}")
            raise

    def health_check(self, service_name: str = "openclip") -> ml_service_pb2.HealthCheckResponse:
        """Health check for OpenCLIP service"""
        try:
            if self.is_initialized and self.clip_model.is_loaded:
                status = ml_service_pb2.HealthCheckResponse.SERVING
                message = f"OpenCLIP model ({self.model_name}) is healthy"
                model_version = self.clip_model.model_name
            else:
                status = ml_service_pb2.HealthCheckResponse.NOT_SERVING
                message = f"OpenCLIP model ({self.model_name}) not available"
                model_version = "unknown"

            return ml_service_pb2.HealthCheckResponse(
                status=status,
                model_name=f"openclip-{self.model_name}",
                model_version=model_version,
                uptime_seconds=int(time.time() - self.start_time),
                message=message
            )

        except Exception as e:
            logger.error(f"Error in OpenCLIP health check: {e}")
            return ml_service_pb2.HealthCheckResponse(
                status=ml_service_pb2.HealthCheckResponse.SERVICE_SPECIFIC_ERROR,
                model_name=f"openclip-{self.model_name}",
                model_version="unknown",
                uptime_seconds=int(time.time() - self.start_time),
                message=f"Health check error: {str(e)}"
            )

    def get_model_info(self):
        """Get comprehensive OpenCLIP model information"""
        base_info = self.clip_model.get_model_info()

        # Add service-specific information
        service_info = {
            "service_type": "OpenCLIP",
            "current_model": self.model_name,
            "service_uptime": time.time() - self.start_time,
            "initialization_status": self.is_initialized
        }

        return {**base_info, **service_info}

    def switch_model(self, new_model_name: str, new_pretrained: str, model_path: Optional[str] = None):
        """
        Switch to a different OpenCLIP model.
        Note: This will reinitialize the service and may take time.
        """
        old_model = self.clip_model
        try:
            logger.info(f"Switching from {self.model_name} to {new_model_name}")

            # Create new model manager
            self.model_name = new_model_name
            self.pretrained = new_pretrained
            self.clip_model = CLIPModelManager(
                new_model_name,
                new_pretrained,
                model_path,
                old_model.imagenet_classes_path if old_model else self.clip_model.imagenet_classes_path
            )

            # Mark as uninitialized
            self.is_initialized = False

            # Initialize new model
            self.initialize()

            logger.info(f"Successfully switched to model: {new_model_name}")
            return True

        except Exception as e:
            logger.error(f"Failed to switch model to {new_model_name}: {e}")
            # Try to restore old model if possible
            try:
                if old_model:
                    self.clip_model = old_model
                    self.model_name = getattr(old_model, 'model_name', 'ViT-B-32')
                    self.pretrained = getattr(old_model, 'pretrained', 'laion2b_s34b_b79k')
                    logger.info("Restored previous model after failed switch")
            except:
                logger.error("Failed to restore previous model")
                self.is_initialized = False
            raise

    def get_performance_stats(self):
        """Get performance statistics for the service"""
        try:
            model_info = self.get_model_info()

            stats = {
                "model": self.model_name,
                "uptime_seconds": time.time() - self.start_time,
                "initialization_time": model_info.get('load_time', 0),
                "device": model_info.get('device', 'unknown'),
                "is_healthy": self.is_initialized and self.clip_model.is_loaded,
            }

            return stats

        except Exception as e:
            logger.error(f"Error getting performance stats: {e}")
            return {"error": str(e)}
