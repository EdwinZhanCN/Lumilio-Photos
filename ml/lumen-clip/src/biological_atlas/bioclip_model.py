from __future__ import annotations
import io
import os
import json
import time
from typing import List, Dict, Any

import numpy as np
import torch
from PIL import Image
import open_clip
from huggingface_hub import hf_hub_download
from typing import Tuple


class BioCLIPModelManager:
    """
    Simplified BioCLIP-2 manager using OpenCLIP and TreeOfLife-10M labels.
    Provides model loading, text embedding computation/caching, and image classification.
    """

    def __init__(
        self,
        model: str = "hf-hub:imageomics/bioclip-2",
        text_repo_id: str = "imageomics/TreeOfLife-10M",
        remote_names_path: str = "embeddings/txt_emb_species.json",  # Path in the HF repo
        batch_size: int = 512,
    ) -> None:
        # Fixed BioCLIP2 model version
        self.model_version = "bioclip2"
        self.model_id = model
        self.text_repo_id = text_repo_id
        self.remote_names_path = remote_names_path  # Keep track of the remote path
        self.batch_size = batch_size

        # Use absolute paths based on the location of this script
        base_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), "../.."))
        data_dir = os.path.join(base_dir, "data", "bioclip")
        os.makedirs(data_dir, exist_ok=True)

        # Local filenames for storing data
        self.names_filename = os.path.join(data_dir, "txt_emb_species.json")
        self.vectors_filename = os.path.join(data_dir, "text_vectors.npz")

        self.device = self._choose_device()
        self._model: torch.nn.Module | None = None
        self._preprocess = None
        self._tokenizer = None
        self.is_initialized = False

        self.labels: List[str] = []
        self.text_embeddings: torch.Tensor | None = None
        self._load_time: float | None = None

    @staticmethod
    def _choose_device() -> torch.device:
        if torch.cuda.is_available():
            return torch.device("cuda")
        if torch.backends.mps.is_available():
            return torch.device("mps")
        return torch.device("cpu")

    def initialize(self) -> None:
        if self.is_initialized:
            return
        t0 = time.time()
        model, _, preprocess = open_clip.create_model_and_transforms(self.model_id)
        tokenizer = open_clip.get_tokenizer(self.model_id)
        model.eval().to(self.device)
        self._model = model
        self._preprocess = preprocess
        self._tokenizer = tokenizer
        self.is_initialized = True
        self._load_time = time.time() - t0

        # Load labels and embeddings
        self._load_label_names()
        self._load_or_compute_text_embeddings()

    def _load_label_names(self) -> None:
        """
        Download and cache TreeOfLife-10M label names as a JSON list.
        Ensures the local directory for the JSON exists before moving.
        """
        # Create parent dir if needed
        dirpath = os.path.dirname(self.names_filename)
        if not os.path.exists(dirpath):
            os.makedirs(dirpath, exist_ok=True)

        # Download if missing
        if not os.path.exists(self.names_filename):
            try:
                path = hf_hub_download(
                    repo_id=self.text_repo_id,
                    repo_type="dataset",
                    filename=self.remote_names_path,
                )
                import shutil
                shutil.copy(path, self.names_filename)
            except Exception as e:
                raise RuntimeError(f"Failed to download label names: {e}")

        # Load label names
        with open(self.names_filename, 'r') as f:
            self.labels = json.load(f)

    def _compute_and_cache_text_embeddings(self) -> None:
        assert self._model and self._tokenizer
        all_vecs: List[np.ndarray] = []
        for i in range(0, len(self.labels), self.batch_size):
            batch = self.labels[i : i + self.batch_size]
            prompts = [f"a photo of {name}" for name in batch]
            tokens = self._tokenizer(prompts).to(self.device)  # type: ignore[arg-type]
            with torch.no_grad():
                feats = self._unit_normalize(self._model.encode_text(tokens))  # type: ignore[attr-defined]
            all_vecs.append(feats.cpu().numpy())
        vecs = np.vstack(all_vecs).astype(np.float32)
        np.savez(self.vectors_filename,
                 names=np.array(self.labels, dtype=object), vecs=vecs)
        self.text_embeddings = torch.tensor(vecs)

    def _load_or_compute_text_embeddings(self) -> None:
        if os.path.exists(self.vectors_filename):
            data = np.load(self.vectors_filename, allow_pickle=True)
            names = data['names'].tolist()
            vecs = data['vecs']
            if names == self.labels:
                self.text_embeddings = torch.tensor(vecs)
                return
        self._compute_and_cache_text_embeddings()



    @staticmethod
    def _unit_normalize(t: torch.Tensor) -> torch.Tensor:
        return t / t.norm(dim=-1, keepdim=True)

    def encode_image(self, image_bytes: bytes) -> np.ndarray:
        self._ensure_initialized()
        img = Image.open(io.BytesIO(image_bytes)).convert('RGB')
        tensor = self._preprocess(img).unsqueeze(0).to(self.device)  # type: ignore[arg-type]
        with torch.no_grad():
            feat = self._unit_normalize(self._model.encode_image(tensor))  # type: ignore[attr-defined]
        return feat.squeeze(0).cpu().numpy()

    @staticmethod
    def extract_scientific_name(label_data: Any) -> str:
        """
        Extract the scientific name from the complex label structure.

        Args:
            label_data: The label data from the model

        Returns:
            The scientific name as a string
        """
        if isinstance(label_data, list) and len(label_data) == 2 and isinstance(label_data[0], list):
            # Format: [['Animalia', ..., 'Genus', 'species'], 'Common Name']
            taxonomy = label_data[0]
            if len(taxonomy) >= 2:
                # Scientific name is genus + species (last two elements)
                return f"{taxonomy[-2]} {taxonomy[-1]}"
        # Fallback to string representation if we can't extract properly
        return str(label_data)

    def classify_image(
        self,
        image_bytes: bytes,
        top_k: int = 3
    ) -> List[Tuple[str, float]]:
        self._ensure_initialized()
        img_vec = torch.tensor(self.encode_image(image_bytes), device=self.device).unsqueeze(0)
        assert self.text_embeddings is not None
        text_emb = self.text_embeddings.to(self.device)
        with torch.no_grad():
            sims = (img_vec @ text_emb.T).softmax(dim=-1).squeeze(0)
            probs, idxs = sims.topk(min(top_k, sims.numel()))

        # Extract scientific names from the label data
        return [(self.extract_scientific_name(self.labels[idx]), float(probs[i]))
                for i, idx in enumerate(idxs)]

    def encode_text(self, text: str) -> np.ndarray:
        """Encodes a single string of text into a unit-normalized embedding vector."""
        self._ensure_initialized()
        tokenizer = self._tokenizer
        model = self._model
        assert tokenizer is not None
        assert model is not None

        # BioCLIP often uses a specific prompt format, but a simple one works for general embedding
        prompt = f"a photo of a {text}"
        tokens = tokenizer([prompt]).to(self.device)
        with torch.no_grad():
            feat = model.encode_text(tokens)  # type: ignore[attr-defined]
            feat = feat / feat.norm(dim=-1, keepdim=True)
        return feat.squeeze(0).cpu().numpy()

    def info(self) -> Dict[str, Any]:
        """
        Return model information including fixed version, device, and load time.
        """
        return {
            "model_version": self.model_version,
            "device": str(self.device),
            "load_time": self._load_time,
        }

    def _ensure_initialized(self) -> None:
        if not self.is_initialized:
            raise RuntimeError("Call initialize() before inference.")
