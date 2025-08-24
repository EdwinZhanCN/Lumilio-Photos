"""
clip_model.py

Refactored CLIPModelManager to align with BioCLIP's elegant design.
- Implements a clear initialize() pattern.
- Downloads and caches ImageNet text embeddings to a local NPZ file.
- Uses cached text embeddings for fast, default classification.
- Standardizes API for encoding and classification.
"""

import io
import json
import logging
import os
import time
from typing import Any, Dict, List, Optional, Tuple

import numpy as np
import torch
from PIL import Image
from huggingface_hub import hf_hub_download
import open_clip

logger = logging.getLogger(__name__)


class CLIPModelManager:
    """
    Manages an OpenCLIP model for image classification using cached ImageNet labels.

    This class mirrors the design of the BioCLIPModelManager, providing a streamlined
    interface for loading a model, caching text embeddings for ImageNet classes,
    and performing inference.
    """

    def __init__(
        self,
        model_name: str = "ViT-B-32",
        pretrained: str = "laion2b_s34b_b79k",
        batch_size: int = 512,
    ) -> None:
        self.model_name = model_name
        self.pretrained = pretrained
        self.model_id = f"{model_name}_{pretrained}"
        self.batch_size = batch_size

        # Define local data paths for caching
        base_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), "../.."))
        data_dir = os.path.join(base_dir, "data", "clip")
        os.makedirs(data_dir, exist_ok=True)
        self.names_filename = os.path.join(data_dir, "imagenet_class_names.json")
        self.vectors_filename = os.path.join(data_dir, f"{self.model_id}_imagenet_vectors.npz")

        # Model and data components
        self.device = self._choose_device()
        self._model: Optional[torch.nn.Module] = None
        self._preprocess: Optional[Any] = None
        self._tokenizer: Optional[Any] = None
        self.is_initialized = False

        self.labels: List[str] = []
        self.text_embeddings: Optional[torch.Tensor] = None
        self._load_time: Optional[float] = None

        # Scene classification prompts and embeddings
        self.scene_prompts = [
            "a photo of an animal",
            "a photo of a bird",
            "a photo of an insect",
            "a photo of a human-made object",
            "a photo of a landscape",
            "an abstract painting",
        ]
        self.scene_prompt_embeddings: Optional[torch.Tensor] = None

    @staticmethod
    def _choose_device() -> torch.device:
        """Chooses the best available device (CUDA, MPS, or CPU)."""
        if torch.cuda.is_available():
            return torch.device("cuda")
        if torch.backends.mps.is_available():
            return torch.device("mps")
        return torch.device("cpu")

    def initialize(self) -> None:
        """
        Loads the model, downloads/caches labels, and computes/loads text embeddings.
        This method must be called before any inference.
        """
        if self.is_initialized:
            return

        t0 = time.time()
        logger.info(f"Initializing model {self.model_name} ({self.pretrained}) on {self.device}...")

        # 1. Load OpenCLIP model, preprocessor, and tokenizer
        model, _, preprocess = open_clip.create_model_and_transforms(
            self.model_name, pretrained=self.pretrained
        )
        tokenizer = open_clip.get_tokenizer(self.model_name)
        model.eval().to(self.device)
        self._model = model
        self._preprocess = preprocess
        self._tokenizer = tokenizer

        # 2. Load labels and embeddings
        self._load_label_names()
        self._load_or_compute_text_embeddings()

        # 3. Initialize scene embeddings
        self._initialize_scene_embeddings()

        self.is_initialized = True
        self._load_time = time.time() - t0
        logger.info(f"Model initialized in {self._load_time:.2f} seconds.")

    def _load_label_names(self) -> None:
        """
        Downloads and caches ImageNet class names from a canonical source.
        The file is a simple JSON list of strings.
        """
        if not os.path.exists(self.names_filename):
            logger.info("Downloading ImageNet class names...")
            try:
                # Using a known reliable source for ImageNet labels as a simple list.
                # Replace with a different repo/file if needed.
                path = hf_hub_download(
                    repo_id="huggingface/label-files",
                    repo_type="dataset",
                    filename="imagenet-1k-id2label.json",
                )
                import shutil
                shutil.copy(path, self.names_filename)
            except Exception as e:
                raise RuntimeError(f"Failed to download label names: {e}")

        with open(self.names_filename, 'r') as f:
            self.labels = json.load(f)
        logger.info(f"Loaded {len(self.labels)} ImageNet class names.")

    def _compute_and_cache_text_embeddings(self) -> None:
        """Computes text embeddings for labels and saves them to a local NPZ file."""
        self._ensure_initialized_for_computation()
        logger.info(f"Computing text embeddings for {len(self.labels)} labels...")

        all_vecs: List[np.ndarray] = []
        prompts = [f"a photo of a {name.replace('_', ' ')}" for name in self.labels]

        for i in range(0, len(prompts), self.batch_size):
            batch = prompts[i : i + self.batch_size]
            tokenizer = self._tokenizer
            model = self._model
            assert tokenizer is not None
            assert model is not None
            tokens = tokenizer(batch).to(self.device)
            with torch.no_grad():
                feats = model.encode_text(tokens)  # type: ignore[attr-defined]
                feats = feats / feats.norm(dim=-1, keepdim=True)
            all_vecs.append(feats.cpu().numpy())

        vecs = np.vstack(all_vecs).astype(np.float32)
        np.savez(self.vectors_filename, names=np.array(self.labels, dtype=object), vecs=vecs)
        self.text_embeddings = torch.tensor(vecs)
        logger.info(f"Computed and cached text embeddings at {self.vectors_filename}.")

    def _load_or_compute_text_embeddings(self) -> None:
        """Loads text embeddings from cache or computes them if cache is invalid."""
        if os.path.exists(self.vectors_filename):
            data = np.load(self.vectors_filename, allow_pickle=True)
            names = data['names'].tolist()
            if names == self.labels:
                self.text_embeddings = torch.tensor(data['vecs'])
                logger.info(f"Loaded cached text embeddings from {self.vectors_filename}.")
                return
            else:
                logger.warning("Cached label names mismatch. Recomputing embeddings.")

        self._compute_and_cache_text_embeddings()

    def _initialize_scene_embeddings(self) -> None:
        """Initialize embeddings for simple scene prompts; store on CPU."""
        self._ensure_initialized_for_computation()
        logger.info("Initializing scene classification embeddings.")
        try:
            with torch.no_grad():
                tokenizer = self._tokenizer
                model = self._model
                assert tokenizer is not None
                assert model is not None
                tokens = tokenizer(self.scene_prompts).to(self.device)
                embeddings = model.encode_text(tokens)  # type: ignore[attr-defined]
                embeddings = embeddings / embeddings.norm(dim=-1, keepdim=True)
                self.scene_prompt_embeddings = embeddings.cpu()
        except Exception as e:
            logger.error(f"Failed to initialize scene embeddings: {e}")
            self.scene_prompt_embeddings = None

    @staticmethod
    def _unit_normalize(t: torch.Tensor) -> torch.Tensor:
        """Normalizes a tensor to unit length."""
        return t / t.norm(dim=-1, keepdim=True)

    def encode_image(self, image_bytes: bytes) -> np.ndarray:
        """Encodes image bytes into a unit-normalized embedding vector."""
        self._ensure_initialized()
        img = Image.open(io.BytesIO(image_bytes)).convert('RGB')
        preprocess = self._preprocess
        model = self._model
        assert preprocess is not None
        assert model is not None
        tensor = preprocess(img).unsqueeze(0).to(self.device)
        with torch.no_grad():
            feat = model.encode_image(tensor)  # type: ignore[attr-defined]
            feat = feat / feat.norm(dim=-1, keepdim=True)
        return feat.squeeze(0).cpu().numpy()

    def encode_text(self, text: str) -> np.ndarray:
        """Encodes a single string of text into a unit-normalized embedding vector."""
        self._ensure_initialized()
        prompt = f"a photo of a {text}"
        tokenizer = self._tokenizer
        model = self._model
        assert tokenizer is not None
        assert model is not None
        tokens = tokenizer([prompt]).to(self.device)
        with torch.no_grad():
            feat = model.encode_text(tokens)  # type: ignore[attr-defined]
            feat = feat / feat.norm(dim=-1, keepdim=True)
        return feat.squeeze(0).cpu().numpy()

    def classify_image(
        self,
        image_bytes: bytes,
        top_k: int = 5
    ) -> List[Tuple[str, float]]:
        """
        Classifies an image against the cached ImageNet labels.

        Args:
            image_bytes: The image to classify, in bytes.
            top_k: The number of top results to return.

        Returns:
            A list of (label, probability) tuples.
        """
        self._ensure_initialized()
        img_vec = torch.tensor(self.encode_image(image_bytes), device=self.device).unsqueeze(0)

        if self.text_embeddings is None:
            raise RuntimeError("Text embeddings are not available.")

        text_emb = self.text_embeddings.to(self.device)
        with torch.no_grad():
            # Similarities -> Probabilities
            sims = (100.0 * img_vec @ text_emb.T).softmax(dim=-1).squeeze(0)
            probs, idxs = sims.topk(min(top_k, sims.numel()))

        return [(self.labels[idx], float(prob)) for prob, idx in zip(probs, idxs)]

    def classify_scene(self, image_bytes: bytes) -> Tuple[str, float]:
        """
        Performs a high-level scene classification on an image.

        Args:
            image_bytes: The image to classify, in bytes.

        Returns:
            A tuple of (scene_label, confidence_score).
        """
        self._ensure_initialized()
        if self.scene_prompt_embeddings is None:
            raise RuntimeError("Scene embeddings not initialized.")

        assert self.scene_prompt_embeddings is not None
        img_vec = torch.tensor(self.encode_image(image_bytes), device='cpu').unsqueeze(0)

        with torch.no_grad():
            sims = (img_vec @ self.scene_prompt_embeddings.T).softmax(-1).squeeze(0)
            confidence, idx = sims.max(dim=0)

        return self.scene_prompts[int(idx.item())], float(confidence.item())

    def info(self) -> Dict[str, Any]:
        """Returns a dictionary with information about the loaded model."""
        return {
            "model_name": self.model_name,
            "pretrained": self.pretrained,
            "device": str(self.device),
            "is_initialized": self.is_initialized,
            "load_time_seconds": self._load_time,
            "label_count": len(self.labels),
            "vectors_cache_path": self.vectors_filename,
        }

    def _ensure_initialized(self) -> None:
        """Raises a RuntimeError if the model is not initialized."""
        if not self.is_initialized:
            raise RuntimeError("Model is not initialized. Call initialize() before inference.")

    def _ensure_initialized_for_computation(self) -> None:
        """Partial check for internal methods that only need the torch model."""
        if self._model is None or self._tokenizer is None:
            raise RuntimeError("Core model components must be loaded before this operation.")
