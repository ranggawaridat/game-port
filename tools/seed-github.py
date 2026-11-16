#!/usr/bin/env python3
"""seed-github.py — isi katalog GamePort dari rilis GitHub.

Kenapa lewat GitHub API, bukan scraping HTML: API resmi, stabil, dan
menghormati rate limit (60 permintaan/jam tanpa token).

Yang diambil otomatis dari GitHub:
    version      tag rilis terbaru
    file_size    ukuran APK (dibulatkan)
    download_url tautan unduhan APK langsung
    source_url   halaman repo
    author       pemilik repo

Yang dikurasi manual di CANDIDATES (biar tidak ngawur):
    nama game, genre, kategori, deskripsi singkat, catatan pemasangan

Repositori tanpa aset .apk di rilis terbaru DILEWATI — bukan ditebak.

Hasilnya SQL idempotent:
    python3 tools/seed-github.py -o /tmp/seed-github.sql

Lalu terapkan di host:
    docker exec -i gameport-db psql -U gameport -d gamedb \
        -v ON_ERROR_STOP=1 < /tmp/seed-github.sql

ON CONFLICT (slug) DO NOTHING — entri yang sudah ada (termasuk yang kamu
isi manual lewat admin) tidak akan pernah ditimpa.
"""

import argparse
import json
import sys
import urllib.error
import urllib.request

API = "https://api.github.com"
UA = "gameport-seed/1.0 (+https://gameport.web.id)"

# fmt: off
CANDIDATES = [
    dict(repo="diasurgical/devilutionX", slug="diablo-devilutionx",
         game="Diablo (DevilutionX)", genre="rpg", category="decompilation",
         needs_own_data=True,
         desc="Port ulang mesin Diablo I ke Android. Butuh berkas DIABDAT.MPQ "
              "dari salinan game asli yang kamu miliki.",
         notes="Pasang APK, lalu salin DIABDAT.MPQ ke folder yang diminta "
               "aplikasi saat pertama dibuka."),

    dict(repo="FWGS/xash3d-fwgs", slug="half-life-xash3d-fwgs",
         game="Half-Life (Xash3D FWGS)", genre="shooter", category="source-port",
         needs_own_data=True,
         desc="Mesin GoldSrc modern yang menjalankan Half-Life di Android. "
              "Data game tidak disertakan — wajib dari salinan asli.",
         notes="Pasang APK, lalu arahkan ke folder valve/ hasil salinan "
               "instalasi Half-Life milikmu."),

    dict(repo="deathkiller/jazz2-native", slug="jazz-jackrabbit-2-native",
         game="Jazz Jackrabbit 2 (Jazz² Native)", genre="platformer",
         category="source-port", needs_own_data=True,
         desc="Reka ulang mesin Jazz Jackrabbit 2 dalam C++ dengan dukungan "
              "Android. Berkas data asli tetap diperlukan.",
         notes="Pasang APK, lalu tunjuk folder berisi berkas data Jazz "
               "Jackrabbit 2 milikmu."),

    dict(repo="dfdx047/Zandatsu", slug="metal-gear-rising-revengeance-zandatsu",
         game="Metal Gear Rising: Revengeance (Zandatsu)", genre="action",
         category="xbox-ps-psp-pc", needs_own_data=False,
         desc="Upaya menghadirkan Metal Gear Rising: Revengeance ke Android. "
              "Pantau catatan rilis untuk kebutuhan perangkat dan data.",
         notes="Pasang APK. Cek catatan rilis di halaman sumber untuk "
               "kebutuhan GPU dan data tambahan."),

    dict(repo="Andiweli/CubeEvil4-Android", slug="cube-evil-4-android-port",
         game="Cube Evil 4 (Android Port)", genre="horror",
         category="xbox-ps-psp-pc", needs_own_data=False,
         desc="Port bergaya survival horror klasik ke Android. Ikuti catatan "
              "rilis untuk perubahan dan masalah yang diketahui.",
         notes="Pasang APK. Rincian tambahan ada di halaman rilis."),

    dict(repo="skuza-oss/Downhill-Revived", slug="downhill-domination-downhill-revived",
         game="Downhill Domination (Downhill Revived)", genre="sports",
         category="xbox-ps-psp-pc", needs_own_data=False,
         desc="Port balap sepeda gunung Downhill Domination ke Android, "
              "lengkap dengan mode online dan kontrol sentuh.",
         notes="Pasang APK. Mode online memakai server komunitas — lihat "
               "catatan rilis."),

    dict(repo="CleverRaven/Cataclysm-DDA", slug="cataclysm-dark-days-ahead",
         game="Cataclysm: Dark Days Ahead", genre="rpg", category="source-port",
         needs_own_data=False,
         desc="Roguelike survival open source berlatar pasca-kiamat, dibangun "
              "untuk Android tanpa emulator.",
         notes="Pasang APK. Simpanan lama bisa dipindahkan ke folder data "
               "aplikasi."),

    dict(repo="yquake2/yquake2", slug="quake-2-yquake2",
         game="Quake II (Yamagi Quake II)", genre="shooter",
         category="source-port", needs_own_data=True,
         desc="Port mesin Quake II yang rapi dan terawat, berjalan native di "
              "Android. Butuh berkas pak dari game asli.",
         notes="Pasang APK, lalu salin folder baseq2/ dari instalasi Quake II "
               "milikmu."),

    dict(repo="XProger/OpenLara", slug="tomb-raider-openlara",
         game="Tomb Raider (OpenLara)", genre="adventure",
         category="source-port", needs_own_data=True,
         desc="Reka ulang mesin Tomb Raider klasik dengan renderer modern. "
              "Data level dari game asli tetap dibutuhkan.",
         notes="Pasang APK, lalu sediakan berkas level dari Tomb Raider "
               "milikmu."),

    dict(repo="dxx-rebirth/dxx-rebirth", slug="descent-dxx-rebirth",
         game="Descent (DXX-Rebirth)", genre="shooter", category="source-port",
         needs_own_data=True,
         desc="Port ulang Descent dan Descent II dengan dukungan OpenGL, "
              "berjalan di Android.",
         notes="Pasang APK, lalu sediakan berkas .hog dari versi asli."),

    dict(repo="opentyrian/opentyrian", slug="tyrian-opentyrian",
         game="Tyrian (OpenTyrian)", genre="shooter", category="source-port",
         needs_own_data=False,
         desc="Port open source dari Tyrian, shmup klasik era DOS yang "
              "dibagikan gratis.",
         notes="Pasang APK. Data game sudah termasuk karena Tyrian freeware."),

    dict(repo="OpenDune/OpenDune", slug="dune-2-opendune",
         game="Dune II (OpenDune)", genre="strategy", category="source-port",
         needs_own_data=True,
         desc="Reka ulang Dune II, nenek moyang RTS modern, untuk dijalankan "
              "native di Android.",
         notes="Pasang APK, lalu sediakan berkas data dari Dune II asli."),
]
# fmt: on


