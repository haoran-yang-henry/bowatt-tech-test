const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8787'

export type Source = {
  id: string
  name: string
  size: number
  type: string
  uploadedAt: string
}

export type SourceDetail = Source & { content: string }

export async function streamResearch(
  request: string,
  onChunk: (chunk: string) => void,
  options?: { source?: 'auto' | 'docs' | 'web'; sourceIds?: string[]; signal?: AbortSignal },
): Promise<void> {
  const response = await fetch(`${API_BASE_URL}/api/research`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      request,
      source: options?.source ?? 'auto',
      source_ids: options?.sourceIds ?? [],
    }),
    signal: options?.signal,
  })

  if (!response.ok) {
    const text = await response.text()
    throw new Error(text || `Research request failed with ${response.status}`)
  }

  if (!response.body) {
    throw new Error('Research response did not include a stream.')
  }

  const decoder = new TextDecoder()
  const reader = response.body.getReader()

  while (true) {
    const { done, value } = await reader.read()

    if (done) {
      break
    }

    onChunk(decoder.decode(value, { stream: true }))
  }

  const finalChunk = decoder.decode()

  if (finalChunk) {
    onChunk(finalChunk)
  }
}

export type UploadResult = {
  uploaded: Source[]
}

// List all uploaded sources (metadata only).
export async function listSources(): Promise<Source[]> {
  const response = await fetch(`${API_BASE_URL}/api/sources`)
  if (!response.ok) {
    throw new Error((await response.text()) || `Listing sources failed with ${response.status}`)
  }
  const data = (await response.json()) as { sources: Source[] }
  return data.sources ?? []
}

// Upload one or more files; each is extracted, embedded, and appended.
export async function uploadSources(files: FileList): Promise<UploadResult> {
  const formData = new FormData()

  for (let i = 0; i < files.length; i++) {
    const file = files.item(i)

    if (file) {
      formData.append('files', file)
    }
  }

  const response = await fetch(`${API_BASE_URL}/api/sources`, {
    method: 'POST',
    body: formData,
  })

  if (!response.ok) {
    const text = await response.text()
    throw new Error(text || `Source upload failed with ${response.status}`)
  }

  return (await response.json()) as UploadResult
}

// Fetch a single source including its editable text content.
export async function getSource(id: string): Promise<SourceDetail> {
  const response = await fetch(`${API_BASE_URL}/api/sources/${encodeURIComponent(id)}`)
  if (!response.ok) {
    throw new Error((await response.text()) || `Loading source failed with ${response.status}`)
  }
  return (await response.json()) as SourceDetail
}

// Save edited content; the backend re-chunks and re-embeds the document.
export async function updateSource(id: string, content: string): Promise<SourceDetail> {
  const response = await fetch(`${API_BASE_URL}/api/sources/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content }),
  })
  if (!response.ok) {
    throw new Error((await response.text()) || `Saving source failed with ${response.status}`)
  }
  return (await response.json()) as SourceDetail
}

// Remove a source.
export async function deleteSource(id: string): Promise<void> {
  const response = await fetch(`${API_BASE_URL}/api/sources/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
  if (!response.ok && response.status !== 204) {
    throw new Error((await response.text()) || `Deleting source failed with ${response.status}`)
  }
}
