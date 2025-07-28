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

## User Location Management (Self-Service)

### Get today's locations

```json
GET /api/checkins/locations
```

**Response:**

```json
[
  {
    "id": 1,
    "checkin_id": 5,
    "location_type": 4,
    "location_detail": "ABSTI Office - Floor 4",
    "created_at": "2025-07-28T10:30:00Z"
  },
  {
    "id": 2,
    "checkin_id": 5,
    "location_type": 3,
    "location_detail": "Client XYZ Office",
    "created_at": "2025-07-28T14:00:00Z"
  }
]
```

### Add locations during the day

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

**Note**: This endpoint adds new locations to the existing array. It does not replace previous locations.

### Delete a specific location

```json
DELETE /api/checkins/locations/{id}
```

**Response:**

```json
{
  "message": "Location deleted successfully",
  "success": true
}
```

## HR/Admin Location Management (Dashboard)

### Get locations for any check-in

```json
GET /api/dashboard/locations/{checkin_id}
```

### Add locations to any check-in

```json
PUT /api/dashboard/locations/{checkin_id}
{
  "locations": [
    {
      "location_type": 4,
      "location_detail": "ABSTI Office - Floor 4"
    }
  ]
}
```

### Delete any location

```json
DELETE /api/dashboard/locations/{location_id}
```

**Note**: HR/Admin endpoints require HR or Admin role. Users can only manage their own locations through the `/api/checkins/locations` endpoints.

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

### Dashboard Attendance Response

```json
{
  "data": [
    {
      "user_id": 1,
      "name": "John Doe",
      "email": "john@example.com",
      "checkin_id": 1,
      "checkin_time": "2025-07-17T08:30:00-03:00",
      "late": false,
      "location_type": 4,
      "location_detail": "ABSTI Office - Floor 4",
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
      ],
      "notes": "On time check-in",
      "late_reason": "",
      "checkout_time": null,
      "checkout_status": "",
      "overtime": false
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 1,
    "total": 1,
    "total_pages": 1
  }
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
  [ABSENCE_TYPES.MATERNITY]: "Licencia por maternidad",
  [ABSENCE_TYPES.SICK_LEAVE]: "Licencia por enfermedad",
  [ABSENCE_TYPES.SICK_ABSENCE]: "Ausente por enfermedad",
  [ABSENCE_TYPES.FAMILY_SICK]: "Ausente por enfermedad familiar",
  [ABSENCE_TYPES.STUDY]: "Ausente por día de estudio/examen",
  [ABSENCE_TYPES.BEREAVEMENT]: "Ausente por duelo",
  [ABSENCE_TYPES.MOVING]: "Día por mudanza",
  [ABSENCE_TYPES.VACATION]: "Vacaciones",
  [ABSENCE_TYPES.LATE]: "Tarde",
  [ABSENCE_TYPES.MEDICAL]: "Médico",
  [ABSENCE_TYPES.GENERAL]: "Ausencia",
} as const;

export const LOCATION_TYPE_LABELS = {
  [LOCATION_TYPES.REMOTE_DECLARED]: "Domicilio remoto declarado",
  [LOCATION_TYPES.REMOTE_ALTERNATIVE]: "Domicilio remoto alternativo",
  [LOCATION_TYPES.CLIENT]: "Domicilio del cliente",
  [LOCATION_TYPES.OFFICE]: "Oficina de ABSTI",
} as const;
```

## Usage

- Always send numeric IDs to the API
- Use the labels for display in the UI
- The backend will validate that the IDs are valid enum values
- If you need to add new enum values, update both frontend and backend constants

## Timezone Handling

### Check-in Time Submission

- **With timezone indicator (Z or +00:00)**: Time is treated as UTC and stored as-is
- **Without timezone indicator**: Time is interpreted as user's local timezone and converted to UTC for storage

### Time Display

- All times are returned in the user's configured timezone
- If no timezone is configured, times are returned in UTC
- Frontend should display times in the user's local timezone

### Example

If user has timezone "America/Argentina/Buenos_Aires" (GMT-3):

- User submits: `"time": "2025-07-28T07:55:00"` (no Z suffix)
- Backend stores: `2025-07-28T10:55:00Z` (UTC)
- Frontend receives: `"time": "2025-07-28T07:55:00-03:00"` (user's timezone)
