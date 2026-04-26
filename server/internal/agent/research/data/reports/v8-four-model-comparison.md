# Benchmark Analysis

- Generated at: `2026-04-26T21:28:26.205665+00:00`
- Report count: `4`

## qwen3-baseline-v8-semantic.json

- Collection: `agent_episodic_memory_qwen3_1024_v8_semantic`
- Model: `qwen3-embedding:0.6b` @ `1024d`
- Queries: `89`
- Overall: `recall@1=0.685`, `recall@5=0.876`, `mrr@10=0.762`
- Numeric slots / with number: `n=78`, `r@1=0.654`, `mrr=0.741`
- Numeric slots / without number: `n=11`, `r@1=0.909`, `mrr=0.909`
- Error topology: `misses=28`, `same_scenario_top1=18`, `same_intent_top1=18`

### By Scenario

| Scenario | N | Recall@1 | Recall@5 | MRR |
| --- | ---: | ---: | ---: | ---: |
| `archive_low_rated_assets` | 8 | 0.875 | 1.000 | 0.938 |
| `audit_import_batch` | 8 | 1.000 | 1.000 | 1.000 |
| `bulk_like_highlights` | 6 | 0.500 | 0.833 | 0.667 |
| `cleanup_duplicate_shoot` | 9 | 0.778 | 1.000 | 0.889 |
| `curate_trip_album` | 9 | 0.000 | 0.111 | 0.041 |
| `group_assets_for_review` | 8 | 1.000 | 1.000 | 1.000 |
| `inspect_camera_metadata` | 8 | 0.500 | 1.000 | 0.677 |
| `prepare_client_delivery` | 9 | 0.889 | 1.000 | 0.944 |
| `recover_lost_album_assets` | 9 | 0.556 | 0.778 | 0.633 |
| `resolve_metadata_mismatch` | 7 | 0.714 | 1.000 | 0.798 |
| `summarize_selected_assets` | 8 | 0.750 | 1.000 | 0.844 |

### Top Misses

