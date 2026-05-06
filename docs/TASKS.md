# Breakdown de tarefas

Este documento e o quadro operacional do projeto. O plano estrategico fica em `PLANO_INICIAL.md`; aqui ficam as tarefas executaveis, seus criterios de aceite e o status atual.

Legenda:

- `[ ]` pendente
- `[~]` em andamento
- `[x]` concluido

## Fase 0: fundacao do repositorio

Objetivo: ter um repositorio Go independente, ignorado pelo repositorio oficial da Rinha, com documentacao inicial e base de desenvolvimento.

- `[x]` Criar repositorio aninhado `rinha-be-2026`.
- `[x]` Inicializar Git dentro do repositorio aninhado.
- `[x]` Adicionar `rinha-be-2026/` ao `.gitignore` do repositorio pai.
- `[x]` Criar `README.md` da solucao.
- `[x]` Criar `docs/PLANO_INICIAL.md`.
- `[x]` Definir Go como linguagem principal.
- `[x]` Fixar versao alvo Go `1.26.2`.

Criterios de aceite:

- `git status` do repositorio pai nao lista arquivos internos da solucao.
- `git status` dentro de `rinha-be-2026` lista apenas arquivos da solucao.
- O plano inicial descreve requisitos, arquitetura e estrategia de performance.

## Fase 1: contrato HTTP e containers

Objetivo: validar cedo a topologia exigida pela Rinha antes de implementar a deteccao real.

- `[x]` Criar servidor HTTP em Go.
- `[x]` Implementar `GET /ready`.
- `[x]` Implementar `POST /fraud-score` com resposta fallback valida.
- `[x]` Criar testes unitarios basicos dos handlers.
- `[x]` Criar `Dockerfile` multi-stage.
- `[x]` Criar `docker-compose.yml`.
- `[x]` Criar `nginx.conf` com round-robin entre `api1` e `api2`.
- `[x]` Definir limites iniciais de CPU e memoria no Compose.
- `[x]` Validar `go test ./...`.
- `[x]` Validar `docker compose build`.
- `[x]` Validar `docker compose up -d`.
- `[x]` Validar `GET /ready` em `localhost:9999`.
- `[x]` Validar `POST /fraud-score` em `localhost:9999`.

Criterios de aceite:

- `/ready` responde HTTP `204`.
- `/fraud-score` responde HTTP `200`.
- Resposta de `/fraud-score` contem `approved` e `fraud_score`.
- Nginx expoe `9999` e encaminha para duas APIs.
- `docker compose config --quiet` passa.

## Fase 2: contrato de dominio e vetorizacao

Objetivo: transformar corretamente o payload oficial no vetor de 14 dimensoes.

- `[x]` Criar structs tipadas para o payload de `POST /fraud-score`.
- `[x]` Criar structs tipadas para a resposta.
- `[x]` Implementar carga de `resources/normalization.json`.
- `[x]` Implementar carga de `resources/mcc_risk.json`.
- `[x]` Implementar funcao `Clamp`.
- `[x]` Implementar calculo de `amount`.
- `[x]` Implementar calculo de `installments`.
- `[x]` Implementar calculo de `amount_vs_avg`.
- `[x]` Implementar calculo de `hour_of_day` em UTC.
- `[x]` Implementar calculo de `day_of_week` com segunda-feira = 0.
- `[x]` Implementar `minutes_since_last_tx`.
- `[x]` Implementar `km_from_last_tx`.
- `[x]` Implementar sentinela `-1` quando `last_transaction` for `null`.
- `[x]` Implementar `km_from_home`.
- `[x]` Implementar `tx_count_24h`.
- `[x]` Implementar `is_online`.
- `[x]` Implementar `card_present`.
- `[x]` Implementar `unknown_merchant`.
- `[x]` Implementar `mcc_risk` com fallback `0.5`.
- `[x]` Implementar `merchant_avg_amount`.
- `[x]` Cobrir vetorizacao com exemplos dos docs.
- `[x]` Cobrir casos de MCC desconhecido e comerciante conhecido/desconhecido.

Criterios de aceite:

- O exemplo legitimo de `REGRAS_DE_DETECCAO.md` gera vetor compativel com o documento.
- O exemplo fraudulento de `REGRAS_DE_DETECCAO.md` gera vetor compativel com o documento.
- `last_transaction: null` sempre gera `-1` nos indices 5 e 6.
- Todos os campos normalizados usam clamp quando a regra exige.

## Fase 3: busca exata baseline

Objetivo: ter uma implementacao correta de KNN para servir de baseline de qualidade.

- `[x]` Criar tipo `Vector [14]float32`.
- `[x]` Criar tipo `Reference` com vetor e label.
- `[x]` Ler `resources/example-references.json`.
- `[x]` Implementar distancia euclidiana quadrada.
- `[x]` Implementar top 5 com array fixo, sem ordenar dataset inteiro.
- `[x]` Calcular `fraud_score = fraudes / 5`.
- `[x]` Calcular `approved = fraud_score < 0.6`.
- `[x]` Integrar baseline ao endpoint com dataset pequeno.
- `[x]` Criar testes de KNN com dataset pequeno.

Criterios de aceite:

- A busca retorna os 5 menores por distancia quadrada.
- Nao usa `sqrt`.
- Nao aloca estruturas grandes por request.
- Endpoint retorna score derivado da busca baseline quando configurado com referencias pequenas.

## Fase 4: pre-processamento do dataset oficial

Objetivo: evitar parse de JSON gzip grande no caminho de execucao da API.

