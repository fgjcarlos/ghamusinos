// Client for the training blocks API (fase 2.4, #317 · docs: openspec
// proposal). Same shape as lib/api/races.ts, with the listAll walker
// for the block option lists.

import { ApiError } from './types';

export interface BlockInput {
  name: string;
  start_date: string;
  end_date: string;
  /** null unlinks the target race on an update. */
  race_id: string | null;
  notes: string;
}

export interface Block extends BlockInput {
  id: string;
  created_at: string;
  updated_at: string;
}

interface BlockPage {
  data: Block[];
  total: number;
  limit: number;
  offset: number;
  has_next: boolean;
}

const base = '/api/v1/blocks';
const blockPath = (id: string) => `${base}/${encodeURIComponent(id)}`;

async function request<T>(token: string, path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Authorization', `Bearer ${token}`);
  if (init.body) headers.set('Content-Type', 'application/json');
  const res = await fetch(path, { ...init, headers });
  if (!res.ok) throw new ApiError(res.status, `HTTP ${res.status}`);
  return res.status === 204 ? (undefined as T) : (res.json() as Promise<T>);
}

const options = (signal?: AbortSignal): RequestInit => (signal ? { signal } : {});

export const listBlocks = (token: string, limit = 20, offset = 0, signal?: AbortSignal) =>
  request<BlockPage>(token, `${base}?limit=${limit}&offset=${offset}`, options(signal));

const saveBlock = (token: string, input: BlockInput, id?: string) =>
  request<Block>(token, id ? blockPath(id) : base, {
    method: id ? 'PUT' : 'POST',
    body: JSON.stringify(input),
  });
export { saveBlock };

export const deleteBlock = (token: string, id: string) =>
  request<void>(token, blockPath(id), { method: 'DELETE' });

/** Walks every page; stops on a short or final page so a bad total cannot loop. */
async function listAll<T>(
  page: (offset: number, signal: AbortSignal) => Promise<{ data: T[]; has_next: boolean }>,
  signal: AbortSignal,
): Promise<T[]> {
  const items: T[] = [];
  for (;;) {
    const result = await page(items.length, signal);
    items.push(...result.data);
    if (signal.aborted || !result.has_next || !result.data.length) return items;
  }
}

export const listAllBlocks = (token: string, signal: AbortSignal) =>
  listAll((offset, signal) => listBlocks(token, 100, offset, signal), signal);
