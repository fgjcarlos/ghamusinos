/**
 * AI analysis API client — Fase 1.5 (issue #17, T7).
 *
 * Mirrors the backend contracts served under /api/v1/ai:
 *
 *   - GET  /ai/status    → AIStatusResponse   (global state + consent)
 *   - POST /ai/analyses  → AIAnalysis          (202, pending analysis)
 *   - GET  /ai/analyses  → AIAnalysisListResponse (the user's analyses)
 *   - GET  /ai/analyses?kind=&scope= → AIAnalysis (one analysis, 404 if missing)
 *
 * Every function takes the bearer token as a parameter (no auth layer is
 * installed yet — same convention as ./profile.ts and ./dashboard.ts).
 * Failures are raised as ApiError so the containers can map them to a
 * message.
 *
 * The client never sends a mixed body: an activity request only carries
 * `activity_id` and a window request only carries `window_start` and
 * `window_days`, because the API rejects a body that mixes them.
 */

import { ApiError, type ProblemDetailShape } from './types';

const BASE_URL = '/api/v1';

/**
 * makeAuthHeader creates the Authorization header with the bearer token.
 */
function makeAuthHeader(token: string): HeadersInit {
  return {
    Authorization: `Bearer ${token}`,
  };
}

/**
 * parseProblemDetail returns a human-readable detail string for an
 * RFC 9457 problem+json response, preferring `detail` and falling back to
 * `title`. Returns null when the body is not a recognised problem detail.
 */
function parseProblemDetail(body: unknown): string | null {
  if (!body || typeof body !== 'object') return null;
  const p = body as ProblemDetailShape;
  if (typeof p.detail === 'string' && p.detail.length > 0) return p.detail;
  if (typeof p.title === 'string' && p.title.length > 0) return p.title;
  return null;
}

/**
 * jsonResponse resolves with the parsed JSON body on 2xx, or throws an
 * ApiError populated from the response status and (for problem+json
 * bodies) the field-level `errors` extension member.
 */
async function jsonResponse<T>(res: Response): Promise<T> {
  if (res.ok) {
    return res.json() as Promise<T>;
  }
  let message = `HTTP ${res.status}: ${res.statusText}`;
  let fields: Record<string, string> | undefined;
  const contentType = res.headers.get('content-type') ?? '';
  if (
    contentType.includes('application/problem+json') ||
    contentType.includes('application/json')
  ) {
    try {
      const body = await res.json();
      const detail = parseProblemDetail(body);
      if (detail) message = detail;
      if (body && typeof body === 'object') {
        const errs = (body as ProblemDetailShape).errors;
        if (errs && typeof errs === 'object') {
          fields = errs;
        }
      }
    } catch {
      // body was not JSON — keep the legacy message.
    }
  }
  throw new ApiError(res.status, message, fields);
}

/** The three kinds of analysis the API accepts. */
export type AIAnalysisKind = 'activity' | 'weekly' | 'load';

/** The lifecycle of an analysis row. */
export type AIAnalysisStatus = 'pending' | 'completed' | 'failed';

/** The global state of the optional AI feature. */
export type AIStatus = 'disabled' | 'unavailable' | 'ready';

/**
 * AIStatusResponse is the body of GET /api/v1/ai/status.
 *
 * `provider` is the configured provider name and it is null when the AI is
 * not available. The API key and the configured model never appear here.
 */
export interface AIStatusResponse {
  enabled: boolean;
  available: boolean;
  status: AIStatus;
  provider: string | null;
  /** Whether the CURRENT USER accepted the data transfer notice. */
  consented: boolean;
}

/**
 * AIAnalysisOutput is the validated interpretation of a completed
 * analysis. It carries no metrics: the provider cannot report a number it
 * never received.
 */
export interface AIAnalysisOutput {
  summary: string;
  observations: string[];
  cautions: string[];
}

/**
 * AIAnalysis is one analysis of the authenticated user.
 *
 * `output` is only present when the analysis completed; `error_code` and
 * `error_message` are only present when it failed, and they are the safe
 * code and message written by the application.
 */
