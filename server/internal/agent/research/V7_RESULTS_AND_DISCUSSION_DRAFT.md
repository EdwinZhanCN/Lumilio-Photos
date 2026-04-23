# Baseline-v7 Results and Discussion Draft

This note provides a paper-ready draft interpretation of the current `baseline-v7` retrieval results.

## Results

We evaluated two embedding models on the `baseline-v7` benchmark under the semantic retrieval setting: `qwen3-embedding:0.6b` and `granite-embedding:278m`.

The overall results are shown below.

| Model | Recall@1 | Recall@5 | MRR@10 |
| --- | ---: | ---: | ---: |
| `qwen3-embedding:0.6b` | 0.589 | 1.000 | 0.761 |
| `granite-embedding:278m` | 0.911 | 1.000 | 0.951 |

Two observations are especially important.

First, both models achieved perfect `Recall@5 = 1.0`. This means the correct target episode was consistently retrieved into the top candidate set. Therefore, the benchmark is not mainly testing whether the model can identify the correct broad task family.

Second, a large gap appears at `Recall@1`. While both models can retrieve the correct episode neighborhood, Granite is substantially better at placing the exact target episode at the top rank. This indicates that `baseline-v7` is primarily testing fine-grained ranking among minimally different episodes rather than only coarse semantic retrieval.

Scenario-level results further support this interpretation.

| Scenario | Qwen3 Recall@1 | Granite Recall@1 |
| --- | ---: | ---: |
| `archive_low_rated_assets` | 0.444 | 0.889 |
| `bulk_like_highlights` | 0.333 | 0.833 |
| `cleanup_duplicate_shoot` | 0.556 | 0.889 |
| `curate_trip_album` | 0.500 | 1.000 |
| `group_assets_for_review` | 0.889 | 1.000 |
| `inspect_camera_metadata` | 0.333 | 1.000 |
| `summarize_selected_assets` | 0.889 | 0.778 |

Granite outperformed Qwen3 in six of the seven scenarios. The largest gains appeared in:

- `archive_low_rated_assets`
- `bulk_like_highlights`
- `cleanup_duplicate_shoot`
- `inspect_camera_metadata`

These scenarios depend strongly on precise disambiguation cues such as rating threshold, similarity threshold, camera model, album name, or time window. This suggests that Granite is better aligned with the slot-sensitive ranking demands of the current benchmark.

At the same time, Granite did not dominate every case. In `summarize_selected_assets`, Qwen3 slightly outperformed Granite (`0.889` vs `0.778` Recall@1). This suggests that the relative advantage of each embedding model may depend on the dominant retrieval cues of a given scenario.

## Error Analysis

Error analysis shows that most Qwen3 failures were not cross-task failures.

- Qwen3 produced `23` non-top-1 errors
- `22` of those `23` errors still had a top-1 result in the same scenario and intent
- Granite produced only `5` non-top-1 errors
- all `5` Granite errors still remained within the same scenario and intent

This is an important result. It means that Qwen3 usually identifies the correct task family, but often fails to rank the exact target episode above highly similar hard negatives. In other words, Qwen3 is usually semantically close, but less stable at instance-level discrimination.

The top Qwen3 failures are consistent with this pattern:

- `1 or lower` is confused with `2 or lower`
- `rating 4+` is confused with `rating 3+`
- `0.74` threshold is confused with `0.82` or `0.86`
- similar duplicate-cleanup episodes are confused when only one slot differs

These are exactly the kinds of minimal-difference distinctions that the benchmark was designed to test.

## Discussion

The most important interpretation is not simply that Granite is a better embedding model in general. A more careful interpretation is that `baseline-v7` emphasizes exact episode disambiguation under slot-rich queries.

Under this setting:

- broad semantic access is already easy enough for both models
- the true challenge is ordering highly similar candidate episodes
- small differences in thresholds, entities, and structured cues matter heavily

This reading is strongly supported by the metric pattern:

- `Recall@5 = 1.0` for both models
- the major difference appears only in `Recall@1`

Therefore, the benchmark is separating two abilities:

1. retrieving the correct semantic neighborhood
2. selecting the exact correct episode within that neighborhood

The current results suggest that Granite is stronger on the second ability.

Another useful implication is methodological. These results indicate that `baseline-v7` is no longer a simple scenario-classification benchmark. If the benchmark only measured broad task-family recognition, both models would likely saturate at top-1 as well. Instead, the large top-1 gap shows that the benchmark is sensitive to minimal-difference hard negatives, which is precisely the intended design goal.

At the same time, the results also show that the current benchmark remains heavily slot-driven. Almost all test queries contain explicit numeric or structured cues, and both models achieve near-perfect retrieval once the top-5 neighborhood is considered. This means the benchmark is best interpreted as a test of precise, slot-guided episodic retrieval rather than free-form fuzzy recollection.

## Suggested Paper Framing

The results can be summarized in the paper as follows:

> On the `baseline-v7` benchmark, both embedding models consistently retrieved the correct episode into the top candidate set, achieving perfect `Recall@5`. However, a large gap remained at `Recall@1`, where Granite substantially outperformed Qwen3. This shows that the benchmark primarily measures fine-grained discrimination among minimally different episodes rather than coarse task-family retrieval.

Another concise formulation is:

> The main challenge in `baseline-v7` is not retrieving the correct semantic neighborhood, but ranking the exact target episode first within that neighborhood.

## Conclusion

The `baseline-v7` benchmark produced a useful and interpretable result.

- both models can retrieve the correct episode neighborhood
- Granite is much stronger at exact top-1 ranking
- Qwen3 errors are mostly same-scenario confusions rather than complete retrieval failures
- the benchmark is successfully measuring hard-negative discrimination at the episode level

For the current project, this is a strong result because it shows that the benchmark is no longer trivially solved at top-1, while still remaining stable and interpretable.
