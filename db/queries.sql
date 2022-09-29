-- name: GetAllRecords :many
SELECT * from weather;

-- name: WriteRecord :one
INSERT INTO weather (
    record,
    temperature,
    pressure,
    rain_mm,
    wind_speed,
    wind_direction
) VALUES (
    now(), $1, $2, $3, $4, $5
) 
RETURNING record;