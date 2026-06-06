# SUSHI LOSPLEBES | Sistema Automático de Pedidos por WhatsApp

Un sistema rápido, sencillo y automático diseñado especialmente para gestionar los pedidos del restaurante **Sushi Los Plebes**. 

Este sistema conecta un asistente virtual (Bot) en WhatsApp directamente con la cocina del restaurante. Funciona completamente solo: atiende al cliente, toma su orden, recibe sus datos de entrega y al finalizar, muestra la orden en una pantalla para los cocineros e imprime automáticamente el ticket de preparación.

---

## ¿Qué hace este sistema?

- **Asistente de WhatsApp Integrado:** Responde automáticamente a los clientes, les envía el menú y les toma la orden paso a paso como un cajero real.
- **Pantalla de Cocina en Tiempo Real:** Una página web para la cocina (Monitor) que atrapará los nuevos pedidos al instante, sin necesidad de recargar la página.
- **Impresión Automática (Tickets):** En cuanto el cliente termina de pedir por WhatsApp, el monitor manda a imprimir silenciosamente el ticket con el formato adecuado para ticketeras térmicas (58mm).
- **Control de Ventas e Inventario:** Guarda de forma segura cuánto dinero entró en el día y resta automáticamente insumos básicos (como arroz o salmón) para llevar un control del negocio.

---

## ¿Cómo es la experiencia del Cliente (El Embudo)?

1. **Saludo:** El cliente manda un mensaje a WhatsApp. El bot le da la bienvenida por su nombre y le manda el menú.
2. **Toma de Orden:** El cliente escribe todo lo que quiere pedir.
3. **Modalidad:** El bot le pregunta si quiere recoger en sucursal (PICKUP) o servicio a domicilio (DELIVERY).
4. **Pago:** El bot confirma cómo va a pagar (Efectivo o Transferencia) y calcula el cambio si es necesario.
5. **Cierre:** El bot confirma que la orden fue enviada a la cocina. ¡Magia! El cajero o cocinero ya tiene el ticket impreso.

---

## Instalación y Puesta en Marcha (Para el Administrador)

El sistema está construido con **Go (Golang)** y **PostgreSQL**, lo que significa que es súper rápido, no se traba y puede correr en casi cualquier computadora o servidor básico.

### 1. ¿Qué necesitas?
- Tener instalado **Go** y una base de datos **PostgreSQL**.
- Una cuenta en **Meta for Developers** (WhatsApp API) para poder enviar y recibir los mensajes de WhatsApp.

### 2. Configura los datos del restaurante
Crea un archivo llamado `.env` en la misma carpeta del proyecto y pon tus claves secretas:

```env
# Claves para que el bot pueda leer y escribir en WhatsApp
WHATSAPP_VERIFY_TOKEN="TuPalabraSecretaParaMeta"
WHATSAPP_ACCESS_TOKEN="ClaveLargaDeMeta..."
WHATSAPP_PHONE_ID="1234567890"

# Conexión a tu Base de Datos
DATABASE_URL="postgres://usuario:contraseña@localhost:5432/sushipos"

# El puerto donde correrá la página (por defecto 8080)
PORT="8080"
```

### 3. Enciende el Sistema
Abre tu consola o terminal, entra a la carpeta del proyecto y escribe:
```bash
go mod tidy
go run .
```

¡Listo! 
- Tu bot de WhatsApp ya está escuchando nuevas órdenes.
- Tu monitor de cocina está listo en: `http://localhost:8080/monitor`

*Desarrollado en 2026 para redefinir el estándar de servicio de Los Plebes.*
