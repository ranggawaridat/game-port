package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// =============================================================================
// Pengaturan yang bisa diubah dari panel admin.
//
// Saat ini hanya script iklan safelink. Disimpan di tabel settings, jadi
// perubahan langsung berlaku tanpa deploy ulang.
// =============================================================================

const maksIklanBytes = 200 << 10 // 200 KB

// handleAdminGetSettings mengembalikan pengaturan beserta status safelink.
func (s *Server) handleAdminGetSettings(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	iklan, err := s.store.AmbilSetting(ctx, settingIklan)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"safelink_ad_html": iklan,
		"safelink_enabled": s.cfg.SafeLinkEnabled,
		"safelink_wait":    s.cfg.SafeLinkWait,
		"safelink_ttl":     s.cfg.SafeLinkTTL,
	})
}

// handleAdminSaveSettings menyimpan script iklan safelink.
func (s *Server) handleAdminSaveSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SafelinkAdHTML *string `json:"safelink_ad_html"`
	}
	body := http.MaxBytesReader(w, r.Body, maksIklanBytes+(16<<10))
	if err := json.NewDecoder(body).Decode(&in); err != nil {
		s.writeErr(w, http.StatusBadRequest, "permintaan tidak valid atau terlalu besar")
		return
	}
	if in.SafelinkAdHTML == nil {
		s.writeErr(w, http.StatusBadRequest, "tidak ada yang disimpan")
		return
	}
	if len(*in.SafelinkAdHTML) > maksIklanBytes {
		s.writeErr(w, http.StatusBadRequest, "script iklan melebihi 200 KB")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	if err := s.store.SimpanSetting(ctx, settingIklan, *in.SafelinkAdHTML); err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
