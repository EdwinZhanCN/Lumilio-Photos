# Qwen3 InfoNCE LoRA Fine-Tuning for Episodic Retrieval

This note gives a complete notebook-style training recipe for LoRA fine-tuning a Qwen3 embedding model on the `baseline-v7` episodic retrieval benchmark.

The design follows the core contrastive objective described in *Qwen3 Embedding: Advancing Text Embedding and Reranking Through Foundation Models*, but is deliberately reduced to a project-scale setup:

- one query anchor
- one positive target episode
- multiple same-scenario hard negatives
- cosine similarity
- temperature-scaled InfoNCE
- LoRA instead of full-parameter training
- local validation with `Recall@1`, `Recall@5`, and `MRR@10`

## 1. Goal

The benchmark already showed the main weakness of `qwen3-embedding:0.6b`:

- `Recall@5 = 1.0`
- but `Recall@1` is much lower

This means the model usually retrieves the correct semantic neighborhood, but often fails to rank the exact target episode at the top under minimal-difference hard negatives.

So the training target is not "learn episodic retrieval from scratch". It is:

> improve exact top-1 episode discrimination under hard negative competition.

## 2. Training Objective

For each query \(q_i\), we construct:

- one positive episode \(d_i^+\)
- \(K\) hard negatives \(d_{i,1}^-, \dots, d_{i,K}^-\)

After encoding and L2-normalizing all texts, we use cosine similarity and optimize:

\[
\mathrm{sim}(x, y) = \frac{x^\top y}{\|x\| \|y\|}
\]

\[
L = - \frac{1}{N} \sum_{i=1}^{N}
\log
\frac{\exp(\mathrm{sim}(q_i, d_i^+)/\tau)}
{\sum_{j=1}^{N} \exp(\mathrm{sim}(q_i, d_j^+)/\tau)
+ \sum_{i'=1}^{N}\sum_{k=1}^{K} \exp(\mathrm{sim}(q_i, d_{i',k}^-)/\tau)}
\]

This is a practical InfoNCE variant:

- other positives in the batch act as in-batch negatives
- explicit hard negatives are appended to the candidate pool
- the target label for query \(i\) is the \(i\)-th positive document

This is aligned with the Qwen3 report in spirit:

- cosine similarity
- temperature scaling
- hard negatives
- in-batch negatives

but simplified to a notebook-friendly implementation.

## 3. Why This Setup Fits `baseline-v7`

Your benchmark is not mainly testing broad scenario recognition. It is testing:

- exact episode recovery
- under same-scenario ambiguity
- with small slot-level differences such as thresholds, camera models, time windows, and rating boundaries

That is exactly the setting where:

- InfoNCE is more appropriate than plain triplet loss
- same-scenario hard negatives are more useful than random negatives

## 4. Notebook

### Cell 1: Install Dependencies

```bash
!pip install -q transformers peft accelerate sentencepiece scikit-learn
```

### Cell 2: Imports and Global Config

```python
import json
import math
import os
import random
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import numpy as np
import torch
import torch.nn as nn
import torch.nn.functional as F
from peft import LoraConfig, TaskType, get_peft_model
from torch.utils.data import DataLoader, Dataset
from transformers import AutoModel, AutoTokenizer, get_linear_schedule_with_warmup


SEED = 42
random.seed(SEED)
np.random.seed(SEED)
torch.manual_seed(SEED)
torch.cuda.manual_seed_all(SEED)

DEVICE = "cuda" if torch.cuda.is_available() else "cpu"
DTYPE = torch.bfloat16 if torch.cuda.is_available() and torch.cuda.is_bf16_supported() else torch.float32

ROOT = Path("/content")
if not (ROOT / "train.bundle.json").exists():
    ROOT = Path("/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/data/splits/baseline-v7")

TRAIN_BUNDLE_PATH = ROOT / "train.bundle.json"
VAL_BUNDLE_PATH = ROOT / "val.bundle.json"
TEST_BUNDLE_PATH = ROOT / "test.bundle.json"

MODEL_NAME = "Qwen/Qwen3-Embedding-0.6B"
OUTPUT_DIR = Path("./qwen3-episodic-lora")

NUM_HARD_NEGATIVES = 4
TRAIN_BATCH_SIZE = 8
EVAL_BATCH_SIZE = 16
GRAD_ACCUM_STEPS = 2
NUM_EPOCHS = 8
LEARNING_RATE = 2e-4
WEIGHT_DECAY = 0.01
WARMUP_RATIO = 0.1
TEMPERATURE = 0.05
MAX_LENGTH = 512
QUERY_FORMAT = "instruction"  # "instruction" follows Qwen3; "raw" matches the current benchmark CLI.
INSTRUCTION = (
    "Given a retrieval query, retrieve the exact prior agent episode that matches "
    "all relevant constraints, entities, and slot values."
)

print("DEVICE:", DEVICE)
print("DTYPE:", DTYPE)
print("TRAIN_BUNDLE_PATH:", TRAIN_BUNDLE_PATH)
print("VAL_BUNDLE_PATH:", VAL_BUNDLE_PATH)
print("TEST_BUNDLE_PATH:", TEST_BUNDLE_PATH)
```

