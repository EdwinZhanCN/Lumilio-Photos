# Benchmark Analysis

- Generated at: `2026-04-24T03:06:02.608868+00:00`
- Report count: `4`

## qwen3-baseline-v7-semantic.json

- Collection: `agent_episodic_memory_qwen3_1024_v7_semantic`
- Model: `qwen3-embedding:0.6b` @ `1024d`
- Queries: `56`
- Overall: `recall@1=0.589`, `recall@5=1.000`, `mrr@10=0.761`
- Numeric slots / with number: `n=55`, `r@1=0.582`, `mrr=0.757`
- Numeric slots / without number: `n=1`, `r@1=1.000`, `mrr=1.000`
- Error topology: `misses=23`, `same_scenario_top1=22`, `same_intent_top1=22`

### By Scenario

| Scenario | N | Recall@1 | Recall@5 | MRR |
| --- | ---: | ---: | ---: | ---: |
| `archive_low_rated_assets` | 9 | 0.444 | 1.000 | 0.615 |
| `bulk_like_highlights` | 6 | 0.333 | 1.000 | 0.639 |
| `cleanup_duplicate_shoot` | 9 | 0.556 | 1.000 | 0.741 |
| `curate_trip_album` | 8 | 0.500 | 1.000 | 0.750 |
| `group_assets_for_review` | 9 | 0.889 | 1.000 | 0.944 |
| `inspect_camera_metadata` | 6 | 0.333 | 1.000 | 0.597 |
| `summarize_selected_assets` | 9 | 0.889 | 1.000 | 0.944 |

### Top Misses

- Rank `2` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Archive assets rated 1 or lower from Tokyo Collection album taken in Tokyo during spring 2023
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c01_anchor_tokyo_collection_tokyo_2_spring_2023`
- Rank `3` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Archive photos rated 5 or below from Family Holiday autumn 2023
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c04_anchor_holiday_cull_family_holiday_2_autumn_2023`
- Rank `5` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Archive assets rated 2 or below from Alps summer 2024 to Wedding Cleanup album
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c01_near_time_window_tokyo_collection_tokyo_2_autumn_2024`
- Rank `4` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Archive assets rated 2 or below from Family Holiday summer 2024 to Wedding Cleanup album
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c01_near_time_window_tokyo_collection_tokyo_2_autumn_2024`
- Rank `4` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Archive assets rated 2 or below from Alps autumn 2024 to Wedding Cleanup album
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c01_near_time_window_tokyo_collection_tokyo_2_autumn_2024`

## granite-baseline-v7-semantic.json

- Collection: `agent_episodic_memory_granite_768_v7_semantic`
- Model: `granite-embedding:278m` @ `768d`
- Queries: `56`
- Overall: `recall@1=0.911`, `recall@5=1.000`, `mrr@10=0.951`
- Numeric slots / with number: `n=55`, `r@1=0.909`, `mrr=0.950`
- Numeric slots / without number: `n=1`, `r@1=1.000`, `mrr=1.000`
- Error topology: `misses=5`, `same_scenario_top1=5`, `same_intent_top1=5`

### By Scenario

| Scenario | N | Recall@1 | Recall@5 | MRR |
| --- | ---: | ---: | ---: | ---: |
| `archive_low_rated_assets` | 9 | 0.889 | 1.000 | 0.944 |
| `bulk_like_highlights` | 6 | 0.833 | 1.000 | 0.917 |
| `cleanup_duplicate_shoot` | 9 | 0.889 | 1.000 | 0.944 |
| `curate_trip_album` | 8 | 1.000 | 1.000 | 1.000 |
| `group_assets_for_review` | 9 | 1.000 | 1.000 | 1.000 |
| `inspect_camera_metadata` | 6 | 1.000 | 1.000 | 1.000 |
| `summarize_selected_assets` | 9 | 0.778 | 1.000 | 0.861 |

### Top Misses

- Rank `2` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Archive photos rated 5 or below from Family Holiday autumn 2023
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c04_anchor_holiday_cull_family_holiday_2_autumn_2023`
- Rank `2` target `bulk_like_highlights/bulk_like` vs top1 `bulk_like_highlights/bulk_like`
  Query: Locate the episode where unliked assets from Johnson Wedding with a rating threshold of 1 were bulk liked into the Wedding Highlights album.
  Top1 episode: `ep_bulk_like_highlights_bulk_like_highlights_c05_near_rating_threshold_wedding_highlights_false_johnson_wedding_2`
