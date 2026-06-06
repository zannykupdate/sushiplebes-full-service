package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InitDB() {
	// Obtenemos la conexión de las variables de entorno
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Println("WARNING: DATABASE_URL no configurada. Por favor configura tu string de conexión PostgreSQL.")
		log.Println("Ejemplo Windows: $env:DATABASE_URL=\"postgres://usuario:password@localhost:5432/sushipos\"")
		return
	}

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("ERROR: No se pudo parsear el string de conexión de la base de datos: %v", err)
	}

	// Iniciamos el pool de conexiones
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatalf("ERROR: No se pudo conectar a PostgreSQL: %v", err)
	}

	// Asignamos a la variable global DB (usada en webhook.go, bot.go y monitor.go)
	DB = pool
	log.Println("SUCCESS: Conexión a PostgreSQL (pgxpool) establecida exitosamente.")

	// Validar e inicializar la base de datos del MVP
	crearTablasAutomaticas()
}

func crearTablasAutomaticas() {
	if DB == nil {
		return
	}
	ctx := context.Background()
	query := `
	CREATE TABLE IF NOT EXISTS mensajes_raw (
		id SERIAL PRIMARY KEY,
		telefono VARCHAR(20),
		payload JSONB,
		creado_en TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS pedidos (
		id SERIAL PRIMARY KEY,
		telefono VARCHAR(20),
		nombre VARCHAR(100),
		detalles_orden TEXT,
		direccion_entrega TEXT,
		metodo_pago VARCHAR(50),
		total DECIMAL(10, 2),
		estado INT DEFAULT 0,
		creado_en TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS ganancias (
		id SERIAL PRIMARY KEY,
		pedido_id INT,
		monto DECIMAL(10, 2),
		creado_en TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS inventario (
		id SERIAL PRIMARY KEY,
		insumo VARCHAR(100),
		cantidad INT DEFAULT 100
	);

	-- Inserción de inventario base para el MVP si las tablas están vacías
	INSERT INTO inventario (insumo, cantidad) 
	SELECT 'Arroz', 100 WHERE NOT EXISTS (SELECT 1 FROM inventario WHERE insumo = 'Arroz');
	
	INSERT INTO inventario (insumo, cantidad) 
	SELECT 'Salmón', 100 WHERE NOT EXISTS (SELECT 1 FROM inventario WHERE insumo = 'Salmón');
	`

	_, err := DB.Exec(ctx, query)
	if err != nil {
		log.Fatalf("ERROR: Fallo inicializando tablas del MVP: %v", err)
	} else {
		log.Println("SUCCESS: Architectura de motor PostgreSQL y esquemas DDL (pedidos, raw, ganancias, inventario) en línea.")
	}
}
