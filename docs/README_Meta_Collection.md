# Meta API Collection - traffic Manager

Esta collection do Postman contém todas as chamadas para a API do Meta utilizadas na aplicação traffic Manager.

## 📋 Visão Geral

A collection está organizada em categorias que refletem as diferentes funcionalidades da integração com o Meta:

### 🔐 Autenticação e Tokens
- **Obter Token de Longa Duração**: Troca um token de curta duração por um token de longa duração
- **Verificar Validade do Token**: Verifica se o token atual é válido
- **Obter Informações de Debug do Token**: Obtém informações detalhadas sobre um token

### 🏢 Business Managers
- **Listar Business Managers**: Lista todos os Business Managers associados ao usuário

### 📊 Contas de Anúncios
- **Obter Contas de Anúncios por Business ID**: Obtém contas de anúncios de um Business Manager específico
- **Batch - Obter Contas de Anúncios**: Executa múltiplas requisições em lote

### 🎯 Campanhas
- **Obter Campanhas por Account ID**: Obtém campanhas ativas de uma conta de anúncios

### 📈 Insights de Contas de Anúncios
- **Obter Insights por Account ID**: Obtém insights básicos (alcance, impressões, frequência)
- **Obter Insights com Filtros Personalizados**: Obtém insights com parâmetros customizados

### 📊 Insights de Campanhas
- **Obter Insights de Campanha por ID**: Obtém insights detalhados de campanhas específicas

## 🚀 Como Usar

### 1. Importar a Collection
1. Abra o Postman
2. Clique em "Import"
3. Selecione o arquivo `Meta_API_Collection.postman_collection.json`

### 2. Configurar Variáveis de Ambiente
Antes de usar as requisições, configure as seguintes variáveis:

#### Variáveis Obrigatórias:
- `meta_access_token`: Seu token de acesso do Meta
- `meta_app_id`: ID da sua aplicação Meta
- `meta_app_secret`: Secret da sua aplicação Meta

#### Variáveis Opcionais (para testes específicos):
- `business_id`: ID do Business Manager
- `account_id`: ID da conta de anúncios
- `campaign_id`: ID da campanha
- `start_date`: Data de início (formato: YYYY-MM-DD)
- `end_date`: Data de fim (formato: YYYY-MM-DD)

### 3. Configurar Variáveis Globais
A collection já inclui as seguintes variáveis com valores padrão:
- `meta_base_url`: https://graph.facebook.com
- `meta_version`: v22.0

## 🔧 Configuração das Variáveis

### No Postman:
1. Clique no ícone de engrenagem (⚙️) no canto superior direito
2. Selecione "Manage Environments"
3. Crie um novo ambiente ou use o "Globals"
4. Configure as variáveis necessárias

### Exemplo de Configuração:
```
meta_access_token: EAABwzLixnjYBO...
meta_app_id: 123456789012345
meta_app_secret: abcdef123456789...
business_id: 123456789
account_id: act_123456789
campaign_id: 123456789
start_date: 2024-01-01
end_date: 2024-01-31
```

## 📝 Detalhes das Requisições

### Estrutura das URLs
Todas as requisições seguem o padrão:
```
https://graph.facebook.com/v22.0/{endpoint}?{parameters}&access_token={token}
```

### Parâmetros Comuns
- `access_token`: Token de acesso obrigatório
- `fields`: Campos específicos a serem retornados
- `time_range`: Período para insights (formato JSON)
- `filtering`: Filtros específicos (formato JSON)

### Formato do time_range
```json
{
  "since": "2024-01-01",
  "until": "2024-01-31"
}
```

### Formato do filtering
```json
[
  {
    "field": "objective",
    "operator": "IN",
    "value": ["OUTCOME_ENGAGEMENT"]
  }
]
```

## 🧪 Testes Automatizados

A collection inclui testes automáticos que verificam:
- Status code 200
- Presença da propriedade 'data' na resposta

## ⚠️ Limitações e Considerações

### Rate Limiting
- A API do Meta possui limites de taxa
- A aplicação implementa delays entre requisições (2 segundos por padrão)
- Use com moderação em testes

### Tokens
- Tokens de longa duração expiram em aproximadamente 60 dias
- A aplicação implementa renovação automática de tokens
- Sempre use tokens válidos para testes

### Paginação
- Algumas respostas podem incluir paginação
- A aplicação implementa loops para buscar todas as páginas
- Considere implementar paginação em testes extensos

## 🔍 Debug e Troubleshooting

### Verificar Token
Use a requisição "Verificar Validade do Token" para confirmar se seu token está funcionando.

### Debug Token
Use "Obter Informações de Debug do Token" para ver detalhes sobre permissões e expiração.

### Logs da Aplicação
A aplicação registra todas as chamadas para o Meta. Verifique os logs para identificar problemas.

## 📚 Documentação Adicional

- [Meta for Developers](https://developers.facebook.com/)
- [Graph API Reference](https://developers.facebook.com/docs/graph-api)
- [Marketing API](https://developers.facebook.com/docs/marketing-apis/)

## 🤝 Suporte

Para dúvidas sobre esta collection ou problemas com a integração:
1. Verifique os logs da aplicação
2. Consulte a documentação oficial do Meta
3. Entre em contato com a equipe de desenvolvimento

