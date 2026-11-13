// Making requests with the global fetch: GET, POST with a JSON body, headers and a JSON round-trip.
import { startServer } from './_shared.ts';

const server = await startServer(async (req, res) => {
  const chunks: Buffer[] = [];
  for await (const chunk of req) {
    chunks.push(chunk as Buffer);
  }
  const body = Buffer.concat(chunks).toString('utf8');
  res.setHeader('Content-Type', 'application/json');
  res.end(
    JSON.stringify({
      method: req.method,
      path: req.url,
      accept: req.headers.accept,
      contentType: req.headers['content-type'] ?? null,
      body: body === '' ? null : JSON.parse(body),
    }),
  );
});

try {
  const getRes = await fetch(`${server.url}/hello?name=ada`, {
    headers: { Accept: 'application/json' },
  });
  console.log(getRes.status, getRes.statusText, getRes.ok);
  console.log(getRes.headers.get('content-type'));
  console.log(await getRes.json());

  const url = new URL('/search', server.url);
  url.searchParams.set('q', 'a b&c');
  const postRes = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title: 'Write docs', tags: ['docs'] }),
  });
  const echoed: unknown = await postRes.json();
  console.log(echoed);
} finally {
  await server.close();
}
