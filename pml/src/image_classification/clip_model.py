"""
clip_model.py

Defines CLIPModelManager for loading an OpenCLIP model, preprocessing,
and performing inference tasks such as image encoding, text encoding,
classification, and similarity computation.
"""

import os
import io
import time
import json
import logging

from typing import Dict, Any, Optional, List, Tuple

import torch
import numpy as np
from PIL import Image
import open_clip

from importlib.resources import files

logger = logging.getLogger(__name__)


class CLIPModelManager:
    """
    Manage an OpenCLIP model and ImageNet class data for image classification.

    Attributes:
        model: Loaded OpenCLIP model.
        preprocess: Preprocessing pipeline for image inputs.
        tokenizer: Tokenizer for text inputs.
        imagenet_labels: Mapping from class index to label info.
        text_descriptions: List of text prompts for classification.
        model_name: Name of the OpenCLIP architecture.
        pretrained: Identifier for pretrained weights.
        imagenet_classes_path: Optional path to custom ImageNet JSON.
        load_time: Time taken to load the model.
        is_loaded: Whether model and classes have been initialized.
        device: Torch device used for inference.
    """

    def __init__(
        self,
        model_name: str = "ViT-B-32",
        pretrained: str = "laion2b_s34b_b79k",
        model_path: Optional[str] = None,
        imagenet_classes_path: Optional[str] = None,
    ) -> None:
        """
        Initialize CLIPModelManager with configuration parameters.

        Args:
            model_name: Architecture name to load.
            pretrained: Pretrained weight identifier.
            model_path: Optional local path to model checkpoint.
            imagenet_classes_path: Optional path to ImageNet class JSON.
        """
        self.model: Optional[Any] = None
        self.preprocess: Optional[Any] = None
        self.tokenizer: Optional[Any] = None
        self.imagenet_labels: Optional[Dict[int, Dict[str, str]]] = None
        self.text_descriptions: Optional[List[str]] = None

        self.model_name = model_name
        self.pretrained = pretrained
        self.imagenet_classes_path = imagenet_classes_path

        self.load_time: Optional[float] = None
        self.is_loaded = False
        self.device: Optional[torch.device] = None

    def initialize(self) -> None:
        """
        Load the OpenCLIP model, transforms, tokenizer, and ImageNet classes.

        Raises:
            RuntimeError: If model or classes fail to load.
        """
        start = time.time()
        self._load_model()
        self._load_imagenet_classes()
        self.is_loaded = True
        self.load_time = time.time() - start
        logger.info("Model initialized in %.2f seconds", self.load_time)

    def _load_model(self) -> None:
        """
        Internal: Create the OpenCLIP model, preprocessing pipeline, and tokenizer.
        """
        self._setup_device()
        if self.device is None:
            raise RuntimeError("No valid device for model loading")

        model, _, preprocess = open_clip.create_model_and_transforms(
            self.model_name,
            pretrained=self.pretrained,
            device=self.device,
        )
        tokenizer = open_clip.get_tokenizer(self.model_name)

        model.eval()
        self.model = model
        self.preprocess = preprocess
        self.tokenizer = tokenizer

        self._log_model_info()

    def _setup_device(self) -> None:
        """
        Internal: Choose the best available compute device (CUDA, MPS, or CPU).
        """
        if torch.cuda.is_available():
            self.device = torch.device("cuda")
        elif torch.backends.mps.is_available():
            self.device = torch.device("mps")
        else:
            self.device = torch.device("cpu")
        logger.info("Using device: %s", self.device)

    def _log_model_info(self) -> None:
        """
        Internal: Log parameter counts for the loaded model.
        """
        if self.model is None:
            logger.warning("Model not loaded; cannot log info")
            return

        total_params = sum(p.numel() for p in self.model.parameters())
        trainable = sum(p.numel() for p in self.model.parameters() if p.requires_grad)
        logger.info("Model %s: total_params=%d, trainable_params=%d",
                    self.model_name, total_params, trainable)

    def _load_imagenet_classes(self) -> None:
        """
        Internal: Load ImageNet class index and build text descriptions.

        Looks first at a custom path, then package resources, then a fallback file.
        """
        try:
            if self.imagenet_classes_path and os.path.exists(self.imagenet_classes_path):
                with open(self.imagenet_classes_path, "r") as f:
                    class_index = json.load(f)
            else:
                res = files("image_classification") / "imagenet_class_index.json"
                with res.open("r") as f:
                    class_index = json.load(f)
        except Exception as e:
            logger.warning("Failed to load ImageNet classes: %s", e)
            class_index = {}

        # Build label mapping and text prompts
        self.imagenet_labels = {
            int(idx): {"id": info[0], "en": info[1]}
            for idx, info in class_index.items()
        }
        self.text_descriptions = [
            f"a photo of a {info[1]}" for info in self.imagenet_labels.values()
        ]

        logger.info("Loaded %d ImageNet classes", len(self.imagenet_labels))

    def encode_image(self, image_bytes: bytes) -> np.ndarray:
        """
        Encode raw image bytes into a unit-normalized feature vector.

        Args:
            image_bytes: Raw image data in bytes.

        Returns:
            1D numpy array of feature values.

        Raises:
            RuntimeError: If model is not initialized.
        """
        if not self.is_loaded or self.model is None or self.preprocess is None:
            raise RuntimeError("Model not initialized")

        image = Image.open(io.BytesIO(image_bytes)).convert("RGB")
        tensor = self.preprocess(image).unsqueeze(0).to(self.device)
        with torch.no_grad():
            features = self.model.encode_image(tensor)
            features = features / features.norm(dim=-1, keepdim=True)
        return features.cpu().numpy().flatten()

    def encode_text(self, text: str) -> np.ndarray:
        """
        Encode text into a unit-normalized feature vector.

        Args:
            text: Input text string.

        Returns:
            1D numpy array of feature values.

        Raises:
            RuntimeError: If model is not initialized.
        """
        if not self.is_loaded or self.model is None or self.tokenizer is None:
            raise RuntimeError("Model not initialized")

        tokens = self.tokenizer([text]).to(self.device)
        with torch.no_grad():
            features = self.model.encode_text(tokens)
            features = features / features.norm(dim=-1, keepdim=True)
        return features.cpu().numpy().flatten()

    def classify_image_with_labels(
        self,
        image_bytes: bytes,
        target_labels: Optional[List[str]] = None,
        top_k: int = 3
    ) -> List[Tuple[str, float]]:
        """
        Classify an image against specified or default labels.

        Args:
            image_bytes: Raw image data in bytes.
            target_labels: Optional list of label names to restrict classification.
            top_k: Number of top predictions to return.

        Returns:
            A list of (label, score) tuples for the top_k predictions.

        Raises:
            RuntimeError: If model is not initialized.
        """
        if not self.is_loaded or self.model is None or self.preprocess is None:
            raise RuntimeError("Model not initialized")

        # Prepare image feature
        image = Image.open(io.BytesIO(image_bytes)).convert("RGB")
        tensor = self.preprocess(image).unsqueeze(0).to(self.device)
        with torch.no_grad():
            img_feat = self.model.encode_image(tensor)
            img_feat = img_feat / img_feat.norm(dim=-1, keepdim=True)

            # Build text feature set
            if target_labels:
                descriptions = [f"a photo of a {lbl}" for lbl in target_labels]
                label_map = {i: lbl for i, lbl in enumerate(target_labels)}
            else:
                descriptions = self.text_descriptions or []
                label_map = {i: info["en"] for i, info in self.imagenet_labels.items()}

            if not descriptions:
                return []

            all_feats = []
            batch_size = 100
            for i in range(0, len(descriptions), batch_size):
                batch = self.tokenizer(descriptions[i:i + batch_size]).to(self.device)
                feats = self.model.encode_text(batch)
                feats = feats / feats.norm(dim=-1, keepdim=True)
                all_feats.append(feats)
            text_feats = torch.cat(all_feats, dim=0)

            # Compute similarity and select top_k
            sims = (img_feat @ text_feats.T).softmax(dim=-1)
            probs, indices = torch.topk(sims, min(top_k, sims.size(-1)), dim=-1)
            probs = probs.cpu().numpy()[0]
            indices = indices.cpu().numpy()[0]

        results = []
        for idx, p in zip(indices, probs):
            label = label_map.get(idx, "unknown")
            results.append((label, float(p)))
        return results

    def compute_similarity(self, image_bytes: bytes, text: str) -> float:
        """
        Compute cosine similarity between an image and a text string.

        Args:
            image_bytes: Raw image data in bytes.
            text: Text input for comparison.

        Returns:
            Cosine similarity score.

        Raises:
            RuntimeError: If model is not initialized.
        """
        img_feat = self.encode_image(image_bytes)
        txt_feat = self.encode_text(text)
        return float(np.dot(img_feat, txt_feat))

    def get_model_info(self) -> Dict[str, Any]:
        """
        Retrieve model configuration and status information.

        Returns:
            Dictionary containing model_name, pretrained, device, load_time, and
            number of ImageNet classes loaded.
        """
        return {
            "model_name": self.model_name,
            "pretrained": self.pretrained,
            "device": str(self.device),
            "is_loaded": self.is_loaded,
            "load_time": self.load_time,
            "imagenet_class_count": len(self.imagenet_labels or {}),
        }
