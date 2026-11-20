# GökRadar

Linux ve Windows 10 (PowerShell) üzerinde ekstra kurulum veya API anahtarı gerektirmeden çalışan, terminal tabanlı (TUI) canlı dünya ve Türkiye uçuş radarı.

## Özellikler

- **API Anahtarsız Hibrit Veri:** Dünya geneli canlı uçuşlar için OpenSky Network, bölgesel zengin veri ve askeri uçuşlar için `adsb.lol` / `airplanes.live`, kalkış–varış rotaları için `adsbdb`.
- **İnteraktif Terminal Haritası:** Gömülü kıyı çizgileri, ülke sınırları ve büyük havalimanları (`IST`, `SAW`, `ESB`, `ADB`, `AYT` ve dünya hub'ları).
- **Braille & ASCII Modu:** Modern terminallerde yüksek çözünürlüklü Unicode Braille harita; eski Windows 10 PowerShell için tek tuşla (`m` veya `--ascii`) saf ASCII modu.
- **3 Panelli Radar Kokpiti:** Harita + canlı uçuş listesi + seçili uçağın tüm detayları (rota, havayolu, uçak tipi, tescil, irtifa, dikey hız, hız, yön, squawk, koordinat).
- **Uçuş İzi ve Rota Çizgisi:** Seçili uçağın geçmiş izi ve kalkış → uçak → varış büyük çember rota çizgisi.
- **Filtreler ve Favoriler:** Arama (`/`), irtifa filtresi (`a`), askeri (`M`), acil durum (`E` — `7700/7600/7500`), kalıcı favori/takip listesi (`s`/`S`).
- **Türkçe Arayüz ve Birim Geçişi:** `ft/kt` ile `m/km/s` arasında tek tuşla (`u`) geçiş.

## İndirme (Hazır Tek Dosya)

En güncel Linux ve Windows sürümlerini **[Releases (Sürümler)](https://github.com/ferdioz61/Gokradar/releases/latest)** sayfasından indirebilirsiniz:

- **Windows 10/11 (64-bit):** `radar-windows-amd64.exe` (ve `WINDOWS_KULLANIM.txt`)
- **Linux (64-bit):** `radar-linux-amd64`

### Windows 10 (PowerShell)
```powershell
.\radar-windows-amd64.exe
.\radar-windows-amd64.exe --region tr
.\radar-windows-amd64.exe --ascii      # Klasik PowerShell fontunda kutucuk çıkarsa
```

### Linux
```bash
chmod +x radar-linux-amd64
./radar-linux-amd64
./radar-linux-amd64 --region tr
```

## Kısayollar

| Tuş | İşlev |
|---|---|
| `1` – `5` | Hızlı bölge: Dünya, Türkiye, Avrupa, Orta Doğu, ABD |
| `Tab` | Liste ↔ Harita odak değişimi |
| `↑` `↓` `←` `→` / `hjkl` | Listede gezin veya haritayı kaydır |
| `+` / `-` | Haritayı yakınlaştır / uzaklaştır |
| `Enter` / `c` | Seçili uçağı haritada ortala |
| `f` | Tam ekran harita |
| `/` | Ara (çağrı kodu, ülke, uçak tipi, havalimanı, havayolu) |
| `a` | İrtifa filtresi (`>30k`, `10k-30k`, `<10k`) |
| `M` / `E` | Sadece askeri / Sadece acil durum (`7700/7600/7500`) |
| `s` / `S` | Favorilere ekle / Sadece favorileri göster |
| `m` | Braille ↔ ASCII harita modu |
| `u` | Havacılık (`ft/kt`) ↔ Metrik (`m/km/s`) birimleri |
| `r` | Verileri hemen yenile |
| `?` | Yardım penceresi |
| `q` | Çıkış |

## Kaynak Koddan Derleme

Go 1.22+ kuruluysa:

```bash
./scripts/build.sh
```

Bu komut Linux ikilisini `~/.local/bin/radar` konumuna, Windows 64-bit ikilisini `dist/radar.exe` konumuna derler.
