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
  _baseUrl: string,
  _path: string,
  _query: Record<string, string | number | boolean | undefined> = {},
): string {
  throw new Error('TODO');
}

/**
 * GET `url` and return the parsed JSON body.
 * Sends `Accept: application/json` and, when `token` is given, `Authorization: Bearer <token>`.
 * A non-2xx response must throw an `HttpError` carrying the status and the raw body text.
 */
export async function getJson(_url: string, _options: { token?: string } = {}): Promise<unknown> {
  throw new Error('TODO');
}
