# Deploy no Railway

Este guia parte de uma branch revisada e enviada ao GitHub. Ele **nao faz**
commit, push ou merge por voce. Para rodar sem Railway, comece pelo
[README](../README.md); para entender cadastro e Google, veja
[contas e seguranca](auth.md).

## Antes de comecar

- Tenha acesso ao repositorio no GitHub, a um projeto Railway com PostgreSQL e
  a um dominio HTTPS para o servico da aplicacao.
- Para cadastro e recuperacao de senha, tenha um provedor SMTP e um remetente
  autorizado. Mailpit e apenas para desenvolvimento local.
- Para entrada com Google, crie um cliente OAuth Web. Para traducao real, tenha
  acesso ao modelo Gemini e suas **outras** credenciais; login Google e traducao
  nao usam a mesma chave.
- Use uma base de teste/homologacao antes de alterar um ambiente com usuarios.

## Arquitetura

O deploy usa dois servicos: uma aplicacao Docker (frontend compilado + servidor
Go) e PostgreSQL. O Railway termina HTTPS; o Go recebe HTTP na porta `PORT`.
Paginas, `/api`, `/healthz` e WebSocket usam o mesmo dominio publico. O frontend
escolhe `wss://` automaticamente quando a pagina usa HTTPS.

O `Dockerfile` da raiz e o `railway.json` sao exclusivos do deploy. Os Dockerfiles
de `frontend/` e `backend/`, o Vite e o `docker-compose.yml` continuam sendo o
ambiente local de desenvolvimento. Nenhum segredo entra no build do frontend.

## Publicar

1. Revise, faca commit e envie a branch preparada para o GitHub. Esses passos nao
   sao executados automaticamente. O Railway precisa ter acesso a essa versao.
2. Crie um projeto no Railway e adicione PostgreSQL. Use uma base nova para
   homologacao, separada dos dados locais.
3. Adicione um servico a partir do repositorio Symphonia e selecione a branch
   revisada. Deixe o Root Directory na raiz do repositorio (`/`), nunca em
   `frontend/` ou `backend/`. O Railway deve detectar `Dockerfile` e `railway.json`.
4. Gere o dominio publico do servico em Settings > Networking > Generate Domain.
   Anote a origem completa `https://...`, sem caminho e sem barra final.
5. Configure as variaveis abaixo no servico da aplicacao. Nao copie o `.env`
   local inteiro: os enderecos e as credenciais sao diferentes.
6. Cadastre o URI de callback do Google, se usar OAuth, e aplique as alteracoes
   pendentes do Railway para disparar o deploy.
7. Aguarde o deploy e o health check. Abra `/healthz`, depois `/register`.
   O banco recebe as migrations automaticamente antes de o servidor aceitar trafego.

| Variavel | Valor no servico da aplicacao |
| --- | --- |
| `DATABASE_URL` | Referencia `${{Postgres.DATABASE_URL}}`; ajuste `Postgres` ao nome real do servico. Preserve as opcoes TLS fornecidas. |
| `JWT_SECRET` | Segredo exclusivo do ambiente, com pelo menos 32 bytes. |
| `APP_PUBLIC_URL` | Dominio publico completo, por exemplo `https://seu-dominio.up.railway.app`, sem barra final. Usado nos links de e-mail e no retorno OAuth. |
| `SMTP_HOST`, `SMTP_PORT` | Servidor SMTP do provedor de e-mail; normalmente porta `587` com STARTTLS. |
| `SMTP_USER`, `SMTP_PASSWORD` | Credenciais do provedor SMTP, como segredos do Railway. |
| `EMAIL_FROM` | Remetente autorizado pelo provedor, por exemplo `Symphonia <contato@seu-dominio.com>`. |
| `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET` | Opcionais; habilitam o botao Google. Crie um cliente OAuth do tipo Web e configure o redirect URI `APP_PUBLIC_URL/api/auth/google/callback`. |
| `CORS_ALLOWED_ORIGINS` | `https://seu-dominio.up.railway.app`, sem barra final. |
| `WS_ALLOWED_ORIGINS` | O mesmo endereco HTTPS. Necessario pois TLS termina no proxy. |
| `TRANSLATION_PROVIDER` | `mock` para validar a infraestrutura; `gemini` para traducao real. |
| `RAILWAY_DEPLOYMENT_DRAINING_SECONDS` | `15`, para permitir o encerramento do processo Go. |

