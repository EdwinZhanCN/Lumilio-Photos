# Paper Map for Benchmark Problems

This note maps papers to the concrete issues documented in [BENCHMARK_PROBLEM_STATEMENT.md](./BENCHMARK_PROBLEM_STATEMENT.md).

The goal is not to collect "agent memory papers" in the abstract.
The goal is to find papers that can directly help fix the current benchmark and make later LoRA experiments interpretable.

## How to Read This Note

Each section answers four questions:

1. which current benchmark problem it addresses
2. which papers are the best fit
3. what part of the paper to study
4. what concrete design change it suggests for this project

## Problem 1: Query Construction Leakage

Current issue:

- the query writer sees target-discriminating information such as `required_slots`
- the query is explicitly optimized to uniquely identify the target episode
- this makes the benchmark too close to a target-aware key lookup task

### Paper 1. BEIR: A Heterogenous Benchmark for Zero-shot Evaluation of Information Retrieval Models

- Link: https://arxiv.org/abs/2104.08663
- Why it matters:
  - BEIR is the clearest reference for retrieval evaluation discipline
  - it makes benchmark diversity and strong baselines central rather than assuming one dataset is enough
- What to read:
  - benchmark design motivation
  - the discussion of zero-shot evaluation across heterogeneous tasks
  - baseline comparisons against sparse and dense retrievers
- What to apply here:
  - always report lexical baselines alongside dense baselines
  - evaluate multiple query regimes instead of one templatic query style
  - treat retrieval realism as part of the benchmark, not as a side detail

### Paper 2. BRIGHT: A Realistic and Challenging Benchmark for Reasoning-Intensive Retrieval

- Link: https://arxiv.org/abs/2407.12883
- Why it matters:
  - BRIGHT is useful because it is explicitly built to be hard and realistic
  - it pushes retrieval beyond shallow lexical overlap and simple pattern matching
- What to read:
  - task design
  - benchmark construction choices that increase difficulty
  - analysis of where current retrievers fail
- What to apply here:
  - build queries from weak or indirect cues rather than explicit differentiating slots
  - create tasks where the retriever must recover the right instance before any generator can help
  - explicitly separate "easy surface match" queries from "reasoning-heavy recall" queries

### Paper 3. Promptagator: Few-shot Dense Retrieval From 8 Examples

- Link: https://openreview.net/forum?id=gmL46YMpu2J
- Why it matters:
  - this is useful as a paper about synthetic query generation for training
  - it is also a warning for your evaluation design: synthetic queries are powerful, but they belong on the training side, not on the main test set
- What to read:
  - the query generation pipeline
  - how few examples condition the synthetic generator
  - how generated queries are used to train task-specific retrievers
- What to apply here:
  - if you want synthetic data, generate it only for retriever training or augmentation
  - keep test queries human-authored or at least generated without access to target-specific discriminative slots
  - if synthetic test queries are unavoidable, report them separately from natural queries

## Problem 2: Synthetic Backfill Queries Inflate Test Performance

Current issue:

- many test queries are backfilled directly from target episode goal and entity fields
- this inflates recall and hides the real difficulty of the task

### Paper 1. Promptagator

- Link: https://openreview.net/forum?id=gmL46YMpu2J
- Why it matters:
  - it shows where synthetic query generation is genuinely useful: creating training supervision
- What to apply here:
  - move synthetic query generation to train/val only
  - do not mix synthetic backfill test queries into the main headline metric

### Paper 2. Improving Text Embeddings with Large Language Models

- Link: https://arxiv.org/abs/2401.00368
- Why it matters:
  - this paper shows how synthetic data can be used to train embedding models effectively
  - it is much more relevant to your future LoRA stage than to your benchmark evaluation stage
- What to read:
  - synthetic task and data generation
  - contrastive tuning setup
  - evaluation on BEIR and MTEB
- What to apply here:
  - if you later fine-tune embeddings or adapters, use synthetic queries as training data only
  - design synthetic data generation around task families and hard negatives, not evaluation targets

### Paper 3. BEIR

- Link: https://arxiv.org/abs/2104.08663
- Why it matters:
  - it reinforces the principle that evaluation quality depends on query realism and robust baselines
- What to apply here:
  - break out metrics into:
    - natural queries
    - synthetic training-style queries
    - weak-cue recall queries
  - never collapse them into one single benchmark number

## Problem 3: The Benchmark Is Too Lexically Solvable

Current issue:

- strong performance can be achieved through simple token or slot overlap
- this makes dense retrieval scores less meaningful

### Paper 1. Dense Passage Retrieval for Open-Domain Question Answering

- Link: https://arxiv.org/abs/2004.04906
- Why it matters:
  - DPR is the clean baseline for dual-encoder dense retrieval
  - it gives you the canonical query encoder / document encoder / contrastive training formulation
- What to read:
  - model definition
  - positive and negative example construction
  - top-k retrieval evaluation
- What to apply here:
  - use DPR-style positive/negative training as the simplest clean baseline for episode retrieval
  - define positives as target episodes and negatives as same-family episodes, not just random episodes