- Rank `2` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Which episode archived 7 assets with rating <= 1 from Tokyo Collection summer 2025?
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c09_anchor_tokyo_collection_tokyo_2_summer_2025`
- Rank `6` target `bulk_like_highlights/bulk_like` vs top1 `bulk_like_highlights/bulk_like`
  Query: Which episode performed bulk like on Wedding Highlights album with rating >= 4, no location filter, and succeeded?
  Top1 episode: `ep_bulk_like_highlights_bulk_like_highlights_c09_near_location_wedding_highlights_false_hawaii_4`
- Rank `3` target `bulk_like_highlights/bulk_like` vs top1 `bulk_like_highlights/bulk_like`
  Query: Which episode liked assets from Paris location in Wedding Highlights album with rating >= 4?
  Top1 episode: `ep_bulk_like_highlights_bulk_like_highlights_c09_anchor_wedding_highlights_false_tokyo_4`
- Rank `2` target `bulk_like_highlights/bulk_like` vs top1 `bulk_like_highlights/bulk_like`
  Query: Which episode liked 80 assets in Wedding Highlights album with rating >= 4 in Tokyo?
  Top1 episode: `ep_bulk_like_highlights_bulk_like_highlights_c01_near_location_wedding_highlights_false_paris_4`
- Rank `2` target `cleanup_duplicate_shoot/cleanup_duplicates` vs top1 `cleanup_duplicate_shoot/cleanup_duplicates`
  Query: Find the portrait session cleanup where 3 near-duplicate burst photos from Canon EOS R5 were archived after inspection, with similarity threshold 0.76.
  Top1 episode: `ep_cleanup_duplicate_shoot_cleanup_duplicate_shoot_c03_near_camera_model_fujifilm_x_t5_near_duplicate_burst_portrait_session_0_76`

## granite-baseline-v8-semantic.json

- Collection: `agent_episodic_memory_granite_768_v8_semantic`
- Model: `granite-embedding:278m` @ `768d`
- Queries: `89`
- Overall: `recall@1=0.933`, `recall@5=1.000`, `mrr@10=0.963`
- Numeric slots / with number: `n=78`, `r@1=0.923`, `mrr=0.957`
- Numeric slots / without number: `n=11`, `r@1=1.000`, `mrr=1.000`
- Error topology: `misses=6`, `same_scenario_top1=5`, `same_intent_top1=5`

### By Scenario

| Scenario | N | Recall@1 | Recall@5 | MRR |
| --- | ---: | ---: | ---: | ---: |
| `archive_low_rated_assets` | 8 | 0.625 | 1.000 | 0.792 |
| `audit_import_batch` | 8 | 1.000 | 1.000 | 1.000 |
| `bulk_like_highlights` | 6 | 0.833 | 1.000 | 0.917 |
| `cleanup_duplicate_shoot` | 9 | 1.000 | 1.000 | 1.000 |
| `curate_trip_album` | 9 | 0.889 | 1.000 | 0.926 |
| `group_assets_for_review` | 8 | 1.000 | 1.000 | 1.000 |
| `inspect_camera_metadata` | 8 | 1.000 | 1.000 | 1.000 |
| `prepare_client_delivery` | 9 | 1.000 | 1.000 | 1.000 |
| `recover_lost_album_assets` | 9 | 0.889 | 1.000 | 0.944 |
| `resolve_metadata_mismatch` | 7 | 1.000 | 1.000 | 1.000 |
| `summarize_selected_assets` | 8 | 1.000 | 1.000 | 1.000 |

### Top Misses

- Rank `3` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Show me the episode that archived 18 assets with rating <= 2 from Tokyo Collection summer 2025.
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c05_anchor_tokyo_collection_tokyo_1_spring_2023`
- Rank `2` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Which episode archived 7 assets with rating <= 1 from Tokyo Collection summer 2025?
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c05_near_time_window_tokyo_collection_tokyo_1_winter_2022`
- Rank `2` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Find the episode that archived 12 assets with rating <= 2 from Tokyo Collection winter 2022.
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c05_near_time_window_tokyo_collection_tokyo_1_winter_2022`
- Rank `2` target `bulk_like_highlights/bulk_like` vs top1 `bulk_like_highlights/bulk_like`
  Query: Which episode performed bulk like on Wedding Highlights album with rating >= 4, no location filter, and succeeded?
  Top1 episode: `ep_bulk_like_highlights_bulk_like_highlights_c01_near_location_wedding_highlights_false_paris_4`
- Rank `3` target `curate_trip_album/curate_album` vs top1 `recover_lost_album_assets/recover_album_membership`
  Query: Retrieve the episode that created 'Yosemite Keeps' from Yosemite autumn 2024 Sony A7III assets.
  Top1 episode: `ep_recover_lost_album_assets_recover_lost_album_assets_c07_anchor_tokyo_review_yosemite_archive_conflict_yosemite_keeps_autumn_2023`

## embeddinggemma-baseline-v8-semantic.json

- Collection: `agent_episodic_memory_embeddinggemma_768_v8_semantic`
- Model: `embeddinggemma:latest` @ `768d`
- Queries: `89`
- Overall: `recall@1=0.685`, `recall@5=1.000`, `mrr@10=0.815`
- Numeric slots / with number: `n=78`, `r@1=0.679`, `mrr=0.811`
- Numeric slots / without number: `n=11`, `r@1=0.727`, `mrr=0.848`
- Error topology: `misses=28`, `same_scenario_top1=27`, `same_intent_top1=27`

### By Scenario

