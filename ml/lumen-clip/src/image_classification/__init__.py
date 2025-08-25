"""
Image Classification Package

This package contains CLIP-based image classification services and models
for the ML prediction gRPC server.

Modules:
    clip_model: CLIP model management and inference
    clip_service: gRPC service implementation for CLIP operations
"""

from .clip_model import CLIPModelManager
from .clip_service import CLIPService

__all__ = ["CLIPModelManager", "CLIPService"]
