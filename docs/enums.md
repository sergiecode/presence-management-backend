# Enum Values for Frontend

## Checkin LocationType (int)
- 1: Domicilio remoto declarado
- 2: Domicilio remoto alternativo
- 3: Domicilio del cliente
- 4: Oficina de ABSTI

## API Usage Examples

### First check-in of the day
```json
POST /api/checkins
{
  "time": "2025-07-17T08:30:00Z",
  "notes": "Starting at office",
  "locations": [
    {
      "location_type": 4,
      "location_detail": "ABSTI Office - Floor 4"
    }
  ]
}
```

### Change location during the day
```json
PUT /api/checkins/locations
{
  "locations": [
    {
      "location_type": 4,
      "location_detail": "ABSTI Office - Floor 4"
    },
    {
      "location_type": 3,
      "location_detail": "Client XYZ Office"
    }
  ]
}
```

### Response format
```json
{
  "id": 1,
  "user_id": 1,
  "time": "2025-07-17T08:30:00Z",
  "notes": "On time check-in",
  "late": false,
  "locations": [
    {
      "id": 1,
      "location_type": 4,
      "location_detail": "ABSTI Office - Floor 4"
    },
    {
      "id": 2,
      "location_type": 3,
      "location_detail": "Client XYZ Office"
    }
  ]
}
```

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