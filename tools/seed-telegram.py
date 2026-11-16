#!/usr/bin/env python3
"""seed-telegram.py — isi katalog GamePort dari channel Telegram publik.

Sumber: https://t.me/PortalGamerID (pratinjau web, tanpa login).

Format post yang dikenali:

    Nama: DOOM 2016 (unofficial Port)
    Versi: v0.5.2
    Size: 19GB
    Genre: Action
    Platform: Switch
    ...
    Link Download
    Apk: https://sfl.gl/xxxx
    Data: https://sfl.gl/yyyy

Yang diambil: nama, versi, ukuran, genre, platform, tautan APK (download_url)
dan tautan data (iso_url). Gambar changelog TIDAK dipakai sebagai sumber data.

Hasilnya SQL idempotent, aman dijalankan berulang:
    python3 tools/seed-telegram.py -o /tmp/seed-telegram.sql

Baris yang sudah ada hanya disegarkan bila source_url-nya masih menunjuk
t.me — entri yang kamu sunting manual tidak akan ditimpa.
"""

import argparse
import html
import re
import sys
import urllib.request

CHANNEL = "PortalGamerID"
BASE = f"https://t.me/s/{CHANNEL}"
UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " \
     "(KHTML, like Gecko) Chrome/131.0 Safari/537.36"

# Genre di database → kata yang mungkin muncul di post
GENRE_MAP = {
    "action": "action", "adventure": "adventure", "rpg": "rpg",
    "strategy": "strategy", "rts": "strategy", "shooter": "shooter",
    "fps": "shooter", "racing": "racing", "sport": "sports",
    "sports": "sports", "simulation": "simulation", "sim": "simulation",
    "puzzle": "puzzle", "platformer": "platformer", "fighting": "fighting",
    "horror": "horror", "sandbox": "sandbox", "roguelike": "roguelike",
    "rhythm": "rhythm", "card": "card", "board": "board", "mmo": "mmo",
}

# Platform asal di post → category_slug di database
CATEGORY_MAP = {
    "pc": "source-port", "windows": "source-port",
    "switch": "nintendo-recomp", "n64": "n64-recomp",
    "ps2": "xbox-ps-psp-pc", "psp": "xbox-ps-psp-pc",
    "ps1": "xbox-ps-psp-pc", "ps3": "xbox-ps-psp-pc",
    "ps4": "xbox-ps-psp-pc", "xbox": "xbox360-recomp",
    "xbox360": "xbox360-recomp", "xbox 360": "xbox360-recomp",
    "ps vita": "xbox-ps-psp-pc", "vita": "xbox-ps-psp-pc",
    "gba": "handheld-16bit", "snes": "handheld-16bit",
    "genesis": "handheld-16bit", "nds": "handheld-16bit",
}


def ambil(url, timeout=25):
    req = urllib.request.Request(url, headers={"User-Agent": UA})
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return r.read().decode("utf-8", "replace")


def slugify(nama):
    s = nama.lower()
    s = re.sub(r"\(.*?\)", " ", s)                    # buang "(unofficial port)"
    s = re.sub(r"\b(unofficial|port|android|mod|apk)\b", " ", s)
    s = re.sub(r"[^a-z0-9]+", "-", s).strip("-")
    s = re.sub(r"-{2,}", "-", s)
    return s or "tanpa-nama"


def rapikan(teks):
    teks = re.sub(r"<br\s*/?>", "\n", teks)
    teks = re.sub(r"<[^>]+>", "", teks)
    return html.unescape(teks).replace("\u00a0", " ")


LABEL = re.compile(
    r"\s*\b(Nama|Versi|Version|Size|Ukuran|Genre|Platform|Link Download|Apk|APK|Data|Note|Catatan)\s*:\s*",
    re.I)


def normalisasi(teks):
    """Sebagian post ditulis satu baris tanpa pemisah, sehingga field
    bercampur ("Versi: v0.1 demo Size: 3GB Genre: Action"). Paksa setiap
    label mulai baris baru supaya pembacaan per-field bisa diandalkan."""
    return LABEL.sub(lambda m: "\n" + m.group(1) + ": ", teks)


