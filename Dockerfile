# Stage 1: Build the Go program
FROM golang:1.20-alpine AS build
WORKDIR /opt/nk2-PLCcapture-go

# Copy the project files and build the program
COPY . .
RUN apk --no-cache add gcc musl-dev
RUN cd 16+32bit && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main16+32bit main.go

# Stage 2: Copy the built Go program into a minimal container
FROM alpine:3.14
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=build /opt/nk2-PLCcapture-go/16+32bit/main16+32bit /app/
COPY 16+32bit/.env.local /app/.env.local

RUN chmod +x /app/main16+32bit

CMD ["/app/main16+32bit"]
