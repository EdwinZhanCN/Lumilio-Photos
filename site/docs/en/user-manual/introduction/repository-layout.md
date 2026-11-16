# Repository Layout

Every Repository is an ordinary folder. Your originals stay where you put them; Lumilio keeps its own data in one hidden folder, `.lumilio/`, sorted by what the data is and how it may be treated.

```text
<Repository>/
├── .lumiliorepo          identity and settings of the Repository
├── .lumiliorepo.lock     held while a Server uses the Repository
├── inbox/                where uploads and cloud imports land
├── <your folders>/       your originals
└── .lumilio/
    ├── user/             what people authored: Studio edits, asset sidecars
    ├── trash/            deleted originals, kept until the Trash expires them
    ├── cache/            thumbnails, web versions, waveforms, face crops
    ├── work/             uploads and writes in progress
    ├── quarantine/       bytes kept for your decision
    └── logs/             this Repository's operation and error logs
```

Lumilio may also create short-lived files named `.lumilio-tmp-…` next to `.lumiliorepo`; they disappear on their own and are never media.

## What each folder holds

| Folder | Holds | Safe to delete? | Back up? |
| --- | --- | --- | --- |
| `user/` | Studio edits and asset sidecars. People made them; nothing can rebuild them. | No | Yes |
| `trash/` | Originals you deleted, until the Trash's retention period expires them. | Only through the Trash | Yes |
| `cache/` | Thumbnails, web-playable versions, waveforms, and face crops. Lumilio rebuilds anything missing. | Yes, the whole folder, while the Server is stopped | No (it carries a `CACHEDIR.TAG`) |
| `work/` | Uploads and writes in progress. | No | No |
| `quarantine/` | Failed uploads and anything a start or a reopened Repository could not place. | Only from the Recovery inventory | Yes |
| `logs/` | Operation and error logs, rotated by size. | Yes | Optional |

Anything else under `.lumilio/`, such as folders an older or newer build wrote, is kept unchanged and shown as unknown. Lumilio never deletes it.

## What Lumilio does on its own

- **At every start**, Lumilio recreates any missing folder above. Anything an interrupted run left in `work/` moves into `quarantine/recovery/`. The exceptions are an upload that is still waiting to be imported and an upload your browser can resume. Nothing is deleted. The moved files appear in the Repository's **Recovery inventory** on the Storage page, where you can export or delete each one.
- **While running**, an upload that nothing will finish is removed once it has been idle for an hour. An upload that cannot be assembled moves to `quarantine/uploads/` instead.
- **The cache** is cleaned of files no Asset refers to any more.
- **Logs** rotate at 50 MB and keep five compressed generations for up to 14 days.

## Backups

Back up the whole Repository folder, or exclude `.lumilio/cache/` and `.lumilio/work/` to save space. Backup tools that honor `CACHEDIR.TAG` skip `cache/` automatically. Never restore only part of `.lumilio/` over a running Server.

Do not move files into `.lumilio/` or edit them by hand; see [Storage Locations and Repositories](./repositories).
