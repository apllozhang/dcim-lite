# Compatibility decisions

| ID | Topic | Decision | Class |
| --- | --- | --- | --- |
| D1 | Duplicate PDU code | 409 `DUPLICATE_CODE` | INTENTIONAL_FIX |
| D2 | Duplicate socket number | 409 | INTENTIONAL_FIX |
| D3 | Duplicate power-role connect | 409 `SOCKET_CONNECTED` | INTENTIONAL_FIX |
| D4 | Rack template snapshot | Persist jsonb on create | INTENTIONAL_FIX |
| V1 | Optimistic lock | `?version=` on PUT/DELETE | MATCH |
