export class HttpError extends Error {
  readonly status: number;
  readonly body: string;

  constructor(status: number, body: string) {
    super(`HTTP ${status}`);
    this.name = 'HttpError';
    this.status = status;
    this.body = body;
  }
}

/**
 * Joins `baseUrl` and `path` (exactly one slash between them, any base path is kept) and appends
 * the query. `undefined` values are skipped; values are URL-encoded and stringified.
 */
export function buildUrl(
  baseUrl: string,
  path: string,
  query: Record<string, string | number | boolean | undefined> = {},
): string {
  const url = new URL(`${baseUrl.replace(/\/+$/, '')}/${path.replace(/^\/+/, '')}`);
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      url.searchParams.set(key, String(value));
    }
  }
  return url.toString();
}

/**
 * GET `url` and return the parsed JSON body.
 * Sends `Accept: application/json` and, when `token` is given, `Authorization: Bearer <token>`.
 * A non-2xx response must throw an `HttpError` carrying the status and the raw body text.
 */
export async function getJson(url: string, options: { token?: string } = {}): Promise<unknown> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (options.token !== undefined) {
    headers.Authorization = `Bearer ${options.token}`;
  }
  const res = await fetch(url, { headers });
  if (!res.ok) {
    throw new HttpError(res.status, await res.text());
  }
  return res.json();
}
