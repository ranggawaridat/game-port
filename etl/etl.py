#!/usr/bin/env python3
"""
etl.py — Crawl sumber data game port lalu muat ke PostgreSQL (gamedb).

Sumber: https://github.com/abduznik/awesome-android-native-ports
        (curated list port native Android dari game PC/konsol)

Pemakaian:
    python3 etl.py --dry-run          # crawl + parse, tampilkan ringkasan, tanpa simpan
    python3 etl.py                    # crawl + load ke database
    python3 etl.py --file X.md        # pakai file lokal (tanpa jaringan)

Kredensial dibaca dari /opt/data/.env: GAMEDB_HOST/PORT/NAME/USER/PASSWORD
"""

import argparse
import json
import os
import re
import sys
import unicodedata
import urllib.request
from collections import Counter

SOURCE_URL = "https://raw.githubusercontent.com/abduznik/awesome-android-native-ports/main/README.md"
ENV_PATH = os.environ.get("HERMES_ENV", "/opt/data/.env")

CATEGORY_MAP = {
    "Source Ports & Engine Reimplementations": ("source-port", "Source Port / Reimplementasi Engine", 1),
    "Decompilation-Based Ports": ("decompilation", "Port Berbasis Dekompilasi", 2),
    "N64: Recompiled": ("n64-recomp", "N64 Recompiled", 3),
    "Wii U, Wii, GameCube, DS & Switch Recompilations": ("nintendo-recomp", "Rekompilasi Nintendo", 4),
    "Xbox 360 Recompilations": ("xbox360-recomp", "Rekompilasi Xbox 360", 5),
    "Original Xbox, PlayStation, PSP & PC Recompilations": ("xbox-ps-psp-pc", "Xbox / PlayStation / PSP / PC", 6),
    "Handheld & 16-Bit Recompilations": ("handheld-16bit", "Handheld & 16-Bit", 7),
    "Games on Their Own Runtimes & Launchers": ("runtime-launcher", "Runtime & Launcher", 8),
    "Classic Ports (Low Activity)": ("classic", "Port Klasik (Aktivitas Rendah)", 9),
}
SKIP_CATEGORIES = {"Tools & Frameworks", "Related Lists"}

PLATFORM_PATTERNS = [
    ("android", r"\bandroid\b"), ("windows", r"\bwindows\b"), ("linux", r"\blinux\b"),
    ("macos", r"\bmacos\b|\bmac os\b"), ("ios", r"\bios\b|\biphone\b"),
    ("steam-deck", r"\bsteam deck\b"), ("web", r"\bbrowser\b"),
    ("quest", r"\bmeta quest\b|\bquest\b"), ("switch", r"\bswitch\b"),
]
ENGINE_PATTERNS = [
    ("sdl2", r"\bsdl2\b"), ("sdl3", r"\bsdl3\b"), ("vulkan", r"\bvulkan\b"),
    ("opengl-es", r"\bopengl es\b"), ("dxvk", r"\bdxvk\b"), ("godot", r"\bgodot\b"),
    ("unity", r"\bunity\b"), ("ndk", r"\bndk\b"), ("rt64", r"\brt64\b"),
    ("rexglue", r"\brexglue\b"), ("wiicompiled", r"\bwiicompiled\b"),
]
FEATURE_PATTERNS = [
    ("touch controls", r"\btouch\b"), ("controller support", r"controller|gamepad"),
    ("gyro", r"gyro"), ("widescreen", r"widescreen"),
    ("multiplayer", r"multiplayer|online play"), ("mod support", r"\bmods?\b"),
]


def log(msg):
    print(msg, flush=True)


def slugify(s):
    s = unicodedata.normalize("NFKD", s).encode("ascii", "ignore").decode()
    s = re.sub(r"[^\w\s-]", "", s).strip().lower()
    return re.sub(r"[-\s]+", "-", s) or "tanpa-nama"


