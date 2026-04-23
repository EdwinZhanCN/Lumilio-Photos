# Decoder Embedding Reading Note

This note records a compact reading roadmap for understanding why decoder-only LLMs can be used as text embedding models, why they may underperform on some retrieval settings without additional adaptation, and which papers are most useful for supporting the argument in this project.

## Core Question

The current `baseline-v7` result suggests a pattern:

- both models achieve `Recall@5 = 1.0`
- the major gap appears at `Recall@1`
- most `Qwen3` errors are still within the correct scenario and intent

This means the main weakness is not broad semantic retrieval, but exact top-1 disambiguation among minimally different episodes.

For this reason, the most useful literature is not only generic retrieval work, but specifically papers about using decoder-only LLMs as embedding models.

## Main Takeaway

The safest interpretation is:

- decoder-only LLMs are not inherently unusable for text embeddings
- however, strong embedding performance usually requires extra adaptation
- this adaptation often includes better pooling, bidirectional attention, and contrastive training

Therefore, if a decoder-based embedding model performs worse on slot-heavy hard negatives, a reasonable explanation is not that decoder architectures are fundamentally incapable, but that the model may not be sufficiently optimized for exact embedding-based ranking.

## Recommended Reading Order

If time is limited, read in this order:

1. `SGPT`
2. `LLM2Vec`
3. `NV-Embed`
4. `Improving Text Embeddings with Large Language Models`
5. `E5`
6. `MTEB`
7. `BEIR`

## Paper 1: SGPT

**Title**: *SGPT: GPT Sentence Embeddings for Semantic Search*  
**Link**: https://arxiv.org/abs/2202.08904

### Why read it

This is one of the earliest influential papers that explicitly turns a decoder-style GPT model into a sentence embedding / semantic search model.

### What it supports in the thesis

It supports the claim that:

- decoder transformers can be repurposed for semantic search
- sentence embedding is not limited to encoder-only architectures

### What to focus on

- why the authors think decoder LLMs are underused in semantic search
- how they form sentence embeddings from a decoder model
- how they evaluate on retrieval benchmarks such as BEIR

### How to use it in writing

Useful argument:

> Prior work such as SGPT showed that decoder-based language models can be adapted for semantic search and sentence embedding, challenging the assumption that retrieval embeddings must rely only on encoder-style architectures.

## Paper 2: LLM2Vec

**Title**: *LLM2Vec: Large Language Models Are Secretly Powerful Text Encoders*  
**Link**: https://arxiv.org/abs/2404.05961

### Why read it

This is the most directly relevant paper for explaining why a decoder-only model may underperform if used as an embedding model without enough adaptation.

### What it argues

LLM2Vec proposes three major steps:

1. enable bidirectional attention
2. apply masked next token prediction
3. use contrastive learning

The paper argues that decoder-only LLMs can become strong text encoders, but typically need these changes.

### What it supports in the thesis

It supports the claim that:

- the original causal setup of decoder-only LLMs is not ideal for embedding tasks
- turning a decoder LLM into a strong text encoder often requires explicit architectural or training modification

### Why it matters for this project

This paper gives a clean explanation for the observed result:

- Qwen3 may already retrieve the correct neighborhood
- but if its embedding representation is not sufficiently adapted for exact ranking, it may fail on minimal-difference top-1 discrimination

### How to use it in writing

Useful argument:

> Recent work such as LLM2Vec suggests that decoder-only LLMs can be strong text encoders, but usually only after bidirectional attention and contrastive training are introduced. This helps explain why an off-the-shelf decoder-based embedding model may retrieve the correct neighborhood while still struggling with exact top-1 discrimination.

## Paper 3: NV-Embed

**Title**: *NV-Embed: Improved Techniques for Training LLMs as Generalist Embedding Models*  
**Link**: https://arxiv.org/abs/2405.17428

### Why read it

This paper is especially useful because it comes from a strong engineering perspective and focuses directly on training decoder-only LLMs as embedding models.

### What it argues

The paper highlights several important techniques:

- better pooling with latent attention
- removing the causal mask during contrastive training
- two-stage contrastive instruction tuning
- careful hard-negative training

### What it supports in the thesis

It supports the claim that:

- strong decoder-based embedding models do not emerge automatically from the base LLM
- instead, they benefit from retrieval-specific design decisions

### Why it matters for this project

This is very close to the logic of the current benchmark:

- hard negatives matter
- exact top-1 ordering matters
- contrastive objectives are useful for improving this behavior

### How to use it in writing

Useful argument:

> Industrial-scale embedding work such as NV-Embed further supports the view that decoder-only LLMs can become highly competitive embedding models, but only after retrieval-oriented modifications such as improved pooling, hard-negative contrastive training, and relaxation of the original causal attention pattern.

## Paper 4: Improving Text Embeddings with Large Language Models

**Title**: *Improving Text Embeddings with Large Language Models*  
**Link**: https://www.microsoft.com/en-us/research/publication/improving-text-embeddings-with-large-language-models/

### Why read it

