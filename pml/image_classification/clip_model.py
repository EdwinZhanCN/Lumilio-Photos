import torch
import json
import time
import logging
from PIL import Image
import io
import mobileclip
import numpy as np
from typing import Dict, Any, Optional, List, Tuple

# Configure logging
logger = logging.getLogger(__name__)


class CLIPModelManager:
    """Manages CLIP model loading and inference for image classification"""

    def __init__(
        self,
        model_path: str = './pt/mobileclip_s1.pt',
        imagenet_classes_path: str = './imagenet_class_index.json'
    ):
        self.model: Optional[Any] = None
        self.preprocess: Optional[Any] = None
        self.tokenizer: Optional[Any] = None
        self.imagenet_labels: Optional[Dict[int, Dict[str, str]]] = None
        self.text_descriptions: Optional[List[str]] = None
        self.model_version: str = 'mobileclip_s1'
        self.model_path: str = model_path
        self.imagenet_classes_path: str = imagenet_classes_path
        self.load_time: Optional[float] = None
        self.is_loaded: bool = False

    def initialize(self):
        """Initialize the CLIP model and ImageNet classes"""
        try:
            self._load_model()
            self._load_imagenet_classes()
            self.is_loaded = True
            logger.info("CLIP model initialized successfully")
        except Exception as e:
            logger.error(f"Failed to initialize CLIP model: {e}")
            raise

    def _load_model(self):
        """Load the CLIP model and preprocessing functions"""
        try:
            start_time = time.time()
            logger.info(f"Loading CLIP model from {self.model_path}")

            self.model, _, self.preprocess = mobileclip.create_model_and_transforms(
                'mobileclip_s1',
                pretrained=self.model_path
            )
            self.tokenizer = mobileclip.get_tokenizer('mobileclip_s1')

            # Set model to evaluation mode
            self.model.eval()

            self.load_time = time.time()
            logger.info(f"CLIP model loaded successfully in {self.load_time - start_time:.2f} seconds")

        except Exception as e:
            logger.error(f"Failed to load CLIP model: {e}")
            raise

    def _load_imagenet_classes(self):
        """Load ImageNet class mappings"""
        try:
            with open(self.imagenet_classes_path) as f:
                imagenet_class_dict = json.load(f)
                self.text_descriptions = [
                    f"a photo of a {class_info[1]}"
                    for class_info in imagenet_class_dict.values()
                ]
                self.imagenet_labels = {
                    int(idx): {"id": class_info[0], "en": class_info[1]}
                    for idx, class_info in imagenet_class_dict.items()
                }
            logger.info(f"Loaded {len(self.imagenet_labels)} ImageNet classes")
        except Exception as e:
            logger.warning(f"Failed to load ImageNet classes: {e}")
            self.imagenet_labels = {}
            self.text_descriptions = []

    def encode_image(self, image_bytes: bytes) -> np.ndarray:
        """Encode image to feature vector"""
        if not self.is_loaded:
            raise RuntimeError("Model not initialized. Call initialize() first.")

        try:
            # Convert bytes to PIL Image
            image = Image.open(io.BytesIO(image_bytes)).convert('RGB')

            # Preprocess image
            image_tensor = self.preprocess(image).unsqueeze(0)

            # Encode with model
            with torch.no_grad():
                image_features = self.model.encode_image(image_tensor)
                image_features = image_features / image_features.norm(dim=-1, keepdim=True)

            return image_features.cpu().numpy().flatten()

        except Exception as e:
            logger.error(f"Error encoding image: {e}")
            raise

    def encode_text(self, text: str) -> np.ndarray:
        """Encode text to feature vector"""
        if not self.is_loaded:
            raise RuntimeError("Model not initialized. Call initialize() first.")

        try:
            text_tokens = self.tokenizer([text])

            with torch.no_grad():
                text_features = self.model.encode_text(text_tokens)
                text_features = text_features / text_features.norm(dim=-1, keepdim=True)

            return text_features.cpu().numpy().flatten()

        except Exception as e:
            logger.error(f"Error encoding text: {e}")
            raise

    def classify_image_with_labels(self, image_bytes: bytes, target_labels: Optional[List[str]] = None, top_k: int = 3) -> List[Tuple[str, float]]:
        """
        Classify image with optional target labels.
        Returns a list of (label, score) tuples for the top_k predictions.
        """
        if not self.is_loaded:
            raise RuntimeError("Model not initialized. Call initialize() first.")

        try:
            # Get image features
            image = Image.open(io.BytesIO(image_bytes)).convert('RGB')
            image_tensor = self.preprocess(image).unsqueeze(0)

            with torch.no_grad():
                image_features = self.model.encode_image(image_tensor)

                # Use target labels if provided, otherwise use ImageNet classes
                if target_labels:
                    text_descriptions = [f"a photo of a {label}" for label in target_labels]
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
                    batch_text = self.tokenizer(text_descriptions[i:i+batch_size])
                    text_features = self.model.encode_text(batch_text)
                    text_features = text_features / text_features.norm(dim=-1, keepdim=True)
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

                return results

        except Exception as e:
            logger.error(f"Error in image classification: {e}")
            raise

    def compute_similarity(self, image_bytes: bytes, text: str) -> float:
        """Compute similarity between image and text"""
        if not self.is_loaded:
            raise RuntimeError("Model not initialized. Call initialize() first.")

        try:
            image_features = self.encode_image(image_bytes)
            text_features = self.encode_text(text)

            # Compute cosine similarity
            similarity = np.dot(image_features, text_features)
            return float(similarity)

        except Exception as e:
            logger.error(f"Error computing similarity: {e}")
            raise

    def get_model_info(self) -> Dict[str, Any]:
        """Get model information"""
        return {
            "model_version": self.model_version,
            "model_path": self.model_path,
            "is_loaded": self.is_loaded,
            "load_time": self.load_time,
            "imagenet_classes_count": len(self.imagenet_labels) if self.imagenet_labels else 0
        }
