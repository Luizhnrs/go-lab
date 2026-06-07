# API didática de usuários em Go

API REST simples usando apenas `net/http`, `database/sql` e SQLite. O SQLite
cumpre em Go um papel semelhante ao H2 no ecossistema Java: banco relacional
embutido, sem necessidade de instalar ou executar um servidor separado.

## Requisitos

- Go 1.24 ou superior

## Executar

```powershell
go mod tidy
go run .
```

A aplicação cria automaticamente o arquivo `users.db` e fica disponível em
`http://localhost:8080`.

Para usar outro endereço ou banco:

```powershell
$env:HTTP_ADDR = ":9090"
$env:DATABASE_URL = "file:meu-banco.db"
go run .
```

## Endpoints

| Método | Rota          | Descrição             |
|--------|---------------|-----------------------|
| GET    | `/health`     | Verifica a aplicação  |
| POST   | `/users`      | Cadastra um usuário   |
| GET    | `/users`      | Lista os usuários     |
| GET    | `/users/{id}` | Consulta um usuário   |
| PUT    | `/users/{id}` | Atualiza um usuário   |
| DELETE | `/users/{id}` | Exclui um usuário     |

Exemplo de cadastro:

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri http://localhost:8080/users `
  -ContentType "application/json" `
  -Body '{"name":"Maria Silva","email":"maria@example.com"}'
```

Listar usuários:

```powershell
Invoke-RestMethod http://localhost:8080/users
```

## Testes

```powershell
go test ./...
```

Os testes usam um banco SQLite em memória e não alteram `users.db`.