def ambil_field(teks, label):
    m = re.search(rf"^{label}\s*:\s*(.*)$", teks, re.I | re.M)
    if not m:
        return ""
    nilai = m.group(1).strip()
    # Kadang label berikutnya menempel tanpa spasi ("Versi:Size: 3GB"),
    # sehingga nilainya masih membawa label lain. Buang berulang.
    for _ in range(4):
        nilai = LABEL_NILAI.sub("", nilai).strip()
    return nilai


LABEL_NILAI = re.compile(
    r"^(Nama|Versi|Version|Size|Ukuran|Genre|Platform|Link Download|Apk|APK|Data|Note|Catatan)\s*:\s*",
    re.I)


def parse_post(blok, msg_id):
    teks_div = re.search(r'<div class="tgme_widget_message_text[^"]*"[^>]*>(.*?)</div>',
                         blok, re.S)
    if not teks_div:
        return None
    teks = normalisasi(rapikan(teks_div.group(1)))
    if not re.search(r"^nama\s*:", teks, re.I | re.M):
        return None                                   # bukan post game

    nama = ambil_field(teks, "Nama")
    if not nama:
        return None

    # Sebagian post menulis tautan sebagai anchor, sebagian lagi sebagai teks
    # polos (Telegram tidak selalu merendernya jadi <a>). Ambil keduanya.
    tautan = re.findall(r'href="(https?://[^"]+)"', teks_div.group(1))
    tautan += re.findall(r'https?://[^\s<>"]+', teks)
    bersih_url, terlihat = [], set()
    for u in tautan:
        u = u.rstrip('.,;:)"\'\u2026')
        if u and u not in terlihat:
            terlihat.add(u)
            bersih_url.append(u)
    tautan = bersih_url
    apk = next((u for u in tautan if "saweria" not in u
                and re.search(r"\b(apk|port)\b", teks.split(u)[0][-40:], re.I)), "")
    if not apk:
        apk = next((u for u in tautan if "saweria" not in u), "")
    data = next((u for u in tautan if u != apk and "saweria" not in u), "")

    waktu = re.search(r'datetime="([^"]+)"', blok)
    catatan = ambil_field(teks, "Note") or ambil_field(teks, "Catatan")
    platform = re.split(r"\s{2,}", ambil_field(teks, "Platform"))[0].strip()
    platform = re.sub(r"\s+Sawer.*$", "", platform, flags=re.I).strip()
    versi = ambil_field(teks, "Versi")

    status = "stable"
    if re.search(r"\bbeta\b", versi + " " + teks[:200], re.I):
        status = "beta"
    elif re.search(r"\balpha\b", versi + " " + teks[:200], re.I):
        status = "alpha"
    elif re.search(r"(-rc|rc\d)", versi, re.I):
        status = "beta"

    genre_raw = ambil_field(teks, "Genre").lower()
    genre = next((v for k, v in GENRE_MAP.items() if k in genre_raw), None)
    kategori = CATEGORY_MAP.get(platform.strip().lower(), "source-port")

    if not apk:
        return None                                   # post tanpa tautan unduhan

    # Field yang tampak seperti field lain berarti parsing meleset
    # (mis. versi berisi "1,7GB" karena post-nya ditulis satu baris).
    def mirip_ukuran(v):
        return bool(re.match(r"^[\d.,]+\s*(gb|mb|kb|tb)\b", v.strip(), re.I))

    def mirip_genre(v):
        return v.strip().lower() in GENRE_MAP

    if mirip_ukuran(versi):
        versi = ""
    if mirip_genre(file_size_mentah := ambil_field(teks, "Size")):
        file_size_mentah = ""

    bersih = bool(re.match(r"^v?\d", versi.strip() or "x")) and bool(ambil_field(teks, "Size"))

    return dict(
        slug=slugify(nama),
        game=nama,
        game_sort=re.sub(r"\s*\(.*?\)", "", nama).strip(),
        version=versi,
        file_size=file_size_mentah,
        genre=genre,
        description=(catatan if catatan else
                     f"{re.sub(r's*\(.*?\)', '', nama).strip()} — port dari {platform or 'PC'} "
                     f"yang berjalan native di Android."),
        author=CHANNEL,
        category=kategori,
        status=status,
        download_url=apk or None,
        iso_url=data or None,
        source_url=f"https://t.me/{CHANNEL}/{msg_id}",
        notes=catatan or "",
        needs_own_data=bool(data),
        tanggal=waktu.group(1) if waktu else "",
        bersih=bersih,
    )