- Rank `2` target `cleanup_duplicate_shoot/cleanup_duplicates` vs top1 `cleanup_duplicate_shoot/cleanup_duplicates`
  Query: Find previous episode where false positive duplicates were handled for Sony A7C II photos from Yosemite using 0.82 similarity threshold, with metadata inspection revealing 8 false positives among 12 assets.
  Top1 episode: `ep_cleanup_duplicate_shoot_cleanup_duplicate_shoot_c09_anchor_sony_a7c_ii_false_positive_duplicate_yosemite_0_86`
- Rank `4` target `summarize_selected_assets/summarize_selection` vs top1 `summarize_selected_assets/summarize_selection`
  Query: Find sunset-themed photos from Beach during summer 2023 with average rating around 4.2
  Top1 episode: `ep_summarize_selected_assets_summarize_selected_assets_c05_anchor_beach_street_moments_summer_2023`
- Rank `2` target `summarize_selected_assets/summarize_selection` vs top1 `summarize_selected_assets/summarize_selection`
  Query: Locate Beach sunset photos from spring 2023 with panoramic shots and average rating 3.8
  Top1 episode: `ep_summarize_selected_assets_summarize_selected_assets_c05_anchor_beach_street_moments_summer_2023`

## embeddinggemma-baseline-v7-semantic.json

- Collection: `agent_episodic_memory_embeddinggemma_768_v7_semantic`
- Model: `embeddinggemma:latest` @ `768d`
- Queries: `56`
- Overall: `recall@1=0.750`, `recall@5=1.000`, `mrr@10=0.856`
- Numeric slots / with number: `n=55`, `r@1=0.745`, `mrr=0.854`
- Numeric slots / without number: `n=1`, `r@1=1.000`, `mrr=1.000`
- Error topology: `misses=14`, `same_scenario_top1=12`, `same_intent_top1=12`

### By Scenario

| Scenario | N | Recall@1 | Recall@5 | MRR |
| --- | ---: | ---: | ---: | ---: |
| `archive_low_rated_assets` | 9 | 0.889 | 1.000 | 0.944 |
| `bulk_like_highlights` | 6 | 0.833 | 1.000 | 0.917 |
| `cleanup_duplicate_shoot` | 9 | 0.667 | 1.000 | 0.833 |
| `curate_trip_album` | 8 | 0.875 | 1.000 | 0.938 |
| `group_assets_for_review` | 9 | 0.667 | 1.000 | 0.769 |
| `inspect_camera_metadata` | 6 | 0.500 | 1.000 | 0.722 |
| `summarize_selected_assets` | 9 | 0.778 | 1.000 | 0.856 |

### Top Misses

