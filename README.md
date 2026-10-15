# Symphonia

O Symphonia conecta duas pessoas que falam idiomas diferentes em uma chamada de audio pelo navegador. O projeto inclui contas, salas com ate dois participantes, chat, transcricao e traducao de voz. A traducao real depende de credenciais e acesso ao Gemini; o modo `mock` serve apenas para testar o fluxo.

## Comece aqui

**Caminho recomendado para desenvolvimento:** Docker Compose. Voce precisa de Docker com Compose e de um navegador recente. Node.js e Go so sao necessarios se quiser executar os servicos fora dos containers.

1. Na raiz do repositorio, crie o arquivo local de configuracao:

   ```powershell
   if (-not (Test-Path .env)) { Copy-Item .env.example .env }
   ```

   Em macOS/Linux, use `test -e .env || cp .env.example .env`. **Nao sobrescreva um `.env` existente**: ele pode conter seus segredos e configuracoes locais. O arquivo `.env` nao deve ser versionado.

2. Gere um `JWT_SECRET` exclusivo, com pelo menos 32 bytes, e coloque o resultado em `.env`:

   ```powershell
   $bytes = [byte[]]::new(48)
   [System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
   [Convert]::ToBase64String($bytes)
   ```

   Os valores de PostgreSQL em `.env.example` sao **somente para desenvolvimento**. Se mudar `POSTGRES_PASSWORD`, atualize tambem o `DATABASE_URL` usado fora do Compose. Dentro do Compose, a URL do backend e montada com as variaveis `POSTGRES_*`.

3. Suba os quatro servicos:

   ```powershell
   docker compose up --build
   ```

