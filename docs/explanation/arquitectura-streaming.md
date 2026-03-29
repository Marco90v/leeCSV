# Arquitectura de Streaming

Este documento explica cómo leeCSV procesa archivos grandes sin agotar la memoria.

## El problema

Los archivos CSV con 30 millones de registros pueden ocupar varios gigabytes. Si intentas cargar todo en memoria:

1. **Agotamiento de RAM** - Tu computadora se queda sin memoria
2. **Intercambio (swap)** - El sistema usa el disco como memoria, muy lento
3. **Colapso del sistema** - Puede freezes o crashes

## La solución: Streaming

leeCSV usa **streaming** (flujo de datos) en lugar de cargar todo en memoria.

### ¿Qué es streaming?

Imagina dos formas de leer un libro:

| Método | Analogía | En leeCSV |
|--------|----------|-----------|
| **Cargar todo** | Comprar el libro entero y leerlo de una vez | `ReadFile()` - carga todo a RAM |
| **Streaming** | Leer página por página, sin poseer el libro | `csv.NewReader()` - lee línea a línea |

### Cómo funciona en leeCSV

```
┌─────────────────────────────────────────────────────────────┐
│                    Archivo CSV (10GB)                       │
│  V;12345678;PEREZ;...;V;12345679;GARCIA;...;V;12345680   │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                 csv.NewReader (streaming)                   │
│  Lee un registro a la vez, no todo el archivo              │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│              Procesamiento registro por registro            │
│  • Validar registro                                         │
│  • Agregar a índice (si corresponde)                        │
│  • Verificar si coincide con búsqueda (si corresponde)      │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    Memoria usada                             │
│  ~100MB constante, sin importar el tamaño del CSV          │
└─────────────────────────────────────────────────────────────┘
```

## Los tres modos y la memoria

### Modo CSV

```
CSV → Reader → Filtrar → Mostrar resultados
            │
            ▼
     Memoria: ~100MB
```

- Lee el archivo registro por registro
- Aplica los filtros mientras lee
- Solo guarda los resultados que coinciden

### Modo Índice

```
CSV → Reader → Construir índice en memoria → Guardar a JSON
            │
            ▼
     Memoria: ~300MB (para índice de 30M registros)
```

- Lee el archivo para construir el índice
- El índice se guarda a disco (JSON)
- Para buscar: carga el índice a memoria (~300MB para 30M)

### Modo SQLite

```
CSV → Reader → Insertar en SQLite → Índices automáticos
            │
            ▼
     Memoria: ~200MB (buffer de SQLite)
```

- Lee el CSV e inserta en SQLite
- SQLite maneja la memoria internamente
- Los índices se crean automáticamente

## Por qué no cargar todo en memoria

### El enfoque "incorrecto" (cargar todo)

```go
// ESTO ESTÁ MAL para archivos grandes
func ReadAll(path string) ([]Record, error) {
    data, err := os.ReadFile(path)  // Carga TODO a memoria
    // ... parsear todo ...
    return records, nil
}
```

**Problema:** Con un archivo de 10GB, esto intenta allocate 10GB de RAM.

### El enfoque correcto (streaming)

```go
// ESTO ESTÁ BIEN
func ReadStreaming(path string, fn func(Record)) error {
    file, _ := os.Open(path)
    reader := csv.NewReader(file)
    
    for {
        record, err := reader.Read()  // Lee UNA línea
        if err == io.EOF { break }
        fn(record)  // Procesa y olvida
    }
    return nil
}
```

**Ventaja:** Solo usa memoria para una línea a la vez (~1KB).

## Rendimiento comparativo

| Enfoque | Memoria | Tiempo (30M registros) |
|---------|---------|----------------------|
| Cargar todo | 10GB+ | Rápido pero imposible |
| Streaming | ~100MB | 5-10 minutos |
| Streaming + workers | ~100MB | 2-5 minutos |

## Conclusión

El streaming permite que leeCSV funcione con archivos de cualquier tamaño usando solo ~100MB de memoria. Este es el diseño correcto para manejar datos masivos.

El tradeoff es tiempo vs. memoria:
- **Más memoria** = más rápido (cargar todo)
- **Menos memoria** = más lento (streaming)

Para 30M+ registros, preferir **streaming** es la única opción práctica en máquinas regulares.