### Paper 2. Approximate Nearest Neighbor Negative Contrastive Learning for Dense Text Retrieval

- Link: https://arxiv.org/abs/2007.00808
- Why it matters:
  - ANCE is directly relevant to your minimal-difference setting
  - its core idea is that hard negatives should come from the model's own nearest-neighbor mistakes, not just random negatives
- What to read:
  - online hard negative mining
  - why static easy negatives saturate too early
- What to apply here:
  - after you build a harder benchmark, mine negatives from current retrieval failures
  - use those failure cases to create new episode pairs and LoRA training triples

### Paper 3. RocketQA: An Optimized Training Approach to Dense Passage Retrieval for Open-Domain Question Answering

- Link: https://arxiv.org/abs/2010.08191
- Why it matters:
  - RocketQA focuses on training stability and hard negative quality
  - this is valuable for your future retriever fine-tuning stage
- What to read:
  - denoised hard negatives
  - training refinements that reduce false negative noise
- What to apply here:
  - when you mine hard negatives from same-scenario episodes, do not assume every top-ranked non-target is a clean negative
  - explicitly check whether some negatives are ambiguous or nearly equivalent

### Paper 4. ColBERT: Efficient and Effective Passage Search via Contextualized Late Interaction over BERT

- Link: https://arxiv.org/abs/2004.12832
- Why it matters:
  - your task often depends on a few discriminative fields such as threshold, camera model, failure mode, or album name
  - single-vector embedding can blur those distinctions
  - late interaction is a strong candidate if dual encoders saturate
- What to read:
  - the MaxSim late-interaction scoring idea
  - comparison with single-vector dense retrieval
- What to apply here:
  - if your future hard benchmark still contains localized but critical distinctions, evaluate a late-interaction retriever
  - especially use it when instance differences are concentrated in a few tokens rather than globally paraphrased semantics

## Problem 4: Hard Negatives Are Still Not Hard Enough

Current issue:

- the benchmark still behaves partly like scenario classification plus slot matching
- within-scenario ambiguity is not yet strong enough

### Paper 1. ANCE

- Link: https://arxiv.org/abs/2007.00808
- Why it matters:
  - this is the most directly useful paper for your current "sub minimal difference" direction
- What to apply here:
  - use retrieved false positives to define next-round clusters
  - create benchmark episodes where the current retriever confuses:
    - same scenario, different failure mode
    - same goal, different procedure
    - same entities, different outcome

### Paper 2. RocketQA

- Link: https://arxiv.org/abs/2010.08191
- Why it matters:
  - it helps distinguish informative hard negatives from noisy hard negatives
- What to apply here:
  - build a negative taxonomy:
    - easy negative
    - family negative
    - cluster negative
    - adversarial negative
  - do not train or evaluate with only one difficulty level

### Paper 3. Unsupervised Dense Information Retrieval with Contrastive Learning

- Link: https://arxiv.org/abs/2112.09118
- Why it matters:
  - Contriever is useful if you want a strong contrastive retrieval baseline without heavy task-specific supervision
  - it also provides a good mental model for retrieval quality coming from contrastive structure rather than handcrafted slot logic
- What to read:
  - unsupervised contrastive objective
  - transfer performance on retrieval tasks
- What to apply here:
  - compare your current embedding baseline against a contrastive retriever baseline before spending effort on LoRA

## Problem 5: Querys Are Too Explicit; Real Recall Uses Partial Cues

Current issue:

- users often remember fragments, not full slot bundles
- the benchmark currently gives queries that are too complete

### Paper 1. Precise Zero-Shot Dense Retrieval without Relevance Labels

- Link: https://arxiv.org/abs/2212.10496
- Why it matters:
  - HyDE is directly relevant to weak or underspecified queries
  - it improves retrieval by generating a hypothetical relevant document and retrieving from that representation
- What to read:
  - query transformation process
  - zero-shot retrieval setup
- What to apply here:
  - add a query-side reformulation baseline for weak-cue recall
  - test whether hypothetical episode reconstruction helps when the original user query only contains partial episodic cues

### Paper 2. Generative Relevance Feedback with Large Language Models

- Link: https://arxiv.org/abs/2304.13157
- Why it matters:
  - this paper is useful for query expansion and richer retrieval cues at inference time
  - it is relevant when the user's recall cue is too sparse for first-pass retrieval
- What to read:
  - the generated feedback text types
  - how long-form generated feedback changes retrieval effectiveness
- What to apply here:
  - compare plain query embedding against expanded-query retrieval
  - use this only as an inference-time baseline, separate from benchmark construction

### Paper 3. E5: Text Embeddings by Weakly-Supervised Contrastive Pre-training

- Link: https://arxiv.org/abs/2212.03533
- Why it matters:
  - E5 is a strong practical embedding recipe that works across retrieval settings
  - it is useful if you want a high-quality retrieval baseline before any custom LoRA
- What to read:
  - weakly supervised pair construction
  - retrieval transfer behavior
- What to apply here:
  - use a stronger general embedding baseline before concluding that task-specific fine-tuning is necessary

