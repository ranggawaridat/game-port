-- =====================================================================
-- seed.sql — Skema GamePort (mode manual, tanpa scraping)
--
-- Data diisi lewat admin panel atau langsung ke PostgreSQL.
-- Idempoten: aman dijalankan berulang.
-- =====================================================================

BEGIN;

-- ---------- Referensi ----------
CREATE TABLE IF NOT EXISTS categories (
    slug    TEXT PRIMARY KEY,
    name    TEXT NOT NULL,
    urutan  INT  NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS statuses (
    slug   TEXT PRIMARY KEY,
    name   TEXT NOT NULL,
    bobot  INT  NOT NULL DEFAULT 0,
    warna  TEXT NOT NULL DEFAULT '#6b7280'
);

CREATE TABLE IF NOT EXISTS platforms (
    slug TEXT PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS genres (
    slug TEXT PRIMARY KEY,
    name TEXT NOT NULL
);

-- ---------- Konten utama ----------
CREATE TABLE IF NOT EXISTS ports (
    id             BIGSERIAL PRIMARY KEY,
    slug           TEXT NOT NULL UNIQUE,
    game           TEXT NOT NULL,
    game_sort      TEXT NOT NULL,
    version        TEXT NOT NULL DEFAULT '',
    file_size      TEXT NOT NULL DEFAULT '',
    genre_slug     TEXT REFERENCES genres(slug),
    description    TEXT NOT NULL DEFAULT '',
    author         TEXT NOT NULL DEFAULT '',
    category_slug  TEXT NOT NULL REFERENCES categories(slug),
    status_slug    TEXT NOT NULL REFERENCES statuses(slug),
    image_url      TEXT,
    download_url   TEXT,
    iso_url        TEXT,
    source_url     TEXT,
    install_notes  TEXT,
    view_count     BIGINT NOT NULL DEFAULT 0,
    download_count BIGINT NOT NULL DEFAULT 0,
    published      BOOLEAN NOT NULL DEFAULT true,
    needs_own_data BOOLEAN NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Migrasi untuk database yang sudah ada sebelum kolom iso_url ditambahkan.
-- Idempoten: aman dijalankan berulang.
ALTER TABLE ports ADD COLUMN IF NOT EXISTS iso_url TEXT;

-- Link sumber (repo/halaman asli port) dan catatan instalasi.
-- Keduanya opsional; dipakai halaman detail bila diisi.
ALTER TABLE ports ADD COLUMN IF NOT EXISTS source_url    TEXT;
ALTER TABLE ports ADD COLUMN IF NOT EXISTS install_notes TEXT;

-- Token sekali pakai untuk safelink: unduhan hanya boleh lewat gerbang iklan.

-- Nonce yang diterbitkan saat halaman gerbang dibuka. Mint token wajib
-- menyertakan nonce ini, sehingga pemanggil harus benar-benar membuka
-- halaman (yang memuat iklan) sebelum bisa menebus unduhan.
CREATE TABLE IF NOT EXISTS safelink_pages (
    nonce      TEXT PRIMARY KEY,
    port_id    BIGINT NOT NULL REFERENCES ports(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('port','iso')),
    ip_hash    TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    used_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_slpage_created ON safelink_pages(created_at);

CREATE TABLE IF NOT EXISTS download_tokens (
    id         BIGSERIAL PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    port_id    BIGINT NOT NULL REFERENCES ports(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('port','iso')),
    ip_hash    TEXT,
    issued_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_dltoken_expiry ON download_tokens(expires_at);

-- Pengaturan sederhana key/value, dipakai untuk script iklan safelink
-- yang bisa diubah dari panel admin tanpa deploy.
CREATE TABLE IF NOT EXISTS settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO settings (key, value) VALUES ('safelink_ad_html', '')
ON CONFLICT (key) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_ports_category ON ports(category_slug);
CREATE INDEX IF NOT EXISTS idx_ports_status   ON ports(status_slug);
CREATE INDEX IF NOT EXISTS idx_ports_genre    ON ports(genre_slug);
CREATE INDEX IF NOT EXISTS idx_ports_sort     ON ports(game_sort);
CREATE INDEX IF NOT EXISTS idx_ports_pub      ON ports(published);

CREATE TABLE IF NOT EXISTS port_platforms (
    port_id       BIGINT NOT NULL REFERENCES ports(id) ON DELETE CASCADE,
    platform_slug TEXT   NOT NULL REFERENCES platforms(slug),
    PRIMARY KEY (port_id, platform_slug)
);
CREATE INDEX IF NOT EXISTS idx_pp_platform ON port_platforms(platform_slug);

-- ---------- Analitik ----------
CREATE TABLE IF NOT EXISTS events (
    id         BIGSERIAL PRIMARY KEY,
    kind       TEXT NOT NULL,          -- view | download | search
    port_id    BIGINT REFERENCES ports(id) ON DELETE SET NULL,
    query      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_events_kind_time ON events(kind, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_port      ON events(port_id);

-- ---------- Sesi admin ----------
CREATE TABLE IF NOT EXISTS admin_sessions (
    token      TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_exp ON admin_sessions(expires_at);

-- ---------- View ----------
DROP VIEW IF EXISTS v_port_detail;
CREATE VIEW v_port_detail AS
SELECT
    p.id, p.slug, p.game, p.game_sort, p.version, p.file_size, p.description,
    p.author, p.image_url, p.download_url, p.iso_url,
    p.source_url, p.install_notes,
    p.view_count, p.download_count,
    p.published, p.needs_own_data, p.created_at, p.updated_at,
    c.slug AS category_slug, c.name AS category_name, c.urutan AS category_urutan,
    s.slug AS status_slug, s.name AS status_name, s.warna AS status_warna,
    s.bobot AS status_bobot,
    g.slug AS genre_slug, g.name AS genre_name,
    COALESCE((SELECT array_agg(pp.platform_slug ORDER BY pp.platform_slug)
              FROM port_platforms pp WHERE pp.port_id = p.id), '{}') AS platforms
FROM ports p
JOIN categories c ON c.slug = p.category_slug
JOIN statuses   s ON s.slug = p.status_slug
LEFT JOIN genres g ON g.slug = p.genre_slug;

DROP VIEW IF EXISTS v_statistik;
CREATE VIEW v_statistik AS
SELECT
    (SELECT count(*) FROM ports)                                    AS total_port,
    (SELECT count(*) FROM ports WHERE published)                    AS terbit,
    (SELECT count(DISTINCT author) FROM ports)                      AS total_author,
    (SELECT count(*) FROM ports WHERE status_slug = 'stable')       AS stabil,
    (SELECT count(*) FROM ports WHERE status_slug = 'beta')         AS beta,
    (SELECT count(*) FROM ports WHERE status_slug = 'experimental') AS eksperimental,
    (SELECT count(*) FROM ports WHERE status_slug = 'alpha')        AS alpha,
    (SELECT COALESCE(sum(view_count),0) FROM ports)                 AS total_view,
    (SELECT COALESCE(sum(download_count),0) FROM ports)             AS total_download;

-- ---------- Seed referensi ----------
INSERT INTO statuses (slug, name, bobot, warna) VALUES
 ('stable',       'Stabil',       4, '#16a34a'),
 ('beta',         'Beta',         3, '#d97706'),
 ('experimental', 'Eksperimental',2, '#ea580c'),
 ('alpha',        'Alpha',        1, '#dc2626')
ON CONFLICT (slug) DO UPDATE
 SET name = EXCLUDED.name, bobot = EXCLUDED.bobot, warna = EXCLUDED.warna;

INSERT INTO platforms (slug, name) VALUES
 ('android','Android'), ('windows','Windows'), ('linux','Linux'),
 ('macos','macOS'), ('ios','iOS'), ('steam-deck','Steam Deck'),
 ('web','Web'), ('quest','Meta Quest'), ('switch','Switch')
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name;

INSERT INTO genres (slug, name) VALUES
 ('action','Action'), ('adventure','Adventure'), ('rpg','RPG'),
 ('strategy','Strategy'), ('shooter','Shooter'), ('racing','Racing'),
 ('sports','Sports'), ('simulation','Simulation'), ('puzzle','Puzzle'),
 ('platformer','Platformer'), ('fighting','Fighting'), ('horror','Horror'),
 ('sandbox','Sandbox'), ('roguelike','Roguelike'), ('rhythm','Rhythm'),
 ('card','Card'), ('board','Board'), ('mmo','MMO')
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name;

INSERT INTO categories (slug, name, urutan) VALUES
 ('source-port',      'Source Port / Reimplementasi Engine', 1),
 ('decompilation',    'Port Berbasis Dekompilasi',           2),
 ('n64-recomp',       'N64 Recompiled',                      3),
 ('nintendo-recomp',  'Rekompilasi Nintendo',                4),
 ('xbox360-recomp',   'Rekompilasi Xbox 360',                5),
 ('xbox-ps-psp-pc',   'Xbox / PlayStation / PSP / PC',       6),
 ('handheld-16bit',   'Handheld & 16-Bit',                   7),
 ('runtime-launcher', 'Runtime & Launcher',                  8),
 ('classic',          'Port Klasik (Aktivitas Rendah)',      9)
ON CONFLICT (slug) DO UPDATE
 SET name = EXCLUDED.name, urutan = EXCLUDED.urutan;

COMMIT;