This paper is highly relevant to the LoRA plan because it fine-tunes decoder-only LLMs for embedding using synthetic data and standard contrastive loss.

### What it supports in the thesis

It supports the claim that:

- synthetic data can be useful for text embedding training
- contrastive fine-tuning of decoder-only LLMs is practical
- strong embedding gains can be obtained without a highly complex pipeline

### Why it matters for this project

This paper is one of the best references for justifying the next experimental step:

- using train split data
- building positive / hard negative pairs or triples
- performing contrastive LoRA on a decoder embedding model

### How to use it in writing

Useful argument:

> Prior work has shown that decoder-only LLMs can be improved as embedding models through contrastive fine-tuning on synthetic or weakly supervised data, making contrastive LoRA a reasonable next step for improving hard-negative discrimination in episodic retrieval.

## Paper 5: E5

**Title**: *Text Embeddings by Weakly-Supervised Contrastive Pre-training*  
**Link**: https://arxiv.org/abs/2212.03533

### Why read it

E5 is not a decoder paper, but it is one of the most useful contrastive embedding references and provides an important comparison point.

### What it supports in the thesis

It supports the claim that:

- contrastive training is a strong foundation for retrieval embeddings
- embedding models can become highly effective through weak supervision and contrastive learning
- retrieval performance should be interpreted through established embedding methodology, not architecture labels alone

### Why it matters for this project

E5 gives the classical embedding background for the LoRA section:

- what positive/negative structure should look like
- why contrastive learning is an appropriate training objective

### How to use it in writing

Useful argument:

> Following strong embedding baselines such as E5, contrastive learning remains one of the most effective paradigms for improving dense retrieval quality, especially when the main challenge lies in distinguishing semantically similar but non-identical candidates.

## Paper 6: MTEB

**Title**: *MTEB: Massive Text Embedding Benchmark*  
**Link**: https://arxiv.org/abs/2210.07316

### Why read it

This is the standard benchmark paper to justify why embedding quality should not be treated as a single-number universal property.

### What it argues

MTEB explicitly finds that:

- no single text embedding method dominates across all tasks

### What it supports in the thesis

It supports the claim that:

- model performance depends on task structure
- a model that is stronger on one retrieval benchmark is not automatically better everywhere

### Why it matters for this project

This is exactly what you need to avoid overclaiming.

You should not write:

- Granite is universally better than Qwen3

You should write:

- Granite is better on the current slot-rich, minimal-difference episodic retrieval benchmark

### How to use it in writing

Useful argument:

> Consistent with MTEB, which finds that no embedding approach dominates across all tasks, the present results should be interpreted as benchmark-specific rather than universal model superiority.

## Paper 7: BEIR

**Title**: *BEIR: A Heterogeneous Benchmark for Zero-shot Evaluation of Information Retrieval Models*  
**Link**: https://arxiv.org/abs/2104.08663

### Why read it

This paper is useful for retrieval methodology and benchmark interpretation.

### What it argues

BEIR shows that:

- strong zero-shot retrieval evaluation requires robust baselines
- dense retrieval is not automatically superior in every setting
- evaluation should distinguish different retrieval behaviors carefully

### What it supports in the thesis

It supports the claim that:

- top-k retrieval and top-1 ranking are different phenomena
- robust evaluation matters
- dense retrieval models should be analyzed carefully instead of treated as universally strong

### Why it matters for this project

It helps explain the current metric pattern:

- both models have `Recall@5 = 1.0`
- but `Recall@1` is very different

This means:

- both models retrieve the correct neighborhood
- but differ sharply in exact ranking

### How to use it in writing

Useful argument:

> Following retrieval evaluation principles emphasized in BEIR, the present results suggest that the main challenge is not candidate recall at larger cutoffs, but fine-grained ordering among highly similar retrieved episodes.

## What to Say in the Paper

A safe, well-supported version is:

> Decoder-only LLMs can be adapted into strong embedding models, as shown by SGPT, LLM2Vec, NV-Embed, and related work. However, these studies also suggest that strong embedding performance often requires retrieval-specific adaptation such as bidirectional attention, improved pooling, and contrastive learning. This provides a plausible explanation for why an off-the-shelf decoder-based embedding model may retrieve the correct semantic neighborhood while still underperforming on exact top-1 disambiguation among minimally different episodes.

Another useful sentence is:

> The current benchmark appears to emphasize slot-sensitive hard-negative ranking rather than broad semantic access, which may favor embedding models that preserve fine-grained lexical or structured distinctions more strongly.

## Minimal Set If Time Is Short

If you only have time to read four papers, read these:

1. `SGPT`
2. `LLM2Vec`
3. `NV-Embed`
4. `Improving Text Embeddings with Large Language Models`

If you have more time, then add:

5. `E5`
6. `MTEB`
7. `BEIR`

## Final Recommendation

For your thesis, the most important intellectual move is:

- do not argue that decoder-only models are inherently poor for embeddings
- argue that decoder-only models often need additional embedding-oriented adaptation
- use your own result to show what happens when exact episode disambiguation is hard

That framing is both more accurate and much easier to defend.