def detect(desc, patterns):
    d = desc.lower()
    return [slug for slug, pat in patterns if re.search(pat, d)]


def detect_status(text):
    d = text.lower()
    if re.search(r"\balpha\b", d):
        return "alpha"
    if re.search(r"\bbeta\b", d):
        return "beta"
    if re.search(r"experimental|in-development|in development|early|prototype|work-in-progress|\bwip\b", d):
        return "experimental"
    return "stable"


def needs_own_data(desc):
    d = desc.lower()
    return bool(re.search(
        r"own (copy|rom|disc|game|files|original)|bring your own|requires your own|"
        r"no game data included|no game assets included|no copyrighted|you supply|you provide", d))


def fetch(url):
    req = urllib.request.Request(url, headers={"User-Agent": "hermes-game-port-etl/1.0"})
    with urllib.request.urlopen(req, timeout=60) as r:
        return r.read().decode("utf-8", errors="replace")


ROW_RE = re.compile(
    r"^\|\s*\[(?P<game>[^\]]+)\]\((?P<repo>https?://[^)]+)\)\s*"
    r"\|\s*(?P<desc>.*?)\s*"
    r"\|\s*\[(?P<author>[^\]]+)\]\((?P<author_url>[^)]+)\)\s*\|\s*$"
)


def parse(markdown):
    rows, seen, current, skipped = [], set(), None, 0
    for line in markdown.splitlines():
        if line.startswith("## "):
            heading = line[3:].strip()
            if heading in SKIP_CATEGORIES:
                current = None
            else:
                current = CATEGORY_MAP.get(heading)
            continue
        if not current:
            continue
        m = ROW_RE.match(line)
        if not m:
            continue
        d = m.groupdict()
        game = re.sub(r"\s+", " ", d["game"]).strip()
        desc = re.sub(r"\s+", " ", d["desc"]).strip()
        slug = slugify(game)
        base, n = slug, 2
        while slug in seen:
            slug = f"{base}-{n}"
            n += 1
        seen.add(slug)
        rows.append({
            "slug": slug,
            "game": game,
            "game_sort": game.lower(),
            "description": desc,
            "author": re.sub(r"\s+", " ", d["author"]).strip(),
            "author_url": d["author_url"],
            "repo_url": d["repo"],
            "category_slug": current[0],
            "status_slug": detect_status(f"{game} {desc}"),
            "needs_own_data": needs_own_data(desc),
            "platforms": detect(desc, PLATFORM_PATTERNS) or ["android"],
            "engines": detect(desc, ENGINE_PATTERNS),
            "features": detect(desc, FEATURE_PATTERNS),
        })
    return rows, skipped


def env(key, default=None):
    if not os.path.exists(ENV_PATH):
        return default
    for line in open(ENV_PATH, encoding="utf-8", errors="replace"):
        line = line.strip()
        if line.startswith(key + "="):
            return line.split("=", 1)[1].strip().strip('"').strip("'")
    return default


def connect():
    try:
        import psycopg
    except ImportError:
        log("ERROR: psycopg tidak terpasang.\n"
            "  PYTHONPATH=/opt/data/pylibs python3 -m pip install --target /opt/data/pylibs 'psycopg[binary]'")
        sys.exit(2)
    return psycopg.connect(
        host=env("GAMEDB_HOST", "postgres"), port=env("GAMEDB_PORT", "5432"),
        dbname=env("GAMEDB_NAME", "gamedb"), user=env("GAMEDB_USER"),
        password=env("GAMEDB_PASSWORD", ""), connect_timeout=10,
    )


def apply_schema(cur, schema_path):
    log(f"  menerapkan skema: {schema_path}")
    cur.execute(open(schema_path, encoding="utf-8").read())


