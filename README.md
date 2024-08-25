# Go WebSocket Chat API

Este projeto é uma API em Go para um sistema de chat baseado em WebSocket, permitindo a criação de salas, o envio de mensagens, reações a mensagens, e a comunicação em tempo real entre clientes. A API utiliza o framework \`chi\` para roteamento e \`pgx\` para interações com o banco de dados PostgreSQL.

## Funcionalidades

- **Gerenciamento de Salas**:
  - Criar novas salas de chat.
  - Listar todas as salas.

- **Gerenciamento de Mensagens**:
  - Enviar mensagens para uma sala específica.
  - Listar mensagens de uma sala.
  - Obter uma mensagem específica.

- **Reações às Mensagens**:
  - Adicionar ou remover reações a mensagens.
  - Impedir a remoção de reações quando o contador atingir zero.

- **Notificações em Tempo Real**:
  - Subscrever-se a uma sala de chat e receber mensagens em tempo real via WebSocket.

## Requisitos

- Go 1.16 ou superior
- PostgreSQL
- Ferramentas Go:
  - \`sqlc\` para geração de código SQL.

## Instalação

1. Clone o repositório:
   \`\`\`bash
   git clone https://github.com/WENDELLDELIMA/go-web-socket.git
   cd go-web-socket
   \`\`\`

2. Instale as dependências:
   \`\`\`bash
   go mod tidy
   \`\`\`

3. Configure o banco de dados PostgreSQL com o esquema necessário.

4. Gere o código SQLC:
   \`\`\`bash
   sqlc generate
   \`\`\`

## Configuração

Certifique-se de que você tem um arquivo de configuração do banco de dados e outras variáveis de ambiente configuradas corretamente para conectar-se ao PostgreSQL.

## Uso

### Inicialização da API

Para iniciar a API, use o comando:

\`\`\`bash
go run cmd/wsrs/main.go
\`\`\`

### Endpoints

#### 1. Salas

- **Criar Sala**
  - \`POST /api/rooms/\`
  - Corpo da Requisição:
    \`\`\`json
    {
      "theme": "Nome da Sala"
    }
    \`\`\`

- **Listar Salas**
  - \`GET /api/rooms/\`

#### 2. Mensagens

- **Criar Mensagem**
  - \`POST /api/rooms/{room_id}/messages/\`
  - Corpo da Requisição:
    \`\`\`json
    {
      "message": "Conteúdo da Mensagem"
    }
    \`\`\`

- **Listar Mensagens**
  - \`GET /api/rooms/{room_id}/messages/\`

- **Obter Mensagem**
  - \`GET /api/rooms/{room_id}/messages/{message_id}\`

#### 3. Reações

- **Adicionar ou Remover Reação**
  - \`PATCH /api/rooms/{room_id}/messages/{message_id}/react\`
  - \`DELETE /api/rooms/{room_id}/messages/{message_id}/react\`

#### 4. WebSocket

- **Subscrever-se a uma Sala**
  - \`GET /subscribe/{room_id}\`

  Após conectar-se via WebSocket, o cliente receberá mensagens em tempo real enviadas para a sala especificada.

## Middleware

- **CORS**: Configurado para permitir origens cruzadas, permitindo que o front-end se comunique com a API.
- **Content-Type JSON**: Middleware que garante que todas as respostas sejam no formato JSON.

## Contribuições

Se você encontrar problemas ou tiver sugestões para melhorar este projeto, sinta-se à vontade para abrir uma issue ou enviar um pull request.

## Licença

Este projeto é licenciado sob a Licença MIT. Consulte o arquivo [LICENSE](LICENSE) para mais detalhes.
`