### Cell 3: Load Bundles

```python
def load_bundle(path: Path) -> dict[str, Any]:
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


train_bundle = load_bundle(TRAIN_BUNDLE_PATH)
val_bundle = load_bundle(VAL_BUNDLE_PATH)
test_bundle = load_bundle(TEST_BUNDLE_PATH)

print("train episodes:", len(train_bundle["episodes"]))
print("train queries:", len(train_bundle["queries"]))
print("val episodes:", len(val_bundle["episodes"]))
print("val queries:", len(val_bundle["queries"]))
print("test episodes:", len(test_bundle["episodes"]))
print("test queries:", len(test_bundle["queries"]))
```

### Cell 4: Episode Text Rendering

This mirrors the Go implementation in `Episode.BuildRetrievalText()`. The production retrieval text intentionally excludes status, tags, write trigger, cluster id, tool outputs, and raw metadata.

```python
def render_entities(entities: list[dict[str, Any]]) -> str:
    parts = []
    for entity in entities:
        entity_name = str(entity.get("name", "")).strip()
        if not entity_name:
            continue
        entity_type = str(entity.get("type", "")).strip()
        if entity_type:
            parts.append(f"{entity_type}={entity_name}")
        else:
            parts.append(entity_name)
    return "\n".join(parts)


def render_steps(steps: list[dict[str, Any]]) -> str:
    parts = []
    for step in steps:
        tool_name = str(step.get("tool_name", "")).strip()
        if tool_name:
            parts.append(tool_name)
    return " -> ".join(parts)


def build_episode_text(episode: dict[str, Any]) -> str:
    sections = []

    what_parts = []
    scenario = str(episode.get("scenario", "")).strip()
    intent = str(episode.get("intent", "")).strip()
    summary = str(episode.get("summary", "")).strip()
    if scenario:
        what_parts.append(f"scenario={scenario}")
    if intent:
        what_parts.append(f"intent={intent}")
    if summary:
        what_parts.append(f"summary={summary}")
    if what_parts:
        sections.append("what:\n" + "\n".join(what_parts))

    goal = str(episode.get("goal", "")).strip()
    if goal:
        sections.append("goal:\n" + goal)

    entities = render_entities(episode.get("entities", []))
    if entities:
        sections.append("task_content:\n" + entities)

    procedure = render_steps(episode.get("steps", []))
    if procedure:
        sections.append("procedure:\n" + procedure)

    return "\n\n".join(sections)


def build_query_text(
    query: str,
    instruction: str = INSTRUCTION,
    query_format: str = QUERY_FORMAT,
) -> str:
    query = query.strip()
    if query_format == "raw":
        return query
    if query_format == "instruction":
        return f"{instruction}\nQuery: {query}"
    raise ValueError(f"unsupported query_format: {query_format}")
```

### Cell 5: Build Train/Val Lookup Tables

```python
def index_episodes(bundle: dict[str, Any]) -> dict[str, dict[str, Any]]:
    return {episode["episode_id"]: episode for episode in bundle["episodes"]}


train_episode_by_id = index_episodes(train_bundle)
val_episode_by_id = index_episodes(val_bundle)


def group_episodes(bundle: dict[str, Any]) -> dict[tuple[str, str], list[dict[str, Any]]]:
    grouped: dict[tuple[str, str], list[dict[str, Any]]] = {}
    for episode in bundle["episodes"]:
        key = (episode["scenario"], episode["intent"])
        grouped.setdefault(key, []).append(episode)
    return grouped


train_grouped = group_episodes(train_bundle)
```

