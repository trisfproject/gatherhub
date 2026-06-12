-- GatherHub Initial Schema Migration
-- This file is executed by PostgreSQL on first container start

-- Ensure the database timezone is consistent
SET timezone = 'Asia/Jakarta';

-- Events table
CREATE TABLE IF NOT EXISTS events (
    id                     SERIAL PRIMARY KEY,
    title                  VARCHAR(255) NOT NULL,
    description            TEXT,
    event_date             TIMESTAMP WITH TIME ZONE NOT NULL,
    location               VARCHAR(500) NOT NULL,
    price                  NUMERIC(12, 2) NOT NULL DEFAULT 0,
    payment_bank           VARCHAR(100),
    payment_account_number VARCHAR(50),
    payment_account_name   VARCHAR(255),
    admin_name             VARCHAR(255) NOT NULL,
    admin_whatsapp         VARCHAR(20) NOT NULL,
    created_at             TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at             TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Participants table
CREATE TYPE participant_status AS ENUM ('PENDING', 'VERIFIED', 'REJECTED');

CREATE TABLE IF NOT EXISTS participants (
    id                 SERIAL PRIMARY KEY,
    event_id           INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    full_name          VARCHAR(255) NOT NULL,
    phone              VARCHAR(20) NOT NULL,
    email              VARCHAR(255) NOT NULL,
    city               VARCHAR(100) NOT NULL,
    company_name       VARCHAR(255),
    industrial_estate  VARCHAR(255),
    telegram_username  VARCHAR(100),
    job_title          VARCHAR(255),          -- nullable
    payment_proof      VARCHAR(500),
    status             participant_status NOT NULL DEFAULT 'PENDING',
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at         TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_participants_event_id ON participants(event_id);
CREATE INDEX IF NOT EXISTS idx_participants_status   ON participants(status);
CREATE INDEX IF NOT EXISTS idx_participants_email    ON participants(email);
