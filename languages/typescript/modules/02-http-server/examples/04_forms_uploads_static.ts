// Form bodies, multipart uploads (streamed, never buffered), static files and streamed responses.

import { createHash } from 'node:crypto';
import { Readable } from 'node:stream';
import { fileURLToPath } from 'node:url';
import fastifyMultipart from '@fastify/multipart';
import fastifyStatic from '@fastify/static';
import Fastify, { type FastifyRequest } from 'fastify';
import { closeOnSignal, listenAddress } from './_shared.ts';

const app = Fastify();

// Fastify only parses JSON and text out of the box: add a parser for HTML forms.
app.addContentTypeParser(
  'application/x-www-form-urlencoded',
  { parseAs: 'string' },
  async (_req: FastifyRequest, body: string) => Object.fromEntries(new URLSearchParams(body)),
);

app.post('/login', async (req) => ({ form: req.body }));

await app.register(fastifyMultipart, { limits: { fileSize: 1024 * 1024, files: 1 } });

app.post('/upload', async (req, reply) => {
  const upload = await req.file();
  if (!upload) return reply.code(400).send({ error: 'multipart field "file" is required' });

  // Consume the stream chunk by chunk: memory use stays flat for any file size.
  const hash = createHash('sha256');
  let size = 0;
  for await (const chunk of upload.file) {
    hash.update(chunk);
    size += chunk.length;
  }
  // The size limit silently truncates the stream: you must check the flag yourself.
  if (upload.file.truncated) return reply.code(413).send({ error: 'file exceeds 1 MiB' });
  return reply
    .code(201)
    .send({ name: upload.filename, type: upload.mimetype, size, sha256: hash.digest('hex') });
});

// Static files are served from a directory; path traversal like /static/../x is rejected.
await app.register(fastifyStatic, {
  root: fileURLToPath(new URL('./public/', import.meta.url)),
  prefix: '/static/',
});

// A readable stream is sent chunked, so the client sees data before the response is complete.
app.get<{ Querystring: { count?: string } }>('/numbers', async (req, reply) => {
  const count = Math.min(Number(req.query.count ?? '5') || 5, 1000);
  async function* lines(): AsyncGenerator<string> {
    for (let n = 1; n <= count; n++) yield `${n}\n`;
  }
  return reply.type('text/plain; charset=utf-8').send(Readable.from(lines()));
});

const { host, port } = listenAddress();
await app.listen({ host, port });
console.log('listening');

closeOnSignal(() => app.close());