### Cell 6: Hard Negative Sampler

```python
def sample_hard_negatives(
    query_item: dict[str, Any],
    episode_by_id: dict[str, dict[str, Any]],
    grouped: dict[tuple[str, str], list[dict[str, Any]]],
    all_episodes: list[dict[str, Any]],
    num_negatives: int,
) -> list[dict[str, Any]]:
    target_id = query_item["target_episode_ids"][0]
    target_scenario = query_item["target_scenario"]
    target_intent = query_item["target_intent"]

    same_group = [
        ep
        for ep in grouped.get((target_scenario, target_intent), [])
        if ep["episode_id"] != target_id
    ]

    same_scenario_other_intent = [
        ep
        for ep in all_episodes
        if ep["episode_id"] != target_id
        and ep["scenario"] == target_scenario
        and ep["intent"] != target_intent
    ]

    global_pool = [
        ep for ep in all_episodes if ep["episode_id"] != target_id
    ]

    random.shuffle(same_group)
    random.shuffle(same_scenario_other_intent)
    random.shuffle(global_pool)

    chosen: list[dict[str, Any]] = []
    seen_ids = set()

    for pool in (same_group, same_scenario_other_intent, global_pool):
        for ep in pool:
            if ep["episode_id"] in seen_ids:
                continue
            chosen.append(ep)
            seen_ids.add(ep["episode_id"])
            if len(chosen) == num_negatives:
                return chosen

    if len(chosen) < num_negatives:
        raise ValueError(
            f"not enough negative episodes for target {target_id}: "
            f"wanted {num_negatives}, got {len(chosen)}"
        )

    return chosen
```

### Cell 7: Dataset

```python
class EpisodicContrastiveDataset(Dataset):
    def __init__(
        self,
        bundle: dict[str, Any],
        episode_by_id: dict[str, dict[str, Any]],
        grouped: dict[tuple[str, str], list[dict[str, Any]]],
        num_negatives: int,
        instruction: str,
    ) -> None:
        self.queries = bundle["queries"]
        self.episodes = bundle["episodes"]
        self.episode_by_id = episode_by_id
        self.grouped = grouped
        self.num_negatives = num_negatives
        self.instruction = instruction

    def __len__(self) -> int:
        return len(self.queries)

    def __getitem__(self, idx: int) -> dict[str, Any]:
        query_item = self.queries[idx]
        target_id = query_item["target_episode_ids"][0]
        positive_episode = self.episode_by_id[target_id]
        negatives = sample_hard_negatives(
            query_item=query_item,
            episode_by_id=self.episode_by_id,
            grouped=self.grouped,
            all_episodes=self.episodes,
            num_negatives=self.num_negatives,
        )

        return {
            "query_text": build_query_text(query_item["query"], self.instruction, QUERY_FORMAT),
            "positive_text": build_episode_text(positive_episode),
            "negative_texts": [build_episode_text(ep) for ep in negatives],
            "target_episode_id": target_id,
        }
```

### Cell 8: Collator

```python
@dataclass
class EpisodicBatch:
    query_texts: list[str]
    positive_texts: list[str]
    negative_texts: list[list[str]]
    target_episode_ids: list[str]


def collate_fn(items: list[dict[str, Any]]) -> EpisodicBatch:
    return EpisodicBatch(
        query_texts=[item["query_text"] for item in items],
        positive_texts=[item["positive_text"] for item in items],
        negative_texts=[item["negative_texts"] for item in items],
        target_episode_ids=[item["target_episode_id"] for item in items],
    )
```

### Cell 9: Tokenizer and Base Model

