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
        names_filename: str = "embeddings/txt_emb_species.json",
        vectors_filename: str = "text_vectors.npz",
        batch_size: int = 512,
    ) -> None:
        # Fixed BioCLIP2 model version
        self.model_version = "bioclip2"
        self.model_id = model
        self.text_repo_id = text_repo_id
        self.names_filename = names_filename
        self.vectors_filename = vectors_filename
        self.batch_size = batch_size

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
        if not os.path.exists(self.names_filename):
            path = hf_hub_download(
                repo_id=self.text_repo_id,
                filename=self.names_filename,
            )
            os.replace(path, self.names_filename)
        with open(self.names_filename, 'r') as f:
            self.labels = json.load(f)

    def _load_or_compute_text_embeddings(self) -> None:
        if os.path.exists(self.vectors_filename):
            data = np.load(self.vectors_filename, allow_pickle=True)
            names = data['names'].tolist()
            vecs = data['vecs']
            if names == self.labels:
                self.text_embeddings = torch.tensor(vecs)
                return
        self._compute_and_cache_text_embeddings()

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
        return [(self.labels[idx], float(probs[i])) for i, idx in enumerate(idxs)]

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
