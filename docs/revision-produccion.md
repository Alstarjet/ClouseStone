# Revisión de producción antes de pasar `main` a `production`

Comandos de **sólo lectura** para revisar la base de producción (`CloudStoneProd`)
antes de desplegar el respaldo por registro (rama `fix/respaldo-solido`) y los planes
(PR #9). Ninguno escribe: sólo `find`, `countDocuments`, `aggregate` sin `$out` ni
`$merge`, `getIndexes` y `getDefaultRWConcern`.

## Cómo entrar

- Con un usuario de Mongo **de sólo lectura** (rol `read` sobre `CloudStoneProd`), o
  desde *Atlas → Data Explorer → mongosh*.
- No pegues la URI con contraseña en el chat ni en un archivo; escríbela sólo en la
  terminal:

```sh
mongosh "<URI de sólo lectura>"
```

```js
use CloudStoneProd
```

## 1. Cuentas de pago: que todas sean `Amethyst` (PR #9)

Con el PR #9, `/UploadData` responde `403 plan_without_backup` a todo lo que no sea
`Amethyst`: una cuenta que paga y tiene otro `typeclient` deja de respaldar.

```js
// Cuántas cuentas hay de cada plan
db.users.aggregate([
  { $group: { _id: "$typeclient", cuentas: { $sum: 1 } } },
  { $sort: { cuentas: -1 } }
])

// Cuentas que tienen datos respaldados en la nube y NO son Amethyst:
// si alguna paga, hay que pasarla a Amethyst ANTES de desplegar.
db.clients.aggregate([
  { $group: { _id: "$usermongoid", clientes: { $sum: 1 }, ultimoRespaldo: { $max: "$backupat" } } },
  { $lookup: {
      from: "users",
      let: { uid: { $convert: { input: "$_id", to: "objectId", onError: null, onNull: null } } },
      pipeline: [
        { $match: { $expr: { $eq: ["$_id", "$$uid"] } } },
        { $project: { _id: 0, email: 1, typeclient: 1, planexpiresat: 1 } }
      ],
      as: "cuenta" } },
  { $unwind: { path: "$cuenta", preserveNullAndEmptyArrays: true } },
  { $match: { "cuenta.typeclient": { $ne: "Amethyst" } } },
  { $project: { _id: 0, usermongoid: "$_id", email: "$cuenta.email", typeclient: "$cuenta.typeclient",
      planexpiresat: "$cuenta.planexpiresat", clientes: 1, ultimoRespaldo: 1 } },
  { $sort: { ultimoRespaldo: -1 } }
])
```

Si una fila sale sin `email`, esos datos son de un `usermongoid` que ya no existe en
`users`.

## 2. Índices únicos `{usermongoid, uuid}` (I-08)

El backend los pide al arrancar (`EnsureSyncIndexes`), pero si ya había duplicados no
se crean y sólo queda en el log («índice único en … no creado»). Sin ellos, dos
subidas a la vez del mismo documento todavía podrían duplicarlo.

```js
["clients", "charges", "orders", "payments"].forEach(c => {
  const idx = db.getCollection(c).getIndexes().find(i => i.name === "usermongoid_uuid_unique")
  print(c.padEnd(9), idx ? (idx.unique === true ? "OK: índice único" : "existe pero NO es único") : "FALTA el índice único")
})
```

## 3. Duplicados por (usuario, uuid) (I-08)

Si el paso 2 dice «FALTA», casi seguro hay duplicados. Antes de crear el índice hay
que limpiarlos (a mano, con permiso: es una escritura).

```js
// Cuántos grupos duplicados hay en cada colección
["clients", "charges", "orders", "payments"].forEach(c => {
  const r = db.getCollection(c).aggregate([
    { $group: { _id: { u: "$usermongoid", uuid: "$uuid" }, n: { $sum: 1 } } },
    { $match: { n: { $gt: 1 } } },
    { $count: "grupos" }
  ], { allowDiskUse: true }).toArray()
  print(c.padEnd(9), r.length ? r[0].grupos + " grupos duplicados" : "sin duplicados")
})

// Detalle de una colección (cámbiala): cada copia con su backupat y updateat, para
// decidir cuál se queda (la de backupat más reciente es la última que llegó)
db.clients.aggregate([
  { $group: { _id: { u: "$usermongoid", uuid: "$uuid" }, n: { $sum: 1 },
      copias: { $push: { _id: "$_id", backupat: "$backupat", updateat: "$updateat", status: "$status" } } } },
  { $match: { n: { $gt: 1 } } },
  { $limit: 20 }
], { allowDiskUse: true })

// Documentos sin uuid o con uuid vacío (chocan entre sí en el índice único)
["clients", "charges", "orders", "payments"].forEach(c => {
  print(c.padEnd(9), db.getCollection(c).countDocuments({ $or: [{ uuid: { $exists: false } }, { uuid: "" }, { uuid: null }] }), "sin uuid")
})
```

## 4. Documento de dispositivos: tamaño y cola (I-07)

La cola por dispositivo ya no crece con este cambio, pero lo que acumuló sigue ahí. Un
documento cerca de 16 MB (16,384 KB) haría fallar el login en un dispositivo nuevo.

```js
// Los 20 documentos más grandes, con cuántos dispositivos y cuántos ids pendientes
db.devices.aggregate([
  { $project: {
      usermongoid: 1,
      tamanoKB: { $round: [{ $divide: [{ $bsonSize: "$$ROOT" }, 1024] }, 1] },
      dispositivos: { $size: { $ifNull: ["$devices", []] } },
      pendientes: { $sum: { $map: { input: { $ifNull: ["$devices", []] }, as: "d", in: { $add: [
        { $size: { $ifNull: ["$$d.clientids", []] } }, { $size: { $ifNull: ["$$d.chargeids", []] } },
        { $size: { $ifNull: ["$$d.orderids", []] } }, { $size: { $ifNull: ["$$d.paymentids", []] } } ] } } } } } },
  { $sort: { tamanoKB: -1 } },
  { $limit: 20 }
])

// Dispositivos repetidos dentro de una cuenta (antes, dos logins a la vez del mismo
// dispositivo podían dejarlo dos veces; /GetJwt compara contra la primera entrada)
db.devices.aggregate([
  { $unwind: "$devices" },
  { $group: { _id: { doc: "$_id", uuid: "$devices.uuid" }, n: { $sum: 1 } } },
  { $match: { n: { $gt: 1 } } },
  { $count: "repetidos" }
])

// Documentos cuya lista de dispositivos no es una lista (el login no podría agregar uno)
db.devices.countDocuments({ devices: { $not: { $type: "array" } } })
```

## 5. Write concern

Desde este cambio el backend usa `w=majority`, salvo que `DATA_BASE_APPLY_URI` ya traiga
`w=`, `journal=` o `wtimeoutMS=`. Revisa la URI en Cloud Run (Variables) sin copiarla a
ningún lado: si trae `w=1`, cámbiala a `w=majority` o quítale el `w`.

```js
// El write concern predeterminado del clúster (sólo lectura)
db.adminCommand({ getDefaultRWConcern: 1 })
```

## 6. Fuera de mongosh

- **Respaldos de Atlas (I-01):** *Atlas → Backup*: confirmar el nivel del clúster y que
  haya copias. Es lo primero.
- **Latencia de `/UploadData` (I-03):** *Cloud Run → clousestone → Métricas →
  Latencia de solicitudes* (p95 y p99). Con este cambio, cada lote son 5 viajes a Mongo
  en vez de dos por registro.
- **Logs después de desplegar:** buscar `índice único en` (paso 2), `registros
  rechazados` (lo que el 422 rechazó, con colección, uuid y motivo) y `UpsertBackup`
  (errores de escritura).
