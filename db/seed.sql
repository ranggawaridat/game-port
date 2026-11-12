-- =====================================================================
-- seed.sql — Skema + data awal GamePort
--
-- Idempoten: boleh dijalankan berulang. Tabel referensi memakai
-- ON CONFLICT, tabel konten di-TRUNCATE lalu diisi ulang dari data
-- yang mengikuti sumber (awesome-android-native-ports).
--
-- Dipakai oleh container `db` saat pertama kali inisialisasi
-- (lihat db/init.sql di docker-compose), atau manual:
--   psql "$DATABASE_URL" -f db/seed.sql
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

CREATE TABLE IF NOT EXISTS engines (
    slug TEXT PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS features (
    slug TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    ikon TEXT NOT NULL DEFAULT '•'
);

-- ---------- Konten ----------
CREATE TABLE IF NOT EXISTS ports (
    id             BIGSERIAL PRIMARY KEY,
    slug           TEXT NOT NULL UNIQUE,
    game           TEXT NOT NULL,
    game_sort      TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    author         TEXT NOT NULL DEFAULT '',
    author_url     TEXT,
    repo_url       TEXT NOT NULL,
    category_slug  TEXT NOT NULL REFERENCES categories(slug),
    status_slug    TEXT NOT NULL REFERENCES statuses(slug),
    needs_own_data BOOLEAN NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS port_platforms (
    port_id       BIGINT NOT NULL REFERENCES ports(id) ON DELETE CASCADE,
    platform_slug TEXT   NOT NULL REFERENCES platforms(slug),
    PRIMARY KEY (port_id, platform_slug)
);

CREATE TABLE IF NOT EXISTS port_engines (
    port_id     BIGINT NOT NULL REFERENCES ports(id) ON DELETE CASCADE,
    engine_slug TEXT   NOT NULL REFERENCES engines(slug),
    PRIMARY KEY (port_id, engine_slug)
);

CREATE TABLE IF NOT EXISTS port_features (
    port_id      BIGINT NOT NULL REFERENCES ports(id) ON DELETE CASCADE,
    feature_slug TEXT   NOT NULL REFERENCES features(slug),
    PRIMARY KEY (port_id, feature_slug)
);

CREATE TABLE IF NOT EXISTS etl_runs (
    id           BIGSERIAL PRIMARY KEY,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at  TIMESTAMPTZ,
    source_url   TEXT NOT NULL,
    total_baris  INT  NOT NULL DEFAULT 0,
    total_insert INT  NOT NULL DEFAULT 0,
    total_update INT  NOT NULL DEFAULT 0,
    catatan      TEXT
);

-- ---------- Index ----------
CREATE INDEX IF NOT EXISTS idx_ports_category ON ports(category_slug);
CREATE INDEX IF NOT EXISTS idx_ports_status   ON ports(status_slug);
CREATE INDEX IF NOT EXISTS idx_ports_sort     ON ports(game_sort);
CREATE INDEX IF NOT EXISTS idx_ports_own      ON ports(needs_own_data);
CREATE INDEX IF NOT EXISTS idx_pp_platform    ON port_platforms(platform_slug);
CREATE INDEX IF NOT EXISTS idx_pe_engine      ON port_engines(engine_slug);

-- ---------- View ----------
CREATE OR REPLACE VIEW v_port_detail AS
SELECT
    p.id, p.slug, p.game, p.game_sort, p.description, p.author, p.author_url,
    p.repo_url, p.needs_own_data,
    c.slug AS category_slug, c.name AS category_name, c.urutan AS category_urutan,
    s.slug AS status_slug, s.name AS status_name, s.warna AS status_warna,
    s.bobot AS status_bobot,
    COALESCE((SELECT array_agg(pp.platform_slug ORDER BY pp.platform_slug)
              FROM port_platforms pp WHERE pp.port_id = p.id), '{}') AS platforms,
    COALESCE((SELECT array_agg(pe.engine_slug ORDER BY pe.engine_slug)
              FROM port_engines pe WHERE pe.port_id = p.id), '{}') AS engines,
    COALESCE((SELECT array_agg(pf.feature_slug ORDER BY pf.feature_slug)
              FROM port_features pf WHERE pf.port_id = p.id), '{}') AS features
FROM ports p
JOIN categories c ON c.slug = p.category_slug
JOIN statuses   s ON s.slug = p.status_slug;

CREATE OR REPLACE VIEW v_statistik AS
SELECT
    (SELECT count(*) FROM ports)                                    AS total_port,
    (SELECT count(*) FROM ports WHERE needs_own_data)               AS butuh_data_asli,
    (SELECT count(DISTINCT author) FROM ports)                      AS total_author,
    (SELECT count(*) FROM ports WHERE status_slug = 'stable')       AS stabil,
    (SELECT count(*) FROM ports WHERE status_slug = 'beta')         AS beta,
    (SELECT count(*) FROM ports WHERE status_slug = 'experimental') AS eksperimental,
    (SELECT count(*) FROM ports WHERE status_slug = 'alpha')        AS alpha;

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

INSERT INTO engines (slug, name) VALUES
 ('sdl2','SDL2'), ('sdl3','SDL3'), ('vulkan','Vulkan'),
 ('opengl-es','OpenGL ES'), ('dxvk','DXVK'), ('godot','Godot'),
 ('unity','Unity'), ('ndk','Android NDK'), ('rt64','RT64'),
 ('rexglue','ReXGlue'), ('wiicompiled','WiiCompiled')
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name;

INSERT INTO features (slug, name, ikon) VALUES
 ('touch controls','Kontrol Sentuh','👆'),
 ('controller support','Dukungan Controller','🎮'),
 ('gyro','Giroskop','🌀'),
 ('widescreen','Widescreen','🖥'),
 ('multiplayer','Multiplayer','👥'),
 ('mod support','Dukungan Mod','🧩')
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, ikon = EXCLUDED.ikon;

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

-- ---------- Seed konten (contoh awal) ----------
-- Data lengkap (210 port) dimuat oleh ETL: etl/etl.py
TRUNCATE port_platforms, port_engines, port_features, ports RESTART IDENTITY CASCADE;

INSERT INTO ports (slug, game, game_sort, description, author, author_url, repo_url,
                   category_slug, status_slug, needs_own_data) VALUES
 ('half-life-xash3d-fwgs','Half-Life (Xash3D FWGS)','half-life (xash3d fwgs)',
  'Custom GoldSrc-compatible engine that runs Half-Life and its mods, with a continuous Android APK build.',
  'FWGS','https://github.com/FWGS','https://github.com/FWGS/xash3d-fwgs',
  'source-port','stable',false),
 ('diablo-devilutionx','Diablo & Hellfire','diablo & hellfire',
  'Port of Diablo and Hellfire with engine improvements and an Android APK attached to every release.',
  'diasurgical','https://github.com/diasurgical','https://github.com/diasurgical/DevilutionX',
  'source-port','stable',false),
 ('openrct2','RollerCoaster Tycoon 2 (OpenRCT2)','rollercoaster tycoon 2 (openrct2)',
  'Open-source re-implementation of RollerCoaster Tycoon 2 with an Android APK attached to each release.',
  'OpenRCT2','https://github.com/OpenRCT2','https://github.com/OpenRCT2/OpenRCT2',
  'source-port','stable',false),
 ('barony-android','Barony','barony',
  'Beta native Android port of Barony; code-only APK that requires a purchased Barony v5.0.2 installation.',
  'DifferentNet','https://github.com/DifferentNet','https://github.com/DifferentNet/barony-android-port',
  'source-port','beta',true)
ON CONFLICT (slug) DO NOTHING;

-- Uji cepat: jumlah baris hasil seed
DO $$
DECLARE n int;
BEGIN
    SELECT count(*) INTO n FROM ports;
    RAISE NOTICE 'seed selesai: % port, % kategori', n, (SELECT count(*) FROM categories);
END $$;

COMMIT;