- Rank `2` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Archive photos rated 5 or below from Family Holiday autumn 2023
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c04_anchor_holiday_cull_family_holiday_2_autumn_2023`
- Rank `2` target `bulk_like_highlights/bulk_like` vs top1 `bulk_like_highlights/bulk_like`
  Query: Locate the episode where unliked assets from Johnson Wedding with a rating threshold of 1 were bulk liked into the Wedding Highlights album.
  Top1 episode: `ep_bulk_like_highlights_bulk_like_highlights_c05_near_rating_threshold_wedding_highlights_false_johnson_wedding_2`
- Rank `2` target `cleanup_duplicate_shoot/cleanup_duplicates` vs top1 `cleanup_duplicate_shoot/cleanup_duplicates`
  Query: Find previous episode where false positive duplicates were handled for Sony A7C II photos from Yosemite using 0.82 similarity threshold, with metadata inspection revealing 8 false positives among 12 assets.
  Top1 episode: `ep_cleanup_duplicate_shoot_cleanup_duplicate_shoot_c09_anchor_sony_a7c_ii_false_positive_duplicate_yosemite_0_86`
- Rank `2` target `cleanup_duplicate_shoot/cleanup_duplicates` vs top1 `cleanup_duplicate_shoot/cleanup_duplicates`
  Query: Retrieve episode about cleaning false positive duplicates from Yosemite shoot with Canon EOS R5 at 0.82 threshold, where 6 false positives were identified among 10 potential duplicate pairs.
  Top1 episode: `ep_cleanup_duplicate_shoot_cleanup_duplicate_shoot_c09_near_camera_model_fujifilm_x_t5_false_positive_duplicate_yosemite_0_86`
- Rank `2` target `cleanup_duplicate_shoot/cleanup_duplicates` vs top1 `cleanup_duplicate_shoot/cleanup_duplicates`
  Query: Search for episode where Sony A7C II photos from Yosemite were processed with 0.74 similarity threshold, resulting in 25 potential duplicate pairs and quota limit issues during archiving.
  Top1 episode: `ep_cleanup_duplicate_shoot_cleanup_duplicate_shoot_c09_near_failure_mode_sony_a7c_ii_near_duplicate_burst_yosemite_0_86`

## bge-m3-baseline-v7-semantic.json

- Collection: `agent_episodic_memory_bge_m3_1024_v7_semantic`
- Model: `bge-m3:latest` @ `1024d`
- Queries: `56`
- Overall: `recall@1=0.911`, `recall@5=1.000`, `mrr@10=0.952`
- Numeric slots / with number: `n=55`, `r@1=0.909`, `mrr=0.952`
- Numeric slots / without number: `n=1`, `r@1=1.000`, `mrr=1.000`
- Error topology: `misses=5`, `same_scenario_top1=4`, `same_intent_top1=4`

### By Scenario

| Scenario | N | Recall@1 | Recall@5 | MRR |
| --- | ---: | ---: | ---: | ---: |
| `archive_low_rated_assets` | 9 | 0.889 | 1.000 | 0.944 |
| `bulk_like_highlights` | 6 | 1.000 | 1.000 | 1.000 |
| `cleanup_duplicate_shoot` | 9 | 0.778 | 1.000 | 0.889 |
| `curate_trip_album` | 8 | 1.000 | 1.000 | 1.000 |
| `group_assets_for_review` | 9 | 0.889 | 1.000 | 0.926 |
| `inspect_camera_metadata` | 6 | 1.000 | 1.000 | 1.000 |
| `summarize_selected_assets` | 9 | 0.889 | 1.000 | 0.944 |

### Top Misses

- Rank `2` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Archive photos rated 5 or below from Family Holiday autumn 2023
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c04_anchor_holiday_cull_family_holiday_2_autumn_2023`
- Rank `2` target `cleanup_duplicate_shoot/cleanup_duplicates` vs top1 `cleanup_duplicate_shoot/cleanup_duplicates`
  Query: Find previous episode where false positive duplicates were handled for Sony A7C II photos from Yosemite using 0.82 similarity threshold, with metadata inspection revealing 8 false positives among 12 assets.
  Top1 episode: `ep_cleanup_duplicate_shoot_cleanup_duplicate_shoot_c09_anchor_sony_a7c_ii_false_positive_duplicate_yosemite_0_86`
- Rank `2` target `cleanup_duplicate_shoot/cleanup_duplicates` vs top1 `cleanup_duplicate_shoot/cleanup_duplicates`
  Query: Search for episode where Sony A7C II photos from Yosemite were processed with 0.74 similarity threshold, resulting in 25 potential duplicate pairs and quota limit issues during archiving.
  Top1 episode: `ep_cleanup_duplicate_shoot_cleanup_duplicate_shoot_c01_anchor_sony_a7c_ii_false_positive_duplicate_yosemite_0_82`
- Rank `3` target `group_assets_for_review/group_assets` vs top1 `curate_trip_album/curate_album`
  Query: Find previous grouping of Tokyo family photos by camera model including Sony A7IV, Fujifilm X-T5, and iPhone 15 Pro
  Top1 episode: `ep_curate_trip_album_curate_trip_album_c07_anchor_tokyo_street_album_fujifilm_x_t5_tokyo_spring_2023`
