package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

func InitDB(databaseURL string) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		log.Printf("ERROR: Error parsing DATABASE_URL: %v", err)
		return
	}

	var pool *pgxpool.Pool
	maxRetries := 5
	for i := 1; i <= maxRetries; i++ {
		pool, err = pgxpool.NewWithConfig(context.Background(), poolConfig)
		if err == nil {
			err = pool.Ping(context.Background())
		}
		
		if err == nil {
			DB = pool
			log.Println("SUCCESS: Conexión a PostgreSQL (pgxpool) establecida exitosamente.")
			crearTablasAutomaticas()
			return
		}
		
		log.Printf("WARNING: Intento %d/%d fallido al conectar a PostgreSQL: %v", i, maxRetries, err)
		if pool != nil {
			pool.Close()
		}
		time.Sleep(2 * time.Second)
	}

	log.Printf("ERROR: No se pudo conectar a PostgreSQL tras %d intentos.", maxRetries)
}

func crearTablasAutomaticas() {
	query := `
	CREATE TABLE IF NOT EXISTS inventory (
		id SERIAL PRIMARY KEY,
		item VARCHAR(100) NOT NULL,
		quantity INT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS orders (
		id SERIAL PRIMARY KEY,
		nombre VARCHAR(100),
		telefono VARCHAR(50),
		detalles_orden TEXT,
		direccion_entrega TEXT,
		metodo_pago VARCHAR(50),
		total NUMERIC,
		status VARCHAR(50) DEFAULT 'PENDING'
	);
	CREATE TABLE IF NOT EXISTS earnings (
		id SERIAL PRIMARY KEY,
		amount NUMERIC NOT NULL,
		order_id INT
	);
	`
	_, err := DB.Exec(context.Background(), query)
	if err != nil {
		log.Printf("ERROR: Fallo inicializando tablas del MVP: %v", err)
	} else {
		log.Println("SUCCESS: Architectura de motor PostgreSQL y esquemas DDL en línea.")
	}
}

func InsertOrder(ctx context.Context, nombre, telefono, detalles, direccion, pago string, total float64) (int, error) {
	var id int
	err := DB.QueryRow(ctx, "INSERT INTO orders (nombre, telefono, detalles_orden, direccion_entrega, metodo_pago, total) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		nombre, telefono, detalles, direccion, pago, total).Scan(&id)
	
	if err == nil {
		// impactar ganancias e inventario de manera super simple
		DB.Exec(ctx, "INSERT INTO earnings (amount, order_id) VALUES ($1, $2)", total, id)
		if strings.Contains(strings.ToLower(detalles), "arroz") {
			DB.Exec(ctx, "UPDATE inventory SET quantity = quantity - 1 WHERE item = 'arroz'")
		}
	}
	return id, err
}