export interface AIAnalysis {
  id: string;
  kind: AIAnalysisKind;
  scope: string;
  status: AIAnalysisStatus;
  provider: string | null;
  model: string | null;
  output: AIAnalysisOutput | null;
  error_code: string | null;
  error_message: string | null;
  created_at: string;
  updated_at: string;
}

/**
 * AIAnalysisListResponse is the body of the listing: the analyses of the
 * authenticated user, most recent first, and the limit that was applied.
 */
export interface AIAnalysisListResponse {
  data: AIAnalysis[];
  limit: number;
}

/**
 * AIAnalysisRequest is the body of POST /api/v1/ai/analyses.
 *
 * `activity_id` belongs to an activity request; `window_start` and
 * `window_days` belong to a weekly or load request.
 */
export interface AIAnalysisRequest {
  kind: AIAnalysisKind;
  activity_id?: string;
  /** YYYY-MM-DD. Defaults to today (UTC) on the server. */
  window_start?: string;
  window_days?: number;
}

/**
 * analysisRequestBody builds the request body without mixing the fields of
 * an activity request with the fields of a window request.
 */
function analysisRequestBody(request: AIAnalysisRequest): Record<string, unknown> {
  const body: Record<string, unknown> = { kind: request.kind };
  if (request.kind === 'activity') {
    if (request.activity_id !== undefined) body.activity_id = request.activity_id;
    return body;
  }
  if (request.window_start !== undefined) body.window_start = request.window_start;
  if (request.window_days !== undefined) body.window_days = request.window_days;
  return body;
}

/**
 * fetchAIStatus reports whether the optional AI can be used right now:
 * the global state, the provider name (when available) and whether the
 * current user has consented.
 *
 * 200 → AIStatusResponse. Throws ApiError on non-2xx.
 */
export async function fetchAIStatus(token: string): Promise<AIStatusResponse> {
  const res = await fetch(`${BASE_URL}/ai/status`, {
    method: 'GET',
    headers: makeAuthHeader(token),
  });
  return jsonResponse<AIStatusResponse>(res);
}

/**
 * createAIAnalysis requests one analysis on demand.
 *
 * 202 → AIAnalysis in "pending" state; the job finishes it later.
 * 403 → ApiError when the user has not consented.
 * 404 → ApiError when the activity does not exist or is not the user's.
 * 422 → ApiError with the field-level `errors` of the problem detail.
 * 503 → ApiError when the AI is off, not available or the queue is down.
 */
export async function createAIAnalysis(
  token: string,
  request: AIAnalysisRequest,
): Promise<AIAnalysis> {
  const res = await fetch(`${BASE_URL}/ai/analyses`, {
    method: 'POST',
    headers: { ...makeAuthHeader(token), 'Content-Type': 'application/json' },
    body: JSON.stringify(analysisRequestBody(request)),
  });
  return jsonResponse<AIAnalysis>(res);
}

/**
 * listAIAnalyses lists the analyses of the authenticated user, most recent
 * first. The server applies a bounded limit (20 by default, 100 at most).
 */
export async function listAIAnalyses(
  token: string,
  options?: { limit?: number },
): Promise<AIAnalysisListResponse> {
  const params = new URLSearchParams();
  if (options?.limit !== undefined) params.set('limit', String(options.limit));
  const query = params.toString();
  const res = await fetch(`${BASE_URL}/ai/analyses${query ? `?${query}` : ''}`, {
    method: 'GET',
    headers: makeAuthHeader(token),
  });
  return jsonResponse<AIAnalysisListResponse>(res);
}

/**
 * fetchAIAnalysis reads one analysis of the authenticated user by kind and
 * scope. Throws ApiError with status 404 when it does not exist.
 */
export async function fetchAIAnalysis(
  token: string,
  kind: AIAnalysisKind,
  scope: string,
): Promise<AIAnalysis> {
  const params = new URLSearchParams({ kind, scope });
  const res = await fetch(`${BASE_URL}/ai/analyses?${params.toString()}`, {
    method: 'GET',
    headers: makeAuthHeader(token),
  });
  return jsonResponse<AIAnalysis>(res);
}