| Scenario | N | Recall@1 | Recall@5 | MRR |
| --- | ---: | ---: | ---: | ---: |
| `archive_low_rated_assets` | 8 | 0.875 | 1.000 | 0.938 |
| `audit_import_batch` | 8 | 0.375 | 1.000 | 0.546 |
| `bulk_like_highlights` | 6 | 0.500 | 1.000 | 0.750 |
| `cleanup_duplicate_shoot` | 9 | 0.667 | 1.000 | 0.744 |
| `curate_trip_album` | 9 | 0.667 | 1.000 | 0.815 |
| `group_assets_for_review` | 8 | 0.625 | 1.000 | 0.792 |
| `inspect_camera_metadata` | 8 | 1.000 | 1.000 | 1.000 |
| `prepare_client_delivery` | 9 | 0.889 | 1.000 | 0.944 |
| `recover_lost_album_assets` | 9 | 0.556 | 1.000 | 0.778 |
| `resolve_metadata_mismatch` | 7 | 0.714 | 1.000 | 0.857 |
| `summarize_selected_assets` | 8 | 0.625 | 1.000 | 0.792 |

### Top Misses

- Rank `2` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Which episode archived 12 low-rated assets from Tokyo Collection winter 2022?
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c09_near_time_window_tokyo_collection_tokyo_2_winter_2022`
- Rank `3` target `audit_import_batch/audit_import` vs top1 `audit_import_batch/audit_import`
  Query: Which episode audited a Tokyo iPhone 15 Pro HEIC mobile upload batch, keeping original duplicates, with 100 assets inspected?
  Top1 episode: `ep_audit_import_batch_audit_import_batch_c07_anchor_iphone_15_pro_keep_original_heic_drone_card_tokyo`
- Rank `4` target `audit_import_batch/audit_import` vs top1 `audit_import_batch/audit_import`
  Query: Which episode audited a Tokyo iPhone 15 Pro HEIC mobile upload batch, keeping geotagged duplicates, with 80 assets inspected?
  Top1 episode: `ep_audit_import_batch_audit_import_batch_c07_anchor_iphone_15_pro_keep_original_heic_drone_card_tokyo`
- Rank `5` target `audit_import_batch/audit_import` vs top1 `audit_import_batch/audit_import`
  Query: Which episode audited a Tokyo iPhone 15 Pro DNG mobile upload batch, keeping originals, with 50 assets and a metadata error during duplicate detection?
  Top1 episode: `ep_audit_import_batch_audit_import_batch_c07_anchor_iphone_15_pro_keep_original_heic_drone_card_tokyo`
- Rank `4` target `audit_import_batch/audit_import` vs top1 `audit_import_batch/audit_import`
  Query: Which episode audited a Safari SD card A import batch with Fujifilm X-T5 RAW NEF, keeping geotagged duplicates, with 80 assets and a corrupted index error?
  Top1 episode: `ep_audit_import_batch_audit_import_batch_c07_near_location_iphone_15_pro_keep_original_heic_drone_card_paris`

## bge-m3-baseline-v8-semantic.json

- Collection: `agent_episodic_memory_bge_m3_1024_v8_semantic`
- Model: `bge-m3:latest` @ `1024d`
- Queries: `89`
- Overall: `recall@1=0.831`, `recall@5=1.000`, `mrr@10=0.901`
- Numeric slots / with number: `n=78`, `r@1=0.833`, `mrr=0.902`
- Numeric slots / without number: `n=11`, `r@1=0.818`, `mrr=0.894`
- Error topology: `misses=15`, `same_scenario_top1=15`, `same_intent_top1=15`

### By Scenario

| Scenario | N | Recall@1 | Recall@5 | MRR |
| --- | ---: | ---: | ---: | ---: |
| `archive_low_rated_assets` | 8 | 0.625 | 1.000 | 0.771 |
| `audit_import_batch` | 8 | 0.750 | 1.000 | 0.844 |
| `bulk_like_highlights` | 6 | 0.833 | 1.000 | 0.875 |
| `cleanup_duplicate_shoot` | 9 | 0.889 | 1.000 | 0.944 |
| `curate_trip_album` | 9 | 0.778 | 1.000 | 0.870 |
| `group_assets_for_review` | 8 | 0.750 | 1.000 | 0.854 |
| `inspect_camera_metadata` | 8 | 1.000 | 1.000 | 1.000 |
| `prepare_client_delivery` | 9 | 1.000 | 1.000 | 1.000 |
| `recover_lost_album_assets` | 9 | 0.778 | 1.000 | 0.889 |
| `resolve_metadata_mismatch` | 7 | 0.714 | 1.000 | 0.833 |
| `summarize_selected_assets` | 8 | 1.000 | 1.000 | 1.000 |

### Top Misses

- Rank `3` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Show me the episode that archived 18 assets with rating <= 2 from Tokyo Collection summer 2025.
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c05_anchor_tokyo_collection_tokyo_1_spring_2023`
- Rank `3` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Which episode archived 7 assets with rating <= 1 from Tokyo Collection summer 2025?
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c05_near_time_window_tokyo_collection_tokyo_1_winter_2022`
- Rank `2` target `archive_low_rated_assets/bulk_archive` vs top1 `archive_low_rated_assets/bulk_archive`
  Query: Find the episode that archived 12 assets with rating <= 2 from Tokyo Collection winter 2022.
  Top1 episode: `ep_archive_low_rated_assets_archive_low_rated_assets_c05_near_time_window_tokyo_collection_tokyo_1_winter_2022`
- Rank `4` target `audit_import_batch/audit_import` vs top1 `audit_import_batch/audit_import`
  Query: Which episode audited a Tokyo drone card import batch with iPhone 15 Pro HEIC files, keeping original duplicates, inspecting 50 assets?
  Top1 episode: `ep_audit_import_batch_audit_import_batch_c03_near_duplicate_policy_iphone_15_pro_keep_geotagged_heic_mobile_upload_tokyo`
- Rank `2` target `audit_import_batch/audit_import` vs top1 `audit_import_batch/audit_import`
  Query: Which episode audited a Paris drone card import batch with iPhone 15 Pro HEIC files, keeping original duplicates, with 45 assets and a policy mismatch error?
  Top1 episode: `ep_audit_import_batch_audit_import_batch_c03_near_duplicate_policy_iphone_15_pro_keep_geotagged_heic_mobile_upload_tokyo`

## Comparison

### Overall

| Report | Recall@1 | Recall@5 | MRR@10 |
| --- | ---: | ---: | ---: |
| `qwen3-baseline-v8-semantic.json` | 0.685 | 0.876 | 0.762 |
| `granite-baseline-v8-semantic.json` | 0.933 | 1.000 | 0.963 |
| `embeddinggemma-baseline-v8-semantic.json` | 0.685 | 1.000 | 0.815 |
| `bge-m3-baseline-v8-semantic.json` | 0.831 | 1.000 | 0.901 |

### By Scenario

- `archive_low_rated_assets`
  - `qwen3-baseline-v8-semantic.json`: `r@1=0.875`, `r@5=1.000`, `mrr=0.938`
  - `granite-baseline-v8-semantic.json`: `r@1=0.625`, `r@5=1.000`, `mrr=0.792`
  - `embeddinggemma-baseline-v8-semantic.json`: `r@1=0.875`, `r@5=1.000`, `mrr=0.938`
  - `bge-m3-baseline-v8-semantic.json`: `r@1=0.625`, `r@5=1.000`, `mrr=0.771`
- `audit_import_batch`
  - `qwen3-baseline-v8-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
  - `granite-baseline-v8-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
  - `embeddinggemma-baseline-v8-semantic.json`: `r@1=0.375`, `r@5=1.000`, `mrr=0.546`
  - `bge-m3-baseline-v8-semantic.json`: `r@1=0.750`, `r@5=1.000`, `mrr=0.844`
- `bulk_like_highlights`
  - `qwen3-baseline-v8-semantic.json`: `r@1=0.500`, `r@5=0.833`, `mrr=0.667`
  - `granite-baseline-v8-semantic.json`: `r@1=0.833`, `r@5=1.000`, `mrr=0.917`
  - `embeddinggemma-baseline-v8-semantic.json`: `r@1=0.500`, `r@5=1.000`, `mrr=0.750`
  - `bge-m3-baseline-v8-semantic.json`: `r@1=0.833`, `r@5=1.000`, `mrr=0.875`
- `cleanup_duplicate_shoot`
  - `qwen3-baseline-v8-semantic.json`: `r@1=0.778`, `r@5=1.000`, `mrr=0.889`
  - `granite-baseline-v8-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
  - `embeddinggemma-baseline-v8-semantic.json`: `r@1=0.667`, `r@5=1.000`, `mrr=0.744`
  - `bge-m3-baseline-v8-semantic.json`: `r@1=0.889`, `r@5=1.000`, `mrr=0.944`
