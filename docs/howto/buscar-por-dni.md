# Cómo buscar por DNI (Cédula)

Esta guía te muestra cómo encontrar un registro específico usando el número de cédula (DNI).

## Cuándo usar esta guía

Usa esta guía cuando:
- Tienes el número de cédula de la persona
- Necesitas una búsqueda exacta y precisa
- Quieres el resultado más rápido posible

## Requisitos previos

- Archivo CSV con datos de identificación
- (Recomendado) Índice o base de datos SQLite construido

## Pasos

### Opción 1: Usar Índice (Recomendado)

El índice proporciona búsquedas instantáneas.

```bash
# 1. Si no tienes índice, créalo primero
./leeCSV index build --csv=datos.csv --index=indice.json

# 2. Busca por DNI
./leeCSV index search --index=indice.json --dni=12345678
```

### Opción 2: Usar SQLite

SQLite es útil si necesitas características adicionales.

```bash
# 1. Si no tienes base de datos, créala primero
./leeCSV db build --csv=datos.csv --db=datos.db

# 2. Busca por DNI
./leeCSV db search --db=datos.db --dni=12345678
```

### Opción 3: Búsqueda directa en CSV

Úsalo solo para archivos pequeños o búsquedas únicas.

```bash
./leeCSV search --csv=datos.csv --dni=12345678
```

## Formato del DNI

El DNI debe ingresarse **sin ceros a la izquierda**:

| Formato correcto | Formato incorrecto |
|-----------------|-------------------|
| `12345678` | `012345678` |
| `9876543` | `09876543` |

> **Nota:** Las búsquedas son case-insensitive para textos, pero el DNI debe coincidir exactamente con el número.

## Ejemplo completo

```bash
# Verificar que el archivo existe
ls -lh datos.csv

# Construir índice (solo una vez)
./leeCSV index build --csv=datos.csv --index=mi-indice.json

# Buscar por DNI
./leeCSV index search --index=mi-indice.json --dni=12345678
```

Salida esperada:

```
Loading index from: mi-indice.json
Index loaded: 100000 records
Found 1 matches

Nacionalidad: V
DNI: 12345678
Primer Apellido: PEREZ
Segundo Apellido: GONZALEZ
Primer Nombre: JUAN
Segundo Nombre: JOSE
Cod Centro: 001
```

## Solución de problemas

### No encuentra el registro

1. **Verifica el formato del DNI:**
   ```bash
   # Mira los primeros registros de tu CSV
   head -5 datos.csv
   ```

2. **Prueba con diferentes formatos:**
   ```bash
   ./leeCSV index search --index=mi-indice.json --dni=012345678
   ```

### Encuentra muchos registros (más de 1)

Es normal si hay duplicados. El DNI debería ser único, pero si tu CSV tiene duplicados, todos aparecerán.

## Siguiente paso

¿No conoces el DNI? Aprende a [buscar por nombre o apellido](../howto/buscar-por-nombre.md).
