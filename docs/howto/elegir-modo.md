# Cómo elegir el modo adecuado

Esta guía te ayuda a decidir qué modo de búsqueda usar según tu situación específica.

## quickstart: ¿Cuál es tu caso?

Responde estas preguntas para encontrar el modo ideal:

### Pregunta 1: ¿Qué tan grande es tu archivo?

| Tamaño | Modo | Comando |
|--------|------|---------|
| < 100,000 registros | CSV | `./leeCSV search --csv=archivo.csv --dni=...` |
| 100,000 - 1M registros | Índice | `./leeCSV index search --index=indice.json --dni=...` |
| > 1M registros | Índice o SQLite | `./leeCSV index search ...` o `./leeCSV db search ...` |

### Pregunta 2: ¿Con qué frecuencia buscarás?

| Frecuencia | Modo | ¿Por qué? |
|------------|------|-----------|
| Una vez | CSV | Sin preparación necesaria |
| Varias veces (mismo archivo) | Índice | Construyes índice una vez, búsquedas instantáneas |
| Muchas veces (múltiples queries) | SQLite | SQL flexible + índices optimizados |

### Pregunta 3: ¿Qué tipo de búsqueda necesitas?

| Tipo de búsqueda | Modo |
|-----------------|------|
| Solo por DNI exacto | Índice |
| Por nombre/apellido | Índice o SQLite |
| "Contiene" o "empieza con" | SQLite |
| Múltiples campos combinados | SQLite |
| Búsqueda full-text | SQLite con FTS5 |

---

## Decisión rápida

```
¿Archivo pequeño + solo una búsqueda?
    └─→ MODO CSV
         └─→ ./leeCSV search --csv=datos.csv --dni=12345678

¿Archivo grande + buscas frecuentemente?
    └─→ MODO ÍNDICE
         └─→ ./leeCSV index build --csv=datos.csv --index=indice.json
         └─→ ./leeCSV index search --index=indice.json --dni=12345678

¿Necesitas búsquedas complejas o "contiene"?
    └─→ MODO SQLite
         └─→ ./leeCSV db build --csv=datos.csv --db=datos.db
         └─→ ./leeCSV db search --db=datos.db --primer-nombre=Juan --pattern=contains
```

---

## Comparación detallada

### Modo CSV

**¿Cuándo usarlo?**
- Archivos pequeños (< 100,000 registros)
- Solo necesitas buscar una vez
- No quieres configurar nada

**Ventajas:**
- Sin preparación
- Funciona inmediatamente
- Bajo uso de memoria

**Desventajas:**
- Más lento para archivos grandes
- Lee todo el archivo en cada búsqueda

**Comando:**
```bash
./leeCSV search --csv=datos.csv --dni=12345678
```

---

### Modo Índice

**¿Cuándo usarlo?**
- Archivos grandes (> 100,000 registros)
- Búsquedas frecuentes sobre el mismo archivo
- Necesitas respuestas instantáneas

**Ventajas:**
- Búsquedas instantáneas (< 100ms)
- Bajo consumo de memoria
- Índice portable (archivo JSON)

**Desventajas:**
- Requiere construir el índice primero
- Solo búsqueda exacta

**Comandos:**
```bash
# Construir (solo una vez)
./leeCSV index build --csv=datos.csv --index=indice.json

# Buscar (muchas veces)
./leeCSV index search --index=indice.json --dni=12345678
```

---

### Modo SQLite

**¿Cuándo usarlo?**
- Necesitas búsquedas "contiene" o "empieza con"
- Consultas complejas con múltiples campos
- Quieres capacidad SQL completa
- Necesitas FTS5 (búsqueda full-text)

**Ventajas:**
- SQL completo
- Índices SQL optimizados
- FTS5 para búsqueda de texto
- Persistencia robusta

**Desventajas:**
- Requiere construir base de datos
- Mayor espacio en disco

**Comandos:**
```bash
# Construir base de datos (solo una vez)
./leeCSV db build --csv=datos.csv --db=datos.db

# Búsqueda exacta
./leeCSV db search --db=datos.db --dni=12345678

# Búsqueda con patrón
./leeCSV db search --db=datos.db --primer-nombre=Juan --pattern=contains
```

---

## FTS5 (Búsqueda Full-Text)

FTS5 es una característica de SQLite para búsquedas avanzadas.

**¿Cuándo usar FTS5?**
- Tienes mucho texto para buscar
- Necesitas ranking de resultados
- Búsquedas muy frecuentes

**¿Cómo habilitarlo?**
```bash
# Compilar con FTS5
go build -tags fts5 -o leeCSV .

# Luego usa SQLite normalmente
./leeCSV db build --csv=datos.csv --db=datos.db
./leeCSV db search --db=datos.db --primer-nombre=Juan
```

---

## Resumen visual

```
                        ┌─────────────────────────────────────┐
                        │     ¿Qué modo debería usar?         │
                        └─────────────────────────────────────┘
                                        │
            ┌───────────────────────────┼───────────────────────────┐
            ▼                           ▼                           ▼
    ┌───────────────┐           ┌───────────────┐           ┌───────────────┐
    │ ¿Archivo      │           │ ¿Necesitas    │           │ ¿Buscas con   │
    │ pequeño?      │           │ SQL avanzado? │           │ "contiene"?   │
    └───────────────┘           └───────────────┘           └───────────────┘
            │                           │                           │
      Sí ───┴─── No              Sí ───┴─── No              Sí ───┴─── No
            │                           │                           │
            ▼                           ▼                           ▼
    ┌───────────────┐           ┌───────────────┐           ┌───────────────┐
    │     CSV       │           │    SQLite     │           │    SQLite     │
    │  (búsqueda    │           │  (FTS5 o      │           │  (pattern=    │
    │   única)      │           │   LIKE)       │           │   contains)   │
    └───────────────┘           └───────────────┘           └───────────────┘
            │
            ▼
    ┌───────────────┐
    │ ¿Archivo      │
    │ grande +      │
    │ frecuente?    │
    └───────────────┘
            │
      Sí ───┴─── No
            │
            ▼
    ┌───────────────┐
    │    ÍNDICE    │
    │  (búsqueda    │
    │  instantánea)│
    └───────────────┘
```

---

## Recomendaciones finales

| Tu situación | Recomendación |
|--------------|---------------|
| Nuevo usuario, archivo pequeño | CSV |
| Búsquedas frecuentes, archivo grande | Índice |
| Queries complejos | SQLite |
| No sabes qué elegir | Índice |

**Mi recomendación personal:** Si tienes dudas, usa el **índice**. Es el mejor equilibrio entre velocidad y simplicidad.
