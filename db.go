package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
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
	CREATE TABLE IF NOT EXISTS menu_items (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		description TEXT,
		price NUMERIC NOT NULL,
		category VARCHAR(50) DEFAULT 'Sushi',
		is_active BOOLEAN DEFAULT TRUE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS expenses (
		id SERIAL PRIMARY KEY,
		description TEXT NOT NULL,
		amount NUMERIC NOT NULL,
		category VARCHAR(100),
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
		
		// Añadir validación de inventario negativo
		_, errConstraint := DB.Exec(context.Background(), "ALTER TABLE inventory ADD CONSTRAINT check_qty_positive CHECK (quantity >= 0)")
		if errConstraint != nil {
			log.Printf("INFO: constraint check_qty_positive ya existe o fallo: %v", errConstraint)
		}
		
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

	var menuCount int
	db.QueryRow(context.Background(), "SELECT COUNT(*) FROM menu_items").Scan(&menuCount)
	if menuCount == 0 {
		menuItems := []struct {
			name  string
			desc  string
			price float64
			cat   string
		}{
			{"Rollo Estándar", "Rollo clásico con ingredientes básicos", 120.0, "Sushi"},
			{"Rollo Especial", "Rollo con mariscos premium y toppings", 150.0, "Sushi"},
			{"Té Helado", "Té helado de la casa", 35.0, "Bebida"},
		}
		for _, m := range menuItems {
			db.Exec(context.Background(), "INSERT INTO menu_items (name, description, price, category) VALUES ($1, $2, $3, $4)", m.name, m.desc, m.price, m.cat)
		}
		log.Println("✅ MENÚ SEMBRADO")
	}

	// Forzado de reseteo de inventario a 10.
	db.Exec(context.Background(), "UPDATE inventory SET quantity = 10")
	log.Println("🚀 INVENTARIO RESETEADO A 10 (HARD RESET)")
}

func InsertOrder(ctx context.Context, nombre, telefono, detalles, direccion, pago string, subtotal, tax, shipping, total decimal.Decimal, inventoryToRemove map[string]int) (int, error) {
	tx, err := DB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var id int
	err = tx.QueryRow(ctx, "INSERT INTO orders (nombre, telefono, detalles_orden, direccion_entrega, metodo_pago, subtotal, tax, shipping, total) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id",
		nombre, telefono, detalles, direccion, pago, subtotal, tax, shipping, total).Scan(&id)
	
	if err != nil {
		return 0, err
	}

	// impactar ganancias
	_, err = tx.Exec(ctx, "INSERT INTO earnings (amount, order_id) VALUES ($1, $2)", total, id)
	if err != nil {
		return 0, err
	}
	
	// Descontar inventario estructurado por Gemini
	for item, count := range inventoryToRemove {
		// Update the quantity if the item exists
		query := `
			UPDATE inventory 
			SET quantity = quantity - $1 
			WHERE item = $2`
		tag, errExec := tx.Exec(ctx, query, count, item)
		if errExec != nil {
			return 0, errExec
		}
		if tag.RowsAffected() == 0 {
			// If it didn't exist, we add it with negative quantity as a fallback.
			_, errExec = tx.Exec(ctx, "INSERT INTO inventory (item, quantity) VALUES ($1, $2)", item, -count)
			if errExec != nil {
				return 0, errExec
			}
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return 0, err
	}

	return id, nil
}
