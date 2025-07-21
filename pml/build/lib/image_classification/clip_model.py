import torch
import json
import time
import logging
import os
from PIL import Image
import io
import open_clip
import numpy as np
from typing import Dict, Any, Optional, List, Tuple
from importlib.resources import files

# Configure logging
logger = logging.getLogger(__name__)


class CLIPModelManager:
    """Manages OpenCLIP model loading and inference for image classification"""

    def __init__(
        self,
        model_name: str = 'ViT-B-32',
        pretrained: str = 'laion2b_s34b_b79k',
        model_path: Optional[str] = None,
        imagenet_classes_path: Optional[str] = None
    ):
        self.model: Optional[Any] = None
        self.preprocess: Optional[Any] = None
        self.tokenizer: Optional[Any] = None
        self.imagenet_labels: Optional[Dict[int, Dict[str, str]]] = None
        self.text_descriptions: Optional[List[str]] = None

        self.model_name = model_name
        self.pretrained = pretrained
        self.model_path = model_path

        self.imagenet_classes_path = imagenet_classes_path
        self.load_time: Optional[float] = None
        self.is_loaded: bool = False
        self.device: Optional[torch.device] = None

        # Debug: Log the actual path being used
        if self.imagenet_classes_path:
            logger.info(f"📖 Initializing CLIPModelManager with custom ImageNet classes: {self.imagenet_classes_path}")
        else:
            logger.info("📖 Initializing CLIPModelManager with default ImageNet classes from package resources.")

    def initialize(self):
        """Initialize the OpenCLIP model and ImageNet classes"""
        try:
            self._load_model()
            self._load_imagenet_classes()
            self.is_loaded = True
            logger.info(f"✅ OpenCLIP model '{self.model_name}' initialized successfully.")
        except Exception as e:
            logger.error(f"❌ Failed to initialize OpenCLIP model: {e}", exc_info=True)
            raise

    def _load_model(self):
        """Load the OpenCLIP model and preprocessing functions"""
        try:
            start_time = time.time()
            logger.info(f"⏳ Loading OpenCLIP model: {self.model_name}...")

            # Device selection
            self._setup_device()

            if self.device is None:
                raise RuntimeError("Device not properly initialized")

            # Load model
            self.model, _, self.preprocess = open_clip.create_model_and_transforms(
                self.model_name,
                pretrained=self.pretrained,
                device=self.device
            )

            # Get tokenizer
            self.tokenizer = open_clip.get_tokenizer(self.model_name)

            # Set model to evaluation mode
            self.model.eval()

            self.load_time = time.time() - start_time
            logger.info(f"✅ OpenCLIP model loaded in {self.load_time:.2f} seconds.")

            self._log_model_info()

        except Exception as e:
            logger.error(f"❌ Failed to load OpenCLIP model: {e}", exc_info=True)
            raise

    def _setup_device(self):
        """Setup the computation device"""
        logger.debug("--- Device Selection Debug ---")
        logger.debug(f"PyTorch version: {torch.__version__}")

        # Check for CUDA
        cuda_available = torch.cuda.is_available()
        logger.debug(f"CUDA available: {cuda_available}")

        # Check for MPS (Apple Silicon)
        mps_built = torch.backends.mps.is_built()
        mps_available = torch.backends.mps.is_available()
        logger.debug(f"MPS built: {mps_built}")
        logger.debug(f"MPS available: {mps_available}")

        if cuda_available:
            self.device = torch.device("cuda")
            logger.info("🔌 Using CUDA for model inference.")
        elif mps_available:
            self.device = torch.device("mps")
            logger.info("🍏 Using MPS for model inference on Apple Silicon.")
        else:
            self.device = torch.device("cpu")
            logger.info("🧠 Using CPU for model inference.")

        logger.debug(f"Selected device: {self.device}")
        logger.debug("-----------------------------")

    def _log_model_info(self):
        """Log detailed model information"""
        try:
            if self.model is None:
                logger.warning("⚠️ Model not loaded, cannot log detailed info.")
                return

            total_params = sum(p.numel() for p in self.model.parameters())
            trainable_params = sum(p.numel() for p in self.model.parameters() if p.requires_grad)

            logger.info(f"  - 🏷️ Model: {self.model_name}")
            logger.info(f"  - 📚 Pretrained: {self.pretrained}")
            logger.info(f"  - 💻 Device: {self.device}")
            logger.info(f"  - 🔢 Total parameters: {total_params:,}")
            logger.info(f"  - ✍️ Trainable parameters: {trainable_params:,}")
        except Exception as e:
            logger.debug(f"Could not log detailed model info: {e}")

    def _load_imagenet_classes(self):
        """Load ImageNet class mappings"""
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
        """Encode image to feature vector"""
        if not self.is_loaded:
            raise RuntimeError("Model not initialized. Call initialize() first.")

        try:
            start_time = time.time()

            # Convert bytes to PIL Image
            image = Image.open(io.BytesIO(image_bytes)).convert('RGB')

            # Preprocess image and move to device
            if self.preprocess is None or self.device is None:
                raise RuntimeError("Model preprocessing or device not properly initialized")
            image_tensor = self.preprocess(image).unsqueeze(0).to(self.device)

            # Encode with model
            with torch.no_grad():
                if self.model is None:
                    raise RuntimeError("Model not properly initialized")

                image_features = self.model.encode_image(image_tensor)
                image_features /= image_features.norm(dim=-1, keepdim=True)

            processing_time = time.time() - start_time
            logger.debug(f"🖼️ Image encoding finished in {processing_time*1000:.2f}ms.")

            result = image_features.cpu().numpy().flatten()
            return result

        except Exception as e:
            logger.error(f"❌ Error encoding image: {e}", exc_info=True)
            raise

    def encode_text(self, text: str) -> np.ndarray:
        """Encode text to feature vector"""
        if not self.is_loaded:
            raise RuntimeError("Model not initialized. Call initialize() first.")

        try:
            start_time = time.time()

            # Tokenize and move to device
            if self.tokenizer is None or self.device is None:
                raise RuntimeError("Tokenizer or device not properly initialized")
            text_tokens = self.tokenizer([text]).to(self.device)

            # Encode with model
            with torch.no_grad():
                if self.model is None:
                    raise RuntimeError("Model not properly initialized")

                text_features = self.model.encode_text(text_tokens)
                text_features /= text_features.norm(dim=-1, keepdim=True)

            processing_time = time.time() - start_time
            logger.debug(f"✍️ Text encoding finished in {processing_time*1000:.2f}ms.")

            result = text_features.cpu().numpy().flatten()
            return result

        except Exception as e:
            logger.error(f"❌ Error encoding text: {e}", exc_info=True)
            raise

    def classify_image_with_labels(self, image_bytes: bytes, target_labels: Optional[List[str]] = None, top_k: Optional[int] = 3) -> List[Tuple[str, float]]:
        """
        Classify image with optional target labels.
        Returns a list of (label, score) tuples for the top_k predictions.
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
            logger.error(f"❌ Error computing similarity: {e}", exc_info=True)
            raise

    def get_model_info(self) -> Dict[str, Any]:
        """Get comprehensive model information"""
        base_info = {
            "model_name": self.model_name,
            "pretrained": self.pretrained,
            "model_path": self.model_path,
            "is_loaded": self.is_loaded,
            "load_time": self.load_time,
            "device": str(self.device) if self.device else None,
            "imagenet_classes_count": len(self.imagenet_labels) if self.imagenet_labels else 0,
        }
        return base_info
