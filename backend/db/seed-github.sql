-- Dihasilkan tools/seed-github.py — jangan diedit tangan.
-- Aman dijalankan berulang: slug yang sudah ada tidak ditimpa.
BEGIN;

INSERT INTO ports (slug, game, game_sort, version, file_size,
                   genre_slug, description, author, category_slug,
                   status_slug, download_url, source_url, install_notes,
                   needs_own_data, published)
VALUES ('diablo-devilutionx', 'Diablo (DevilutionX)', 'Diablo DevilutionX', '1.5.5', '18 MB',
        'rpg', 'Port ulang mesin Diablo I ke Android. Butuh berkas DIABDAT.MPQ dari salinan game asli yang kamu miliki.', 'diasurgical', 'decompilation',
        'stable', 'https://github.com/diasurgical/DevilutionX/releases/download/1.5.5/devilutionx-android.apk', 'https://github.com/diasurgical/DevilutionX', 'Pasang APK, lalu salin DIABDAT.MPQ ke folder yang diminta aplikasi saat pertama dibuka.',
        true, true)
ON CONFLICT (slug) DO NOTHING;
INSERT INTO port_platforms (port_id, platform_slug)
SELECT id, 'android' FROM ports WHERE slug = 'diablo-devilutionx'
ON CONFLICT DO NOTHING;

INSERT INTO ports (slug, game, game_sort, version, file_size,
                   genre_slug, description, author, category_slug,
                   status_slug, download_url, source_url, install_notes,
                   needs_own_data, published)
VALUES ('half-life-xash3d', 'Half-Life (Xash3D FWGS)', 'Half-Life Xash3D FWGS', 'continuous', '45.2 MB',
        'shooter', 'Mesin GoldSrc modern yang menjalankan Half-Life di Android. Data game tidak disertakan — wajib dari salinan asli.', 'FWGS', 'source-port',
        'beta', 'https://github.com/FWGS/xash3d-fwgs/releases/download/continuous/xash3d-fwgs-android.apk', 'https://github.com/FWGS/xash3d-fwgs', 'Pasang APK, lalu arahkan ke folder valve/ hasil salinan instalasi Half-Life milikmu.',
        true, true)
ON CONFLICT (slug) DO NOTHING;
INSERT INTO port_platforms (port_id, platform_slug)
SELECT id, 'android' FROM ports WHERE slug = 'half-life-xash3d'
ON CONFLICT DO NOTHING;

INSERT INTO ports (slug, game, game_sort, version, file_size,
                   genre_slug, description, author, category_slug,
                   status_slug, download_url, source_url, install_notes,
                   needs_own_data, published)
VALUES ('metal-gear-rising-zandatsu', 'Metal Gear Rising: Revengeance (Zandatsu)', 'Metal Gear Rising: Revengeance Zandatsu', 'v1.3', '73.9 MB',
        'action', 'Upaya menghadirkan Metal Gear Rising: Revengeance ke Android. Pantau catatan rilis untuk kebutuhan perangkat dan data.', 'dfdx047', 'xbox-ps-psp-pc',
        'stable', 'https://github.com/dfdx047/Zandatsu/releases/download/v1.3/Zandatsu-v1.3.apk', 'https://github.com/dfdx047/Zandatsu', 'Pasang APK. Cek catatan rilis di halaman sumber untuk kebutuhan GPU dan data tambahan.',
        false, true)
ON CONFLICT (slug) DO NOTHING;
INSERT INTO port_platforms (port_id, platform_slug)
SELECT id, 'android' FROM ports WHERE slug = 'metal-gear-rising-zandatsu'
ON CONFLICT DO NOTHING;

INSERT INTO ports (slug, game, game_sort, version, file_size,
                   genre_slug, description, author, category_slug,
                   status_slug, download_url, source_url, install_notes,
                   needs_own_data, published)
VALUES ('cube-evil-4-android', 'Cube Evil 4 (Android Port)', 'Cube Evil 4 Android Port', '0.99.14-rc', '1.2 MB',
        'horror', 'Port bergaya survival horror klasik ke Android. Ikuti catatan rilis untuk perubahan dan masalah yang diketahui.', 'Andiweli', 'xbox-ps-psp-pc',
        'stable', 'https://github.com/Andiweli/CubeEvil4-Android/releases/download/0.99.14-rc/CubeEvil4-TexturePacker-1.3.apk', 'https://github.com/Andiweli/CubeEvil4-Android', 'Pasang APK. Rincian tambahan ada di halaman rilis.',
        false, true)
ON CONFLICT (slug) DO NOTHING;
INSERT INTO port_platforms (port_id, platform_slug)
SELECT id, 'android' FROM ports WHERE slug = 'cube-evil-4-android'
ON CONFLICT DO NOTHING;

INSERT INTO ports (slug, game, game_sort, version, file_size,
                   genre_slug, description, author, category_slug,
                   status_slug, download_url, source_url, install_notes,
                   needs_own_data, published)
VALUES ('downhill-domination-revived', 'Downhill Domination (Downhill Revived)', 'Downhill Domination Downhill Revived', 'v0.8.2', '35.1 MB',
        'sports', 'Port balap sepeda gunung Downhill Domination ke Android, lengkap dengan mode online dan kontrol sentuh.', 'skuza-oss', 'xbox-ps-psp-pc',
        'stable', 'https://github.com/skuza-oss/downhill-revived/releases/download/v0.8.2/DownhillRevived-0.8.2-android-experimental.apk', 'https://github.com/skuza-oss/downhill-revived', 'Pasang APK. Mode online memakai server komunitas — lihat catatan rilis.',
        false, true)
ON CONFLICT (slug) DO NOTHING;
INSERT INTO port_platforms (port_id, platform_slug)
SELECT id, 'android' FROM ports WHERE slug = 'downhill-domination-revived'
ON CONFLICT DO NOTHING;

INSERT INTO ports (slug, game, game_sort, version, file_size,
                   genre_slug, description, author, category_slug,
                   status_slug, download_url, source_url, install_notes,
                   needs_own_data, published)
VALUES ('cataclysm-dark-days-ahead', 'Cataclysm: Dark Days Ahead', 'Cataclysm: Dark Days Ahead', '0.I-1', '159.6 MB',
        'rpg', 'Roguelike survival open source berlatar pasca-kiamat, dibangun untuk Android tanpa emulator.', 'CleverRaven', 'source-port',
        'stable', 'https://github.com/CleverRaven/Cataclysm-DDA/releases/download/0.I-1/cdda-android-x32-2026-09-19-2324.apk', 'https://github.com/CleverRaven/Cataclysm-DDA', 'Pasang APK. Simpanan lama bisa dipindahkan ke folder data aplikasi.',
        false, true)
ON CONFLICT (slug) DO NOTHING;
INSERT INTO port_platforms (port_id, platform_slug)
SELECT id, 'android' FROM ports WHERE slug = 'cataclysm-dark-days-ahead'
ON CONFLICT DO NOTHING;

COMMIT;
