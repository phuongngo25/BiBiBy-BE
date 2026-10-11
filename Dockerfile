FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/seeder ./cmd/seeder
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/seed_demo ./cmd/seed_demo

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/server /app/server
COPY --from=builder /out/seeder /app/seeder
COPY --from=builder /out/seed_demo /app/seed_demo

COPY migrations /app/migrations
COPY DRIs.json /app/DRIs.json
COPY met_activities.json /app/met_activities.json
COPY tags.json /app/tags.json
COPY usda_core_foods.json /app/usda_core_foods.json
COPY vfa_dishes_db.json /app/vfa_dishes_db.json
COPY vfa_food_db.json /app/vfa_food_db.json

EXPOSE 8080

CMD ["/app/server"]
