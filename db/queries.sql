-- name: GetAllRecords :many
SELECT * from weather;

-- name: WriteRecord :one
INSERT INTO weather (
    record_date,
    temperature,
    pressure,
    rain_mm,
    wind_speed,
    wind_gust,
    wind_direction
) VALUES (
    now(), $1, $2, $3, $4, $5, $6
);