def upsert(cur, rows):
    ins = upd = 0
    for r in rows:
        cur.execute("""
            INSERT INTO ports (slug, game, game_sort, description, author, author_url,
                               repo_url, category_slug, status_slug, needs_own_data)
            VALUES (%(slug)s, %(game)s, %(game_sort)s, %(description)s, %(author)s,
                    %(author_url)s, %(repo_url)s, %(category_slug)s, %(status_slug)s,
                    %(needs_own_data)s)
            ON CONFLICT (slug) DO UPDATE SET
                game = EXCLUDED.game,
                game_sort = EXCLUDED.game_sort,
                description = EXCLUDED.description,
                author = EXCLUDED.author,
                author_url = EXCLUDED.author_url,
                repo_url = EXCLUDED.repo_url,
                category_slug = EXCLUDED.category_slug,
                status_slug = EXCLUDED.status_slug,
                needs_own_data = EXCLUDED.needs_own_data,
                updated_at = now()
            RETURNING id, (xmax = 0) AS inserted
        """, r)
        port_id, was_insert = cur.fetchone()
        ins += 1 if was_insert else 0
        upd += 0 if was_insert else 1

        for table, col, key in (("port_platforms", "platform_slug", "platforms"),
                                ("port_engines", "engine_slug", "engines"),
                                ("port_features", "feature_slug", "features")):
            cur.execute(f"DELETE FROM {table} WHERE port_id = %s", (port_id,))
            for val in r[key]:
                cur.execute(f"INSERT INTO {table} (port_id, {col}) VALUES (%s, %s) "
                            f"ON CONFLICT DO NOTHING", (port_id, val))
    return ins, upd


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dry-run", action="store_true", help="crawl & parse tanpa menyimpan")
    ap.add_argument("--file", help="pakai markdown lokal, bukan unduh")
    ap.add_argument("--schema", help="terapkan schema.sql (destruktif: drop & create)")
    ap.add_argument("--out", help="simpan hasil parse sebagai JSON")
    args = ap.parse_args()

    log(f"[1/4] Sumber: {'file lokal ' + args.file if args.file else SOURCE_URL}")
    markdown = open(args.file, encoding="utf-8").read() if args.file else fetch(SOURCE_URL)
    log(f"      {len(markdown):,} karakter")

    log("[2/4] Parsing tabel…")
    rows, _ = parse(markdown)
    log(f"      {len(rows)} port terparse")
    for k, v in Counter(r["category_slug"] for r in rows).most_common():
        log(f"        {k:<20} {v}")
    log(f"      status: {dict(Counter(r['status_slug'] for r in rows))}")
    log(f"      butuh data asli: {sum(1 for r in rows if r['needs_own_data'])}")

    if args.out:
        json.dump(rows, open(args.out, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
        log(f"      JSON disimpan: {args.out}")

    if args.dry_run:
        log("[3/4] --dry-run: tidak menyimpan apa pun")
        return

    log("[3/4] Koneksi database…")
    conn = connect()
    cur = conn.cursor()
    cur.execute("select current_user, current_database()")
    u, db = cur.fetchone()
    log(f"      terhubung sebagai {u} @ {db}")

    if args.schema:
        log("[3b/4] Menerapkan skema (DESTRUKTIF)…")
        apply_schema(cur, args.schema)

    log("[4/4] Memuat data…")
    cur.execute("INSERT INTO etl_runs (source_url) VALUES (%s) RETURNING id", (SOURCE_URL,))
    run_id = cur.fetchone()[0]
    ins, upd = upsert(cur, rows)
    cur.execute("""UPDATE etl_runs SET finished_at = now(), total_baris = %s,
                   total_insert = %s, total_update = %s WHERE id = %s""",
                (len(rows), ins, upd, run_id))
    conn.commit()

    cur.execute("SELECT count(*) FROM ports")
    total = cur.fetchone()[0]
    log(f"      selesai: +{ins} baru, ~{upd} diperbarui, {total} total di tabel ports")
    conn.close()


if __name__ == "__main__":
    main()