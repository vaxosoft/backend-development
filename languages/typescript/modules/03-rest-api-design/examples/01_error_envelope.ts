// One error envelope for every failure: status codes, setErrorHandler and setNotFoundHandler.
import Fastify from 'fastify';
import { closeOnSignal, listenAddress } from '../../02-http-server/examples/_shared.ts';
import { HttpError, notFound, registerErrorHandling } from './_shared.ts';

const app = Fastify({ bodyLimit: 256 }); // tiny limit so the 413 case is easy to trigger
registerErrorHandling(app);

app.get<{ Params: { id: string } }>('/tasks/:id', async (req) => {
  throw notFound(`task ${req.params.id}`); // thrown errors reach the error handler, even from async code
});

app.get('/conflict', async () => {
  throw new HttpError(409, 'conflict', 'slug already taken');
});

app.get('/boom', async () => {
  throw new Error('connection to db failed: password=hunter2'); // must never reach the client
});

app.post('/echo', async (req) => ({ received: req.body }));

const { host, port } = listenAddress();
await app.listen({ host, port });
console.log('listening');

closeOnSignal(() => app.close());
