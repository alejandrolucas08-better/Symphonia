# Contas, e-mail e Google OAuth

Este guia descreve **o que a branch atual implementa**. Comece pelo [inicio rapido](../README.md) para subir PostgreSQL, backend, frontend e Mailpit.

## Fluxos para quem usa a aplicacao

| Acao | O que acontece |
| --- | --- |
| Criar conta com e-mail e senha | O backend salva a conta como pendente e envia um link. O cadastro **nao inicia sessao**. |
| Confirmar e-mail | O link abre `/verify-email`; a confirmacao libera o login e inicia uma sessao no navegador. |
| Reenviar confirmacao | Em `/resend-verification`, informe o mesmo e-mail. A resposta nao revela se existe conta. |
| Esqueci a senha | Em `/forgot-password`, solicite um link para `/reset-password`. A resposta tambem nao revela se o e-mail existe. |
| Alterar senha conectado | Abra **Conta** (`/account/password`) e informe a senha atual e a nova. |
| Entrar com Google | O botao aparece apenas quando o OAuth esta configurado no backend. Uma conta nova criada pelo Google ja tem e-mail verificado e pode nao ter senha local. |
| Vincular Google a uma conta existente | Entre com a senha, abra **Conta** e escolha **Vincular Google**. O e-mail verificado no Google precisa ser o mesmo da conta local. |

Uma conta Google sem senha local pode usar **Defina uma senha por e-mail** na pagina Conta. A mudanca ou redefinicao de senha invalida os JWTs anteriores; uma conexao WebSocket ja aberta so e revalidada ao reconectar.

## Configuracao de e-mail

O backend precisa de `APP_PUBLIC_URL`, `SMTP_HOST` e `EMAIL_FROM` para gerar links e enviar mensagens. `APP_PUBLIC_URL` e a **origem vista no navegador**, sem caminho nem barra final: `http://localhost:5173` no Compose local ou `https://seu-dominio` no deploy. Configure `SMTP_PORT` (padrao 587 quando omitida), `SMTP_USER` e `SMTP_PASSWORD` conforme o provedor. Em servidores SMTP remotos, o backend exige STARTTLS; a excecao sem TLS e apenas para `localhost`, `127.0.0.1` e `mailpit` no desenvolvimento.

No Compose, o Mailpit recebe as mensagens em `http://localhost:8025`. Acesse o link recebido ali; nao e necessario possuir uma caixa de e-mail real. Se Go roda no host, use `SMTP_HOST=localhost` e `SMTP_PORT=1025`; `mailpit` e o nome do servico **dentro** da rede Docker. No Railway, use um provedor SMTP real, confirme o remetente e configure as variaveis no servico da aplicacao, nao no PostgreSQL.

Se o envio falhar durante o cadastro, a conta fica pendente. Corrija a configuracao e solicite reenvio. Sem SMTP valido, novos cadastros e recuperacao de senha ficam indisponiveis; contas ja verificadas continuam podendo entrar com senha.

## Google OAuth

1. Crie um cliente OAuth do tipo **Aplicacao Web** no projeto Google e configure sua tela de consentimento. Se o projeto estiver em modo de teste, inclua os usuarios de teste necessarios.
2. Cadastre como URI de redirecionamento autorizado o valor exato `<APP_PUBLIC_URL>/api/auth/google/callback`. Exemplos: `http://localhost:5173/api/auth/google/callback` no Compose e `https://seu-dominio/api/auth/google/callback` no deploy.
3. Configure `GOOGLE_OAUTH_CLIENT_ID` e `GOOGLE_OAUTH_CLIENT_SECRET` **no backend**. Reinicie o servico e verifique `GET /api/auth/providers`: `{"google":true}` indica que o botao pode aparecer.
4. Teste primeiro uma conta Google nova. Para uma conta local com o mesmo e-mail, use a vinculacao explicita na pagina Conta; o sistema nao une contas apenas pela coincidencia do e-mail.

O backend usa authorization code com PKCE, valida o ID token e devolve ao frontend um ticket de uso unico. O segredo OAuth nao vai para o bundle Vite. Essas credenciais **nao sao** `GEMINI_API_KEY` nem as credenciais Vertex da traducao. A disponibilidade do provedor Google precisa ser testada manualmente com um cliente OAuth real; os testes automatizados simulam o provedor.

## Prazos e limites de seguranca

- Link de confirmacao: 24 horas; link de redefinicao: 30 minutos; ambos sao de uso unico. O reenvio e limitado a uma solicitacao por minuto por conta e finalidade.
- JWT de sessao: 72 horas. A aplicacao o guarda no `localStorage` do perfil atual; fechar a guia nao encerra a sessao, mas outro navegador ou perfil nao compartilha esse armazenamento. Nao use computadores compartilhados sem sair da conta.
- Tickets de conclusao do OAuth: um minuto e uso unico; mudanca/redefinicao de senha invalida tickets anteriores. O estado do redirecionamento expira em dez minutos.
- Senha: de 8 a 72 bytes. O limite e em **bytes**, portanto caracteres acentuados podem ocupar mais de um byte.
- Use HTTPS fora de `localhost`. Nunca coloque `JWT_SECRET`, senha SMTP, segredo OAuth ou credenciais Gemini em `VITE_*`, codigo fonte, logs ou prints. A perda/rotacao do `JWT_SECRET` invalida sessoes existentes.

Contas existentes na primeira aplicacao da migration `007_account_auth.sql` sao marcadas como verificadas para nao bloquear quem ja usava o sistema. Novos cadastros com senha exigem confirmacao. O login nao tem refresh token nesta etapa; ao expirar o JWT, e necessario entrar novamente.

## Onde os dados ficam

`users` guarda o hash da senha (quando existe), o horario de verificacao do e-mail e `auth_version`. `oauth_identities` liga o identificador estavel do Google ao usuario local. `auth_tokens` guarda **apenas o hash** dos links/tickets temporarios, sua finalidade e a expiracao; o token bruto aparece somente no link enviado ou no retorno do OAuth. O uso de um link consome seu registro em uma transacao, impedindo reutilizacao.

Os JWTs nao sao uma tabela de sessoes. Cada requisicao autenticada confere a versao do token contra `auth_version` no banco. Alterar ou redefinir a senha aumenta essa versao, revogando os JWTs anteriores e os tickets OAuth pendentes. Um socket que ja passou pelo handshake continua ate desconectar; a verificacao da versao acontece novamente na proxima conexao.

## Conferencia rapida

1. Com Compose ativo, crie uma conta em `/register` e confirme que o login antes do link e recusado.
2. Abra o Mailpit, confirme o e-mail e entre. Feche e reabra a guia no mesmo perfil: a sessao deve continuar ate o token expirar ou ser invalidado.
3. Solicite redefinicao, use o link uma vez e tente reutiliza-lo: a segunda tentativa deve falhar.
4. Altere a senha conectado e confirme que a senha antiga nao entra mais.
5. Se configurou Google, confira o login de uma conta nova e a vinculacao de uma conta existente com o mesmo e-mail.

Para diagnostico de deploy e variaveis de producao, consulte o [guia do Railway](deploy.md).

Referencia do provedor: [Google OpenID Connect](https://developers.google.com/identity/openid-connect/openid-connect). O `redirect_uri` cadastrado precisa coincidir exatamente com o enviado pelo backend.
