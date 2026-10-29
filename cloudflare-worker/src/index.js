export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);

    // 1. Входящие вебхуки от Telegram: пересылаем на сервер бота
    if (url.pathname.startsWith('/telegram/webhook')) {
      const backendHost = env.BACKEND_HOST;
      if (!backendHost) {
        return new Response('BACKEND_HOST environment variable is not configured', { status: 500 });
      }

      url.hostname = backendHost;
      url.port = '443';
      url.protocol = 'https:';

      const webhookRequest = new Request(url.toString(), {
        method: request.method,
        headers: request.headers,
        body: request.body,
        redirect: 'follow',
      });

      return fetch(webhookRequest);
    }

    // 2. Исходящие запросы от бота к Telegram API: проксируем в api.telegram.org
    url.hostname = 'api.telegram.org';
    url.port = '443';
    url.protocol = 'https:';

    const apiRequest = new Request(url.toString(), {
      method: request.method,
      headers: request.headers,
      body: request.body,
      redirect: 'follow',
    });

    return fetch(apiRequest);
  },
};