4. Abra [a aplicacao](http://localhost:5173), crie uma conta e confirme o e-mail na [caixa local do Mailpit](http://localhost:8025). O Mailpit captura mensagens de desenvolvimento; nao as envia para fora da maquina. Para testar duas pessoas, use contas diferentes em navegadores ou perfis diferentes.

| Servico | Endereco local | Para que serve |
| --- | --- | --- |
| Frontend | `http://localhost:5173` | Interface e proxy de `/api`/WebSocket para o backend |
| Backend | `http://localhost:8080` | API; `GET /healthz` consulta tambem o banco |
| PostgreSQL | `localhost:5432` | Dados de usuarios, salas e participantes |
| Mailpit | `http://localhost:8025` | E-mails de confirmacao e recuperacao |

As migrations SQL sao aplicadas pelo backend na inicializacao. O volume do PostgreSQL persiste entre `docker compose down` e `docker compose up`. Para parar, use `Ctrl+C` ou `docker compose down`; **nao use `down -v` se quiser preservar os dados locais**.

### Executar Go e Vite fora do Docker

Requer Node.js 22+, npm e Go 1.25+. Mantenha PostgreSQL e Mailpit em containers:

```powershell
docker compose up -d postgres mailpit
```

O Go e o Vite **nao carregam `.env` automaticamente** nesse modo. Em um terminal PowerShell, exporte as variaveis necessarias e inicie o backend:

```powershell
$env:DATABASE_URL = 'postgres://symphonia:symphonia@localhost:5432/symphonia?sslmode=disable'
$env:JWT_SECRET = '<o-segredo-que-voce-gerou>'
$env:APP_PUBLIC_URL = 'http://localhost:5173'
$env:SMTP_HOST = 'localhost'
$env:SMTP_PORT = '1025'
$env:EMAIL_FROM = 'Symphonia <no-reply@symphonia.local>'
$env:TRANSLATION_PROVIDER = 'mock'
cd backend
go run ./cmd/server
```

Em outro terminal:

```powershell
cd frontend
npm ci
npm run dev
```

O Vite encaminha `/api` e WebSocket para `http://127.0.0.1:8080` por padrao. Se alterar a porta do backend, defina `API_PROXY_TARGET` no terminal do Vite. Para esse uso, deixe `VITE_API_BASE_URL` vazio, preservando a mesma origem no navegador.

## O que ja funciona

- Cadastro com confirmacao de e-mail; login; recuperacao e alteracao de senha; OAuth Google opcional. Contas anteriores a migration de verificacao sao marcadas como verificadas; novos cadastros precisam confirmar o e-mail.
- Criacao e entrada em salas de ate duas pessoas, nome opcional da reuniao (padrao `Meeting`), convite com codigo e QR code.
- Microfone, modo somente escuta, mute, transcricoes, audio traduzido e chat em tempo real. A traducao de chat e de voz depende do provider configurado.
- Sessao de login no `localStorage` do **navegador/perfil atual**. O token expira em 72 horas; mudar ou redefinir a senha invalida tokens anteriores.

**Limites importantes:** mensagens do chat, notificacoes de nao lidas e conexoes de chamada vivem na memoria; recarregar a pagina nao recupera o historico. O modo `mock` nao e traducao real. Chamadas ativas podem ser interrompidas por reinicio/deploy, e a arquitetura atual usa uma unica replica do backend.

## Traducao e acesso por outros dispositivos

O padrao `TRANSLATION_PROVIDER=mock` permite verificar transporte e interface sem credenciais externas. Para Gemini, use Vertex AI com ADC/conta de servico ou, quando compativel com o modelo, `GEMINI_API_KEY`. OAuth Google para **login** usa credenciais distintas das de **traducao**. Veja [configuracao e limites da traducao](docs/translation.md).

Para abrir a interface em outro aparelho da mesma rede, use `http://<IP-LAN-do-computador>:5173`. O IP exibido pelo Docker como `Network`, por exemplo `172.x.x.x`, e interno ao container; use o IP do computador. O frontend pode abrir, mas navegadores normalmente bloqueiam o **microfone em HTTP por IP de rede**. Para testar chamada entre celular e computador, use o deploy HTTPS ou um tunel HTTPS confiavel. `localhost` no celular refere-se ao proprio celular. Nao exponha o servidor Vite de desenvolvimento diretamente na internet. Veja [deploy e teste em dispositivos](docs/deploy.md).

## Verificacao

```powershell
cd backend
go test ./...
```

Os testes de integracao com banco exigem `TEST_DATABASE_URL` apontando para um **banco de teste separado**; sem essa variavel, eles sao ignorados. O teste SMTP opcional usa `TEST_SMTP_HOST` e `TEST_SMTP_PORT`. Para o frontend:

```powershell
cd frontend
npm ci
npm run typecheck
npm run build
npx playwright install chromium
npm run test:e2e
```

O Playwright usa mocks de API por padrao. O cenario de integracao `realtime-live.spec.ts` ainda espera login imediato apos cadastro e precisa ser adaptado a confirmacao de e-mail; **nao o use como aprovacao da branch atual**. Veja [guia do frontend](docs/frontend.md).

## Mapa da documentacao

- [Indice dos guias](docs/README.md): escolha o assunto ou a tarefa.
- [Contas e seguranca](docs/auth.md): confirmacao de e-mail, senhas e Google OAuth.
- [Frontend](docs/frontend.md): telas, componentes, audio e testes.
- [WebSocket](docs/websocket.md): eventos, autenticacao, limites e frames binarios.
- [Traducao](docs/translation.md): providers, credenciais, fluxo e limitacoes.
- [Deploy no Railway](docs/deploy.md): variaveis, publicacao, teste e diagnostico.

## Estrutura

```text
symphonia/
├── backend/                  # Servidor Go, regras e migrations
├── frontend/                 # React, Vite e testes de navegador
├── docs/                     # Guias e protocolos
├── .env.example              # Modelo de configuracao, sem segredos reais
├── docker-compose.yml        # Desenvolvimento
├── docker-compose.deploy.yml # Previa local da imagem de producao
├── Dockerfile                # Imagem unica para deploy
└── railway.json              # Health check e replica no Railway
```
