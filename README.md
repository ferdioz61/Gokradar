# GökRadar

Linux, Windows 10 ve Windows 11 (Windows Terminal, PowerShell, CMD) üzerinde ekstra kurulum veya API anahtarı gerektirmeden çalışan, terminal tabanlı (TUI) canlı dünya ve Türkiye uçuş radarı.

## Özellikler

- **API Anahtarsız Hibrit Veri:** Dünya geneli canlı uçuşlar için OpenSky Network, bölgesel zengin veri ve askeri uçuşlar için `adsb.lol` / `airplanes.live`, kalkış–varış rotaları için `adsbdb`.
- **İnteraktif Terminal Haritası:** Gömülü kıyı çizgileri, ülke sınırları ve büyük havalimanları (`IST`, `SAW`, `ESB`, `ADB`, `AYT` ve dünya hub'ları).
- **Braille & ASCII Modu:** Linux terminallerinde ve Windows 11/10 Windows Terminal'de yüksek çözünürlüklü Unicode Braille harita; klasik Windows 10 PowerShell/CMD için tek tuşla (`m` veya `--ascii`) saf ASCII modu.
- **3 Panelli Radar Kokpiti:** Harita + canlı uçuş listesi + seçili uçağın tüm detayları (rota, havayolu, uçak tipi, tescil, irtifa, dikey hız, hız, yön, squawk, koordinat).
- **Uçuş İzi ve Rota Çizgisi:** Seçili uçağın geçmiş izi ve kalkış → uçak → varış büyük çember rota çizgisi.
- **Filtreler ve Favoriler:** Arama (`/`), irtifa filtresi (`a`), askeri (`M`), acil durum (`E` — `7700/7600/7500`), kalıcı favori/takip listesi (`s`/`S`).
- **Türkçe Arayüz ve Birim Geçişi:** `ft/kt` ile `m/km/s` arasında tek tuşla (`u`) geçiş.

## İndirme (Hazır Tek Dosya — Intel & AMD Uyumlu)

En güncel sürümleri **[Releases (Sürümler)](https://github.com/ferdioz61/Gokradar/releases/latest)** sayfasından indirebilirsiniz:

- **Windows 10 / 11 (64-bit, Intel & AMD):** `radar-windows-64bit.exe`
- **Windows 10 (eski 32-bit sistemler):** `radar-windows-32bit.exe`
- **Linux (64-bit, Intel & AMD):** `radar-linux-64bit`
- **Windows kullanım rehberi:** `WINDOWS_KULLANIM.txt`

### Windows 10 / 11 (Windows Terminal, PowerShell veya CMD)
Doğrudan `.exe` dosyasına çift tıklayabilir veya terminalden çalıştırabilirsiniz:
```powershell
.\radar-windows-64bit.exe
.\radar-windows-64bit.exe --region tr
.\radar-windows-64bit.exe --ascii      # Klasik PowerShell/CMD fontunda kutucuk çıkarsa
```

### Linux
```bash
chmod +x radar-linux-64bit
./radar-linux-64bit
./radar-linux-64bit --region tr
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

Bu komut Linux (`~/.local/bin/radar` ve `dist/radar-linux-64bit`), Windows 64-bit (`dist/radar-windows-64bit.exe`) ve Windows 32-bit (`dist/radar-windows-32bit.exe`) ikililerini tek seferde derler.