- Rank `2` target `summarize_selected_assets/summarize_selection` vs top1 `summarize_selected_assets/summarize_selection`
  Query: Find sunset-themed photos from Beach during summer 2023 with average rating around 4.2
  Top1 episode: `ep_summarize_selected_assets_summarize_selected_assets_c01_near_time_window_beach_sunset_spring_2023`

## Comparison

### Overall

| Report | Recall@1 | Recall@5 | MRR@10 |
| --- | ---: | ---: | ---: |
| `qwen3-baseline-v7-semantic.json` | 0.589 | 1.000 | 0.761 |
| `granite-baseline-v7-semantic.json` | 0.911 | 1.000 | 0.951 |
| `embeddinggemma-baseline-v7-semantic.json` | 0.750 | 1.000 | 0.856 |
| `bge-m3-baseline-v7-semantic.json` | 0.911 | 1.000 | 0.952 |

### By Scenario

- `archive_low_rated_assets`
  - `qwen3-baseline-v7-semantic.json`: `r@1=0.444`, `r@5=1.000`, `mrr=0.615`
  - `granite-baseline-v7-semantic.json`: `r@1=0.889`, `r@5=1.000`, `mrr=0.944`
  - `embeddinggemma-baseline-v7-semantic.json`: `r@1=0.889`, `r@5=1.000`, `mrr=0.944`
  - `bge-m3-baseline-v7-semantic.json`: `r@1=0.889`, `r@5=1.000`, `mrr=0.944`
- `bulk_like_highlights`
  - `qwen3-baseline-v7-semantic.json`: `r@1=0.333`, `r@5=1.000`, `mrr=0.639`
  - `granite-baseline-v7-semantic.json`: `r@1=0.833`, `r@5=1.000`, `mrr=0.917`
  - `embeddinggemma-baseline-v7-semantic.json`: `r@1=0.833`, `r@5=1.000`, `mrr=0.917`
  - `bge-m3-baseline-v7-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
- `cleanup_duplicate_shoot`
  - `qwen3-baseline-v7-semantic.json`: `r@1=0.556`, `r@5=1.000`, `mrr=0.741`
  - `granite-baseline-v7-semantic.json`: `r@1=0.889`, `r@5=1.000`, `mrr=0.944`
  - `embeddinggemma-baseline-v7-semantic.json`: `r@1=0.667`, `r@5=1.000`, `mrr=0.833`
  - `bge-m3-baseline-v7-semantic.json`: `r@1=0.778`, `r@5=1.000`, `mrr=0.889`
- `curate_trip_album`
  - `qwen3-baseline-v7-semantic.json`: `r@1=0.500`, `r@5=1.000`, `mrr=0.750`
  - `granite-baseline-v7-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
  - `embeddinggemma-baseline-v7-semantic.json`: `r@1=0.875`, `r@5=1.000`, `mrr=0.938`
  - `bge-m3-baseline-v7-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
- `group_assets_for_review`
  - `qwen3-baseline-v7-semantic.json`: `r@1=0.889`, `r@5=1.000`, `mrr=0.944`
  - `granite-baseline-v7-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
  - `embeddinggemma-baseline-v7-semantic.json`: `r@1=0.667`, `r@5=1.000`, `mrr=0.769`
  - `bge-m3-baseline-v7-semantic.json`: `r@1=0.889`, `r@5=1.000`, `mrr=0.926`
- `inspect_camera_metadata`
  - `qwen3-baseline-v7-semantic.json`: `r@1=0.333`, `r@5=1.000`, `mrr=0.597`
  - `granite-baseline-v7-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
  - `embeddinggemma-baseline-v7-semantic.json`: `r@1=0.500`, `r@5=1.000`, `mrr=0.722`
  - `bge-m3-baseline-v7-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
- `summarize_selected_assets`
  - `qwen3-baseline-v7-semantic.json`: `r@1=0.889`, `r@5=1.000`, `mrr=0.944`
  - `granite-baseline-v7-semantic.json`: `r@1=0.778`, `r@5=1.000`, `mrr=0.861`
  - `embeddinggemma-baseline-v7-semantic.json`: `r@1=0.778`, `r@5=1.000`, `mrr=0.856`
  - `bge-m3-baseline-v7-semantic.json`: `r@1=0.889`, `r@5=1.000`, `mrr=0.944`
