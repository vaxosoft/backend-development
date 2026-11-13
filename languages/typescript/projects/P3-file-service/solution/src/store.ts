import { createHash, randomBytes } from 'node:crypto';
import { createReadStream, createWriteStream } from 'node:fs';
import { mkdir, readFile, rename, rm, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { type Readable, Transform } from 'node:stream';
import { pipeline } from 'node:stream/promises';

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

  // Only generated ids reach these helpers, so no client input is part of a path.
  private dataPath = (id: string) => join(this.dir, `${id}.bin`);
  private metaPath = (id: string) => join(this.dir, `${id}.json`);

  /**
   * Streams `source` to disk under a generated id while computing size and
   * SHA-256. A failed or truncated upload must leave nothing behind.
   */
  async save(source: Readable, options: SaveOptions): Promise<FileRecord> {
    const id = randomBytes(16).toString('hex');
    const partial = join(this.dir, `${id}.part`);
    const hash = createHash('sha256');
    let size = 0;
    const measure = new Transform({
      transform(chunk: Buffer, _encoding, callback) {
        hash.update(chunk);
        size += chunk.length;
        callback(null, chunk);
      },
    });

    try {
      await pipeline(source, measure, createWriteStream(partial, { flags: 'wx' }));
    } catch (error) {
      await rm(partial, { force: true });
      throw error;
    }

    const record: FileRecord = {
      id,
      name: options.name,
      size,
      sha256: hash.digest('hex'),
      content_type: options.contentType,
    };
    await rename(partial, this.dataPath(id));
    await writeFile(this.metaPath(id), JSON.stringify(record));
    return record;
  }

  async find(id: string): Promise<FileRecord | undefined> {
    if (!ID_PATTERN.test(id)) return undefined;
    try {
      return JSON.parse(await readFile(this.metaPath(id), 'utf8')) as FileRecord;
    } catch (error) {
      if (error instanceof Error && 'code' in error && error.code === 'ENOENT') return undefined;
      throw error;
    }
  }

  /** A stream over the stored bytes; call only for ids that `find` returned. */
  open(id: string): Readable {
    return createReadStream(this.dataPath(id));
  }

  /** Returns false when the id is unknown. */
  async remove(id: string): Promise<boolean> {
    if (!(await this.find(id))) return false;
    await rm(this.metaPath(id), { force: true });
    await rm(this.dataPath(id), { force: true });
    return true;
  }
}
