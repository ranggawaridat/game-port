#!/usr/bin/env python3
"""seed-napl.py — isi katalog GamePort dari NAPL (Native Android Port List).

Sumber: Google Sheets yang dipakai fe2rr.github.io/public — CSV publik,
1.100+ baris. Kolomnya: PORTS_ID, PLATFORM, GAME, PROJECT, URL, DEVELOPER,
VERSION, LAST UPDATE, NEEDS GAME FILES?, DATA URL, NOTES, WORKS?

Pemetaan ke tabel ports:
    GAME            -> game / slug
    PLATFORM        -> category_slug (asal game)
    DEVELOPER       -> author
    VERSION         -> version
    URL             -> download_url  (tautan port)
    DATA URL        -> iso_url       (berkas data, bila ada)
    NEEDS GAME FILES-> needs_own_data
    NOTES           -> install_notes + cuplikan deskripsi
    WORKS?          -> status_slug (YES=stable, NO=experimental, lain=beta)

Yang dibuang: kategori "Multi-Game Apps" (itu aplikasi, bukan game) dan
baris tanpa nama/tautan.

    python3 tools/seed-napl.py -o /tmp/seed-napl.sql
"""

import argparse
import csv
import io
import re
import sys
import urllib.request

CSV_URL = ("https://docs.google.com/spreadsheets/d/"
           "1CO7dH7mbj9sl67g4e94wczHESp0NAsOa7_chKKii9OA/export?format=csv")
UA = "Mozilla/5.0 (compatible; gameport-seed/1.0; +https://gameport.web.id)"
NAPL_PAGE = "https://fe2rr.github.io/public/game.html?game="

# PLATFORM (asal game) -> category_slug di database
CATEGORY = {
    "xbox 360": "xbox360-recomp",
    "xbox": "xbox-ps-psp-pc",
    "nintendo switch": "nintendo-recomp",
    "nintendo switch 2": "nintendo-recomp",
    "nintendo 64": "n64-recomp",
    "nintendo gamecube": "nintendo-recomp",
    "nintendo wii": "nintendo-recomp",
    "nintendo wii u": "nintendo-recomp",
    "playstation": "xbox-ps-psp-pc",
    "playstation 2": "xbox-ps-psp-pc",
    "playstation 3": "xbox-ps-psp-pc",
    "playstation 4": "xbox-ps-psp-pc",
    "playstation portable": "xbox-ps-psp-pc",
    "nintendo entertainment system": "handheld-16bit",
    "super nintendo": "handheld-16bit",
    "game boy": "handheld-16bit",
    "gameboy advance": "handheld-16bit",
    "nintendo ds": "handheld-16bit",
    "nintendo 3ds": "handheld-16bit",
    "sega mega drive / genesis": "handheld-16bit",
    "ios": "xbox-ps-psp-pc",
    "android (32 to 64bits)": "source-port",
}

# Awalan/sufiks yang tidak perlu ikut jadi slug
BUANG = re.compile(r"\b(remastered|remaster|definitive edition|complete edition|"
                   r"game of the year edition|goty|hd|classic)\b", re.I)


def ambil_csv(timeout=60):
    req = urllib.request.Request(CSV_URL, headers={"User-Agent": UA})
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return r.read().decode("utf-8", "replace")


def slugify(nama):
    s = nama.lower()
    s = re.sub(r"\(.*?\)", " ", s)
    s = BUANG.sub(" ", s)
    s = re.sub(r"[^a-z0-9]+", "-", s).strip("-")
    return re.sub(r"-{2,}", "-", s) or "tanpa-nama"


def status_dari(works):
    w = (works or "").strip().lower()
    if w == "yes":
        return "stable"
    if w == "no":
        return "experimental"
    return "beta"                     # "can't test" dan kosong = belum terverifikasi


def butuh_data(needs):
    n = (needs or "").strip().lower()
    return bool(n) and n not in ("no", "tidak")


