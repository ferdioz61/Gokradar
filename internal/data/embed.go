package data

import (
	_ "embed"
	"encoding/json"

	"gokradar/internal/model"
)

//go:embed world.json
var worldJSON []byte

//go:embed borders.json
var bordersJSON []byte

//go:embed airports.json
var airportsJSON []byte

// WorldData holds embedded geographic polylines and airport markers.
type WorldData struct {
	Coastlines [][][2]float64 // [polyline][point][lon, lat]
	Borders    [][][2]float64 // [polyline][point][lon, lat]
	Airports   []model.Airport
}

// LoadWorldData decodes embedded coastline, border, and airport datasets.
func LoadWorldData() (*WorldData, error) {
	wd := &WorldData{}
	if err := json.Unmarshal(worldJSON, &wd.Coastlines); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bordersJSON, &wd.Borders); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(airportsJSON, &wd.Airports); err != nil {
		return nil, err
	}
	return wd, nil
}
