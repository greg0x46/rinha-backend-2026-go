# rinha-be-2026

Implementacao em Go para a Rinha de Backend 2026.

Este repositorio fica aninhado dentro do repositorio oficial apenas para facilitar a leitura dos arquivos em `../resources`, `../docs` e `../test` durante o desenvolvimento.

## Alvo inicial

- API em Go com baixo overhead.
- Load balancer na porta `9999`.
- Duas instancias da API.
- Dataset pre-processado em formato binario.
- Busca vetorial aproximada com rerank exato dos candidatos.

Veja o plano em [docs/PLANO_INICIAL.md](docs/PLANO_INICIAL.md).

O breakdown operacional esta em [docs/TASKS.md](docs/TASKS.md).

## Desenvolvimento

Rodar testes:

```sh
go test ./...
```

Gerar o indice binario local:

```sh
go run ./cmd/preprocess
```

O dataset oficial compactado fica versionado em `data/references.json.gz`.
O preprocessamento gera `data/references.bin`, que e ignorado pelo Git.
O build Docker tambem gera esse binario automaticamente dentro da imagem.

Subir a topologia local:

```sh
docker compose up -d --build
```

Validar endpoints:

```sh
curl -i http://127.0.0.1:9999/ready
curl -i -X POST http://127.0.0.1:9999/fraud-score \
  -H 'Content-Type: application/json' \
  -d '{"id":"tx-1"}'
```

Parar os containers:

```sh
docker compose down
```
