# Slot-Guided Episodic Memory Retrieval for LLM Agents

Author: Zhanzihao

## Abstract

Large language model agents increasingly require memory systems that can recover relevant prior experiences instead of relying only on the current prompt. In this project, we study episodic memory retrieval for an agent operating in a media-management environment. Rather than modeling fully ambiguous human-style recollection, we focus on a more agent-oriented setting: the query may contain several discriminative cues, and the retrieval system must use them to precisely recover the correct prior episode from a set of highly similar candidate episodes. To support this setting, we formulate each memory item as an `episode` containing task-level content, goal, and tool trace, and we organize the benchmark into `scenario`, `cluster`, and `episode` levels. We then construct a minimal-difference evaluation protocol in which hard negatives differ from the target episode by only a small number of fields. Our analysis shows that retrieval performance is strongly affected by benchmark design, especially query construction and test-set composition. Based on this observation, we argue that the main challenge is not only embedding quality but also fine-grained episode discrimination. This project provides a structured episodic memory formulation, a benchmark design for instance-level retrieval, and a foundation for later LoRA-based retriever adaptation.

## 1. Introduction

Recent LLM agents have demonstrated strong reasoning and tool-use capabilities, but they still struggle with long-horizon memory. In many practical applications, an agent should be able to answer questions such as: *How did I solve a similar task last time?*, *Which prior attempt is most relevant to the current request?*, or *Which previous workflow should be reused?* These questions are not standard document retrieval problems. Instead, they require retrieving a specific prior experience.

This project studies episodic memory retrieval for an LLM-based media agent. The agent performs tasks such as curating albums, inspecting photo metadata, cleaning duplicate images, grouping assets for review, and summarizing selected photos. Each past execution can be represented as an episode containing what the task was about, why it was performed, and how it was completed. When a new query arrives, the memory system should recover the most relevant prior episode.

The key challenge is that many episodes are highly similar. For example, two duplicate-cleanup episodes may share the same scenario and nearly the same procedure, while differing only in location, camera model, or similarity threshold. Therefore, the retrieval problem is not only to recognize the correct task family, but also to distinguish between minimally different episodes.

In this project, we intentionally focus on a **slot-guided** retrieval setting. The query is allowed to contain several important cues, such as location, album name, camera model, time window, or failure mode. This design reflects an agent-oriented use case: at runtime, the agent may already know part of the current context, and the memory module should use these cues to precisely identify the correct prior episode. Thus, our task is better described as **agent-guided precise episodic retrieval** rather than fully ambiguous human-like recollection.

The main contributions of this project are as follows:

1. We define an episodic memory representation for agents based on task content, goal, and tool trace.
2. We organize the retrieval benchmark into `scenario`, `cluster`, and `episode` levels to evaluate both coarse and fine-grained retrieval.
3. We construct a minimal-difference retrieval setting that emphasizes hard negative discrimination.
4. We analyze benchmark design issues that can inflate retrieval scores and identify directions for later LoRA-based retriever adaptation.

## 2. Related Work

Retrieval-augmented systems typically retrieve external text passages to support downstream reasoning or generation. Classical retrieval-augmented generation work focuses on retrieving factual knowledge passages from large corpora. However, episodic memory retrieval for agents differs from standard RAG because the retrieval target is not a generic passage but a structured prior experience.

Research on dense retrieval, such as dual-encoder methods, provides the technical foundation for this project. These methods encode queries and candidate items into a shared embedding space and rank candidates by similarity. Later work on hard negative mining further shows that retrieval quality depends strongly on whether the model is trained to distinguish highly similar non-target examples. This is especially relevant in our setting because the primary difficulty lies in separating closely related episodes rather than separating unrelated task families.

Work on agent memory introduces a complementary perspective. Instead of treating memory as a flat document store, agent memory research emphasizes the importance of storing experiences, goals, and execution traces over time. In this view, memory retrieval supports future planning and adaptation rather than only factual answering. Our project follows this direction by treating each memory item as an episode and by explicitly modeling both semantic task content and procedural information.

