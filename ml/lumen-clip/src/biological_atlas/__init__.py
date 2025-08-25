"""
Biological Atlas Package

This package contains CLIP-based image classification services and models
for the ML prediction gRPC server.

Modules:
    bioclip_model: BioCLIP model management and inference
    bioclip_service: gRPC service implementation for CLIP operations
"""

from .bioclip_model import BioCLIPModelManager
from .bioclip_service import BioClipService

__all__ = ["BioCLIPModelManager", "BioClipService"]
