package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"gokradar/internal/data"
	"gokradar/internal/ui"
)

func main() {
	ascii := flag.Bool("ascii", false, "Saf ASCII harita modu (eski Windows 10 PowerShell fontları için)")
	metric := flag.Bool("metric", false, "Metrik birimler (metre, km/s) ile başlat")
	region := flag.String("region", "", "Başlangıç bölgesi: dunya, tr, eu, us veya 1-5")
	flag.Parse()

	world, err := data.LoadWorldData()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Harita verisi yüklenemedi: %v\n", err)
		os.Exit(1)
	}

	app := ui.NewAppModel(world, ui.Options{
		ASCII:  *ascii,
		Metric: *metric,
		Region: *region,
	})

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "GökRadar hatası: %v\n", err)
		os.Exit(1)
	}
}
