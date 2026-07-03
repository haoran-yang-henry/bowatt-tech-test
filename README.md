# Overview

A research-agent that answers the user's question by combining their uploaded sources (embedding-based retrieval) with external web sources, reasoning and searching over multiple rounds to stream back a cited Markdown answer.

## Architecture

```
Frontend -> Go backend
             |- upload -> extract -> chunk -> embedding -> store (short-term memory)
             |- query  -> agent orchestration:
                          |1. derive focus  <- anchor, threads through 2-4
                          |2. retrieve from stored document embeddings (short-term memory)
                          |3. search loop (LLM-determined, max 3 rounds)
                          |4. synthesize and stream answers
```


## Features

- Upload text files → chunked & embedded into a short-term memory store
- Retrieval-augmented answering (cosine similarity over document chunks)
- Focus anchor to prevent topic drift
- Multi-round, LLM-driven web search (Tavily, max 3 rounds)
- Streamed Markdown answers with source citations
- Config via environment variables






## Setup

The frontend lives in the `frontend/` folder. Install dependencies:

```sh
cd frontend
npm install
```

Run the frontend locally:

```sh
npm run dev
```

Vite will print the local URL, usually `http://localhost:5173/`.

## Backend API

By default, the frontend calls:

```txt
http://localhost:8787
```

Set a different backend URL with:

```sh
VITE_API_BASE_URL=http://localhost:8787 npm run dev
```

Expected endpoints:

- `POST /api/research` — accepts JSON `{ "request": "..." }` and returns a streamed markdown response.
- `POST /api/sources` — accepts multipart form uploads under the repeated field name `files`.


## Backend Setup

The backend is a Go service that reads all LLM config from environment
variables (`LLM_API_KEY`, `LLM_BASE_URL`, `LLM_MODEL`, `SEARCH_API_KEY`, `EMBED_MODEL` ).
Open a **second terminal** for it, separate from the frontend.

### Option A — use a `.env` file (recommended)

From the project root, copy the example file and fill in your real keys:

```sh
cp .env.example .env
```
edit .env and set your keys

Then load it into the environment and start the server:

```sh
set -a; source .env; set +a
cd backend && go run .
```
### Option B — export variables manually

If you prefer not to use a file, export the variables in the terminal
before starting the server:

```sh
cd backend
export LLM_API_KEY="your api key"
export LLM_BASE_URL="https://api.openai.com/v1"
export LLM_MODEL="model name"
export SEARCH_API_KEY="your search api key, tavily recommended"
export EMBED_MODEL="model name"
go run .

```
Either way, the backend listens on http://localhost:8787.

## Run with Docker

The whole app (frontend + backend) is containerized and orchestrated with
Docker Compose. This is the simplest way to run everything with one command.

1. From the project root, create your `.env` and fill in your keys:

   ```sh
   cp .env.example .env
   # edit .env and set LLM_API_KEY, SEARCH_API_KEY, etc.
   ```

2. Build and start both services:

   ```sh
   docker compose up --build
   ```

3. Open **http://localhost:8080** in the browser.

How it works:

- **backend** — multi-stage Go build; reads config from `./.env` at runtime;
  not exposed to the host, only reachable inside the Compose network.
- **frontend** — built with Vite and served by nginx, which also reverse-proxies
  `/api/*` to the backend. The browser talks to a single origin, so no CORS is
  needed and streaming responses are forwarded unbuffered.


# System Design Rationale

## Background Understanding
The task requires designing backend functionality for the provided frontend. The frontend includes a document upload module, a user query module, and a streaming response module.

The goal of this project is to build a research agent that allows users to conduct research on relevant topics through LLM-based reasoning and receive structured research results. In addition, the system should support research based on the user’s own information sources.

Inspired by mainstream research agent workflows, I decided to add a web search module to the backend. This enables the system to support three main research scenarios:

1. Query-based research, such as predicting the winner of the current World Cup. This scenario relies on the web search functionality.
2. Query + own information source research, such as breaking down requirements and assigning tasks based on a requirements document.
3. Query + own information source + web search research, such as using a requirements document to identify similar existing solutions and research how such requirements are currently addressed.


## Design Log

### Design Round 1

1. **Frontend–backend communication:** the test provides two APIs, one for uploading documents and one for the user query. Designed the backend endpoints to receive this information.
2. **File upload:** first implemented the `extract` and `memory` functionality to store the original document text.
3. **Agent:** combined the user query and the original document text into a single context.
4. Passed the context (query + document text) to the LLM to produce the answer.

**Problem:** document context plus the query alone was sometimes not enough to answer the question — especially when external web information was needed. This motivated adding a web search capability.

### Design Round 2

1. Added a `tools` file in the `agent` package implementing the web search logic.
2. Passed the context (query + document text) to the LLM to run multi-round web search and answering.

**Problem:** during the multi-round process, topic drift appeared. I suspected it was because the LLM decides the next round's query from the retrieved results, so I decided to add a **focus** feature to keep every round anchored to the user's query and avoid drift. At the same time, re-reading the test requirements I noticed that document **embedding** needed to be implemented. In Round 1 I had assumed that modern models generally have long context windows, so I chose to inject the raw text directly into the context. I therefore decided to add the document embedding logic in Round 3.

### Design Round 3

1. Added an `embed` file in the `agent` package implementing document chunking, embedding, and cosine-similarity search — enabling similarity-based retrieval of documents against the user query.
2. Set the chunking logic to a (1600, 200) overlapping scheme. More advanced chunking techniques are left as future work.
3. For the multi-round web search, added **goroutines within a single round to run searches concurrently.**



## Future Work

### System

1. Currently only short-term memory is supported: vectorized documents are held in memory awaiting research. In the future, add long-term memory — a personal, long-term knowledge base for the user built on a vector database plus document-management features — to support more complex research tasks.
2. Add more tools. Currently there is only a web search tool; in the future, add MCP tools, a deep-research tool, a memory-export tool, and others.
3. Support more document types. Currently only `.txt` is supported; in the future, support PDF, HTML, PPTX, Word, etc., along with multimodal document understanding — e.g. understanding diagrams inside a PDF requirements document.
4. Add multi-turn conversation and conversation saving, and build long-term conversational memory on top of that.

### Frontend

1. Render the answer: turn the streamed Markdown answer into a more readable, well-formatted display.
2. Support uploading multiple documents and multiple document formats, with the ability to add or remove documents within a session.
3. Support queries that combine images and text.
4. Build a conversational interaction interface.
5. Develop a document-management page — a graphical interface for managing a personal or organizational internal knowledge base.

### Backend

1. Improve the architecture. The current `agent` file is a fixed, pre-orchestrated agent workflow; in the future, move to a modular design that gives users more freedom to interact with the agent and lets them toggle individual capabilities on and off.
2. Improve the chunking logic with a more efficient and more accurate chunking method.
3. Improve retrieval quality. The current approach is single-stage dense retrieval (cosine similarity over embeddings). Future options: hybrid retrieval that combines keyword search (e.g. BM25) with dense vectors, a second-stage cross-encoder reranker to reorder the top candidates, and MMR-style selection to reduce redundancy among retrieved chunks.
4. Optimize the code structure to reduce cost and improve runtime performance, especially in production scenarios.





