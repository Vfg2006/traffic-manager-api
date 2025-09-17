-- TRIGGER

-- Criar função para atualizar updated_at automaticamente
CREATE FUNCTION set_timestamp() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;


-- ROLES
CREATE TABLE roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE
);

INSERT INTO roles (name) VALUES ('admin'), ('manager'), ('supervisor'), ('customer');


-- USERS
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    lastname VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    active BOOLEAN DEFAULT TRUE,
    role_id INT,
    avatar_url VARCHAR(100),
    deleted BOOLEAN DEFAULT FALSE,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE SET NULL
);

-- Criar trigger para chamar a função antes de cada update
CREATE TRIGGER trigger_set_timestamp
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_timestamp();


-- STATUS
CREATE TYPE generic_status AS ENUM ('ACTIVE', 'INACTIVE');


-- BUSINESS MANAGER
CREATE TABLE business_manager (
    id CHAR(6) PRIMARY KEY,
    external_id VARCHAR(30),
    name VARCHAR(100) NOT NULL,
    origin VARCHAR(10) NOT NULL,
    status generic_status NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (external_id, origin)
);

-- Criar trigger para chamar a função antes de cada update
CREATE TRIGGER trigger_set_timestamp
BEFORE UPDATE ON business_manager
FOR EACH ROW
EXECUTE FUNCTION set_timestamp();


-- ACCOUNTS
CREATE TABLE accounts (
    id CHAR(6) PRIMARY KEY,
    external_id VARCHAR(30) NOT NULL,
    name VARCHAR(100) NOT NULL,
    nickname VARCHAR(100),
    business_id CHAR(6) NOT NULL,
    cnpj VARCHAR(14),
    secret_name VARCHAR(100),
    origin VARCHAR(10) NOT NULL,
    status generic_status NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (business_id) REFERENCES business_manager(id) ON DELETE SET NULL,
    UNIQUE (external_id, origin)
);

-- Criar trigger para chamar a função antes de cada update
CREATE TRIGGER trigger_set_timestamp
BEFORE UPDATE ON accounts
FOR EACH ROW
EXECUTE FUNCTION set_timestamp();

