import type { FastifyInstance } from 'fastify';
import { badRequest, HttpError, notFound } from './errors.ts';
import { sanitizeFilename } from './filename.ts';
import type { FileStore } from './store.ts';

const MEDIA_TYPE = /^[a-z0-9][\w.+-]*\/[a-z0-9][\w.+-]*$/i;

/** Matches an If-None-Match header against our ETag (weak validators and `*` included). */
function etagMatches(header: string | undefined, etag: string): boolean {
  if (!header) return false;
  return header
    .split(',')
    .map((candidate) => candidate.trim().replace(/^W\//, ''))
    .some((candidate) => candidate === '*' || candidate === etag);
}

/** ASCII fallback plus an RFC 5987 `filename*` for everything else. */
function contentDisposition(name: string): string {
  const ascii = name.replace(/[^\x20-\x7e]/g, '_');
  if (ascii === name) return `attachment; filename="${name}"`;
  const encoded = encodeURIComponent(name).replace(
    /['()*]/g,
    (char) => `%${char.charCodeAt(0).toString(16).toUpperCase()}`,
  );
  return `attachment; filename="${ascii}"; filename*=UTF-8''${encoded}`;
}

export function registerFileRoutes(app: FastifyInstance, store: FileStore): void {
  app.post('/files', async (req, reply) => {
    if (!req.isMultipart()) {
      throw new HttpError(415, 'unsupported_media_type', 'expected multipart/form-data');
    }
    const part = await req.file();
    if (part?.fieldname !== 'file') throw badRequest('multipart field "file" is required');

    const record = await store.save(part.file, {
      name: sanitizeFilename(part.filename),
      contentType: MEDIA_TYPE.test(part.mimetype) ? part.mimetype : 'application/octet-stream',
    });
    // Past the size limit the parser silently truncates the stream; it does not fail it.
    if (part.file.truncated) {
      await store.remove(record.id);
      throw new HttpError(413, 'payload_too_large', 'file exceeds the maximum size');
    }
    return reply.code(201).header('location', `/files/${record.id}`).send(record);
  });

  app.get<{ Params: { id: string } }>('/files/:id', async (req, reply) => {
    const record = await store.find(req.params.id);
    if (!record) throw notFound();

    const etag = `"${record.sha256}"`;
    reply.header('etag', etag);
    if (etagMatches(req.headers['if-none-match'], etag)) return reply.code(304).send();

    return reply
      .header('content-type', record.content_type)
      .header('content-length', record.size)
      .header('content-disposition', contentDisposition(record.name))
      .header('x-content-type-options', 'nosniff')
      .send(store.open(record.id));
  });

  app.delete<{ Params: { id: string } }>('/files/:id', async (req, reply) => {
    if (!(await store.remove(req.params.id))) throw notFound();
    return reply.code(204).send();
  });
}