`DATABASE_URL` pertence ao **servico da aplicacao** e deve ser uma referencia
ao valor fornecido pelo servico PostgreSQL. Nao substitua manualmente sua senha
nem copie a URL exibida em capturas de tela. Se rotacionar credenciais do banco,
atualize a referencia/conexao do app e faca redeploy; uma URL antiga pode fazer
`/healthz` retornar 503. `POSTGRES_PASSWORD` e variavel do **servico do banco**
ou do Compose local, nao algo a adicionar ao app Railway por causa desta feature.

`PORT` e fornecida pelo Railway; tem prioridade sobre `BACKEND_PORT`. A imagem
define `APP_ENV=production` e `FRONTEND_DIST=/app/public`. Nao configure
`VITE_API_BASE_URL` nem `API_PROXY_TARGET`: no deploy o proprio Go serve a API e
o build no mesmo endereco. Dominios adicionais precisam entrar nas origens,
separados por virgula; nao use `*`.

Para conferir o endereco escolhido, `GET https://seu-dominio/healthz` deve
responder HTTP 200 com `status: "ok"`. Isso prova que a aplicacao alcancou o
banco, **nao** que SMTP, OAuth, microfone ou Gemini estejam funcionando.

Sem SMTP e `APP_PUBLIC_URL`, os cadastros novos e os links de recuperacao ficam
indisponiveis; contas existentes continuam podendo entrar. Em desenvolvimento,
o Compose usa Mailpit em `http://localhost:8025`. No Railway, configure um
provedor SMTP real e verifique o remetente antes de testar o cadastro. O segredo
OAuth do Google fica somente no backend. Configure tambem a tela de consentimento
e os usuarios de teste no Google Cloud Console, se o app ainda nao estiver
publicado. O e-mail usado no OAuth nao e vinculado automaticamente a uma conta
local preexistente; entre com senha e vincule o Google pela pagina Conta.

Para gerar o segredo (execute localmente e cadastre somente no Railway):

```powershell
$secretBytes = [byte[]]::new(48)
[System.Security.Cryptography.RandomNumberGenerator]::Fill($secretBytes)
[Convert]::ToHexString($secretBytes)
```

Mantenha **uma replica em uma unica regiao**, sem escalonamento horizontal.
As salas e conexoes ficam na memoria do processo. Deploys e reinicios podem
interromper chamadas; programe-os fora das sessoes de teste. O health check de
deploy confirma servidor e banco, mas nao testa acesso ao Gemini nem substitui
monitoramento continuo. Configure backups do PostgreSQL antes de guardar dados
que precisem ser preservados. Reverter a imagem nao reverte o schema do banco.

## Gemini no ambiente hospedado

O modo `mock` valida o transporte, mas nao fornece traducao real. Para Vertex:

1. Habilite a API, o faturamento e o acesso ao modelo no projeto Google Cloud.
2. Use uma conta de servico dedicada com acesso ao Vertex AI no projeto correto.
3. Configure `TRANSLATION_PROVIDER=gemini`, `GOOGLE_CLOUD_PROJECT` e
   `TRANSLATION_MODEL=gemini-3.5-live-translate-preview`.
4. Cadastre o JSON da conta de servico na variavel secreta
   `GOOGLE_CREDENTIALS_JSON` do Railway. O backend le esse JSON em memoria; nao
   precisa de arquivo na imagem nem de credenciais ADC do seu computador.
   Nunca versione esse JSON nem o coloque em uma variavel `VITE_*`.

Sem `GOOGLE_CREDENTIALS_JSON`, a descoberta ADC existente continua funcionando
(arquivo indicado por `GOOGLE_APPLICATION_CREDENTIALS` ou identidade disponivel
no ambiente). Sem `GOOGLE_CLOUD_PROJECT`, o backend usa a alternativa existente
`GEMINI_API_KEY`; confirme que o modelo escolhido esta disponivel nessa API.
O chat usa as mesmas credenciais e traduz mensagens com
`TEXT_TRANSLATION_MODEL=gemini-3.5-flash-lite` (valor padrao). Sem Gemini, as
mensagens originais continuam chegando, mas a traducao fica indisponivel.

