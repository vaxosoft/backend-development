// A validated, typed config object built from env with zod: one schema = runtime check + static type.
import { z } from 'zod';

const envSchema = z.object({
  PORT: z.coerce.number().int().min(1).max(65535).default(8080),
  LOG_LEVEL: z.enum(['debug', 'info', 'warn', 'error']).default('info'),
  ENV: z.enum(['development', 'production']).default('development'),
  // z.coerce.boolean() would turn "false" into true; stringbool understands real words.
  DEBUG: z.stringbool().default(false),
  DATABASE_URL: z.url(),
});

const configSchema = envSchema.transform((env) => ({
  port: env.PORT,
  logLevel: env.LOG_LEVEL,
  environment: env.ENV,
  debug: env.DEBUG,
  databaseUrl: env.DATABASE_URL,
}));
type Config = z.output<typeof configSchema>;

function loadConfig(env: Record<string, string | undefined>): Config {
  const result = configSchema.safeParse(env);
  if (!result.success) {
    const lines = result.error.issues.map((i) => `  ${i.path.join('.')}: ${i.message}`);
    throw new Error(`invalid configuration:\n${lines.join('\n')}`);
  }
  return result.data;
}

console.log(loadConfig({ DATABASE_URL: 'sqlite:///tracker.db' }));
console.log(
  loadConfig({
    PORT: '9090',
    LOG_LEVEL: 'debug',
    ENV: 'production',
    DEBUG: 'off',
    DATABASE_URL: 'postgres://app@db.internal/tracker',
  }),
);

console.log('coerce.boolean("false") =', z.coerce.boolean().parse('false'));
console.log('stringbool("false")     =', z.stringbool().parse('false'));

try {
  loadConfig({ PORT: '99999', LOG_LEVEL: 'verbose', DEBUG: 'maybe' });
} catch (err) {
  console.log((err as Error).message);
}