def kumpulkan(pages):
    """Ambil beberapa halaman pratinjau; t.me/s/ hanya menampilkan ~20 post."""
    hasil, before = [], ""
    for _ in range(pages):
        url = BASE + (f"?before={before}" if before else "")
        try:
            page = ambil(url)
        except Exception as e:  # noqa: BLE001
            print(f"  gagal ambil halaman: {e}", file=sys.stderr)
            break
        id_urut = re.findall(r'data-post="[^"]+/(\d+)"', page)
        if not id_urut:
            break
        blok_semua = re.split(r'(?=<div class="tgme_widget_message )', page)
        for blok in blok_semua:
            m = re.search(r'data-post="[^"]+/(\d+)"', blok)
            if not m:
                continue
            p = parse_post(blok, m.group(1))
            if p:
                hasil.append(p)
        before = id_urut[-1]                      # halaman berikutnya = lebih lama
        if before in id_urut[:-1]:
            break
    # buang duplikat slug, pertahankan yang terbaru (post teratas)
    unik = {}
    for p in hasil:
        unik.setdefault(p["slug"], p)
    return list(unik.values())


def q(s):
    if s is None:
        return "NULL"
    return "'" + str(s).replace("'", "''") + "'"


def to_sql(rows):
    out = [
        "-- Dihasilkan tools/seed-telegram.py — jangan diedit tangan.",
        "-- Hanya menimpa baris yang source_url-nya masih t.me.",
        "BEGIN;", "",
    ]
    for r in rows:
        out.append(
            "INSERT INTO ports (slug, game, game_sort, version, file_size,\n"
            "                   genre_slug, description, author, category_slug,\n"
            "                   status_slug, download_url, iso_url, source_url,\n"
            "                   install_notes, needs_own_data, published)\n"
            f"VALUES ({q(r['slug'])}, {q(r['game'])}, {q(r['game_sort'])}, "
            f"{q(r['version'])}, {q(r['file_size'])},\n"
            f"        {q(r['genre'])}, {q(r['description'])}, {q(r['author'])}, "
            f"{q(r['category'])},\n"
            f"        {q(r['status'])}, {q(r['download_url'])}, {q(r['iso_url'])}, "
            f"{q(r['source_url'])},\n"
            f"        {q(r['notes'])}, {str(r['needs_own_data']).lower()}, "
            f"{str(r['bersih']).lower()})\n"
            "ON CONFLICT (slug) DO UPDATE SET\n"
            "    version      = EXCLUDED.version,\n"
            "    file_size    = EXCLUDED.file_size,\n"
            "    download_url = EXCLUDED.download_url,\n"
            "    iso_url      = EXCLUDED.iso_url,\n"
            "    status_slug  = EXCLUDED.status_slug,\n"
            "    updated_at   = now()\n"
            "WHERE ports.source_url LIKE 'https://t.me/%';"
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
    ap.add_argument("--pages", type=int, default=3, help="halaman pratinjau (default 3 ≈ 60 post)")
    ap.add_argument("--json", action="store_true", help="cetak ringkasan JSON")
    args = ap.parse_args()

    rows = kumpulkan(args.pages)
    sql = to_sql(rows)
    if args.out:
        open(args.out, "w", encoding="utf-8").write(sql)
        print(f"  SQL  : {args.out}")
    else:
        sys.stdout.write(sql)
    print(f"  post game terbaca: {len(rows)}", file=sys.stderr)
    for r in rows:
        tanda = "" if r["bersih"] else "  ← DRAF (periksa di admin)"
        print(f"   - {r['slug']:42s} v{r['version'] or '?':10s} "
              f"{r['file_size'] or '?':8s} {r['genre'] or '-':10s} "
              f"{r['category']}{tanda}", file=sys.stderr)


if __name__ == "__main__":
    main()