`GOOGLE_OAUTH_CLIENT_ID` e `GOOGLE_OAUTH_CLIENT_SECRET` sao somente para
**autenticacao com Google**. O URI de redirecionamento autorizado no cliente
OAuth Web deve ser exatamente `https://seu-dominio/api/auth/google/callback`.
Confira tambem a tela de consentimento e usuarios de teste. `GEMINI_API_KEY`,
`GOOGLE_CLOUD_PROJECT` e `GOOGLE_CREDENTIALS_JSON` sao para **traducao**, nao
habilitam o botao de login Google.

A autorizacao real so e exercitada ao abrir uma sessao de traducao. O modelo e
preview e seu acesso depende do Google. Use inicialmente dados de teste e
acompanhe o consumo do Railway e do Gemini.

## Desenvolvimento local

O fluxo anterior permanece igual:

```powershell
docker compose up --build
```

Ou execute Go e Vite separadamente como descrito no README. `localhost` permite
microfone. Um IP da LAN em HTTP continua sem acesso ao microfone: para dois
dispositivos, use o dominio HTTPS hospedado ou um tunel HTTPS configurado.

## Previa local da imagem de producao

Com `JWT_SECRET` e `POSTGRES_PASSWORD` preenchidos no `.env`, execute na raiz:

```powershell
docker compose -p symphonia-deploy -f docker-compose.deploy.yml up -d --build --wait
```

Acesse `http://localhost:8081`. A porta pode ser alterada por `DEPLOY_PORT`.
Essa previa usa banco/volume separados do desenvolvimento, nao publica a porta
do PostgreSQL e fixa o tradutor `mock`. Para a senha de teste, use caracteres
alfanumericos (por exemplo, um segredo hexadecimal), pois ela compoe a URL do banco.
O endereco e restrito ao proprio computador e nao e uma hospedagem HTTPS.

Para parar, preservando o banco:

```powershell
docker compose -p symphonia-deploy -f docker-compose.deploy.yml down
```

Para verificar rotas, assets e o worklet contra essa imagem, execute em
`frontend/`:

```powershell
$env:E2E_BASE_URL = 'http://127.0.0.1:8081'
$env:DEPLOY_E2E = '1'
npx playwright test deploy.spec.ts --project desktop
```

`realtime-live.spec.ts` ainda espera um JWT no retorno de `/api/register` e
precisa ser adaptado a confirmacao de e-mail antes de funcionar com o backend
atual. Nao o use como criterio de aprovacao desta branch. Feche esse terminal
depois ou remova `E2E_BASE_URL` e `DEPLOY_E2E` para voltar aos testes locais.
Quando o teste real for atualizado, execute-o somente em banco de homologacao:
ele cria usuarios e chamadas. Para nao consumir Gemini, use provider `mock`.

## Teste final em dispositivos

Abra o dominio HTTPS como dois usuarios diferentes, um no computador e outro
no celular (inclusive Wi-Fi versus 4G/5G). Autorize o microfone, entre na mesma
sala e confira som nos dois sentidos, chat, mute, saida e reconexao. Para PT/EN
com Gemini, configure A para falar/ouvir portugues e B para falar/ouvir ingles.
Use fones para evitar realimentacao entre os dispositivos.

Se o microfone falhar, confira HTTPS e a permissao do navegador/sistema. Se o
WebSocket retornar 403, confira `WS_ALLOWED_ORIGINS` com o dominio exato. Se o
health retornar 503, confira a disponibilidade do banco. Se a traducao falhar
com o health em 200, verifique as credenciais e o acesso ao modelo nos logs.
Se o cadastro disser que o envio de e-mail esta indisponivel, confira
`APP_PUBLIC_URL`, `SMTP_HOST`, `SMTP_PORT`, `EMAIL_FROM` e as credenciais SMTP.
Se o botao Google nao aparecer, confira `/api/auth/providers`; se aparecer mas
o retorno falhar, compare o URI de callback autorizado com o dominio atual.
Nao cole tokens, URLs com token, senhas ou chaves em issues ou logs publicos.

## Referencias

- [Railway: Dockerfiles](https://docs.railway.com/builds/dockerfiles)
- [Railway: configuracao como codigo](https://docs.railway.com/config-as-code/reference)
- [Railway: health checks e PORT](https://docs.railway.com/deployments/healthchecks)
- [Railway: PostgreSQL](https://docs.railway.com/databases/postgresql)
- [Railway: WebSockets e limites](https://docs.railway.com/networking/public-networking/specs-and-limits)
- [MDN: contexto seguro para microfone](https://developer.mozilla.org/en-US/docs/Web/API/MediaDevices/getUserMedia#privacy_and_security)
