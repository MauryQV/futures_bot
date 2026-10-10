| `go version` | Comprueba que Go (1.22+) está instalado. |
| `cp .env.example .env` | Crea tu archivo de configuración (en Windows: `copy .env.example .env`). |
| `go run . -check` | Verifica que las claves de Binance demo y los permisos de Futures funcionan, y muestra los balances. |
| `go test ./...` | Ejecuta todos los tests del proyecto. |
| `go run .` | Modo manual: muestra el plan de la operación y la ejecuta solo si escribes `SI`. |
| `go run . -side short -margin 50 -leverage 3` | Modo manual con parámetros propios (lado, margen, apalancamiento, `-tp`, `-sl`, `-type limit -price X`). |
| `go run . -auto` | Modo autónomo: el LLM decide y el bot opera solo, sin confirmación. |
| `go run . -auto -strategy ema` | Modo autónomo con cruce de EMAs, sin usar el LLM. |
| `go run . -auto -interval-min 5 -cooldown-min 5` | Modo autónomo con ciclos cortos, útil para probar rápido. |
| `go build -o bot .` | Compila un ejecutable (luego se usa `./bot -auto`). |
| `Ctrl+C` | Detiene el bot; las posiciones abiertas conservan su TP/SL en el exchange. |
| `tail -f bot.log` | Sigue el log en vivo (en Windows PowerShell: `Get-Content bot.log -Wait`). |
