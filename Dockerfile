# Stage 1: Build the Go program
FROM golang:1.20-alpine AS build
WORKDIR /opt/nk2-PLCcapture-go

# Copy the project files and build the program
COPY . .
RUN apk --no-cache add gcc musl-dev
RUN cd 1.5v && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main1.5v main.go

# Stage 2: Copy the built Go program into a minimal container
FROM alpine:3.14
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=build /opt/nk2-PLCcapture-go/1.5v/main1.5v /app/
COPY 1.5v/.env.local /app/.env.local

RUN chmod +x /app/main1.5v

CMD ["/app/main1.5v"]

# Build Image with command
# docker build -t nk2-msp:${version} .