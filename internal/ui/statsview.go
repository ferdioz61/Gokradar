package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"gokradar/internal/model"
)

// RenderStats renders a live traffic statistics modal summarizing global & visible aircraft.
func RenderStats(
	width, height int,
	visible []model.Flight,
	allMap map[string]model.Flight,
	metric bool,
	ascii bool,
	theme Theme,
) string {
	border := lipgloss.RoundedBorder()
	if ascii {
		border = lipgloss.ASCIIBorder()
	}

	var (
		totalAll   = len(allMap)
		totalVis   = len(visible)
		milVis     = 0
		emergVis   = 0
		groundVis  = 0
		airVis     = 0
		buckets    [5]int // <10k, 10k-20k, 20k-30k, 30k-40k, >40k
		maxAlt     *model.Flight
		maxSpd     *model.Flight
		countries  = make(map[string]int)
	)

	for i := range visible {
		f := visible[i]
		if f.IsMilitary {
			milVis++
		}
		if f.IsEmergency() {
			emergVis++
		}
		if f.OnGround {
			groundVis++
		} else {
			airVis++
			switch {
			case f.AltFeet < 10000:
				buckets[0]++
			case f.AltFeet < 20000:
				buckets[1]++
			case f.AltFeet < 30000:
				buckets[2]++
			case f.AltFeet < 40000:
				buckets[3]++
			default:
				buckets[4]++
			}
			if maxAlt == nil || f.AltFeet > maxAlt.AltFeet {
				cp := f
				maxAlt = &cp
			}
			if maxSpd == nil || f.SpeedKt > maxSpd.SpeedKt {
				cp := f
				maxSpd = &cp
			}
		}
		c := f.Country
		if c == "" {
			c = "Bilinmiyor"
		}
		countries[c]++
	}

	type kv struct {
		k string
		v int
	}
	topCountries := make([]kv, 0, len(countries))
	for k, v := range countries {
		topCountries = append(topCountries, kv{k: k, v: v})
	}
	sort.Slice(topCountries, func(i, j int) bool {
		if topCountries[i].v != topCountries[j].v {
			return topCountries[i].v > topCountries[j].v
		}
		return topCountries[i].k < topCountries[j].k
	})
	if len(topCountries) > 5 {
		topCountries = topCountries[:5]
	}

	maxBucket := 1
	for _, b := range buckets {
		if b > maxBucket {
			maxBucket = b
		}
	}

	barChar := "█"
	if ascii {
		barChar = "#"
	}
	renderBar := func(count int) string {
		n := (count * 22) / maxBucket
		if count > 0 && n == 0 {
			n = 1
		}
		return strings.Repeat(barChar, n)
	}

	lines := []string{
		theme.DetailTitle.Render("GÖKRADAR — CANLI TRAFİK İSTATİSTİKLERİ"),
		"",
		fmt.Sprintf("  Global Takip Edilen : %d uçak", totalAll),
		fmt.Sprintf("  Ekranda Görünür     : %d uçak  (Havada: %d  |  Yerde: %d)", totalVis, airVis, groundVis),
		fmt.Sprintf("  Askeri / Acil Durum : %d askeri  |  %d acil durum", milVis, emergVis),
		"",
		theme.ListHeader.Render("  İRTİFA DAĞILIMI (Ekrandaki Havadaki Uçaklar)"),
		fmt.Sprintf("   < 10.000 ft   : %-22s %d", renderBar(buckets[0]), buckets[0]),
		fmt.Sprintf("   10k - 20k ft  : %-22s %d", renderBar(buckets[1]), buckets[1]),
		fmt.Sprintf("   20k - 30k ft  : %-22s %d", renderBar(buckets[2]), buckets[2]),
		fmt.Sprintf("   30k - 40k ft  : %-22s %d", renderBar(buckets[3]), buckets[3]),
		fmt.Sprintf("   > 40.000 ft   : %-22s %d", renderBar(buckets[4]), buckets[4]),
		"",
		theme.ListHeader.Render("  REKORLAR (Görünür Alan)"),
	}

	if maxAlt != nil {
		lines = append(lines, fmt.Sprintf("   En Yüksek : %-9s  %s  (%s)",
			maxAlt.DisplayID(), model.FormatAlt(maxAlt.AltFeet, false, metric), maxAlt.Country))
	} else {
		lines = append(lines, "   En Yüksek : -")
	}
	if maxSpd != nil {
		lines = append(lines, fmt.Sprintf("   En Hızlı  : %-9s  %s  (%s)",
			maxSpd.DisplayID(), model.FormatSpeed(maxSpd.SpeedKt, metric), maxSpd.Country))
	} else {
		lines = append(lines, "   En Hızlı  : -")
	}

	lines = append(lines, "", theme.ListHeader.Render("  EN YOĞUN ÜLKELER (Görünür Alan)"))
	if len(topCountries) == 0 {
		lines = append(lines, "   Veri yok")
	} else {
		for idx, c := range topCountries {
			lines = append(lines, fmt.Sprintf("   %d. %-24s %d uçak", idx+1, truncate(c.k, 24), c.v))
		}
	}

	lines = append(lines, "", "  Kapatmak için i, Esc veya Enter tuşuna basın.")

	box := lipgloss.NewStyle().
		Border(border).
		BorderForeground(theme.FocusBorder).
		Padding(1, 3).
		Render(strings.Join(lines, "\n"))

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
