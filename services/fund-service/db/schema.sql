CREATE TABLE IF NOT EXISTS investment_funds (
    id                   BIGSERIAL PRIMARY KEY,
    name                 VARCHAR(100)   UNIQUE NOT NULL,
    description          TEXT,
    minimum_contribution DECIMAL(18,4)  NOT NULL,
    manager_id           BIGINT         NOT NULL,
    liquid_assets        DECIMAL(18,4)  NOT NULL DEFAULT 0,
    account_id           BIGINT,
    created_at           TIMESTAMP      NOT NULL DEFAULT NOW(),
    active               BOOLEAN        NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS client_fund_positions (
    id                    BIGSERIAL PRIMARY KEY,
    client_id             BIGINT         NOT NULL,
    client_type           VARCHAR(10)    NOT NULL DEFAULT 'CLIENT',
    fund_id               BIGINT         NOT NULL REFERENCES investment_funds(id),
    total_invested_amount DECIMAL(18,4)  NOT NULL DEFAULT 0,
    last_modified_at      TIMESTAMP      NOT NULL DEFAULT NOW(),
    UNIQUE(client_id, client_type, fund_id)
);

CREATE TABLE IF NOT EXISTS fund_portfolio_positions (
    id               BIGSERIAL PRIMARY KEY,
    fund_id          BIGINT        NOT NULL REFERENCES investment_funds(id),
    listing_id       BIGINT        NOT NULL,
    quantity         NUMERIC(20,6) NOT NULL DEFAULT 0,
    average_cost     DECIMAL(18,4) NOT NULL DEFAULT 0,
    acquisition_date DATE          NOT NULL DEFAULT CURRENT_DATE,
    UNIQUE(fund_id, listing_id)
);

CREATE TABLE IF NOT EXISTS client_fund_transactions (
    id                     BIGSERIAL PRIMARY KEY,
    client_id              BIGINT        NOT NULL,
    client_type            VARCHAR(10)   NOT NULL DEFAULT 'CLIENT',
    fund_id                BIGINT        NOT NULL REFERENCES investment_funds(id),
    amount                 DECIMAL(18,4) NOT NULL,
    is_inflow              BOOLEAN       NOT NULL,
    status                 VARCHAR(20)   NOT NULL DEFAULT 'PENDING',
    destination_account_id BIGINT,
    timestamp              TIMESTAMP     NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fund_performance_history (
    id         BIGSERIAL PRIMARY KEY,
    fund_id    BIGINT        NOT NULL REFERENCES investment_funds(id),
    date       DATE          NOT NULL,
    fund_value DECIMAL(18,4) NOT NULL,
    profit     DECIMAL(18,4) NOT NULL,
    UNIQUE(fund_id, date)
);
