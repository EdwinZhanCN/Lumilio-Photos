# `server/config` Dependency Map

How the `config` package relates to the rest of the server.

## Design observations

- **`config` is a leaf package.** It imports only the standard library and
  `github.com/pelletier/go-toml/v2`. It depends on *no* internal package, so the
  dependency arrows below all point **into** `config`, never out of it. This keeps
  it free of import cycles and safe to import from anywhere.
- **`AppConfig` is the single typed runtime contract.** It is assembled once at a
  host boundary, then consumed read-only everywhere else.
- **Two host entry points build it, never the consumers:**
  - `cmd/main.go` — web/docker host. Collects process env + TOML via
    `LoadAppConfigWithOptions(LoadOptions{...})`.
  - `desktop/supervisor` — desktop host. Builds typed `DesktopParams` →
    `NewDesktopConfig`, deliberately skipping generic `SERVER_*`/`DB_*` env overrides.
- **`app.Run` is the composition root.** It receives the whole `AppConfig`, then
  fans out *narrow sub-structs* to each component. Components depend on the smallest
  slice they need (e.g. `db` only sees `DatabaseConfig`, `auth_service` only sees
  `AuthConfig`) — interface-segregation applied to config, not the god-object.

## Diagram

```mermaid
flowchart TB
    subgraph hosts["Host boundaries (build config)"]
        main["cmd/main.go<br/>web/docker host"]
        supervisor["desktop/supervisor<br/>desktop host"]
    end

    subgraph configpkg["server/config (leaf package)"]
        direction TB
        loaders["LoadAppConfigWithOptions / LoadOptions / ProcessEnv<br/>NewDesktopConfig / DesktopParams"]
        appcfg["AppConfig<br/>(single typed contract)"]
        subgraph slices["Sub-config slices"]
            db_c["DatabaseConfig"]
            srv_c["ServerConfig"]
            log_c["LoggingConfig"]
            stor_c["StorageConfig"]
            llm_c["LLMConfig"]
            ml_c["MLConfig"]
            scan_c["RepositoryScanConfig"]
            geo_c["GeocodingConfig"]
            auth_c["AuthConfig"]
            trans_c["TranscodeConfig"]
            lumen_c["LumenConfig"]
            tools_c["ToolsConfig"]
        end
        loaders --> appcfg
        appcfg --> slices
    end

    main -->|"env + TOML"| loaders
    supervisor -->|"typed params"| loaders

    appcfg ==>|"whole config"| app["app.Run<br/>(composition root)"]

    %% app fans out narrow slices to components
    app --> db_c
    app --> auth_c
    app --> ml_c
    app --> llm_c
    app --> stor_c
    app --> scan_c
    app --> geo_c
    app --> trans_c
    app --> tools_c

    %% consumers depend only on their slice
    db_c --> dbpkg["internal/db<br/>(db, dsn, migration)"]
    db_c --> setup["service/setup_service"]
    auth_c --> authsvc["service/auth_service"]
    ml_c --> indexing["service/indexing_service"]
    ml_c --> mlprov["queue/ml_config_provider"]
    llm_c --> agent["agent/core/agent_service"]
    llm_c --> chat["llm/chat_model"]
    scan_c --> scanner["storage/scanner"]
    geo_c --> location["service/location_service"]
    trans_c --> proc["processors/asset_processor"]
    trans_c --> vid["processors/video_helpers"]
    tools_c --> proc

    %% settings_service reads several slices (read model)
    auth_c --> settings["service/settings_service"]
    llm_c --> settings
    ml_c --> settings
    stor_c --> settings

    classDef leaf fill:#e6f3ff,stroke:#3b82f6;
    classDef host fill:#fef3c7,stroke:#d97706;
    classDef root fill:#dcfce7,stroke:#16a34a;
    class configpkg,loaders,appcfg,slices,db_c,srv_c,log_c,stor_c,llm_c,ml_c,scan_c,geo_c,auth_c,trans_c,lumen_c,tools_c leaf;
    class main,supervisor host;
    class app root;
```

## Component → slice table

| Slice | Consumers |
|---|---|
| `DatabaseConfig` | `internal/db` (db, dsn, migration), `service/setup_service` |
| `AuthConfig` | `service/auth_service`, `service/settings_service` |
| `MLConfig` | `service/indexing_service`, `queue/ml_config_provider`, `service/settings_service` |
| `LLMConfig` | `agent/core/agent_service`, `llm/chat_model`, `service/settings_service` |
| `StorageConfig` | `service/settings_service` |
| `RepositoryScanConfig` | `storage/scanner` |
| `GeocodingConfig` | `service/location_service` |
| `TranscodeConfig` | `processors/asset_processor`, `processors/video_helpers` |
| `ToolsConfig` | `processors/asset_processor` |
| `ServerConfig` / `LoggingConfig` / `LumenConfig` | consumed inside `app.Run` wiring |
