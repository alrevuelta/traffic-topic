FROM golang:1.18-alpine AS build

WORKDIR /app

COPY . .

RUN apk add --update gcc g++
RUN go mod download
RUN go build -o /main

FROM golang:1.18-alpine

WORKDIR /

COPY --from=build /main /main

ENTRYPOINT ["/main"]
