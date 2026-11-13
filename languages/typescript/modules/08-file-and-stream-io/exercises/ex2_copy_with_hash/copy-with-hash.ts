export interface CopyResult {
  bytes: number;
  sha256: string;
}

/**
 * Copies `src` to `dest` as a stream (never read the whole file into memory) and computes the
 * SHA-256 of the copied bytes on the way, as lowercase hex.
 * If anything fails (for example `src` does not exist) the returned promise rejects with that error
 * and `dest` must not exist afterwards.
 */
export async function copyWithSha256(_src: string, _dest: string): Promise<CopyResult> {
  throw new Error('TODO');
}
