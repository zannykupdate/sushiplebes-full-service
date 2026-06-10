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
	// Prevenir error "prepared statement already exists" causados por PgBouncer o poolers
	if !strings.Contains(databaseURL, "?") {
		databaseURL += "?default_query_exec_mode=exec"
	} else if !strings.Contains(databaseURL, "default_query_exec_mode") {
		databaseURL += "&default_query_exec_mode=exec"
	}

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
		subtotal NUMERIC,
		tax NUMERIC,
		shipping NUMERIC,
		total NUMERIC,
		status VARCHAR(50) DEFAULT 'PENDING',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS earnings (
		id SERIAL PRIMARY KEY,
		amount NUMERIC NOT NULL,
		order_id INT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS support_tickets (
		id SERIAL PRIMARY KEY,
		telefono VARCHAR(50),
		mensaje TEXT,
		status VARCHAR(50) DEFAULT 'OPEN',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := DB.Exec(context.Background(), query)
	if err != nil {
		log.Printf("ERROR: Fallo inicializando tablas del MVP: %v", err)
	} else {
		// Intentando añadir las columnas para dbs existentes
		DB.Exec(context.Background(), "ALTER TABLE orders ADD COLUMN created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP")
		DB.Exec(context.Background(), "ALTER TABLE earnings ADD COLUMN created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP")
		
		// Llenar inventario para al menos 10 sushis si está vacío o para pruebas
		seedInventory(DB)

		log.Println("SUCCESS: Architectura de motor PostgreSQL y esquemas DDL en línea.")
	}
}

func seedInventory(db *pgxpool.Pool) {
	var count int
	db.QueryRow(context.Background(), "SELECT COUNT(*) FROM inventory").Scan(&count)
	if count < 15 {
		items := []struct{name string; qty int}{
			{"arroz 265g", 10},
			{"proteinas 50g", 10},
			{"pollo 40g", 10},
			{"pepino 20g", 10},
			{"zanahoria 15g", 10},
			{"cebolla 10g", 10},
			{"queso_philadelphia 30g", 10},
			{"aderezo 10g", 10},
			{"salsa_soya 1", 10},
			{"salsa_roja 1", 10},
			{"contenedor_7x7 1", 10},
			{"p200 1", 10},
			{"palillos_chinos 1", 10},
			{"aluminio 1", 10},
			{"servilletas 2", 10},
		}

		for _, item := range items {
			_, err := db.Exec(context.Background(), "INSERT INTO inventory (item, quantity) VALUES ($1, $2)", item.name, item.qty)
			if err != nil {
				log.Printf("Error seeding inventory %s: %v", item.name, err)
			}
		}
		log.Println("✅ INVENTARIO SEMBRADO PARA 10 SUSHIS DE PRUEBA")
	}
}

func InsertOrder(ctx context.Context, nombre, telefono, detalles, direccion, pago string, subtotal, tax, shipping, total float64, inventoryToRemove map[string]int) (int, error) {
	var id int
	err := DB.QueryRow(ctx, "INSERT INTO orders (nombre, telefono, detalles_orden, direccion_entrega, metodo_pago, subtotal, tax, shipping, total) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id",
		nombre, telefono, detalles, direccion, pago, subtotal, tax, shipping, total).Scan(&id)
	
	if err == nil {
		// impactar ganancias
		DB.Exec(ctx, "INSERT INTO earnings (amount, order_id) VALUES ($1, $2)", total, id)
		
		// Descontar inventario estructurado por Gemini
		for item, count := range inventoryToRemove {
			// Update the quantity if the item exists
			query := `
				UPDATE inventory 
				SET quantity = quantity - $1 
				WHERE item = $2`
			tag, errExec := DB.Exec(ctx, query, count, item)
			if errExec == nil && tag.RowsAffected() == 0 {
			    // If it didn't exist, we add it with negative quantity as a fallback.
			    DB.Exec(ctx, "INSERT INTO inventory (item, quantity) VALUES ($1, $2)", item, -count)
			}
		}
	}
	return id, err
}
