export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);

    // 1. Входящие вебхуки от Telegram: пересылаем на сервер бота
    if (url.pathname === '/telegram/webhook' || url.pathname.startsWith('/telegram/webhook/')) {
      if (request.method !== 'POST') {
        return new Response('Method Not Allowed', { status: 405 });
      }

      const backendRaw = env.BACKEND_HOST || env.BACKEND_URL;
      if (!backendRaw) {
        return new Response('BACKEND_HOST environment variable is not configured', { status: 500 });
      }

      // Валидация секретного токена на уровне Edge (защита от спуфинга и DoS)
      if (env.WEBHOOK_SECRET) {
        const secretHeader = request.headers.get('x-telegram-bot-api-secret-token');
        if (secretHeader !== env.WEBHOOK_SECRET) {
          return new Response('Unauthorized', { status: 401 });
        }
      }

      // Корректный парсинг адреса бэкенда с поддержкой схем (http/https) и нестандартных портов
      let targetBase;
      try {
        const formatted = backendRaw.includes('://') ? backendRaw : `https://${backendRaw}`;
        targetBase = new URL(formatted);
      } catch {
        return new Response('Invalid BACKEND_HOST configuration', { status: 500 });
      }

      // Защита от бесконечного цикла субзапросов (Subrequest Loop Guard)
      if (targetBase.hostname === url.hostname) {
        return new Response('Loop detected: BACKEND_HOST points to worker', { status: 508 });
      }

      const targetUrl = new URL(url.pathname + url.search, targetBase);

      // Перезаписываем заголовок Host для корректного разрешения vhost (Nginx, Caddy и т.д.)
      const headers = new Headers(request.headers);
      headers.set('Host', targetUrl.host);

      const webhookRequest = new Request(targetUrl.toString(), {
        method: request.method,
        headers: headers,
        body: request.body,
        redirect: 'manual',
      });

      return fetch(webhookRequest);
    }

    // 2. Исходящие запросы от бота к Telegram API: проксируем в api.telegram.org
    url.hostname = 'api.telegram.org';
    url.port = '443';
    url.protocol = 'https:';

    const egressHeaders = new Headers(request.headers);
    egressHeaders.set('Host', 'api.telegram.org');

    const apiRequest = new Request(url.toString(), {
      method: request.method,
      headers: egressHeaders,
      body: request.body,
      redirect: 'follow',
    });

    return fetch(apiRequest);
  },
};
