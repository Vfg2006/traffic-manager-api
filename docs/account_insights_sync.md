# Serviço de Sincronização de Insights de Contas

Este documento descreve o funcionamento e a configuração do serviço de sincronização de insights para contas de anúncios do Meta.

## Visão Geral

O Serviço de Sincronização de Insights (`InsightSyncService`) é um componente que executa em segundo plano para:

1. Obter dados de insights de forma programada (via cron) para todas as contas ativas
2. Armazenar os dados em cache no banco de dados
3. Otimizar o desempenho e reduzir chamadas à API do Meta
4. Manter um histórico permanente de dados de insights para análise e relatórios

Este serviço foi projetado para minimizar o tempo de resposta das consultas de insights e reduzir a carga nas APIs externas (Meta e SSOtica), evitando bloqueios por excesso de requisições.

## Arquitetura

O serviço é composto por:

1. **Tabela de cache no banco de dados** (`account_insights`)
2. **Repositório dedicado** (`AccountInsightRepository`)
3. **Serviço de agendamento** (`InsightSyncService`)
4. **Integração com o serviço de insights** existente via cache

## Configuração

O serviço é configurável através de variáveis de ambiente e utiliza a configuração central do sistema:

| Variável | Descrição | Valor Padrão |
|----------|-----------|--------------|
| `INSIGHT_SYNC_CRON` | Programação cron para sincronização | `0 3 * * *` (3h da manhã) |
| `INSIGHT_SYNC_LOOKBACK_DAYS` | Dias anteriores para buscar dados | `7` |
| `INSIGHT_SYNC_REQUEST_DELAY_SECONDS` | Segundos de espera entre requisições | `2` |
| `INSIGHT_SYNC_MAX_CONCURRENT_JOBS` | Máximo de jobs concorrentes | `3` |
| `INSIGHT_SYNC_ENABLED` | Habilitar a sincronização | `true` |

## Funcionamento

### Sincronização Agendada

O serviço executa diariamente (ou conforme configurado) para:

1. Buscar todas as contas ativas no sistema
2. Para cada dia no período de lookback (default: 7 dias):
   - Para cada conta, buscar insights da data específica
   - Salvar os dados no cache

### Retenção de Dados Históricos

* Todos os dados de insights são mantidos permanentemente no banco de dados
* Não há processo automático de limpeza, permitindo análises históricas completas
* Os dados acumulados permanecem disponíveis para consultas e relatórios históricos

### Mecanismo de Cache

O serviço de insights foi modificado para:

1. Verificar se o cache existe para a data e conta solicitada
2. Usar o cache se disponível
3. Buscar da API em caso de cache miss ou período maior que um dia

## Benefícios

1. **Melhor desempenho**: respostas instantâneas para dados em cache
2. **Redução de carga**: menos chamadas para APIs externas
3. **Histórico completo de dados**: disponibilidade permanente de dados históricos 
4. **Resiliência**: funcionamento mesmo quando as APIs externas estão instáveis

## Logs

O serviço gera logs detalhados para monitoramento:

1. Início e conclusão de cada ciclo de sincronização
2. Contas e datas processadas
3. Erros de processamento ou API
4. Estatísticas de uso do cache

Exemplo de logs durante a sincronização:
```
[INFO] Iniciando sincronização de insights para todas as contas ativas
[INFO] Contas encontradas para sincronização de insights (total_accounts=85, active_accounts=72)
[INFO] Período para sincronização de insights (days=7, start_date=2023-06-15, end_date=2023-06-21)
[INFO] Processando insights para data 2023-06-21
[INFO] Obtendo insights para conta e data (account_id=abc123, external_id=123456789, account_name="IVS Example", date=2023-06-21)
[INFO] Insights salvos com sucesso para conta e data (account_id=abc123, external_id=123456789, date=2023-06-21)
...
[INFO] Sincronização de insights concluída (duration=15m42s, accounts=72, days=7)
```

## Monitoramento

O status da sincronização pode ser verificado através do método `GetStatus()` que retorna:

- Estado atual do processo de sincronização
- Configurações ativas
- Estatísticas gerais

## Considerações

1. **Volume de dados**: O volume de dados aumentará com o tempo, já que todos os insights são preservados
2. **Desempenho do banco**: O uso de índices adequados é essencial para consultas rápidas mesmo com grandes volumes
3. **Limites de API**: O serviço respeita limites da API utilizando delays entre requisições 