# InfoNCE Embedding Papers Note

This note collects the most relevant papers for understanding why an InfoNCE-style objective is a good fit for this project's LoRA stage.

The goal here is not to list every contrastive learning paper, but to identify the papers that are most useful for explaining:

- why contrastive objectives are effective for text embeddings
- why multiple negatives are valuable
- why this project should prefer an InfoNCE-style setup over a pure triplet-only formulation
- how decoder-based embedding models can also be fine-tuned under this paradigm

## Main Motivation for This Project

The current dataset structure naturally supports:

- one query
- one positive target episode
- multiple hard negative episodes

This makes an InfoNCE-style objective especially natural, because it can compare the positive against a whole set of competing negatives in one loss term.

For this reason, the most useful literature is the line of work that uses contrastive learning for text embedding and dense retrieval.

## Recommended Reading Order

If you only want the most relevant path for the LoRA stage, read in this order:

1. `SimCSE`
2. `E5`
3. `Improving Text Embeddings with Large Language Models`
4. `NV-Embed`
5. `LLM2Vec`
6. `Contriever`
7. `DPR`

## Paper 1: SimCSE

**Title**: *SimCSE: Simple Contrastive Learning of Sentence Embeddings*  
**Link**: https://arxiv.org/abs/2104.08821

### Why this paper matters

SimCSE is one of the clearest and most influential papers showing that a simple contrastive objective can produce strong sentence embeddings.

### What it supports

It supports the claim that:

- text embedding models can be effectively improved through contrastive learning
- even a relatively simple setup can produce large gains
- contrastive objectives improve sentence representation geometry

### What to focus on

- the difference between unsupervised and supervised SimCSE
- how positives and negatives are defined
- why the contrastive objective helps regularize the embedding space

### How it helps this project

This paper is useful for justifying the general move from raw embedding behavior to embedding fine-tuning with a contrastive objective.

### Useful paper-writing sentence

> Following SimCSE, contrastive learning can directly improve the geometry of sentence representations and is therefore a natural objective for improving query-episode retrieval embeddings.

## Paper 2: E5

**Title**: *Text Embeddings by Weakly-Supervised Contrastive Pre-training*  
**Link**: https://arxiv.org/abs/2212.03533

### Why this paper matters

E5 is one of the strongest references for using contrastive learning in text embedding and dense retrieval.

### What it supports

It supports the claim that:

- weakly supervised contrastive pre-training is highly effective for text embeddings
- multiple negatives and large-scale pair construction matter
- contrastive training can produce strong general-purpose retrieval representations

### What to focus on

- how training pairs are constructed
- why weak supervision is sufficient
- how the model is evaluated on BEIR and MTEB

### How it helps this project

This is one of the best references for explaining why contrastive learning is a principled choice for episodic retrieval embeddings.

It is also especially useful if you want to argue:

- the objective should not merely separate unrelated samples
- it should sharpen distinctions among semantically close candidates

### Useful paper-writing sentence

> E5 demonstrates that contrastive learning with large-scale weak supervision is highly effective for retrieval-oriented text embeddings, supporting the use of a contrastive objective for episode-level ranking.

## Paper 3: Improving Text Embeddings with Large Language Models

**Title**: *Improving Text Embeddings with Large Language Models*  
**Link**: https://arxiv.org/abs/2401.00368

### Why this paper matters

This paper is highly aligned with the current project because it fine-tunes decoder-only LLMs for embedding using standard contrastive loss.

### What it supports

It supports the claim that:

- decoder-only LLMs can be turned into stronger embedding models through contrastive fine-tuning
- synthetic training data is a viable source of supervision
- a relatively simple contrastive setup can be effective

### What to focus on

- how synthetic data is generated
- how the decoder-only model is fine-tuned
- what kind of contrastive loss is used
- how the authors compare against strong text embedding baselines

### How it helps this project

This is probably the most important reference for the planned LoRA stage, because it directly supports:

- taking a decoder-based embedding model
- fine-tuning it with contrastive data
- improving retrieval behavior

### Useful paper-writing sentence

> Recent work has shown that decoder-only LLMs can be improved as embedding models through standard contrastive fine-tuning, which motivates the use of an InfoNCE-style LoRA objective in this project.

## Paper 4: NV-Embed

**Title**: *NV-Embed: Improved Techniques for Training LLMs as Generalist Embedding Models*  
**Link**: https://arxiv.org/abs/2405.17428

### Why this paper matters

This is one of the strongest modern papers on training decoder-only LLMs as embedding models.

### What it supports

It supports the claim that:

- decoder-only LLMs can become very strong embedding models
- however, this typically requires retrieval-specific adaptation
- multiple negatives and hard negatives are central to strong performance

### What to focus on

- latent attention pooling
- removal of the causal mask during contrastive training
- two-stage contrastive instruction tuning
- how hard negatives are used

### How it helps this project

This paper is especially useful if you want to explain why:

