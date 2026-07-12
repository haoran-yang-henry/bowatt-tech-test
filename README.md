# encrypt-research-agent

**Exploring what AI agents can do when knowledge, memory, and inference are encrypted.**



## The idea

Today's agents assume plaintext everywhere: documents sit unencrypted in vector
stores, agent memory is readable by whoever runs the host, and every prompt is
visible to the inference provider. That assumption breaks the moment agents
touch anything confidential.

`encrypt-agent` explores the alternative, one step at a time. The guiding metric
across the whole roadmap is simple:

> **At every phase, shrink the set of parties who can ever see plaintext.**

Starting point: a central knowledge base where **all knowledge is encrypted at
rest**, and **key permissions decide who reads which subset**. Humans and agents
hold different keys — a master key reads everything; a scoped key reads only
what it is authorized for; an agent gets a short-lived, revocable key that opens
exactly the slice it needs. End state: agents whose retrieval, memory, and even
LLM inference happen inside an encrypted envelope.

See **[ROADMAP.md](ROADMAP.md)** for the full plan and the honest technical
tensions (RAG vs. encryption, embedding leakage, why FHE isn't ready for LLMs).

## Structure

This is a monorepo. Each component lives in its own top-level directory with its
own README, so it can be run and reasoned about independently.

```
encrypt-agent/
├── research-agent/    ← PoC #1 — the plaintext baseline (live)
├── knowledge-vault/   ← next: encrypted knowledge base + key hierarchy (planned)
└── ROADMAP.md         ← where this is going
```

## Components

| Component | Status | What it does |
|-----------|--------|--------------|
| [**research-agent**](research-agent/) | ✅ PoC | Answers a question by combining uploaded sources (embedding-based retrieval) with live web search, reasoning over multiple plan–act–evaluate rounds to stream back a cited Markdown answer. The plaintext baseline every later phase hardens. |
| **knowledge-vault** | 🚧 planned | Encrypted-at-rest knowledge store with envelope encryption (KEK→DEK) and key-scoped access: one ciphertext store, different keys see different subsets. |

## Getting started

The ecosystem currently ships its first PoC:

```sh
cd research-agent
# follow research-agent/README.md (Docker Compose or local dev)
```

## research-agent HTTP API

The research-agent backend exposes a small API under `/api`. **Sources** are the
uploaded documents the agent can research over; the frontend's file manager
drives their full lifecycle (list, upload, view, edit, delete) and picks which
sources take part in a research run.

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/sources` | List all uploaded sources (metadata only). |
| `POST` | `/api/sources` | Upload one or more files (multipart field `files`). Each file is extracted, chunked, embedded, and **appended** to the store. |
| `GET` | `/api/sources/{id}` | Fetch one source including its text content (for the view/edit pane). |
| `PUT` | `/api/sources/{id}` | Save edited content; the document is re-chunked and re-embedded. |
| `DELETE` | `/api/sources/{id}` | Remove a source. |
| `POST` | `/api/research` | Run the agent over the selected sources; streams the Markdown answer. |

**Source object** (returned by the `/api/sources` endpoints):

```json
{
  "id": "b3f9c2a1",
  "name": "General_Equipment_URS.txt",
  "size": 18324,
  "type": "text/plain",
  "uploadedAt": "2026-07-12T09:30:00Z"
}
```

- `GET /api/sources` → `{ "sources": [Source, …] }`
- `POST /api/sources` → `{ "uploaded": [Source, …] }`
- `GET /api/sources/{id}` → `Source` plus a `"content"` field
- `PUT /api/sources/{id}` with body `{ "content": "…" }` → the updated `Source`
- `DELETE /api/sources/{id}` → `204 No Content`
- `POST /api/research` with body:

  ```json
  {
    "request": "What are the safety requirements?",
    "source": "auto",
    "source_ids": ["b3f9c2a1"]
  }
  ```

  `source` is `auto` (default) | `docs` | `web`. `source_ids` limits document
  retrieval to the selected sources; omitted or empty means *all* uploaded
  sources. The response streams `text/plain` Markdown chunks.

Selection is deliberately client-side state: which sources join a run is a
property of the *request*, not of the server, so the backend stays stateless
and needs no session tracking.

### Frontend file manager

The `<FileManager>` component (`frontend/src/FileManager.tsx`) is the single
place that talks to the source endpoints:

- **View** — on mount it calls `GET /api/sources` and renders the list; each
  row's *View / Edit* button lazy-loads the content via `GET /api/sources/{id}`.
- **Add / change** — the *Upload* form posts to `POST /api/sources`; the editor
  pane's *Save* button sends `PUT /api/sources/{id}`; *Delete* sends
  `DELETE /api/sources/{id}`. Each mutation re-fetches the list.
- **Select** — a checkbox per row toggles membership in a lifted `Set<string>`
  of selected IDs; `App.tsx` passes `Array.from(selectedIds)` as `source_ids`
  when it calls `POST /api/research`. No selection means "research all sources".

### Running with Docker

Nothing in the Docker setup has to change for the file manager — the new routes
all live under the existing `/api` prefix that nginx already reverse-proxies,
and the container images are unchanged.

```sh
# from research-agent/ (the directory holding docker-compose.yml)
cp .env.example .env          # set LLM_API_KEY, SEARCH_API_KEY, etc.
docker compose up --build     # frontend on http://localhost:8080
```

How the pieces fit:

- **backend** (`backend/Dockerfile`) builds the Go binary and serves the API on
  `:8787`, exposed only inside the compose network. Uploaded sources live in the
  in-memory store, so they reset when the container restarts — acceptable for
  the PoC. To persist them, back `store.Memory` with a volume-mounted directory
  or a database and add that volume in `docker-compose.yml`.
- **frontend** (`frontend/Dockerfile`) builds the Vite app with an empty
  `VITE_API_BASE_URL`, so the browser uses relative `/api/...` paths. nginx
  (`frontend/nginx.conf`) serves the static SPA and reverse-proxies `/api/` to
  `backend:8787` — one origin, so no CORS is needed and the streaming response
  passes through untouched (`proxy_buffering off`).

Because all file-manager traffic is same-origin JSON/multipart under `/api/`,
the only Docker-relevant caveat is upload size: nginx defaults to a 1 MB body
limit. If you upload files larger than that, add `client_max_body_size 32m;`
inside the `location /api/` block in `frontend/nginx.conf` to match the
backend's 32 MB `ParseMultipartForm` limit.

## License

[MIT](LICENSE)