def api_get(path, timeout=25):
    req = urllib.request.Request(
        API + path,
        headers={"User-Agent": UA, "Accept": "application/vnd.github+json"},
    )
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return json.loads(r.read().decode())


def human_size(n):
    for unit in ("B", "KB", "MB", "GB"):
        if n < 1024 or unit == "GB":
            return f"{n:.0f} {unit}" if unit == "B" else f"{n:.1f}".rstrip("0").rstrip(".") + " " + unit
        n /= 1024.0


def pick_apk(assets):
    """Pilih satu APK: utamakan arm64, lalu universal, lalu yang terkecil."""
    apks = [a for a in assets if a["name"].lower().endswith(".apk")]
    if not apks:
        return None
    for key in ("arm64", "universal", "all"):
        hit = [a for a in apks if key in a["name"].lower()]
        if hit:
            return min(hit, key=lambda a: a["size"])
    return min(apks, key=lambda a: a["size"])


def q(s):
    """Literal SQL: kutip tunggal digandakan, None jadi NULL."""
    if s is None:
        return "NULL"
    return "'" + str(s).replace("'", "''") + "'"


def build():
    rows, skipped, errors = [], [], []

    for c in CANDIDATES:
        try:
            repo = api_get(f"/repos/{c['repo']}")
        except urllib.error.HTTPError as e:
            errors.append(f"{c['repo']}: HTTP {e.code} saat baca repo")
            continue
        except Exception as e:  # noqa: BLE001
            errors.append(f"{c['repo']}: {e}")
            continue

        # Ambil beberapa rilis terakhir, lalu pakai yang PALING BARU di antara
        # yang punya berkas .apk. Rilis terbaru sering berupa build tanpa APK
        # (mis. Cataclysm CDDA experimental), sehingga memilih "terbaru" saja
        # membuat entri tidak pernah tersegarkan. Cara ini sekaligus menangani
        # proyek yang menandai semua rilisnya sebagai pre-release.
        try:
            rilis = api_get(f"/repos/{c['repo']}/releases?per_page=15")
        except Exception as e:  # noqa: BLE001
            errors.append(f"{c['repo']}: gagal baca rilis ({e})")
            continue

        if not rilis:
            skipped.append(f"{c['repo']}: belum punya rilis")
            continue

        rel, apk = {}, None
        for kandidat in rilis:
            if kandidat.get("draft"):
                continue
            a = pick_apk(kandidat.get("assets") or [])
            if a:
                rel, apk = kandidat, a
                break

        if not apk:
            skipped.append(f"{c['repo']}: 15 rilis terakhir tanpa berkas .apk")
            continue

        rows.append(
            dict(
                slug=c["slug"],
                game=c["game"],
                game_sort=str(c["game"]).replace(" (", " ").split(")")[0].strip(),
                version=(rel.get("tag_name") or "").strip(),
                file_size=human_size(apk["size"]),
                genre=c["genre"],
                desc=c["desc"],
                author=repo["owner"]["login"],
                category=c["category"],
                status="stable" if not rel.get("prerelease") else "beta",
                download_url=apk["browser_download_url"],
                source_url=repo["html_url"],
                notes=c["notes"],
                needs_own_data=c["needs_own_data"],
            )
        )

    return rows, skipped, errors


