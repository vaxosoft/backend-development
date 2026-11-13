import { mkdir } from 'node:fs/promises';
import type { Readable } from 'node:stream';

/** Metadata of a stored file; this is also the JSON returned by POST /files. */
export interface FileRecord {
  id: string;
  name: string;
  size: number;
  sha256: string;
  content_type: string;
}

export interface SaveOptions {
  /** Already sanitized display name. */
  name: string;
  contentType: string;
}

/** Ids are 32 lowercase hex characters; anything else can never name a stored file. */
export const ID_PATTERN = /^[0-9a-f]{32}$/;

export class FileStore {
  readonly dir: string;

  constructor(dir: string) {
    this.dir = dir;
  }

  async init(): Promise<void> {
    await mkdir(this.dir, { recursive: true });
  }

  /**
   * Streams `source` to disk under a generated id while computing size and
   * SHA-256. A failed or truncated upload must leave nothing behind.
   */
  async save(_source: Readable, _options: SaveOptions): Promise<FileRecord> {
    throw new Error('TODO');
  }

  async find(_id: string): Promise<FileRecord | undefined> {
    throw new Error('TODO');
  }

  /** A stream over the stored bytes; call only for ids that `find` returned. */
  open(_id: string): Readable {
    throw new Error('TODO');
  }

  /** Returns false when the id is unknown. */
  async remove(_id: string): Promise<boolean> {
    throw new Error('TODO');
  }
}