- off-the-shelf decoder embeddings may underperform
- but LoRA or contrastive adaptation could realistically improve them

### Useful paper-writing sentence

> NV-Embed shows that strong decoder-based embedding performance depends not only on model scale but also on retrieval-specific design choices such as contrastive training, improved pooling, and hard-negative optimization.

## Paper 5: LLM2Vec

**Title**: *LLM2Vec: Large Language Models Are Secretly Powerful Text Encoders*  
**Link**: https://arxiv.org/abs/2404.05961

### Why this paper matters

LLM2Vec is important because it explicitly explains how decoder-only LLMs can be converted into stronger text encoders.

### What it supports

It supports the claim that:

- raw decoder-only representations are not necessarily ideal for embedding
- additional adaptation such as bidirectional attention and contrastive training helps significantly

### What to focus on

- bidirectional attention
- masked next token prediction
- unsupervised and supervised contrastive learning

### How it helps this project

This paper gives you a strong explanation for why a decoder-based embedding model may retrieve the right neighborhood but still struggle on exact top-1 ranking.

### Useful paper-writing sentence

> LLM2Vec suggests that decoder-only LLMs can become strong text encoders, but typically only after explicit architectural and contrastive adaptation, which supports the decision to fine-tune a decoder-based embedding model rather than relying on its off-the-shelf representation alone.

## Paper 6: Contriever

**Title**: *Unsupervised Dense Information Retrieval with Contrastive Learning*  
**Link**: https://arxiv.org/abs/2112.09118

### Why this paper matters

Contriever is especially useful because it is retrieval-centered and explicitly based on contrastive learning.

### What it supports

It supports the claim that:

- contrastive learning can train useful dense retrievers even with limited supervision
- retrieval quality can improve substantially from a contrastive objective alone

### What to focus on

- how they define positives and negatives
- how contrastive learning is used for retrieval rather than only sentence similarity
- how performance is evaluated on BEIR

### How it helps this project

This paper is helpful when you want to emphasize that the current task is not just sentence embedding, but dense retrieval with strong hard-negative structure.

### Useful paper-writing sentence

> Similar to Contriever, our setting treats retrieval as a contrastive representation learning problem in which the correct target must be ranked above a set of competing candidates.

## Paper 7: DPR

**Title**: *Dense Passage Retrieval for Open-Domain Question Answering*  
**Link**: https://arxiv.org/abs/2004.04906

### Why this paper matters

DPR is not usually introduced as an InfoNCE paper in name, but it is one of the foundational dense retrieval papers using a contrastive dual-encoder objective.

### What it supports

It supports the claim that:

- dense retrieval can be framed as query-document representation learning
- ranking the correct positive above negatives is a standard retrieval formulation

### What to focus on

- query encoder / passage encoder structure
- negative sampling
- dual-encoder ranking setup

### How it helps this project

It gives the historical retrieval framing for the current approach:

- query as anchor
- target episode as positive
- hard negative episodes as negatives

### Useful paper-writing sentence

> Following the dense retrieval view popularized by DPR, the current episodic retrieval task can be formulated as learning a query embedding that ranks the correct episode above competing hard negatives.

## Why InfoNCE Fits This Project Better Than Pure Triplet Loss

This project naturally produces:

- one positive target episode
- multiple same-scenario hard negatives

For this reason, an InfoNCE-style objective is more natural than a strict one-negative triplet formulation.

Triplet loss is still conceptually useful because it clearly expresses the ranking intuition:

- pull the anchor toward the positive
- push it away from the negative

However, InfoNCE is a better fit for this project because it:

- uses all negatives at once
- is closer to the actual top-1 ranking problem
- aligns well with candidate competition during retrieval
- is widely used in modern embedding papers

## What to Say in the Paper

A safe, practical formulation is:

> We adopt an InfoNCE-style contrastive objective for LoRA fine-tuning. For each training query, the gold target episode is treated as the positive example, while multiple minimally different episodes from the same scenario serve as hard negatives. This setup is aligned with prior contrastive embedding work such as SimCSE, E5, Contriever, and recent LLM-based embedding models.

Another useful sentence is:

> Compared with a pure triplet formulation, the InfoNCE-style objective better matches the structure of the current training data, where each query is associated with one positive target and multiple competing hard negatives.

## Minimal Reading Set If Time Is Short

If you only have time to read four papers for the LoRA justification, read:

1. `SimCSE`
2. `E5`
3. `Improving Text Embeddings with Large Language Models`
4. `NV-Embed`

If you have more time, then add:

5. `LLM2Vec`
6. `Contriever`
7. `DPR`

## Final Recommendation

For the current thesis, the most defensible chain of reasoning is:

1. modern embedding models are often improved through contrastive learning
2. this project naturally provides one positive and multiple hard negatives
3. therefore an InfoNCE-style objective is better aligned with the dataset structure than a pure triplet-only formulation
4. this is especially appropriate because the main benchmark challenge is exact top-1 ranking among minimally different episodes
