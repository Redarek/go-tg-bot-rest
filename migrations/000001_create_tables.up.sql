CREATE TABLE IF NOT EXISTS promotions (
                                             id SERIAL PRIMARY KEY,
                                             name TEXT UNIQUE NOT NULL,
                                             value TEXT NOT NULL,
                                             image_url TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS user_claims (
                                           user_id BIGINT PRIMARY KEY
);
CREATE TABLE IF NOT EXISTS admin_states (
                                            user_id BIGINT PRIMARY KEY,
                                            state TEXT NOT NULL,
                                            data TEXT
);

CREATE TABLE bot_users (
                           user_id BIGINT PRIMARY KEY,
                           created_at TIMESTAMPTZ DEFAULT now()
);