export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    url.hostname = 'api.telegram.org';
    url.port = '443';
    url.protocol = 'https:';

    const newRequest = new Request(url.toString(), {
      method: request.method,
      headers: request.headers,
      body: request.body,
      redirect: 'follow',
    });

    return fetch(newRequest);
  },
};