def to_sql(rows, update=False):
    out = [
        "-- Dihasilkan tools/seed-github.py — jangan diedit tangan.",
        "-- Aman dijalankan berulang: slug yang sudah ada tidak ditimpa.",
    ]
    if update:
        out += [
            "-- Mode --update: versi, ukuran, tautan, dan status ikut disegarkan",
            "-- HANYA untuk baris yang tautannya masih menunjuk GitHub. Entri yang",
            "-- sudah kamu ganti ke sfl.gl (atau apa pun selain GitHub) dibiarkan.",
        ]
    out += ["BEGIN;", ""]
    for r in rows:
        if update:
            konflik = (
                "ON CONFLICT (slug) DO UPDATE SET\n"
                "    version      = EXCLUDED.version,\n"
                "    file_size    = EXCLUDED.file_size,\n"
                "    download_url = EXCLUDED.download_url,\n"
                "    status_slug  = EXCLUDED.status_slug,\n"
                "    updated_at   = now()\n"
                "WHERE ports.download_url IS NULL\n"
                "   OR ports.download_url LIKE 'https://github.com/%';"
            )
        else:
            konflik = "ON CONFLICT (slug) DO NOTHING;"
        out.append(
            "INSERT INTO ports (slug, game, game_sort, version, file_size,\n"
            "                   genre_slug, description, author, category_slug,\n"
            "                   status_slug, download_url, source_url, install_notes,\n"
            "                   needs_own_data, published)\n"
            f"VALUES ({q(r['slug'])}, {q(r['game'])}, {q(r['game_sort'])}, "
            f"{q(r['version'])}, {q(r['file_size'])},\n"
            f"        {q(r['genre'])}, {q(r['desc'])}, {q(r['author'])}, "
            f"{q(r['category'])},\n"
            f"        {q(r['status'])}, {q(r['download_url'])}, "
            f"{q(r['source_url'])}, {q(r['notes'])},\n"
            f"        {str(r['needs_own_data']).lower()}, true)\n"
            + konflik
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
    ap.add_argument("--json", action="store_true", help="cetak ringkasan JSON")
    ap.add_argument("--update", action="store_true",
                    help="segarkan versi/ukuran/tautan untuk entri yang masih "
                         "menunjuk GitHub (entri yang sudah kamu ganti dibiarkan)")
    args = ap.parse_args()

    rows, skipped, errors = build()

    if args.json:
        print(json.dumps({"masuk": rows, "dilewati": skipped, "error": errors},
                         indent=2, ensure_ascii=False))

    sql = to_sql(rows, update=args.update)
    if args.out:
        with open(args.out, "w", encoding="utf-8") as f:
            f.write(sql)
        print(f"  SQL  : {args.out}")
    else:
        sys.stdout.write(sql)

    print(f"  masuk    : {len(rows)}", file=sys.stderr)
    for s in skipped:
        print(f"  dilewati : {s}", file=sys.stderr)
    for e in errors:
        print(f"  error    : {e}", file=sys.stderr)


if __name__ == "__main__":
    main()