```python
tokenizer = AutoTokenizer.from_pretrained(MODEL_NAME, trust_remote_code=True)
if tokenizer.pad_token is None:
    tokenizer.pad_token = tokenizer.eos_token
tokenizer.padding_side = "right"

base_model = AutoModel.from_pretrained(
    MODEL_NAME,
    trust_remote_code=True,
    torch_dtype=DTYPE if DEVICE == "cuda" else torch.float32,
)

base_model.config.use_cache = False
if hasattr(base_model, "gradient_checkpointing_enable"):
    base_model.gradient_checkpointing_enable()
```

### Cell 10: LoRA Adapter

```python
lora_config = LoraConfig(
    r=16,
    lora_alpha=32,
    lora_dropout=0.05,
    bias="none",
    task_type=TaskType.FEATURE_EXTRACTION,
    target_modules=[
        "q_proj",
        "k_proj",
        "v_proj",
        "o_proj",
        "gate_proj",
        "up_proj",
        "down_proj",
    ],
)

model = get_peft_model(base_model, lora_config)
model.print_trainable_parameters()
model.to(DEVICE)
```

### Cell 11: EOS Pooling Encoder

```python
class Qwen3Embedder(nn.Module):
    def __init__(self, model: nn.Module, tokenizer) -> None:
        super().__init__()
        self.model = model
        self.tokenizer = tokenizer

    def encode(self, texts: list[str], max_length: int = MAX_LENGTH) -> torch.Tensor:
        eos = self.tokenizer.eos_token or ""
        if eos:
            texts = [text if text.rstrip().endswith(eos) else text + eos for text in texts]

        batch = self.tokenizer(
            texts,
            padding=True,
            truncation=True,
            max_length=max_length,
            return_tensors="pt",
        )
        batch = {k: v.to(DEVICE) for k, v in batch.items()}

        outputs = self.model(**batch)
        hidden = outputs.last_hidden_state
        attention_mask = batch["attention_mask"]
        last_token_indices = attention_mask.sum(dim=1) - 1
        pooled = hidden[torch.arange(hidden.size(0), device=hidden.device), last_token_indices]
        return F.normalize(pooled, p=2, dim=-1)


embedder = Qwen3Embedder(model, tokenizer)
```

### Cell 12: InfoNCE Loss with In-Batch Positives and Explicit Hard Negatives

```python
def compute_infonce_loss(
    query_embeds: torch.Tensor,
    positive_embeds: torch.Tensor,
    negative_embeds: torch.Tensor,
    temperature: float = TEMPERATURE,
) -> tuple[torch.Tensor, torch.Tensor]:
    """
    query_embeds:    [B, D]
    positive_embeds: [B, D]
    negative_embeds: [B, K, D]
    """
    batch_size = query_embeds.size(0)
    neg_flat = negative_embeds.reshape(-1, negative_embeds.size(-1))

    candidate_docs = torch.cat([positive_embeds, neg_flat], dim=0)  # [B + B*K, D]
    logits = torch.matmul(query_embeds, candidate_docs.t()) / temperature
    labels = torch.arange(batch_size, device=logits.device)
    loss = F.cross_entropy(logits, labels)
    return loss, logits
```

### Cell 13: Local Retrieval Evaluation

```python
@torch.no_grad()
def encode_corpus(texts: list[str], batch_size: int = EVAL_BATCH_SIZE) -> torch.Tensor:
    all_embeddings = []
    for start in range(0, len(texts), batch_size):
        chunk = texts[start : start + batch_size]
        chunk_embeds = embedder.encode(chunk)
        all_embeddings.append(chunk_embeds.cpu())
    return torch.cat(all_embeddings, dim=0)


@torch.no_grad()
def evaluate_bundle(bundle: dict[str, Any]) -> dict[str, float]:
    embedder.model.eval()

    episode_texts = [build_episode_text(ep) for ep in bundle["episodes"]]
    episode_ids = [ep["episode_id"] for ep in bundle["episodes"]]
    episode_embeds = encode_corpus(episode_texts)
    id_to_index = {episode_id: idx for idx, episode_id in enumerate(episode_ids)}

    query_texts = [build_query_text(item["query"], INSTRUCTION, QUERY_FORMAT) for item in bundle["queries"]]
    query_embeds = encode_corpus(query_texts)

    sims = query_embeds @ episode_embeds.t()

    recall_at_1 = 0
    recall_at_5 = 0
    reciprocal_ranks = []

    for i, query_item in enumerate(bundle["queries"]):
        target_id = query_item["target_episode_ids"][0]
        target_index = id_to_index[target_id]

        ranking = torch.argsort(sims[i], descending=True).tolist()
        rank = ranking.index(target_index) + 1

        recall_at_1 += int(rank <= 1)
        recall_at_5 += int(rank <= 5)
        reciprocal_ranks.append(1.0 / rank if rank <= 10 else 0.0)

    n = len(bundle["queries"])
    return {
        "recall@1": recall_at_1 / n,
        "recall@5": recall_at_5 / n,
        "mrr@10": float(np.mean(reciprocal_ranks)),
    }
```

