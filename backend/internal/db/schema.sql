CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    email       TEXT        NOT NULL UNIQUE,
    password    TEXT        NOT NULL,
    role        TEXT        NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS expenses (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        TEXT        NOT NULL CHECK (type IN ('expense', 'income')),
    amount      NUMERIC     NOT NULL,
    category    TEXT        NOT NULL,
    tags        TEXT[]      NOT NULL DEFAULT '{}',
    notes       TEXT        NOT NULL DEFAULT '',
    date        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_expenses_user ON expenses(user_id);

CREATE TABLE IF NOT EXISTS budgets (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category      TEXT        NOT NULL,
    limit_amount  NUMERIC     NOT NULL,
    month         INT         NOT NULL,
    year          INT         NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, category, month, year)
);

CREATE TABLE IF NOT EXISTS groups (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    created_by  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS group_members (
    group_id    UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, user_id)
);

CREATE TABLE IF NOT EXISTS group_expenses (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id    UUID        NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    paid_by     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    description TEXT        NOT NULL,
    amount      NUMERIC     NOT NULL,
    split_type  TEXT        NOT NULL DEFAULT 'equal' CHECK (split_type IN ('equal', 'unequal', 'percentage')),
    date        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_group_expenses_group ON group_expenses(group_id);

CREATE TABLE IF NOT EXISTS split_details (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_expense_id  UUID        NOT NULL REFERENCES group_expenses(id) ON DELETE CASCADE,
    user_id           UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount            NUMERIC     NOT NULL,
    percentage        NUMERIC     NOT NULL DEFAULT 0,
    is_paid           BOOLEAN     NOT NULL DEFAULT FALSE,
    paid_at           TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_split_details_expense ON split_details(group_expense_id);

CREATE TABLE IF NOT EXISTS notifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message     TEXT        NOT NULL,
    type        TEXT        NOT NULL CHECK (type IN ('budget_breach', 'budget_warning', 'large_transaction', 'group_activity')),
    is_read     BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id);
