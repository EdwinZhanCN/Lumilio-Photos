"""
clip_service.py

Provides gRPC service implementation for OpenCLIP operations, delegating
requests to a CLIPModelManager to perform image encoding, text embedding,
classification, similarity computation, health check, and model management.
"""

import grpc
import time
import logging
import sys
import os
from typing import Optional, Dict, Any

# Ensure proto imports resolve correctly
sys.path.append(os.path.dirname(os.path.dirname(__file__)))

from proto import ml_service_pb2
from .clip_model import CLIPModelManager

logger = logging.getLogger(__name__)


class CLIPService:
    """
    gRPC service for OpenCLIP model operations.

    Wraps a CLIPModelManager instance and exposes methods for:
      - Initializing the model
      - Image processing
      - Text embedding
      - Similarity computation
      - Health checks
      - Switching models at runtime
      - Retrieving service and model metadata
    """

    def __init__(
        self,
        model_name: str = "ViT-B-32",
        pretrained: str = "laion2b_s34b_b79k",
        model_path: Optional[str] = None,
        imagenet_classes_path: Optional[str] = None
    ) -> None:
        """
        Initialize the CLIPService.

        Args:
            model_name: Name of the OpenCLIP architecture to load.
            pretrained: Identifier for pretrained weights.
            model_path: Optional path to a custom model checkpoint.
            imagenet_classes_path: Optional path to an ImageNet class index JSON.
        """
        self.model_name = model_name
        self.pretrained = pretrained
        self.clip_model = CLIPModelManager(
            model_name, pretrained, model_path, imagenet_classes_path
        )
        self.start_time = time.time()
        self.is_initialized = False

        logger.info("CLIPService created for model: %s", model_name)

    def initialize(self) -> None:
        """
        Load and initialize the OpenCLIP model and class data.

        Raises:
            RuntimeError: If model initialization fails.
        """
        logger.info("Initializing OpenCLIP model: %s", self.model_name)
        self.clip_model.initialize()
        self.is_initialized = True
        info = self.clip_model.get_model_info()
        logger.info("Model initialized: %s on device %s (load_time=%.2fs)",
                    info["model_name"], info["device"], info["load_time"])

    def process_image_for_clip(
        self,
        request: ml_service_pb2.ImageProcessRequest,
        context
    ) -> ml_service_pb2.ImageProcessResponse:
        """
        Handle an ImageProcessRequest via CLIPModelManager.

        Args:
            request: Protobuf request containing image bytes, image_id,
                     optional target_labels and top_k.
            context: gRPC context for setting status codes and details.

        Returns:
            An ImageProcessResponse with feature vector, label scores,
            model_version, and processing_time_ms set. Status codes will
            be set on context for invalid input or internal errors.
        """
        start = time.time()

        if not self.is_initialized:
            context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
            context.set_details("Model not initialized")
            return ml_service_pb2.ImageProcessResponse()

        if not request.image_data:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("No image data provided")
            return ml_service_pb2.ImageProcessResponse()

        try:
            features = self.clip_model.encode_image(request.image_data)
        except Exception as e:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details(f"Invalid image data: {e}")
            return ml_service_pb2.ImageProcessResponse()

        labels = list(request.target_labels) if request.target_labels else None
        top_k = getattr(request, "top_k", 3) or 3

        scores = self.clip_model.classify_image_with_labels(
            request.image_data, labels, top_k
        )
        label_scores = [
            ml_service_pb2.LabelScore(label=lbl, similarity_score=score)
            for lbl, score in scores
        ]

        elapsed_ms = int((time.time() - start) * 1000)
        response = ml_service_pb2.ImageProcessResponse(
            image_id=request.image_id,
            image_feature_vector=features.tolist(),
            predicted_scores=label_scores,
            model_version=self.clip_model.model_name,
            processing_time_ms=elapsed_ms
        )

        logger.info(
            "Processed image '%s' in %dms, returned %d labels",
            request.image_id, elapsed_ms, len(label_scores)
        )
        return response

    def get_text_embedding_for_clip(
        self,
        request: ml_service_pb2.TextEmbeddingRequest,
        context
    ) -> ml_service_pb2.TextEmbeddingResponse:
        """
        Handle a TextEmbeddingRequest via CLIPModelManager.

        Args:
            request: Protobuf request containing a non-empty text field.
            context: gRPC context for status codes.

        Returns:
            A TextEmbeddingResponse with text_feature_vector, model_version,
            and processing_time_ms. Status codes set for invalid input.
        """
        start = time.time()

        if not self.is_initialized:
            context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
            context.set_details("Model not initialized")
            return ml_service_pb2.TextEmbeddingResponse()

        text = request.text or ""
        if not text.strip():
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("No text provided")
            return ml_service_pb2.TextEmbeddingResponse()

        features = self.clip_model.encode_text(text)
        elapsed_ms = int((time.time() - start) * 1000)

        response = ml_service_pb2.TextEmbeddingResponse(
            text_feature_vector=features.tolist(),
            model_version=self.clip_model.model_name,
            processing_time_ms=elapsed_ms
        )
        logger.info("Processed text embedding in %dms", elapsed_ms)
        return response

    def compute_similarity_for_clip(
        self,
        image_bytes: bytes,
        text: str
    ) -> float:
        """
        Compute cosine similarity between image and text embeddings.

        Args:
            image_bytes: Raw image bytes.
            text: Input text string.

        Returns:
            Cosine similarity score as a float.

        Raises:
            RuntimeError: If the model is not initialized.
        """
        if not self.is_initialized:
            raise RuntimeError("Model not initialized")
        return self.clip_model.compute_similarity(image_bytes, text)

    def health_check(
        self,
        service_name: str = "openclip"
    ) -> ml_service_pb2.HealthCheckResponse:
        """
        Perform a health check of the CLIPService.

        Args:
            service_name: Identifier returned in the response.

        Returns:
            A HealthCheckResponse with status, model_name, model_version,
            uptime_seconds, and a descriptive message.
        """
        uptime = int(time.time() - self.start_time)
        if self.is_initialized and self.clip_model.is_loaded:
            status = ml_service_pb2.HealthCheckResponse.SERVING
            message = f"Model '{self.model_name}' is initialized and serving."
            version = self.clip_model.model_name
        else:
            status = ml_service_pb2.HealthCheckResponse.NOT_SERVING
            message = f"Model '{self.model_name}' not available."
            version = "unknown"

        return ml_service_pb2.HealthCheckResponse(
            status=status,
            model_name=service_name,
            model_version=version,
            uptime_seconds=uptime,
            message=message
        )

    def get_model_info(self) -> Dict[str, Any]:
        """
        Retrieve metadata about the current model and service.

        Returns:
            A dictionary containing model_name, pretrained weights,
            device, is_loaded, load_time, and service uptime.
        """
        info = self.clip_model.get_model_info()
        return {
            **info,
            "service_uptime": time.time() - self.start_time,
            "initialized": self.is_initialized
        }

    def switch_model(
        self,
        new_model_name: str,
        new_pretrained: str,
        model_path: Optional[str] = None
    ) -> bool:
        """
        Replace the current CLIP model with a new one at runtime.

        Args:
            new_model_name: Name of the new model architecture.
            new_pretrained: Identifier for the new pretrained weights.
            model_path: Optional checkpoint path for the new model.

        Returns:
            True if the switch succeeds; raises on failure.
        """
        old = self.clip_model
        try:
            logger.info("Switching model '%s' -> '%s'", self.model_name, new_model_name)
            self.model_name = new_model_name
            self.pretrained = new_pretrained
            self.clip_model = CLIPModelManager(
                new_model_name, new_pretrained, model_path,
                getattr(old, "imagenet_classes_path", None)
            )
            self.is_initialized = False
            self.initialize()
            logger.info("Model switch to '%s' completed", new_model_name)
            return True
        except Exception as e:
            logger.error("Model switch failed: %s", e)
            # revert on failure
            self.clip_model = old
            self.model_name = old.model_name
            self.pretrained = old.pretrained
            raise

    def get_performance_stats(self) -> Dict[str, Any]:
        """
        Gather runtime performance statistics for the service.

        Returns:
            A dictionary with model, device, uptime, load_time, and health status.
        """
        info = self.get_model_info()
        return {
            "model": self.model_name,
            "device": info.get("device"),
            "service_uptime": info.get("service_uptime"),
            "model_load_time": info.get("load_time"),
            "is_healthy": self.is_initialized and self.clip_model.is_loaded
        }
