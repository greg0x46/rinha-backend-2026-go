# rinha-be-2026

Implementacao em Go para a Rinha de Backend 2026.

O desafio e responder um servico de antifraude dentro de um limite apertado de
`1 CPU` e `350 MB`, mantendo a regra de negocio: vetorizar a transacao em 14
dimensoes, encontrar os 5 vizinhos mais proximos e calcular
`fraud_score = fraudes / 5`. A API publica fica em `:9999`, atras de um load
balancer, distribuindo chamadas para duas instancias da API.

Endpoints:

- `GET /ready`
- `POST /fraud-score`

Resposta:

```json
{
  "approved": true,
  "fraud_score": 0.0
}
```

## Stack

- Go `1.26` com `net/http`.
- HAProxy em `mode tcp` como load balancer.
- Unix Domain Sockets entre HAProxy e APIs.
- Indice binario gerado no build a partir de `data/references.json.gz`.
- IVF/k-means offline com vetores quantizados em `int16`.
- Parser JSON manual no hot path, com fallback para `encoding/json`.
- Kernel `amd64` SSE4.1 para distancia em blocos, com fallback Go.

## Solucao

A implementacao partiu de um baseline exato por brute force. Ele era simples e
correto, mas inviavel para competicao: com `3.000.000` de referencias, a stack
chegou a p99 de `4327ms` e as APIs ficaram praticamente no limite de memoria
usando vetores `float32`.

A versao atual reduz o trabalho sem mudar a regra de score:

1. O payload e convertido para o vetor oficial de 14 dimensoes.
2. O indice IVF/k-means seleciona poucos centroides candidatos.
3. Somente os blocos dessas listas sao escaneados.
4. A distancia e calculada de forma exata dentro dos candidatos quantizados.
5. O top 5 define um dos 6 JSONs de resposta pre-formatados.

O indice aproximado decide onde procurar; a resposta continua sendo derivada dos
5 vizinhos encontrados.

## Decisoes tecnicas

### Pre-processamento binario

Carregar JSON no runtime custava CPU, memoria e tempo de startup. O dataset
compactado fica no repositorio, mas a imagem gera um binario de indice durante o
build.

Resultados medidos:

- `3.000.000` referencias pre-processadas em aproximadamente `16s`.
- Loader `float32`: `493ms`.
- Loader quantizado: `179ms/op`.

### Quantizacao

O formato inicial `float32 + uint8` ajudou a validar a regra, mas nao cabia com
folga em duas APIs. A troca para `int16[14] + uint8` reduziu o artefato de
aproximadamente `164M` para `87MB`.

Na pratica, cada API caiu de perto de `160MiB` para cerca de `86MiB` apos
startup. O tradeoff e aceitar pequenas diferencas em bordas, compensadas por
comparacoes contra o baseline exato e pelo tuning do indice.

### IVF/k-means com probes seletivos

Brute force fazia `3M * 14` comparacoes por request. O IVF/k-means reduz esse
custo buscando apenas listas proximas. Mais probes melhoram qualidade, mas
aumentam CPU; poucos probes reduzem latencia, mas geram erro de deteccao.

A configuracao final usa probes rapidos para a maioria dos casos e busca
expandida quando o top 5 cai perto da fronteira de decisao (`2` ou `3` fraudes).
Com `KMEANS_QUICK_PROBE=6` e `KMEANS_EXPANDED_PROBE=16`, a deteccao medida ficou
em `FP=2`, `FN=0`, `E_weighted=2`.

### Layout em blocos e pruning

Quando a qualidade do indice melhorou, o custo dominante voltou a ser calcular
distancias. Os vetores passaram para um layout em blocos SoA de 8 registros, com
calculo parcial da distancia. Se um bloco ja nao consegue entrar no top 5, o
scan para cedo.

Esse pruning permitiu aumentar probes para ganhar qualidade sem colapsar p99. O
layout em blocos reduziu p99 de `796ms` para `622ms` em um A/B, e o kernel
SSE4.1 reduziu o custo medio do score offline em cerca de `11%`.

### Transporte

Depois que o handler ficou rapido, o load balancer virou gargalo. HAProxy em
`mode tcp` com Unix Domain Sockets substituiu o proxy HTTP e removeu trabalho do
LB.

Resultado A/B:

| Configuracao | p99 | http_errors | final_score |
| --- | ---: | ---: | ---: |
| Proxy HTTP + UDS | `2001.85ms` | `2192` | `-3730.73` |
| HAProxy TCP + UDS | `679.90ms` | `0` | `430.80` |

### CPU split entre LB e APIs

A configuracao inicial dava `0.05` para o LB e `0.475` para cada API (total
`1.0`). Sob carga, `cpu.stat` mostrou `api1` com `11.5%` do wall clock em
`throttling` (eventos medios de `134ms`), enquanto `api2` ficava em `0.8%`. O
`Recv-Q` do UDS sempre vazio refutou accept queue como causa: o gargalo era
`cpu_throttling` puro, e o desbalanceamento entre as APIs era artefato do LB
sem folga, nao da topologia `mode tcp` em si.

Um sweep de `LB cpus` mantendo total `1.0`, com mesma imagem e mesmo
`haproxy.cfg`, trocou o regime:

