package handlers

import "time"

// dbTimeout acota las consultas a Mongo de las rutas sin tiempo límite propio
// (route.go): terminan antes de los 15 s del servidor (main.go), a tiempo de
// responder el error en vez de que se corte la conexión. /UploadData y /login
// usan el contexto con el tiempo límite de su ruta.
const dbTimeout = 10 * time.Second
