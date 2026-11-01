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

      // Валидация секретного токена на уровне Edge с constant-time сравнением (защита от timing attacks)
      if (env.WEBHOOK_SECRET) {
        const secretHeader = request.headers.get('x-telegram-bot-api-secret-token') || '';
        const encoder = new TextEncoder();
        const a = encoder.encode(secretHeader);
        const b = encoder.encode(env.WEBHOOK_SECRET);
        if (a.byteLength !== b.byteLength || !crypto.subtle.timingSafeEqual(a, b)) {
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

      // Буферизуем тело вебхука для исключения Transfer-Encoding: chunked и явной передачи Content-Length
      const bodyBuffer = await request.arrayBuffer();
      const headers = new Headers(request.headers);
      headers.set('Host', targetUrl.host);
      headers.set('Content-Length', bodyBuffer.byteLength.toString());

      const webhookRequest = new Request(targetUrl.toString(), {
        method: request.method,
        headers: headers,
        body: bodyBuffer,
        redirect: 'manual',
      });

      try {
        return await fetch(webhookRequest);
      } catch (err) {
        return new Response(`Bad Gateway: ${err.message}`, { status: 502 });
      }
    }

    // 2. Исходящие запросы от бота к Telegram API: проксируем в api.telegram.org
    // Ограничение путей strictly to /bot${env.BOT_TOKEN}/* для исключения открытого прокси
    if (env.BOT_TOKEN) {
      const allowedPrefix = `/bot${env.BOT_TOKEN}/`;
      if (!url.pathname.startsWith(allowedPrefix)) {
        return new Response('Forbidden: invalid bot path', { status: 403 });
      }
    } else if (!/^\/bot[^/]+\//.test(url.pathname)) {
      return new Response('Forbidden: bot token path required', { status: 403 });
    }

    url.hostname = 'api.telegram.org';
    url.port = '443';
    url.protocol = 'https:';

    const egressHeaders = new Headers(request.headers);
    egressHeaders.set('Host', 'api.telegram.org');

    const hasBody = request.method !== 'GET' && request.method !== 'HEAD';
    const apiRequestInit = {
      method: request.method,
      headers: egressHeaders,
      redirect: 'follow',
    };
    if (hasBody) {
      apiRequestInit.body = request.body;
    }

    const apiRequest = new Request(url.toString(), apiRequestInit);

    try {
      return await fetch(apiRequest);
    } catch (err) {
      return new Response(`Bad Gateway: ${err.message}`, { status: 502 });
    }
  },
};
