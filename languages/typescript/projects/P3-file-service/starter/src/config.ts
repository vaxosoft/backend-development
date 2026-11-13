export const DEFAULT_MAX_FILE_SIZE = 5 * 1024 * 1024;

export interface Config {
  /** Directory that holds every stored file (created if missing). */
  storageDir: string;
  /** Largest accepted upload in bytes; defaults to 5 MiB. */
  maxFileSize?: number;
}