### Cell 14: DataLoaders

```python
train_dataset = EpisodicContrastiveDataset(
    bundle=train_bundle,
    episode_by_id=train_episode_by_id,
    grouped=train_grouped,
    num_negatives=NUM_HARD_NEGATIVES,
    instruction=INSTRUCTION,
)

train_loader = DataLoader(
    train_dataset,
    batch_size=TRAIN_BATCH_SIZE,
    shuffle=True,
    collate_fn=collate_fn,
    drop_last=False,
)

steps_per_epoch = math.ceil(len(train_loader) / GRAD_ACCUM_STEPS)
total_train_steps = steps_per_epoch * NUM_EPOCHS
warmup_steps = int(total_train_steps * WARMUP_RATIO)

optimizer = torch.optim.AdamW(
    model.parameters(),
    lr=LEARNING_RATE,
    weight_decay=WEIGHT_DECAY,
)

scheduler = get_linear_schedule_with_warmup(
    optimizer,
    num_warmup_steps=warmup_steps,
    num_training_steps=total_train_steps,
)

print("steps_per_epoch:", steps_per_epoch)
print("total_train_steps:", total_train_steps)
print("warmup_steps:", warmup_steps)
```

### Cell 15: Training Loop

```python
best_val_r1 = -1.0
best_metrics = None
global_step = 0

for epoch in range(NUM_EPOCHS):
    model.train()
    running_loss = 0.0
    optimizer.zero_grad()

    for step, batch in enumerate(train_loader, start=1):
        query_embeds = embedder.encode(batch.query_texts)
        positive_embeds = embedder.encode(batch.positive_texts)

        flat_negatives = [neg for neg_list in batch.negative_texts for neg in neg_list]
        negative_embeds = embedder.encode(flat_negatives)
        negative_embeds = negative_embeds.view(
            len(batch.query_texts),
            NUM_HARD_NEGATIVES,
            -1,
        )

        loss, _ = compute_infonce_loss(
            query_embeds=query_embeds,
            positive_embeds=positive_embeds,
            negative_embeds=negative_embeds,
            temperature=TEMPERATURE,
        )

        loss = loss / GRAD_ACCUM_STEPS
        loss.backward()
        running_loss += loss.item() * GRAD_ACCUM_STEPS

        if step % GRAD_ACCUM_STEPS == 0 or step == len(train_loader):
            torch.nn.utils.clip_grad_norm_(model.parameters(), 1.0)
            optimizer.step()
            scheduler.step()
            optimizer.zero_grad()
            global_step += 1

    val_metrics = evaluate_bundle(val_bundle)
    epoch_loss = running_loss / len(train_loader)

    print(
        f"epoch={epoch + 1} "
        f"train_loss={epoch_loss:.4f} "
        f"val_r@1={val_metrics['recall@1']:.4f} "
        f"val_r@5={val_metrics['recall@5']:.4f} "
        f"val_mrr@10={val_metrics['mrr@10']:.4f}"
    )

    if val_metrics["recall@1"] > best_val_r1:
        best_val_r1 = val_metrics["recall@1"]
        best_metrics = val_metrics
        OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
        model.save_pretrained(OUTPUT_DIR)
        tokenizer.save_pretrained(OUTPUT_DIR)
        print(f"saved best adapter to {OUTPUT_DIR}")

print("best_metrics:", best_metrics)
```

### Cell 16: Reload the Best LoRA Adapter

