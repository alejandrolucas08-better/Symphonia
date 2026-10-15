# Guias do Symphonia

Comece pelo [README da raiz](../README.md) para instalar, configurar `.env` e subir a aplicacao. Use este indice para aprofundar um assunto:

| Quero... | Leia |
| --- | --- |
| Entender cadastro, verificacao de e-mail, senhas e Google OAuth | [Contas e seguranca](auth.md) |
| Navegar pelo frontend, audio no navegador e testes | [Frontend](frontend.md) |
| Integrar um cliente de chamada ou depurar eventos em tempo real | [Protocolo WebSocket](websocket.md) |
| Configurar Gemini e entender quando a traducao fica indisponivel | [Traducao](translation.md) |
| Publicar no Railway ou testar em celular por HTTPS | [Deploy](deploy.md) |

**Estado atual:** banco guarda contas, salas e participantes; chat, presenca e audio nao sao persistidos. O provider `mock` valida a integracao, mas nao traduz linguagem natural. A traducao Gemini e o OAuth Google precisam de configuracao externa. Consulte os limites e a verificacao manual em cada guia antes de usar em producao.
