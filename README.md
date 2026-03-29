# leeCSV

[![Go Version](https://img.shields.io/github/go-mod/go-version/Marco90v/leeCSV)](https://github.com/Marco90v/leeCSV)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Herramienta CLI de alto rendimiento para buscar y filtrar registros en archivos CSV masivos (30M+ registros) conteniendo datos de identificación nacional venezolana.

## Características

- ✅ **Streaming** - Procesa archivos Gigabyte sin agotar memoria
- ✅ **Paralelismo** - Workers configurables para búsquedas rápidas
- ✅ **Múltiples modos** - CSV, Índice en memoria, SQLite
- ✅ **Búsqueda flexible** - Exacta, contiene, comienza con
- ✅ **Lógica combinable** - AND/OR para múltiples criterios
- ✅ **FTS5** - Búsqueda full-text opcional (build tag)
- ✅ **Case-insensitive** - Búsquedas sin distinción de mayúsculas
- ✅ **Cancelación** - Context support para operaciones largas

## Instalación

```bash
# Clonar el repositorio
git clone https://github.com/Marco90v/leeCSV.git
cd leeCSV

# Compilar (versión básica)
go build -o leeCSV .

# Compilar con FTS5 para búsqueda full-text
go build -tags fts5 -o leeCSV .

# Instalar globalmente
go install

# Ver ayuda
./leeCSV --help
```

**Requisitos:**
- Go 1.21+
- (Opcional) gcc para compilar go-sqlite3

## Uso Rápido

### Modo CSV (streaming)

```bash
leeCSV search --csv=data.csv --dni=12345678
```

### Modo Índice (recomendado para 30M+)

```bash
leeCSV index build --csv=data.csv --index=index.json
leeCSV index search --index=index.json --dni=12345678
```

### Modo SQLite (mejor para queries complejos)

```bash
leeCSV db build --csv=data.csv --db=data.db
leeCSV db search --db=data.db --primer-nombre=Juan --pattern=contains
```

## Modos de Búsqueda

| Modo | Comando | Ventajas | Desventajas | Mejor para |
|------|---------|----------|-------------|------------|
| **CSV** | `search` | Sin preparación | Más lento | Archivos pequeños/medios |
| **Index** | `index search` | Rápido (~instantáneo) | Carga índice en RAM | Búsquedas repetidas |
| **SQLite** | `db search` | Queries complejos, FTS5 | Requiere build | Análisis avanzado |

## Referencia de Comandos

### Flags globales

| Flag | Descripción |
|------|-------------|
| `--csv` | Ruta al archivo CSV |
| `--index` | Ruta al archivo de índice |
| `--db` | Ruta a la base de datos SQLite |
| `-w, --workers` | Número de workers (0=auto) |

### Búsqueda por campo

| Flag | Descripción |
|------|-------------|
| `--dni` | Cédula de identidad |
| `--primer-nombre` | Primer nombre |
| `--segundo-nombre` | Segundo nombre |
| `--primer-apellido` | Primer apellido |
| `--segundo-apellido` | Segundo apellido |

### Patrones de búsqueda

| Patrón | Descripción |
|--------|-------------|
| `--pattern=exact` | Coincidencia exacta |
| `--pattern=contains` | Contiene el término |
| `--pattern=startswith` | Comienza con |

### Lógica de búsqueda

| Lógica | Descripción |
|--------|-------------|
| `--logic=AND` | Todas las condiciones deben cumplirse |
| `--logic=OR` | Cualquier condición se cumple |

## Formato del CSV

```
Nacionalidad;Cedula;Primer_Apellido;Segundo_Apellido;Primer_Nombre;Segundo_Nombre;Cod_Centro
V;12345678;PEREZ;GONZALEZ;JUAN;JOSE;001
```

### Campos

| # | Campo | Descripción |
|---|-------|-------------|
| 1 | Nacionalidad | Nacionalidad (V, E, etc.) |
| 2 | Cedula | Número de cédula |
| 3 | Primer_Apellido | Primer apellido |
| 4 | Segundo_Apellido | Segundo apellido |
| 5 | Primer_Nombre | Primer nombre |
| 6 | Segundo_Nombre | Segundo nombre |
| 7 | Cod_Centro | Código del centro |

## Rendimiento

- **Indexación:** ~6-7M registros/minuto
- **Búsqueda por DNI:** <100ms (modo Index/SQLite)
- **Memoria:** Streaming usa ~100MB independiente del tamaño del archivo
- **Workers:** Por defecto usa todos los cores de CPU disponibles

## Construcción con FTS5

```bash
# Habilitar búsqueda full-text
go build -tags fts5 -o leeCSV .
```

FTS5 permite búsquedas más rápidas en campos de texto.

## Contribución

1. Fork del repositorio
2. Crear una rama (`git checkout -b feature/nueva-funcionalidad`)
3. Commit de los cambios (`git commit -m 'Agrega nueva funcionalidad'`)
4. Push a la rama (`git push origin feature/nueva-funcionalidad`)
5. Crear un Pull Request

## Licencia

MIT License - Ver archivo LICENSE para más detalles.
