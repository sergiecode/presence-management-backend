# Enum Values for Frontend

This document contains the enum values that should be used in the frontend as constants.

## Absence Types

| ID | Constant Name | Display Text |
|----|---------------|--------------|
| 1 | `ABSENCE_MATERNITY` | Licencia por maternidad |
| 2 | `ABSENCE_SICK_LEAVE` | Licencia por enfermedad |
| 3 | `ABSENCE_SICK_ABSENCE` | Ausente por enfermedad |
| 4 | `ABSENCE_FAMILY_SICK` | Ausente por enfermedad familiar |
| 5 | `ABSENCE_STUDY` | Ausente por día de estudio/examen |
| 6 | `ABSENCE_BEREAVEMENT` | Ausente por duelo |
| 7 | `ABSENCE_MOVING` | Día por mudanza |
| 8 | `ABSENCE_VACATION` | Vacaciones |
| 9 | `ABSENCE_LATE` | Tarde |
| 10 | `ABSENCE_MEDICAL` | Médico |
| 11 | `ABSENCE_GENERAL` | Ausencia |

## Location Types

| ID | Constant Name | Display Text |
|----|---------------|--------------|
| 1 | `LOCATION_REMOTE_DECLARED` | Domicilio remoto declarado |
| 2 | `LOCATION_REMOTE_ALTERNATIVE` | Domicilio remoto alternativo |
| 3 | `LOCATION_CLIENT` | Domicilio del cliente |
| 4 | `LOCATION_OFFICE` | Oficina de ABSTI |

## Frontend Implementation Example

```typescript
// constants/enums.ts
export const ABSENCE_TYPES = {
  MATERNITY: 1,
  SICK_LEAVE: 2,
  SICK_ABSENCE: 3,
  FAMILY_SICK: 4,
  STUDY: 5,
  BEREAVEMENT: 6,
  MOVING: 7,
  VACATION: 8,
  LATE: 9,
  MEDICAL: 10,
  GENERAL: 11,
} as const;

export const LOCATION_TYPES = {
  REMOTE_DECLARED: 1,
  REMOTE_ALTERNATIVE: 2,
  CLIENT: 3,
  OFFICE: 4,
} as const;

export const ABSENCE_TYPE_LABELS = {
  [ABSENCE_TYPES.MATERNITY]: 'Licencia por maternidad',
  [ABSENCE_TYPES.SICK_LEAVE]: 'Licencia por enfermedad',
  [ABSENCE_TYPES.SICK_ABSENCE]: 'Ausente por enfermedad',
  [ABSENCE_TYPES.FAMILY_SICK]: 'Ausente por enfermedad familiar',
  [ABSENCE_TYPES.STUDY]: 'Ausente por día de estudio/examen',
  [ABSENCE_TYPES.BEREAVEMENT]: 'Ausente por duelo',
  [ABSENCE_TYPES.MOVING]: 'Día por mudanza',
  [ABSENCE_TYPES.VACATION]: 'Vacaciones',
  [ABSENCE_TYPES.LATE]: 'Tarde',
  [ABSENCE_TYPES.MEDICAL]: 'Médico',
  [ABSENCE_TYPES.GENERAL]: 'Ausencia',
} as const;

export const LOCATION_TYPE_LABELS = {
  [LOCATION_TYPES.REMOTE_DECLARED]: 'Domicilio remoto declarado',
  [LOCATION_TYPES.REMOTE_ALTERNATIVE]: 'Domicilio remoto alternativo',
  [LOCATION_TYPES.CLIENT]: 'Domicilio del cliente',
  [LOCATION_TYPES.OFFICE]: 'Oficina de ABSTI',
} as const;
```

## Usage

- Always send numeric IDs to the API
- Use the labels for display in the UI
- The backend will validate that the IDs are valid enum values
- If you need to add new enum values, update both frontend and backend constants 