| LB / API cada | p99 mediana | final_score mediano | spread (stdev) |
| --- | ---: | ---: | ---: |
| `0.05 / 0.475` | `586.66ms` | `3088.48` | n=1 |
| `0.06 / 0.47` | `~380ms` | `~3275` | `~12` |
| `0.08 / 0.46` | `156.65ms` | `3661.93` | `~397` |
| `0.10 / 0.45` | `87.45ms` | `3915.12` | `~43` |
| `0.12 / 0.44` | `68.78ms` | `4019.39` | `~31` |
| `0.15 / 0.425` | `30.89ms` | `4367.10` | `~41` (5 amostras) |
| `0.2 / 0.4` | `26.51ms` | `4433.38` | `~712` |

A escolha promovida foi `LB 0.15 / APIs 0.425`: variancia colapsa (stdev p99
`~3ms`), p99 cai uma ordem de grandeza e ainda sobra cota nas APIs. Em
`LB 0.2 / APIs 0.4` a media e ligeiramente maior, mas a cota das APIs fica
no fio e a variancia dispara.

### Parser manual

`encoding/json` era correto, mas alocava em todo request. O parser manual cobre
o formato esperado do teste e vetoriza direto dos bytes; entradas fora do fast
path usam fallback seguro.

Microbench:

| Caminho | Tempo | Memoria | Alocacoes |
| --- | ---: | ---: | ---: |
| `encoding/json + Vectorize` | `11850 ns/op` | `664 B/op` | `19` |
| `fastjson + VectorizeFromPayload` | `1571 ns/op` | `0 B/op` | `0` |

## Resultados

Rodada final de referencia, com 5 amostras (`LB 0.15 / APIs 0.425`):

| Metrica | min | mediana | media | max | stdev |
| --- | ---: | ---: | ---: | ---: | ---: |
| p99 | `27.66ms` | `30.89ms` | `31.41ms` | `35.73ms` | `2.99ms` |
| final_score | `4303.80` | `4367.10` | `4361.33` | `4414.94` | `40.95` |
| http_errors | `0` | `0` | `0` | `0` | - |

Resumo da evolucao:

| Marco | Problema atacado | Resultado |
| --- | --- | ---: |
| Brute force `float32` | baseline correto | p99 `4327ms`, RSS alto |
| Quantizacao `int16` | memoria | `164M -> 87MB` |
| IVF inicial | custo de busca | p50 `2154ms -> 2ms` em carga curta |
| HAProxy TCP + UDS | gargalo de transporte | `2192 -> 0` erros HTTP |
| K-means + pruning | qualidade sem estourar p99 | `FP=2`, `FN=0`, p99 mediana `596.91ms` |
| Re-split de CPU LB/APIs | throttling oculto do LB | p99 `597ms -> 31ms`, final `+1287` |

## Aprendizados

- Otimizacao local nao basta: reduzir custo do handler ajudou, mas o maior salto
  veio de remover overhead do load balancer.
- `GOMAXPROCS` precisou ser fixado em `1`; o default gerava mais paralelismo do
  que o cgroup realmente permitia.
- Menos probes nem sempre melhora o resultado. Uma configuracao mais barata
  offline aumentou variancia e piorou p99 sob carga.
- Desligar GC cedo demais piorou a cauda em uma configuracao intermediaria. O
  tuning de runtime so faz sentido medido na topologia completa.
- SIMD precisa ser validado fim a fim. SSE4.1 ficou estavel; AVX2 ganhou pouco
  no microbench local e piorou no teste completo desse ambiente.
- Distribuir CPU entre LB e APIs nao e neutro: dar `0.05` ao LB parecia
  conservador, mas era exatamente o gargalo escondido. O re-split com mais
  cota no LB reduziu p99 em uma ordem de grandeza sem mudar uma linha de
  codigo.
- Medicao de cgroup via `docker exec` em containers de cota pequena distorce
  o que se quer medir: o proprio `cat /sys/fs/cgroup/cpu.stat` consome
  fracao relevante da janela de `100ms`. Diagnostico ficou confiavel apenas
  apos retirar o sampler do haproxy do circuito.

## Proximos pontos de melhoria

- Investigar re-rank adicional para tentar zerar os falsos positivos restantes.
- Reavaliar kernels vetoriais em hardware diferente (AVX2 nativo).
- Avaliar se reduzir CPU adicional do LB sem ganho marginal libera espaco
  para outras melhorias de runtime/config.

## Como rodar

Testes:

```sh
go test ./...
```

Gerar o indice usado pela imagem Docker:

```sh
go run ./cmd/preprocess \
  -input data/references.json.gz \
  -output data/references.bin \
  -format kmeans-ivf-int16 \
  -nlist 2048 \
  -kmeans-iter 8 \
  -kmeans-sample 20000
```

Subir a stack:

```sh
docker compose up -d --build
```

Validar:

```sh
curl -i http://127.0.0.1:9999/ready
curl -i -X POST http://127.0.0.1:9999/fraud-score \
  -H 'Content-Type: application/json' \
  -d '{"id":"tx-1"}'
```

Parar:

```sh
docker compose down
```