Compared with previous work, this project makes two narrower choices. First, we do not attempt to build a full long-horizon agent architecture with reflection, forgetting, and hierarchical memory control. Second, we do not frame the retrieval task as unconstrained natural-language recall. Instead, we study a simpler and more controllable setting in which the query may contain structured or semi-structured cues. This narrower formulation is suitable for a course project because it isolates the core retrieval problem and enables more interpretable experiments.

## 3. Methodology

### 3.1 Problem Formulation

We define episodic memory retrieval as the following task:

- input: a query describing the current recall need
- memory corpus: a set of previously stored episodes
- output: the single most relevant prior episode

Unlike standard passage retrieval, the target is one concrete past agent experience. The evaluation metric is therefore instance-level retrieval accuracy, especially `Recall@1`.

### 3.2 Episodic Memory Representation

Each episode is represented using the following dimensions:

- `What`: scenario, intent, summary
- `Goal`: the explicit objective of the episode
- `Task Content`: sparse entities such as location, album name, camera model, time window, rating threshold, or failure mode
- `Tool Trace`: the sequence of tools used during execution
- `Where`: workspace and route
- `When`: started and ended timestamps

In this project, the dense retrieval text is built mainly from `What`, `Goal`, `Task Content`, and `Tool Trace`. Contextual fields such as workspace and timestamps are stored as metadata and can be used later for analysis or reranking.

### 3.3 Scenario, Cluster, and Episode

The benchmark is organized into three levels.

#### Scenario

A `scenario` is a high-level task family, such as:

- trip album curation
- duplicate cleanup
- metadata inspection
- selection summarization

This is the coarsest level of abstraction.

#### Cluster

A `cluster` groups several highly similar episodes within the same scenario. Episodes in the same cluster share most of their semantics and procedure, but differ in one or a few discriminative fields. For example, two duplicate-cleanup episodes may differ only in location or similarity threshold.

The purpose of clusters is to create **hard negatives**. If a retriever can only separate different scenarios, then the task is too easy. Cluster-level organization makes the benchmark test whether the system can discriminate between near-neighbor episodes.

#### Episode

An `episode` is one concrete past execution. It is the final retrieval target. Each episode contains its own goal, summary, entities, tool trace, and metadata.

This three-level design lets us distinguish three retrieval abilities:

1. recognizing the correct task family (`scenario`)
2. narrowing the search to the right local neighborhood (`cluster`)
3. recovering the exact target experience (`episode`)

### 3.4 Slot-Guided Query Design

The query design in this project is intentionally **slot-guided**. A query is allowed to contain important discriminative cues, such as location, album, time window, threshold, or camera model. This differs from a fully open-ended human memory task, but it matches a realistic agent use case: an agent may already know partial structured context from the current interaction and should use that context to recall the right episode.

For this reason, the benchmark should be interpreted as a test of **precise episode capture under partial known cues**, not as a test of free-form human-style recollection.

### 3.5 Retrieval Objective

Given a query and a corpus of candidate episodes, the retriever should assign the highest score to the correct target episode. The main objective is not only semantic matching at the task-family level, but also discrimination among minimally different episodes.

This makes hard negative handling central to the method. In our setting, a useful retriever should distinguish:

- same scenario, different cluster
- same cluster family, different discriminative slot
- similar goal, different execution details

## 4. Experimental Setup

### 4.1 Dataset

The dataset consists of synthetic but structured media-agent episodes. Each episode belongs to one of several scenarios, such as album curation, duplicate cleanup, bulk like, archiving low-rated assets, metadata inspection, grouping assets, and summarization.

To increase difficulty, episodes are grouped into minimal-difference clusters. Within each cluster, the episodes share most of their content but differ in one or two fields, such as:

- location
- time window
- camera model
- rating threshold
- failure mode
- album name

This setup allows us to evaluate whether the retriever can identify the correct instance rather than only the correct task family.

### 4.2 Query Sets

We distinguish between two types of test queries:

1. **Primary test queries**
   These are intended to reflect the main retrieval setting and should not be directly backfilled from the target episode using simple templates.

