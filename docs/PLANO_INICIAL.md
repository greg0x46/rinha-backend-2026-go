# Plano inicial de implementacao

## Objetivo

Construir uma submissao em Go para a Rinha de Backend 2026 focada em cumprir o contrato oficial e competir bem em latencia, memoria e qualidade de deteccao.

O backend deve responder:

- `GET /ready`
- `POST /fraud-score`

Na submissao final, o `docker-compose.yml` deve expor a porta `9999` via load balancer e distribuir chamadas para pelo menos duas instancias da API.

## Requisitos oficiais que guiam a arquitetura

- Porta publica: `9999`.
- Topologia minima: 1 load balancer + 2 APIs.
- Limite total: 1 CPU e 350 MB de memoria.
- Rede Docker: `bridge`.
- Imagens publicas e compativeis com `linux-amd64`.
- `POST /fraud-score` deve retornar HTTP 200 com:

```json
{
  "approved": true,
  "fraud_score": 0.0
}
```

- Regra de decisao:
  - vetorizar a transacao em 14 dimensoes;
  - encontrar os 5 vizinhos mais proximos;
  - `fraud_score = quantidade_de_fraudes / 5`;
  - `approved = fraud_score < 0.6`.

## Estrategia tecnica

### 1. Vetorizacao correta antes de otimizar

Implementar primeiro a transformacao oficial do payload em vetor de 14 dimensoes:

1. `amount`
2. `installments`
3. `amount_vs_avg`
4. `hour_of_day`
5. `day_of_week`
6. `minutes_since_last_tx`
7. `km_from_last_tx`
8. `km_from_home`
9. `tx_count_24h`
10. `is_online`
11. `card_present`
12. `unknown_merchant`
13. `mcc_risk`
14. `merchant_avg_amount`

Pontos sensiveis:

- usar UTC nos timestamps;
- segunda-feira = 0 e domingo = 6;
- `last_transaction: null` deve gerar `-1` nos indices 5 e 6;
- aplicar clamp `[0.0, 1.0]` onde a regra manda;
- usar risco MCC padrao `0.5` quando o MCC nao estiver em `mcc_risk.json`;
- `unknown_merchant = 1` quando `merchant.id` nao aparece em `customer.known_merchants`.

### 2. Baseline exato

Criar primeiro uma versao correta com busca brute force sobre um arquivo binario pre-processado.

Mesmo que ela nao seja competitiva, ela serve para:

- validar vetorizacao;
- medir qualidade perfeita contra a regra oficial;
- comparar qualquer busca aproximada futura;
- encontrar bugs de parser, distancia e top 5.

Otimizacoes basicas do baseline:

- comparar distancia quadrada, sem `sqrt`;
- manter top 5 em array fixo;
- evitar alocacoes por request;
- armazenar labels como `uint8`.

### 3. Pre-processamento do dataset

Converter `../resources/references.json.gz` para formato binario durante build ou etapa local.

Primeira versao:

```text
[float32; 14] + uint8 label
```

Memoria aproximada:

```text
3.000.000 * 14 * 4 = 168 MB so de vetores
```

Com duas instancias isso pode estourar o limite. Por isso, a segunda versao deve usar quantizacao:

```text
[int16; 14] + uint8 label
```

Memoria aproximada:

```text
3.000.000 * 14 * 2 = 84 MB so de vetores
```

O objetivo e deixar a API com memoria previsivel e barata o bastante para rodar duas instancias sob 350 MB totais.

### 4. Busca candidata antes do rerank

Brute force completo por request custa `3M * 14`, o que tende a ser caro demais no limite de 1 CPU.

Plano de busca competitiva:

1. Pre-processar os vetores em buckets ou listas invertidas.
2. Em runtime, selecionar um conjunto pequeno de candidatos.
3. Calcular distancia exata so nesses candidatos.
4. Retornar os 5 vizinhos mais proximos dentro do conjunto.

Heuristicas iniciais para buckets:

- `last_transaction` ausente ou presente;
- `is_online`;
- `card_present`;
- `unknown_merchant`;
- faixa de `amount`;
- faixa de `amount_vs_avg`;
- faixa de `km_from_home`;
- faixa de `mcc_risk`.

Alternativa seguinte: IVF offline com centroides e rerank exato das listas mais proximas.

### 5. Runtime da API

O caminho quente do `POST /fraud-score` deve ser curto:

1. Parse JSON.
2. Montar vetor em array fixo.
3. Selecionar candidatos.
4. Calcular top 5.
5. Responder JSON.

Regras operacionais:

- sem log por request;
- sem dependencias pesadas;
- evitar reflection no caminho quente quando possivel;
- retornar HTTP 200 sempre que houver payload minimamente parseavel;
- preferir fallback com resposta valida a HTTP 500, pois erro HTTP pesa mais na avaliacao.

### 6. Docker Compose

Topologia alvo:

```text
nginx:9999
  -> api1
  -> api2
```

Orcamento inicial de recursos:

```text
nginx: 0.05 CPU, 16-24 MB
api1: 0.475 CPU, 150-165 MB
api2: 0.475 CPU, 150-165 MB
```

Isso sera ajustado por medicao real.

### 7. Testes

Testes minimos:

- vetorizacao com exemplos dos docs;
- `last_transaction: null`;
- fallback de MCC;
- comerciante conhecido/desconhecido;
- top 5 com `resources/example-references.json`;
- API retornando HTTP 200 e JSON valido.

Testes de desempenho:

- carga local com o script em `../test`;
- medir p99;
- medir RSS das duas APIs e do load balancer;
- medir tempo de startup ate `/ready`.

## Fases de execucao

### Fase 1: base correta

- Criar servidor HTTP em Go.
- Implementar structs do payload.
- Implementar vetorizacao.
- Implementar `/ready`.
- Implementar `/fraud-score` com referencia pequena para validar.

### Fase 2: dataset binario

- Criar comando de pre-processamento.
- Converter `references.json.gz`.
- Implementar loader binario.
- Implementar brute force exato.

### Fase 3: compose

- Criar Dockerfile.
- Criar `docker-compose.yml`.
- Criar config de nginx.
- Rodar duas instancias locais.

### Fase 4: performance

- Adicionar quantizacao.
- Adicionar buckets ou IVF.
- Medir impacto em p99 e acuracia.
- Ajustar candidatos por bucket/lista.

### Fase 5: submissao

- Criar branch `submission`.
- Manter apenas arquivos necessarios para executar.
- Garantir `docker-compose.yml` na raiz.
- Adicionar `info.json`.
- Validar imagem `linux-amd64`.

## Decisoes iniciais

- Linguagem: Go.
- Versao alvo: Go 1.26.2.
- API: `net/http` inicialmente.
- Formato de indice: binario local.
- Busca inicial: brute force para baseline.
- Busca competitiva: bucketizacao ou IVF com rerank exato.
- Load balancer: nginx, salvo medicao indicar alternativa melhor.
