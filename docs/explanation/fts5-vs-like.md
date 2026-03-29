# FTS5 vs LIKE: Cuándo usar cada uno

Este documento explica las diferencias entre FTS5 (Full-Text Search) y LIKE para que puedas elegir correctamente.

## En resumen

| Característica | LIKE | FTS5 |
|---------------|------|------|
| **Complejidad** | Simple | Configuración adicional |
| **Velocidad** | Lento para texto largo | Rápido para texto largo |
| **Búsqueda** | Patrones simples | Búsqueda de texto avanzado |
| **Relevancia** | No tiene ranking | Ranking de resultados |
| **Build** | No requiere | Requiere `-tags fts5` |

---

## LIKE: Búsqueda tradicional

LIKE es el método estándar de SQL para buscar patrones en texto.

### Cómo funciona

```sql
-- Búsqueda exacta
SELECT * FROM personas WHERE nombre = 'Juan'

-- LIKE: contiene
SELECT * FROM personas WHERE nombre LIKE '%Juan%'

-- LIKE: comienza con
SELECT * FROM personas WHERE nombre LIKE 'Juan%'
```

### En leeCSV (modo SQLite sin FTS5)

```bash
# Búsqueda exacta (usa índice)
./leeCSV db search --db=datos.db --dni=12345678

# "Contiene" (usa LIKE %...%)
./leeCSV db search --db=datos.db --primer-nombre=Juan --pattern=contains

# "Comienza con" (usa LIKE ...)
./leeCSV db search --db=datos.db --primer-nombre=Juan --pattern=startswith
```

### Ventajas de LIKE

- ✅ **Simple** - No requiere configuración especial
- ✅ **Compatible** - Funciona en cualquier instalación de SQLite
- ✅ **Flexible** - Soporta cualquier patrón con wildcards (`%`, `_`)

### Desventajas de LIKE

- ❌ **Lento en texto largo** - Debe escanear cada caracter
- ❌ **Sin ranking** - No sabe qué resultado es "mejor"
- ❌ **Sin operadores avanzados** - No hay OR, AND, frases, etc.

---

## FTS5: Búsqueda Full-Text

FTS5 es una extensión de SQLite para búsqueda de texto completo.

### Cómo funciona

FTS5 crea un **índice invertido** (como Google):

```
Documento: "El perro juega con la pelota"

Índice FTS5:
  "el"     → [doc1, doc5, doc10]
  "perro"  → [doc1, doc3]
  "juega"  → [doc1]
  "pelota" → [doc1, doc7]
```

Esto permite búsquedas **mucho más rápidas** porque no escanea el texto.

### En leeCSV (modo SQLite con FTS5)

```bash
# Primero compila con FTS5
go build -tags fts5 -o leeCSV .

# Construcción automática usa FTS5
./leeCSV db build --csv=datos.csv --db=datos.db

# Búsqueda automática usa FTS5
./leeCSV db search --db=datos.db --primer-nombre=Juan
```

### Ventajas de FTS5

- ✅ **Muy rápido** - Índice invertido, no escanea texto
- ✅ **Relevancia** -Puedes ordenar por "match score"
- ✅ **Operadores avanzados** - AND, OR, NOT, frases con comillas
- ✅ **Tokenización** - Separa palabras intelligently

### Desventajas de FTS5

- ❌ **Build adicional** - Requiere compilar con `-tags fts5`
- ❌ **Más espacio** - El índice FTS5 ocupa espacio adicional
- ❌ **Setup** - Un paso extra en el build

---

## Cuándo usar cada uno

### Usa LIKE cuando:

| Situación | Ejemplo |
|-----------|---------|
| ✅ Archivos pequeños | < 100,000 registros |
| ✅ Búsquedas simples | Nombres cortos, búsquedas exactas |
| ✅ No quieres compilar con tags | Simplicidad prima |
| ✅ Búsquedas frecuentes | LIKE con índices es rápido |

### Usa FTS5 cuando:

| Situación | Ejemplo |
|-----------|---------|
| ✅ Archivos grandes | > 1 millón de registros |
| ✅ Búsquedas complejas | Múltiples términos, frases |
| ✅ Necesitas ranking | "Los más relevantes primero" |
| ✅ Textos largos | Descripciones, comentarios |

---

## Ejemplo comparativo

### Escenario: Buscar "Juan" en 1 millón de registros

**Con LIKE:**
```bash
time ./leeCSV db search --db=datos.db --primer-nombre=Juan --pattern=contains
# Tiempo: ~5-10 segundos
```

**Con FTS5:**
```bash
# Compilar con FTS5
go build -tags fts5 -o leeCSV .

# Rebuild de la base
./leeCSV db build --csv=datos.csv --db=datos.db

# Buscar
time ./leeCSV db search --db=datos.db --primer-nombre=Juan
# Tiempo: ~0.1-0.5 segundos
```

**Resultado:** FTS5 es **10-100x más rápido** para búsquedas de texto.

---

## Recomendación

| Tu caso | Recomendación |
|---------|---------------|
| Nuevousuario, probar | LIKE (sin FTS5) |
| Búsquedasoccionales | LIKE (suficiente) |
| Alto volumen, perf crítica | FTS5 |
| Búsquedascomplejas | FTS5 |
| No sabes qué elegir | LIKE (empieza simple) |

**Mi recomendación:** Empieza con LIKE (más simple). Cuando notes que las búsquedas son lentas, migra a FTS5.

---

## Cómo migrar a FTS5

1. **Recompilar:**
   ```bash
   go build -tags fts5 -o leeCSV .
   ```

2. **Rebuild de la base de datos:**
   ```bash
   ./leeCSV db build --csv=datos.csv --db=datos.db
   ```

3. **Listo!** Las búsquedas usarán FTS5 automáticamente.

> **Nota:** No necesitas cambiar los comandos de búsqueda. LeeCSV detecta automáticamente si FTS5 está disponible.
