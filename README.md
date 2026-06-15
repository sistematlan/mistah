# mistah

> *mistah* (maya yucateco): «él que barre», del verbo *mis* (barrer).

CLI open-source para macOS que limpia tu disco como desarrollador.
Detecta cachés de desarrollo, datos huérfanos y archivos olvidados.
Cero telemetría. Código auditable. MIT.

🌐 https://mistah.sistematlan.com

## Instalación

```sh
curl -fsSL https://mistah.sistematlan.com/install.sh | sh
```

El script detecta tu arquitectura (`arm64` o `amd64`), descarga el binario
de la última release de GitHub, y lo coloca en `/usr/local/bin/mistah`.

Lee `install.sh` antes de ejecutarlo si tienes dudas — son 200 líneas de
shell POSIX sin sorpresas.

## Uso rápido

```sh
mistah                          # wizard guiado con tres niveles
mistah scan                     # escaneo informativo, no borra nada
mistah clean --dry-run          # ver qué se borraría
mistah clean                    # interactivo, ítem por ítem
mistah --help                   # ayuda completa
mistah --advanced --help        # mostrar todos los comandos avanzados
```

### Niveles del wizard

| Nivel | Qué incluye |
|---|---|
| **Ligera** | Solo cachés seguras (npm, brew, pip, uv, etc.) |
| **Estándar** | + Docker prune + JetBrains old + Xcode artifacts |
| **Profunda** | + datos huérfanos + candidatos en Downloads |

## Características

- 🧹 18+ detectores de caché (npm, pnpm, brew, JetBrains, Docker, Cargo, Xcode...)
- 🧠 Clasificación inteligente de Downloads (instaladores duplicados, ZIPs ya extraídos, dumps viejos)
- 🌐 Bilingüe — autodetecta `$LANG` (es / en)
- 🔒 Cero telemetría, cero red, cero analytics
- ✋ Confirmación explícita por ítem (modo `clean`) o por nivel (modo wizard)
- 🛡️ `SafeRoots` — sólo borra dentro de `$HOME` y `/tmp`, jamás fuera

## Construir desde código

```sh
git clone https://github.com/sistematlan/mistah.git
cd mistah
make build
./bin/mistah
```

Requiere Go 1.26+.

## Roadmap

Ver [BACKLOG.md](BACKLOG.md). En curso:

- `mistah report --json` para integración con scripts
- Detectores P0 para v0.2.0: Electron caches, iOS Simulator, Time Machine snapshots, Mail/Messages
- Apple notarization para que Gatekeeper no se queje al primer arranque
- Soporte Linux (largo plazo)

## Contribuir

PRs y reports bienvenidos. Antes de PRs grandes:

1. Abre un issue describiendo el cambio.
2. `make test` debe pasar.
3. Sigue el estilo Go idiomático del repo (errores explícitos, paquetes pequeños, sin globals).

## Licencia

MIT. Ver [LICENSE](LICENSE).

## Créditos

Construido por [@chrisherlan](https://github.com/chrisherlan) en México,
bajo el paraguas de [sistematlan](https://sistematlan.com).
