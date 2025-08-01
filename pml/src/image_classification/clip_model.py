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

        # Scene classification prompts and embeddings
        self.scene_prompts = [
            "a photo of an animal",      # all fauna
            "a photo of a bird",         # Aves
            "a photo of an insect",      # Insecta / Arthropoda
            "a photo of a human-made object",
            "a photo of a landscape",
            "an abstract painting"
        ]
        self.scene_prompt_embeddings: Optional[torch.Tensor] = None

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

        # Initialize scene classification embeddings
        self._initialize_scene_embeddings()

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

    def _initialize_scene_embeddings(self) -> None:
        """
        Internal: Initialize embeddings for scene classification prompts.
        """
        if self.model is None or self.tokenizer is None or self.device is None:
            logger.warning("Cannot initialize scene embeddings: model components not ready")
            return

        try:
            with torch.no_grad():
                tokens = self.tokenizer(self.scene_prompts).to(self.device)
                embeddings = self.model.encode_text(tokens)
                # Normalize embeddings
                embeddings = embeddings / embeddings.norm(dim=-1, keepdim=True)
                self.scene_prompt_embeddings = embeddings.cpu()
            logger.info("Initialized scene classification embeddings for %d prompts", len(self.scene_prompts))
        except Exception as e:
            logger.error("Failed to initialize scene embeddings: %s", e)
            self.scene_prompt_embeddings = None


    def _load_imagenet_classes(self):
        """
        Internal: Load ImageNet class index and build text descriptions.

        Looks first at a custom path, then package resources, then a fallback file.
        """
        try:
            if self.imagenet_classes_path:
                # Use custom path if provided
                logger.info(f"📖 Attempting to load ImageNet classes from custom path: {self.imagenet_classes_path}")
                logger.debug(f"Custom ImageNet file exists: {os.path.exists(self.imagenet_classes_path)}")
                with open(self.imagenet_classes_path) as f:
                    imagenet_class_dict = json.load(f)
            else:
                # Use importlib.resources to load from package
                logger.info("📖 Loading ImageNet classes from package resources.")
                try:
                    package_files = files('image_classification')
                    json_file = package_files / 'imagenet_class_index.json'
                    with json_file.open('r') as f:
                        imagenet_class_dict = json.load(f)
                except Exception as resource_error:
                    logger.warning(f"⚠️ Failed to load ImageNet classes from package resources: {resource_error}")
                    # Fallback to relative path for development
                    fallback_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'imagenet_class_index.json')
                    logger.info(f"↪️ Falling back to relative path for ImageNet classes: {fallback_path}")
                    with open(fallback_path) as f:
                        imagenet_class_dict = json.load(f)

            # Create text descriptions
            template = 'a photo of a {}'
            self.text_descriptions = [
                template.format(class_info[1])
                for class_info in imagenet_class_dict.values()
            ]

            self.imagenet_labels = {
                int(idx): {"id": class_info[0], "en": class_info[1]}
                for idx, class_info in imagenet_class_dict.items()
            }
            logger.info(f"✅ Loaded {len(self.imagenet_labels)} ImageNet classes.")
        except Exception as e:
            logger.warning(f"⚠️ Failed to load ImageNet classes: {e}", exc_info=True)
            self.imagenet_labels = {}
            self.text_descriptions = []

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

    def classify_image_with_labels(self, image_bytes: bytes, target_labels: Optional[List[str]] = None, top_k: Optional[int] = 3) -> List[Tuple[str, float]]:
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
        if not self.is_loaded:
            raise RuntimeError("Model not initialized. Call initialize() first.")

        if not isinstance(top_k, int) or top_k <= 0:
            top_k = 3

        try:
            start_time = time.time()

            # Get image features
            image = Image.open(io.BytesIO(image_bytes)).convert('RGB')
            if self.preprocess is None or self.device is None:
                raise RuntimeError("Model preprocessing or device not properly initialized")
            image_tensor = self.preprocess(image).unsqueeze(0).to(self.device)

            with torch.no_grad():
                # Encode image
                if self.model is None:
                    raise RuntimeError("Model not properly initialized")

                image_features = self.model.encode_image(image_tensor)
                image_features /= image_features.norm(dim=-1, keepdim=True)

                # Prepare text descriptions
                if target_labels:
                    template = 'a photo of a {}'
                    text_descriptions = [template.format(label) for label in target_labels]
                    label_mapping = {i: label for i, label in enumerate(target_labels)}
                else:
                    text_descriptions = self.text_descriptions or []
                    label_mapping = self.imagenet_labels or {}

                if not text_descriptions:
                    return []

                # Process text in batches to avoid memory issues
                batch_size = 100
                all_text_features = []

                for i in range(0, len(text_descriptions), batch_size):
                    if self.tokenizer is None or self.device is None or self.model is None:
                        raise RuntimeError("Model components not properly initialized")
                    batch_text = self.tokenizer(text_descriptions[i:i+batch_size]).to(self.device)

                    text_features = self.model.encode_text(batch_text)
                    text_features /= text_features.norm(dim=-1, keepdim=True)
                    all_text_features.append(text_features)

                # Combine all text features
                text_features = torch.cat(all_text_features, dim=0)

                # Calculate similarities
                similarities = (100.0 * image_features @ text_features.T).softmax(dim=-1)
                top_probs, top_indices = torch.topk(similarities, min(top_k, len(text_descriptions)), dim=-1)

                top_probs = top_probs.cpu().numpy()[0]
                top_indices = top_indices.cpu().numpy()[0]

                # Format results
                results = []
                for idx, prob in zip(top_indices, top_probs):
                    if target_labels:
                        label = label_mapping.get(idx, "unknown")
                    else:
                        label_info = label_mapping.get(idx, {"en": "unknown"})
                        if isinstance(label_info, dict):
                            label = label_info.get("en", "unknown")
                        else:
                            label = str(label_info)
                    results.append((label, float(prob)))

                processing_time = time.time() - start_time
                logger.debug(f"🎨 Image classification finished in {processing_time*1000:.2f}ms.")

                return results

        except Exception as e:
            logger.error(f"❌ Error in image classification: {e}", exc_info=True)
            raise

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

    def classify_scene(self, img_vec: torch.Tensor) -> Tuple[str, float]:
        """
        Classify the scene type of an image using predefined prompts.

        Args:
            img_vec: Normalized image feature vector as a torch tensor.

        Returns:
            Tuple of (scene_label, confidence_score)

        Raises:
            RuntimeError: If scene embeddings are not initialized.
        """
        if self.scene_prompt_embeddings is None:
            raise RuntimeError("Scene embeddings not initialized")

        # Ensure img_vec is on CPU and has correct shape
        if img_vec.device != torch.device('cpu'):
            img_vec = img_vec.cpu()
        if img_vec.dim() == 1:
            img_vec = img_vec.unsqueeze(0)  # Add batch dimension

        # Compute similarities
        sims = (img_vec @ self.scene_prompt_embeddings.T).softmax(-1).squeeze(0)
        idx = int(sims.argmax().item())

        return self.scene_prompts[idx], float(sims[idx])

    def is_animal_like(self, image_bytes: bytes) -> bool:
        """
        Binary filter to determine if an image contains animal/bird/insect.

        Args:
            image_bytes: Raw image data in bytes.

        Returns:
            True if the image is classified as animal, bird, or insect; False otherwise.

        Raises:
            RuntimeError: If model is not initialized.
        """
        if not self.is_loaded:
            raise RuntimeError("Model not initialized")

        # Get image features
        img_features = self.encode_image(image_bytes)
        img_tensor = torch.from_numpy(img_features).unsqueeze(0)

        # Classify scene
        scene_label, confidence = self.classify_scene(img_tensor)

        # Check if it's animal-like (first 3 prompts are animal, bird, insect)
        animal_like_scenes = self.scene_prompts[:3]
        is_animal = scene_label in animal_like_scenes

        logger.debug("Scene classification: %s (%.3f) -> Animal-like: %s",
                    scene_label, confidence, is_animal)

        return is_animal
