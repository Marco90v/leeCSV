# Cómo buscar por nombre o apellido

Esta guía te muestra cómo encontrar registros cuando no conoces el número de cédula.

## Cuándo usar esta guía

Usa esta guía cuando:
- No tienes el número de cédula
- Conoces el nombre o apellido de la persona
- Necesitas encontrar múltiples registros que coincidan

## Búsqueda básica por nombre

### Por primer nombre

```bash
./leeCSV index search --index=indice.json --primer-nombre=Juan
```

### Por apellido

```bash
./leeCSV index search --index=indice.json --primer-apellido=Perez
```

## Búsqueda combinada

Usa múltiples campos para ser más preciso:

```bash
# Nombre + Apellido
./leeCSV index search --index=indice.json --primer-nombre=Juan --primer-apellido=Perez
```

## Búsqueda con SQLite (patrones)

Si necesitas búsquedas más flexibles, usa SQLite con patrones:

### Búsqueda "contiene"

Encuentra nombres que **contengan** el término:

```bash
./leeCSV db search --db=datos.db --primer-nombre=Juan --pattern=contains
```

Esto encuentra: "Juan", "Juana", "Juan Carlos", etc.

### Búsqueda "comienza con"

Encuentra nombres que **empiecen con** el término:

```bash
./leeCSV db search --db=datos.db --primer-nombre=Ju --pattern=startswith
```

Esto encuentra: "Juan", "Juana", "Julian", etc.

## Lógica de búsqueda

### AND (todas las condiciones)

Por defecto, todas las condiciones deben cumplirse:

```bash
./leeCSV index search --index=indice.json --primer-nombre=Juan --primer-apellido=Perez --logic=AND
```

**Resultado:** Solo personas llamadas "Juan" con apellido "Perez".

### OR (cualquier condición)

Usa `--logic=OR` para encontrar cualquiera de las condiciones:

```bash
./leeCSV index search --index=indice.json --primer-nombre=Juan --primer-apellido=Perez --logic=OR
```

**Resultado:** Personas llamadas "Juan" O con apellido "Perez".

## Ejemplo completo

### Escenario
Tienes un archivo grande y buscas a "María García".

### Paso 1: Construir índice (primera vez)
```bash
./leeCSV index build --csv=datos.csv --index=indice.json
```

### Paso 2: Buscar por nombre
```bash
./leeCSV index search --index=indice.json --primer-nombre=Maria --primer-apellido=Garcia
```

### Paso 3: Si hay muchos resultados,refina
```bash
./leeCSV index search --index=indice.json --primer-nombre=Maria --segundo-nombre=Elena --primer-apellido=Garcia
```

## Campos disponibles

| Campo | Flag | Ejemplo |
|-------|------|---------|
| Primer nombre | `--primer-nombre` | `--primer-nombre=Juan` |
| Segundo nombre | `--segundo-nombre` | `--segundo-nombre=Carlos` |
| Primer apellido | `--primer-apellido` | `--primer-apellido=Perez` |
| Segundo apellido | `--segundo-apellido` | `--segundo-apellido=Gonzalez` |

## Solución de problemas

### No encuentra resultados

1. **Verifica mayúsculas/minúsculas:**
   Las búsquedas son case-insensitive, pero el texto debe existir.
   
   ```bash
   # Prueba variaciones
   ./leeCSV index search --index=indice.json --primer-nombre=juan
   ./leeCSV index search --index=indice.json --primer-nombre=JUAN
   ```

2. **Usa SQLite con "contains":**
   ```bash
   ./leeCSV db search --db=datos.db --primer-nombre=Mar --pattern=contains
   ```

### Demasiados resultados

1. **Agrega más criterios:**
   ```bash
   # En lugar de solo nombre
   ./leeCSV index search --index=indice.json --primer-nombre=Maria
   
   # Agrega apellido
   ./leeCSV index search --index=indice.json --primer-nombre=Maria --primer-apellido=Garcia
   ```

2. **Usa lógica AND (más restrictivo)**

## Comparación de modos

| Necesidad | Modo recomendado |
|-----------|------------------|
| Búsqueda exacta por nombre | Índice |
| Búsqueda parcial (contiene) | SQLite con `--pattern=contains` |
| Múltiples nombres/apellidos | Índice o SQLite |
| Búsqueda muy específica | SQLite |

## Siguiente paso

¿Aún no sabes qué modo usar? Consulta [elegir el modo adecuado](./elegir-modo.md).