def ambil_baris(teks):
    rows = []
    seen_slug = {}   # slug yang sudah dipakai dalam batch ini
    for r in csv.DictReader(io.StringIO(teks)):
        game = (r.get("GAME") or "").strip()
        url = (r.get("URL") or "").strip()
        platform = (r.get("PLATFORM") or "").strip()
        if not game or not url:
            continue
        if platform.lower() == "multi-game apps":       # aplikasi, bukan game
            continue
        if not url.startswith("http"):
            continue

        # Nama sama untuk platform berbeda (atau duplikat di sheet) harus
        # mendapat slug unik, kalau tidak barisnya saling menimpa.
        dasar = slugify(game)
        slug = dasar
        if slug in seen_slug:
            cadangan = f"{dasar}-{slugify(platform)}" if platform else dasar
            slug = cadangan
            n = 2
            while slug in seen_slug:
                slug = f"{cadangan}-{n}"
                n += 1
        seen_slug[slug] = True

        catatan = (r.get("NOTES") or "").strip()
        rows.append(dict(
            slug=slug,
            game=game,
            game_sort=BUANG.sub("", re.sub(r"\(.*?\)", "", game)).strip(),
            version=(r.get("VERSION") or "").strip(),
            genre=None,
            description=(catatan if catatan else
                         f"{game} — port native Android dari {platform or 'sumber lain'}."),
            author=(r.get("DEVELOPER") or "NAPL").strip()[:80],
            category=CATEGORY.get(platform.lower(), "source-port"),
            status=status_dari(r.get("WORKS?")),
            download_url=url,
            iso_url=(r.get("DATA URL") or "").strip() or None,
            source_url=NAPL_PAGE + (r.get("PORTS_ID") or slug),
            notes=catatan,
            needs_own_data=butuh_data(r.get("NEEDS GAME FILES?")),
            published=(r.get("WORKS?") or "").strip().lower() != "no",
        ))
    return rows


def q(s):
    if s is None or s == "":
        return "NULL" if s is None else "''"
    return "'" + str(s).replace("'", "''") + "'"


def to_sql(rows):
    out = [
        "-- Dihasilkan tools/seed-napl.py — jangan diedit tangan.",
        "-- Menambah entri baru; baris yang sudah ada hanya disegarkan bila",
        "-- source_url-nya menunjuk NAPL (entri manual tidak ditimpa).",
        "BEGIN;", "",
    ]
    for r in rows:
        out.append(
            "INSERT INTO ports (slug, game, game_sort, version, file_size,\n"
            "                   genre_slug, description, author, category_slug,\n"
            "                   status_slug, download_url, iso_url, source_url,\n"
            "                   install_notes, needs_own_data, published)\n"
            f"VALUES ({q(r['slug'])}, {q(r['game'])}, {q(r['game_sort'])}, "
            f"{q(r['version'])}, '',\n"
            f"        {q(r['genre'])}, {q(r['description'])}, {q(r['author'])}, "
            f"{q(r['category'])},\n"
            f"        {q(r['status'])}, {q(r['download_url'])}, {q(r['iso_url'])}, "
            f"{q(r['source_url'])},\n"
            f"        {q(r['notes'])}, {str(r['needs_own_data']).lower()}, "
            f"{str(r['published']).lower()})\n"
            "ON CONFLICT (slug) DO UPDATE SET\n"
            "    version      = EXCLUDED.version,\n"
            "    download_url = EXCLUDED.download_url,\n"
            "    iso_url      = EXCLUDED.iso_url,\n"
            "    needs_own_data = EXCLUDED.needs_own_data,\n"
            "    updated_at   = now()\n"
            "WHERE ports.source_url LIKE 'https://fe2rr.github.io/%';"
        )
        out.append(
            "INSERT INTO port_platforms (port_id, platform_slug)\n"
            f"SELECT id, 'android' FROM ports WHERE slug = {q(r['slug'])}\n"
            "ON CONFLICT DO NOTHING;"
        )
        out.append("")
    out += ["COMMIT;", ""]
    return "\n".join(out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-o", "--out", help="tulis SQL ke berkas ini")
    ap.add_argument("--limit", type=int, default=0, help="batasi jumlah baris (untuk uji)")
    args = ap.parse_args()

    rows = ambil_baris(ambil_csv())
    if args.limit:
        rows = rows[:args.limit]
    sql = to_sql(rows)
    if args.out:
        open(args.out, "w", encoding="utf-8").write(sql)
        print(f"  SQL  : {args.out}")
    else:
        sys.stdout.write(sql)
    print(f"  entri NAPL: {len(rows)}", file=sys.stderr)
    import collections
    for k, v in collections.Counter(r["category"] for r in rows).most_common():
        print(f"   {k:18s} {v}", file=sys.stderr)


if __name__ == "__main__":
    main()