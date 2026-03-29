# leeCSV

[![Go Version](https://img.shields.io/github/go-mod/go-version/Marco90v/leeCSV)](https://github.com/Marco90v/leeCSV)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Herramienta CLI de alto rendimiento para buscar y filtrar registros en archivos CSV masivos (30M+ registros) conteniendo datos de identificación nacional venezolana.

## Tutorial: Primeros Pasos

Esta guía te llevará de la mano para tu primera búsqueda con leeCSV. No necesitas conocer Go ni programación.

### 1. Instalación

```bash
# Clonar el repositorio
git clone https://github.com/Marco90v/leeCSV.git
cd leeCSV

# Compilar
go build -o leeCSV .

# Ver ayuda
./leeCSV --help
```

### 2. Prepara tu archivo CSV

Asegúrate de tener un archivo CSV con el formato esperado:

```csv
Nacionalidad;Cedula;Primer_Apellido;Segundo_Apellido;Primer_Nombre;Segundo_Nombre;Cod_Centro
V;12345678;PEREZ;GONZALEZ;JUAN;JOSE;001
```

> **Nota:** El separador es punto y coma (`;`).

### 3. Tu primera búsqueda

**Opción A: Búsqueda directa (para archivos pequeños)**

```bash
./leeCSV search --csv=tus-datos.csv --dni=12345678
```

**Opción B: Construir índice (recomendado para archivos grandes)**

```bash
# Paso 1: Construir el índice (solo una vez)
./leeCSV index build --csv=tus-datos.csv --index=mi-indice.json

# Paso 2: Buscar usando el índice (rápido)
./leeCSV index search --index=mi-indice.json --dni=12345678
```

### 4. ¿Qué hacer si no conoces el DNI?

Usa búsqueda por nombre o apellido:

```bash
# Buscar por primer nombre
./leeCSV index search --index=mi-indice.json --primer-nombre=Juan

# Buscar por apellido
./leeCSV index search --index=mi-indice.json --primer-apellido=Perez

# Combin múltiples criterios
./leeCSV index search --index=mi-indice.json --primer-nombre=Juan --primer-apellido=Perez
```

### 5. Siguientes pasos

- Explora los [modos de búsqueda](#modos-de-búsqueda)
- Aprende a [elegir el modo correcto](#elegir-el-modo-adecuado)
- Consulta la [referencia de comandos](#referencia-de-comandos)

---

## Elegir el Modo Adecuado

Esta guía te ayuda a decidir qué modo usar según tu situación.

### ¿Qué tipo de archivo tienes?

| Tamaño del archivo | Modo recomendado | Por qué |
|-------------------|------------------|---------|
| < 100,000 registros | CSV | Sin preparación necesaria |
| 100,000 - 1M registros | Índice | Rápido y simple |
| 1M - 10M registros | Índice | Búsquedas instantáneas |
| > 10M registros | Índice o SQLite | SQL para queries complejos |

### ¿Con qué frecuencia buscarás?

| Frecuencia | Modo recomendado | Por qué |
|------------|------------------|---------|
| Una sola vez | CSV | No requiere preparación |
| Varias veces (mismo archivo) | Índice | Construyes el índice una vez, búsquedas instantáneas |
| Queries complejos (múltiples campos) | SQLite | Potencia de SQL |

### ¿Qué tipo de búsqueda necesitas?

| Necesidad | Modo recomendado |
|-----------|------------------|
| Solo por DNI exacto | Índice |
| Búsquedas parciales (contiene, empieza con) | SQLite |
| Múltiples campos combinados | SQLite |
| Búsqueda full-text | SQLite con FTS5 |

### Resumen rápido

```
¿Archivo pequeño + búsqueda única?
    └─→ Usa modo CSV

¿Archivo grande + búsquedas frecuentes?
    └─→ Usa modo Índice

¿Necesitas SQL avanzado o FTS5?
    └─→ Usa modo SQLite
```

---

## Modos de Búsqueda

leeCSV ofrece tres modos de búsqueda, cada uno con ventajas específicas.

### Comparación

| Modo | Comando | Ventajas | Desventajas | Mejor para |
|------|---------|----------|-------------|------------|
| **CSV** | `search` | Sin preparación, simple | Más lento | Archivos pequeños, búsqueda única |
| **Índice** | `index search` | Rápido (~instantáneo), bajo uso de memoria | Carga índice en RAM | Búsquedas frecuentes, archivos grandes |
| **SQLite** | `db search` | Queries complejos, FTS5, índices SQL | Requiere build inicial | Análisis avanzado, búsquedas flexibles |

### Modo CSV

Búsqueda directa en el archivo CSV. No requiere preparación.

```bash
./leeCSV search --csv=datos.csv --dni=12345678
```

**Ventajas:**
- No requiere construir nada
- Funciona inmediatamente
- Bajo uso de memoria (streaming)

**Desventajas:**
- Más lento para archivos grandes
- Lee el archivo completo en cada búsqueda

### Modo Índice

Construyes un índice una vez, luego búsquedas instantáneas.

```bash
# Construir índice
./leeCSV index build --csv=datos.csv --index=indice.json

# Buscar
./leeCSV index search --index=indice.json --dni=12345678
```

**Ventajas:**
- Búsquedas instantáneas (< 100ms)
- Bajo consumo de memoria
- Persistente (guardas el índice)

**Desventajas:**
- Requiere construir el índice primero
- El índice ocupa RAM

### Modo SQLite

Base de datos embebida para máxima flexibilidad.

```bash
# Construir base de datos
./leeCSV db build --csv=datos.csv --db=datos.db

# Buscar
./leeCSV db search --db=datos.db --dni=12345678

# Búsqueda con patrón "contiene"
./leeCSV db search --db=datos.db --primer-nombre=Juan --pattern=contains
```

**Ventajas:**
- SQL completo para consultas complejas
- Soporta FTS5 (búsqueda full-text)
- Índices SQL optimizados automáticamente

**Desventajas:**
- Requiere construir la base de datos
- Mayor espacio en disco

---

## Instalación

### Requisitos

- Go 1.21+
- (Opcional) gcc para compilar go-sqlite3

### Compilación

```bash
# Versión básica
go build -o leeCSV .

# Con FTS5 para búsqueda full-text
go build -tags fts5 -o leeCSV .

# Instalar globalmente
go install
```

---

## Referencia de Comandos

### Flags globales

| Flag | Descripción | Valor por defecto |
|------|-------------|-------------------|
| `--csv` | Ruta al archivo CSV | `./nacional.csv` |
| `--index` | Ruta al archivo de índice | `./index.json` |
| `--db` | Ruta a la base de datos SQLite | `./data.db` |
| `-w, --workers` | Número de workers (0=auto) | 0 (automático) |

### Campos de búsqueda

| Flag | Descripción |
|------|-------------|
| `--dni` | Cédula de identidad |
| `--primer-nombre` | Primer nombre |
| `--segundo-nombre` | Segundo nombre |
| `--primer-apellido` | Primer apellido |
| `--segundo-apellido` | Segundo apellido |

### Patrones de búsqueda (SQLite)

| Patrón | Descripción |
|--------|-------------|
| `exact` | Coincidencia exacta (por defecto) |
| `contains` | Contiene el término |
| `startswith` | Comienza con |

### Lógica de búsqueda

| Lógica | Descripción |
|--------|-------------|
| `AND` | Todas las condiciones deben cumplirse (por defecto) |
| `OR` | Cualquier condición se cumple |

---

## Formato del CSV

### Estructura esperada

```
Nacionalidad;Cedula;Primer_Apellido;Segundo_Apellido;Primer_Nombre;Segundo_Nombre;Cod_Centro
V;12345678;PEREZ;GONZALEZ;JUAN;JOSE;001
E;87654321;GARCIA;MARTINEZ;MARIA;ELENA;002
```

### Campos

| # | Campo | Descripción |
|---|-------|-------------|
| 1 | Nacionalidad | Nacionalidad (V, E, etc.) |
| 2 | Cedula | Número de cédula (DNI) |
| 3 | Primer_Apellido | Primer apellido |
| 4 | Segundo_Apellido | Segundo apellido |
| 5 | Primer_Nombre | Primer nombre |
| 6 | Segundo_Nombre | Segundo nombre |
| 7 | Cod_Centro | Código del centro de emisión |

**Notas:**
- Separador: punto y coma (`;`)
-Primera línea: header (se omite automáticamente)
- Codificación: UTF-8

---

## Rendimiento

### Benchmarks aproximados

| Operación | Tiempo estimado |
|-----------|------------------|
| Indexación (índice) | ~6-7M registros/minuto |
| Búsqueda por DNI (índice) | < 100ms |
| Búsqueda por nombre (índice) | < 100ms |
| Memoria (streaming CSV) | ~100MB independiente del tamaño |

### Optimización

- **Workers:** Por defecto usa todos los cores de CPU disponibles. Usa `-w` para limitar.
- **Índice:** Construye el índice una vez, úsalo muchas veces.
- **FTS5:** Habilita con `go build -tags fts5` para búsquedas full-text más rápidas.

---

## Construcción con FTS5

FTS5 (Full-Text Search) proporciona búsquedas más rápidas en campos de texto.

```bash
# Compilar con FTS5
go build -tags fts5 -o leeCSV .
```

**Cuándo usar FTS5:**
- Búsquedas frecuentes de texto
- Necesitas buscar dentro de texto largo
- Queries complejos con ranking de resultados

---

## Contribución

1. Fork del repositorio
2. Crear una rama (`git checkout -b feature/nueva-funcionalidad`)
3. Commit de los cambios (`git commit -m 'Agrega nueva funcionalidad'`)
4. Push a la rama (`git push origin feature/nueva-funcionalidad`)
5. Crear un Pull Request

---

## Licencia

MIT License - Ver archivo LICENSE para más detalles.
