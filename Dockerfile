FROM golang:1.22.2-alpine3.19 AS build
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go build -o librerest

FROM alpine:3.19
COPY --from=build /app/librerest /librerest
COPY templates /templates
COPY static /static
COPY --from=build /app/.env /.env
EXPOSE 3000
CMD ["/librerest"]