-- USER_ACCOUNTS RELATIONSHIP
-- Tabela para armazenar o relacionamento entre usuários e contas
CREATE TABLE user_accounts (
    user_id INT NOT NULL,
    account_id CHAR(6) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, account_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

CREATE TRIGGER update_user_accounts_timestamp
BEFORE UPDATE ON user_accounts
FOR EACH ROW
EXECUTE FUNCTION set_timestamp();

-- Índice para melhorar performance de consultas
CREATE INDEX idx_user_accounts_user_id ON user_accounts(user_id);
CREATE INDEX idx_user_accounts_account_id ON user_accounts(account_id); 


-- Criação da tabela para armazenar insights de anúncios
CREATE TABLE ad_insights (
    id SERIAL PRIMARY KEY,
    account_id CHAR(6) NOT NULL,
    external_id VARCHAR(30) NOT NULL,
    date DATE NOT NULL,
    ad_metrics JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    UNIQUE (account_id, date)
);

-- Criar trigger para atualizar o timestamp automaticamente
CREATE TRIGGER trigger_set_timestamp_ad_insights
BEFORE UPDATE ON ad_insights
FOR EACH ROW
EXECUTE FUNCTION set_timestamp();

-- Criação da tabela para armazenar insights de vendas
CREATE TABLE sales_insights (
    id SERIAL PRIMARY KEY,
    account_id CHAR(6) NOT NULL,
    date DATE NOT NULL,
    sales_metrics JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    UNIQUE (account_id, date)
);

-- Criar trigger para atualizar o timestamp automaticamente
CREATE TRIGGER trigger_set_timestamp_sales_insights
BEFORE UPDATE ON sales_insights
FOR EACH ROW
EXECUTE FUNCTION set_timestamp();

-- Índices para melhorar performance de consultas
CREATE INDEX idx_ad_insights_account_date ON ad_insights (account_id, date);
CREATE INDEX idx_ad_insights_external_id ON ad_insights (external_id);
CREATE INDEX idx_ad_insights_date ON ad_insights (date);

CREATE INDEX idx_sales_insights_account_date ON sales_insights (account_id, date);
CREATE INDEX idx_sales_insights_date ON sales_insights (date);

-- Comentários para documentação das tabelas
COMMENT ON TABLE ad_insights IS 'Armazena métricas de anúncios de contas por data';
COMMENT ON COLUMN ad_insights.ad_metrics IS 'Métricas de anúncios como JSON (impressões, cliques, gastos, etc.)';

COMMENT ON TABLE sales_insights IS 'Armazena métricas de vendas de contas por data';
COMMENT ON COLUMN sales_insights.sales_metrics IS 'Métricas de vendas como JSON (quantidade, valor, etc.)';

-- Criação da tabela para armazenar insights mensais de anúncios
CREATE TABLE monthly_ad_insights (
    id SERIAL PRIMARY KEY,
    account_id CHAR(6) NOT NULL,
    external_id VARCHAR(30) NOT NULL,
    period VARCHAR(7) NOT NULL, -- Formato mm-yyyy
    ad_metrics JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    UNIQUE (account_id, period)
);

-- Criar trigger para atualizar o timestamp automaticamente
CREATE TRIGGER trigger_set_timestamp_monthly_ad_insights
BEFORE UPDATE ON monthly_ad_insights
FOR EACH ROW
EXECUTE FUNCTION set_timestamp();

-- Criação da tabela para armazenar insights mensais de vendas
CREATE TABLE monthly_sales_insights (
    id SERIAL PRIMARY KEY,
    account_id CHAR(6) NOT NULL,
    period VARCHAR(7) NOT NULL, -- Formato mm-yyyy
    sales_metrics JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    UNIQUE (account_id, period)
);

-- Criar trigger para atualizar o timestamp automaticamente
CREATE TRIGGER trigger_set_timestamp_monthly_sales_insights
BEFORE UPDATE ON monthly_sales_insights
FOR EACH ROW
EXECUTE FUNCTION set_timestamp();

-- Índices para melhorar performance de consultas
CREATE INDEX idx_monthly_ad_insights_account_period ON monthly_ad_insights (account_id, period);
CREATE INDEX idx_monthly_ad_insights_period ON monthly_ad_insights (period);
CREATE INDEX idx_monthly_sales_insights_account_period ON monthly_sales_insights (account_id, period);
CREATE INDEX idx_monthly_sales_insights_period ON monthly_sales_insights (period);

-- Comentários para documentação das tabelas
COMMENT ON TABLE monthly_ad_insights IS 'Armazena métricas mensais agregadas de anúncios por conta';
COMMENT ON COLUMN monthly_ad_insights.ad_metrics IS 'Métricas de anúncios mensais como JSON (impressões, cliques, gastos, etc.)';
COMMENT ON COLUMN monthly_ad_insights.period IS 'Período no formato mm-yyyy';

COMMENT ON TABLE monthly_sales_insights IS 'Armazena métricas mensais agregadas de vendas por conta';
COMMENT ON COLUMN monthly_sales_insights.sales_metrics IS 'Métricas de vendas mensais como JSON (quantidade, valor, etc.)';
COMMENT ON COLUMN monthly_sales_insights.period IS 'Período no formato mm-yyyy'; 

-- STORE RANKING
CREATE TABLE store_ranking (
    id SERIAL PRIMARY KEY,
    account_id CHAR(6) NOT NULL,
    month VARCHAR(7) NOT NULL, -- Formato mm-yyyy (ex: 01-2024)
    store_name VARCHAR(100) NOT NULL,
    social_network_revenue DECIMAL(10, 2) NOT NULL,
    position INT NOT NULL,
    position_change INT NOT NULL,
    previous_position INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    UNIQUE (account_id, month)
);

-- Criar trigger para atualizar o timestamp automaticamente
CREATE TRIGGER trigger_set_timestamp_store_ranking
BEFORE UPDATE ON store_ranking
FOR EACH ROW
EXECUTE FUNCTION set_timestamp();

-- Índices para melhorar performance de consultas 
CREATE INDEX idx_store_ranking_account_id_month ON store_ranking (account_id, month);
