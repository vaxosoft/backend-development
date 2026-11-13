// Imports every pinned dependency once so a broken install fails loudly. Not a lesson.
import fastifyCookie from '@fastify/cookie';
import fastifyCors from '@fastify/cors';
import fastifyMultipart from '@fastify/multipart';
import fastifyStatic from '@fastify/static';
import Database from 'better-sqlite3';
import { parse } from 'csv-parse/sync';
import { stringify } from 'csv-stringify/sync';
import Fastify from 'fastify';
import fp from 'fastify-plugin';
import { SignJWT } from 'jose';
import pino from 'pino';
import { parse as parseYaml } from 'yaml';
import { z } from 'zod';

const db = new Database(':memory:');
const app = Fastify();
const schema = z.object({ ok: z.boolean() });
console.log(
  typeof db.prepare,
  typeof app.inject,
  typeof fp,
  typeof SignJWT,
  typeof pino,
  typeof parse,
  typeof stringify,
  typeof parseYaml,
  typeof fastifyCookie,
  typeof fastifyCors,
  typeof fastifyMultipart,
  typeof fastifyStatic,
  schema.parse({ ok: true }).ok,
);
db.close();
