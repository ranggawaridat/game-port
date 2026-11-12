package db

// Port adalah satu baris view v_port_detail yang dikirim ke klien.
type Port struct {
	ID            int64    `json:"id"`
	Slug          string   `json:"slug"`
	Game          string   `json:"game"`
	GameSort      string   `json:"game_sort"`
	Description   string   `json:"description"`
	Author        string   `json:"author"`
	AuthorURL     *string  `json:"author_url"`
	RepoURL       string   `json:"repo_url"`
	NeedsOwnData  bool     `json:"needs_own_data"`
	CategorySlug  string   `json:"category_slug"`
	CategoryName  string   `json:"category_name"`
	CategoryOrder int      `json:"category_urutan"`
	StatusSlug    string   `json:"status_slug"`
	StatusName    string   `json:"status_name"`
	StatusColor   string   `json:"status_warna"`
	StatusWeight  int      `json:"status_bobot"`
	Platforms     []string `json:"platforms"`
	Engines       []string `json:"engines"`
	Features      []string `json:"features"`
}

// PortList adalah respons berhalaman.
type PortList struct {
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Items  []Port `json:"items"`
}

// Statistik adalah angka agregat untuk dashboard.
type Statistik struct {
	TotalPort     int `json:"total_port"`
	TotalAuthor   int `json:"total_author"`
	ButuhDataAsli int `json:"butuh_data_asli"`
	Stabil        int `json:"stabil"`
	Beta          int `json:"beta"`
	Eksperimental int `json:"eksperimental"`
	Alpha         int `json:"alpha"`
}

// Kategori adalah satu kelompok port.
type Kategori struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Urutan int    `json:"urutan"`
	Jumlah int    `json:"jumlah"`
}

// Status tingkat kematangan.
type Status struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Warna  string `json:"warna"`
	Bobot  int    `json:"bobot"`
	Jumlah int    `json:"jumlah"`
}

// Platform tujuan port.
type Platform struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Jumlah int    `json:"jumlah"`
}

// Engine/framework yang dipakai.
type Engine struct {
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Jumlah int   `json:"jumlah"`
}

// Ref singkat untuk daftar terkait.
type Ref struct {
	Slug string `json:"slug"`
	Game string `json:"game"`
}

// PortDetail menambahkan daftar terkait.
type PortDetail struct {
	Port
	Related []Ref `json:"related"`
}