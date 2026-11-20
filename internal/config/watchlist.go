package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Watchlist persists user-starred callsigns or hex codes.
type Watchlist struct {
	Items map[string]bool `json:"items"`
	path  string
}

// LoadWatchlist reads the watchlist from the OS config directory.
func LoadWatchlist() *Watchlist {
	w := &Watchlist{
		Items: make(map[string]bool),
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return w
	}
	w.path = filepath.Join(dir, "gokradar", "watchlist.json")
	b, err := os.ReadFile(w.path)
	if err != nil {
		return w
	}
	var list []string
	if err := json.Unmarshal(b, &list); err == nil {
		for _, item := range list {
			k := normalizeKey(item)
			if k != "" {
				w.Items[k] = true
			}
		}
	}
	return w
}

// Toggle adds or removes an identifier (callsign or hex) and saves to disk.
func (w *Watchlist) Toggle(id string) bool {
	k := normalizeKey(id)
	if k == "" {
		return false
	}
	if w.Items[k] {
		delete(w.Items, k)
		_ = w.Save()
		return false
	}
	w.Items[k] = true
	_ = w.Save()
	return true
}

// Has checks whether either callsign or hex is in the watchlist.
func (w *Watchlist) Has(callsign, hex string) bool {
	if cs := normalizeKey(callsign); cs != "" && w.Items[cs] {
		return true
	}
	if h := normalizeKey(hex); h != "" && w.Items[h] {
		return true
	}
	return false
}

// Save writes the watchlist to disk.
func (w *Watchlist) Save() error {
	if w.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil {
		return err
	}
	list := make([]string, 0, len(w.Items))
	for k := range w.Items {
		list = append(list, k)
	}
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(w.path, b, 0o644)
}

func normalizeKey(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}
