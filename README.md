# Research Agent Frontend

Small React + TypeScript frontend for the AI Engineer research-agent tech test, bootstrapped with Vite.

The app lets a user:

- enter a research request
- upload source files
- view a streamed markdown response

## Setup

Install dependencies:

```sh
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

## LLM API Setup

For LLM functions, please set up your API keys, base URL and Model:

```sh
export LLM_API_KEY="your api key"
export LLM_BASE_URL="https://api.openai.com/v1"   
export LLM_MODEL="model name" 
export SEARCH_API_KEY="your search api key, tavily recommended" 
```

## Backend Setup

The backend is a Go service that reads all LLM config from environment
variables (`LLM_API_KEY`, `LLM_BASE_URL`, `LLM_MODEL`, `SEARCH_API_KEY`).
Open a **second terminal** for it, separate from the frontend.

### Option A — use a `.env` file (recommended)

Copy the example file and fill in your real keys:

```sh
cd backend
cp .env.example .env

```
edit .env and set your keys

Then load it into the environment and start the server:

```sh
set -a; source .env; set +a
go run .
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