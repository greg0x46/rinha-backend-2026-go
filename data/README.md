# Dataset local

Este diretorio guarda artefatos de dados usados pela imagem da API.

Fluxo canonico:

```sh
go run ./cmd/preprocess
```

Entradas e saidas:

- Entrada versionada: `data/references.json.gz`.
- Saida gerada localmente: `data/references.bin`.
- Formato padrao da saida: manifest v2 + `int16[14] + uint8 label`.

`references.bin` e gerado localmente e ignorado pelo Git. O Dockerfile tambem
gera esse binario durante o build da imagem e copia apenas o binario para o
runtime em `/app/data/references.bin`.

Para gerar um artefato `float32` de baseline, use:

```sh
go run ./cmd/preprocess -format float32 -output data/references.float32.bin
```