## Problem 6: LoRA May Only Learn Slot Copying Instead of Episode Discrimination

Current issue:

- the current benchmark is too easy to interpret future gains
- LoRA could appear to help while only learning better slot-template matching

### Paper 1. Improving Text Embeddings with Large Language Models

- Link: https://arxiv.org/abs/2401.00368
- Why it matters:
  - this is the most relevant paper for the eventual fine-tuning stage
  - it combines synthetic data generation with embedding fine-tuning rather than assuming off-the-shelf embeddings are enough
- What to read:
  - synthetic task generation
  - embedding tuning objective
  - evaluation protocol
- What to apply here:
  - if you do LoRA, train on hard episode triples:
    - query
    - target episode
    - same-family hard negative
  - do not train on easy synthetic pairs alone

### Paper 2. E5

- Link: https://arxiv.org/abs/2212.03533
- Why it matters:
  - E5 is a concrete reference for large-scale weak supervision before task-specific tuning
- What to apply here:
  - if LoRA is attempted, initialize from a retriever already trained with contrastive objectives, not a random or purely generative base

### Paper 3. Contriever

- Link: https://arxiv.org/abs/2112.09118
- Why it matters:
  - Contriever gives you a supervision-light contrastive baseline to compare against LoRA
- What to apply here:
  - ask the question:
    - does LoRA beat a strong contrastive baseline on hard negatives and weak-cue queries?
  - if not, the problem may be benchmark design rather than model capacity

## Problem 7: Memory Unit and Retrieval Key Are Not Fully Specified Yet

Current issue:

- the project has a strong conceptual mapping for `What`, `Where`, `When`, `Goal`, and `Tool Trace`
- but the current benchmark still under-exercises procedure and temporal context

### Paper 1. Generative Agents: Interactive Simulacra of Human Behavior

- Link: https://arxiv.org/abs/2304.03442
- Why it matters:
  - this is the most influential memory retrieval framing for agent-like systems
  - it uses memory scoring based on relevance, recency, and importance
- What to read:
  - memory stream design
  - retrieval scoring
  - reflection over stored memories
- What to apply here:
  - separate semantic relevance from temporal and salience signals
  - test whether episode retrieval should remain pure dense retrieval or become a reranked combination of:
    - semantic similarity
    - recency
    - recovery importance

### Paper 2. MemoryBank: Enhancing Large Language Models with Long-Term Memory

- Link: https://arxiv.org/abs/2305.10250
- Why it matters:
  - MemoryBank is useful for thinking about memory writing, retention, and retrieval over long horizons
- What to read:
  - memory storage and update strategy
  - retrieval and memory use in downstream behavior
- What to apply here:
  - define not only retrieval format, but also memory lifecycle:
    - what gets written
    - what gets compressed
    - what gets forgotten

### Paper 3. Augmenting Language Models with Long-Term Memory

- Link: https://arxiv.org/abs/2306.07174
- Why it matters:
  - LongMem is useful if the project grows from external retrieval into memory-augmented generation
- What to read:
  - external memory integration
  - retrieval-conditioned generation setup
- What to apply here:
  - keep your current episode retrieval benchmark separate from later response generation benchmarks
  - otherwise retrieval quality and generator compensation will get mixed together

### Paper 4. MemGPT: Towards LLMs as Operating Systems

- Link: https://arxiv.org/abs/2310.08560
- Why it matters:
  - MemGPT is useful for memory hierarchy design rather than retrieval scoring alone
- What to read:
  - working memory versus archival memory
  - paging and memory management concepts
- What to apply here:
  - define whether episodic memory in this project is:
    - archival store only
    - retrieval cache
    - or part of a multi-tier memory system

## Suggested Reading Order

If the goal is to improve the current benchmark quickly, read in this order:

1. BEIR
2. BRIGHT
3. DPR
4. ANCE
5. RocketQA
6. HyDE
7. Improving Text Embeddings with Large Language Models
8. Generative Agents

This order is deliberate:

- `BEIR + BRIGHT` fix how you think about evaluation
- `DPR + ANCE + RocketQA` fix how you think about hard negatives and retriever training
- `HyDE` addresses weak-cue query retrieval
- `Improving Text Embeddings with LLMs` addresses the later LoRA stage
- `Generative Agents` helps reconnect all of this to agent memory rather than generic IR

## Minimal Core Set for This Project

If you only read six papers, read these:

1. BEIR
2. BRIGHT
3. ANCE
4. HyDE
5. Improving Text Embeddings with Large Language Models
6. Generative Agents

That set covers:

- benchmark realism
- hard negative design
- weak-cue retrieval
- fine-tuning direction
- agent memory framing

## Practical Takeaways

From the literature perspective, the next move should be:

1. redesign the benchmark before trusting LoRA conclusions
2. report lexical and dense baselines separately
3. split test queries into natural, synthetic, and weak-cue subsets
4. mine hard negatives from actual retrieval failures
5. only then train a task-specific retriever or LoRA adapter

The key lesson is simple:

the current bottleneck is more likely benchmark design than model capacity.