```python
from peft import PeftModel

base_model_for_eval = AutoModel.from_pretrained(
    MODEL_NAME,
    trust_remote_code=True,
    torch_dtype=DTYPE if DEVICE == "cuda" else torch.float32,
)
eval_model = PeftModel.from_pretrained(base_model_for_eval, OUTPUT_DIR)
eval_model.to(DEVICE)
eval_model.eval()

embedder = Qwen3Embedder(eval_model, tokenizer)
print(evaluate_bundle(val_bundle))
```

### Cell 17: Quick Retrieval Demo

```python
@torch.no_grad()
def retrieve_top_k(bundle: dict[str, Any], query: str, top_k: int = 5) -> list[tuple[str, float]]:
    episode_texts = [build_episode_text(ep) for ep in bundle["episodes"]]
    episode_ids = [ep["episode_id"] for ep in bundle["episodes"]]

    query_embed = encode_corpus([build_query_text(query, INSTRUCTION, QUERY_FORMAT)], batch_size=1)
    episode_embeds = encode_corpus(episode_texts)
    sims = (query_embed @ episode_embeds.t()).squeeze(0)
    top_indices = torch.topk(sims, k=top_k).indices.tolist()

    return [(episode_ids[i], float(sims[i])) for i in top_indices]


sample_query = val_bundle["queries"][0]["query"]
retrieve_top_k(val_bundle, sample_query, top_k=5)
```

### Cell 18: Frozen Test Evaluation

Run this only after choosing the best adapter on validation.

```python
test_metrics = evaluate_bundle(test_bundle)
print("test_metrics:", test_metrics)
```

## 5. Notes on Design Choices

### Why use `instruction + query` but not `instruction + episode`?

This follows the Qwen3 report's formulation:

- query side is instruction-aware
- document side stays as the raw retrieval object

That is a good fit here because your downstream task is:

- fixed retrieval target objects: prior episodes
- changing user requests: retrieval queries

The notebook exposes this as `QUERY_FORMAT`:

- `QUERY_FORMAT = "instruction"` follows the Qwen3 report and should be used if the LoRA-tuned model is evaluated with the same instruction prefix.
- `QUERY_FORMAT = "raw"` matches the current `benchmark-retrieval` CLI, which embeds `query_spec["query"]` directly.

Do not train with one query format and report with the other.

### Why same-scenario hard negatives?

Because your benchmark results already showed the real difficulty:

- not broad semantic access
- but exact discrimination inside the correct scenario neighborhood

So training should reflect that.

### Why EOS pooling?

Because Qwen3 Embedding uses a causal backbone and derives the final embedding from the last-layer hidden state at the end token. This notebook preserves that design.

## 6. What This Follows from the Qwen3 Report

This notebook follows the Qwen3 report on:

- contrastive InfoNCE-style training
- cosine similarity
- temperature scaling
- instruction-aware query formulation
- causal LLM embedding with end-token pooling
- hard negatives plus in-batch negatives

It does **not** attempt to reproduce:

- massive synthetic data generation
- multi-stage weak-supervision at industrial scale
- model merging / slerp
- multilingual and multi-task full recipe

That is intentional. For your project, the right question is not "can I reproduce their whole pipeline?" but:

> can I continue their contrastive embedding objective on my own minimal-difference episodic retrieval task?

This notebook is designed to answer exactly that.

## 7. Expected Next Step

Once this runs, the clean evaluation protocol is:

1. train on `baseline-v7/train.bundle.json`
2. select checkpoint on `baseline-v7/val.bundle.json`
3. freeze the best adapter
4. make the test-time query formatting match `QUERY_FORMAT`
5. evaluate the frozen adapter on `baseline-v7/test.bundle.json` with the same local evaluator
6. optionally build a Hugging Face / LoRA embedder path for seeding Qdrant
7. compare against the current untuned Qwen3 baseline

The current Go `seed-spec-bundle` command embeds through Ollama and cannot directly load this Hugging Face LoRA adapter. For a formal Qdrant-based report, add a separate embedding path that loads:

- the base Hugging Face Qwen3 embedding model
- the saved LoRA adapter
- the same episode text renderer
- the same `QUERY_FORMAT`

If the tuning works, the improvement should appear mainly in:

- `Recall@1`
- same-scenario hard negatives
- numeric-slot disambiguation

not necessarily in `Recall@5`, which is already saturated.