- `[x]` Criar comando `cmd/preprocess`.
- `[x]` Ler dataset oficial a partir de caminho interno/configuravel, sem exigir `..` no fluxo canonico.
- `[x]` Escrever formato binario `float32` inicial.
- `[x]` Escrever labels como `uint8`.
- `[x]` Criar manifest com quantidade de registros e versao do formato.
- `[x]` Implementar loader binario na API.
- `[x]` Remover dependencia runtime de caminhos `..`; carregar referencias de artefato interno da imagem ou caminho explicito via `REFERENCES_PATH`.
- `[x]` Validar quantidade esperada de 3.000.000 registros.
- `[x]` Medir tempo de pre-processamento.
- `[x]` Medir tempo de startup carregando arquivo binario.

Criterios de aceite:

- Arquivo binario e gerado de forma deterministica.
- Loader rejeita arquivo com manifest invalido.
- API fica pronta apenas apos carregar o indice.
- `/ready` responde `2xx` somente quando o indice estiver disponivel.

## Fase 5: baseline com dataset completo

Objetivo: medir a realidade de performance e memoria com busca exata completa.

- `[ ]` Integrar loader do dataset completo.
- `[ ]` Rodar brute force completo em uma API.
- `[ ]` Rodar brute force completo com duas APIs.
- `[ ]` Medir RSS por container.
- `[ ]` Medir p50, p95 e p99.
- `[ ]` Medir taxa de erro HTTP.
- `[ ]` Registrar resultados em `docs/RESULTADOS.md`.

Criterios de aceite:

- API responde sem erro HTTP sob teste local.
- Temos numeros de memoria e latencia para comparar contra otimizacoes.
- Qualidade do baseline e tratada como referencia para buscas aproximadas.

## Fase 6: reducao de memoria

Objetivo: encaixar duas APIs e o load balancer no limite de 350 MB.

- `[ ]` Definir estrategia de quantizacao.
- `[ ]` Implementar formato binario quantizado `int16` ou `uint16`.
- `[ ]` Adaptar distancia para vetor quantizado.
- `[ ]` Comparar resultado contra `float32`.
- `[ ]` Medir RSS por container.
- `[ ]` Ajustar limites de memoria no Compose.

Criterios de aceite:

- Duas APIs sobem dentro do limite planejado.
- A diferenca de decisao contra baseline `float32` e medida.
- A economia de memoria compensa a perda de precisao, se houver.

## Fase 7: busca candidata e performance

Objetivo: reduzir o numero de distancias calculadas por request mantendo boa qualidade de deteccao.

- `[ ]` Projetar buckets iniciais.
- `[ ]` Implementar geracao de buckets no pre-processamento.
- `[ ]` Implementar selecao de candidatos por bucket.
- `[ ]` Implementar fallback quando bucket tiver poucos candidatos.
- `[ ]` Fazer rerank exato dos candidatos.
- `[ ]` Medir p99 e qualidade contra baseline.
- `[ ]` Ajustar quantidade de candidatos.
- `[ ]` Avaliar IVF offline se bucketizacao nao for suficiente.

Criterios de aceite:

- P99 melhora de forma mensuravel contra brute force completo.
- Taxa de falhas de deteccao fica longe do corte de 15%.
- Nenhuma otimizacao aumenta erro HTTP.

## Fase 8: caminho quente da API

Objetivo: reduzir overhead por request depois que a estrategia de busca estiver definida.

- `[ ]` Trocar `map[string]any` por structs tipadas no handler.
- `[ ]` Avaliar parser JSON alternativo apenas se `encoding/json` aparecer como gargalo.
- `[ ]` Evitar logs por request.
- `[ ]` Reduzir alocacoes no vetor de consulta.
- `[ ]` Reutilizar buffers somente se isso nao criar risco de corrida.
- `[ ]` Adicionar timeouts coerentes no servidor e nginx.
- `[ ]` Garantir fallback HTTP 200 para falhas recuperaveis.

Criterios de aceite:

- Benchmark do handler mostra queda de alocacoes.
- Nenhum request valido retorna HTTP 500.
- O caminho quente permanece simples o suficiente para manutencao.

## Fase 9: teste local de carga

Objetivo: reproduzir localmente parte da avaliacao da Rinha.

- `[ ]` Rodar script de teste em `../test`.
- `[ ]` Coletar `results.json`.
- `[ ]` Registrar p99.
- `[ ]` Registrar matriz de confusao.
- `[ ]` Registrar score estimado.
- `[ ]` Medir containers durante carga.
- `[ ]` Ajustar recursos de CPU/memoria.

Criterios de aceite:

- Resultado local registrado em `docs/RESULTADOS.md`.
- Falhas HTTP iguais a zero ou justificadas.
- Decisoes de tuning baseadas em medicao.

## Fase 10: preparacao de submissao

Objetivo: deixar a branch `submission` no formato esperado pela Rinha.

- `[ ]` Criar `info.json`.
- `[ ]` Definir nome/id da submissao.
- `[ ]` Definir URL do repositorio publico.
- `[ ]` Garantir licenca MIT.
- `[ ]` Criar branch `submission`.
- `[ ]` Remover codigo-fonte da branch `submission`, mantendo apenas artefatos necessarios.
- `[ ]` Garantir `docker-compose.yml` na raiz da branch `submission`.
- `[ ]` Validar imagens publicas.
- `[ ]` Validar compatibilidade `linux-amd64`.
- `[ ]` Documentar comando de teste local.

Criterios de aceite:

- Branch `main` contem codigo-fonte.
- Branch `submission` contem apenas arquivos necessarios para execucao.
- Compose da branch `submission` sobe em ambiente limpo.
- Repositorio esta publico e apontado no JSON de participantes.

## Proxima tarefa recomendada

Comecar a Fase 2:

1. Criar os tipos do payload oficial.
2. Implementar vetorizacao pura e testavel.
3. Usar os exemplos de `REGRAS_DE_DETECCAO.md` como testes de regressao.
