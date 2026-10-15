# Frontend

O frontend usa React, TypeScript, React Router e Vite. O backend fornece autenticacao, salas, chat em tempo real e traducao. A landing page publica usa frases de exemplo; as telas autenticadas usam dados do backend. Veja o [inicio rapido](../README.md) para subir o conjunto.

## Onde encontrar cada parte

```text
frontend/
├── src/
│   ├── components/
│   │   ├── auth/       # Rota protegida e entrada Google
│   │   ├── call/       # Participantes, convite, chat, transcricao e controles
│   │   ├── layout/     # Cabecalhos e layout de autenticacao
│   │   └── ui/         # Controles compartilhados
│   ├── contexts/       # Sessao e preferencias compartilhadas
│   ├── data/           # Idiomas e conteudo de exemplo da landing
│   ├── hooks/          # Auth, audio, WebSocket e preferencias de movimento
│   ├── pages/          # Telas e fluxos de conta/chamada
│   ├── routes/         # Rotas e titulos da pagina
│   ├── services/       # Cliente HTTP, socket e processamento de audio
│   ├── types/          # Contratos HTTP/WebSocket
│   ├── App.tsx
│   └── main.tsx
├── public/             # AudioWorklet servido diretamente
└── tests/              # Playwright
```

O cliente HTTP em `src/services/api.ts` usa `/api` na mesma origem. Em desenvolvimento, o proxy do Vite (`vite.config.ts`) encaminha HTTP e WebSocket ao Go; em producao, o Go serve o frontend compilado e a API no mesmo dominio. Evite definir `VITE_API_BASE_URL` sem necessidade, pois isso muda a origem vista pelo navegador.

## Telas e navegacao

| Caminho | Papel |
| --- | --- |
| `/` | Apresentacao publica; exemplo local de conversa |
| `/register`, `/login` | Cadastro e entrada |
| `/verify-email`, `/resend-verification` | Confirmacao e reenvio de e-mail |
| `/forgot-password`, `/reset-password` | Recuperacao de senha |
| `/auth/google/complete` | Conclusao do retorno OAuth |
| `/account/password` | Alteracao de senha e vinculacao Google; exige sessao |
| `/home` | Criar ou entrar em uma sala; exige sessao |
| `/call/:id/setup` | Idiomas, dispositivo de entrada e modo somente escuta |
| `/call/:id` | Chamada, chat, transcricoes e controles |

`AppRoutes.tsx` e a fonte para as rotas atuais. Criar uma sala aceita nome opcional, cujo padrao e `Meeting`; o backend retorna o codigo canonico usado nas URLs. O convite mostra codigo e QR code. A sala admite no maximo duas pessoas.

O contexto de autenticacao guarda o JWT no `localStorage` do perfil. O frontend consulta `/api/session` para restaurar a sessao ao reabrir a pagina. Isso nao sincroniza login entre navegadores/perfis. Cadastro com senha nao entra automaticamente: a pessoa abre o link recebido por e-mail. Veja [contas e Google OAuth](auth.md).

## Chamada, chat e audio

- A tela de preparo pede permissao do microfone por acao explicita ou ao entrar na chamada. Streams temporarios sao encerrados; se a permissao falhar, e possivel tentar novamente ou entrar somente para ouvir.
- `getUserMedia` e `AudioWorklet` convertem a taxa real do contexto de audio para PCM16 mono de 16 kHz. O cliente envia pacotes de 320 amostras (20 ms), com cabecalho binario `SA` de 24 bytes. A traducao recebida do backend e reproduzida com um pequeno agendamento para absorver variacao de rede. Consulte o [formato exato dos frames](websocket.md).
- Uma sala sem outro participante mostra estado de espera; transcricoes so aparecem apos eventos reais. O audio do microfone vai direto ao colega quando os idiomas coincidem, ou passa pelo provider de traducao quando diferem.
- O chat envia o original aos dois usuarios. O destinatario recebe a traducao depois, quando disponivel, e pode ver o original. `Enter` envia; `Shift+Enter` insere linha. O limite e de 1000 caracteres Unicode. A aba mostra contagem de nao lidas enquanto o chat esta fechado.
- **Nao ha historico persistido do chat:** mensagens, traducoes recebidas e contagem de nao lidas se perdem ao recarregar ou abrir outra aba. A caixa de texto em edicao permanece apenas na aba atual durante uma desconexao temporaria.

Para diagnostico de audio, abra `/call/<codigo>?audioDiagnostics=1` e consulte `window.__symphoniaAudioDiagnostics` no console. O objeto expoe contadores de frames de entrada/saida e trilhas vivas; nao expoe nem persiste as amostras. O microfone exige contexto seguro: `localhost` funciona para desenvolvimento, mas um IP da LAN em HTTP normalmente nao funciona em outro dispositivo.

## Interface e acessibilidade

IBM Plex Mono vem do pacote Fontsource e os icones sao Lucide. As telas usam labels, foco visivel, mensagens de erro, atalho para o conteudo principal e nomes acessiveis em controles por icone. Mudancas de rota levam o foco ao conteudo; preferencias de movimento reduzido diminuem animacoes. O layout de chamada se reorganiza em mobile e mantem controles alcancaveis.

## Testes

Em `frontend/`:

```powershell
npm ci
npm run typecheck
npm run build
npx playwright install chromium
npm run test:e2e
```

O Playwright inicia o Vite quando necessario e executa projetos desktop, tablet e mobile em Chromium. A maioria dos testes intercepta a API; portanto, verifica interface e contratos esperados, **nao** o banco ou o Gemini real. Screenshots e traces de falha vao para `frontend/test-results/` (ignorado pelo Git). Teste manualmente em navegadores e aparelhos reais, especialmente Samsung Internet e permissao de microfone.

Os testes `realtime-live.spec.ts` e `deploy.spec.ts` sao opcionais e exigem backend/ambiente reais. **Atencao:** `realtime-live.spec.ts` ainda espera que `/api/register` retorne um JWT imediatamente; apos a verificacao obrigatoria de e-mail, esse fluxo precisa ser atualizado antes de voltar a passar. O teste `deploy.spec.ts` verifica rotas e assets da imagem, mas nao substitui o teste de autenticacao real. Nao aponte testes que criam usuarios/salas para o banco de producao.