2. **Auxiliary synthetic queries**
   These may be generated to improve coverage or debugging, but they should be reported separately because they are typically easier and may inflate benchmark scores.

This distinction is important because test queries generated directly from target goals and entities can make the benchmark unrealistically easy.

### 4.3 Retrieval Model

The current baseline uses an embedding-based dense retriever:

- each episode is converted into a retrieval text representation
- the retrieval text is embedded into a vector space
- the query is also embedded
- nearest-neighbor search is performed in the vector database

The primary evaluation metric is `Recall@1`, with `Recall@5` and mean reciprocal rank (MRR) used as secondary measures.

### 4.4 Planned LoRA Extension

A later extension of this project is to apply LoRA to the embedding model or retrieval encoder. However, the goal of LoRA is not to improve every query uniformly. Instead, the intended objective is:

- stronger discrimination on hard negatives
- better ranking within the same scenario
- better cluster-level instance separation

Therefore, the most meaningful LoRA evaluation should focus on hard subsets rather than only overall average recall.

## 5. Results and Preliminary Analysis

Preliminary experiments suggest that retrieval quality is strongly affected by benchmark construction. In particular, results can be artificially increased when:

- test queries are generated directly from target episode information
- queries expose too many discriminative slots
- test sets are too small
- hard negative coverage is limited

This observation is important for interpreting high baseline scores. A high score does not necessarily mean the memory formulation is already solved. It may instead mean that the benchmark has become too easy due to query design or insufficiently difficult negatives.

A second preliminary observation is that the most meaningful challenge in this project is not coarse scenario classification, but fine-grained episode discrimination. In many cases, the retriever can already identify the correct scenario family. The remaining difficulty is to distinguish between closely related episodes inside that family.

This insight motivates the planned LoRA stage: if fine-tuning is applied, it should be evaluated mainly on hard same-scenario and same-cluster confusions rather than only on global recall.

## 6. Discussion

The main conceptual lesson of this project is that episodic retrieval for agents must be defined carefully. If the task is described as human-like memory recall, then allowing discriminative slots in the query may seem too strong. However, for an agent-oriented memory tool, slot-guided recall is a reasonable design choice because the agent often has structured contextual information available at runtime.

This leads to an important distinction between two tasks:

1. **free-form episodic recollection**
2. **agent-guided precise episodic retrieval**

Our project studies the second task. This narrower formulation is still useful, because many agent systems need exactly this capability: given several known cues, recover the most relevant prior episode.

At the same time, the experiments also reveal a limitation. If the benchmark relies too heavily on queries that expose target-specific slots or are constructed directly from target episodes, then the retrieval task may become overly lexical and not sufficiently challenging. Therefore, later benchmark refinement should focus on producing more balanced query sets and stronger hard negatives without abandoning the slot-guided setting entirely.

## 7. Conclusion

This project studies episodic memory retrieval for LLM agents in a media-management environment. We formulate memory retrieval as instance-level episode search, define a structured episode representation, and organize the benchmark using scenario, cluster, and episode levels. Our design emphasizes precise retrieval under partially known cues and highlights the importance of minimal-difference hard negatives.

The main takeaway is that benchmark design is as important as model choice. High retrieval scores may not reflect true instance-level memory quality unless the query set and candidate set are carefully constructed. For this reason, future work should improve query generation, isolate cleaner hard subsets, and evaluate whether LoRA can improve fine-grained same-family discrimination.

## References

This section should include the final bibliography in the citation format required by the course. Likely references include work on dense retrieval, hard negatives, agent memory, and memory-augmented language models.

## Appendix: Notes for Revision

This draft is intentionally written as a course-project-ready scaffold rather than a final paper. Before submission, the following items should be revised:

1. replace the placeholder author line with the final author list
2. update the abstract after final results are available
3. add the exact retrieval model names and benchmark statistics
4. insert tables and figures in the Results section
5. expand the Related Work section with specific citations
6. clarify whether LoRA was completed or remains future work