- `curate_trip_album`
  - `qwen3-baseline-v8-semantic.json`: `r@1=0.000`, `r@5=0.111`, `mrr=0.041`
  - `granite-baseline-v8-semantic.json`: `r@1=0.889`, `r@5=1.000`, `mrr=0.926`
  - `embeddinggemma-baseline-v8-semantic.json`: `r@1=0.667`, `r@5=1.000`, `mrr=0.815`
  - `bge-m3-baseline-v8-semantic.json`: `r@1=0.778`, `r@5=1.000`, `mrr=0.870`
- `group_assets_for_review`
  - `qwen3-baseline-v8-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
  - `granite-baseline-v8-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
  - `embeddinggemma-baseline-v8-semantic.json`: `r@1=0.625`, `r@5=1.000`, `mrr=0.792`
  - `bge-m3-baseline-v8-semantic.json`: `r@1=0.750`, `r@5=1.000`, `mrr=0.854`
- `inspect_camera_metadata`
  - `qwen3-baseline-v8-semantic.json`: `r@1=0.500`, `r@5=1.000`, `mrr=0.677`
  - `granite-baseline-v8-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
  - `embeddinggemma-baseline-v8-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
  - `bge-m3-baseline-v8-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
- `prepare_client_delivery`
  - `qwen3-baseline-v8-semantic.json`: `r@1=0.889`, `r@5=1.000`, `mrr=0.944`
  - `granite-baseline-v8-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
  - `embeddinggemma-baseline-v8-semantic.json`: `r@1=0.889`, `r@5=1.000`, `mrr=0.944`
  - `bge-m3-baseline-v8-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
- `recover_lost_album_assets`
  - `qwen3-baseline-v8-semantic.json`: `r@1=0.556`, `r@5=0.778`, `mrr=0.633`
  - `granite-baseline-v8-semantic.json`: `r@1=0.889`, `r@5=1.000`, `mrr=0.944`
  - `embeddinggemma-baseline-v8-semantic.json`: `r@1=0.556`, `r@5=1.000`, `mrr=0.778`
  - `bge-m3-baseline-v8-semantic.json`: `r@1=0.778`, `r@5=1.000`, `mrr=0.889`
- `resolve_metadata_mismatch`
  - `qwen3-baseline-v8-semantic.json`: `r@1=0.714`, `r@5=1.000`, `mrr=0.798`
  - `granite-baseline-v8-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
  - `embeddinggemma-baseline-v8-semantic.json`: `r@1=0.714`, `r@5=1.000`, `mrr=0.857`
  - `bge-m3-baseline-v8-semantic.json`: `r@1=0.714`, `r@5=1.000`, `mrr=0.833`
- `summarize_selected_assets`
  - `qwen3-baseline-v8-semantic.json`: `r@1=0.750`, `r@5=1.000`, `mrr=0.844`
  - `granite-baseline-v8-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
  - `embeddinggemma-baseline-v8-semantic.json`: `r@1=0.625`, `r@5=1.000`, `mrr=0.792`
  - `bge-m3-baseline-v8-semantic.json`: `r@1=1.000`, `r@5=1.000`, `mrr=1.000`
