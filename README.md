# GökRadar

Linux, Windows 10 ve Windows 11 (Windows Terminal, PowerShell, CMD) üzerinde ekstra kurulum veya API anahtarı gerektirmeden çalışan, terminal tabanlı (TUI) canlı dünya ve Türkiye uçuş radarı.

## Özellikler

- **API Anahtarsız Hibrit Veri:** Dünya geneli canlı uçuşlar için OpenSky Network, bölgesel zengin veri ve askeri uçuşlar için `adsb.lol` / `airplanes.live`, kalkış–varış rotaları için `adsbdb`.
- **Tam Fare Kontrolü:** Haritada sol tık + sürükle ile kaydırma, fare tekerleği ile imleç odaklı yakınlaştırma/uzaklaştırma, tıklayarak uçak/liste seçimi, çift veya sağ tık ile canlı takip kilidi ve ortalama.
- **Yüksek Okunabilirlikli Radar Haritası:**
  - Braille arka planında net seçilen dolu 8 yönlü uçak simgeleri (`▲ ◥ ▶ ◢ ▼ ◣ ◀ ◤`), hız/yön vektör çizgileri (`v`) ve çakışmayı önleyen akıllı ATC etiketleri (`b`).
  - **Akıcı Hareket (Dead-Reckoning):** 15 saniyelik API güncellemeleri arasında uçakların hız ve yönlerine göre haritada canlı süzülmesi (`p` ile duraklatılabilir).
  - **İki Aşamalı Rota ve İrtifa Renkli İzler:** Tamamlanan (`Kalkış → Uçak`, düz parlak çizgi) ve kalan (`Uçak → Varış`, kesikli turuncu çizgi) rota ayrımı, 120 noktalık irtifa renkli geçmiş iz (`t`), detay panelinde canlı ilerleme çubuğu + kalan mesafe + tahmini varış süresi (ETA).
  - **Radar Mesafe Halkaları & Ölçek Çubuğu (`g`):** Merkeze göre eşmerkezli ATC mesafe halkaları ve sağ altta dinamik ölçek göstergesi.
- **Canlı Takip Kilidi (`L`):** Seçili uçağı kamera merkezinde otomatik olarak kilitleyip takip etme.
- **3 Renk Teması (`T`) ve Canlı İstatistikler (`i`):** Klasik, ATC Yeşil (CRT Phosphor) ve Yüksek Kontrast temaları; irtifa dağılım histogramı, rekorlar ve en yoğun ülkeler özeti.
- **Braille & ASCII Modu:** Modern terminallerde yüksek çözünürlüklü Unicode Braille; klasik Windows 10 PowerShell/CMD için tek tuşla (`m` veya `--ascii`) saf ASCII modu.
- **Filtreler, Favoriler ve Birimler:** Arama (`/`), irtifa filtresi (`a`), askeri (`M`), acil durum (`E` — `7700/7600/7500`), kalıcı favori listesi (`s`/`S`), `ft/kt` ↔ `m/km/s` birim geçişi (`u`).

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
.\radar-windows-64bit.exe --no-mouse   # Fare desteğini kapatmak gerekirse
```

### Linux
```bash
chmod +x radar-linux-64bit
./radar-linux-64bit
./radar-linux-64bit --region tr
```

## Fare Kontrolleri

| Hareket | İşlev |
|---|---|
| **Sol Tık + Sürükle** | Haritayı serbestçe kaydır |
| **Fare Tekerleği** | Haritada imleç odaklı yakınlaş/uzaklaş • Listede kaydır |
| **Sol Tık** | Haritada en yakın uçağı veya listeden satırı seç |
| **Çift Sol Tık** | Uçak üzerinde canlı takip kilidini aç • Boş haritada yakınlaş |
| **Sağ Tık** | Uçak üzerinde takip kilidini aç/kapat • Boş noktayı ortala |

## Klavye Kısayolları

| Tuş | İşlev |
|---|---|
| `1` – `5` | Hızlı bölge: Dünya, Türkiye, Avrupa, Orta Doğu, ABD |
| `Tab` | Liste ↔ Harita odak değişimi |
| `↑` `↓` `←` `→` / `hjkl` | Listede gezin veya haritayı kaydır |
| `+` / `-` | Haritayı yakınlaştır / uzaklaştır |
| `Enter` / `c` | Seçili uçağı haritada ortala |
| `L` | Canlı Takip Kilidi (Follow Mode) aç / kapat |
| `b` | Etiket modu (`AKILLI` → `ATC` → `MİN`) |
| `v` | Hız / yön vektör çizgilerini aç / kapat |
| `t` | Geçmiş iz modu (`SEÇİLİ` → `TÜMÜ` → `KAPALI`) |
| `g` | Radar mesafe halkaları ve ölçek çubuğu |
| `T` | Renk teması (`KLASİK` → `ATC YEŞİL` → `YÜKSEK KONTRAST`) |
| `i` | Canlı trafik istatistikleri paneli |
| `p` | Canlı süzülme (dead-reckoning) animasyonunu duraklat / devam ettir |
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
