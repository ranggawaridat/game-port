package db

import "time"

// Port adalah satu baris v_port_detail.
type Port struct {
	ID            int64     `json:"id"`
	Slug          string    `json:"slug"`
	Game          string    `json:"game"`
	GameSort      string    `json:"game_sort"`
	Version       string    `json:"version"`
	FileSize      string    `json:"file_size"`
	Description   string    `json:"description"`
	Author        string    `json:"author"`
	ImageURL      *string   `json:"image_url"`
	DownloadURL   *string   `json:"download_url"`
	IsoURL        *string   `json:"iso_url"`
	SourceURL     *string   `json:"source_url"`
	InstallNotes  *string   `json:"install_notes"`
	ViewCount     int64     `json:"view_count"`
	DownloadCount int64     `json:"download_count"`
	Published     bool      `json:"published"`
	NeedsOwnData  bool      `json:"needs_own_data"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	CategorySlug  string `json:"category_slug"`
	CategoryName  string `json:"category_name"`
	CategoryOrder int    `json:"category_urutan"`

	StatusSlug   string `json:"status_slug"`
	StatusName   string `json:"status_name"`
	StatusColor  string `json:"status_warna"`
	StatusWeight int    `json:"status_bobot"`

	GenreSlug *string `json:"genre_slug"`
	GenreName *string `json:"genre_name"`

	Platforms []string `json:"platforms"`
}

// PortList adalah respons berhalaman.
type PortList struct {
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Items  []Port `json:"items"`
}

// PortInput adalah payload admin untuk membuat/mengubah port.
type PortInput struct {
	Game         string   `json:"game"`
	Version      string   `json:"version"`
	FileSize     string   `json:"file_size"`
	GenreSlug    *string  `json:"genre_slug"`
	Description  string   `json:"description"`
	Author       string   `json:"author"`
	CategorySlug string   `json:"category_slug"`
	StatusSlug   string   `json:"status_slug"`
	ImageURL     *string  `json:"image_url"`
	DownloadURL  *string  `json:"download_url"`
	IsoURL       *string  `json:"iso_url"`
	SourceURL    *string  `json:"source_url"`
	InstallNotes *string  `json:"install_notes"`
	Published    bool     `json:"published"`
	NeedsOwnData bool     `json:"needs_own_data"`
	Platforms    []string `json:"platforms"`
}

// Statistik ringkas.
type Statistik struct {
	TotalPort     int64 `json:"total_port"`
	Terbit        int64 `json:"terbit"`
	TotalAuthor   int64 `json:"total_author"`
	Stabil        int64 `json:"stabil"`
	Beta          int64 `json:"beta"`
	Eksperimental int64 `json:"eksperimental"`
	Alpha         int64 `json:"alpha"`
	TotalView     int64 `json:"total_view"`
	TotalDownld   int64 `json:"total_download"`
}

// Ref untuk daftar pilihan.
type Ref struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Jumlah int64  `json:"jumlah,omitempty"`
}

// Kategori.
type Kategori struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Urutan int    `json:"urutan"`
	Jumlah int64  `json:"jumlah"`
}

// Status kematangan.
type Status struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Warna  string `json:"warna"`
	Bobot  int    `json:"bobot"`
	Jumlah int64  `json:"jumlah"`
}

// Platform.
type Platform struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Jumlah int64  `json:"jumlah"`
}

// Genre.
type Genre struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Jumlah int64  `json:"jumlah"`
}

// PortDetail menambahkan daftar terkait.
type PortDetail struct {
	Port
	Related []Ref `json:"related"`
}

// ---------- Analitik ----------

// StatAdmin adalah ringkasan untuk dashboard admin.
type StatAdmin struct {
	TotalPort     int64 `json:"total_port"`
	Terbit        int64 `json:"terbit"`
	Draf          int64 `json:"draf"`
	TotalView     int64 `json:"total_view"`
	TotalDownload int64 `json:"total_download"`
	ViewHariIni   int64 `json:"view_hari_ini"`
	DownldHariIni int64 `json:"download_hari_ini"`
	View7Hari     int64 `json:"view_7hari"`
	Downld7Hari   int64 `json:"download_7hari"`
}

// TitikHarian untuk grafik.
type TitikHarian struct {
	Tanggal  string `json:"tanggal"`
	View     int64  `json:"view"`
	Download int64  `json:"download"`
}

// PortPopuler untuk tabel "paling banyak dilihat".
type PortPopuler struct {
	ID            int64   `json:"id"`
	Game          string  `json:"game"`
	Slug          string  `json:"slug"`
	ViewCount     int64   `json:"view_count"`
	DownloadCount int64   `json:"download_count"`
	ImageURL      *string `json:"image_url"`
}

// PencarianTeratas untuk melihat apa yang dicari pengunjung.
type PencarianTeratas struct {
	Query  string `json:"query"`
	Jumlah int64  `json:"jumlah"`
}