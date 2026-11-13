export interface Config {
  /** SQLite file path, or `:memory:`. */
  dbPath: string;
  /** Source of `created_at`; tests inject a deterministic clock. */
  clock?: () => Date;
  /** Enable Fastify's pino request logging. */
  logger?: boolean;
}
