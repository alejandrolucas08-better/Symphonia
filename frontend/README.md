# Frontend do Symphonia

Interface React + TypeScript + Vite. Para subir toda a aplicacao com PostgreSQL, backend e caixa de e-mail local, siga o [inicio rapido da raiz](../README.md). Este arquivo cobre apenas o trabalho no frontend.

## Desenvolvimento

```powershell
npm ci
npm run dev
```

Execute os comandos em `frontend/`. O Vite abre em `http://localhost:5173` e encaminha `/api` e WebSocket ao backend em `http://127.0.0.1:8080`. Se o backend estiver em outra porta, defina `API_PROXY_TARGET` no processo do Vite. Fora do Docker, variaveis do `.env` da raiz nao sao carregadas automaticamente pelo Go nem pelo Vite.

## Verificacao

```powershell
npm run typecheck
npm run build
npx playwright install chromium
npm run test:e2e
```

Os testes Playwright comuns simulam respostas de API e nao exigem PostgreSQL ou Gemini. O cenario `tests/realtime-live.spec.ts` usa backend real, mas **ainda precisa ser adaptado ao cadastro com confirmacao de e-mail**; nao use seu resultado como validacao da feature nova. Veja [guia do frontend](../docs/frontend.md) para telas, audio, estrutura e testes.

## Audio no navegador

A captura pede permissao do microfone apenas durante o preparo/entrada na chamada. Um `AudioWorklet` converte o audio para PCM16 mono a 16 kHz e o envia em frames binarios de 20 ms; a traducao recebida tambem chega em PCM a 16 kHz. Para depuracao, acrescente `?audioDiagnostics=1` a URL da chamada e consulte `window.__symphoniaAudioDiagnostics` no console. Os contadores mostram frames enviados/recebidos e trilhas ativas, sem guardar amostras de audio.

Microfone em outro dispositivo requer uma origem segura (HTTPS); abrir o Vite pelo IP da rede em HTTP costuma bloquear a captura. Consulte [deploy e teste em dispositivos](../docs/deploy.md).
