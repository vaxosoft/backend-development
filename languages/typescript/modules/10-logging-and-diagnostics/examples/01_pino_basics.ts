import pino from 'pino';

// `base` replaces the default pid/hostname and `timestamp: false` drops `time`, so the output is stable.
const logger = pino({
  level: 'info',
  base: { service: 'task-tracker' },
  timestamp: false,
  // Real services keep the default `err` serializer (it includes the stack); this one keeps output stable.
  serializers: { err: (e: Error) => ({ type: e.name, message: e.message }) },
  redact: { paths: ['password', '*.password', 'req.headers.authorization'], censor: '[REDACTED]' },
});

logger.debug('not printed: below the configured level');
logger.info('server starting');
logger.info({ port: 8080, env: 'dev' }, 'listening');
logger.warn({ retries: 3 }, 'slow dependency');

const requestLogger = logger.child({ request_id: 'req-42', user_id: 7 });
requestLogger.info({ status: 200, duration_ms: 12 }, 'request completed');

logger.info({ user: { email: 'a@example.com', password: 'hunter2' } }, 'user created');
logger.info({ req: { headers: { authorization: 'Bearer abc.def.ghi' } } }, 'incoming');

try {
  throw new Error('connection refused', { cause: 'db:5432' });
} catch (err) {
  logger.error({ err }, 'could not reach database');
}
