# Usa la imagen oficial de Golang
FROM golang:1.22

# Establece el directorio de trabajo dentro del contenedor
WORKDIR /go/src/app

# Copia el código del proyecto al contenedor
COPY . .

# Descarga las dependencias del proyecto
RUN go mod download

# Compila el proyecto
RUN go build -o app .

# Expone el puerto 8080
EXPOSE 8080

# Comando para ejecutar la aplicación
CMD ["./app"]
